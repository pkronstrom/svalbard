package commands

import (
	"bytes"
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
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

func fakePDF(text string) string { return "%PDF-1.7\n" + text }

// collectionVault is a vault whose catalog is one user collection file that
// the test rewrites between applies, as a user edits it.
type collectionVault struct {
	t       *testing.T
	root    string
	recipes string
}

func newCollectionVault(t *testing.T, id string) *collectionVault {
	t.Helper()
	v := &collectionVault{t: t, root: t.TempDir(), recipes: t.TempDir()}
	m := manifest.New("test")
	m.Desired.Items = []string{id}
	v.save(m)
	return v
}

func (v *collectionVault) define(id, documents string) {
	v.t.Helper()
	if err := os.WriteFile(filepath.Join(v.recipes, id+".yaml"), []byte("description: test collection\ndocuments:\n"+documents), 0o644); err != nil {
		v.t.Fatal(err)
	}
}

func (v *collectionVault) catalog() *catalog.Catalog {
	v.t.Helper()
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		v.t.Fatal(err)
	}
	if skipped := cat.LoadUserRecipes(v.recipes); len(skipped) > 0 {
		v.t.Fatalf("collection skipped: %v", skipped)
	}
	// As the CLI does: what this computer does not define comes from the
	// copy the vault keeps.
	cat.LoadVaultRecipes(catalog.VaultRecipeDir(v.root))
	return cat
}

func (v *collectionVault) recipeCopy(id string) string {
	data, err := os.ReadFile(filepath.Join(catalog.VaultRecipeDir(v.root), id+".yaml"))
	if err != nil {
		return ""
	}
	return string(data)
}

func (v *collectionVault) apply() error {
	v.t.Helper()
	return ApplyVault(context.Background(), v.root, v.catalog())
}

func (v *collectionVault) manifest() manifest.Manifest {
	v.t.Helper()
	m, err := manifest.Load(filepath.Join(v.root, "manifest.yaml"))
	if err != nil {
		v.t.Fatal(err)
	}
	return m
}

func (v *collectionVault) save(m manifest.Manifest) {
	v.t.Helper()
	if err := manifest.Save(filepath.Join(v.root, "manifest.yaml"), m); err != nil {
		v.t.Fatal(err)
	}
}

// realize records a document as if an earlier apply had installed it.
func (v *collectionVault) realize(id, rel, body string) {
	v.t.Helper()
	path := filepath.Join(v.root, "books", id, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		v.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		v.t.Fatal(err)
	}
	m := v.manifest()
	m.Realized.Entries = append(m.Realized.Entries, manifest.RealizedEntry{
		ID: id, Type: "pdf", Filename: filepath.Base(path), RelativePath: filepath.Join("books", id, filepath.FromSlash(rel)),
		SizeBytes: int64(len(body)), ChecksumSHA256: checksum([]byte(body)), SourceStrategy: "download",
	})
	v.save(m)
}

func (v *collectionVault) plan() string {
	v.t.Helper()
	plan, documents := BuildPlan(v.manifest(), v.catalog())
	var out bytes.Buffer
	if err := WritePlan(&out, plan); err != nil {
		v.t.Fatal(err)
	}
	WriteDocumentPlans(&out, documents)
	return out.String()
}

