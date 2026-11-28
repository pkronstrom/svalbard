package search

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

// A document hit is opened in the browser at its page. Browsers honour
// "#page=N" for a PDF they load over HTTP, while a file:// link goes to
// whatever application the host has for PDFs and loses the page. So the
// session serves the drive's books/ folder on a loopback port for as long as
// it lives. The workspace server does the serving: read-only, one folder,
// loopback Host names only, range requests for the viewer, and documents
// only, not the records a download leaves beside them.

// IsDocumentSource reports whether a result's source is a document on the
// drive rather than a ZIM archive.
func IsDocumentSource(filename string) bool { return engine.IsDocumentSource(filename) }

// SourceLabel is a source as a results list names it: an archive without its
// extension, a document by its path below books/, and a document inside an
// archive by the archive's name and its path there.
func SourceLabel(filename string) string {
	if archive, entry, ok := engine.EmbeddedArchive(filename); ok {
		return strings.TrimSuffix(archive, ".zim") + "/" + entry
	}
	if IsDocumentSource(filename) {
		return strings.TrimPrefix(filename, engine.DocumentsDir+"/")
	}
	return strings.TrimSuffix(filename, ".zim")
}

// DocumentFile is a document page as a drive-relative address:
// "books/manuals/volvo.pdf#page=12". A document inside an archive is no file
// on the drive and has none.
func DocumentFile(filename string, page int) string {
	if engine.IsEmbeddedSource(filename) {
		return ""
	}
	return fmt.Sprintf("%s#page=%d", filename, page)
}

// embeddedURL is the kiwix-serve address of a document inside an archive,
// at its page for a PDF: browsers honour "#page=N" only there. Each segment
// of the entry path is escaped: nautilus files are named like
// "Emergency Preparedness (10).pdf".
func embeddedURL(base, filename string, page int) string {
	archive, entry, _ := engine.EmbeddedArchive(filename)
	segments := strings.Split(entry, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	address := base + "/content/" + url.PathEscape(strings.TrimSuffix(archive, path.Ext(archive))) + "/" + strings.Join(segments, "/")
	if page > 0 && strings.EqualFold(path.Ext(entry), ".pdf") {
		address += fmt.Sprintf("#page=%d", page)
	}
	return address
}

// VideoURL opens the original archived media at a transcript passage.
func VideoURL(base, filename, articlePath string) (string, bool) {
	ms, ok := engine.VideoTime(articlePath)
	if !ok || !engine.IsEmbeddedSource(filename) {
		return "", false
	}
	return embeddedURL(base, filename, 0) + fmt.Sprintf("#t=%d.%03d", ms/1000, ms%1000), true
}

// documentServer serves books/ on 127.0.0.1 until it is closed.
type documentServer struct {
	base   string // http://127.0.0.1:PORT
	folder *workspace.Workspace
	stop   context.CancelFunc
	done   chan struct{}
}

func startDocumentServer(driveRoot string) (*documentServer, error) {
	dir := filepath.Join(driveRoot, engine.DocumentsDir)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("the drive has no %s folder", engine.DocumentsDir)
	}
	folder, err := workspace.OpenWith(dir, workspace.Options{Serves: engine.IsDocumentFile})
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		folder.Close()
		return nil, err
	}
	ctx, stop := context.WithCancel(context.Background())
	server := &documentServer{
		base:   fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port),
		folder: folder,
		stop:   stop,
		done:   make(chan struct{}),
	}
	go func() {
		defer close(server.done)
		_ = folder.Serve(ctx, listener)
	}()
	return server, nil
}

// pageURL is the address of a document, at a page when one is given.
func (d *documentServer) pageURL(filename string, page int) string {
	address := workspace.URL(d.base, strings.TrimPrefix(filename, engine.DocumentsDir+"/"))
	if page > 0 && strings.EqualFold(path.Ext(filename), ".pdf") {
		address += fmt.Sprintf("#page=%d", page)
	}
	return address
}

func (d *documentServer) Close() {
	d.stop()
	<-d.done
	d.folder.Close()
}
