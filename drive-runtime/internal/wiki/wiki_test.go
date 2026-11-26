package wiki

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedWritesTheStartPageIntoAnEmptyFolderOnly(t *testing.T) {
	dir := t.TempDir()
	// The host's litter does not make the folder a wiki.
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Seed(dir); err != nil {
		t.Fatalf("Seed() error = %v", err)
	}
	index, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatalf("start page not written: %v", err)
	}
	// What the page has to tell the user: the pages are files, agents can
	// write them, and where a deleted page went.
	for _, want := range []string{"plain Markdown file", "`wiki` folder", "AI clients", "can read, search and write", TrashPath} {
		if !strings.Contains(string(index), want) {
			t.Errorf("start page does not mention %q:\n%s", want, index)
		}
	}

	// A start page the user changed, or deleted while other pages exist, is
	// left as it is.
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "index.md")); string(data) != "mine\n" {
		t.Errorf("the user's start page was replaced: %q", data)
	}
	if err := os.Rename(filepath.Join(dir, "index.md"), filepath.Join(dir, "Notes.md")); err != nil {
		t.Fatal(err)
	}
	if err := Seed(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.md")); err == nil {
		t.Error("a start page was written into a wiki that has pages")
	}
}

func TestFoldersAreOnTheDrive(t *testing.T) {
	root := filepath.Join("/Volumes", "STICK")
	if got, want := Dir(root), "/Volumes/STICK/wiki"; got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
	// The trash is outside the page folder, so SilverBullet does not show
	// it, and on the drive, so a delete is a rename on one volume.
	if got, want := TrashDir(root), "/Volumes/STICK/.svalbard/state/wiki-trash"; got != want {
		t.Errorf("TrashDir() = %q, want %q", got, want)
	}
	if got := filepath.ToSlash(strings.TrimPrefix(TrashDir(root), root+"/")); got != TrashPath {
		t.Errorf("TrashPath = %q, but the trash is at %q", TrashPath, got)
	}
}

func TestPageURLEscapesEachPartOfTheName(t *testing.T) {
	for name, want := range map[string]string{
		"index":                "http://localhost:8088/index",
		"Projects/Garden plan": "http://localhost:8088/Projects/Garden%20plan",
		"Lääkkeet/Särky 100%":  "http://localhost:8088/L%C3%A4%C3%A4kkeet/S%C3%A4rky%20100%25",
	} {
		if got := PageURL("http://localhost:8088", name); got != want {
			t.Errorf("PageURL(%q) = %q, want %q", name, got, want)
		}
	}
}

// servePing answers /.ping the way the launcher's proxy does for the page
// folder it was started with, and returns its port.
func servePing(t *testing.T, folder string) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pingPath {
			http.NotFound(w, r)
			return
		}
		if folder != "" {
			w.Header().Set(folderHeader, folder)
		}
		w.Write([]byte("OK"))
	})}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return listener.Addr().(*net.TCPAddr).Port
}

func TestRunningURLFindsThisDrivesWikiAmongItsPorts(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	folder, err := filepath.EvalSymlinks(Dir(root))
	if err != nil {
		t.Fatal(err)
	}

	// Nothing is running.
	free := servePing(t, "")
	if got := runningURL(root, free+1, 1); got != "" {
		t.Errorf("runningURL() with nothing listening = %q, want none", got)
	}
	// A server that answers /.ping but is not a wiki, and another drive's
	// wiki, are both passed over.
	if got := runningURL(root, free, 1); got != "" {
		t.Errorf("runningURL() = %q for a server that names no folder", got)
	}
	other := servePing(t, filepath.Join(t.TempDir(), "wiki"))
	if got := runningURL(root, other, 1); got != "" {
		t.Errorf("runningURL() = %q for another drive's wiki", got)
	}

	port := servePing(t, folder)
	// Spelled with "localhost", which small models copy more reliably.
	if got, want := runningURL(root, port, 1), fmt.Sprintf("http://localhost:%d", port); got != want {
		t.Errorf("runningURL() = %q, want %q", got, want)
	}
	// The drive reached through another spelling of its path is the same wiki.
	link := filepath.Join(t.TempDir(), "mount")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if got := runningURL(link, port, 1); got == "" {
		t.Error("runningURL() did not find the wiki through a symlinked drive path")
	}
}

func TestRunningURLLooksAtEveryPortTheLauncherTries(t *testing.T) {
	// The launcher takes the first free port from PreferredPort upward, as
	// netutil.Listen does; the search must cover the same ports.
	if portSpan != 20 {
		t.Errorf("portSpan = %d, but netutil.Listen tries 20 ports", portSpan)
	}
}
