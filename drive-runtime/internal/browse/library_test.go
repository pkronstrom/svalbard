package browse

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	switch filepath.Base(os.Args[0]) {
	case "kiwix-manage":
		if err := fakeManage(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	case "kiwix-serve":
		fmt.Fprintln(os.Stderr, fakeServe(os.Args[1:]))
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestLibraryAddsAndRemovesArchivesWithoutRestarting(t *testing.T) {
	root := fixtureLibraryDrive(t, true)
	library := NewLibrary(root)
	t.Cleanup(library.Close)
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	first := mustAcquire(t, library, ctx, "first.zim")
	duplicate := mustAcquire(t, library, ctx, "first.zim")
	second := mustAcquire(t, library, ctx, "second.zim")
	if first.BaseURL != second.BaseURL || first.Exited != second.Exited {
		t.Fatalf("archive switch restarted Kiwix: first=%q second=%q", first.BaseURL, second.BaseURL)
	}
	if duplicate.ViewerURL != first.ViewerURL || second.ViewerURL != first.BaseURL+"/viewer#second" || second.ContentName != "second" {
		t.Fatalf("unexpected viewer links: first=%q duplicate=%q second=%q", first.ViewerURL, duplicate.ViewerURL, second.ViewerURL)
	}

	first.Close()
	first.Close()
	requireContentStatus(t, duplicate, http.StatusOK)
	requireContentStatus(t, second, http.StatusOK)
	duplicate.Close()
	requireContentStatus(t, first, http.StatusNotFound)
	requireContentStatus(t, second, http.StatusOK)

	second.Close()
	requireContentStatus(t, second, http.StatusNotFound)
	reopened := mustAcquire(t, library, ctx, "first.zim")
	if reopened.BaseURL != first.BaseURL || reopened.Exited != first.Exited {
		t.Fatal("an empty library did not keep its listener")
	}

	library.Close()
	select {
	case <-first.Exited:
	default:
		t.Fatal("Close returned while Kiwix was still running")
	}
	reopened.Close()
}

func TestLibrarySharesDuplicateArchiveIDs(t *testing.T) {
	root := fixtureLibraryDrive(t, true)
	library := NewLibrary(root)
	t.Cleanup(library.Close)
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	first := mustAcquire(t, library, ctx, "first.zim")
	alias := mustAcquire(t, library, ctx, "alias.zim")
	if first.ViewerURL != alias.ViewerURL || first.ContentName != alias.ContentName {
		t.Fatalf("duplicate IDs should use the same content URL: %s and %s", first.ViewerURL, alias.ViewerURL)
	}
	first.Close()
	requireContentStatus(t, alias, http.StatusOK)
	alias.Close()
	requireContentStatus(t, alias, http.StatusNotFound)
}

func TestClosingLibraryCancelsPreparingArchive(t *testing.T) {
	root := fixtureLibraryDrive(t, true)
	path := filepath.Join(root, "zim", "slow.zim")
	if err := os.WriteFile(path, []byte("slow-id"), 0o644); err != nil {
		t.Fatal(err)
	}
	library := NewLibrary(root)
	t.Cleanup(library.Close)
	ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	result := make(chan error, 1)
	go func() {
		_, err := library.Acquire(ctx, "slow.zim")
		result <- err
	}()

	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(path + ".started"); err == nil {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("Acquire completed before preparing its metadata: %v", err)
		case <-ctx.Done():
			t.Fatal("metadata process did not start")
		case <-tick.C:
		}
	}

	library.Close()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire error = %v, want context.Canceled", err)
	}
}

func TestLibraryWithoutCompanionReportsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	library := NewLibrary(fixtureLibraryDrive(t, false))
	t.Cleanup(library.Close)
	if _, err := library.Acquire(t.Context(), "first.zim"); !errors.Is(err, ErrLibraryUnavailable) {
		t.Fatalf("Acquire: got %v, want ErrLibraryUnavailable", err)
	}
}

func TestLibraryRejectsNonVaultFilenames(t *testing.T) {
	library := NewLibrary(fixtureLibraryDrive(t, true))
	t.Cleanup(library.Close)
	for _, target := range []string{"", "../first.zim", "foo/first.zim", `foo\first.zim`, "._first.zim", "first.txt"} {
		if _, err := library.Acquire(t.Context(), target); err == nil {
			t.Errorf("Acquire accepted %q", target)
		}
	}
}

