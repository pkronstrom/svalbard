# Search Accuracy & RAG Improvements

**Date:** 2026-04-18
**Branch:** go-rewrite
**Status:** Design complete, not yet implemented

## Goal

Improve semantic search accuracy and enable RAG (manual + MCP) by:
1. Chunking articles into sections instead of one-vector-per-article
2. Hybrid FTS5 + vector search with RRF fusion (replace either/or)
3. Nomic at 256 matryoshka dims (drop MiniLM)
4. MMR diversity filtering
5. In-process vector search via ncruces/go-sqlite3 + sqlite-vec

**Supersedes:** This plan replaces Task 6 of `2026-04-17-swappable-embedding-models.md` (which made MiniLM the default). MiniLM is dropped entirely due to its 256-token context limit being incompatible with chunking.

## Key Decisions

- **Model:** Nomic Embed Text v1.5 at 256 matryoshka dims. MiniLM dropped (256-token context too small for chunks, no matryoshka, no prefixes).
- **Search modes:** Keyword (FTS5 only) and Hybrid (FTS5 + vector + RRF). No standalone semantic mode — if vectors are available, always combine with FTS5.
- **SQLite library:** Swap `modernc.org/sqlite` to `ncruces/go-sqlite3` everywhere (host-cli, drive-runtime, MCP). Pure Go (Wasm), no CGo, supports sqlite-vec. Driver name: `"sqlite3"`. Migrate host-cli first, then drive-runtime, to limit blast radius.
- **sqlite3 CLI binary:** Stays on the drive for external tools. No longer used for search.
- **Chunking strategy:** Section-aware splitting from HTML headings. Sections stored as JSON in articles table. Chunking happens at embedding time only — FTS5 indexes the full flat body unchanged.
- **Shared search engine:** Extract hybrid search logic (RRF, MMR, KNN) into a shared package under `drive-runtime/internal/search/engine/` used by both the interactive CLI and MCP. Avoid duplicating ranking logic across two code paths.

## 1. Dependencies

### Drop
- `modernc.org/sqlite` (host-cli and drive-runtime)
- `recipes/models/all-minilm-l6-v2.yaml` + `host-cli/internal/catalog/embedded/recipes/models/all-minilm-l6-v2.yaml`
- MiniLM references in presets: `presets/test-1gb.yaml`, `presets/test-runtime-2gb.yaml`, `presets/packs/tools-base.yaml` — replace with `nomic-embed-text-v1.5`

### Add
- `github.com/ncruces/go-sqlite3` + `github.com/ncruces/go-sqlite3/driver`
- `github.com/asg017/sqlite-vec-go-bindings/ncruces`

All `sql.Open("sqlite", ...)` calls change to `sql.Open("sqlite3", ...)`.

## 2. Schema Changes

```sql
-- articles table: add sections column
ALTER TABLE articles ADD COLUMN sections TEXT;  -- JSON: [{"heading":"","body":"..."},...]

-- Replace embeddings table (1:many with articles, was 1:1)
DROP TABLE IF EXISTS embeddings;

CREATE TABLE IF NOT EXISTS embeddings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id INTEGER NOT NULL REFERENCES articles(id),
    chunk_index INTEGER NOT NULL,
    chunk_header TEXT NOT NULL,
    vector BLOB NOT NULL,
    UNIQUE(article_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_embeddings_article ON embeddings(article_id);

-- sqlite-vec virtual table for KNN (rowids match embeddings.id)
CREATE VIRTUAL TABLE IF NOT EXISTS vec_chunks USING vec0(
    embedding float[256]
);
```

**Migration:** Detect old schema (embeddings table without chunk_index), drop and recreate. Model change detection already triggers re-embedding — same flow.

**1:many rework:** The following DB methods assume 1:1 article↔embedding and need updating:
- `InsertEmbeddings` → `InsertChunkEmbeddings` (multiple per article)
- `UnembeddedArticles` / `UnembeddedArticlesBySource` → query articles that have zero chunks (not just missing from embeddings)
- `EmbeddingCountBySource` → count distinct article_ids in embeddings, not raw row count
- `EmbeddingCount` → report chunk count, but progress tracking should compare distinct embedded articles vs total articles
- `DeleteAllEmbeddings` → also clear `vec_chunks`

**Storage estimate:** 256 dims = 1024 bytes/vector. ~3 chunks/article average. ~3.4 KB/article vs current 3.1 KB (768d single vector). Roughly flat.

## 3. ZIM Extraction

New function in `zimext/text.go`:

```go
type Section struct {
    Heading string `json:"heading"`  // "" for intro/lead
    Body    string `json:"body"`
}

func ExtractSections(htmlContent string) []Section
```

