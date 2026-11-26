package apply

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// documentServer serves bodies by path and counts the GET requests per path.
type documentServer struct {
	*httptest.Server
	mu       sync.Mutex
	bodies   map[string]string
	requests map[string]int
}

func newDocumentServer(t *testing.T, bodies map[string]string) *documentServer {
	t.Helper()
	s := &documentServer{bodies: bodies, requests: map[string]int{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		body, ok := s.bodies[r.URL.Path]
		if r.Method == http.MethodGet {
			s.requests[r.URL.Path]++
		}
		s.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *documentServer) serve(path, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bodies[path] = body
}

// takeRequests returns the GET counts since the last call.
func (s *documentServer) takeRequests() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.requests
	s.requests = map[string]int{}
	return out
}

func fakePDF(text string) string { return "%PDF-1.7\n" + text }

// collectionCatalog loads one user collection whose documents: list is body.
func collectionCatalog(t *testing.T, id, body string) *catalog.Catalog {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, id+".yaml"), []byte("description: test collection\ndocuments:\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if skipped := cat.LoadUserRecipes(dir); len(skipped) > 0 {
		t.Fatalf("collection skipped: %v", skipped)
	}
	return cat
}

// assemble runs the collection job as apply does, against realized.
func assemble(t *testing.T, root, body string, realized []manifest.RealizedEntry) ([]manifest.RealizedEntry, error) {
	t.Helper()
	recipe, _ := collectionCatalog(t, "manuals", body).RecipeByID("manuals")
	return assembleDocuments(context.Background(), root, "manuals", recipe, realized, nil)
}

func entryPaths(entries []manifest.RealizedEntry) []string {
	var paths []string
	for _, entry := range entries {
		paths = append(paths, filepath.ToSlash(entry.RelativePath))
	}
	sort.Strings(paths)
	return paths
}

// vaultFiles lists every file below dir in the vault, sorted.
func vaultFiles(t *testing.T, root, dir string) []string {
	t.Helper()
	var files []string
	_ = filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleDocumentsFetchesOnlyTheDifference(t *testing.T) {
	server := newDocumentServer(t, map[string]string{
		"/a.pdf": fakePDF("alpha"), "/b.pdf": fakePDF("bravo"), "/c.pdf": fakePDF("charlie"),
	})
	root := t.TempDir()
	a, b, c := "  - url: "+server.URL+"/a.pdf\n", "  - url: "+server.URL+"/b.pdf\n", "  - url: "+server.URL+"/c.pdf\n"

	entries, err := assemble(t, root, a+b, nil)
	if err != nil {
		t.Fatal(err)
	}
	both := []string{"books/manuals/a.pdf", "books/manuals/b.pdf"}
	if got := entryPaths(entries); !reflect.DeepEqual(got, both) {
		t.Fatalf("entries = %v, want %v", got, both)
	}
	if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, both) {
		t.Fatalf("files = %v, want %v", got, both)
	}
	first := entries[0]
	if first.ID != "manuals" || first.Type != "pdf" || first.Filename != "a.pdf" || first.SourceStrategy != "download" ||
		first.ChecksumSHA256 != sha(fakePDF("alpha")) || first.SizeBytes != int64(len(fakePDF("alpha"))) {
		t.Errorf("entry = %+v", first)
	}
	if _, err := os.Stat(filepath.Join(root, ".staging", "documents", "manuals")); !os.IsNotExist(err) {
		t.Errorf("staging left behind: %v", err)
	}
	server.takeRequests()

	// Nothing changed: nothing is requested.
	if entries, err = assemble(t, root, a+b, entries); err != nil {
		t.Fatal(err)
	}
	if got := server.takeRequests(); len(got) != 0 {
		t.Fatalf("unchanged collection made requests: %v", got)
	}

	// A third document costs exactly one request.
	if entries, err = assemble(t, root, a+b+c, entries); err != nil {
		t.Fatal(err)
	}
	if got := server.takeRequests(); !reflect.DeepEqual(got, map[string]int{"/c.pdf": 1}) {
		t.Fatalf("requests after adding one document = %v, want one GET of /c.pdf", got)
	}

	// A dropped document is deleted without a request.
	if entries, err = assemble(t, root, a+c, entries); err != nil {
		t.Fatal(err)
	}
	want := []string{"books/manuals/a.pdf", "books/manuals/c.pdf"}
	if got := entryPaths(entries); !reflect.DeepEqual(got, want) {
		t.Fatalf("entries after dropping b = %v, want %v", got, want)
	}
	if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, want) {
		t.Fatalf("files after dropping b = %v, want %v", got, want)
	}
	if got := server.takeRequests(); len(got) != 0 {
		t.Fatalf("dropping a document made requests: %v", got)
	}

	// A realized file that vanished from the vault is fetched again.
	if err := os.Remove(filepath.Join(root, "books", "manuals", "c.pdf")); err != nil {
		t.Fatal(err)
	}
	if _, err = assemble(t, root, a+c, entries); err != nil {
		t.Fatal(err)
	}
	if got := server.takeRequests(); !reflect.DeepEqual(got, map[string]int{"/c.pdf": 1}) {
		t.Fatalf("requests after losing a file = %v, want one GET of /c.pdf", got)
	}
}