func fixtureLibraryDrive(t *testing.T, companion bool) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"bin", "zim"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, id := range map[string]string{"first.zim": "first-id", "second.zim": "second-id", "alias.zim": "first-id"} {
		if err := os.WriteFile(filepath.Join(root, "zim", name), []byte(id), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	symlink := func(name string) {
		if err := os.Symlink(self, filepath.Join(root, "bin", name)); err != nil {
			t.Fatal(err)
		}
	}
	symlink("kiwix-serve")
	if companion {
		symlink("kiwix-manage")
	}
	return root
}

func mustAcquire(t *testing.T, library *Library, ctx context.Context, target string) *Archive {
	t.Helper()
	archive, err := library.Acquire(ctx, target)
	if err != nil {
		t.Fatalf("opening %s: %v", target, err)
	}
	return archive
}

func requireContentStatus(t *testing.T, archive *Archive, want int) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	address := archive.BaseURL + "/content/" + url.PathEscape(archive.ContentName)
	response, err := client.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != want {
		t.Errorf("GET %s: got %d, want %d", address, response.StatusCode, want)
	}
}

type fakeKiwixBook struct {
	ID   string `xml:"id,attr"`
	Path string `xml:"path,attr"`
}

type fakeKiwixLibrary struct {
	XMLName xml.Name        `xml:"library"`
	Version string          `xml:"version,attr"`
	Books   []fakeKiwixBook `xml:"book"`
}

func fakeManage(args []string) error {
	if len(args) != 3 || args[1] != "add" {
		return fmt.Errorf("expected LIBRARY add ZIM, got %v", args)
	}
	id, err := os.ReadFile(args[2])
	if err != nil {
		return err
	}
	if string(id) == "slow-id" {
		if err := os.WriteFile(args[2]+".started", nil, 0o600); err != nil {
			return err
		}
		time.Sleep(time.Minute)
	}
	rel, err := filepath.Rel(filepath.Dir(args[0]), args[2])
	if err != nil {
		return err
	}
	doc := fakeKiwixLibrary{
		Version: "20110515",
		Books:   []fakeKiwixBook{{ID: string(id), Path: rel}},
	}
	data, err := xml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(args[0], data, 0o600)
}

func fakeServe(args []string) error {
	options := map[string]string{}
	for i := 0; i+1 < len(args); i++ {
		if strings.HasPrefix(args[i], "--") && !strings.HasPrefix(args[i+1], "--") {
			options[args[i]] = args[i+1]
		}
	}
	if options["--address"] != "127.0.0.1" || options["--port"] == "" || options["--library"] == "" {
		return fmt.Errorf("missing expected options: %v", args)
	}

	var mu sync.Mutex
	var current fakeKiwixLibrary
	refresh := func() error {
		mu.Lock()
		defer mu.Unlock()
		data, err := os.ReadFile(options["--library"])
		if err != nil {
			return err
		}
		current = fakeKiwixLibrary{}
		return xml.Unmarshal(data, &current)
	}
	if err := refresh(); err != nil {
		return err
	}
	reloads := make(chan os.Signal, 1)
	signal.Notify(reloads, syscall.SIGHUP)
	go func() {
		for range reloads {
			if err := refresh(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}()
	contentName := func(path string) string {
		return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /catalog/v2/entries", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprint(w, "<feed>")
		for _, book := range current.Books {
			fmt.Fprintf(w, `<entry><id>urn:uuid:%s</id><link type="text/html" href="/content/%s"/></entry>`, book.ID, url.PathEscape(contentName(book.Path)))
		}
		fmt.Fprint(w, "</feed>")
	})
	mux.HandleFunc("GET /content/{book}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		for _, book := range current.Books {
			if contentName(book.Path) == r.PathValue("book") {
				fmt.Fprint(w, "Archive content")
				return
			}
		}
		http.NotFound(w, r)
	})
	return http.ListenAndServe(options["--address"]+":"+options["--port"], mux)
}
