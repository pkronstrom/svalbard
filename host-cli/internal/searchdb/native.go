package searchdb

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"time"
)

// SetNativeZIM registers an archive after its embedded documents are ready.
// Any legacy HTML rows have already been excluded from the staged database.
func (d *DB) SetNativeZIM(ctx context.Context, source Source) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO sources
		(filename, title, indexed_at, source_uuid, source_size, source_mtime_ns, reader_version, native_language)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(filename) DO UPDATE SET title=excluded.title, indexed_at=excluded.indexed_at,
			source_uuid=excluded.source_uuid, source_size=excluded.source_size, source_mtime_ns=excluded.source_mtime_ns,
			reader_version=excluded.reader_version, native_language=excluded.native_language`,
		source.Filename, source.Title, time.Now().UTC().Format(time.RFC3339), source.UUID,
		source.Size, source.ModTimeNS, source.ReaderVersion, source.NativeLanguage)
	return err
}

func (d *DB) MarkSQLiteZIM(ctx context.Context, filename string) error {
	_, err := d.db.ExecContext(ctx, "UPDATE sources SET native_language = '' WHERE filename = ?", filename)
	return err
}

// CopyFallback copies the legacy database into a new compact database,
// omitting articles and vectors for archives switching to native search.
// Source and article IDs stay stable for all retained content.
func (d *DB) CopyFallback(ctx context.Context, oldPath string, native []string) error {
	absolute, err := filepath.Abs(oldPath)
	if err != nil {
		return err
	}
	path := filepath.ToSlash(absolute)
	if filepath.VolumeName(absolute) != "" && path[0] != '/' {
		path = "/" + path
	}
	old := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE ? AS legacy", old.String()); err != nil {
		return fmt.Errorf("attaching legacy search database: %w", err)
	}
	defer conn.ExecContext(context.Background(), "DETACH DATABASE legacy")
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "CREATE TEMP TABLE omit_archive (filename TEXT PRIMARY KEY)"); err != nil {
		return err
	}
	for _, name := range native {
		if _, err := tx.ExecContext(ctx, "INSERT INTO omit_archive VALUES (?)", name); err != nil {
			return err
		}
	}
	queries := []string{
		`INSERT INTO main.meta SELECT key, value FROM legacy.meta`,
		`INSERT INTO main.sources (id, filename, title, indexed_at, source_uuid, source_size, source_mtime_ns, note, reader_version, native_language)
		 SELECT id, filename, title, indexed_at, source_uuid, source_size, source_mtime_ns, note, reader_version, native_language FROM legacy.sources`,
		`INSERT INTO main.articles (id, source_id, path, title, body, sections)
		 SELECT a.id, a.source_id, a.path, a.title, a.body, a.sections
		 FROM main.sources s CROSS JOIN legacy.articles a INDEXED BY idx_articles_source
		 WHERE s.filename NOT IN (SELECT filename FROM omit_archive) AND a.source_id = s.id`,
		`INSERT INTO main.embeddings (id, article_id, chunk_index, chunk_header, vector)
		 SELECT e.id, e.article_id, e.chunk_index, e.chunk_header, e.vector
		 FROM main.articles a CROSS JOIN legacy.embeddings e INDEXED BY idx_embeddings_article
		 WHERE e.article_id = a.id`,
		`DROP TABLE omit_archive`,
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("copying fallback search data: %w", err)
		}
	}
	return tx.Commit()
}

func (d *DB) CheckIntegrity(ctx context.Context) error {
	var integrity string
	if err := d.db.QueryRowContext(ctx, "PRAGMA integrity_check(1)").Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("SQLite integrity check: %s", integrity)
	}
	_, err := d.db.ExecContext(ctx, "INSERT INTO articles_fts(articles_fts, rank) VALUES ('integrity-check', 1)")
	return err
}

// PrepareReplacement drains the WAL and requires an exclusive SQLite lock
// to leave WAL mode. An active reader can make this fail safely before rename.
func (d *DB) PrepareReplacement(ctx context.Context) error {
	var mode string
	if err := d.db.QueryRowContext(ctx, "PRAGMA journal_mode=DELETE").Scan(&mode); err != nil {
		return err
	}
	if mode != "delete" {
		return fmt.Errorf("could not leave WAL mode (journal mode %s)", mode)
	}
	return nil
}
