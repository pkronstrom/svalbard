# Search Accuracy & RAG — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve semantic search accuracy via section-aware chunking, hybrid FTS5+vector search with RRF/MMR, Nomic at 256 matryoshka dims, and in-process sqlite-vec — as specified in `docs/plans/2026-04-18-search-accuracy-rag.md`.

**Architecture:** Two Go modules (`host-cli`, `drive-runtime`) swap from `modernc.org/sqlite` to `ncruces/go-sqlite3` with sqlite-vec. Host-cli gains chunked indexing. Drive-runtime gains a shared search engine with hybrid RRF+MMR ranking, used by both the interactive CLI and MCP server.

**Tech Stack:** Go 1.25, ncruces/go-sqlite3 (Wasm, no CGo), sqlite-vec via asg017/sqlite-vec-go-bindings, llama-server (GGUF inference)

---

### Task 1: host-cli — swap modernc → ncruces + sqlite-vec

**Files:**
- Modify: `host-cli/go.mod` (drop modernc.org/sqlite, add ncruces + sqlite-vec-go-bindings)
- Modify: `host-cli/internal/searchdb/db.go:7-11` (import swap), `db.go:34-68` (schema), `db.go:71` (driver name)

**Step 1: Update go.mod**

```bash
cd host-cli
go get github.com/ncruces/go-sqlite3@latest
go get github.com/ncruces/go-sqlite3/driver@latest
go get github.com/asg017/sqlite-vec-go-bindings/ncruces@latest
```

Then remove `modernc.org/sqlite` from go.mod requires and run `go mod tidy`.

**Step 2: Swap imports in searchdb/db.go**

Replace:
```go
import (
    "database/sql"
    "fmt"
    "time"

    _ "modernc.org/sqlite"
)
```

With:
```go
import (
    "database/sql"
    "fmt"
    "time"

    _ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
    _ "github.com/ncruces/go-sqlite3/driver"
)
```

**Step 3: Change driver name**

In `Open()` (line 71), change `sql.Open("sqlite", path)` to `sql.Open("sqlite3", path)`.

**Step 4: Verify build and tests**

Run: `cd host-cli && go build ./... && go test ./internal/searchdb/ -v`
Expected: PASS (FTS5 works with ncruces, API-compatible)

**Step 5: Commit**

```
feat(searchdb): swap modernc.org/sqlite to ncruces/go-sqlite3 with sqlite-vec
```

---

### Task 2: host-cli — new schema with chunked embeddings and vec0

**Files:**
- Modify: `host-cli/internal/searchdb/db.go:34-68` (schema constant)
- Modify: `host-cli/internal/searchdb/db.go:238-392` (embedding methods)
- Test: `host-cli/internal/searchdb/db_test.go`

**Step 1: Write tests for new schema and methods**

Create/update `host-cli/internal/searchdb/db_test.go` with tests for:
- `InsertChunkEmbeddings` — insert multiple chunks per article, verify UNIQUE constraint on (article_id, chunk_index)
- `DeleteAllEmbeddings` — verify both embeddings and vec_chunks are cleared
- `EmbeddedArticleCount` — count distinct article_ids
- `UnembeddedArticlesBySource` — returns articles with zero chunks

**Step 2: Run tests to verify they fail**

Run: `cd host-cli && go test ./internal/searchdb/ -v -run TestChunk`
Expected: FAIL (methods don't exist yet)

**Step 3: Update schema constant**

Replace the schema constant (lines 34-68) with:

```go
const schema = `
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);

CREATE TABLE IF NOT EXISTS sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    indexed_at TEXT
);

CREATE TABLE IF NOT EXISTS articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    sections TEXT
);

CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source_id);

CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(title, body, content='articles', content_rowid='id');

CREATE TRIGGER IF NOT EXISTS articles_ai AFTER INSERT ON articles BEGIN
    INSERT INTO articles_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
END;

CREATE TRIGGER IF NOT EXISTS articles_ad AFTER DELETE ON articles BEGIN
    INSERT INTO articles_fts(articles_fts, rowid, title, body) VALUES('delete', old.id, old.title, old.body);
END;

