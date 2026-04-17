# Svalbard MCP Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `svalbard-drive mcp` subcommand that exposes the drive's content as MCP tools (search, vault, query) over stdio for any AI client.

**Architecture:** The `internal/mcp/` package owns all MCP-specific code: the Capability interface, server, metadata loading, and capability adapters that wrap domain packages (`search`, `inspect`, `query`). Domain packages stay transport-agnostic. The MCP server uses `mcp-go` SDK for protocol compliance.

**Tech Stack:** Go 1.25, `mcp-go` (MCP protocol), `modernc.org/sqlite` (read-only SQL), `golang.org/x/net/html` (content extraction)

**Design doc:** `docs/plans/2026-04-16-svalbard-mcp-server-design.md`

**New packages:** 3 (`internal/mcp/`, `internal/netutil/`, `internal/query/`)
**New files in existing packages:** `search/content.go`, `inspect/structured.go`
**Modified files:** `main.go`, `search.go`, `session.go`, `inspect.go`, `agent.go`, `browse.go`, `chat.go`, `go.mod`, `toolkit_generator.py`

---

## Phase 1: Foundation

### Task 1: Extract `netutil.FindAvailablePort`

`findAvailablePort` is duplicated in 4 packages. Extract to shared utility.

**Files:**
- Create: `drive-runtime/internal/netutil/port.go`
- Create: `drive-runtime/internal/netutil/port_test.go`
- Modify: `drive-runtime/internal/search/search.go`
- Modify: `drive-runtime/internal/browse/browse.go`
- Modify: `drive-runtime/internal/agent/agent.go`
- Modify: `drive-runtime/internal/chat/chat.go`

**Step 1: Write the test**

```go
// internal/netutil/port_test.go
package netutil_test

import (
	"net"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
)

func TestFindAvailablePortReturnsPreferredWhenFree(t *testing.T) {
	port, err := netutil.FindAvailablePort("127.0.0.1", 19876)
	if err != nil {
		t.Fatalf("FindAvailablePort() error = %v", err)
	}
	if port < 19876 || port > 19896 {
		t.Fatalf("FindAvailablePort() = %d, want in range [19876, 19896]", port)
	}
}

func TestFindAvailablePortSkipsOccupiedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:19877")
	if err != nil {
		t.Skip("cannot bind test port")
	}
	defer ln.Close()
	port, err := netutil.FindAvailablePort("127.0.0.1", 19877)
	if err != nil {
		t.Fatalf("FindAvailablePort() error = %v", err)
	}
	if port == 19877 {
		t.Fatal("FindAvailablePort() returned occupied port")
	}
}
```

**Step 2:** Run test — expect FAIL (package missing)
**Step 3:** Implement `netutil.FindAvailablePort` (same logic as existing)
**Step 4:** Run test — expect PASS
**Step 5:** Replace all 4 duplicates with `netutil.FindAvailablePort(`
**Step 6:** Run `go test ./...` — PASS
**Step 7:** Commit: `refactor: extract FindAvailablePort to internal/netutil`

---

### Task 2: Add new dependencies

```bash
cd drive-runtime
go get github.com/mark3labs/mcp-go
go get modernc.org/sqlite
go get golang.org/x/net
go mod tidy && go build ./...
```

Commit: `deps: add mcp-go, modernc.org/sqlite, x/net/html`

---

### Task 3: MCP package — interface, server, metadata

All MCP-specific code lives in `internal/mcp/`. This package:
- Defines the `Capability` interface
- Wraps `mcp-go` SDK for stdio serving
- Loads drive metadata (manifest + recipes)

**Files:**
- Create: `drive-runtime/internal/mcp/capability.go`
- Create: `drive-runtime/internal/mcp/server.go`
- Create: `drive-runtime/internal/mcp/metadata.go`
- Create: `drive-runtime/internal/mcp/server_test.go`

**Step 1: Write the test**

