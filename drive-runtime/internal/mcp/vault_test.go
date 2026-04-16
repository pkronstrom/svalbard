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
	if cap.Description() == "" {
		t.Error("expected non-empty description")
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