- Split HTML on `<h2>` and `<h3>` tags
- Extract heading text from the tag
- StripHTML each section body independently
- Content before first heading → Section with empty Heading

`ExtractArticles` changes: call `ExtractSections` on raw HTML before stripping. Store result as JSON in `Article.Sections`. The flat `Body` stays as-is (all sections concatenated, for FTS5).

## 4. Chunk Assembly

In `index_semantic.go`, replace `prepareEmbeddingText` with:

```go
type Chunk struct {
    Header string  // "Article Title" or "Article Title > Section Heading"
    Text   string  // full embedding input with doc prefix
}

func buildChunks(docPrefix, title string, sections []zimext.Section) []Chunk
```

Rules:
- Sections < ~80 tokens: merge with next section
- Sections > ~500 tokens: split at paragraph boundaries (\n\n)
- Target: 300-500 tokens per chunk
- Embedding text format: `search_document: {chunk_header}: {chunk_body}`
- Articles with no/empty sections: single chunk (same as today's behavior)

After embedding (768d from Nomic), truncate to 256 dims and L2-normalize:

```go
func truncateDims(vec []float32, dims int) []float32
```

Recipe YAML gains dims field:
```yaml
embedding:
  doc_prefix: "search_document: "
  query_prefix: "search_query: "
  dims: 256
```

## 5. Embedding Storage

Write to both relational table and vec0 virtual table in one transaction:

Extend the existing `EmbeddingPair` struct with chunk fields (rename to `ChunkEmbedding`):

```go
type ChunkEmbedding struct {
    ArticleID  int64
    ChunkIndex int
    Header     string
    Vector     []byte  // 256 x float32 = 1024 bytes
}

func (d *DB) InsertChunkEmbeddings(chunks []ChunkEmbedding) error {
    // In a single transaction (batch 1000+ rows per tx):
    // 1. INSERT INTO embeddings → get rowid
    // 2. INSERT INTO vec_chunks(rowid, embedding) VALUES(?, ?)
}
```

Delete the old `EmbeddingPair` struct and `InsertEmbeddings` method — they are fully replaced.

## 6. Hybrid Search with RRF

Extract core search logic into a shared package (`drive-runtime/internal/search/engine/`). Extend the existing `mcp/searchDB` pattern rather than creating a parallel struct — the engine wraps `*sql.DB` (ncruces with vec) and provides both keyword and hybrid search:

```go
package engine

type Engine struct {
    db *sql.DB // ncruces sqlite with vec
}

func New(db *sql.DB) *Engine
func (e *Engine) Hybrid(query string, queryVec []float32, limit int) ([]Result, error)
func (e *Engine) Keyword(query string, limit int) ([]Result, error)
```

Query embedding is the caller's responsibility (pass `queryVec` in). This keeps the engine free of llama-server lifecycle concerns. Both the interactive search (`search.go`) and MCP (`search.go`) use this engine. Existing helpers (`dotProduct`, `BuildFTSQuery`, `DecodeVectorHex`) move into the engine package.

Replace ModeSemantic with ModeHybrid. When embedding server is available, always run both:

```
1. FTS5 keyword search → top 100 results  }  run in parallel
2. KNN via vec_chunks → top 100 chunks    }  (goroutines, WAL allows concurrent reads)
3. RRF merge: score(doc) = 1/(60+rank_fts) + 1/(60+rank_vec)
4. Deduplicate to best chunk per article (after RRF, not before — preserves ranking signal)
5. Sort by RRF score → top 40 candidates
6. MMR diversity → final top N
7. Fetch article details + chunk_header for snippets
```

KNN query (in-process via ncruces):
```sql
SELECT rowid, distance FROM vec_chunks
WHERE embedding MATCH ? ORDER BY distance LIMIT 100
```

Join embeddings on rowid to get article_id and chunk_header.

**Scale path (>50K chunks):** Pre-filter with FTS5 article IDs, then scan only those articles' chunks. If sqlite-vec doesn't support WHERE filters alongside MATCH, fall back to: FTS5 → article IDs → SELECT vectors WHERE article_id IN (...) → dot product in Go for filtered set.

**Fallback:** No embedding server → keyword-only (current behavior, no change).

## 7. MMR Diversity

Post-RRF, before returning final results:

```go
func applyMMR(candidates []rankedResult, lambda float64, limit int) []rankedResult
```

- lambda = 0.7 (favor relevance)
- Greedy selection: pick best score, then iteratively pick candidate maximizing lambda*score - (1-lambda)*maxSimilarityToSelected
- Uses dot product between candidate vectors (already in memory from KNN)
- Prevents multiple sections from same topic dominating results

## 8. MCP Integration

`drive-runtime/internal/mcp/search.go` uses the shared `Engine` from section 6. The `SearchCapability` owns an `Engine` instance and handles the embedding server lifecycle separately:

**Embedding server lifecycle for MCP:** Add `ensureEmbedServer` (same `sync.Once` pattern as `ensureKiwix`):
1. Locates llama-server binary and embedding model
2. Starts on an available port, polls health
3. On hybrid search: embed query via HTTP, pass vector to `Engine.Hybrid(query, queryVec, limit)`
4. Kills on `SearchCapability.Close()`

The interactive search (`search.go`) does the same — own embed server lifecycle, passes query vector to engine. Both use `embedQuery` from `search/` package (consolidate the existing function with `embedder.Server.EmbedBatch` — use the simpler single-query version for drive-side, keep retry logic for indexing).

Query prefix read from DB meta (already stored). MCP search action auto-detects: if embedding model exists on drive and embeddings table has data → hybrid, otherwise → keyword.

## 9. Files Changed

| File | Change |
|---|---|
| `host-cli/go.mod` | modernc → ncruces + sqlite-vec-go-bindings |
| `drive-runtime/go.mod` | modernc → ncruces + sqlite-vec-go-bindings |
| `host-cli/internal/searchdb/db.go` | Schema, InsertChunkEmbeddings, vec0 table, driver swap |
| `host-cli/internal/zimext/extractor.go` | Store sections JSON in Article |
| `host-cli/internal/zimext/text.go` | New ExtractSections (heading-aware HTML split) |
| `host-cli/internal/commands/index.go` | Pass sections to article storage |
| `host-cli/internal/commands/index_semantic.go` | buildChunks, multi-chunk pipeline |
| `host-cli/internal/embedder/embedder.go` | truncateDims helper (vector dimension truncation + L2 normalize) |
| `host-cli/internal/catalog/catalog.go` | EmbeddingSpec gains Dims field |
| `recipes/models/nomic-embed-text-v1.5.yaml` | Add dims: 256 |
| `host-cli/internal/catalog/embedded/recipes/models/nomic-embed-text-v1.5.yaml` | Add dims: 256 |
| `recipes/models/all-minilm-l6-v2.yaml` | Delete |
| `host-cli/internal/catalog/embedded/recipes/models/all-minilm-l6-v2.yaml` | Delete |
| `host-cli/internal/catalog/embedded/presets/test-1gb.yaml` | Replace all-minilm-l6-v2 with nomic-embed-text-v1.5 |
| `host-cli/internal/catalog/embedded/presets/test-runtime-2gb.yaml` | Replace all-minilm-l6-v2 with nomic-embed-text-v1.5 |
| `host-cli/internal/catalog/embedded/presets/packs/tools-base.yaml` | Replace all-minilm-l6-v2 with nomic-embed-text-v1.5 |
| `drive-runtime/internal/search/engine/` | **New** shared search engine (RRF, MMR, KNN, keyword) |
| `drive-runtime/internal/search/search.go` | Use engine, swap to ncruces, ModeHybrid |
| `drive-runtime/internal/search/session.go` | Remove ModeSemantic, update to ModeHybrid |
| `drive-runtime/internal/mcp/searchdb.go` | Use engine, driver swap |
| `drive-runtime/internal/mcp/search.go` | ensureEmbedServer, hybrid mode |
| `host-tui/internal/index/model.go` | UI labels if needed |

## 10. Migration Sequence

The modernc → ncruces swap and schema changes touch both modules. Sequence to limit risk:

1. **host-cli: swap modernc → ncruces + sqlite-vec** — searchdb schema change, chunked embeddings, vec0 table. All indexing tests pass before proceeding.
2. **host-cli: chunking pipeline** — ExtractSections, buildChunks, truncateDims. Verify indexing produces correct chunked embeddings.
3. **drive-runtime: shared search engine** — new `engine/` package with RRF, MMR, KNN via ncruces. Unit testable in isolation.
4. **drive-runtime: swap interactive search** — search.go uses engine, drops sqlite3 shell-out for search queries. ModeHybrid replaces ModeSemantic.
5. **drive-runtime: MCP integration** — searchdb.go uses engine, ensureEmbedServer added to SearchCapability.
6. **Cleanup** — delete MiniLM recipes, update presets, remove dead ModeSemantic code.

## 11. Not Doing

- No HNSW/IVF index — brute-force + FTS5 pre-filter sufficient at scale
- No cross-encoder reranking — needs second model
- No HyDE or query expansion — needs LLM at query time
- No contextual retrieval — needs LLM at index time
- No chunk overlap — no measurable benefit for structured content
- No FTS5 schema change — full article body works fine for keyword
- No FTS5 field boosting — could add later as separate small win