```go
// internal/mcp/server_test.go
package mcp_test

import (
	"context"
	"testing"

	mcpserver "github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

type stubCapability struct{}

func (s *stubCapability) Tool() string        { return "test" }
func (s *stubCapability) Description() string { return "A test tool" }
func (s *stubCapability) Actions() []mcpserver.ActionDef {
	return []mcpserver.ActionDef{
		{Name: "ping", Desc: "Returns pong", Params: nil},
	}
}
func (s *stubCapability) Handle(ctx context.Context, action string, params map[string]any) (mcpserver.ActionResult, error) {
	return mcpserver.ActionResult{Text: "pong"}, nil
}
func (s *stubCapability) Close() error { return nil }

func TestNewServerRegistersCapabilities(t *testing.T) {
	srv := mcpserver.NewServer(&stubCapability{})
	tools := srv.Tools()
	if len(tools) != 1 {
		t.Fatalf("Tools() = %d tools, want 1", len(tools))
	}
	if tools[0].Name != "test" {
		t.Fatalf("Tools()[0].Name = %q, want %q", tools[0].Name, "test")
	}
}
```

**Step 2:** Run test — expect FAIL

**Step 3: Implement capability.go**

```go
// internal/mcp/capability.go
package mcp

import "context"

type ParamDef struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Desc     string   `json:"description"`
	Default  any      `json:"default,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

type ActionDef struct {
	Name   string     `json:"name"`
	Desc   string     `json:"description"`
	Params []ParamDef `json:"params"`
}

// ActionResult carries tool output. Errors use the error return only.
type ActionResult struct {
	Data any    `json:"data,omitempty"`
	Text string `json:"text,omitempty"`
}

type Capability interface {
	Tool() string
	Description() string
	Actions() []ActionDef
	Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error)
	Close() error
}
```

**Step 4: Implement metadata.go**

```go
// internal/mcp/metadata.go
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
)

type RecipeMeta struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Viewer      map[string]any `json:"viewer,omitempty"`
	Build       map[string]any `json:"build,omitempty"`
}

type DriveMetadata struct {
	Manifest map[string]string
	Recipes  map[string]RecipeMeta
}

func LoadMetadata(driveRoot string) (DriveMetadata, error) {
	meta := DriveMetadata{
		Manifest: map[string]string{},
		Recipes:  map[string]RecipeMeta{},
	}
	// Reuse inspect's manifest parser (DRY)
	manifest, _ := inspect.ReadManifestMetadata(
		filepath.Join(driveRoot, "manifest.yaml"))
	meta.Manifest = manifest

	recipesPath := filepath.Join(driveRoot, ".svalbard", "recipes.json")
	if data, err := os.ReadFile(recipesPath); err == nil {
		if err := json.Unmarshal(data, &meta.Recipes); err != nil {
			return meta, fmt.Errorf("invalid recipes.json: %w", err)
		}
	}
	return meta, nil
}
```

Note: requires Task 5 (export `ReadManifestMetadata`) first. Reorder
during implementation if needed, or inline a simple parser initially
and switch to the exported helper after Task 5.

**Step 5: Implement server.go**

Wraps `mcp-go` SDK. Consult SDK docs for exact API. Must:
- Register one MCP tool per `Capability` with JSON Schema `inputSchema`
- Route `tools/call` to the correct `cap.Handle()`
- Handle `initialize` / `initialized` handshake
- `Close()` all capabilities on shutdown

```go
// internal/mcp/server.go
package mcp

type ToolInfo struct {
	Name        string
	Description string
}

type Server struct {
	capabilities []Capability
}

func NewServer(caps ...Capability) *Server {
	return &Server{capabilities: caps}
}

func (s *Server) Tools() []ToolInfo { /* iterate capabilities */ }
func (s *Server) ServeStdio() error { /* mcp-go stdio transport */ }
func (s *Server) Close() error {
	for _, c := range s.capabilities {
		c.Close()
	}
	return nil
}
```

**Step 6:** Run test — PASS
**Step 7:** Commit: `feat(mcp): add Capability interface, server, and metadata loader`

---

### Task 4: MCP subcommand entry point

**Files:**
- Modify: `drive-runtime/cmd/svalbard-drive/main.go`

Add early intercept *before* `config.Load()` so the MCP server works
without `actions.json` (external invocation):

```go
func run() error {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		drive := ""
		for i, arg := range os.Args {
			if arg == "--drive" && i+1 < len(os.Args) {
				drive = os.Args[i+1]
			}
		}
		if drive == "" {
			drive = os.Getenv("DRIVE_ROOT")
		}
		if drive == "" {
			return fmt.Errorf("--drive path required")
		}
		return runMCP(drive)
	}
	// ... existing config.Load() and menu dispatch ...
}