// files lists every file below books/, sorted.
func (v *collectionVault) files() []string {
	v.t.Helper()
	var files []string
	_ = filepath.WalkDir(filepath.Join(v.root, "books"), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(v.root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func (v *collectionVault) realized() []string {
	v.t.Helper()
	var paths []string
	for _, entry := range v.manifest().Realized.Entries {
		paths = append(paths, filepath.ToSlash(entry.RelativePath))
	}
	sort.Strings(paths)
	return paths
}

// TestCollectionLifecycle follows a collection through the user's path: add
// and apply, edit the file, plan, apply the difference, then remove it.
func TestCollectionLifecycle(t *testing.T) {
	var mu sync.Mutex
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mu.Lock()
			requests[r.URL.Path]++
			mu.Unlock()
		}
		_, _ = w.Write([]byte(fakePDF(r.URL.Path)))
	}))
	defer server.Close()
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "workshop", "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "workshop", "engine", "timing.pdf"), []byte(fakePDF("timing")), 0o644); err != nil {
		t.Fatal(err)
	}

	v := newCollectionVault(t, "manuals")
	a := "  - title: Alpha Manual\n    url: " + server.URL + "/a.pdf\n"
	b := "  - url: " + server.URL + "/b.pdf\n"
	c := "  - url: " + server.URL + "/c.pdf\n"
	folder := "  - path: " + filepath.Join(src, "workshop") + "\n"
	v.define("manuals", a+b+folder)
	if got, want := v.plan(), "download: 1\n  + manuals\nremove: 0\nunmanaged: 0\nnew manuals: 3 documents\n"; got != want {
		t.Fatalf("first plan = %q, want %q", got, want)
	}
	if err := v.apply(); err != nil {
		t.Fatal(err)
	}
	want := []string{"books/manuals/a.pdf", "books/manuals/b.pdf", "books/manuals/workshop/engine/timing.pdf"}
	if got := v.files(); !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	if got := v.realized(); !reflect.DeepEqual(got, want) {
		t.Fatalf("realized = %v, want %v", got, want)
	}
	meta, err := toolkit.ReadDocumentsMetadata(v.root)
	if err != nil {
		t.Fatal(err)
	}
	if got := meta["books/manuals/a.pdf"]; got.Title != "Alpha Manual" || got.Collection != "manuals" || got.URL != server.URL+"/a.pdf" {
		t.Errorf("documents.json entry = %+v", got)
	}
	if got, want := v.plan(), "download: 0\nremove: 0\nunmanaged: 0\n"; got != want {
		t.Fatalf("plan after apply = %q, want nothing to do", got)
	}
	definition, _ := os.ReadFile(filepath.Join(v.recipes, "manuals.yaml"))
	if got := v.recipeCopy("manuals"); got != string(definition) {
		t.Fatalf("the vault's recipe copy = %q, want the definition", got)
	}

	// Edit: one document added, one dropped.
	mu.Lock()
	requests = map[string]int{}
	mu.Unlock()
	v.define("manuals", a+c+folder)
	if got, want := v.plan(), "download: 1\n  + manuals\nremove: 0\nunmanaged: 0\nupdate manuals: +1 document, -1 document\n"; got != want {
		t.Fatalf("plan after edit = %q, want %q", got, want)
	}
	if err := v.apply(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := requests
	mu.Unlock()
	if !reflect.DeepEqual(got, map[string]int{"/c.pdf": 1}) {
		t.Fatalf("requests after edit = %v, want one GET of /c.pdf", got)
	}
	want = []string{"books/manuals/a.pdf", "books/manuals/c.pdf", "books/manuals/workshop/engine/timing.pdf"}
	if got := v.files(); !reflect.DeepEqual(got, want) {
		t.Fatalf("files after edit = %v, want %v", got, want)
	}
	meta, _ = toolkit.ReadDocumentsMetadata(v.root)
	if _, stale := meta["books/manuals/b.pdf"]; stale || len(meta) != 3 {
		t.Errorf("documents.json after edit = %v", meta)
	}
	if got := v.recipeCopy("manuals"); !strings.Contains(got, "/c.pdf") || strings.Contains(got, "/b.pdf") {
		t.Fatalf("the vault's recipe copy did not follow the edit: %q", got)
	}

	// Remove, on a computer that does not even have the definition.
	m := v.manifest()
	if err := RemoveItems(&m, []string{"manuals"}); err != nil {
		t.Fatal(err)
	}
	v.save(m)
	if err := os.Remove(filepath.Join(v.recipes, "manuals.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := v.apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v.root, "books", "manuals")); !os.IsNotExist(err) {
		t.Fatalf("books/manuals still exists after removal: %v", err)
	}
	if entries := v.manifest().Realized.Entries; len(entries) != 0 {
		t.Fatalf("entries after removal = %+v", entries)
	}
	if _, err := os.Stat(filepath.Join(v.root, ".svalbard", "documents.json")); !os.IsNotExist(err) {
		t.Errorf("documents.json left behind: %v", err)
	}
	if got := v.recipeCopy("manuals"); got != "" {
		t.Errorf("the vault's recipe copy left behind: %q", got)
	}
}

func TestApplyVaultKeepsArrivedDocumentsOfFailedCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/good.pdf" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(fakePDF("good")))
	}))
	defer server.Close()
	v := newCollectionVault(t, "manuals")
	v.define("manuals", "  - url: "+server.URL+"/good.pdf\n  - url: "+server.URL+"/gone.pdf\n")
	err := v.apply()
	if err == nil || !strings.Contains(err.Error(), server.URL+"/gone.pdf") || !strings.Contains(err.Error(), "HTTP status 404") {
		t.Fatalf("err = %v, want the failing URL and its status", err)
	}
	if got := v.realized(); !reflect.DeepEqual(got, []string{"books/manuals/good.pdf"}) {
		t.Fatalf("saved entries = %v, want the document that arrived", got)
	}
	if got, want := v.plan(), "download: 1\n  + manuals\nremove: 0\nunmanaged: 0\nupdate manuals: +1 document\n"; got != want {
		t.Fatalf("plan after failure = %q, want %q", got, want)
	}
}

