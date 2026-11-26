package apply

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

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

	// The layout the maplibre-vendor app-bundle recipe actually installs.
	for _, rel := range []string{
		filepath.Join("apps", "maplibre-vendor", "vendor", "maplibre-gl.js"),
		filepath.Join("apps", "maplibre-vendor", "vendor", "maplibre-gl.css"),
		filepath.Join("apps", "maplibre-vendor", "vendor", "pmtiles.js"),
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
	if err := Run(context.Background(), root, &m, plan, cat); err != nil {
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

func TestPlatformIOArchivesUsePackageDestinations(t *testing.T) {
	useMockHTTPResponses(t, map[string]string{
		"/download/platformio/tool/tool-cmake/3.30.2/tool-cmake.tar.gz":       "cmake",
		"/download/platformio/tool/tool-cmake/3.30.2/tool-cmake-arm64.tar.gz": "cmake arm64",
	})
	root := t.TempDir()
	recipe := catalog.Item{ID: "tool-cmake", Type: "toolchain", URL: "https://dl.registry.platformio.org/download/platformio/tool/tool-cmake/3.30.2/tool-cmake.tar.gz"}
	shared, err := fetchAndRecord(context.Background(), root, recipe.ID, recipe, recipe.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := filepath.ToSlash(shared.RelativePath), "tools/platformio/packages/tool-cmake.tar.gz"; got != want {
		t.Fatalf("shared path = %q, want %q", got, want)
	}
	platformURL := "https://dl.registry.platformio.org/download/platformio/tool/tool-cmake/3.30.2/tool-cmake-arm64.tar.gz"
	platform, err := fetchAndRecordForPlatform(context.Background(), root, recipe.ID, recipe, platformURL, "macos-arm64", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := filepath.ToSlash(platform.RelativePath), "tools/platformio/packages/macos-arm64/tool-cmake-arm64.tar.gz"; got != want {
		t.Fatalf("target path = %q, want %q", got, want)
	}
}

func TestPlatformIOMissingTargetFails(t *testing.T) {
	recipe := catalog.Item{ID: "tool-cmake", Type: "toolchain", Platforms: map[string]string{"macos-arm64": "https://dl.registry.platformio.org/download/platformio/tool/tool-cmake/3.30.2/a.tar.gz"}}
	_, err := downloadPlatformItems(context.Background(), t.TempDir(), recipe.ID, recipe, []string{"linux-arm64"}, nil)
	if err == nil || !strings.Contains(err.Error(), "required target linux-arm64") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlatformIOAggregatorWaitsForPythonVenvAggregate(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml":   {Data: []byte("id: svalbard-python\ntype: python-venv\nstrategy: build\nbuild: {family: python-venv}\n")},
		"core.yaml":     {Data: []byte("id: platformio-core\ntype: python-package\nvenv: svalbard-python\n")},
		"idf.yaml":      {Data: []byte("id: espidf-python\ntype: python-package\nvenv: svalbard-python\n")},
		"platform.yaml": {Data: []byte("id: platformio-espressif32\ntype: toolchain\nurl: https://dl.registry.platformio.org/download/platformio/platform/espressif32/7.1.3/espressif32-7.1.3.tar.gz\ndeps: [platformio-core, espidf-python]\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	venv, _ := cat.RecipeByID("svalbard-python")
	platformRecipe, _ := cat.RecipeByID("platformio-espressif32")
	platformRecipe.Deps = schedulerDependencies(platformRecipe, cat)
	levels, err := jobLevels([]downloadJob{{id: venv.ID, recipe: venv}, {id: platformRecipe.ID, recipe: platformRecipe}}, []manifest.RealizedEntry{{ID: "uv"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 2 || levels[0][0].id != "svalbard-python" || levels[1][0].id != "platformio-espressif32" {
		t.Fatalf("levels = %#v", levels)
	}
}

func TestRunWithCheckpointSavesCompletedRecipeBeforeDoneEvent(t *testing.T) {
	content := "checkpointed content"
	useMockHTTPResponses(t, map[string]string{"/checkpoint.zim": content})
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"checkpoint"}
	cat := newTestCatalogWithURL(t, "checkpoint", "zim", "http://example.test/checkpoint.zim")

	var checkpointed bool
	var doneAfterCheckpoint bool
	err := RunWithCheckpoint(context.Background(), root, &m, planner.Plan{ToDownload: []string{"checkpoint"}}, cat, func(snapshot manifest.Manifest) error {
		checkpointed = len(snapshot.Realized.Entries) == 1 && snapshot.Realized.Entries[0].ID == "checkpoint"
		return nil
	}, func(event ProgressEvent) {
		if event.ID == "checkpoint" && event.Status == "done" {
			doneAfterCheckpoint = checkpointed
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !checkpointed {
		t.Fatal("checkpoint did not contain completed recipe")
	}
	if !doneAfterCheckpoint {
		t.Fatal("done event arrived before checkpoint")
	}
}

func TestRunWithCheckpointDoesNotRealizeRejectedSnapshot(t *testing.T) {
	useMockHTTPResponses(t, map[string]string{"/rejected.zim": "content"})
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"rejected"}
	cat := newTestCatalogWithURL(t, "rejected", "zim", "http://example.test/rejected.zim")
	var done bool
	err := RunWithCheckpoint(context.Background(), root, &m, planner.Plan{ToDownload: []string{"rejected"}}, cat, func(manifest.Manifest) error {
		return fmt.Errorf("disk full")
	}, func(event ProgressEvent) {
		done = done || event.Status == "done"
	})
	if err == nil {
		t.Fatal("RunWithCheckpoint succeeded after checkpoint failure")
	}
	if len(m.Realized.Entries) != 0 {
		t.Fatalf("rejected snapshot changed live manifest: %+v", m.Realized.Entries)
	}
	if done {
		t.Fatal("done event emitted for rejected snapshot")
	}
}

func TestRunReturnsIncompleteErrorForMissingRecipe(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	m := manifest.New("test")
	m.Desired.Items = []string{"missing"}
	err = Run(context.Background(), t.TempDir(), &m, planner.Plan{ToDownload: []string{"missing"}}, cat)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("Run error = %v, want missing recipe error", err)
	}
	if m.Realized.AppliedAt != "" {
		t.Errorf("AppliedAt = %q, want empty", m.Realized.AppliedAt)
	}
}

func TestRunReturnsErrorForRecipeWithoutAcquisitionStrategy(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"none.yaml": &fstest.MapFile{Data: []byte("id: none\ntype: zim\nstrategy: none\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	m := manifest.New("test")
	m.Desired.Items = []string{"none"}
	err = Run(context.Background(), t.TempDir(), &m, planner.Plan{ToDownload: []string{"none"}}, cat)
	if err == nil || !strings.Contains(err.Error(), "no acquisition strategy") {
		t.Fatalf("Run error = %v, want no acquisition strategy", err)
	}
	if m.Realized.AppliedAt != "" {
		t.Errorf("AppliedAt = %q, want empty", m.Realized.AppliedAt)
	}
}

func TestRunJobBatchDoesNotStartJobAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var requests int
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("request should not have started")
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
	cat := newTestCatalogWithURL(t, "cancelled", "zim", "http://example.test/cancelled.zim")
	recipe, ok := cat.RecipeByID("cancelled")
	if !ok {
		t.Fatal("missing test recipe")
	}
	for result := range runJobBatch(ctx, t.TempDir(), []downloadJob{{id: "cancelled", recipe: recipe}}, cat, nil, map[string]bool{"cancelled": true}, updateSwap{}, func(ProgressEvent) {}) {
		close(result.ack)
	}
	if requests != 0 {
		t.Fatalf("started %d HTTP requests after cancellation", requests)
	}
}

func TestMultiPlatformRecipeIsRealizedOnlyAfterEveryPlatformSucceeds(t *testing.T) {
	var linuxRequests, darwinRequests int
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/linux.bin":
			linuxRequests++
			return response(req, http.StatusOK, "linux"), nil
		case "/darwin.bin":
			darwinRequests++
			if darwinRequests == 1 {
				return nil, fmt.Errorf("second platform failed")
			}
			return response(req, http.StatusOK, "darwin"), nil
		default:
			return nil, fmt.Errorf("unexpected URL %s", req.URL)
		}
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"platform.yaml": &fstest.MapFile{Data: []byte("id: platform\ntype: binary\nstrategy: download\nplatforms:\n  linux-arm64: http://example.test/linux.bin\n  darwin-arm64: http://example.test/darwin.bin\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"platform"}
	m.Desired.Options.HostPlatforms = []string{"linux-arm64", "darwin-arm64"}
	plan := planner.Plan{ToDownload: []string{"platform"}}
	if err := Run(context.Background(), root, &m, plan, cat); err == nil {
		t.Fatal("first multi-platform apply unexpectedly succeeded")
	}
	if len(m.Realized.Entries) != 0 {
		t.Fatalf("partial platform recipe was realized: %+v", m.Realized.Entries)
	}
	if err := Run(context.Background(), root, &m, plan, cat); err != nil {
		t.Fatalf("second multi-platform apply: %v", err)
	}
	if linuxRequests != 1 || darwinRequests != 2 {
		t.Fatalf("requests linux=%d darwin=%d, want 1 and 2", linuxRequests, darwinRequests)
	}
	if len(m.Realized.Entries) != 2 || m.Realized.AppliedAt == "" {
		t.Fatalf("completed multi-platform recipe was not fully realized: %+v", m.Realized)
	}
}

func TestPlannedIDsCoveredByIncludesBuilderChildEntries(t *testing.T) {
	got := plannedIDsCoveredBy([]manifest.RealizedEntry{
		{ID: "python-venv"},
		{ID: "child-package"},
	}, []string{"python-venv", "child-package", "pending"})
	if !slices.Equal(got, []string{"python-venv", "child-package"}) {
		t.Fatalf("covered IDs = %v, want builder and child IDs", got)
	}
}

func response(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
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
	if err := Run(context.Background(), root, &m, plan, cat); err != nil {
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
	if err := Run(context.Background(), root, &m, plan, cat); err != nil {
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
	if err := Run(context.Background(), root, &m, plan1, cat); err != nil {
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
	if err := Run(context.Background(), root, &m, plan, cat); err != nil {
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
	useMockHTTPResponses(t, map[string]string{"/water.pmtiles": content})

	root := t.TempDir()
	writeVendorBundle(t, root)

	m := manifest.New("test")
	m.Desired.Items = []string{"water"}

	recipes := fstest.MapFS{"water.yaml": {Data: []byte(`id: water
type: pmtiles
strategy: download
url: http://example.test/water.pmtiles
viewer:
  name: Public water posts
  category: water
  style:
    points:
      circle-color: "#1f78b4"
      circle-radius: 5
license:
  attribution: Helsinki Region Environmental Services
  url: https://www.hsy.fi/avoindata
`)}}
	cat, err := catalog.LoadFromFS(recipes, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	plan := planner.Plan{ToDownload: []string{"water"}}
	if err := Run(context.Background(), root, &m, plan, cat); err != nil {
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
		"../maplibre-vendor/vendor/maplibre-gl.js",
		"../maplibre-vendor/vendor/maplibre-gl.css",
		"../maplibre-vendor/vendor/pmtiles.js",
		"maps/water.pmtiles",
		"Public water posts",
		`"Category":"water"`,
		`"points":{"circle-color":"#1f78b4","circle-radius":5}`,
		"Helsinki Region Environmental Services",
		"https://www.hsy.fi/avoindata",
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
	if err := Run(context.Background(), root, &m, plan, cat); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "apps", "map", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected map viewer to be removed, got err=%v", err)
	}
}

func TestCheckpointFailureCancelsStartedPeersAndPreventsQueuedAcquisition(t *testing.T) {
	var peers sync.WaitGroup
	peers.Add(3)
	var queuedRequested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a-fast.zim":
			peers.Wait()
			_, _ = w.Write([]byte("completed but not checkpointed"))
		case "/e-queued.zim":
			queuedRequested.Store(true)
			_, _ = w.Write([]byte("must not start"))
		default:
			peers.Done()
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	defer server.Close()
	ids := []string{"a-fast", "b-peer", "c-peer", "d-peer", "e-queued"}
	recipes := fstest.MapFS{}
	for _, id := range ids {
		recipes[id+".yaml"] = &fstest.MapFile{Data: []byte("id: " + id + "\ntype: zim\nurl: " + server.URL + "/" + id + ".zim\n")}
	}
	cat, err := catalog.LoadFromFS(recipes, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	m := manifest.New("failure")
	m.Desired.Items = ids
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	storageErr := errors.New("storage unavailable")
	err = RunWithCheckpoint(ctx, t.TempDir(), &m, planner.Build(m), cat, func(manifest.Manifest) error { return storageErr })
	if !errors.Is(err, storageErr) || ctx.Err() != nil {
		t.Fatalf("checkpoint failure did not stop/drain peers promptly: %v parent=%v", err, ctx.Err())
	}
	if queuedRequested.Load() {
		t.Fatal("queued acquisition started after rejected checkpoint")
	}
	if len(m.Realized.Entries) != 0 || m.Realized.AppliedAt != "" {
		t.Fatalf("rejected state realized: %+v", m.Realized)
	}
}

func TestApplyEnforcesPlatformSHA256(t *testing.T) {
	content := "pinned binary"
	sum := sha256.Sum256([]byte(content))
	good := hex.EncodeToString(sum[:])
	useMockHTTPResponses(t, map[string]string{"/tool-linux-amd64.tar.gz": content})

	for _, tc := range []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{"match", good, false},
		{"mismatch", strings.Repeat("0", 64), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recipeYAML := "id: tool\ntype: binary\nplatforms:\n  linux-x86_64: http://example.test/tool-linux-amd64.tar.gz\n" +
				"platform_sha256:\n  linux-x86_64: " + tc.hash + "\n"
			cat, err := catalog.LoadFromFS(fstest.MapFS{"tool.yaml": &fstest.MapFile{Data: []byte(recipeYAML)}}, fstest.MapFS{})
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			m := manifest.New("test")
			m.Desired.Items = []string{"tool"}
			m.Desired.Options.HostPlatforms = []string{"linux-x86_64"}

			err = Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{"tool"}}, cat)
			dest := filepath.Join(root, "bin", "linux-x86_64", "tool-linux-amd64.tar.gz")
			if tc.wantErr {
				if err == nil || len(m.Realized.Entries) != 0 {
					t.Fatalf("err = %v, realized = %d; want failure with nothing realized", err, len(m.Realized.Entries))
				}
				if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
					t.Errorf("mismatching download left on disk: %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := m.Realized.Entries[0].ChecksumSHA256; got != good {
				t.Errorf("checksum = %q, want %q", got, good)
			}
		})
	}
}

func TestPlatformBinariesAreKeptApartPerPlatform(t *testing.T) {
	useMockHTTPResponses(t, map[string]string{
		"/tool-darwin-arm64":          "mac build",
		"/tool-linux-x64":             "linux build",
		"/packed-darwin-arm64.tar.gz": "mac archive",
		"/packed-linux-x64.tar.gz":    "linux archive",
	})
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"tool.yaml":   &fstest.MapFile{Data: []byte("id: tool\ntype: binary\nstrategy: download\nplatforms:\n  macos-arm64: http://example.test/tool-darwin-arm64\n  linux-x86_64: http://example.test/tool-linux-x64\n")},
		"packed.yaml": &fstest.MapFile{Data: []byte("id: packed\ntype: binary\nstrategy: download\nplatforms:\n  macos-arm64: http://example.test/packed-darwin-arm64.tar.gz\n  linux-x86_64: http://example.test/packed-linux-x64.tar.gz\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"tool", "packed"}
	// "darwin-arm64" is what an unset host platform resolves to on a Mac.
	m.Desired.Options.HostPlatforms = []string{"darwin-arm64", "linux-x86_64"}
	if err := Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{"tool", "packed"}}, cat); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// A bare executable is stored under the recipe id, ready to run. An archive
	// keeps its upstream name for the drive runtime to unpack in place.
	for rel, want := range map[string]string{
		"bin/macos-arm64/tool":                       "mac build",
		"bin/linux-x86_64/tool":                      "linux build",
		"bin/macos-arm64/packed-darwin-arm64.tar.gz": "mac archive",
		"bin/linux-x86_64/packed-linux-x64.tar.gz":   "linux archive",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("platform binary not at %s: %v", rel, err)
		}
		if string(data) != want {
			t.Errorf("%s = %q, want %q", rel, data, want)
		}
		if info, _ := os.Stat(path); !strings.HasSuffix(rel, ".tar.gz") && info.Mode()&0o111 == 0 {
			t.Errorf("%s mode = %v, want executable", rel, info.Mode())
		}
	}
	// Nothing per-platform is left in flat bin/, where the runtime would unpack
	// every platform's archive into one directory.
	entries, err := os.ReadDir(filepath.Join(root, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			t.Errorf("per-platform file left in flat bin/: %s", entry.Name())
		}
	}
	var paths []string
	for _, e := range m.Realized.Entries {
		if e.ID == "tool" {
			paths = append(paths, filepath.ToSlash(e.RelativePath))
		}
	}
	slices.Sort(paths)
	if !slices.Equal(paths, []string{"bin/linux-x86_64/tool", "bin/macos-arm64/tool"}) {
		t.Errorf("realized paths = %v", paths)
	}
}

// skills-core through apply: the build family routes, the skills whose
// requirements are installed are written, a local edit is put back and
// reported, and removing the recipe removes its skills but not skills/.
func TestApplyWritesAndRemovesEmbeddedSkills(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"skills-core.yaml": {Data: []byte("id: skills-core\ntype: skills\nstrategy: build\nbuild: {family: embedded}\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"skills-core"}
	if err := Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{"skills-core"}}, cat); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(root, "skills", "skills-core", "vault-research", "SKILL.md")
	if _, err := os.Stat(skill); err != nil {
		t.Fatalf("vault-research (requires nothing) not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "skills-core", "spreadsheet-file")); !os.IsNotExist(err) {
		t.Fatalf("spreadsheet-file written without py-docs: %v", err)
	}
	if !realizedID(m.Realized.Entries, "skills-core") {
		t.Fatalf("skills-core not realized: %+v", m.Realized.Entries)
	}
	recipes, err := os.ReadFile(filepath.Join(root, ".svalbard", "recipes.json"))
	if err != nil || !strings.Contains(string(recipes), `"skills"`) {
		t.Fatalf("recipes.json does not list the skills recipe: %v %s", err, recipes)
	}

	if err := os.WriteFile(skill, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var notices []string
	if err := Run(context.Background(), root, &m, planner.Plan{}, cat, func(ev ProgressEvent) {
		if ev.Message != "" {
			notices = append(notices, ev.ID+": "+ev.Message)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(notices, []string{"svalbard: restored svalbard skill vault-research (local edits replaced)"}) {
		t.Fatalf("notices = %q", notices)
	}
	if got, _ := os.ReadFile(skill); string(got) == "mine\n" {
		t.Fatal("local edit kept")
	}

	m.Desired.Items = nil
	if err := Run(context.Background(), root, &m, planner.Plan{ToRemove: []string{"skills-core"}}, cat); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "skills-core")); !os.IsNotExist(err) {
		t.Fatalf("skills/skills-core kept: %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "skills")); err != nil || !info.IsDir() {
		t.Fatalf("skills/ removed: %v", err)
	}
	recipes, _ = os.ReadFile(filepath.Join(root, ".svalbard", "recipes.json"))
	if strings.Contains(string(recipes), `"skills"`) {
		t.Fatalf("recipes.json still lists a skills recipe: %s", recipes)
	}
}

func TestApplyRemovesPythonLauncherWithPackage(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: \"3.12\"\nstrategy: build\nbuild: {family: python-venv}\n")},
		"docs.yaml":   {Data: []byte("id: py-docs\ntype: python-package\nvenv: svalbard-python\npackages: [docpkg==1.0]\nentry_points: [docs-cli]\npython_launcher: py-docs\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := manifest.New("test")
	for _, platform := range []string{"macos-arm64", "linux-arm64"} {
		tree := filepath.Join("runtime", "python", platform, "tools", "py-docs")
		if err := os.MkdirAll(filepath.Join(root, tree), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"py-docs", "docs-cli", "other-tool"} {
			path := filepath.Join(root, "bin", platform, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		m.Realized.Entries = append(m.Realized.Entries, manifest.RealizedEntry{ID: "py-docs", Type: "python-package", Filename: "py-docs", RelativePath: tree})
	}
	if err := Run(context.Background(), root, &m, planner.Plan{ToRemove: []string{"py-docs"}}, cat); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"macos-arm64", "linux-arm64"} {
		for _, name := range []string{"py-docs", "docs-cli"} {
			if _, err := os.Stat(filepath.Join(root, "bin", platform, name)); !os.IsNotExist(err) {
				t.Errorf("bin/%s/%s kept after its package was removed: %v", platform, name, err)
			}
		}
		if _, err := os.Stat(filepath.Join(root, "bin", platform, "other-tool")); err != nil {
			t.Errorf("another tool's command removed: %v", err)
		}
	}
}

func TestPythonRepairJobsRebuildAVenvOfAnOlderLayout(t *testing.T) {
	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"python.yaml": {Data: []byte("id: svalbard-python\ntype: python-venv\npython: \"3.12\"\nstrategy: build\nbuild: {family: python-venv}\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"svalbard-python"}
	m.Desired.Options.HostPlatforms = []string{"linux-arm64"}
	m.Realized.Entries = []manifest.RealizedEntry{{ID: "svalbard-python", Type: "python-venv", RelativePath: "runtime/python"}}
	desired := map[string]bool{"svalbard-python": true}

	if jobs := pythonRepairJobs(root, &m, cat, desired, map[string]bool{}); len(jobs) != 0 {
		t.Fatalf("a vault without Python trees queued %v", jobs)
	}
	// An interpreter from before layout stamps.
	if err := os.MkdirAll(filepath.Join(root, "runtime", "python", "linux-arm64", "python", "cpython-3.12.16-linux-aarch64-gnu"), 0o755); err != nil {
		t.Fatal(err)
	}
	jobs := pythonRepairJobs(root, &m, cat, desired, map[string]bool{})
	if len(jobs) != 1 || jobs[0].id != "svalbard-python" {
		t.Fatalf("jobs = %v, want a rebuild of svalbard-python", jobs)
	}
	if jobs := pythonRepairJobs(root, &m, cat, desired, map[string]bool{"svalbard-python": true}); len(jobs) != 0 {
		t.Errorf("an already queued venv queued again: %v", jobs)
	}
	if jobs := pythonRepairJobs(root, &m, cat, map[string]bool{}, map[string]bool{}); len(jobs) != 0 {
		t.Errorf("a venv being removed queued: %v", jobs)
	}
}
