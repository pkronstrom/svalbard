package mcp_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

func TestVaultCapabilityToolName(t *testing.T) {
	cap := mcp.NewVaultCapability(t.TempDir(), mcp.DriveMetadata{})
	if cap.Tool() != "vault" {
		t.Errorf("expected tool name 'vault', got %q", cap.Tool())
	}
	actions := cap.Actions()
	if len(actions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(actions))
	}
	names := map[string]bool{}
	for _, a := range actions {
		names[a.Name] = true
	}
	for _, want := range []string{"sources", "databases", "maps", "stats"} {
		if !names[want] {
			t.Errorf("missing action %q", want)
		}
	}
}

func TestVaultSourcesAction(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "zim", "wikipedia.zim"), []byte("zim-content"))

	meta := mcp.DriveMetadata{
		Manifest: map[string]string{},
		Recipes: map[string]mcp.RecipeMeta{
			"wikipedia": {
				ID:          "wikipedia",
				Type:        "zim",
				Description: "English Wikipedia",
				Tags:        []string{"reference", "english"},
			},
		},
	}

	cap := mcp.NewVaultCapability(dir, meta)
	result, err := cap.Handle(context.Background(), "sources", map[string]any{})
	if err != nil {
		t.Fatalf("Handle(sources) error = %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected non-nil data")
	}

	// The result.Data should be a slice of SourceInfo (via inspect package)
	// We can type-assert to check enrichment
	sources, ok := result.Data.([]inspect.SourceInfo)
	if !ok {
		// It's returned from inspect.Sources, should be the right type
		t.Fatalf("expected []inspect.SourceInfo, got %T", result.Data)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Description != "English Wikipedia" {
		t.Errorf("expected enriched description 'English Wikipedia', got %q", sources[0].Description)
	}
	if len(sources[0].Tags) != 2 {
		t.Errorf("expected 2 enriched tags, got %d", len(sources[0].Tags))
	}
}

func TestVaultSourcesPagesAndFiltersTheInventory(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 15; i++ {
		writeTestFile(t, filepath.Join(root, "zim", fmt.Sprintf("source-%02d.zim", i)), []byte("zim"))
	}
	cap := mcp.NewVaultCapability(root, mcp.DriveMetadata{})
	first, err := cap.Handle(context.Background(), "sources", map[string]any{})
	if err != nil || len(first.Data.([]inspect.SourceInfo)) != 12 || !strings.Contains(first.Text, "of 15 sources") {
		t.Fatalf("first page = %+v, %v", first, err)
	}
	last, err := cap.Handle(context.Background(), "sources", map[string]any{"offset": 12})
	if err != nil || len(last.Data.([]inspect.SourceInfo)) != 3 || last.Text != "" {
		t.Fatalf("last page = %+v, %v", last, err)
	}
	match, err := cap.Handle(context.Background(), "sources", map[string]any{"match": "SOURCE-14"})
	if err != nil || len(match.Data.([]inspect.SourceInfo)) != 1 || match.Data.([]inspect.SourceInfo)[0].ID != "source-14" {
		t.Fatalf("source match = %+v, %v", match, err)
	}
}

// A dated archive is described by the recipe the manifest names for it: the
// recipe's id is not the file's name.
func TestVaultSourcesDescribesFilesByTheirManifestRecipe(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "zim", "wikem_en_all_maxi_2026-04.zim"), []byte("zim"))
	writeTestFile(t, filepath.Join(dir, "zim", "handmade.zim"), []byte("zim"))
	writeTestFile(t, filepath.Join(dir, "zim", "unknown_2026-01.zim"), []byte("zim"))
	writeTestFile(t, filepath.Join(dir, "manifest.yaml"), []byte(`version: 1
realized:
    entries:
        - id: wikem
          type: zim
          filename: wikem_en_all_maxi_2026-04.zim
          relative_path: zim/wikem_en_all_maxi_2026-04.zim
`))
	writeTestFile(t, filepath.Join(dir, ".svalbard", "recipes.json"), []byte(`{
		"wikem": {"id": "wikem", "type": "zim", "description": "WikEM, an emergency medicine wiki", "tags": ["medicine"]},
		"handmade": {"id": "handmade", "type": "zim", "description": "Named like its file"}
	}`))
	meta, err := mcp.LoadMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}

	result, err := mcp.NewVaultCapability(dir, meta).Handle(context.Background(), "sources", map[string]any{})
	if err != nil {
		t.Fatalf("Handle(sources) error = %v", err)
	}
	described := map[string]inspect.SourceInfo{}
	for _, source := range result.Data.([]inspect.SourceInfo) {
		described[source.ID] = source
	}
	if got := described["wikem_en_all_maxi_2026-04"]; got.Description != "WikEM, an emergency medicine wiki" || len(got.Tags) != 1 {
		t.Errorf("dated archive = %+v, want the wikem recipe's description and tags", got)
	}
	// No manifest entry: the recipe named like the file still applies.
	if got := described["handmade"]; got.Description != "Named like its file" {
		t.Errorf("archive without a manifest entry = %+v, want the same-named recipe's description", got)
	}
	if got := described["unknown_2026-01"]; got.Description != "" {
		t.Errorf("archive with no recipe = %+v, want no description", got)
	}
}

