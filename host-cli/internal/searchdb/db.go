// Package searchdb provides a SQLite FTS5 full-text search database
// for cross-ZIM article search.
package searchdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// DB wraps a SQLite database with FTS5 for article search.
type DB struct {
	db *sql.DB
}

// SemanticContract describes the vector compatibility boundary for one build.
type SemanticContract struct {
	Version     int    `json:"version"`
	Model       string `json:"model"`
	ModelSHA256 string `json:"model_sha256"`
	DocPrefix   string `json:"doc_prefix"`
	QueryPrefix string `json:"query_prefix"`
	Dimensions  int    `json:"dimensions"`
	Matryoshka  bool   `json:"matryoshka"`
	// SourceSelectors is a canonical JSON array of exact filenames and
	// directory prefixes. An empty value retains legacy full coverage.
	SourceSelectors string `json:"source_selectors,omitempty"`
}

// Article represents an article to be indexed.
type Article struct {
	Path     string
	Title    string
	Body     string
	Sections string // JSON: [{"heading":"","body":"..."}]
}

// SearchResult represents a single search hit.
type SearchResult struct {
	Title    string
	Body     string
	Path     string
	SourceID int64
	Rank     float64
}

const schema = `
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);

CREATE TABLE IF NOT EXISTS sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    indexed_at TEXT,
    source_uuid TEXT,
    source_size INTEGER,
    source_mtime_ns INTEGER,
    note TEXT
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
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    chunk_header TEXT NOT NULL,
    vector BLOB NOT NULL,
    UNIQUE(article_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_embeddings_article ON embeddings(article_id);
`