func runMCP(driveRoot string) error {
	srv := mcp.NewServer() // empty for now, capabilities added in later tasks
	defer srv.Close()
	return srv.ServeStdio()
}
```

Build: `go build ./cmd/svalbard-drive/`
Commit: `feat(mcp): add mcp subcommand entry point`

---

## Phase 2: Vault Capability

### Task 5: Export inspect helpers + add structured data functions

**Files:**
- Modify: `drive-runtime/internal/inspect/inspect.go` (export 4 helpers)
- Create: `drive-runtime/internal/inspect/structured.go` (new data types + functions)

**Step 1:** Export helpers in `inspect.go`:
- `listFilesWithExtension` → `ListFilesWithExtension`
- `summarizeDirectory` → `SummarizeDirectory`
- `readManifestMetadata` → `ReadManifestMetadata`
- `humanSize` → `HumanSize`
- Update all callers within the file

**Step 2: Write test**

```go
// Add to inspect_test.go
func TestSourcesReturnsZIMAndSQLiteFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "zim"), 0o755)
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	os.WriteFile(filepath.Join(dir, "zim", "ifixit.zim"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "data", "pharma.sqlite"), []byte("x"), 0o644)

	sources, err := inspect.Sources(dir)
	if err != nil {
		t.Fatalf("Sources() error = %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("Sources() = %d items, want 2", len(sources))
	}
}

func TestStatsReturnsDriveSummary(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "zim"), 0o755)
	os.WriteFile(filepath.Join(dir, "zim", "wiki.zim"), []byte("data"), 0o644)
	os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("preset: test\n"), 0o644)

	stats, err := inspect.Stats(dir)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.Preset != "test" {
		t.Fatalf("Stats().Preset = %q, want %q", stats.Preset, "test")
	}
}
```

**Step 3: Implement structured.go**

```go
// internal/inspect/structured.go
package inspect

type SourceInfo struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Size        int64    `json:"size"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type DriveStats struct {
	Preset  string            `json:"preset"`
	Region  string            `json:"region"`
	Created string            `json:"created"`
	Counts  map[string]int    `json:"counts"`
	Sizes   map[string]string `json:"sizes"`
}

type DatabaseInfo struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Tables []string `json:"tables"`
}

type MapInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Category string `json:"category,omitempty"`
	Coverage string `json:"coverage,omitempty"`
}

func Sources(driveRoot string, filterType ...string) ([]SourceInfo, error) { ... }
func Stats(driveRoot string) (DriveStats, error) { ... }
func Databases(driveRoot string) ([]DatabaseInfo, error) { ... }
func Maps(driveRoot string) ([]MapInfo, error) { ... }
```

`Databases()` populates `Tables` via best-effort introspection using
`modernc.org/sqlite` (read-only open, query `sqlite_master`). If the DB
can't be opened, `Tables` is nil.

**Step 4:** Run tests — PASS
**Step 5:** Commit: `feat(mcp): export inspect helpers and add structured data functions`

---

### Task 6: Vault capability adapter (in mcp/)

The vault adapter lives in `internal/mcp/`, not in `inspect/`. This keeps
domain packages transport-agnostic (correct dependency direction).

**Files:**
- Create: `drive-runtime/internal/mcp/vault.go`
- Create: `drive-runtime/internal/mcp/vault_test.go`
- Modify: `drive-runtime/cmd/svalbard-drive/main.go` (wire in)

**Step 1: Write the test**

```go
// internal/mcp/vault_test.go
package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcpserver "github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

func TestVaultCapabilityToolName(t *testing.T) {
	cap := mcpserver.NewVaultCapability(t.TempDir(), mcpserver.DriveMetadata{})
	if cap.Tool() != "vault" {
		t.Fatalf("Tool() = %q, want %q", cap.Tool(), "vault")
	}
}

func TestVaultSourcesAction(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "zim"), 0o755)
	os.WriteFile(filepath.Join(dir, "zim", "test.zim"), []byte("x"), 0o644)

	cap := mcpserver.NewVaultCapability(dir, mcpserver.DriveMetadata{})
	result, err := cap.Handle(context.Background(), "sources", nil)
	if err != nil {
		t.Fatalf("Handle(sources) error = %v", err)
	}
	if result.Data == nil {
		t.Fatal("Handle(sources) returned nil Data")
	}
}

func TestVaultUnknownActionReturnsError(t *testing.T) {
	cap := mcpserver.NewVaultCapability(t.TempDir(), mcpserver.DriveMetadata{})
	_, err := cap.Handle(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("Handle(nonexistent) should return error")
	}
}
```

