# Svalbard MCP Server Design

**Date:** 2026-04-16
**Status:** Draft

## Goal

Expose the Svalbard drive's knowledge and data to any MCP-capable AI client
(Claude Code, OpenCode, Goose, Cursor, etc.) so the AI can research, search,
browse, and query the vault's offline content.

Primary use case: AI-assisted research. The user asks the AI a question and
the AI can search across hundreds of ZIM archives, read articles, query
structured databases, and understand what is available on the drive.

## Design Principles

- **Stdio transport** — the MCP server is a binary that speaks JSON-RPC over
  stdin/stdout. No port management, no lifecycle coordination. The AI client
  spawns it, talks to it, and kills it when done. This also means any
  MCP-capable client on the host can connect to a plugged-in drive.
- **Few tools, action-dispatched** — minimize AI context pollution. Each tool
  takes an `action` string + `params` object rather than exposing many
  separate tools.
- **Self-discovering** — capabilities are built from drive state at startup
  (search.db, data/*.sqlite, maps/*.pmtiles, recipe index). Add a new ZIM
  or database to the drive and it becomes available without code changes.
- **DRY with the TUI** — the MCP server and the Bubble Tea menu share core
  logic for search, capability detection, and drive scanning. New code is
  needed for content extraction (HTML-to-text) and structured data APIs
  where the TUI only had formatted text output.

## Architecture

```
┌──────────────────────────────────────────────────┐
│  AI Client (Claude Code / OpenCode / Goose / …)  │
└──────────────────┬───────────────────────────────┘
                   │ stdio (JSON-RPC)
┌──────────────────▼───────────────────────────────┐
│  svalbard-drive mcp --drive /path/to/drive       │
│                                                  │
│  ┌─────────┐  ┌─────────┐  ┌─────────────────┐  │
│  │ search  │  │  vault  │  │     query       │  │
│  │  Cap.   │  │  Cap.   │  │     Cap.        │  │
│  └────┬────┘  └────┬────┘  └───────┬─────────┘  │
│       │            │               │             │
│  ┌────▼────────────▼───────────────▼──────────┐  │
│  │           Shared Go packages               │  │
│  │  search.KeywordSearch()  inspect.Sources() │  │
│  │  search.SemanticSearch() inspect.Stats()   │  │
│  │  content.ExtractText()   netutil.Port()    │  │
│  │  binary.Resolve()        config.Load()     │  │
│  └────────────────────────────────────────────┘  │
│       │            │                             │
│  ┌────▼─────┐ ┌────▼─────┐                      │
│  │kiwix-srv │ │ sqlite3  │   (lazy, sync.Once)   │
│  │llama-srv │ │          │                       │
│  └──────────┘ └──────────┘                       │
└──────────────────────────────────────────────────┘
```

## MCP Protocol

Use `github.com/mark3labs/mcp-go` (or equivalent Go MCP SDK) for protocol
compliance rather than hand-rolling JSON-RPC. This handles:

- `initialize` / `initialized` handshake with capabilities negotiation
- Tool `inputSchema` as valid JSON Schema
- Tool call results in MCP's `content` array format (text/image blocks)
- Stdio transport framing

Each `Capability` generates its JSON Schema `inputSchema` from its
`ActionDef` list. The MCP server merges them into proper tool definitions.

## Capability Interface

```go
// internal/mcp/capability.go

type ParamDef struct {
    Name     string   `json:"name"`
    Type     string   `json:"type"`        // string, integer, boolean
    Required bool     `json:"required"`
    Desc     string   `json:"description"`
    Default  any      `json:"default,omitempty"`
    Enum     []string `json:"enum,omitempty"` // for constrained values
}

type ActionDef struct {
    Name   string     `json:"name"`
    Desc   string     `json:"description"`
    Params []ParamDef `json:"params"`
}

type ActionResult struct {
    Data any    `json:"data,omitempty"`
    Text string `json:"text,omitempty"`
    // Errors are returned via the error return value only.
}

type Capability interface {
    Tool() string
    Description() string
    Actions() []ActionDef
    Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error)
    Close() error // cleanup lazy child processes (kiwix, llama-server)
}
```

The server maps `Capability` → MCP tool:

```go
mcp.NewServer(
    search.NewCapability(driveRoot, metadata),
    vault.NewCapability(driveRoot, metadata),
    query.NewCapability(driveRoot, metadata),
).ServeStdio()
```

`Close()` is called on shutdown (stdin EOF / signal). `context.Context`
on `Handle` enables per-request cancellation.

Adding a new capability = implement the interface, add one line to main.

## Drive Metadata

Each capability receives drive metadata at startup for richer descriptions.
To avoid adding a YAML dependency to the Go module, `toolkit_generator.py`
emits a JSON recipe index (`recipes.json`) alongside `actions.json` during
provisioning.

```go
type DriveMetadata struct {
    Manifest map[string]string          // manifest.yaml (preset, region, created)
    Recipes  map[string]RecipeMeta      // .svalbard/recipes.json
    Actions  config.RuntimeConfig       // .svalbard/actions.json
}

type RecipeMeta struct {
    ID          string         `json:"id"`
    Type        string         `json:"type"`
    Description string         `json:"description"`
    Tags        []string       `json:"tags"`
    Viewer      map[string]any `json:"viewer,omitempty"`
    Build       map[string]any `json:"build,omitempty"`
}
```

This lets capabilities provide rich context: "iFixit (repair, electronics)"
or "Fimea pharmaceutical DB has FTS on name, active_ingredient, atc_code."

## Source Identifiers

Search results, `vault.sources`, and `search.read` all use the **Kiwix book
name** as the canonical source identifier: the ZIM filename with `.zim`
stripped (e.g., `ifixit_en_all_2024-01`). This is what Kiwix uses in its
URLs (`/content/ifixit_en_all_2024-01/...`) and what `search.db` stores in
the `sources.filename` column.

The `vault.sources` action enriches these with recipe metadata (description,
tags) by matching filenames to recipe IDs via `DriveMetadata.Recipes`.

All paths in results and `read` params are URL-encoded via `net/url` to
handle spaces, non-ASCII characters, and fragments.

## Tools

### `search` — Find and read content across offline archives

| Action | Params | Returns |
|---|---|---|
| `keyword` | `query` (required), `detail` (link\|snippet\|full, default: snippet), `limit` (default: 20, max 5 for full) | Array of results |
| `semantic` | `query` (required), `detail` (link\|snippet\|full, default: snippet), `limit` (default: 20, max 5 for full) | Array of results (falls back to keyword if unavailable) |
| `read` | `source` (required), `path` (optional — main page if omitted) | Page text + navigable links |

**Result shape** varies by `detail` level:

| `detail` | Fields returned |
|---|---|
| `link` | source, path, title |
| `snippet` | source, path, title, snippet (~200 chars) |
| `full` | source, path, title, body (full text), links (extracted navigation). Capped at limit=5 to prevent oversized responses. |

**`read` action** returns a page with text content and extracted links,
enabling the AI to browse ZIM archives like a website:

```json
{"action": "read", "params": {"source": "ifixit_en_all_2024-01"}}
→ {
    "title": "iFixit — The Free Repair Manual",
    "body": "Categories: Laptops, Phones, Tablets, ...",
    "links": [
      {"path": "Category/Laptop", "label": "Laptops"},
      {"path": "Category/Phone", "label": "Phones"}
    ]
  }

{"action": "read", "params": {"source": "ifixit_en_all_2024-01", "path": "Category/Laptop"}}
→ { "title": "Laptop Repair", "body": "...", "links": [...] }
```

`read` stays in `search` permanently — the AI doesn't need to know the
content type. As the drive gains PDFs and EPUBs, `read` resolves the source
type from recipe metadata and delegates to the right extractor:

| Source type | Extraction strategy |
|---|---|
| ZIM | Kiwix HTTP → `golang.org/x/net/html` → text + links |
| PDF | `pdftotext` or Go library (future) |
| EPUB | Unzip + parse XHTML chapters (future) |

**Implementation notes:**

- `keyword` and `semantic` reuse exported `search.KeywordSearch()` and
  `search.SemanticSearch()` with parameterized `limit` (current code
  hardcodes `LIMIT 20` — must be changed to accept a parameter).
- `snippet` detail uses FTS snippet or first ~200 chars from
  `articles.body` in search.db (cheap, no Kiwix needed).
- `full` and `read` fetch from kiwix-serve (lazy-started via `sync.Once`
  on first call). HTML-to-text + link extraction is new code using
  `golang.org/x/net/html`.
- Kiwix startup uses a proper health-check loop (like `waitForHTTPReady()`
  in `agent.go`), not the current 2-second sleep.

### `vault` — See what's on this drive

| Action | Params | Returns |
|---|---|---|
| `sources` | `type` (optional filter: zim, sqlite, pmtiles, epub, pdf) | Array of sources with id, type, description, tags, size |
| `databases` | — | Array of SQLite DBs with table names |
| `maps` | — | Array of map layers with name, category, coverage |
| `stats` | — | Drive summary: preset, region, counts, total sizes |

**Implementation:** The existing `internal/inspect/` functions produce
human-readable text output. New structured-data functions are needed
alongside the existing `Run()`:

```go
// New functions in internal/inspect/
func Sources(driveRoot string) ([]SourceInfo, error)    // structured
func Stats(driveRoot string) (DriveStats, error)        // structured
func Databases(driveRoot string) ([]DatabaseInfo, error) // structured
func Maps(driveRoot string) ([]MapInfo, error)          // structured

// Existing function stays for TUI
func Run(w io.Writer, driveRoot string) error           // formatted text
```

Both the TUI `Run()` and the vault capability call the same low-level
helpers (`listFilesWithExtension`, `summarizeDirectory`,
`readManifestMetadata` — exported). The vault capability enriches results
with recipe metadata (tags, viewer info) from `DriveMetadata.Recipes`.

### `query` — Query structured databases

| Action | Params | Returns |
|---|---|---|
| `describe` | `database` (required), `table` (optional) | Schema: column names, types, FTS status. Sample rows if table specified. |
| `sql` | `database` (required), `sql` (required) | Query results as array of objects. |

**SQL safety:** The `query` capability uses `database/sql` with
`modernc.org/sqlite` (pure Go, no CGo) and opens databases with
`?mode=ro&_query_only=true` for a true read-only connection at the driver
level. This replaces the shell-out to `sqlite3` for the query tool
specifically. The search capability continues using the `sqlite3` binary
for its own queries (which are not user-controlled).

Database names are resolved from the `data/` directory — the AI uses names
from `vault.databases` output. Only databases in `data/` are accessible;
path traversal is rejected.

## Shared Code: What's Reused, What's New

| Code | Status | Used by |
|---|---|---|
| `search.KeywordSearch()` | Export + add `limit` param | TUI + MCP |
| `search.SemanticSearch()` | Export + add `limit` param | TUI + MCP |
| `search.DetectCapabilities()` | Export | TUI + MCP |
| `inspect.ListFilesWithExtension()` | Export | TUI + MCP |
| `inspect.SummarizeDirectory()` | Export | TUI + MCP |
| `inspect.ReadManifestMetadata()` | Export | TUI + MCP |
| `binary.Resolve()` | Already exported | TUI + MCP |
| `config.Load()` | Already exported | TUI + MCP |
| `netutil.FindAvailablePort()` | Extract from 3 packages | TUI + MCP |
| `content.ExtractText()` | **New** — HTML-to-text + links | MCP only |
| `inspect.Sources()` / `Stats()` | **New** — structured returns | MCP only |
| `query.Execute()` | **New** — `database/sql` read-only | MCP only |
| `metadata.Load()` | **New** — recipe index loader | MCP only |
| `mcp/server.go` | **New** — MCP protocol via SDK | MCP only |

## Entry Point

New subcommand on the existing binary:

```
svalbard-drive mcp --drive /path/to/drive
```

A thin wrapper script on the drive for external MCP client configs:

```bash
#!/bin/sh
# .svalbard/mcp-server.sh
DRIVE_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
exec "$DRIVE_ROOT/bin/$(uname -s | tr A-Z a-z)-$(uname -m)/svalbard-drive" \
    mcp --drive "$DRIVE_ROOT"
```

### Auto-Configuration for Drive AI Clients

When a user launches an AI client from the drive menu (`agent` action), the
MCP server is pre-wired automatically. `PrepareClientLaunchConfig()` in
`internal/agent/agent.go` already generates per-client config files and
environment variables — the MCP server config is added to the same flow.

**OpenCode** — inject `mcpServers` into the generated `opencode.json`:

```go
// In PrepareClientLaunchConfig(), opencode case:
mcpBinary, _ := os.Executable()
content := fmt.Sprintf(`{
  ...existing provider config...,
  "mcpServers": {
    "svalbard": {
      "command": "%s",
      "args": ["mcp", "--drive", "%s"]
    }
  }
}`, mcpBinary, driveRoot)
```

The MCP server command points at the same `svalbard-drive` binary that's
already resolved and running. No external script needed.

**Goose** — add MCP config via environment or generated config file:

```go
// In PrepareClientLaunchConfig(), goose case:
mcpConfigPath := filepath.Join(configRoot, "mcp.json")
// Write: {"svalbard": {"command": mcpBinary, "args": ["mcp", "--drive", driveRoot]}}
cfg.Env["GOOSE_MCP_SERVERS"] = mcpConfigPath
```

**Llama-server web UI** — not MCP-aware (simple completion interface).
No integration needed. Users who want MCP with the web UI connect an
external MCP client to the drive.

**Key principle:** `os.Executable()` resolves the MCP server command from
the currently running binary. Same binary, different subcommand. No second
binary, no path resolution.

### External Client Configuration

For AI clients running on the host (not launched from the drive menu),
users point their MCP config at the wrapper script:

**Claude Code** (`~/.claude.json` or project settings):
```json
{
  "mcpServers": {
    "svalbard": {
      "command": "/Volumes/MyStick/.svalbard/mcp-server.sh"
    }
  }
}
```

**OpenCode** / **Goose** — same pattern with their respective config files.

## Lazy Service Lifecycle

Services are started on demand via `sync.Once` and cleaned up via
`Capability.Close()` when the MCP server shuts down:

| Service | Started when | Readiness |
|---|---|---|
| kiwix-serve | First `search.read` or `detail: full` | Health-check loop (HTTP GET until 200) |
| llama-server (embed) | First `search.semantic` | Health-check loop (HTTP GET /health) |

Both bind to 127.0.0.1 on auto-selected ports via `netutil.FindAvailablePort()`.
`sync.Once` prevents duplicate instances from concurrent tool calls.
`Close()` kills child processes on shutdown (stdin EOF or signal).

## Expandability

### Auto-discovered content (no code changes needed)

- New ZIM added to `zim/` and indexed → appears in search results and
  vault sources
- New SQLite DB added to `data/` → queryable via `query.sql`
- New PMTiles added to `maps/` → listed in `vault.maps`

### New content types (add extractor, no API change)

`search.read` resolves source type from recipe metadata and delegates to
the right extractor. Adding PDF support = add a PDF extractor, register
it. The API surface (`source` + `path`) stays the same.

### New capabilities (implement interface + one line in main)

Future capabilities follow the same pattern — implement `Capability`,
add one line to `main.go`.

### Recipe-driven MCP exposure (future)

Custom actions in recipes can opt into MCP exposure:

```yaml
# In a recipe YAML
mcp:
  tool: search
  action: lookup
  description: "Look up plant species by common name"
  params:
    name: {type: string, required: true}
```

The MCP server reads recipe metadata from `recipes.json` and
auto-registers capabilities defined there. Future work.

## New Dependencies

| Dependency | Purpose | Notes |
|---|---|---|
| `github.com/mark3labs/mcp-go` | MCP protocol (JSON-RPC, stdio, schema) | Avoids hand-rolling protocol |
| `modernc.org/sqlite` | Read-only SQL for `query` tool | Pure Go, no CGo. Only for query tool. |
| `golang.org/x/net/html` | HTML-to-text + link extraction | Already in Go ecosystem |

Existing search code continues using the `sqlite3` binary for its own
internal queries (FTS, embeddings). The `modernc.org/sqlite` driver is only
used by the `query` capability for user-provided SQL, where the read-only
guarantee must be enforced at the driver level.

## Provisioning Changes

`toolkit_generator.py` gains one new output:

```
.svalbard/
  actions.json      # existing — menu structure
  recipes.json      # NEW — recipe metadata index for MCP
  mcp-server.sh     # NEW — wrapper script for external clients
  checksums.sha256   # existing
```

`recipes.json` is a flat map of recipe ID → metadata (id, type,
description, tags, viewer, build). Generated from the same recipe
snapshots already copied to `.svalbard/config/recipes/`.

`mcp-server.sh` is a static script generated during provisioning.

## File Layout

```
drive-runtime/
  cmd/svalbard-drive/
    main.go                    # add "mcp" subcommand
  internal/
    mcp/
      capability.go            # Capability interface + types
      server.go                # MCP server using mcp-go SDK
    search/
      search.go                # export + parameterize core functions
      capability.go            # implements mcp.Capability
    inspect/
      inspect.go               # export low-level helpers
      structured.go            # NEW: Sources(), Stats(), etc.
      capability.go            # implements mcp.Capability (vault tool)
    query/
      query.go                 # NEW: database/sql read-only queries
      capability.go            # implements mcp.Capability
    content/
      extract.go               # NEW: HTML-to-text + link extraction
    metadata/
      metadata.go              # NEW: loads DriveMetadata from JSON
    netutil/
      port.go                  # extracted FindAvailablePort()
```
