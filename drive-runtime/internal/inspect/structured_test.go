package inspect_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
)

func TestSourcesReturnsZIMAndSQLiteFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "zim", "wikipedia.zim"), []byte("zim-data"))
	mustWriteFile(t, filepath.Join(dir, "data", "library.sqlite"), []byte("db-data"))
	mustWriteFile(t, filepath.Join(dir, "maps", "world.pmtiles"), []byte("map-data"))

	sources, err := inspect.Sources(dir)
	if err != nil {
		t.Fatalf("Sources() error = %v", err)
	}

	if len(sources) != 3 {
		t.Fatalf("expected 3 sources, got %d: %+v", len(sources), sources)
	}

	// Check we got one of each type
	typeSet := map[string]bool{}
	for _, s := range sources {
		typeSet[s.Type] = true
		if s.ID == "" {
			t.Errorf("source %q has empty ID", s.Name)
		}
		if s.Size == 0 {
			t.Errorf("source %q has zero size", s.Name)
		}
	}
	for _, wantType := range []string{"zim", "database", "map"} {
		if !typeSet[wantType] {
			t.Errorf("missing source type %q", wantType)
		}
	}
}

func TestSourcesFiltersByType(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "zim", "wikipedia.zim"), []byte("zim"))
	mustWriteFile(t, filepath.Join(dir, "data", "library.sqlite"), []byte("db"))
	mustWriteFile(t, filepath.Join(dir, "maps", "world.pmtiles"), []byte("map"))

	sources, err := inspect.Sources(dir, "zim")
	if err != nil {
		t.Fatalf("Sources(zim) error = %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source with filter 'zim', got %d", len(sources))
	}
	if sources[0].Type != "zim" {
		t.Errorf("expected type 'zim', got %q", sources[0].Type)
	}
	if sources[0].ID != "wikipedia" {
		t.Errorf("expected ID 'wikipedia', got %q", sources[0].ID)
	}
}

func TestSourcesEmptyDrive(t *testing.T) {
	dir := t.TempDir()
	sources, err := inspect.Sources(dir)
	if err != nil {
		t.Fatalf("Sources() error = %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestStatsReturnsDriveSummary(t *testing.T) {
	dir := t.TempDir()

	manifest := "preset: survival-32\nregion: finland\ncreated: 2026-01-01\n"
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(dir, "zim", "wiki.zim"), []byte("data"))
	mustWriteFile(t, filepath.Join(dir, "data", "db.sqlite"), []byte("db"))

	stats, err := inspect.Stats(dir)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	if stats.Preset != "survival-32" {
		t.Errorf("expected preset 'survival-32', got %q", stats.Preset)
	}
	if stats.Region != "finland" {
		t.Errorf("expected region 'finland', got %q", stats.Region)
	}
	if stats.Created != "2026-01-01" {
		t.Errorf("expected created '2026-01-01', got %q", stats.Created)
	}
	if stats.Counts["zim"] != 1 {
		t.Errorf("expected zim count 1, got %d", stats.Counts["zim"])
	}
	if stats.Counts["data"] != 1 {
		t.Errorf("expected data count 1, got %d", stats.Counts["data"])
	}
	if _, ok := stats.Sizes["zim"]; !ok {
		t.Error("expected zim size entry")
	}
}

func TestStatsEmptyDrive(t *testing.T) {
	dir := t.TempDir()
	stats, err := inspect.Stats(dir)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if len(stats.Counts) != 0 {
		t.Errorf("expected empty counts, got %v", stats.Counts)
	}
}

func TestDatabasesListsTables(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dataDir, "test.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open error = %v", err)
	}
	if _, err := db.Exec("CREATE TABLE articles (id INTEGER PRIMARY KEY, title TEXT)"); err != nil {
		t.Fatalf("CREATE TABLE error = %v", err)
	}
	if _, err := db.Exec("CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("CREATE TABLE error = %v", err)
	}
	db.Close()

	dbs, err := inspect.Databases(dir)
	if err != nil {
		t.Fatalf("Databases() error = %v", err)
	}
	if len(dbs) != 1 {
		t.Fatalf("expected 1 database, got %d", len(dbs))
	}

	dbInfo := dbs[0]
	if dbInfo.Name != "test.sqlite" {
		t.Errorf("expected name 'test.sqlite', got %q", dbInfo.Name)
	}
	if dbInfo.Path != "data/test.sqlite" {
		t.Errorf("expected path 'data/test.sqlite', got %q", dbInfo.Path)
	}
	if len(dbInfo.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %v", len(dbInfo.Tables), dbInfo.Tables)
	}

	tableSet := map[string]bool{}
	for _, tbl := range dbInfo.Tables {
		tableSet[tbl] = true
	}
	if !tableSet["articles"] {
		t.Error("missing table 'articles'")
	}
	if !tableSet["authors"] {
		t.Error("missing table 'authors'")
	}
}

func TestDatabasesEmptyDataDir(t *testing.T) {
	dir := t.TempDir()
	dbs, err := inspect.Databases(dir)
	if err != nil {
		t.Fatalf("Databases() error = %v", err)
	}
	if len(dbs) != 0 {
		t.Errorf("expected 0 databases, got %d", len(dbs))
	}
}

func TestMapsListsPMTilesFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "maps", "world.pmtiles"), []byte("tiles-data"))

	maps, err := inspect.Maps(dir)
	if err != nil {
		t.Fatalf("Maps() error = %v", err)
	}
	if len(maps) != 1 {
		t.Fatalf("expected 1 map, got %d", len(maps))
	}
	if maps[0].Name != "world.pmtiles" {
		t.Errorf("expected name 'world.pmtiles', got %q", maps[0].Name)
	}
	if maps[0].Size == 0 {
		t.Error("expected non-zero size")
	}
}