func TestAssembleDocumentsKeepsArrivedDocumentsWhenOthersFail(t *testing.T) {
	server := newDocumentServer(t, map[string]string{
		"/good.pdf":  fakePDF("good"),
		"/login.pdf": "<!DOCTYPE html><html><body>Please sign in</body></html>",
	})
	root := t.TempDir()
	body := "  - url: " + server.URL + "/good.pdf\n  - url: " + server.URL + "/login.pdf\n  - url: " + server.URL + "/gone.pdf\n"
	entries, err := assemble(t, root, body, nil)
	if err == nil {
		t.Fatal("collection succeeded although two documents failed")
	}
	for _, want := range []string{server.URL + "/login.pdf", "web page", server.URL + "/gone.pdf", "HTTP status 404"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), server.URL+"/good.pdf") {
		t.Errorf("error %q blames the document that arrived", err)
	}
	good := []string{"books/manuals/good.pdf"}
	if got := entryPaths(entries); !reflect.DeepEqual(got, good) {
		t.Fatalf("entries = %v, want the document that arrived", got)
	}
	if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, good) {
		t.Fatalf("files = %v, want only the document that arrived", got)
	}

	// The retry asks only for what is missing.
	server.takeRequests()
	server.serve("/login.pdf", fakePDF("now a real file"))
	server.serve("/gone.pdf", fakePDF("back"))
	entries, err = assemble(t, root, body, entries)
	if err != nil {
		t.Fatal(err)
	}
	if got := server.takeRequests(); !reflect.DeepEqual(got, map[string]int{"/login.pdf": 1, "/gone.pdf": 1}) {
		t.Fatalf("retry requests = %v, want only the two missing documents", got)
	}
	if len(entries) != 3 {
		t.Fatalf("entries after retry = %v", entryPaths(entries))
	}
}

func TestAssembleDocumentsMirrorsLocalFolders(t *testing.T) {
	src, root := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "trailer.pdf"), fakePDF("trailer"))
	writeFile(t, filepath.Join(src, "workshop", "brakes.pdf"), fakePDF("brakes"))
	writeFile(t, filepath.Join(src, "workshop", "engine", "b20", "timing.pdf"), fakePDF("timing"))
	writeFile(t, filepath.Join(src, "workshop", "engine", "notes.txt"), "not a document")
	body := "  - path: " + filepath.Join(src, "trailer.pdf") + "\n  - path: " + filepath.Join(src, "workshop") + "\n"

	entries, err := assemble(t, root, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"books/manuals/trailer.pdf", "books/manuals/workshop/brakes.pdf", "books/manuals/workshop/engine/b20/timing.pdf"}
	if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	if got := entryPaths(entries); !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for _, entry := range entries {
		if entry.SourceStrategy != "local" || entry.ChecksumSHA256 == "" {
			t.Errorf("local entry = %+v", entry)
		}
	}

	// A local file whose size changed is copied again; a file removed from
	// the folder is deleted with the subfolders it leaves empty.
	writeFile(t, filepath.Join(src, "trailer.pdf"), fakePDF("trailer, second edition"))
	if err := os.Remove(filepath.Join(src, "workshop", "engine", "b20", "timing.pdf")); err != nil {
		t.Fatal(err)
	}
	recipe, _ := collectionCatalog(t, "manuals", body).RecipeByID("manuals")
	docs, err := catalog.ExpectedDocuments(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if diff := DiffDocuments(recipe, docs, entries); !reflect.DeepEqual(diff, DocumentDiff{Changed: 1, Removed: 1}) {
		t.Fatalf("diff after local edits = %+v", diff)
	}
	if entries, err = assemble(t, root, body, entries); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "books", "manuals", "trailer.pdf")); string(got) != fakePDF("trailer, second edition") {
		t.Fatalf("trailer.pdf = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "books", "manuals", "workshop", "engine")); !os.IsNotExist(err) {
		t.Errorf("emptied subfolder left behind: %v", err)
	}
	if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, want[:2]) {
		t.Fatalf("files after local edits = %v, want %v", got, want[:2])
	}
}

