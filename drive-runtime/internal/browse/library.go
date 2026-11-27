package browse

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
)

// ErrLibraryUnavailable means the drive's Kiwix installation has no companion
// kiwix-manage. A caller can still run the standalone reader.
var ErrLibraryUnavailable = errors.New("a shared Kiwix library requires kiwix-manage")

// Library owns a single loopback Kiwix process. Archives are added on demand
// and removed when their last lease is closed. The listener is kept until Close.
type Library struct {
	root   string
	ctx    context.Context
	cancel context.CancelFunc

	// A channel lets queued acquisitions abandon their wait on cancellation.
	gate    chan struct{}
	process *libraryProcess
}

// Archive is a lease on one book in a running library.
// ContentName is the archive alias as advertised by Kiwix, for article URLs.
type Archive struct {
	BaseURL     string
	ViewerURL   string
	ContentName string
	Exited      <-chan struct{}
	release     func()
	once        sync.Once
}

// Close releases the lease. Other leases on the archive remain usable.
func (a *Archive) Close() {
	a.once.Do(a.release)
}

// NewLibrary prepares an on-demand library without opening a listener.
func NewLibrary(driveRoot string) *Library {
	ctx, cancel := context.WithCancel(context.Background())
	return &Library{root: driveRoot, ctx: ctx, cancel: cancel, gate: make(chan struct{}, 1)}
}

// Acquire waits for target to appear in Kiwix's catalog and returns its viewer
// URL. The caller must close the lease when it no longer needs the archive.
func (l *Library) Acquire(ctx context.Context, target string) (*Archive, error) {
	if target == "" || strings.HasPrefix(target, ".") ||
		strings.ContainsAny(target, "/\\") || !strings.EqualFold(filepath.Ext(target), ".zim") {
		return nil, fmt.Errorf("invalid archive filename: %q", target)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	remove := context.AfterFunc(l.ctx, cancel)
	defer remove()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case l.gate <- struct{}{}:
	}
	defer l.unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := l.ctx.Err(); err != nil {
		return nil, err
	}

	if l.process != nil && l.process.stopped() {
		l.stopProcess()
	}
	process := l.process
	if process != nil {
		if book := process.byTarget[target]; book != nil {
			return l.lease(process, book), nil
		}
	}
	targets, err := ResolveTargets(l.root, target)
	if err != nil {
		return nil, err
	}
	filename, err := filepath.Abs(targets[0])
	if err != nil {
		return nil, err
	}

	if process == nil {
		process, err = l.start(ctx, target, filename)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			process.close()
			return nil, err
		}
		l.process = process
		return l.lease(process, process.byTarget[target]), nil
	}

	metadata, err := process.describe(ctx, filename)
	if err != nil {
		return nil, err
	}
	record := metadata.Books[0]
	id := record.id()
	if book := process.books[id]; book != nil {
		process.byTarget[target] = book
		return l.lease(process, book), nil
	}
	books := process.records()
	updated := append(books, record)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := process.writeLibrary(updated); err != nil {
		return nil, err
	}

	// After publishing the XML, finish or roll back the reload independently
	// of this request. Another lease may still be using the old catalog.
	catalog, err := process.reload(l.ctx, updated)
	if err != nil {
		restoreErr := process.writeLibrary(books)
		if restoreErr == nil {
			_, restoreErr = process.reload(l.ctx, books)
		}
		if restoreErr != nil {
			l.stopProcess()
		}
		return nil, err
	}
	book := &loadedArchive{record: record, contentName: catalog[id]}
	process.books[id] = book
	process.byTarget[target] = book
	if err := ctx.Err(); err != nil {
		l.removeArchive(process, book)
		return nil, err
	}
	return l.lease(process, book), nil
}

func (l *Library) lease(process *libraryProcess, book *loadedArchive) *Archive {
	book.leases++
	viewer := url.URL{Path: "/viewer", Fragment: book.contentName}
	return &Archive{
		BaseURL:     process.baseURL,
		ViewerURL:   process.baseURL + viewer.String(),
		ContentName: book.contentName,
		Exited:      process.exited,
		release: func() {
			select {
			case <-l.ctx.Done():
				return
			case l.gate <- struct{}{}:
			}
			defer l.unlock()
			if l.process != process || l.ctx.Err() != nil {
				return
			}
			book.leases--
			if book.leases == 0 {
				l.removeArchive(process, book)
			}
		},
	}
}

