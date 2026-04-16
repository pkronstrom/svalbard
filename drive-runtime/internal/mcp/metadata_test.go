package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

func TestLoadMetadataWithManifestAndRecipes(t *testing.T) {
	dir := t.TempDir()

	// Write a manifest.yaml
	manifest := "preset: survival\nregion: finland\ncreated: 2025-01-01\n"
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Write recipes.json
	recipes := []mcp.RecipeMeta{
		{
			ID:          "wiki-fi",
			Type:        "zim",
			Description: "Finnish Wikipedia",
			Tags:        []string{"reference", "finnish"},
		},
		{
			ID:          "osm-fi",
			Type:        "map",
			Description: "OpenStreetMap Finland",
			Tags:        []string{"maps"},
		},
	}
	recipesData, _ := json.Marshal(recipes)
	svalbardDir := filepath.Join(dir, ".svalbard")
	if err := os.Mkdir(svalbardDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svalbardDir, "recipes.json"), recipesData, 0644); err != nil {
		t.Fatal(err)
	}

	dm, err := mcp.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check manifest
	if dm.Manifest["preset"] != "survival" {
		t.Errorf("expected preset 'survival', got %q", dm.Manifest["preset"])
	}
	if dm.Manifest["region"] != "finland" {
		t.Errorf("expected region 'finland', got %q", dm.Manifest["region"])
	}
	if dm.Manifest["created"] != "2025-01-01" {
		t.Errorf("expected created '2025-01-01', got %q", dm.Manifest["created"])
	}

	// Check recipes
	if len(dm.Recipes) != 2 {
		t.Fatalf("expected 2 recipes, got %d", len(dm.Recipes))
	}
	wiki, ok := dm.Recipes["wiki-fi"]
	if !ok {
		t.Fatal("expected recipe 'wiki-fi'")
	}
	if wiki.Description != "Finnish Wikipedia" {
		t.Errorf("expected description 'Finnish Wikipedia', got %q", wiki.Description)
	}
	if len(wiki.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(wiki.Tags))
	}
}

func TestLoadMetadataMissingFiles(t *testing.T) {
	dir := t.TempDir()

	dm, err := mcp.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("expected no error for missing files, got: %v", err)
	}
	if len(dm.Manifest) != 0 {
		t.Errorf("expected empty manifest, got %v", dm.Manifest)
	}
	if len(dm.Recipes) != 0 {
		t.Errorf("expected empty recipes, got %v", dm.Recipes)
	}
}

func TestLoadMetadataManifestOnly(t *testing.T) {
	dir := t.TempDir()

	manifest := "preset: offline-kit\n# comment line\n\nregion: nordic\n"
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	dm, err := mcp.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dm.Manifest["preset"] != "offline-kit" {
		t.Errorf("expected preset 'offline-kit', got %q", dm.Manifest["preset"])
	}
	if dm.Manifest["region"] != "nordic" {
		t.Errorf("expected region 'nordic', got %q", dm.Manifest["region"])
	}
	if len(dm.Recipes) != 0 {
		t.Errorf("expected empty recipes, got %v", dm.Recipes)
	}
}

func TestLoadMetadataInvalidRecipesJSON(t *testing.T) {
	dir := t.TempDir()

	svalbardDir := filepath.Join(dir, ".svalbard")
	if err := os.Mkdir(svalbardDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svalbardDir, "recipes.json"), []byte("{not valid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := mcp.LoadMetadata(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