CREATE TABLE IF NOT EXISTS embeddings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id INTEGER NOT NULL REFERENCES articles(id),
    chunk_index INTEGER NOT NULL,
    chunk_header TEXT NOT NULL,
    vector BLOB NOT NULL,
    UNIQUE(article_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_embeddings_article ON embeddings(article_id);
`
```

Note: the `vec_chunks` vec0 virtual table is created separately after `Open()` because it requires the sqlite-vec extension loaded. Add a `ensureVecTable` method:

```go
func (d *DB) ensureVecTable(dims int) error {
    _, err := d.db.Exec(fmt.Sprintf(
        `CREATE VIRTUAL TABLE IF NOT EXISTS vec_chunks USING vec0(embedding float[%d])`, dims))
    return err
}
```

**Step 4: Replace EmbeddingPair with ChunkEmbedding and rewrite methods**

```go
type ChunkEmbedding struct {
    ArticleID  int64
    ChunkIndex int
    Header     string
    Vector     []byte
}
```

- `InsertChunkEmbeddings(chunks []ChunkEmbedding) error` — in a single transaction, insert into both `embeddings` and `vec_chunks` (using embeddings.id as rowid for vec_chunks). Batch 1000+ rows per tx.
- `DeleteAllEmbeddings()` — delete from both `embeddings` and `vec_chunks`.
- `EmbeddedArticleCount() (int64, error)` — `SELECT COUNT(DISTINCT article_id) FROM embeddings`
- Update `UnembeddedArticlesBySource` — `LEFT JOIN embeddings e ON e.article_id = a.id WHERE e.article_id IS NULL` (same logic, still works with 1:many)
- Update `EmbeddingCountBySource` — `SELECT COUNT(DISTINCT e.article_id)` instead of `COUNT(*)`
- Keep `EmbeddingDims()` as-is (reads from first vector)

**Step 5: Update Article struct to include Sections**

In the `Article` struct (line 18-23), add:
```go
type Article struct {
    Path     string
    Title    string
    Body     string
    Sections string // JSON: [{"heading":"","body":"..."}]
}
```

Update `InsertArticles` prepared statement to include the sections column.

**Step 6: Add migration detection in Open()**

After schema creation, check for old-style embeddings table and drop if needed:

```go
var hasChunkIndex int
d.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('embeddings') WHERE name='chunk_index'`).Scan(&hasChunkIndex)
if hasChunkIndex == 0 {
    d.db.Exec("DROP TABLE IF EXISTS embeddings")
    // Re-run the embeddings portion of schema
}
```

**Step 7: Run tests**

Run: `cd host-cli && go test ./internal/searchdb/ -v`
Expected: PASS

**Step 8: Commit**

```
feat(searchdb): chunked embeddings schema with vec0 virtual table
```

---

### Task 3: host-cli — ExtractSections in zimext

**Files:**
- Modify: `host-cli/internal/zimext/text.go` (add ExtractSections)
- Test: `host-cli/internal/zimext/text_test.go`

**Step 1: Write tests**

```go
func TestExtractSections(t *testing.T) {
    html := `<p>Intro paragraph.</p>
<h2>First Section</h2>
<p>Content of first section.</p>
<h3>Subsection</h3>
<p>Subsection content.</p>
<h2>Second Section</h2>
<p>Content of second.</p>`

    sections := ExtractSections(html)

    if len(sections) != 4 {
        t.Fatalf("got %d sections, want 4", len(sections))
    }
    if sections[0].Heading != "" {
        t.Errorf("intro heading = %q, want empty", sections[0].Heading)
    }
    if !strings.Contains(sections[0].Body, "Intro paragraph") {
        t.Errorf("intro body missing content")
    }
    if sections[1].Heading != "First Section" {
        t.Errorf("section 1 heading = %q", sections[1].Heading)
    }
    if sections[3].Heading != "Second Section" {
        t.Errorf("section 3 heading = %q", sections[3].Heading)
    }
}

func TestExtractSectionsNoHeadings(t *testing.T) {
    html := `<p>Just a paragraph.</p>`
    sections := ExtractSections(html)
    if len(sections) != 1 || sections[0].Heading != "" {
        t.Errorf("expected single section with empty heading")
    }
}
```

**Step 2: Run to verify fail**

Run: `cd host-cli && go test ./internal/zimext/ -v -run TestExtractSections`
Expected: FAIL

**Step 3: Implement ExtractSections**

```go
type Section struct {
    Heading string `json:"heading"`
    Body    string `json:"body"`
}