func (l *Library) removeArchive(process *libraryProcess, book *loadedArchive) {
	delete(process.books, book.record.id())
	for target, candidate := range process.byTarget {
		if candidate == book {
			delete(process.byTarget, target)
		}
	}
	records := process.records()
	if err := process.writeLibrary(records); err == nil {
		if _, err := process.reload(l.ctx, records); err == nil {
			return
		}
	}
	// If the removal cannot be confirmed, stop the process so it cannot
	// continue to serve an archive after its last lease was released.
	l.stopProcess()
}

func (l *Library) unlock() {
	<-l.gate
}

// Close cancels in-flight work, reaps Kiwix and removes its temporary catalog.
func (l *Library) Close() {
	l.cancel()
	l.gate <- struct{}{}
	defer l.unlock()
	l.stopProcess()
}

func (l *Library) stopProcess() {
	if l.process != nil {
		l.process.close()
		l.process = nil
	}
}

type loadedArchive struct {
	record      bookRecord
	contentName string
	leases      int
}

type libraryProcess struct {
	dir     string
	manage  string
	baseURL string
	version string

	cmd    *exec.Cmd
	cancel context.CancelFunc
	exited chan struct{}
	// Set before exited is closed.
	exitErr error
	stderr  bytes.Buffer

	books    map[string]*loadedArchive
	byTarget map[string]*loadedArchive
}

func (l *Library) start(ctx context.Context, target, filename string) (*libraryProcess, error) {
	serve, err := binary.Resolve("kiwix-serve", l.root)
	if err != nil {
		return nil, err
	}
	// Prefer the companion from the same bundle as the running server.
	manage := filepath.Join(filepath.Dir(serve), "kiwix-manage")
	if info, err := os.Stat(manage); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		manage, err = binary.Resolve("kiwix-manage", l.root)
		if err != nil {
			return nil, ErrLibraryUnavailable
		}
	}
	dir, err := os.MkdirTemp("", "svalbard-kiwix-")
	if err != nil {
		return nil, err
	}
	process := &libraryProcess{
		dir: dir, manage: manage,
		exited:   make(chan struct{}),
		books:    make(map[string]*loadedArchive),
		byTarget: make(map[string]*loadedArchive),
	}
	ready := false
	defer func() {
		if !ready {
			process.close()
		}
	}()
	metadata, err := process.describe(ctx, filename)
	if err != nil {
		return nil, err
	}
	process.version = metadata.Version
	record := metadata.Books[0]
	if err := process.writeLibrary(metadata.Books); err != nil {
		return nil, err
	}
	port, release, err := netutil.ReserveEphemeralPort("127.0.0.1")
	if err != nil {
		return nil, err
	}
	process.baseURL = fmt.Sprintf("http://localhost:%d", port)
	runCtx, stop := context.WithCancel(l.ctx)
	cmd := exec.CommandContext(runCtx, serve, "--address", "127.0.0.1", "--port", fmt.Sprint(port), "--library", process.libraryPath())
	cmd.Stderr = &process.stderr
	if err := cmd.Start(); err != nil {
		release()
		stop()
		return nil, fmt.Errorf("starting Kiwix: %w", err)
	}
	process.cmd, process.cancel = cmd, stop
	go func() {
		process.exitErr = cmd.Wait()
		release()
		close(process.exited)
	}()
	catalog, err := process.waitForCatalog(ctx, metadata.Books)
	if err != nil {
		return nil, err
	}
	id := record.id()
	book := &loadedArchive{record: record, contentName: catalog[id]}
	process.books[id] = book
	process.byTarget[target] = book
	ready = true
	return process, nil
}

func (p *libraryProcess) close() {
	if p.cmd != nil {
		p.cancel()
		<-p.exited
	}
	_ = os.RemoveAll(p.dir)
}

func (p *libraryProcess) stopped() bool {
	select {
	case <-p.exited:
		return true
	default:
		return false
	}
}

type libraryDocument struct {
	XMLName xml.Name     `xml:"library"`
	Version string       `xml:"version,attr"`
	Books   []bookRecord `xml:"book"`
}