// A local source that is missing, unplugged or empty on this computer says
// nothing about the documents it held. Their installed files must survive it.
func TestAssembleDocumentsKeepsFilesOfUnreadableLocalSources(t *testing.T) {
	server := newDocumentServer(t, map[string]string{"/a.pdf": fakePDF("alpha"), "/b.pdf": fakePDF("bravo")})
	src, root := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "trailer.pdf"), fakePDF("trailer"))
	writeFile(t, filepath.Join(src, "workshop", "brakes.pdf"), fakePDF("brakes"))
	writeFile(t, filepath.Join(src, "workshop", "engine", "timing.pdf"), fakePDF("timing"))
	url := func(name string) string { return "  - url: " + server.URL + "/" + name + "\n" }
	local := "  - path: " + filepath.Join(src, "trailer.pdf") + "\n  - path: " + filepath.Join(src, "workshop") + "\n"

	entries, err := assemble(t, root, url("a.pdf")+local, nil)
	if err != nil {
		t.Fatal(err)
	}
	installed := []string{"books/manuals/a.pdf", "books/manuals/trailer.pdf", "books/manuals/workshop/brakes.pdf", "books/manuals/workshop/engine/timing.pdf"}
	if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, installed) {
		t.Fatalf("files = %v, want %v", got, installed)
	}

	check := func(step string, entries []manifest.RealizedEntry, err error, want []string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: files = %v, want %v", step, got, want)
		}
		if got := entryPaths(entries); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: entries = %v, want %v", step, got, want)
		}
	}

	// The folder is there but empty, as a cloud folder is before it syncs.
	if err := os.Rename(filepath.Join(src, "workshop"), filepath.Join(src, "elsewhere")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "workshop"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err = assemble(t, root, url("a.pdf")+local, entries)
	check("empty folder", entries, err, installed)

	// The folder and the single file are gone, and the rest of the list
	// changes: the change is applied, the unreadable sources are left alone.
	if err := os.Remove(filepath.Join(src, "workshop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(src, "trailer.pdf")); err != nil {
		t.Fatal(err)
	}
	recipe, _ := collectionCatalog(t, "manuals", url("b.pdf")+local).RecipeByID("manuals")
	set, err := catalog.ExpectedDocuments(recipe)
	if err != nil {
		t.Fatal(err)
	}
	diff := DiffDocuments(recipe, set, entries)
	if diff.Added != 1 || diff.Removed != 1 || diff.Changed != 0 || len(diff.Missing) != 0 {
		t.Fatalf("diff = %+v, want one added, one removed and nothing missing", diff)
	}
	var kept []string
	for _, doc := range diff.Kept {
		kept = append(kept, doc.RelPath+" <- "+doc.Source.Path)
	}
	wantKept := []string{
		"trailer.pdf <- " + filepath.Join(src, "trailer.pdf"),
		"workshop/brakes.pdf <- " + filepath.Join(src, "workshop"),
		"workshop/engine/timing.pdf <- " + filepath.Join(src, "workshop"),
	}
	if !reflect.DeepEqual(kept, wantKept) {
		t.Fatalf("kept = %v, want %v", kept, wantKept)
	}
	entries, err = assemble(t, root, url("b.pdf")+local, entries)
	check("missing sources", entries, err, []string{"books/manuals/b.pdf", "books/manuals/trailer.pdf", "books/manuals/workshop/brakes.pdf", "books/manuals/workshop/engine/timing.pdf"})

	// Taking the entry out of the list is how its documents are removed.
	entries, err = assemble(t, root, url("b.pdf")+"  - path: "+filepath.Join(src, "trailer.pdf")+"\n", entries)
	check("folder entry dropped", entries, err, []string{"books/manuals/b.pdf", "books/manuals/trailer.pdf"})

	// A source the vault holds nothing from cannot be installed from here:
	// that fails the item like a dead URL, and still deletes nothing.
	body := url("b.pdf") + "  - path: " + filepath.Join(src, "trailer.pdf") + "\n  - path: " + filepath.Join(src, "never-there") + "\n"
	recipe, _ = collectionCatalog(t, "manuals", body).RecipeByID("manuals")
	set, _ = catalog.ExpectedDocuments(recipe)
	if diff := DiffDocuments(recipe, set, entries); !diff.Any() || len(diff.Missing) != 1 {
		t.Fatalf("diff with a source that was never installed = %+v", diff)
	}
	kept2, err := assemble(t, root, body, entries)
	if err == nil || !strings.Contains(err.Error(), filepath.Join(src, "never-there")+": cannot read") {
		t.Fatalf("err = %v, want the unreadable source named", err)
	}
	if got := entryPaths(kept2); !reflect.DeepEqual(got, entryPaths(entries)) {
		t.Fatalf("entries after the failure = %v, want them unchanged", got)
	}
}

func TestAssembleDocumentsRefetchesWhenPinChanges(t *testing.T) {
	first, second := fakePDF("first edition"), fakePDF("second edition")
	server := newDocumentServer(t, map[string]string{"/manual.pdf": first})
	root := t.TempDir()
	pinned := func(body string) string {
		return "  - url: " + server.URL + "/manual.pdf\n    sha256: " + sha(body) + "\n"
	}
	entries, err := assemble(t, root, pinned(first), nil)
	if err != nil {
		t.Fatal(err)
	}

	// A pin the server does not satisfy fails, and the previous file and its
	// record stay so the next plan still sees the difference.
	kept, err := assemble(t, root, pinned(second), entries)
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("err = %v, want sha256 mismatch", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "books", "manuals", "manual.pdf")); string(got) != first {
		t.Fatalf("previous document was not kept: %q", got)
	}
	if len(kept) != 1 || kept[0].ChecksumSHA256 != sha(first) {
		t.Fatalf("entries after failed refetch = %+v", kept)
	}

	server.serve("/manual.pdf", second)
	entries, err = assemble(t, root, pinned(second), kept)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "books", "manuals", "manual.pdf")); string(got) != second {
		t.Fatalf("document after refetch = %q", got)
	}
	if len(entries) != 1 || entries[0].ChecksumSHA256 != sha(second) {
		t.Fatalf("entries after refetch = %+v", entries)
	}
}

