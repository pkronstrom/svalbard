package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