**Step 2: Implement vault.go**

```go
// internal/mcp/vault.go
package mcp

import (
	"context"
	"fmt"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
)

type VaultCapability struct {
	driveRoot string
	meta      DriveMetadata
}

func NewVaultCapability(driveRoot string, meta DriveMetadata) *VaultCapability {
	return &VaultCapability{driveRoot: driveRoot, meta: meta}
}

func (c *VaultCapability) Tool() string        { return "vault" }
func (c *VaultCapability) Description() string { return "See what's on this Svalbard drive" }

func (c *VaultCapability) Actions() []ActionDef {
	return []ActionDef{
		{Name: "sources", Desc: "List all content sources", Params: []ParamDef{
			{Name: "type", Type: "string", Desc: "Filter by type", Enum: []string{"zim", "sqlite", "pmtiles", "pdf", "epub"}},
		}},
		{Name: "databases", Desc: "List SQLite databases with table names"},
		{Name: "maps", Desc: "List available map layers with category and coverage"},
		{Name: "stats", Desc: "Drive summary: preset, region, content counts"},
	}
}

func (c *VaultCapability) Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error) {
	switch action {
	case "sources":
		typeFilter, _ := params["type"].(string)
		var sources []inspect.SourceInfo
		var err error
		if typeFilter != "" {
			sources, err = inspect.Sources(c.driveRoot, typeFilter)
		} else {
			sources, err = inspect.Sources(c.driveRoot)
		}
		if err != nil {
			return ActionResult{}, err
		}
		// Enrich with recipe metadata
		for i, s := range sources {
			if r, ok := c.meta.Recipes[s.ID]; ok {
				sources[i].Description = r.Description
				sources[i].Tags = r.Tags
			}
		}
		return ActionResult{Data: sources}, nil

	case "databases":
		dbs, err := inspect.Databases(c.driveRoot)
		if err != nil {
			return ActionResult{}, err
		}
		return ActionResult{Data: dbs}, nil

	case "maps":
		m, err := inspect.Maps(c.driveRoot)
		if err != nil {
			return ActionResult{}, err
		}
		// Enrich with recipe metadata
		for i, mi := range m {
			if r, ok := c.meta.Recipes[mi.Name]; ok {
				if v, ok := r.Viewer["category"].(string); ok {
					m[i].Category = v
				}
				m[i].Coverage = r.Description
			}
		}
		return ActionResult{Data: m}, nil

	case "stats":
		stats, err := inspect.Stats(c.driveRoot)
		if err != nil {
			return ActionResult{}, err
		}
		return ActionResult{Data: stats}, nil

	default:
		return ActionResult{}, fmt.Errorf("unknown vault action: %s", action)
	}
}

func (c *VaultCapability) Close() error { return nil }
```

**Step 3: Wire into runMCP**

```go
func runMCP(driveRoot string) error {
	meta, _ := mcp.LoadMetadata(driveRoot)
	srv := mcp.NewServer(
		mcp.NewVaultCapability(driveRoot, meta),
	)
	defer srv.Close()
	return srv.ServeStdio()
}
```

**Step 4:** Run tests — PASS
**Step 5:** Commit: `feat(mcp): implement vault capability`

---

## Phase 3: Search Capability

### Task 7: Parameterize search + add Kiwix lifecycle methods

**Files:**
- Modify: `drive-runtime/internal/search/search.go`
- Modify: `drive-runtime/internal/search/session.go`

**Changes:**

In `search.go`:
- `keywordSearch` — add `limit int` param, replace `LIMIT 20` with `LIMIT %d`
- `semanticSearch` — add `limit int` param
- Replace `time.Sleep(2*time.Second)` in `startKiwix` with health-check loop

