package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
)

// RecipeMeta holds parsed metadata for a single recipe entry.
type RecipeMeta struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Viewer      map[string]any `json:"viewer,omitempty"`
	Build       map[string]any `json:"build,omitempty"`
}

// DriveMetadata aggregates drive-level metadata from manifest.yaml and recipes.json.
type DriveMetadata struct {
	Manifest map[string]string
	Recipes  map[string]RecipeMeta
}

// LoadMetadata reads manifest.yaml and recipes.json from driveRoot.
// Missing files are not errors — the corresponding field will be empty.
func LoadMetadata(driveRoot string) (DriveMetadata, error) {
	dm := DriveMetadata{
		Manifest: map[string]string{},
		Recipes:  map[string]RecipeMeta{},
	}

	// Reuse inspect's manifest parser (DRY — same logic used by TUI).
	manifest, _ := inspect.ReadManifestMetadata(filepath.Join(driveRoot, "manifest.yaml"))
	dm.Manifest = manifest

	recipes, err := readRecipes(filepath.Join(driveRoot, ".svalbard", "recipes.json"))
	if err != nil && !os.IsNotExist(err) {
		return dm, err
	}
	dm.Recipes = recipes

	return dm, nil
}

// readRecipes parses .svalbard/recipes.json into a map keyed by recipe ID.
// Accepts both object format {"id": {...}} and array format [{...}, ...].
func readRecipes(path string) (map[string]RecipeMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]RecipeMeta{}, err
	}

	// Try object format first ({"recipe-id": {...}, ...}) — this is what
	// toolkit_generator.py produces.
	var obj map[string]RecipeMeta
	if err := json.Unmarshal(data, &obj); err == nil && len(obj) > 0 {
		return obj, nil
	}

	// Fall back to array format ([{...}, ...]).
	var list []RecipeMeta
	if err := json.Unmarshal(data, &list); err != nil {
		return map[string]RecipeMeta{}, fmt.Errorf("invalid recipes.json: %w", err)
	}
	out := make(map[string]RecipeMeta, len(list))
	for _, r := range list {
		out[r.ID] = r
	}
	return out, nil
}