func TestVaultStatsAction(t *testing.T) {
	dir := t.TempDir()
	manifest := "preset: test-pack\nregion: nordic\ncreated: 2026-01-01\n"
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "zim", "wiki.zim"), []byte("data"))

	cap := mcp.NewVaultCapability(dir, mcp.DriveMetadata{
		Manifest: map[string]string{},
		Recipes:  map[string]mcp.RecipeMeta{},
	})
	result, err := cap.Handle(context.Background(), "stats", map[string]any{})
	if err != nil {
		t.Fatalf("Handle(stats) error = %v", err)
	}
	if result.Data == nil {
		t.Fatal("expected non-nil data")
	}

	stats, ok := result.Data.(inspect.DriveStats)
	if !ok {
		t.Fatalf("expected inspect.DriveStats, got %T", result.Data)
	}
	if stats.Preset != "test-pack" {
		t.Errorf("expected preset 'test-pack', got %q", stats.Preset)
	}
	if stats.Counts["zim"] != 1 {
		t.Errorf("expected zim count 1, got %d", stats.Counts["zim"])
	}
}

func TestVaultUnknownActionReturnsError(t *testing.T) {
	cap := mcp.NewVaultCapability(t.TempDir(), mcp.DriveMetadata{})
	_, err := cap.Handle(context.Background(), "nonexistent", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestVaultDatabasesAction(t *testing.T) {
	dir := t.TempDir()
	// Empty drive - databases dir doesn't exist
	cap := mcp.NewVaultCapability(dir, mcp.DriveMetadata{
		Manifest: map[string]string{},
		Recipes:  map[string]mcp.RecipeMeta{},
	})
	result, err := cap.Handle(context.Background(), "databases", map[string]any{})
	if err != nil {
		t.Fatalf("Handle(databases) error = %v", err)
	}
	// nil data is fine for empty
	_ = result
}

func TestVaultMapsAction(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "maps", "finland.pmtiles"), []byte("tiles"))

	cap := mcp.NewVaultCapability(dir, mcp.DriveMetadata{
		Manifest: map[string]string{},
		Recipes:  map[string]mcp.RecipeMeta{},
	})
	result, err := cap.Handle(context.Background(), "maps", map[string]any{})
	if err != nil {
		t.Fatalf("Handle(maps) error = %v", err)
	}
	maps, ok := result.Data.([]inspect.MapInfo)
	if !ok {
		t.Fatalf("expected []inspect.MapInfo, got %T", result.Data)
	}
	if len(maps) != 1 {
		t.Fatalf("expected 1 map, got %d", len(maps))
	}
	if maps[0].Name != "finland.pmtiles" {
		t.Errorf("expected 'finland.pmtiles', got %q", maps[0].Name)
	}
}

func TestVaultClose(t *testing.T) {
	cap := mcp.NewVaultCapability(t.TempDir(), mcp.DriveMetadata{})
	if err := cap.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func TestVaultDatabasesShowsGpkgTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data", "natural_earth_vector.gpkg")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"CREATE TABLE gpkg_contents (table_name TEXT PRIMARY KEY, data_type TEXT)",
		"CREATE TABLE ne_110m_admin_0_countries (fid INTEGER PRIMARY KEY, geom BLOB, NAME TEXT, POP_EST REAL)",
		"INSERT INTO gpkg_contents VALUES ('ne_110m_admin_0_countries', 'features')",
		"INSERT INTO ne_110m_admin_0_countries VALUES (1, X'4750000100000000', 'Finland', 5520314)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	meta := mcp.DriveMetadata{Manifest: map[string]string{}, Recipes: map[string]mcp.RecipeMeta{}}
	result, err := mcp.NewVaultCapability(dir, meta).Handle(context.Background(), "databases", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	dbs, ok := result.Data.([]inspect.DatabaseInfo)
	if !ok || len(dbs) != 1 || dbs[0].Name != "natural_earth_vector.gpkg" || len(dbs[0].Tables) != 1 || dbs[0].Tables[0] != "ne_110m_admin_0_countries" {
		t.Fatalf("databases = %#v, want the GeoPackage with its countries table", result.Data)
	}

	rows, err := mcp.NewQueryCapability(dir, meta).Handle(context.Background(), "sql", map[string]any{
		"database": "natural_earth_vector.gpkg", "sql": "SELECT NAME, POP_EST, geom FROM ne_110m_admin_0_countries LIMIT 3",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(rows.Data)
	if got := string(data); !strings.Contains(got, `"NAME":"Finland"`) || !strings.Contains(got, `"POP_EST":5520314`) || !strings.Contains(got, `"geom":"\u003cGeoPackage geometry, 8 bytes\u003e"`) {
		t.Errorf("query_sql = %s, want the name, the population and the geometry by its size", got)
	}
}