In `session.go`:
- `Session.Search()` — add `limit int` param, pass to internal functions
- Add `Session.EnsureKiwix(ctx context.Context) error` — lazily starts kiwix
  with mutex protection and proper health-check
- Add `Session.KiwixPort() int` — returns port (0 if not started)
- Update `search.Run()` (TUI) to pass `limit: 20`

Do NOT export internal helpers (`runSQLite`, `startKiwix`, etc.).

**Test:** Run `go test ./internal/search/` — existing tests pass
**Commit:** `refactor(search): parameterize limit and add Kiwix lifecycle methods`

---

### Task 8: HTML-to-text content extraction (in search/)

**Files:**
- Create: `drive-runtime/internal/search/content.go`
- Create: `drive-runtime/internal/search/content_test.go`

HTML extraction is only used by the search capability, so it lives in the
`search` package. No separate `content/` package needed.

**Step 1: Write the test**

```go
// internal/search/content_test.go
package search_test

import (
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

func TestExtractTextReturnsPlainTextFromHTML(t *testing.T) {
	html := `<html><body><h1>Title</h1><p>Hello <b>world</b></p></body></html>`
	page, err := search.ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if page.Title != "Title" {
		t.Fatalf("Title = %q, want %q", page.Title, "Title")
	}
	if page.Body == "" {
		t.Fatal("Body is empty")
	}
}

func TestExtractTextExtractsLinks(t *testing.T) {
	html := `<html><body><a href="/content/book/Article">Link Text</a></body></html>`
	page, err := search.ExtractText(html)
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if len(page.Links) == 0 {
		t.Fatal("no links extracted")
	}
}

func TestExtractTextStripsKiwixChrome(t *testing.T) {
	html := `<html><body>
		<div id="kiwix_serve_taskbar">nav</div>
		<p>Content</p>
	</body></html>`
	page, _ := search.ExtractText(html)
	if strings.Contains(page.Body, "nav") {
		t.Fatal("should strip kiwix chrome")
	}
}
```

**Step 2: Implement content.go**

Types: `PageLink{Path, Label}`, `Page{Title, Body, Links}`.
Uses `golang.org/x/net/html` to parse HTML, walk tree, extract text +
internal links, skip kiwix chrome elements, skip scripts/styles.
URL-decode extracted paths via `net/url.PathUnescape`.

**Step 3:** Run tests — PASS
**Step 4:** Commit: `feat(mcp): add HTML-to-text extraction in search package`

---

### Task 9: Search capability adapter (in mcp/)

**Files:**
- Create: `drive-runtime/internal/mcp/search.go`
- Create: `drive-runtime/internal/mcp/search_test.go`
- Modify: `drive-runtime/cmd/svalbard-drive/main.go` (wire in)

**Step 1: Write the test**

```go
// internal/mcp/search_test.go
package mcp_test

func TestSearchCapabilityToolName(t *testing.T) {
	cap := mcpserver.NewSearchCapability(t.TempDir(), mcpserver.DriveMetadata{})
	if cap.Tool() != "search" {
		t.Fatalf("Tool() = %q", cap.Tool())
	}
}

func TestSearchCapabilityKeywordFailsWithoutSearchDB(t *testing.T) {
	cap := mcpserver.NewSearchCapability(t.TempDir(), mcpserver.DriveMetadata{})
	_, err := cap.Handle(context.Background(), "keyword", map[string]any{"query": "test"})
	if err == nil {
		t.Fatal("should fail without search.db")
	}
}
```

**Step 2: Implement search.go**

```go
// internal/mcp/search.go
package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

type SearchCapability struct {
	driveRoot   string
	meta        DriveMetadata
	sessionOnce sync.Once
	session     *search.Session
	sessionErr  error
}

func NewSearchCapability(driveRoot string, meta DriveMetadata) *SearchCapability { ... }
func (c *SearchCapability) Tool() string        { return "search" }
func (c *SearchCapability) Description() string { return "Search and read offline archives" }
func (c *SearchCapability) Actions() []ActionDef { /* keyword, semantic, read */ }

func (c *SearchCapability) Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error) {
	switch action {
	case "keyword", "semantic":
		return c.handleSearch(ctx, action, params)
	case "read":
		return c.handleRead(ctx, params)
	default:
		return ActionResult{}, fmt.Errorf("unknown search action: %s", action)
	}
}
```