var headingRe = regexp.MustCompile(`(?i)<h[23][^>]*>(.*?)</h[23]>`)

func ExtractSections(htmlContent string) []Section {
    locs := headingRe.FindAllStringIndex(htmlContent, -1)
    if len(locs) == 0 {
        body := StripHTML(htmlContent)
        if body == "" {
            return nil
        }
        return []Section{{Body: body}}
    }

    var sections []Section

    // Content before first heading
    if locs[0][0] > 0 {
        intro := StripHTML(htmlContent[:locs[0][0]])
        if intro != "" {
            sections = append(sections, Section{Body: intro})
        }
    }

    for i, loc := range locs {
        match := headingRe.FindStringSubmatch(htmlContent[loc[0]:loc[1]])
        heading := StripHTML(match[1])

        var bodyHTML string
        if i+1 < len(locs) {
            bodyHTML = htmlContent[loc[1]:locs[i+1][0]]
        } else {
            bodyHTML = htmlContent[loc[1]:]
        }
        body := StripHTML(bodyHTML)

        sections = append(sections, Section{Heading: heading, Body: body})
    }

    return sections
}
```

**Step 4: Run tests**

Run: `cd host-cli && go test ./internal/zimext/ -v`
Expected: PASS

**Step 5: Commit**

```
feat(zimext): add ExtractSections for heading-aware HTML splitting
```

---

### Task 4: host-cli — integrate sections into extraction pipeline

**Files:**
- Modify: `host-cli/internal/zimext/extractor.go:47-60` (extract sections from raw HTML)
- Modify: `host-cli/internal/searchdb/db.go` (InsertArticles includes sections)
- Modify: `host-cli/internal/commands/index.go` (no change needed if Article struct updated)

**Step 1: Update ExtractArticles**

In `extractor.go`, between reading content (line 47) and stripping HTML (line 52), extract sections:

```go
content, err := entry.ReadContent()
if err != nil {
    return nil, "", fmt.Errorf("read %s: %w", entry.FullPath(), err)
}

// Extract sections from raw HTML before stripping.
sections := ExtractSections(string(content))
var sectionsJSON string
if len(sections) > 0 {
    if data, err := json.Marshal(sections); err == nil {
        sectionsJSON = string(data)
    }
}

body := StripHTML(string(content))
if body == "" {
    continue
}

articles = append(articles, searchdb.Article{
    Path:     "/" + strings.TrimLeft(entry.Path(), "/"),
    Title:    strings.TrimSpace(entry.Title()),
    Body:     TruncateText(body, maxArticleBodyChars),
    Sections: sectionsJSON,
})
```

Add `"encoding/json"` to imports.

**Step 2: Update InsertArticles in searchdb**

Change the INSERT statement to include sections:

```go
stmt, err := tx.Prepare("INSERT INTO articles (source_id, path, title, body, sections) VALUES (?, ?, ?, ?, ?)")
```

And the exec call:

```go
stmt.Exec(sourceID, a.Path, a.Title, a.Body, a.Sections)
```

**Step 3: Build and test**

Run: `cd host-cli && go build ./... && go test ./... -v`
Expected: PASS

**Step 4: Commit**

```
feat(index): store section-aware HTML structure in articles table
```

---

### Task 5: host-cli — EmbeddingSpec gains Dims, update recipes

**Files:**
- Modify: `host-cli/internal/catalog/catalog.go:48-51` (add Dims field)
- Modify: `host-cli/internal/apply/apply.go` (sidecar includes dims)
- Modify: `recipes/models/nomic-embed-text-v1.5.yaml` (add dims: 256)
- Modify: `host-cli/internal/catalog/embedded/recipes/models/nomic-embed-text-v1.5.yaml` (add dims: 256)

**Step 1: Add Dims to EmbeddingSpec**

```go
type EmbeddingSpec struct {
    DocPrefix   string `yaml:"doc_prefix,omitempty"   json:"doc_prefix,omitempty"`
    QueryPrefix string `yaml:"query_prefix,omitempty" json:"query_prefix,omitempty"`
    Dims        int    `yaml:"dims,omitempty"          json:"dims,omitempty"`
}
```

**Step 2: Update recipe YAMLs**

Both `recipes/models/nomic-embed-text-v1.5.yaml` and embedded copy — add `dims: 256`:

```yaml
embedding:
  doc_prefix: "search_document: "
  query_prefix: "search_query: "
  dims: 256
