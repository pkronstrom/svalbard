package query

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver" // register sqlite3 driver
)

// TableSchema describes one table in a SQLite database.
type TableSchema struct {
	Name    string           `json:"name"`
	Columns []string         `json:"columns"`
	IsFTS   bool             `json:"is_fts,omitempty"`
	Samples []map[string]any `json:"samples,omitempty"` // first 3 rows when table specified
}

// SchemaInfo describes the schema of a SQLite database.
type SchemaInfo struct {
	Database string        `json:"database"`
	Tables   []TableSchema `json:"tables"`
}

// Execute runs a read-only SQL query against the named database under driveRoot/data/.
// It returns result rows as a slice of column-keyed maps.
func Execute(driveRoot, database, sqlQuery string) ([]map[string]any, error) {
	dbPath, err := resolvePath(driveRoot, database)
	if err != nil {
		return nil, err
	}

	db, err := openReadOnly(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("reading columns: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		row := make(map[string]any, len(cols))
		for i, col := range cols {
			// Convert []byte values to string for JSON-friendliness.
			if b, ok := values[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = values[i]
			}
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}

// Describe returns schema information for a database. If table is non-empty,
// it returns columns, FTS status, and sample rows for that specific table.
// If table is empty, it returns all tables with their columns.
func Describe(driveRoot, database, table string) (SchemaInfo, error) {
	dbPath, err := resolvePath(driveRoot, database)
	if err != nil {
		return SchemaInfo{}, err
	}

	db, err := openReadOnly(dbPath)
	if err != nil {
		return SchemaInfo{}, err
	}
	defer db.Close()

	info := SchemaInfo{Database: database}

	if table != "" {
		ts, err := describeTable(db, table)
		if err != nil {
			return SchemaInfo{}, err
		}
		info.Tables = []TableSchema{ts}
	} else {
		tables, err := listTables(db)
		if err != nil {
			return SchemaInfo{}, err
		}
		info.Tables = tables
	}

	return info, nil
}

// resolvePath validates the database name and builds the full path.
// It rejects path traversal attempts.
func resolvePath(driveRoot, database string) (string, error) {
	if database == "" {
		return "", fmt.Errorf("database name is required")
	}
	if strings.Contains(database, "..") ||
		strings.Contains(database, "/") ||
		strings.Contains(database, "\\") {
		return "", fmt.Errorf("invalid database name: %q", database)
	}
	return filepath.Join(driveRoot, "data", database), nil
}

// openReadOnly opens a SQLite database in read-only mode with ATTACH disabled.
func openReadOnly(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_query_only=true", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	// Verify connectivity.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	// Limit to 0 attached databases to prevent ATTACH escape.
	if _, err := db.Exec("PRAGMA max_attached=0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting max_attached: %w", err)
	}
	return db, nil
}

// listTables returns all user tables with their columns.
func listTables(db *sql.DB) ([]TableSchema, error) {
	rows, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()

	var tables []TableSchema
	for rows.Next() {
		var name string
		var createSQL sql.NullString
		if err := rows.Scan(&name, &createSQL); err != nil {
			return nil, fmt.Errorf("scanning table: %w", err)
		}

		cols, err := tableColumns(db, name)
		if err != nil {
			return nil, err
		}

		ts := TableSchema{
			Name:    name,
			Columns: cols,
			IsFTS:   isFTSTable(createSQL.String),
		}
		tables = append(tables, ts)
	}
	return tables, rows.Err()
}

// describeTable validates the table name against sqlite_master and returns
// its full schema with sample rows.
func describeTable(db *sql.DB, table string) (TableSchema, error) {
	// Validate table exists in sqlite_master (prevents SQL injection).
	var createSQL sql.NullString
	err := db.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table,
	).Scan(&createSQL)
	if err == sql.ErrNoRows {
		return TableSchema{}, fmt.Errorf("table not found: %q", table)
	}
	if err != nil {
		return TableSchema{}, fmt.Errorf("validating table: %w", err)
	}

	cols, err := tableColumns(db, table)
	if err != nil {
		return TableSchema{}, err
	}

	ts := TableSchema{
		Name:    table,
		Columns: cols,
		IsFTS:   isFTSTable(createSQL.String),
	}

	samples, err := sampleRows(db, table)
	if err != nil {
		return TableSchema{}, err
	}
	ts.Samples = samples

	return ts, nil
}

// tableColumns returns the column names for a validated table.
func tableColumns(db *sql.DB, table string) ([]string, error) {
	// table is already validated against sqlite_master.
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("reading columns for %q: %w", table, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("scanning column info: %w", err)
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

// isFTSTable checks whether a CREATE TABLE statement uses FTS4 or FTS5.
func isFTSTable(createSQL string) bool {
	upper := strings.ToUpper(createSQL)
	return strings.Contains(upper, "FTS5") || strings.Contains(upper, "FTS4")
}

// sampleRows returns the first 3 rows from a validated table.
func sampleRows(db *sql.DB, table string) ([]map[string]any, error) {
	// table is already validated against sqlite_master.
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %q LIMIT 3", table))
	if err != nil {
		return nil, fmt.Errorf("sampling rows: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("reading sample columns: %w", err)
	}

	var samples []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scanning sample row: %w", err)
		}

		row := make(map[string]any, len(cols))
		for i, col := range cols {
			if b, ok := values[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = values[i]
			}
		}
		samples = append(samples, row)
	}
	return samples, rows.Err()
}