Key implementation details:
- **Limit validation:** `if limit < 1 { limit = 1 }; if limit > 50 { limit = 50 }; if detail == "full" && limit > 5 { limit = 5 }`
- **Kiwix lifecycle:** `fetchPage` calls `session.EnsureKiwix(ctx)` before HTTP fetch
- **URL encoding:** both book name and article path use `url.PathEscape()`
- **Source ID:** `strings.TrimSuffix(result.Filename, ".zim")` = Kiwix book name
- **`detail` levels:** `link` = pointer only, `snippet` = + FTS snippet, `full` = + Kiwix fetch + text extraction
- **`read` action:** calls `fetchPage(ctx, source+".zim", path)`, returns `search.Page`
- **`Close()`:** calls `session.Close()` to kill lazy kiwix/embed processes

**Step 3: Wire into runMCP**

```go
srv := mcp.NewServer(
	mcp.NewSearchCapability(driveRoot, meta),
	mcp.NewVaultCapability(driveRoot, meta),
)
```

**Step 4:** Run tests — PASS
**Step 5:** Commit: `feat(mcp): implement search capability with keyword, semantic, and read`

---

## Phase 4: Query Capability

### Task 10: Read-only SQL query execution

**Files:**
- Create: `drive-runtime/internal/query/query.go`
- Create: `drive-runtime/internal/query/query_test.go`

Uses `database/sql` + `modernc.org/sqlite` with `?mode=ro&_query_only=true`
for true read-only at the driver level.

**Step 1: Write tests**

- `TestExecuteSelectReturnsRows` — SELECT returns 2 rows
- `TestExecuteRejectsWriteStatements` — INSERT fails
- `TestExecuteRejectsDropStatements` — DROP fails
- `TestExecuteRejectsPathTraversal` — `../etc/passwd` rejected
- `TestDescribeReturnsSchema` — lists tables + columns
- `TestDescribeReturnsFTSStatus` — detects FTS5 virtual tables
- `TestDescribeReturnsSampleRows` — includes first 3 rows when table specified

**Step 2: Implement**

```go
// internal/query/query.go
package query

func Execute(driveRoot, database, sqlQuery string) ([]map[string]any, error) { ... }
func Describe(driveRoot, database, table string) (SchemaInfo, error) { ... }
```

Types: `TableSchema{Name, Columns, IsFTS, Samples}`, `SchemaInfo{Database, Tables}`

Key details:
- `resolvePath` rejects `..`, `/`, `\` in database name
- `openReadOnly` uses `file:PATH?mode=ro&_query_only=true`
- `describeTable` validates table name against `sqlite_master` before
  using in `PRAGMA table_info(%q)` (prevents injection)
- `isFTSTable` checks `sqlite_master.sql` for FTS5/FTS4
- `sampleRows` returns first 3 rows when a specific table is requested

**Step 3:** Run tests — PASS
**Step 4:** Commit: `feat(mcp): add read-only SQL query execution`

---

### Task 11: Query capability adapter (in mcp/)

**Files:**
- Create: `drive-runtime/internal/mcp/query.go`
- Create: `drive-runtime/internal/mcp/query_test.go`
- Modify: `drive-runtime/cmd/svalbard-drive/main.go` (wire all 3)

**Step 1: Write tests**

```go
func TestQueryCapabilitySQLAction(t *testing.T) { ... }
func TestQueryCapabilityDescribeAction(t *testing.T) { ... }
```

**Step 2: Implement query.go**

```go
// internal/mcp/query.go
package mcp

import "github.com/pkronstrom/svalbard/drive-runtime/internal/query"