// Preserve all metadata that kiwix-manage wrote, including attributes and
// nested elements unfamiliar to the runtime.
type bookRecord struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Inner string     `xml:",innerxml"`
}

func (b bookRecord) id() string {
	for _, attr := range b.Attrs {
		if attr.Name.Local == "id" {
			return attr.Value
		}
	}
	return ""
}

func (p *libraryProcess) describe(ctx context.Context, filename string) (libraryDocument, error) {
	staged := filepath.Join(p.dir, "single.xml")
	defer os.Remove(staged)
	cmd := exec.CommandContext(ctx, p.manage, staged, "add", filename)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return libraryDocument{}, ctx.Err()
		}
		return libraryDocument{}, fmt.Errorf("reading Kiwix metadata for %s: %w %s", filepath.Base(filename), err, strings.TrimSpace(stderr.String()))
	}
	file, err := os.Open(staged)
	if err != nil {
		return libraryDocument{}, err
	}
	defer file.Close()
	var doc libraryDocument
	if err := xml.NewDecoder(file).Decode(&doc); err != nil {
		return libraryDocument{}, fmt.Errorf("reading Kiwix library record: %w", err)
	}
	if doc.Version == "" || len(doc.Books) != 1 || doc.Books[0].id() == "" {
		return libraryDocument{}, fmt.Errorf("Kiwix did not describe the archive %s", filepath.Base(filename))
	}
	return doc, nil
}

func (p *libraryProcess) records() []bookRecord {
	records := make([]bookRecord, 0, len(p.books))
	for _, book := range p.books {
		records = append(records, book.record)
	}
	slices.SortFunc(records, func(a, b bookRecord) int { return strings.Compare(a.id(), b.id()) })
	return records
}

func (p *libraryProcess) libraryPath() string {
	return filepath.Join(p.dir, "library.xml")
}

func (p *libraryProcess) writeLibrary(records []bookRecord) error {
	file, err := os.CreateTemp(p.dir, "library-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = xml.NewEncoder(file).Encode(libraryDocument{Version: p.version, Books: records})
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	return os.Rename(file.Name(), p.libraryPath())
}

func (p *libraryProcess) reload(ctx context.Context, records []bookRecord) (map[string]string, error) {
	if err := p.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		return nil, fmt.Errorf("reloading Kiwix library: %w", err)
	}
	return p.waitForCatalog(ctx, records)
}

func (p *libraryProcess) waitForCatalog(ctx context.Context, records []bookRecord) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()
	tick := time.NewTicker(60 * time.Millisecond)
	defer tick.Stop()
	for {
		catalog, err := readLibraryCatalog(ctx, client, p.baseURL)
		if err == nil && catalogMatches(catalog, records) {
			return catalog, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for Kiwix library: %w", ctx.Err())
		case <-p.exited:
			if p.exitErr == nil {
				return nil, fmt.Errorf("Kiwix exited while updating its library")
			}
			return nil, fmt.Errorf("Kiwix exited while updating its library: %w %s", p.exitErr, strings.TrimSpace(p.stderr.String()))
		case <-tick.C:
		}
	}
}

func catalogMatches(catalog map[string]string, records []bookRecord) bool {
	if len(catalog) != len(records) {
		return false
	}
	for _, record := range records {
		if catalog[record.id()] == "" {
			return false
		}
	}
	return true
}

func readLibraryCatalog(ctx context.Context, client *http.Client, baseURL string) (map[string]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/catalog/v2/entries?count=-1", nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kiwix catalog returned %s", response.Status)
	}
	var feed struct {
		XMLName xml.Name `xml:"feed"`
		Entries []struct {
			ID    string `xml:"id"`
			Links []struct {
				Type string `xml:"type,attr"`
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&feed); err != nil {
		return nil, err
	}
	catalog := make(map[string]string, len(feed.Entries))
	for _, entry := range feed.Entries {
		for _, link := range entry.Links {
			if link.Type != "text/html" {
				continue
			}
			address, err := url.Parse(link.Href)
			if err != nil || address.Scheme != "" || address.Host != "" {
				continue
			}
			name, ok := strings.CutPrefix(address.Path, "/content/")
			if ok && name != "" && !strings.Contains(name, "/") {
				catalog[strings.TrimPrefix(entry.ID, "urn:uuid:")] = name
				break
			}
		}
	}
	return catalog, nil
}