func TestAssembleDocumentsStopsOnCancellation(t *testing.T) {
	server := newDocumentServer(t, map[string]string{"/a.pdf": fakePDF("alpha")})
	root := t.TempDir()
	recipe, _ := collectionCatalog(t, "manuals", "  - url: "+server.URL+"/a.pdf\n").RecipeByID("manuals")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	entries, err := assembleDocuments(ctx, root, "manuals", recipe, nil, nil)
	if err == nil || len(entries) != 0 {
		t.Fatalf("entries = %v, err = %v; want a cancelled job with nothing realized", entries, err)
	}
	if got := server.takeRequests(); len(got) != 0 {
		t.Fatalf("cancelled job made requests: %v", got)
	}
}

func TestRunRecordsArrivedDocumentsOfAFailedCollection(t *testing.T) {
	server := newDocumentServer(t, map[string]string{"/good.pdf": fakePDF("good")})
	root := t.TempDir()
	cat := collectionCatalog(t, "manuals", "  - title: Good Manual\n    url: "+server.URL+"/good.pdf\n  - url: "+server.URL+"/gone.pdf\n")
	m := manifest.New("test")
	m.Desired.Items = []string{"manuals"}

	var saved []manifest.Manifest
	var statuses []string
	err := RunWithCheckpoint(context.Background(), root, &m, planner.Plan{ToDownload: []string{"manuals"}}, cat,
		func(snapshot manifest.Manifest) error {
			saved = append(saved, snapshot)
			return nil
		},
		func(ev ProgressEvent) { statuses = append(statuses, ev.Status) })
	if err == nil || !strings.Contains(err.Error(), "/gone.pdf") || !strings.Contains(err.Error(), "HTTP status 404") {
		t.Fatalf("err = %v, want the failing URL and its status", err)
	}
	if got := entryPaths(m.Realized.Entries); !reflect.DeepEqual(got, []string{"books/manuals/good.pdf"}) {
		t.Fatalf("realized = %v, want the document that arrived", got)
	}
	if len(saved) != 1 || len(saved[0].Realized.Entries) != 1 {
		t.Fatalf("checkpoints = %+v, want the arrived document saved", saved)
	}
	if m.Realized.AppliedAt != "" {
		t.Errorf("AppliedAt = %q, want empty for an incomplete apply", m.Realized.AppliedAt)
	}
	if last := statuses[len(statuses)-1]; last != "failed" {
		t.Errorf("last status = %q, want the item reported as failed", last)
	}
	// Indexing by hand after the failed apply still knows the title.
	meta, err := toolkit.ReadDocumentsMetadata(root)
	if err != nil || meta["books/manuals/good.pdf"].Title != "Good Manual" {
		t.Errorf("documents.json after the failed apply = %+v, %v", meta, err)
	}
}
