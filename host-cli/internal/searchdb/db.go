// Package searchdb provides a SQLite FTS5 full-text search database
// for cross-ZIM article search.
package searchdb

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database with FTS5 for article search.
type DB struct {
	db *sql.DB
}

// Article represents an article to be indexed.
type Article struct {
	Path  string
	Title string
	Body  string
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
    indexed_at TEXT
);

CREATE TABLE IF NOT EXISTS articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    path TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL
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
    article_id INTEGER PRIMARY KEY REFERENCES articles(id),
    vector BLOB NOT NULL
);
`

// Open opens (or creates) the search database at path and ensures the schema exists.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("searchdb: open %s: %w", path, err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("searchdb: set WAL mode: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("searchdb: create schema: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// SetMeta sets a key-value pair in the meta table.
func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(
		"INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
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

// UpsertSource inserts or updates a source (ZIM file) and returns its ID.
func (d *DB) UpsertSource(filename, title string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(
		`INSERT INTO sources (filename, title, indexed_at) VALUES (?, ?, ?)
		 ON CONFLICT(filename) DO UPDATE SET title = excluded.title, indexed_at = excluded.indexed_at`,
		filename, title, now,
	)
	if err != nil {
		return 0, fmt.Errorf("searchdb: upsert source: %w", err)
	}

	var id int64
	err = d.db.QueryRow("SELECT id FROM sources WHERE filename = ?", filename).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("searchdb: get source id: %w", err)
	}
	return id, nil
}

// DeleteSource removes a source and all its articles from the database.
func (d *DB) DeleteSource(sourceID int64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("searchdb: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM articles WHERE source_id = ?", sourceID); err != nil {
		return fmt.Errorf("searchdb: delete articles: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sources WHERE id = ?", sourceID); err != nil {
		return fmt.Errorf("searchdb: delete source: %w", err)
	}

	return tx.Commit()
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

// InsertArticles inserts a batch of articles for the given source within a transaction.
func (d *DB) InsertArticles(sourceID int64, articles []Article) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("searchdb: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO articles (source_id, path, title, body) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("searchdb: prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, a := range articles {
		if _, err := stmt.Exec(sourceID, a.Path, a.Title, a.Body); err != nil {
			return fmt.Errorf("searchdb: insert article %q: %w", a.Path, err)
		}
	}

	return tx.Commit()
}

// DeleteSourceArticles removes all articles belonging to a source.
func (d *DB) DeleteSourceArticles(sourceID int64) error {
	_, err := d.db.Exec("DELETE FROM articles WHERE source_id = ?", sourceID)
	if err != nil {
		return fmt.Errorf("searchdb: delete source articles: %w", err)
	}
	return nil
}

// ArticleCount returns the total number of articles in the database.
func (d *DB) ArticleCount() (int64, error) {
	var count int64
	err := d.db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&count)
	return count, err
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

// EmbeddingPair holds an article ID and its embedding vector blob.
type EmbeddingPair struct {
	ArticleID int64
	Vector    []byte // little-endian float32 blob
}

// UnembeddedArticle is an article that has not yet been embedded.
type UnembeddedArticle struct {
	ID    int64
	Title string
	Body  string
}

// InsertEmbeddings stores a batch of embedding vectors within a transaction.
func (d *DB) InsertEmbeddings(pairs []EmbeddingPair) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("searchdb: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		"INSERT OR REPLACE INTO embeddings (article_id, vector) VALUES (?, ?)",
	)
	if err != nil {
		return fmt.Errorf("searchdb: prepare insert embedding: %w", err)
	}
	defer stmt.Close()

	for _, p := range pairs {
		if _, err := stmt.Exec(p.ArticleID, p.Vector); err != nil {
			return fmt.Errorf("searchdb: insert embedding %d: %w", p.ArticleID, err)
		}
	}
	return tx.Commit()
}

// UnembeddedArticles returns articles without embeddings, ordered by ID,
// starting after afterID, limited to limit rows.
func (d *DB) UnembeddedArticles(afterID int64, limit int) ([]UnembeddedArticle, error) {
	rows, err := d.db.Query(
		`SELECT a.id, a.title, a.body FROM articles a
		 LEFT JOIN embeddings e ON e.article_id = a.id
		 WHERE e.article_id IS NULL AND a.id > ?
		 ORDER BY a.id LIMIT ?`,
		afterID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("searchdb: query unembedded: %w", err)
	}
	defer rows.Close()

	var articles []UnembeddedArticle
	for rows.Next() {
		var a UnembeddedArticle
		if err := rows.Scan(&a.ID, &a.Title, &a.Body); err != nil {
			return nil, fmt.Errorf("searchdb: scan unembedded: %w", err)
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// DeleteAllEmbeddings removes all stored embeddings.
func (d *DB) DeleteAllEmbeddings() error {
	_, err := d.db.Exec("DELETE FROM embeddings")
	return err
}

// EmbeddingCount returns the number of articles with embeddings.
func (d *DB) EmbeddingCount() (int64, error) {
	var count int64
	err := d.db.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&count)
	return count, err
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
		`SELECT a.id, a.title, a.body FROM articles a
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
		if err := rows.Scan(&a.ID, &a.Title, &a.Body); err != nil {
			return nil, fmt.Errorf("searchdb: scan unembedded: %w", err)
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// EmbeddingCountBySource returns the number of embeddings for a given source.
func (d *DB) EmbeddingCountBySource(sourceID int64) (int64, error) {
	var count int64
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM embeddings e
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
