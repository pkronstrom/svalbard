package mcp

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

// searchDB provides direct database/sql access to search.db, bypassing the
// Session's sqlite3 binary shell-out. This uses modernc.org/sqlite which has
// FTS5 compiled in, so it works without any external binaries.
type searchDB struct {
	db *sql.DB
}

func openSearchDB(driveRoot string) (*searchDB, error) {
	dbPath := filepath.Join(driveRoot, "data", "search.db")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, fmt.Errorf("opening search.db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("search.db not accessible: %w", err)
	}
	return &searchDB{db: db}, nil
}

func (s *searchDB) Close() error {
	return s.db.Close()
}

func (s *searchDB) keywordSearch(query string, limit int) ([]search.Result, error) {
	ftsQuery := search.BuildFTSQuery(query)
	rows, err := s.db.Query(
		`SELECT a.id, src.filename, a.path, a.title, snippet(articles_fts, 1, '»', '«', '...', 12)
		 FROM articles_fts
		 JOIN articles a ON a.id = articles_fts.rowid
		 JOIN sources src ON src.id = a.source_id
		 WHERE articles_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		ftsQuery, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("FTS query failed: %w", err)
	}
	defer rows.Close()

	var results []search.Result
	for rows.Next() {
		var r search.Result
		var snippet sql.NullString
		if err := rows.Scan(&r.ID, &r.Filename, &r.Path, &r.Title, &snippet); err != nil {
			continue
		}
		if snippet.Valid {
			r.Snippet = snippet.String
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *searchDB) readArticle(source, path string) (string, error) {
	filename := source
	if !strings.HasSuffix(filename, ".zim") {
		filename += ".zim"
	}
	var body sql.NullString
	err := s.db.QueryRow(
		`SELECT a.body FROM articles a
		 JOIN sources src ON src.id = a.source_id
		 WHERE src.filename = ? AND a.path = ?`,
		filename, path,
	).Scan(&body)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("article not found: %s/%s", source, path)
	}
	if err != nil {
		return "", err
	}
	return body.String, nil
}

func (s *searchDB) listSources() ([]sourceEntry, error) {
	rows, err := s.db.Query(
		`SELECT filename, article_count FROM sources ORDER BY filename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []sourceEntry
	for rows.Next() {
		var e sourceEntry
		if err := rows.Scan(&e.Filename, &e.ArticleCount); err != nil {
			continue
		}
		sources = append(sources, e)
	}
	return sources, rows.Err()
}

type sourceEntry struct {
	Filename     string `json:"filename"`
	ArticleCount int    `json:"article_count"`
}
