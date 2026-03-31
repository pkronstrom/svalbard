# Cross-ZIM Search Design

Offline keyword + semantic search across all ZIM files on a Svalbard drive.

## Problem

A Svalbard drive can contain 50+ ZIM files spanning Wikipedia, StackExchange,
iFixit, medical references, and more. Kiwix handles single-ZIM search, but
there is no way to search across the entire archive. A user looking for "audi
2017 motor repair" should see results from iFixit, Wikipedia, and DIY
StackExchange in one ranked list.

## Architecture

```
BUILD MACHINE (Python)                 USB DRIVE (shell + bundled binaries)
────────────────────────               ────────────────────────────────────
svalbard index                         .svalbard/actions/search.sh      (CLI)
  ├─ libzim  → extract text            .svalbard/actions/search-server.sh (REST)
  ├─ sqlite3 → build search.db           ├─ sqlite3 CLI   → query FTS5
  └─ llama-server → embed vectors        ├─ kiwix-serve    → page content
                                          └─ llama-server   → query embedding
Output:
  data/search.db  ──copied to──→       data/search.db
```

Indexing runs on the build machine where Python and svalbard are installed.
Search runs on any machine from the drive using only shell scripts and bundled
binaries (sqlite3, kiwix-serve, llama-server). No Python required at runtime.

## Database Schema

Single file: `data/search.db`

```sql
-- Index metadata and checkpoint state
CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT
);

-- One row per indexed ZIM — enables incremental runs
CREATE TABLE sources (
    id            INTEGER PRIMARY KEY,
    filename      TEXT UNIQUE NOT NULL,
    size_bytes    INTEGER,
    checksum      TEXT,
    article_count INTEGER,
    indexed_at    TEXT
);

-- Extracted content
CREATE TABLE articles (
    id        INTEGER PRIMARY KEY,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    path      TEXT NOT NULL,
    title     TEXT,
    body      TEXT,
    UNIQUE(source_id, path)
);

-- FTS5 index (fast tier creates this)
CREATE VIRTUAL TABLE articles_fts USING fts5(
    title, body,
    content='articles',
    content_rowid='id'
);

-- Semantic tier adds this
CREATE TABLE embeddings (
    article_id INTEGER PRIMARY KEY REFERENCES articles(id),
    vector     BLOB
);
```

## Indexing Strategy Tiers

Each tier builds on the previous — no re-extraction needed when upgrading.

### fast (default)

- Extracts title + first ~500 characters of body text per article
- Builds FTS5 index
- DB overhead: ~1-3% of total ZIM size
- Speed: fast, limited by disk I/O

### standard

- Re-extracts full article body text (UPDATE existing rows)
- Rebuilds FTS5 index with complete content
- DB overhead: ~3-8% of total ZIM size
- Better ranking due to full-text matching

### semantic

- Adds embedding vectors on top of existing fast or standard index
- Uses nomic-embed-text-v1.5 (Q8_0, 140 MB, 384 dims, 8K context)
- FTS5 for recall, embeddings for reranking — never scans full vector table
- DB overhead: additional ~0.5-1 KB per article for vectors
- Speed: ~200-500 embeddings/sec on CPU, hours for millions of articles

<!-- Alternative embedding models for future consideration:
  - snowflake-arctic-embed-l-v2.0 (Q4_K_M 438 MB, 1024 dims, BEIR 55.6, Finnish support)
  - nomic-embed-text-v2-moe (Q4_K_M 328 MB, 768 dims, 100 languages, but only 512 token context)
-->

## Indexing Pipeline

```
svalbard index [--strategy fast|standard|semantic] [--drive PATH]
```

### Flow

1. Scan `zim/` directory on drive
2. Compare against `sources` table:
   - Match by filename + file size + mtime (fast check)
   - If size/mtime changed, verify with checksum
   - New/changed ZIMs are queued for indexing
   - Missing ZIMs trigger a warning (optionally prune)
3. Estimate and confirm:
   ```
   Found 47 ZIM files (186 GB), ~2.1M articles
     12 already indexed, 35 new

   Strategy: fast
     Estimated index size: ~1.2 GB
     Estimated time: ~20 min

   Proceed? [Y/n]
   ```
4. For each new/changed ZIM:
   a. Open with libzim
   b. Iterate articles, skip redirects and metadata entries
   c. Strip HTML tags, collapse whitespace
   d. INSERT into articles + articles_fts
   e. UPDATE sources row
5. If upgrading to standard: re-extract full bodies, rebuild FTS5
6. If upgrading to semantic: embed articles via llama-server (see below)
7. Update `meta` table with tier level and timestamp

### Estimation Heuristics

- Article count: libzim exposes this per ZIM without iteration
- DB size: ~500 bytes/article (fast), ~2 KB (standard), +1 KB (semantic)
- Time: benchmark first ZIM, extrapolate for the rest

### SQLite Bulk Insert Optimization

- `PRAGMA journal_mode=WAL`
- `PRAGMA synchronous=OFF` during build (safe — not serving concurrently)
- Batch inserts in transactions of ~10K rows
- Build FTS5 index after all inserts (faster than incremental)

### Resumable Embedding Builds

The embeddings table is the checkpoint. No separate state file needed.

```sql
-- Find resume point
SELECT MAX(article_id) FROM embeddings;

-- Fetch next batch
SELECT id, title, body FROM articles
WHERE id > :last_embedded_id
ORDER BY id LIMIT 1000;
```