func TestBuildPlanDetectsCollectionDrift(t *testing.T) {
	src := t.TempDir()
	local := filepath.Join(src, "trailer.pdf")
	if err := os.WriteFile(local, []byte(fakePDF("trailer")), 0o644); err != nil {
		t.Fatal(err)
	}
	pinned := "  - url: https://example.test/a.pdf\n    sha256: " + checksum([]byte(fakePDF("a"))) + "\n"
	definition := pinned + "  - path: " + local + "\n"
	v := newCollectionVault(t, "manuals")
	v.define("manuals", definition)
	v.realize("manuals", "a.pdf", fakePDF("a"))
	v.realize("manuals", "trailer.pdf", fakePDF("trailer"))
	nothing := "download: 0\nremove: 0\nunmanaged: 0\n"
	update := "download: 1\n  + manuals\nremove: 0\nunmanaged: 0\nupdate manuals: "

	if got := v.plan(); got != nothing {
		t.Fatalf("plan without drift = %q", got)
	}
	// The document list changed.
	v.define("manuals", definition+"  - url: https://example.test/new.pdf\n")
	if got, want := v.plan(), update+"+1 document\n"; got != want {
		t.Fatalf("plan with an added document = %q, want %q", got, want)
	}
	v.define("manuals", pinned)
	if got, want := v.plan(), update+"-1 document\n"; got != want {
		t.Fatalf("plan with a dropped document = %q, want %q", got, want)
	}
	// A pin no longer matches the recorded checksum.
	v.define("manuals", strings.Replace(definition, checksum([]byte(fakePDF("a"))), checksum([]byte("other")), 1))
	if got, want := v.plan(), update+"1 document changed\n"; got != want {
		t.Fatalf("plan with a changed pin = %q, want %q", got, want)
	}
	// A local source changed size.
	v.define("manuals", definition)
	if err := os.WriteFile(local, []byte(fakePDF("trailer, revised")), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := v.plan(), update+"1 document changed\n"; got != want {
		t.Fatalf("plan with a changed local file = %q, want %q", got, want)
	}
	// A local source that cannot be read is no reason to do anything: its
	// installed document is kept and said so.
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if got, want := v.plan(), nothing+"manuals: kept trailer.pdf: cannot read "+local+" (no such file or directory)\n"; got != want {
		t.Fatalf("plan with an unreadable source = %q, want %q", got, want)
	}
	// One the vault holds nothing from is a document that is missing.
	v.define("manuals", definition+"  - path: "+filepath.Join(src, "never-there")+"\n")
	want := "download: 1\n  + manuals\nremove: 0\nunmanaged: 0\n" +
		"manuals: kept trailer.pdf: cannot read " + local + " (no such file or directory)\n" +
		"manuals: cannot read " + filepath.Join(src, "never-there") + " (no such file or directory)\n"
	if got := v.plan(); got != want {
		t.Fatalf("plan with a source that was never installed = %q, want %q", got, want)
	}
}

// On a computer without the definition the vault's copy of the recipe is
// used: URL documents are planned as usual, and documents that came from the
// other computer's disk are kept and named one by one.
func TestBuildPlanUsesTheVaultCopyOnAnotherComputer(t *testing.T) {
	v := newCollectionVault(t, "manuals")
	copyDir := catalog.VaultRecipeDir(v.root)
	if err := os.MkdirAll(copyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	definition := "documents:\n  - url: https://example.test/a.pdf\n  - url: https://example.test/new.pdf\n" +
		"  - title: Trailer\n    path: ~/Documents/manuals/trailer.pdf\n  - path: ~/Documents/manuals/workshop/\n"
	if err := os.WriteFile(filepath.Join(copyDir, "manuals.yaml"), []byte(definition), 0o644); err != nil {
		t.Fatal(err)
	}
	v.realize("manuals", "a.pdf", fakePDF("a"))
	v.realize("manuals", "trailer.pdf", fakePDF("trailer"))
	v.realize("manuals", "workshop/engine/timing.pdf", fakePDF("timing"))
	v.realize("manuals", "dropped.pdf", fakePDF("dropped"))

	want := "download: 1\n  + manuals\nremove: 0\nunmanaged: 0\n" +
		"update manuals: +1 document, -1 document\n" +
		"manuals: kept trailer.pdf: cannot read ~/Documents/manuals/trailer.pdf (defined on another computer)\n" +
		"manuals: kept workshop/engine/timing.pdf: cannot read ~/Documents/manuals/workshop/ (defined on another computer)\n"
	if got := v.plan(); got != want {
		t.Fatalf("plan from the vault's copy = %q, want %q", got, want)
	}
	// The definition on this computer wins as soon as there is one.
	v.define("manuals", "  - url: https://example.test/a.pdf\n")
	if got, want := v.plan(), "download: 1\n  + manuals\nremove: 0\nunmanaged: 0\nupdate manuals: -3 documents\n"; got != want {
		t.Fatalf("plan with a local definition = %q, want %q", got, want)
	}
}

func TestBuildPlanPausesCollectionWithoutDefinition(t *testing.T) {
	v := newCollectionVault(t, "manuals")
	v.realize("manuals", "a.pdf", fakePDF("a"))
	if got, want := v.plan(), "download: 0\nremove: 0\nunmanaged: 0\nmanuals: definition not found here or on the vault; updates paused\n"; got != want {
		t.Fatalf("plan without the definition = %q, want %q", got, want)
	}
}

func TestExpandItemRefsExplainsSkippedUserRecipe(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manuals.yaml"), []byte("description: no documents\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat.LoadUserRecipes(dir)
	if _, err := ExpandItemRefs(cat, []string{"manuals"}); err == nil || !strings.Contains(err.Error(), "missing documents") {
		t.Fatalf("err = %v, want the skip reason", err)
	}
}