```

**Step 3: Build and test**

Run: `cd host-cli && go build ./... && go test ./internal/catalog/ -v`
Expected: PASS

**Step 4: Commit**

```
feat(catalog): add dims field to EmbeddingSpec, set Nomic to 256
```

---

### Task 6: host-cli — truncateDims helper

**Files:**
- Modify: `host-cli/internal/embedder/embedder.go` (add truncateDims)
- Test: `host-cli/internal/embedder/embedder_test.go`

**Step 1: Write test**

```go
func TestTruncateDims(t *testing.T) {
    vec := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}
    result := TruncateDims(vec, 3)
    if len(result) != 3 {
        t.Fatalf("len = %d, want 3", len(result))
    }
    // Check L2 normalized
    var sum float64
    for _, v := range result {
        sum += float64(v) * float64(v)
    }
    if math.Abs(sum-1.0) > 1e-6 {
        t.Errorf("not L2 normalized: magnitude^2 = %f", sum)
    }
}

func TestTruncateDimsNoop(t *testing.T) {
    vec := []float32{0.6, 0.8}
    result := TruncateDims(vec, 5)
    if len(result) != 2 {
        t.Fatalf("should not grow: len = %d", len(result))
    }
}
```

**Step 2: Implement**

```go
// TruncateDims truncates a vector to the first dims elements and L2-normalizes.
// If the vector is shorter than dims, it is normalized but not grown.
func TruncateDims(vec []float32, dims int) []float32 {
    if dims > 0 && dims < len(vec) {
        vec = vec[:dims]
    }
    var norm float64
    for _, v := range vec {
        norm += float64(v) * float64(v)
    }
    if norm > 0 {
        norm = math.Sqrt(norm)
        for i := range vec {
            vec[i] = float32(float64(vec[i]) / norm)
        }
    }
    return vec
}
```

**Step 3: Run tests**

Run: `cd host-cli && go test ./internal/embedder/ -v -run TestTruncateDims`
Expected: PASS

**Step 4: Commit**

```
feat(embedder): add TruncateDims for matryoshka dimension reduction
```

---

### Task 7: host-cli — buildChunks and multi-chunk embedding pipeline

**Files:**
- Modify: `host-cli/internal/commands/index_semantic.go` (replace prepareEmbeddingText with buildChunks, update embedSource)
- Test: `host-cli/internal/commands/index_semantic_test.go`

**Step 1: Write tests for buildChunks**

```go
func TestBuildChunksSingleSection(t *testing.T) {
    sections := `[{"heading":"","body":"Short article body."}]`
    chunks := buildChunks("search_document: ", "Test Article", sections)
    if len(chunks) != 1 {
        t.Fatalf("got %d chunks, want 1", len(chunks))
    }
    if chunks[0].Header != "Test Article" {
        t.Errorf("header = %q", chunks[0].Header)
    }
    if !strings.HasPrefix(chunks[0].Text, "search_document: ") {
        t.Errorf("missing doc prefix")
    }
}

func TestBuildChunksMultipleSections(t *testing.T) {
    sections := `[{"heading":"","body":"Intro text."},{"heading":"Section A","body":"` +
        strings.Repeat("word ", 200) + `"},{"heading":"Section B","body":"More content."}]`
    chunks := buildChunks("search_document: ", "Title", sections)
    if len(chunks) < 2 {
        t.Fatalf("expected multiple chunks, got %d", len(chunks))
    }
    // Check that section headings appear in chunk headers
    found := false
    for _, c := range chunks {
        if strings.Contains(c.Header, "Section A") {
            found = true
        }
    }
    if !found {
        t.Error("no chunk header contains 'Section A'")
    }
}

func TestBuildChunksEmptySections(t *testing.T) {
    // Fallback when no sections JSON
    chunks := buildChunks("search_document: ", "Title", "")
    if len(chunks) != 0 {
        t.Errorf("expected 0 chunks for empty sections, got %d", len(chunks))
    }
}
```

**Step 2: Implement buildChunks**

```go
type Chunk struct {
    Header string
    Text   string
}