// Open opens (or creates) the search database at path and ensures the schema exists.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("searchdb: open %s: %w", path, err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("searchdb: set WAL mode: %w", err)
	}

	// Enable foreign key enforcement (required for ON DELETE CASCADE).
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("searchdb: enable foreign keys: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("searchdb: create schema: %w", err)
	}

	for _, column := range []struct {
		name string
		typ  string
	}{
		{"sections", "TEXT"},
		{"source_uuid", "TEXT"},
		{"source_size", "INTEGER"},
		{"source_mtime_ns", "INTEGER"},
		{"note", "TEXT"},
		{"reader_version", "INTEGER"},
		{"native_language", "TEXT"},
	} {
		var present int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, tableForColumn(column.name), column.name).Scan(&present); err != nil {
			db.Close()
			return nil, fmt.Errorf("searchdb: inspect %s migration: %w", column.name, err)
		}
		if present == 0 {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableForColumn(column.name), column.name, column.typ)); err != nil {
				db.Close()
				return nil, fmt.Errorf("searchdb: migrate %s: %w", column.name, err)
			}
		}
	}

	// Migrate: detect old-style embeddings table (article_id as PK, no chunk_index)
	// and recreate with the new schema.
	var hasChunkIndex int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('embeddings') WHERE name='chunk_index'`,
	).Scan(&hasChunkIndex)
	if err == nil && hasChunkIndex == 0 {
		db.Exec("DROP TABLE IF EXISTS embeddings")
		db.Exec(`CREATE TABLE IF NOT EXISTS embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			article_id INTEGER NOT NULL REFERENCES articles(id),
			chunk_index INTEGER NOT NULL,
			chunk_header TEXT NOT NULL,
			vector BLOB NOT NULL,
			UNIQUE(article_id, chunk_index)
		)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_embeddings_article ON embeddings(article_id)`)
	}

	return &DB{db: db}, nil
}

func tableForColumn(column string) string {
	if column == "sections" {
		return "articles"
	}
	return "sources"
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// SetMeta sets a key-value pair in the meta table.
func (d *DB) SetMeta(key, value string) error {
	return d.SetMetaContext(context.Background(), key, value)
}

// SetMetaContext sets a key-value pair while honoring caller cancellation.
func (d *DB) SetMetaContext(ctx context.Context, key, value string) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("searchdb: begin metadata update: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("searchdb: commit metadata update: %w", err)
	}
	return nil
}

// GetMeta retrieves a value from the meta table. Returns "" if the key is not found.
func (d *DB) GetMeta(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// IndexedFilenames returns all filenames currently indexed.
func (d *DB) IndexedFilenames() ([]string, error) {
	rows, err := d.db.Query("SELECT filename FROM sources ORDER BY filename")
	if err != nil {
		return nil, fmt.Errorf("searchdb: query filenames: %w", err)
	}
	defer rows.Close()

	var filenames []string
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err != nil {
			return nil, fmt.Errorf("searchdb: scan filename: %w", err)
		}
		filenames = append(filenames, fn)
	}
	return filenames, rows.Err()
}

// Source identifies an archive or document and the cheap identity used to
// tell whether its indexed articles are still current. A document's Filename
// is its vault-relative path (books/...), which holds a "/" where an archive
// name never does.
type Source struct {
	Filename  string
	Title     string
	UUID      string
	Size      int64
	ModTimeNS int64
	// Note says why a source is not, or not fully, searchable: a scanned or
	// password-protected document. Empty for a fully indexed source.
	Note string
	// ReaderVersion is the version of the reader that indexed the source, 0
	// for a row written before versions were recorded. Like the identity it
	// is a cache key, not a checksum: a ZIM indexed by an older reader is
	// read again.
	ReaderVersion int
	// ZIMSearchKnown records that this archive was checked for full-text
	// search. NativeLanguage is empty when SQLite indexes its HTML.
	ZIMSearchKnown bool
	NativeLanguage string
}

// IndexedSources returns all currently indexed source identities. Legacy rows
// without an identity are returned with an empty UUID and are never current.
func (d *DB) IndexedSources() ([]Source, error) {
	rows, err := d.db.Query(`SELECT filename, title, COALESCE(source_uuid, ''), COALESCE(source_size, 0), COALESCE(source_mtime_ns, 0), COALESCE(note, ''), COALESCE(reader_version, 0), native_language FROM sources ORDER BY filename`)
	if err != nil {
		return nil, fmt.Errorf("searchdb: query sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var source Source
		var language sql.NullString
		if err := rows.Scan(&source.Filename, &source.Title, &source.UUID, &source.Size, &source.ModTimeNS, &source.Note, &source.ReaderVersion, &language); err != nil {
			return nil, fmt.Errorf("searchdb: scan source: %w", err)
		}
		source.ZIMSearchKnown, source.NativeLanguage = language.Valid, language.String
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

// SetSourceReaderVersion records the reader version of an indexed source
// without touching its articles. The ZIM pass calls it last, once the
// archive's documents are all indexed, so a run cut short leaves the archive
// to be visited again.
func (d *DB) SetSourceReaderVersion(ctx context.Context, filename string, version int) error {
	result, err := d.db.ExecContext(ctx, `UPDATE sources SET reader_version = ? WHERE filename = ?`, version, filename)
	if err != nil {
		return fmt.Errorf("searchdb: set reader version of %s: %w", filename, err)
	}
	if n, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("searchdb: set reader version of %s: %w", filename, err)
	} else if n == 0 {
		return fmt.Errorf("searchdb: set reader version: %s is not indexed", filename)
	}
	return nil
}

// ArticleHead is an indexed article's path and title.
type ArticleHead struct {
	Path  string
	Title string
}

// EachArticleHead calls fn with the path and title of each of one source's
// articles, in no particular order, until fn returns false. The rows are
// streamed: an archive may hold millions.
func (d *DB) EachArticleHead(ctx context.Context, filename string, fn func(ArticleHead) bool) error {
	rows, err := d.db.QueryContext(ctx, `SELECT a.path, a.title FROM articles a JOIN sources s ON s.id = a.source_id WHERE s.filename = ?`, filename)
	if err != nil {
		return fmt.Errorf("searchdb: article heads of %s: %w", filename, err)
	}
	defer rows.Close()
	for rows.Next() {
		var head ArticleHead
		if err := rows.Scan(&head.Path, &head.Title); err != nil {
			return fmt.Errorf("searchdb: scan article head: %w", err)
		}
		if !fn(head) {
			return nil
		}
	}
	return rows.Err()
}

// IdentityCurrent reports whether a persisted source has the same complete
// archive identity as candidate. An identity missing from a legacy row is dirty.
func (s Source) IdentityCurrent(candidate Source) bool {
	return s.UUID != "" && s.UUID == candidate.UUID && s.Size == candidate.Size && s.ModTimeNS == candidate.ModTimeNS
}

// ReplaceSource atomically replaces a source's articles and identity. It
// deletes embeddings explicitly because older databases did not cascade them.
func (d *DB) ReplaceSource(ctx context.Context, source Source, articles []Article) error {
	return d.ReplaceSourceStream(ctx, source, articleSequence(articles))
}

// ReplaceTranscript preserves an existing selective semantic index when its
// recorded coverage excludes this optional keyword source.
func (d *DB) ReplaceTranscript(ctx context.Context, source Source, articles []Article) error {
	return d.replaceSourceStream(ctx, source, articleSequence(articles), true)
}

func articleSequence(articles []Article) iter.Seq2[Article, error] {
	return func(yield func(Article, error) bool) {
		for _, article := range articles {
			if !yield(article, nil) {
				return
			}
		}
	}
}

// ReplaceSourceStream rolls back when article extraction or insertion fails.
func (d *DB) ReplaceSourceStream(ctx context.Context, source Source, articles iter.Seq2[Article, error]) error {
	return d.replaceSourceStream(ctx, source, articles, false)
}

func (d *DB) replaceSourceStream(ctx context.Context, source Source, articles iter.Seq2[Article, error], transcript bool) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("searchdb: begin replacement: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM sources WHERE filename = ?", source.Filename).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("searchdb: find source %q: %w", source.Filename, err)
	}
	if err == nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings WHERE article_id IN (SELECT id FROM articles WHERE source_id = ?)`, existingID); err != nil {
			return fmt.Errorf("searchdb: delete source embeddings: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM articles WHERE source_id = ?", existingID); err != nil {
			return fmt.Errorf("searchdb: delete source articles: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var note any
	if source.Note != "" {
		note = source.Note
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sources (filename, title, indexed_at, source_uuid, source_size, source_mtime_ns, note, reader_version) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(filename) DO UPDATE SET title=excluded.title, indexed_at=excluded.indexed_at, source_uuid=excluded.source_uuid, source_size=excluded.source_size, source_mtime_ns=excluded.source_mtime_ns, note=excluded.note, reader_version=excluded.reader_version, native_language=NULL`,
		source.Filename, source.Title, now, source.UUID, source.Size, source.ModTimeNS, note, source.ReaderVersion); err != nil {
		return fmt.Errorf("searchdb: replace source: %w", err)
	}
	var sourceID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM sources WHERE filename = ?", source.Filename).Scan(&sourceID); err != nil {
		return fmt.Errorf("searchdb: get replacement source id: %w", err)
	}
	if err := insertArticles(ctx, tx, sourceID, articles); err != nil {
		return err
	}
	if !transcript || !outsideSemanticSelection(ctx, tx, source.Filename) {
		if _, err := tx.ExecContext(ctx, `UPDATE meta SET value = 'building' WHERE key = 'semantic_state' AND value = 'complete'`); err != nil {
			return fmt.Errorf("searchdb: invalidate semantic state: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("searchdb: commit replacement: %w", err)
	}
	return nil
}

func outsideSemanticSelection(ctx context.Context, tx *sql.Tx, filename string) bool {
	var raw string
	var contract SemanticContract
	var selectors []string
	if err := tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key = 'semantic_contract'").Scan(&raw); err != nil ||
		json.Unmarshal([]byte(raw), &contract) != nil ||
		json.Unmarshal([]byte(contract.SourceSelectors), &selectors) != nil || len(selectors) == 0 {
		return false
	}
	for _, selector := range selectors {
		if filename == selector || strings.HasSuffix(selector, "/") && strings.HasPrefix(filename, selector) {
			return false
		}
	}
	return true
}

// PruneSources removes every indexed source absent from present. Source removal
// leaves a completed semantic index valid for the surviving articles.
func (d *DB) PruneSources(ctx context.Context, present map[string]bool) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("searchdb: begin prune: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id, filename FROM sources")
	if err != nil {
		return fmt.Errorf("searchdb: list prune sources: %w", err)
	}
	var ids []int64
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			rows.Close()
			return err
		}
		var id int64
		var filename string
		if err := rows.Scan(&id, &filename); err != nil {
			rows.Close()
			return fmt.Errorf("searchdb: scan prune source: %w", err)
		}
		if !present[filename] {
			ids = append(ids, id)
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("searchdb: close prune sources: %w", err)
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings WHERE article_id IN (SELECT id FROM articles WHERE source_id = ?)`, id); err != nil {
			return fmt.Errorf("searchdb: prune embeddings: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM articles WHERE source_id = ?", id); err != nil {
			return fmt.Errorf("searchdb: prune articles: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM sources WHERE id = ?", id); err != nil {
			return fmt.Errorf("searchdb: prune source: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("searchdb: commit prune: %w", err)
	}
	return nil
}

func insertArticles(ctx context.Context, tx *sql.Tx, sourceID int64, articles iter.Seq2[Article, error]) error {
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO articles (source_id, path, title, body, sections) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("searchdb: prepare insert: %w", err)
	}
	defer stmt.Close()

	for a, extractErr := range articles {
		if extractErr != nil {
			return extractErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var sections interface{}
		if a.Sections != "" {
			sections = a.Sections
		}
		if _, err := stmt.ExecContext(ctx, sourceID, a.Path, a.Title, a.Body, sections); err != nil {
			return fmt.Errorf("searchdb: insert article %q: %w", a.Path, err)
		}
	}

	return nil
}

// SemanticState returns the current semantic lifecycle marker, or absent when
// no semantic contract has been published.
func (d *DB) SemanticState() (string, error) {
	return d.GetMeta("semantic_state")
}

// LoadSemanticContract returns the stored contract and lifecycle state. Legacy
// vector rows without this metadata are deliberately unavailable to readers.
func (d *DB) LoadSemanticContract() (SemanticContract, string, error) {
	state, err := d.SemanticState()
	if err != nil || state == "" {
		return SemanticContract{}, state, err
	}
	raw, err := d.GetMeta("semantic_contract")
	if err != nil {
		return SemanticContract{}, "", err
	}
	var contract SemanticContract
	if err := json.Unmarshal([]byte(raw), &contract); err != nil {
		return SemanticContract{}, "", fmt.Errorf("searchdb: decode semantic contract: %w", err)
	}
	return contract, state, nil
}

// BeginSemanticContract atomically publishes a building contract and clears
// incompatible vectors. Matching building work is retained for resume.
func (d *DB) BeginSemanticContract(ctx context.Context, contract SemanticContract, force bool) (bool, error) {
	current, state, err := d.LoadSemanticContract()
	if err != nil {
		return false, err
	}
	if !force && current == contract && state == "building" {
		return false, nil
	}
	if !force && current == contract && state == "complete" {
		return false, nil
	}
	raw, err := json.Marshal(contract)
	if err != nil {
		return false, fmt.Errorf("searchdb: encode semantic contract: %w", err)
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("searchdb: begin semantic contract: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM embeddings"); err != nil {
		return false, fmt.Errorf("searchdb: clear incompatible embeddings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES ('semantic_contract', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, string(raw)); err != nil {
		return false, fmt.Errorf("searchdb: store semantic contract: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES ('semantic_state', 'building') ON CONFLICT(key) DO UPDATE SET value=excluded.value`); err != nil {
		return false, fmt.Errorf("searchdb: mark semantic building: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("searchdb: commit semantic contract: %w", err)
	}
	return true, nil
}

// SemanticCompletionMetadata is the legacy reader metadata published with a
// completed semantic contract. Its fixed fields prevent callers from changing
// contract lifecycle keys after coverage has been validated.
type SemanticCompletionMetadata struct {
	IndexedAt string
	Model     string
}

// CompleteSemanticContract publishes legacy reader metadata and
// the completion marker in the same transaction as the coverage check.
func (d *DB) CompleteSemanticContract(ctx context.Context, contract SemanticContract, metadata SemanticCompletionMetadata) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("searchdb: begin semantic completion: %w", err)
	}
	defer tx.Rollback()
	var raw string
	var state string
	if err := tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='semantic_contract'").Scan(&raw); err != nil {
		return fmt.Errorf("searchdb: load completion contract: %w", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='semantic_state'").Scan(&state); err != nil {
		return fmt.Errorf("searchdb: load completion state: %w", err)
	}
	var current SemanticContract
	if err := json.Unmarshal([]byte(raw), &current); err != nil || current != contract {
		return fmt.Errorf("searchdb: semantic contract changed before completion")
	}
	if state == "complete" {
		return nil
	}
	var missing int
	coverage := `SELECT COUNT(*) FROM articles a JOIN sources s ON s.id=a.source_id
		WHERE NOT EXISTS (SELECT 1 FROM embeddings e WHERE e.article_id=a.id)`
	var args []any
	if contract.SourceSelectors != "" {
		coverage += ` AND EXISTS (SELECT 1 FROM json_each(?) selected
			WHERE s.filename=selected.value OR
				(substr(selected.value, -1)='/' AND substr(s.filename, 1, length(selected.value))=selected.value))`
		args = append(args, contract.SourceSelectors)
	}
	if err := tx.QueryRowContext(ctx, coverage, args...).Scan(&missing); err != nil {
		return err
	}
	if missing != 0 {
		return fmt.Errorf("searchdb: semantic coverage incomplete: %d articles missing embeddings", missing)
	}
	for key, value := range map[string]string{
		"semantic_indexed_at":    metadata.IndexedAt,
		"embedding_model":        metadata.Model,
		"embedding_doc_prefix":   contract.DocPrefix,
		"embedding_query_prefix": contract.QueryPrefix,
		"embedding_dims":         fmt.Sprintf("%d", contract.Dimensions),
	} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO meta (key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value); err != nil {
			return fmt.Errorf("searchdb: publish %s: %w", key, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO meta (key,value) VALUES ('semantic_state','complete') ON CONFLICT(key) DO UPDATE SET value=excluded.value"); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("searchdb: commit semantic completion: %w", err)
	}
	return nil
}

// Search performs an FTS5 MATCH query and returns ranked results.
func (d *DB) Search(query string, limit int) ([]SearchResult, error) {
	rows, err := d.db.Query(
		`SELECT a.title, a.body, a.path, a.source_id, f.rank
		 FROM articles_fts f
		 JOIN articles a ON a.id = f.rowid
		 WHERE articles_fts MATCH ?
		 ORDER BY f.rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searchdb: search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Title, &r.Body, &r.Path, &r.SourceID, &r.Rank); err != nil {
			return nil, fmt.Errorf("searchdb: scan result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ChunkEmbedding holds a chunk-level embedding for an article.
type ChunkEmbedding struct {
	ArticleID  int64
	ChunkIndex int
	Header     string
	Vector     []byte // little-endian float32 blob
}

// UnembeddedArticle is an article that has not yet been embedded.
type UnembeddedArticle struct {
	ID       int64
	Title    string
	Body     string
	Sections string
}

// InsertChunkEmbeddings stores a batch of chunk embedding vectors within a transaction.
func (d *DB) InsertChunkEmbeddings(chunks []ChunkEmbedding) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("searchdb: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		"INSERT OR REPLACE INTO embeddings (article_id, chunk_index, chunk_header, vector) VALUES (?, ?, ?, ?)",
	)
	if err != nil {
		return fmt.Errorf("searchdb: prepare insert chunk embedding: %w", err)
	}
	defer stmt.Close()

	for _, c := range chunks {
		if _, err := stmt.Exec(c.ArticleID, c.ChunkIndex, c.Header, c.Vector); err != nil {
			return fmt.Errorf("searchdb: insert chunk embedding %d/%d: %w", c.ArticleID, c.ChunkIndex, err)
		}
	}
	return tx.Commit()
}

// DeleteAllEmbeddings removes all stored embeddings.
func (d *DB) DeleteAllEmbeddings() error {
	_, err := d.db.Exec("DELETE FROM embeddings")
	return err
}

// EmbeddingDims returns the dimension count from the first stored vector (4 bytes per float32).
func (d *DB) EmbeddingDims() (int, error) {
	var blobLen int
	err := d.db.QueryRow("SELECT length(vector) FROM embeddings LIMIT 1").Scan(&blobLen)
	if err != nil {
		return 0, err
	}
	return blobLen / 4, nil
}

// SourceInfo holds a source's ID, filename, and article count.
type SourceInfo struct {
	ID           int64
	Filename     string
	ArticleCount int64
}

// Sources returns all indexed sources with their article counts.
func (d *DB) Sources() ([]SourceInfo, error) {
	rows, err := d.db.Query(
		`SELECT s.id, s.filename, COUNT(a.id)
		 FROM sources s
		 LEFT JOIN articles a ON a.source_id = s.id
		 GROUP BY s.id
		 ORDER BY s.filename`,
	)
	if err != nil {
		return nil, fmt.Errorf("searchdb: query sources: %w", err)
	}
	defer rows.Close()

	var sources []SourceInfo
	for rows.Next() {
		var s SourceInfo
		if err := rows.Scan(&s.ID, &s.Filename, &s.ArticleCount); err != nil {
			return nil, fmt.Errorf("searchdb: scan source: %w", err)
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

// UnembeddedArticlesBySource returns articles without embeddings for a given
// source, ordered by ID, starting after afterID, limited to limit rows.
func (d *DB) UnembeddedArticlesBySource(sourceID int64, afterID int64, limit int) ([]UnembeddedArticle, error) {
	rows, err := d.db.Query(
		`SELECT a.id, a.title, a.body, COALESCE(a.sections, '') FROM articles a
		 LEFT JOIN embeddings e ON e.article_id = a.id
		 WHERE e.article_id IS NULL AND a.source_id = ? AND a.id > ?
		 ORDER BY a.id LIMIT ?`,
		sourceID, afterID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searchdb: query unembedded by source: %w", err)
	}
	defer rows.Close()

	var articles []UnembeddedArticle
	for rows.Next() {
		var a UnembeddedArticle
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.Sections); err != nil {
			return nil, fmt.Errorf("searchdb: scan unembedded: %w", err)
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// EmbeddingCountBySource returns the number of articles with embeddings for a given source.
func (d *DB) EmbeddingCountBySource(sourceID int64) (int64, error) {
	var count int64
	err := d.db.QueryRow(
		`SELECT COUNT(DISTINCT e.article_id) FROM embeddings e
		 JOIN articles a ON a.id = e.article_id
		 WHERE a.source_id = ?`,
		sourceID,
	).Scan(&count)
	return count, err
}

// Stats returns the number of sources and articles in the database.
func (d *DB) Stats() (sourceCount, articleCount int64, err error) {
	err = d.db.QueryRow("SELECT COUNT(*) FROM sources").Scan(&sourceCount)
	if err != nil {
		return 0, 0, fmt.Errorf("searchdb: count sources: %w", err)
	}
	err = d.db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&articleCount)
	if err != nil {
		return 0, 0, fmt.Errorf("searchdb: count articles: %w", err)
	}
	return sourceCount, articleCount, nil
}
