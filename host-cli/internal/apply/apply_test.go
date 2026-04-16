package apply

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// newTestCatalogWithURL creates a catalog containing a single recipe with the given
// id, type, and URL. It writes temporary YAML files and loads them via catalog.LoadFromFS.
func newTestCatalogWithURL(t *testing.T, id, typ, url string) *catalog.Catalog {
	t.Helper()

	recipeYAML := "id: " + id + "\ntype: " + typ + "\nstrategy: download\nurl: " + url + "\n"

	recipesFS := fstest.MapFS{
		id + ".yaml": &fstest.MapFile{Data: []byte(recipeYAML)},
	}
	presetsFS := fstest.MapFS{}

	cat, err := catalog.LoadFromFS(recipesFS, presetsFS)
	if err != nil {
		t.Fatalf("loading test catalog: %v", err)
	}
	return cat
}

func TestApplyDownloadsRealFiles(t *testing.T) {
	content := "fake zim content for testing"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"test-item"}

	cat := newTestCatalogWithURL(t, "test-item", "zim", ts.URL+"/test.zim")

	plan := planner.Plan{ToDownload: []string{"test-item"}}
	if err := Run(root, &m, plan, cat); err != nil {
		t.Fatal(err)
	}

	// Verify file downloaded to correct location.
	got, err := os.ReadFile(filepath.Join(root, "zim", "test.zim"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", string(got), content)
	}

	// Verify realized entry created.
	if len(m.Realized.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(m.Realized.Entries))
	}
	entry := m.Realized.Entries[0]
	if entry.ID != "test-item" {
		t.Errorf("id = %q, want %q", entry.ID, "test-item")
	}
	if entry.RelativePath != "zim/test.zim" {
		t.Errorf("path = %q, want %q", entry.RelativePath, "zim/test.zim")
	}
	if entry.Type != "zim" {
		t.Errorf("type = %q, want %q", entry.Type, "zim")
	}
	if entry.Filename != "test.zim" {
		t.Errorf("filename = %q, want %q", entry.Filename, "test.zim")
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("size = %d, want %d", entry.SizeBytes, len(content))
	}
	if entry.ChecksumSHA256 == "" {
		t.Error("checksum should be set")
	}
	if entry.SourceStrategy != "download" {
		t.Errorf("strategy = %q, want %q", entry.SourceStrategy, "download")
	}
	if m.Realized.AppliedAt == "" {
		t.Error("AppliedAt should be set")
	}
}

func TestApplyRemovesFiles(t *testing.T) {
	root := t.TempDir()
	// Create a file to be removed.
	os.MkdirAll(filepath.Join(root, "zim"), 0755)
	os.WriteFile(filepath.Join(root, "zim", "old.zim"), []byte("old"), 0644)

	m := manifest.New("test")
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "old-item", Type: "zim", Filename: "old.zim", RelativePath: "zim/old.zim"},
	}

	// Empty catalog (not needed for removals).
	recipesFS := fstest.MapFS{}
	presetsFS := fstest.MapFS{}
	cat, err := catalog.LoadFromFS(recipesFS, presetsFS)
	if err != nil {
		t.Fatal(err)
	}

	plan := planner.Plan{ToRemove: []string{"old-item"}}
	if err := Run(root, &m, plan, cat); err != nil {
		t.Fatal(err)
	}

	// File should be deleted.
	if _, err := os.Stat(filepath.Join(root, "zim", "old.zim")); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}

	// Entry should be removed.
	if len(m.Realized.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(m.Realized.Entries))
	}
}

func TestApplyRemoveMissingFileIsNotError(t *testing.T) {
	root := t.TempDir()

	m := manifest.New("test")
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "gone-item", Type: "zim", Filename: "gone.zim", RelativePath: "zim/gone.zim"},
	}

	recipesFS := fstest.MapFS{}
	presetsFS := fstest.MapFS{}
	cat, err := catalog.LoadFromFS(recipesFS, presetsFS)
	if err != nil {
		t.Fatal(err)
	}

	plan := planner.Plan{ToRemove: []string{"gone-item"}}
	if err := Run(root, &m, plan, cat); err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}

	if len(m.Realized.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(m.Realized.Entries))
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	content := "idempotent test content"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"wiki"}

	cat := newTestCatalogWithURL(t, "wiki", "zim", ts.URL+"/wiki.zim")

	// First apply: download wiki.
	plan1 := planner.Build(m)
	if len(plan1.ToDownload) != 1 {
		t.Fatalf("expected 1 download in first plan, got %d", len(plan1.ToDownload))
	}
	if err := Run(root, &m, plan1, cat); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}

	// Second plan should be empty since realized now matches desired.
	plan2 := planner.Build(m)
	if len(plan2.ToDownload) != 0 {
		t.Fatalf("expected 0 downloads in second plan, got %d", len(plan2.ToDownload))
	}
	if len(plan2.ToRemove) != 0 {
		t.Fatalf("expected 0 removals in second plan, got %d", len(plan2.ToRemove))
	}
}

func TestApplyUsesRecipeFilename(t *testing.T) {
	content := "custom filename test"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	recipeYAML := "id: custom\ntype: zim\nstrategy: download\nurl: " + ts.URL + "/some/path/download\nfilename: custom-name.zim\n"
	recipesFS := fstest.MapFS{
		"custom.yaml": &fstest.MapFile{Data: []byte(recipeYAML)},
	}
	presetsFS := fstest.MapFS{}
	cat, err := catalog.LoadFromFS(recipesFS, presetsFS)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"custom"}

	plan := planner.Plan{ToDownload: []string{"custom"}}
	if err := Run(root, &m, plan, cat); err != nil {
		t.Fatal(err)
	}

	// Should use the recipe's Filename, not the URL path segment.
	if m.Realized.Entries[0].Filename != "custom-name.zim" {
		t.Errorf("filename = %q, want %q", m.Realized.Entries[0].Filename, "custom-name.zim")
	}
	if _, err := os.Stat(filepath.Join(root, "zim", "custom-name.zim")); err != nil {
		t.Errorf("expected file at zim/custom-name.zim: %v", err)
	}
}
