package apply

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func useMockHTTPResponses(t *testing.T, responses map[string]string) {
	t.Helper()

	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if body, ok := responses[req.URL.Path]; ok {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     fmt.Sprintf("%d %s", http.StatusNotFound, http.StatusText(http.StatusNotFound)),
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = orig
	})
}

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

func writeVendorBundle(t *testing.T, root string) {
	t.Helper()

	for _, rel := range []string{
		filepath.Join("vendor", "maplibre-gl.js"),
		filepath.Join("vendor", "maplibre-gl.css"),
		filepath.Join("vendor", "pmtiles.js"),
	} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte("test asset"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func TestApplyDownloadsRealFiles(t *testing.T) {
	content := "fake zim content for testing"
	useMockHTTPResponses(t, map[string]string{"/test.zim": content})

	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"test-item"}

	cat := newTestCatalogWithURL(t, "test-item", "zim", "http://example.test/test.zim")

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
	useMockHTTPResponses(t, map[string]string{"/wiki.zim": content})

	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"wiki"}

	cat := newTestCatalogWithURL(t, "wiki", "zim", "http://example.test/wiki.zim")

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
	useMockHTTPResponses(t, map[string]string{"/some/path/download": content})

	recipeYAML := "id: custom\ntype: zim\nstrategy: download\nurl: http://example.test/some/path/download\nfilename: custom-name.zim\n"
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

func TestApplyGeneratesOfflineMapViewerForPMTiles(t *testing.T) {
	content := "pmtiles test content"
	useMockHTTPResponses(t, map[string]string{"/osm.pmtiles": content})

	root := t.TempDir()
	writeVendorBundle(t, root)

	m := manifest.New("test")
	m.Desired.Items = []string{"osm"}

	cat := newTestCatalogWithURL(t, "osm", "pmtiles", "http://example.test/osm.pmtiles")
	plan := planner.Plan{ToDownload: []string{"osm"}}
	if err := Run(root, &m, plan, cat); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "apps", "map", "index.html")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading map viewer: %v", err)
	}
	html := string(raw)

	if strings.Contains(html, "unpkg.com") {
		t.Fatal("map viewer should not reference CDN assets")
	}
	for _, want := range []string{
		"../../vendor/maplibre-gl.js",
		"../../vendor/maplibre-gl.css",
		"../../vendor/pmtiles.js",
		"osm.pmtiles",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("map viewer missing %q", want)
		}
	}
}

func TestApplyRemovesMapViewerWhenNoPMTilesRemain(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps", "map"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "map", "index.html"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := manifest.New("test")
	m.Desired.Items = []string{"doc"}
	plan := planner.Plan{}
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(root, &m, plan, cat); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "apps", "map", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected map viewer to be removed, got err=%v", err)
	}
}
