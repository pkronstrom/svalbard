package query_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/query"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func setupTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	db, err := sql.Open("sqlite3", filepath.Join(dir, "data", "test.sqlite"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE medicines (id INTEGER PRIMARY KEY, name TEXT, ingredient TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO medicines VALUES (1, 'Aspirin', 'acetylsalicylic acid')"); err != nil {
		t.Fatalf("insert row 1: %v", err)
	}
	if _, err := db.Exec("INSERT INTO medicines VALUES (2, 'Ibuprofen', 'ibuprofen')"); err != nil {
		t.Fatalf("insert row 2: %v", err)
	}
	return dir
}

func TestExecuteSelectReturnsRows(t *testing.T) {
	dir := setupTestDB(t)
	rows, err := query.Execute(dir, "test.sqlite", "SELECT id, name FROM medicines ORDER BY id")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Aspirin" {
		t.Errorf("expected first row name 'Aspirin', got %v", rows[0]["name"])
	}
	if rows[1]["name"] != "Ibuprofen" {
		t.Errorf("expected second row name 'Ibuprofen', got %v", rows[1]["name"])
	}
}

func TestExecuteRejectsWriteStatements(t *testing.T) {
	dir := setupTestDB(t)
	_, err := query.Execute(dir, "test.sqlite", "INSERT INTO medicines VALUES (3, 'Paracetamol', 'paracetamol')")
	if err == nil {
		t.Fatal("expected error for INSERT statement")
	}
}

func TestExecuteRejectsDropStatements(t *testing.T) {
	dir := setupTestDB(t)
	_, err := query.Execute(dir, "test.sqlite", "DROP TABLE medicines")
	if err == nil {
		t.Fatal("expected error for DROP statement")
	}
}

func TestExecuteRejectsPathTraversal(t *testing.T) {
	dir := setupTestDB(t)
	_, err := query.Execute(dir, "../etc/passwd", "SELECT 1")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestDescribeReturnsSchema(t *testing.T) {
	dir := setupTestDB(t)
	info, err := query.Describe(dir, "test.sqlite", "")
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if info.Database != "test.sqlite" {
		t.Errorf("expected database 'test.sqlite', got %q", info.Database)
	}
	if len(info.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(info.Tables))
	}
	if info.Tables[0].Name != "medicines" {
		t.Errorf("expected table name 'medicines', got %q", info.Tables[0].Name)
	}
	wantCols := []string{"id", "name", "ingredient"}
	if len(info.Tables[0].Columns) != len(wantCols) {
		t.Fatalf("expected %d columns, got %d", len(wantCols), len(info.Tables[0].Columns))
	}
	for i, want := range wantCols {
		if info.Tables[0].Columns[i] != want {
			t.Errorf("column[%d]: expected %q, got %q", i, want, info.Tables[0].Columns[i])
		}
	}
}

func TestDescribeReturnsFTSStatus(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	db, err := sql.Open("sqlite3", filepath.Join(dir, "data", "fts.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE VIRTUAL TABLE articles USING fts5(title, body)"); err != nil {
		t.Fatalf("create FTS5 table: %v", err)
	}

	info, err := query.Describe(dir, "fts.sqlite", "articles")
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if len(info.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(info.Tables))
	}
	if !info.Tables[0].IsFTS {
		t.Error("expected IsFTS=true for FTS5 table")
	}
}

func TestDescribeReturnsSampleRows(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	db, err := sql.Open("sqlite3", filepath.Join(dir, "data", "sample.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, label TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := db.Exec("INSERT INTO items VALUES (?, ?)", i, "item-"+string(rune('A'-1+i))); err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}

	info, err := query.Describe(dir, "sample.sqlite", "items")
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if len(info.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(info.Tables))
	}
	samples := info.Tables[0].Samples
	if len(samples) != 3 {
		t.Fatalf("expected 3 sample rows, got %d", len(samples))
	}
}