type QueryCapability struct { ... }
func NewQueryCapability(driveRoot string, meta DriveMetadata) *QueryCapability { ... }
func (c *QueryCapability) Tool() string { return "query" }
func (c *QueryCapability) Handle(...) (ActionResult, error) {
	// Routes "describe" → query.Describe(), "sql" → query.Execute()
}
```

**Step 3: Wire all three into runMCP**

```go
func runMCP(driveRoot string) error {
	meta, _ := mcp.LoadMetadata(driveRoot)
	srv := mcp.NewServer(
		mcp.NewSearchCapability(driveRoot, meta),
		mcp.NewVaultCapability(driveRoot, meta),
		mcp.NewQueryCapability(driveRoot, meta),
	)
	defer srv.Close()
	return srv.ServeStdio()
}
```

**Step 4:** Run `go test ./...` — ALL PASS
**Step 5:** Commit: `feat(mcp): implement query capability`

---

## Phase 5: Auto-Configuration

### Task 12: Inject MCP config into AI client launchers

**Files:**
- Modify: `drive-runtime/internal/agent/agent.go`
- Modify: `drive-runtime/internal/agent/agent_test.go`

In `PrepareClientLaunchConfig()`:

**OpenCode** — add `mcpServers` to generated `opencode.json`:

```go
mcpBinary, err := os.Executable()
if err != nil {
	return LaunchConfig{}, fmt.Errorf("resolve mcp binary: %w", err)
}
// Add to the JSON template:
//   "mcpServers": {"svalbard": {"command": mcpBinary, "args": ["mcp", "--drive", driveRoot]}}
```

**Goose** — write `mcp-servers.json` and set env var:

```go
mcpBinary, err := os.Executable()
// ...
if err := os.WriteFile(mcpConfigPath, []byte(mcpConfig), 0o644); err != nil {
	return LaunchConfig{}, fmt.Errorf("write mcp config: %w", err)
}
cfg.Env["GOOSE_MCP_SERVERS"] = mcpConfigPath
```

**Test:** verify generated `opencode.json` contains `mcpServers.svalbard`
**Commit:** `feat(mcp): auto-configure MCP server for OpenCode and Goose`

---

### Task 13: Generate recipes.json and mcp-server.sh during provisioning

**Files:**
- Modify: `src/svalbard/toolkit_generator.py`

**recipes.json** — flat map of recipe ID → metadata (id, type, description,
tags, viewer, build). Generated from preset sources.

**mcp-server.sh** — wrapper script with correct platform naming
(`Darwin→macos`, `Linux→linux`, matching `platform.go`):

```bash
#!/bin/sh
DRIVE_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OS="$(uname -s)"
case "$OS" in Darwin) OS="macos" ;; Linux) OS="linux" ;; esac
exec "$DRIVE_ROOT/bin/$OS-$(uname -m)/svalbard-drive" mcp --drive "$DRIVE_ROOT"
```

**Test:** Run existing toolkit tests
**Commit:** `feat(mcp): generate recipes.json and mcp-server.sh during provisioning`

---

## Phase 6: Integration & Verification

### Task 14: End-to-end smoke test

**Files:**
- Create: `drive-runtime/internal/mcp/integration_test.go`

Creates minimal drive layout (manifest.yaml, .zim placeholder, .sqlite test
DB), instantiates all three capabilities, verifies:
- `vault.stats` returns preset name
- `vault.sources` lists the ZIM
- `query.describe` returns table schema
- `query.sql` returns rows
- `search.keyword` returns error gracefully (no sqlite3 binary in test)

**Commit:** `test(mcp): add integration smoke test`

---

### Task 15: Build and verify

```bash
cd drive-runtime && go build -o /tmp/svalbard-drive ./cmd/svalbard-drive/
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | /tmp/svalbard-drive mcp --drive /tmp/test
cd drive-runtime && go test ./... -v
```

**Commit:** `feat(mcp): complete MCP server implementation`

---

## File Summary

**3 new packages (+ tests):**
```
internal/mcp/          capability.go, server.go, metadata.go,
                       vault.go, search.go, query.go,
                       server_test.go, vault_test.go, search_test.go,
                       query_test.go, integration_test.go
internal/netutil/      port.go, port_test.go
internal/query/        query.go, query_test.go
```

**New files in existing packages:**
```
internal/search/       content.go, content_test.go
internal/inspect/      structured.go
```

**Modified files:**
```
cmd/svalbard-drive/    main.go
internal/search/       search.go, session.go
internal/inspect/      inspect.go
internal/agent/        agent.go, agent_test.go
internal/browse/       browse.go
internal/chat/         chat.go
go.mod, go.sum
src/svalbard/          toolkit_generator.py
```

**Dependency direction:** `mcp/` → imports → `search/`, `inspect/`, `query/` (correct).
Domain packages never import `mcp/`.