func buildChunks(docPrefix, title, sectionsJSON string) []Chunk {
    if sectionsJSON == "" {
        return nil
    }

    var sections []struct {
        Heading string `json:"heading"`
        Body    string `json:"body"`
    }
    if err := json.Unmarshal([]byte(sectionsJSON), &sections); err != nil || len(sections) == 0 {
        return nil
    }

    // Merge small sections, split large ones
    type block struct {
        heading string
        body    string
    }
    var blocks []block
    var pending block
    for _, s := range sections {
        body := strings.TrimSpace(s.Body)
        if body == "" {
            continue
        }
        heading := s.Heading
        if pending.body == "" {
            pending = block{heading: heading, body: body}
            continue
        }
        words := len(strings.Fields(pending.body))
        if words < 80 {
            // Merge with current
            if heading != "" && pending.heading == "" {
                pending.heading = heading
            }
            pending.body += " " + body
            continue
        }
        blocks = append(blocks, pending)
        pending = block{heading: heading, body: body}
    }
    if pending.body != "" {
        blocks = append(blocks, pending)
    }

    // Split oversized blocks at paragraph boundaries
    var finalBlocks []block
    for _, b := range blocks {
        words := len(strings.Fields(b.body))
        if words <= 500 {
            finalBlocks = append(finalBlocks, b)
            continue
        }
        // Split at paragraph boundaries
        paras := strings.Split(b.body, "\n\n")
        var current block
        current.heading = b.heading
        for _, p := range paras {
            p = strings.TrimSpace(p)
            if p == "" {
                continue
            }
            if current.body == "" {
                current.body = p
                continue
            }
            if len(strings.Fields(current.body)) + len(strings.Fields(p)) > 500 {
                finalBlocks = append(finalBlocks, current)
                current = block{heading: b.heading, body: p}
            } else {
                current.body += "\n\n" + p
            }
        }
        if current.body != "" {
            finalBlocks = append(finalBlocks, current)
        }
    }

    // Build chunks
    chunks := make([]Chunk, 0, len(finalBlocks))
    for _, b := range finalBlocks {
        header := title
        if b.heading != "" {
            header = title + " > " + b.heading
        }
        text := docPrefix + header + ": " + b.body
        chunks = append(chunks, Chunk{Header: header, Text: text})
    }
    return chunks
}
```

**Step 3: Update embedSource to use buildChunks**

In `embedSource` (line 163), change the loop to:
- Read `article.Sections` JSON for each article in the batch
- Call `buildChunks()` for each article
- If no chunks (empty sections), fall back to single-chunk using old `prepareEmbeddingText` logic with just title+body
- Collect all chunk texts for the batch, embed them, then store with `InsertChunkEmbeddings`
- Update progress tracking to use `EmbeddedArticleCount` (distinct articles) instead of raw embedding count

Also read `embSpec.Dims` from the sidecar and call `embedder.TruncateDims()` on each vector before storage.

Update `UnembeddedArticle` struct to include Sections field (add to searchdb query).

**Step 4: Keep prepareEmbeddingText as fallback**

Rename to `prepareFallbackChunk` — used when sections JSON is empty (articles indexed before section extraction was added, or non-HTML content).

**Step 5: Run tests**

Run: `cd host-cli && go test ./internal/commands/ -v`
Expected: PASS

**Step 6: Commit**

```
feat(index): section-aware chunking with multi-chunk embedding pipeline
```

---

### Task 8: drive-runtime — shared search engine package

**Files:**
- Create: `drive-runtime/internal/search/engine/engine.go`
- Create: `drive-runtime/internal/search/engine/engine_test.go`

**Step 1: Write tests**

Test in isolation using an in-memory ncruces DB with the schema from searchdb:

```go
func TestKeywordSearch(t *testing.T) {
    // Create in-memory DB with schema, insert test articles, run keyword search
}

func TestRRFMerge(t *testing.T) {
    // Unit test RRF with known inputs
}

func TestApplyMMR(t *testing.T) {
    // Unit test MMR with known vectors
}

func TestHybridSearch(t *testing.T) {
    // Integration test: insert articles + vec_chunks, run hybrid search
}
```

**Step 2: Implement engine.go**

Move `BuildFTSQuery`, `dotProduct` from `search.go` into the engine package. Add:

```go
package engine

type Result struct {
    ArticleID   int
    Filename    string
    Path        string
    Title       string
    Snippet     string
    ChunkHeader string
    Score       float64
}

type Engine struct {
    db *sql.DB
}

