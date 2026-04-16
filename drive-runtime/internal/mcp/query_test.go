package mcp_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/query"

	_ "modernc.org/sqlite"
)

func setupQueryTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0o755)
	db, err := sql.Open("sqlite", filepath.Join(dir, "data", "test.sqlite"))
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

func TestQueryCapabilityToolName(t *testing.T) {
	cap := mcp.NewQueryCapability(t.TempDir(), mcp.DriveMetadata{})
	if cap.Tool() != "query" {
		t.Errorf("expected tool name 'query', got %q", cap.Tool())
	}
	if cap.Description() == "" {
		t.Error("expected non-empty description")
	}
	actions := cap.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	names := map[string]bool{}
	for _, a := range actions {
		names[a.Name] = true
	}
	for _, want := range []string{"describe", "sql"} {
		if !names[want] {
			t.Errorf("missing action %q", want)
		}
	}
}

func TestQueryCapabilitySQLAction(t *testing.T) {
	dir := setupQueryTestDB(t)
	cap := mcp.NewQueryCapability(dir, mcp.DriveMetadata{})

	result, err := cap.Handle(context.Background(), "sql", map[string]any{
		"database": "test.sqlite",
		"sql":      "SELECT name FROM medicines ORDER BY id",
	})
	if err != nil {
		t.Fatalf("Handle(sql) error = %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected non-nil data")
	}

	rows, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any, got %T", result.Data)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Aspirin" {
		t.Errorf("expected first row name 'Aspirin', got %v", rows[0]["name"])
	}
}

func TestQueryCapabilityDescribeAction(t *testing.T) {
	dir := setupQueryTestDB(t)
	cap := mcp.NewQueryCapability(dir, mcp.DriveMetadata{})

	result, err := cap.Handle(context.Background(), "describe", map[string]any{
		"database": "test.sqlite",
	})
	if err != nil {
		t.Fatalf("Handle(describe) error = %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected non-nil data")
	}

	info, ok := result.Data.(query.SchemaInfo)
	if !ok {
		t.Fatalf("expected query.SchemaInfo, got %T", result.Data)
	}
	if info.Database != "test.sqlite" {
		t.Errorf("expected database 'test.sqlite', got %q", info.Database)
	}
	if len(info.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(info.Tables))
	}
	if info.Tables[0].Name != "medicines" {
		t.Errorf("expected table 'medicines', got %q", info.Tables[0].Name)
	}
}

func TestQueryCapabilityUnknownAction(t *testing.T) {
	cap := mcp.NewQueryCapability(t.TempDir(), mcp.DriveMetadata{})
	_, err := cap.Handle(context.Background(), "nonexistent", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}
