package mcp_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
	mcpserver "github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/query"

	_ "modernc.org/sqlite"
)

func setupTestDrive(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// manifest.yaml
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"),
		[]byte("preset: test-64\nregion: test\ncreated: 2026-04-16\n"), 0o644); err != nil {
		t.Fatalf("write manifest.yaml: %v", err)
	}

	// .svalbard/recipes.json  (array format expected by readRecipes)
	if err := os.MkdirAll(filepath.Join(dir, ".svalbard"), 0o755); err != nil {
		t.Fatalf("mkdir .svalbard: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".svalbard", "recipes.json"), []byte(`[
		{"id":"test-wiki","type":"zim","description":"Test Wikipedia","tags":["reference"]}
	]`), 0o644); err != nil {
		t.Fatalf("write recipes.json: %v", err)
	}

	// ZIM placeholder
	if err := os.MkdirAll(filepath.Join(dir, "zim"), 0o755); err != nil {
		t.Fatalf("mkdir zim: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zim", "test-wiki.zim"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write test-wiki.zim: %v", err)
	}

	// SQLite test DB
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "data", "test.sqlite"))
	if err != nil {
		t.Fatalf("open test.sqlite: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create items table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO items VALUES (1, 'Alpha')"); err != nil {
		t.Fatalf("insert Alpha: %v", err)
	}
	if _, err := db.Exec("INSERT INTO items VALUES (2, 'Beta')"); err != nil {
		t.Fatalf("insert Beta: %v", err)
	}
	db.Close()

	// PMTiles placeholder
	if err := os.MkdirAll(filepath.Join(dir, "maps"), 0o755); err != nil {
		t.Fatalf("mkdir maps: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "maps", "test-map.pmtiles"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write test-map.pmtiles: %v", err)
	}

	return dir
}

func TestIntegrationVaultStats(t *testing.T) {
	dir := setupTestDrive(t)
	meta, err := mcpserver.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	vault := mcpserver.NewVaultCapability(dir, meta)
	t.Cleanup(func() { _ = vault.Close() })

	result, err := vault.Handle(context.Background(), "stats", nil)
	if err != nil {
		t.Fatalf("vault.Handle(stats) error = %v", err)
	}

	stats, ok := result.Data.(inspect.DriveStats)
	if !ok {
		t.Fatalf("result.Data type = %T, want inspect.DriveStats", result.Data)
	}
	if stats.Preset != "test-64" {
		t.Errorf("stats.Preset = %q, want %q", stats.Preset, "test-64")
	}
}

func TestIntegrationVaultSources(t *testing.T) {
	dir := setupTestDrive(t)
	meta, err := mcpserver.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	vault := mcpserver.NewVaultCapability(dir, meta)
	t.Cleanup(func() { _ = vault.Close() })

	result, err := vault.Handle(context.Background(), "sources", map[string]any{})
	if err != nil {
		t.Fatalf("vault.Handle(sources) error = %v", err)
	}

	sources, ok := result.Data.([]inspect.SourceInfo)
	if !ok {
		t.Fatalf("result.Data type = %T, want []inspect.SourceInfo", result.Data)
	}
	if len(sources) < 3 {
		t.Fatalf("len(sources) = %d, want >= 3", len(sources))
	}

	// ZIM source should be enriched with description from recipes.
	var zimSource *inspect.SourceInfo
	for i := range sources {
		if sources[i].ID == "test-wiki" && sources[i].Type == "zim" {
			zimSource = &sources[i]
			break
		}
	}
	if zimSource == nil {
		t.Fatal("test-wiki ZIM source not found")
	}
	if zimSource.Description != "Test Wikipedia" {
		t.Errorf("zim description = %q, want %q", zimSource.Description, "Test Wikipedia")
	}
	if len(zimSource.Tags) == 0 || zimSource.Tags[0] != "reference" {
		t.Errorf("zim tags = %v, want [reference]", zimSource.Tags)
	}
}

func TestIntegrationVaultDatabases(t *testing.T) {
	dir := setupTestDrive(t)
	meta, err := mcpserver.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	vault := mcpserver.NewVaultCapability(dir, meta)
	t.Cleanup(func() { _ = vault.Close() })

	result, err := vault.Handle(context.Background(), "databases", nil)
	if err != nil {
		t.Fatalf("vault.Handle(databases) error = %v", err)
	}

	dbs, ok := result.Data.([]inspect.DatabaseInfo)
	if !ok {
		t.Fatalf("result.Data type = %T, want []inspect.DatabaseInfo", result.Data)
	}

	// Find the "test" DB (file is test.sqlite, name will be "test.sqlite").
	var testDB *inspect.DatabaseInfo
	for i := range dbs {
		if dbs[i].Name == "test.sqlite" {
			testDB = &dbs[i]
			break
		}
	}
	if testDB == nil {
		t.Fatalf("test.sqlite database not found in %v", dbs)
	}

	found := false
	for _, tbl := range testDB.Tables {
		if tbl == "items" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("items table not found in test.sqlite, tables = %v", testDB.Tables)
	}
}

func TestIntegrationQuerySQL(t *testing.T) {
	dir := setupTestDrive(t)
	meta, err := mcpserver.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	queryCap := mcpserver.NewQueryCapability(dir, meta)
	t.Cleanup(func() { _ = queryCap.Close() })

	result, err := queryCap.Handle(context.Background(), "sql", map[string]any{
		"database": "test.sqlite",
		"sql":      "SELECT name FROM items ORDER BY name",
	})
	if err != nil {
		t.Fatalf("query.Handle(sql) error = %v", err)
	}

	rows, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("result.Data type = %T, want []map[string]any", result.Data)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0]["name"] != "Alpha" {
		t.Errorf("rows[0][name] = %v, want Alpha", rows[0]["name"])
	}
	if rows[1]["name"] != "Beta" {
		t.Errorf("rows[1][name] = %v, want Beta", rows[1]["name"])
	}
}

func TestIntegrationQueryDescribe(t *testing.T) {
	dir := setupTestDrive(t)
	meta, err := mcpserver.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	queryCap := mcpserver.NewQueryCapability(dir, meta)
	t.Cleanup(func() { _ = queryCap.Close() })

	result, err := queryCap.Handle(context.Background(), "describe", map[string]any{
		"database": "test.sqlite",
		"table":    "items",
	})
	if err != nil {
		t.Fatalf("query.Handle(describe) error = %v", err)
	}

	info, ok := result.Data.(query.SchemaInfo)
	if !ok {
		t.Fatalf("result.Data type = %T, want query.SchemaInfo", result.Data)
	}
	if len(info.Tables) != 1 {
		t.Fatalf("len(tables) = %d, want 1", len(info.Tables))
	}

	tbl := info.Tables[0]
	if tbl.Name != "items" {
		t.Errorf("table name = %q, want %q", tbl.Name, "items")
	}
	if got := strings.Join(tbl.Columns, ","); got != "id,name" {
		t.Errorf("columns = %q, want %q", got, "id,name")
	}
	if len(tbl.Samples) != 2 {
		t.Errorf("len(samples) = %d, want 2", len(tbl.Samples))
	}
}

func TestIntegrationSearchFailsGracefully(t *testing.T) {
	dir := setupTestDrive(t)
	meta, err := mcpserver.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	// No search.db exists in setupTestDrive, so search should fail.
	searchCap := mcpserver.NewSearchCapability(dir, meta)
	t.Cleanup(func() { _ = searchCap.Close() })

	_, err = searchCap.Handle(context.Background(), "keyword", map[string]any{
		"query": "test",
	})
	if err == nil {
		t.Fatal("expected error from search without search.db, got nil")
	}
	// Should mention the missing index or sqlite3.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "search index not found") && !strings.Contains(errMsg, "sqlite3 not found") {
		t.Errorf("unexpected error message: %q", errMsg)
	}
}

func TestIntegrationServerToolsList(t *testing.T) {
	dir := setupTestDrive(t)
	meta, err := mcpserver.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	vault := mcpserver.NewVaultCapability(dir, meta)
	queryCap := mcpserver.NewQueryCapability(dir, meta)
	searchCap := mcpserver.NewSearchCapability(dir, meta)

	srv := mcpserver.NewServer(vault, queryCap, searchCap)
	t.Cleanup(func() { _ = srv.Close() })

	tools := srv.Tools()
	if len(tools) != 9 {
		t.Fatalf("len(tools) = %d, want 9", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{
		"vault_sources",
		"vault_databases",
		"vault_maps",
		"vault_stats",
		"query_describe",
		"query_sql",
		"search_keyword",
		"search_semantic",
		"search_read",
	} {
		if !names[expected] {
			t.Errorf("missing tool %q in %v", expected, names)
		}
	}
}
