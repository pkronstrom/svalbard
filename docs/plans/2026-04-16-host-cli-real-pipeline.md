# Host CLI Real Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the placeholder apply pipeline with real HTTP downloading, URL resolution, proper toolkit generation, search indexing, sync/status commands, and map viewer generation — making `svalbard` a fully functional Go CLI without Python.

**Architecture:** Extend `host-cli/` with new internal packages: `downloader` (HTTP with resume/SHA256), `resolver` (URL pattern expansion), real `toolkit` generation (full actions.json matching drive-runtime's RuntimeConfig), `searchdb` (SQLite FTS5), `zimext` (ZIM article extraction via go-libzim CGO), and `mapview` (MapLibre HTML generation). The `apply` pipeline wires these together. `sync` orchestrates the full download+build+index flow. `status` reads vault state.

**Tech Stack:** Go 1.25, net/http (downloads), modernc.org/sqlite (pure Go SQLite for FTS5), github.com/nickstenning/kiwix-go or CGO libzim (ZIM extraction), regexp (URL patterns), encoding/json (toolkit), html/template (map viewer).

**Phases:**
- Phase A: Real catalog (embed repo recipes/presets, not test fixtures)
- Phase B: URL resolver + HTTP downloader
- Phase C: Real toolkit generation (full actions.json)
- Phase D: Wire real apply pipeline
- Phase E: sync + status commands
- Phase F: Search indexing (SQLite FTS5 + ZIM extraction)
- Phase G: Map viewer generation

---

## Phase A: Real Catalog Embedding

### Task A.1: Embed actual repo recipes and presets

**Files:**
- Modify: `host-cli/internal/catalog/embed.go`
- Modify: `host-cli/internal/catalog/catalog.go`
- Create: `host-cli/internal/catalog/catalog_integration_test.go`

The current catalog uses test fixtures with 2 recipes and 2 presets. We need to load from the real `recipes/` and `presets/` directories at the repo root. The Python code resolves these at runtime via filesystem paths. For Go, we have two options:

1. `go:embed` the YAML files into the binary at build time
2. Load from filesystem at runtime (using repo-root relative paths)

Choice: **Runtime filesystem loading** via `NewDefaultCatalog()` which already exists but uses `runtime.Caller` path resolution. This is correct for development. For distribution, the binary will need the recipes/presets alongside it or embedded — but that's a packaging concern, not a code concern now.

**Step 1: Write integration test**

```go
// host-cli/internal/catalog/catalog_integration_test.go
func TestDefaultCatalogLoadsRealRecipes(t *testing.T) {
    c := NewDefaultCatalog()
    names := c.PresetNames()
    if len(names) == 0 {
        t.Fatal("no presets loaded from repo root")
    }
    // Should have real presets like default-32
    found := false
    for _, n := range names {
        if n == "default-32" {
            found = true
        }
    }
    if !found {
        t.Fatal("missing default-32 preset from repo root")
    }
}
```

**Step 2: Verify the test passes with existing code** (it should — NewDefaultCatalog already loads from repo root)

Run: `cd host-cli && go test ./internal/catalog/ -run TestDefaultCatalog -v`

**Step 3: Extend catalog Item type to match real recipe fields**

The real recipes have additional fields beyond what the test fixtures use. Add to `catalog.go`:

```go
type Item struct {
    ID          string            `yaml:"id"`
    Type        string            `yaml:"type"`
    Description string            `yaml:"description"`
    SizeGB      float64           `yaml:"size_gb"`
    Strategy    string            `yaml:"strategy"`
    URL         string            `yaml:"url"`
    URLPattern  string            `yaml:"url_pattern"`
    Filename    string            `yaml:"filename"`
    Platforms   []PlatformVariant `yaml:"platforms,omitempty"`
    Build       *BuildConfig      `yaml:"build,omitempty"`
    Viewer      *ViewerConfig     `yaml:"viewer,omitempty"`
    License     *LicenseInfo      `yaml:"license,omitempty"`
    Tags        []string          `yaml:"tags,omitempty"`
    Menu        *MenuConfig       `yaml:"menu,omitempty"`
}

type PlatformVariant struct {
    Platform string  `yaml:"platform"`
    URL      string  `yaml:"url"`
    SizeGB   float64 `yaml:"size_gb"`
}

type BuildConfig struct {
    Family string `yaml:"family"`
    // Additional build params vary by family
}

type ViewerConfig struct {
    Name     string `yaml:"name"`
    Category string `yaml:"category"`
}

type LicenseInfo struct {
    ID          string `yaml:"id"`
    Attribution string `yaml:"attribution"`
}

type MenuConfig struct {
    Group       string `yaml:"group"`
    Label       string `yaml:"label"`
    Description string `yaml:"description"`
}
```

**Step 4: Add test for real recipe field loading**

```go
func TestDefaultCatalogRecipeHasRealFields(t *testing.T) {
    c := NewDefaultCatalog()
    item, ok := c.RecipeByID("wikipedia-en-nopic")
    if !ok {
        t.Fatal("missing wikipedia-en-nopic recipe")
    }
    if item.Type != "zim" {
        t.Errorf("type = %q, want zim", item.Type)
    }
    if item.SizeGB <= 0 {
        t.Error("SizeGB should be positive")
    }
}
```

**Step 5: Run tests, commit**

Run: `cd host-cli && go test ./internal/catalog/ -v`
Commit: `feat(host-cli): extend catalog to load real recipe fields`

---

## Phase B: URL Resolver + HTTP Downloader

### Task B.1: URL resolver

**Files:**
- Create: `host-cli/internal/resolver/resolver.go`
- Create: `host-cli/internal/resolver/resolver_test.go`

Resolves `{date}` patterns in recipe URLs by fetching directory listings and finding the latest match.

**Step 1: Write failing tests**

```go
func TestResolveStaticURL(t *testing.T) {
    // Static URL (no pattern) returns as-is
    url, err := Resolve("https://example.com/file.zim", "")
    if err != nil {
        t.Fatal(err)
    }
    if url != "https://example.com/file.zim" {
        t.Errorf("got %q", url)
    }
}

func TestResolvePatternURL(t *testing.T) {
    // Uses a test HTTP server that returns a directory listing
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, `<a href="file_2026-03.zim">file_2026-03.zim</a>
<a href="file_2026-01.zim">file_2026-01.zim</a>`)
    }))
    defer ts.Close()

    url, err := Resolve("", ts.URL+"/file_{date}.zim")
    if err != nil {
        t.Fatal(err)
    }
    if !strings.HasSuffix(url, "file_2026-03.zim") {
        t.Errorf("should resolve to latest date, got %q", url)
    }
}
```

**Step 2: Implement**

```go
func Resolve(staticURL, urlPattern string) (string, error)
```

- If staticURL is set: return it
- Split urlPattern at last "/" into baseURL + filenamePattern
- Replace `{date}` with `(\d{4}-\d{2})` regex
- HTTP GET baseURL, find all href matches, sort by date, return baseURL + latest

**Step 3: Run tests, commit**

Commit: `feat(host-cli): add URL pattern resolver`

---

### Task B.2: HTTP downloader with resume and SHA256

**Files:**
- Create: `host-cli/internal/downloader/downloader.go`
- Create: `host-cli/internal/downloader/downloader_test.go`

**Step 1: Write failing tests**

```go
func TestDownloadNewFile(t *testing.T) {
    content := "hello world content"
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(content))
    }))
    defer ts.Close()

    dest := filepath.Join(t.TempDir(), "file.txt")
    result, err := Download(ts.URL+"/file.txt", dest, "")
    if err != nil {
        t.Fatal(err)
    }
    if result.SHA256 == "" {
        t.Error("missing sha256")
    }
    got, _ := os.ReadFile(dest)
    if string(got) != content {
        t.Errorf("content = %q", got)
    }
}

func TestDownloadResumesPartialFile(t *testing.T) {
    content := "hello world content"
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rangeHeader := r.Header.Get("Range")
        if rangeHeader != "" {
            // Parse "bytes=N-"
            var start int
            fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
            w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(content)-1, len(content)))
            w.WriteHeader(206)
            w.Write([]byte(content[start:]))
            return
        }
        w.Write([]byte(content))
    }))
    defer ts.Close()

    dest := filepath.Join(t.TempDir(), "file.txt")
    // Write partial file
    os.WriteFile(dest, []byte("hello"), 0644)

    result, err := Download(ts.URL+"/file.txt", dest, "")
    if err != nil {
        t.Fatal(err)
    }
    got, _ := os.ReadFile(dest)
    if string(got) != content {
        t.Errorf("content = %q", got)
    }
    _ = result
}

func TestDownloadVerifiesSHA256(t *testing.T) {
    content := "test content"
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(content))
    }))
    defer ts.Close()

    dest := filepath.Join(t.TempDir(), "file.txt")
    _, err := Download(ts.URL+"/file.txt", dest, "wrong-sha256")
    if err == nil {
        t.Fatal("expected SHA256 mismatch error")
    }
}

func TestDownloadSkipsWhenSHA256Matches(t *testing.T) {
    content := "test content"
    // Pre-compute sha256
    h := sha256.Sum256([]byte(content))
    expected := hex.EncodeToString(h[:])

    dest := filepath.Join(t.TempDir(), "file.txt")
    os.WriteFile(dest, []byte(content), 0644)

    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Fatal("should not have made HTTP request — file already cached")
    }))
    defer ts.Close()

    result, err := Download(ts.URL+"/file.txt", dest, expected)
    if err != nil {
        t.Fatal(err)
    }
    if !result.Cached {
        t.Error("expected Cached=true")
    }
}
```

**Step 2: Implement**

```go
type Result struct {
    Path   string
    SHA256 string
    Cached bool
}

type ProgressFunc func(downloaded, total int64)

func Download(url, destPath, expectedSHA256 string) (Result, error)
func ComputeSHA256(path string) (string, error)
```

Download logic:
1. If file exists and expectedSHA256 matches → return Cached
2. If file exists → resume with Range header
3. Stream response body to file in 64KB chunks
4. Compute SHA256 of completed file
5. If expectedSHA256 set and doesn't match → return error

**Step 3: Run tests, commit**

Commit: `feat(host-cli): add HTTP downloader with resume and SHA256 verification`

---

## Phase C: Real Toolkit Generation

### Task C.1: Full actions.json generation

**Files:**
- Modify: `host-cli/internal/toolkit/toolkit.go`
- Modify: `host-cli/internal/toolkit/toolkit_test.go`

Replace the stub toolkit with real actions.json generation matching the drive-runtime RuntimeConfig format. Scans manifest entries and builds appropriate menu groups.

**Step 1: Write failing tests**

```go
func TestGenerateCreatesCorrectGroupsForZimEntries(t *testing.T) {
    root := t.TempDir()
    entries := []manifest.RealizedEntry{
        {ID: "wikipedia-en-nopic", Type: "zim", Filename: "wikipedia-en-nopic.zim", RelativePath: "zim/wikipedia-en-nopic.zim"},
        {ID: "ifixit", Type: "zim", Filename: "ifixit.zim", RelativePath: "zim/ifixit.zim"},
    }
    if err := Generate(root, entries, "default-32"); err != nil {
        t.Fatal(err)
    }

    raw, err := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
    if err != nil {
        t.Fatal(err)
    }

    var cfg struct {
        Version int `json:"version"`
        Groups  []struct {
            ID    string `json:"id"`
            Items []struct {
                ID string `json:"id"`
            } `json:"items"`
        } `json:"groups"`
    }
    if err := json.Unmarshal(raw, &cfg); err != nil {
        t.Fatal(err)
    }

    if cfg.Version != 2 {
        t.Errorf("version = %d", cfg.Version)
    }

    // Should have a library/browse group with 2 items
    found := false
    for _, g := range cfg.Groups {
        if g.ID == "library" && len(g.Items) == 2 {
            found = true
        }
    }
    if !found {
        t.Error("missing library group with 2 items")
    }
}

func TestGenerateIncludesToolsGroup(t *testing.T) {
    root := t.TempDir()
    entries := []manifest.RealizedEntry{
        {ID: "wikipedia", Type: "zim", Filename: "wikipedia.zim", RelativePath: "zim/wikipedia.zim"},
    }
    if err := Generate(root, entries, "default-32"); err != nil {
        t.Fatal(err)
    }

    raw, _ := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
    if !strings.Contains(string(raw), "tools") {
        t.Error("missing tools group")
    }
}
```

**Step 2: Implement real Generate**

Signature change: `Generate(root string, entries []manifest.RealizedEntry, presetName string) error`

Build groups by scanning entries:
- **library** group: per ZIM entry → builtin browse action
- **maps** group: if PMTiles entries → builtin maps action
- **local-ai** group: if GGUF entries (excluding embedding models) → builtin chat actions
- **apps** group: if app entries → builtin apps actions
- **tools** group: always present with inspect, verify, share, activate-shell

**Step 3: Update apply.go to pass entries and preset to Generate**

**Step 4: Run tests, commit**

Commit: `feat(host-cli): generate real actions.json for drive-runtime`

---

## Phase D: Wire Real Apply Pipeline

### Task D.1: Replace placeholder apply with downloader

**Files:**
- Modify: `host-cli/internal/apply/apply.go`
- Modify: `host-cli/internal/apply/apply_test.go`

Replace the placeholder file writes with actual HTTP downloads using resolver + downloader. The apply pipeline becomes:

1. For each ToDownload: resolve URL → download to correct type directory → record realized entry
2. For each ToRemove: delete artifact files → remove realized entry
3. Regenerate toolkit

**Step 1: Write failing test with test HTTP server**

```go
func TestApplyDownloadsRealFiles(t *testing.T) {
    content := "fake zim content"
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(content))
    }))
    defer ts.Close()

    root := t.TempDir()
    catalog := // test catalog where wikipedia-en-nopic has URL = ts.URL + "/wiki.zim"
    m := manifest.New("test")
    m.Desired.Items = []string{"wikipedia-en-nopic"}
    plan := planner.Plan{ToDownload: []string{"wikipedia-en-nopic"}}

    if err := Run(root, &m, plan, catalog); err != nil {
        t.Fatal(err)
    }

    // Verify file was actually downloaded
    got, err := os.ReadFile(filepath.Join(root, "zim", "wikipedia-en-nopic.zim"))
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != content {
        t.Errorf("content = %q", got)
    }
}
```

**Step 2: Implement — Run now takes catalog parameter**

```go
func Run(root string, m *manifest.Manifest, plan planner.Plan, cat *catalog.Catalog) error
```

Per download item:
1. Look up recipe in catalog
2. Resolve URL (static or pattern)
3. Determine destination directory from type (TYPE_DIRS map)
4. Download to dest
5. Record RealizedEntry with actual size, SHA256, relative path

TYPE_DIRS map:
```go
var TypeDirs = map[string]string{
    "zim": "zim", "pmtiles": "maps", "pdf": "books", "epub": "books",
    "gguf": "models", "binary": "bin", "app": "apps",
}
```

**Step 3: Update commands/apply.go to pass catalog**

**Step 4: Run tests, commit**

Commit: `feat(host-cli): wire real download pipeline into apply`

---

## Phase E: Sync + Status Commands

### Task E.1: Status command

**Files:**
- Create: `host-cli/internal/commands/status.go`
- Create: `host-cli/internal/commands/status_test.go`
- Modify: `host-cli/internal/cli/root.go`

**Step 1: Write failing test**

```go
func TestWriteStatusShowsEntries(t *testing.T) {
    m := manifest.New("test")
    m.Desired.Items = []string{"wikipedia-en-nopic", "ifixit"}
    m.Realized.Entries = []manifest.RealizedEntry{
        {ID: "wikipedia-en-nopic", Type: "zim", SizeBytes: 4500000000},
    }

    var buf bytes.Buffer
    WriteStatus(&buf, m)
    out := buf.String()

    if !strings.Contains(out, "wikipedia-en-nopic") {
        t.Error("missing realized entry")
    }
    if !strings.Contains(out, "ifixit") {
        t.Error("missing pending entry")
    }
}
```

**Step 2: Implement**

```go
func WriteStatus(w io.Writer, m manifest.Manifest) error
```

Prints: vault name, desired count, realized count, pending count, per-entry status (realized with size, or pending).

**Step 3: Wire `status` command into root CLI**

Note: `status` is a new command not in the original 7. Add it to root.go.

**Step 4: Run tests, commit**

Commit: `feat(host-cli): add status command`

---

### Task E.2: Sync command

**Files:**
- Create: `host-cli/internal/commands/sync.go`
- Create: `host-cli/internal/commands/sync_test.go`
- Modify: `host-cli/internal/cli/root.go`

Sync is essentially plan + apply in one step, with progress output.

**Step 1: Write failing test**

```go
func TestSyncVaultRunsPlanAndApply(t *testing.T) {
    // Setup: init a vault, then sync it
    root := t.TempDir()
    // Create manifest with desired items
    // Sync should produce realized entries
}
```

**Step 2: Implement**

```go
func SyncVault(root string, cat *catalog.Catalog, w io.Writer) error
```

1. Load manifest
2. Build plan
3. Print plan summary to w
4. If nothing to do, return
5. Run apply
6. Save manifest
7. Print completion summary

**Step 3: Wire sync command**

**Step 4: Run tests, commit**

Commit: `feat(host-cli): add sync command`

---

## Phase F: Search Indexing

### Task F.1: SQLite FTS5 search database

**Files:**
- Create: `host-cli/internal/searchdb/db.go`
- Create: `host-cli/internal/searchdb/db_test.go`

Pure Go SQLite via `modernc.org/sqlite`. Creates FTS5 tables for cross-ZIM article search.

**Step 1: Write failing tests**

```go
func TestCreateAndSearch(t *testing.T) {
    db, err := Open(filepath.Join(t.TempDir(), "search.db"))
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    sourceID, err := db.UpsertSource("wiki.zim", "Wikipedia")
    if err != nil {
        t.Fatal(err)
    }
    err = db.InsertArticles(sourceID, []Article{
        {Path: "A/Linux", Title: "Linux", Body: "Linux is an operating system kernel"},
        {Path: "A/Go", Title: "Go", Body: "Go is a programming language by Google"},
    })
    if err != nil {
        t.Fatal(err)
    }

    results, err := db.Search("linux kernel", 10)
    if err != nil {
        t.Fatal(err)
    }
    if len(results) == 0 {
        t.Fatal("expected search results")
    }
    if results[0].Title != "Linux" {
        t.Errorf("first result = %q", results[0].Title)
    }
}
```

**Step 2: Implement**

Schema matches Python: sources, articles, articles_fts (FTS5), meta table.

**Step 3: Run tests, commit**

Commit: `feat(host-cli): add SQLite FTS5 search database`

---

### Task F.2: ZIM article extraction

**Files:**
- Create: `host-cli/internal/zimext/extract.go`
- Create: `host-cli/internal/zimext/extract_test.go`

Extract article text from ZIM files for indexing. Uses CGO with libzim or a pure Go ZIM reader.

Note: If CGO is not available or libzim is not installed, this package provides a stub that returns an error. The indexer handles this gracefully by skipping ZIM extraction.

**Step 1: Write failing test**

```go
func TestStripHTML(t *testing.T) {
    got := StripHTML("<p>Hello <b>world</b></p>")
    if got != "Hello world" {
        t.Errorf("got %q", got)
    }
}

func TestTruncateText(t *testing.T) {
    text := "First sentence. Second sentence. Third sentence."
    got := TruncateText(text, 20)
    if len(got) > 20 {
        t.Errorf("got %d chars", len(got))
    }
}
```

**Step 2: Implement StripHTML and TruncateText (pure Go, no CGO needed)**

**Step 3: Run tests, commit**

Commit: `feat(host-cli): add HTML stripping and text truncation for ZIM extraction`

---

### Task F.3: Index command

**Files:**
- Create: `host-cli/internal/commands/index.go`
- Create: `host-cli/internal/commands/index_test.go`
- Modify: `host-cli/internal/cli/root.go`

Orchestrates ZIM scanning, article extraction, and FTS5 indexing.

**Step 1: Write failing test**

**Step 2: Implement IndexVault(root string, strategy string) error**

1. Scan for .zim files in root/zim/
2. Open search.db in root/data/
3. Per ZIM: extract articles, insert into FTS5
4. Store metadata (tier, indexed_at)

**Step 3: Wire index command**

**Step 4: Run tests, commit**

Commit: `feat(host-cli): add search index command`

---

## Phase G: Map Viewer Generation

### Task G.1: MapLibre HTML viewer

**Files:**
- Create: `host-cli/internal/mapview/generate.go`
- Create: `host-cli/internal/mapview/generate_test.go`

Generates a standalone HTML file with MapLibre GL JS that renders PMTiles layers.

**Step 1: Write failing test**

```go
func TestGenerateCreatesHTMLWithLayers(t *testing.T) {
    root := t.TempDir()
    layers := []Layer{
        {Name: "OSM Finland", Filename: "osm-finland.pmtiles", Category: "basemap"},
    }
    if err := Generate(root, layers); err != nil {
        t.Fatal(err)
    }
    raw, err := os.ReadFile(filepath.Join(root, "apps", "map", "index.html"))
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(string(raw), "maplibregl") {
        t.Error("missing MapLibre JS")
    }
    if !strings.Contains(string(raw), "osm-finland") {
        t.Error("missing layer reference")
    }
}
```

**Step 2: Implement with html/template**

**Step 3: Wire into apply pipeline (generate after downloads if PMTiles present)**

**Step 4: Run tests, commit**

Commit: `feat(host-cli): add MapLibre map viewer generation`

---

## Summary

| Phase | Tasks | Delivers |
|-------|-------|----------|
| A | A.1 | Real catalog with full recipe fields |
| B | B.1–B.2 | URL resolver + HTTP downloader with resume/SHA256 |
| C | C.1 | Full actions.json generation for drive-runtime |
| D | D.1 | Real apply pipeline with actual downloads |
| E | E.1–E.2 | status + sync commands |
| F | F.1–F.3 | SQLite FTS5 search DB + ZIM extraction + index command |
| G | G.1 | MapLibre map viewer HTML generation |

**Dependencies:**
- Phase B depends on A (resolver needs catalog URLs)
- Phase C is independent
- Phase D depends on B + C (apply uses downloader + toolkit)
- Phase E depends on D (sync uses apply)
- Phase F is independent of B-E (search is a separate concern)
- Phase G is independent (can run after C)

**Parallelizable:** A, C, F.1 can start simultaneously. B follows A. D follows B+C. E follows D. F.2-F.3 follow F.1. G is independent.