Flow:
1. Check embeddings table — if rows exist, report progress and resume
2. Query remaining articles in batches of 1000
3. POST batch to llama-server `/embedding` endpoint
4. INSERT vectors, COMMIT transaction — each commit is a crash-safe checkpoint
5. Update `meta` with progress: `embed_done`, `embed_total`

Model mismatch detection: `meta.embed_model` records which model was used.
If the model changes between runs, warn and offer to re-embed from scratch.

Embedding via llama-server:
1. Python starts `llama-server --embeddings --port 8085 -m MODEL`
2. Sends batches of 32-64 texts via HTTP POST to `/embedding`
3. Receives normalized vectors (llama-server handles normalization)
4. Stores as float16 BLOB in embeddings table

## Staleness Detection

### At index time (Python)

- Scan `zim/` and compare against `sources` table
- File size + mtime for fast check, checksum for verification
- New files → index. Changed files → re-index. Missing files → warn.
- `meta.tier` tracks current level — knows when upgrade is needed
- `meta.version` tracks schema version — triggers rebuild if format changed

### At search time (drive, shell)

- Count ZIM files vs `sources` rows
- Print warning if mismatch: `"3 ZIM files not in search index"`
- No attempt to fix — just inform the user to run `svalbard index`

## Drive-Side Search

### CLI — `search.sh`

```
.svalbard/actions/search.sh "audi 2017 motor"

Searching 47 sources, 2.1M articles...

 1. [ifixit]       Audi A4 B9 2017 Engine Repair Guide
 2. [wikipedia]    Audi EA888 engine
 3. [diy]          2017 Audi A4 rough idle diagnosis
 4. [wikipedia]    List of Volkswagen Group petrol engines
 5. [ifixit]       Timing chain replacement — VAG 2.0 TFSI

Open result [1-10, q to quit]: 3
→ Opening in kiwix-serve: http://localhost:8080/stackexchange-diy/questions/123456
```

Implementation: sqlite3 CLI query against FTS5, tab-separated output, parsed
in bash. If embeddings are available, a second pass reranks via llama-server
embedding + awk dot product.

```bash
# FTS5 query
sqlite3 "$DB" "
  SELECT a.id, s.filename, a.path, a.title,
         snippet(articles_fts, 1, '>', '<', '...', 20)
  FROM articles_fts
  JOIN articles a ON a.id = articles_fts.rowid
  JOIN sources  s ON s.id = a.source_id
  WHERE articles_fts MATCH '$query'
  ORDER BY rank
  LIMIT 20;
"
```

If semantic tier is available:
```bash
# 1. FTS5 → top 100 candidates
# 2. Embed query via llama-server
# 3. Fetch candidate vectors from embeddings table
# 4. Dot product rerank (vectors are pre-normalized, so dot = cosine)
# 5. Return top 20
```

### REST API — `search-server.sh`

Lightweight HTTP server wrapping sqlite3 queries. Uses socat or busybox httpd
with CGI for the PoC. Upgrade path to a Go binary later.

```
GET /search?q=audi+2017+motor       → JSON array of ranked results
GET /search?q=...&semantic=1         → FTS5 + embedding rerank
GET /article/{id}                    → redirect to kiwix-serve URL
GET /health                          → index stats (sources, articles, tier)
```

The `/article/{id}` endpoint maps `sources.filename` + `articles.path` to a
kiwix-serve URL:
```
http://localhost:{kiwix_port}/{zim_name_without_ext}/{article_path}
```

Requires kiwix-serve to be running (started by browse.sh or serve-all.sh).

## Toolkit Integration

### entries.tab

The toolkit generator adds a search entry when `data/search.db` exists:

```
[search]
Search all content — 2.1M articles indexed	.svalbard/actions/search.sh
```

### serve-all.sh

Starts the search server alongside other services:

```bash
SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -n "$SQLITE_BIN" ] && [ -f "$DRIVE_ROOT/data/search.db" ]; then
    port="$(find_free_port 8084)"
    "$DRIVE_ROOT/.svalbard/actions/search-server.sh" "$port" &
    SVALBARD_PIDS+=($!)
    ui_status "Search: http://$BIND:$port"
fi
```

### Wizard integration

After build completes, the wizard optionally offers indexing:

```
Build complete. Index content for cross-ZIM search?
  1) Fast — keyword search (~20 min, ~1.2 GB)
  2) Standard — full-text search (~45 min, ~4 GB)
  3) Semantic — keyword + meaning (~4 hours, ~5.5 GB)
  4) Skip for now (run 'svalbard index' later)
```

## Bundled Binaries

- `bin/{platform}/sqlite3` — ~1-2 MB, for search queries on the drive
- `kiwix-serve` — already bundled, serves ZIM page content
- `llama-server` — already bundled, used for query embedding at search time
  (semantic tier only, same binary used for chat)
- `nomic-embed-text-v1.5-q8_0.gguf` — 140 MB, only needed for semantic tier

## Files on Drive

```
data/search.db                              # search index
models/nomic-embed-text-v1.5-q8_0.gguf     # embedding model (semantic only)
bin/{platform}/sqlite3                      # bundled binary
.svalbard/actions/search.sh                 # CLI search
.svalbard/actions/search-server.sh          # REST API server
```

## Evolution Path

1. **PoC (this design):** shell + sqlite3 + kiwix-serve, FTS5 search, optional semantic rerank
2. **Go binary:** replace shell glue + sqlite3 CLI with a single Go binary that embeds SQLite, serves REST, handles ZIM reading directly
3. **Richer content:** index video metadata, EPUBs, PDF text, local sources
4. **Better ranking:** BM25 tuning, field weighting (title boost), source-type weighting
