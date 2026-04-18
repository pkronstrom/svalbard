package mcp

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// openSearchDB opens search.db in read-only mode using ncruces/go-sqlite3
// (which has FTS5 compiled in). Returns the raw *sql.DB; the engine.Engine
// built on top of it provides keyword, hybrid, and read operations.
func openSearchDB(driveRoot string) (*sql.DB, error) {
	dbPath := filepath.Join(driveRoot, "data", "search.db")
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, fmt.Errorf("opening search.db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("search.db not accessible: %w", err)
	}
	return db, nil
}