func New(db *sql.DB) *Engine

func (e *Engine) Keyword(query string, limit int) ([]Result, error)
    // FTS5 MATCH query, same SQL as current keywordSearch

func (e *Engine) Hybrid(query string, queryVec []float32, limit int) ([]Result, error)
    // 1. Launch FTS5 and KNN in parallel goroutines
    // 2. RRF merge
    // 3. Dedup to best chunk per article (after RRF)
    // 4. MMR diversity
    // 5. Fetch article details

func (e *Engine) EnsureVecTable(dims int) error
    // CREATE VIRTUAL TABLE IF NOT EXISTS vec_chunks ...

// Internal helpers
func rrfMerge(ftsResults, vecResults []scoredResult, k float64) []scoredResult
func applyMMR(candidates []scoredResult, vectors map[int][]float32, lambda float64, limit int) []scoredResult
func dotProduct(a, b []float32) float64
func BuildFTSQuery(query string) string
```

**Step 3: Run tests**

Run: `cd drive-runtime && go test ./internal/search/engine/ -v`
Expected: PASS

**Step 4: Commit**

```
feat(search): add shared search engine with hybrid RRF+MMR ranking
```

---

### Task 9: drive-runtime — swap modernc → ncruces + sqlite-vec

**Files:**
- Modify: `drive-runtime/go.mod`
- Modify: `drive-runtime/internal/mcp/searchdb.go:7-9` (import swap)

**Step 1: Update go.mod**

```bash
cd drive-runtime
go get github.com/ncruces/go-sqlite3@latest
go get github.com/ncruces/go-sqlite3/driver@latest
go get github.com/asg017/sqlite-vec-go-bindings/ncruces@latest
```

Remove `modernc.org/sqlite`. Run `go mod tidy`.

**Step 2: Swap imports in mcp/searchdb.go**

Replace `_ "modernc.org/sqlite"` with:
```go
_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
_ "github.com/ncruces/go-sqlite3/driver"
```

Change `sql.Open("sqlite", ...)` to `sql.Open("sqlite3", ...)` in `openSearchDB`.

**Step 3: Build and test**

Run: `cd drive-runtime && go build ./... && go test ./... -v`
Expected: PASS

**Step 4: Commit**

```
feat(drive): swap modernc.org/sqlite to ncruces/go-sqlite3 with sqlite-vec
```

---

### Task 10: drive-runtime — integrate engine into interactive search

**Files:**
- Modify: `drive-runtime/internal/search/search.go` (use engine, remove sqlite3 shell-out for search)
- Modify: `drive-runtime/internal/search/session.go` (ModeHybrid, use engine)

**Step 1: Add engine to Session**

In `session.go`, add `*engine.Engine` field to `Session`. In `NewSession`, open the search.db via ncruces and create an engine:

```go
db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
s.engine = engine.New(db)
```

**Step 2: Replace ModeSemantic with ModeHybrid**

Change constants:
```go
const (
    ModeKeyword Mode = "keyword"
    ModeHybrid  Mode = "hybrid"
)
```

Update `BestMode`: return `ModeHybrid` when embedding conditions met.

**Step 3: Update search routing**

In `Session.Search()` or the `Run()` loop:
- `ModeHybrid`: embed query via `embedQuery()`, call `engine.Hybrid(query, vec, limit)`
- `ModeKeyword`: call `engine.Keyword(query, limit)`

Remove the old `semanticSearch()`, `keywordSearch()` functions that shell out to sqlite3 (keep `embedQuery` for now).

**Step 4: Update UI toggles**

Replace `/sem` with `/full` or `/hybrid`. Replace `/fts` with `/keyword`.

**Step 5: Build and test**

Run: `cd drive-runtime && go build ./... && go test ./internal/search/ -v`
Expected: PASS

**Step 6: Commit**

```
feat(search): use shared engine with hybrid RRF+MMR, drop sqlite3 shell-out for search
```

---

### Task 11: drive-runtime — MCP hybrid search with embedding server

**Files:**
- Modify: `drive-runtime/internal/mcp/search.go` (ensureEmbedServer, use engine)
- Modify: `drive-runtime/internal/mcp/searchdb.go` (replace keywordSearch with engine)

**Step 1: Add engine + embedding server to SearchCapability**

```go
type SearchCapability struct {
    driveRoot string
    meta      DriveMetadata

    dbOnce sync.Once
    db     *sql.DB
    dbErr  error
    eng    *engine.Engine

    embedOnce sync.Once
    embedCmd  *exec.Cmd
    embedPort int
    embedErr  error
    queryPfx  string

    kiwixOnce sync.Once
    // ... existing kiwix fields
}
```

**Step 2: Add ensureEmbedServer**

Same `sync.Once` pattern as `ensureKiwix`:
```go
func (c *SearchCapability) ensureEmbedServer() error {
    c.embedOnce.Do(func() {
        // Find llama-server binary and embedding model
        // Start on available port, poll health
        // Read query prefix from DB meta
    })
    return c.embedErr
}
```

**Step 3: Update handleSearch**

```go
func (c *SearchCapability) handleSearch(_ context.Context, params map[string]any) (ActionResult, error) {
    eng, err := c.getEngine()
    // ...
    // Try hybrid if embedding server available
    if c.ensureEmbedServer() == nil {
        queryVec, err := embedQuery(c.queryPfx + query, c.embedPort)
        if err == nil {
            results, err = eng.Hybrid(query, queryVec, limit)
        }
    }
    // Fallback to keyword
    if results == nil {
        results, err = eng.Keyword(query, limit)
    }
}
```

**Step 4: Remove old searchDB wrapper**

The engine replaces `searchDB.keywordSearch`. Keep `readArticle` as a standalone function or method on the capability (it's not search logic).

**Step 5: Build and test**

Run: `cd drive-runtime && go build ./... && go test ./internal/mcp/ -v`
Expected: PASS

**Step 6: Commit**

```
feat(mcp): hybrid search with lazy embedding server via shared engine
```

---

### Task 12: Cleanup — delete MiniLM, update presets

**Files:**
- Delete: `recipes/models/all-minilm-l6-v2.yaml`
- Delete: `host-cli/internal/catalog/embedded/recipes/models/all-minilm-l6-v2.yaml`
- Modify: `host-cli/internal/catalog/embedded/presets/test-1gb.yaml:20`
- Modify: `host-cli/internal/catalog/embedded/presets/test-runtime-2gb.yaml:16`
- Modify: `host-cli/internal/catalog/embedded/presets/packs/tools-base.yaml:13`

**Step 1: Delete MiniLM recipe files**

```bash
rm recipes/models/all-minilm-l6-v2.yaml
rm host-cli/internal/catalog/embedded/recipes/models/all-minilm-l6-v2.yaml
```

**Step 2: Update presets**

In all three preset files, replace `all-minilm-l6-v2` with `nomic-embed-text-v1.5`.

**Step 3: Remove dead code**

- Remove `ModeSemantic` constant and `/sem` handler from search.go
- Remove old `semanticSearch`, `keywordSearch` functions from search.go (if not already removed in Task 10)
- Remove `pythonKeywordSearch` fallback (no longer shelling out)
- Remove `runSQLite`, `queryResults`, `scalarInt` functions (no longer shelling out for search)
- Remove `DecodeVectorHex` (vectors now handled by sqlite-vec in-process)
- Clean up unused imports

**Step 4: Build and test everything**

```bash
cd host-cli && go build ./... && go test ./...
cd ../drive-runtime && go build ./... && go test ./...
```

Expected: PASS

**Step 5: Commit**

```
chore: drop MiniLM, update presets to Nomic, remove dead semantic-only code
```

---

### Task 13: End-to-end verification

**Step 1: Build both binaries**

```bash
cd host-cli && go build -o svalbard ./cmd/svalbard
cd ../drive-runtime && go build -o svalbard-drive ./cmd/svalbard-drive
```

**Step 2: Index a test vault**

```bash
./svalbard --vault /tmp/svalbard-test index
./svalbard --vault /tmp/svalbard-test index --semantic
```

Verify:
- Sections JSON populated in articles table
- Multiple embedding rows per article
- vec_chunks virtual table populated
- Meta table has correct model ID, dims=256, prefixes

**Step 3: Test search**

```bash
./svalbard-drive --root /tmp/svalbard-test search
```

Verify:
- Hybrid mode auto-detected
- FTS5 + vector results merged
- Chunk headers shown in results
- `/keyword` toggle works

**Step 4: Test MCP search**

Verify MCP search auto-detects hybrid mode and returns results with chunk context.
