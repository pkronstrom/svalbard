package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	manifest, err := readManifest(filepath.Join(driveRoot, "manifest.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return dm, err
	}
	dm.Manifest = manifest

	recipes, err := readRecipes(filepath.Join(driveRoot, ".svalbard", "recipes.json"))
	if err != nil && !os.IsNotExist(err) {
		return dm, err
	}
	dm.Recipes = recipes

	return dm, nil
}

// readManifest does simple key:value parsing of manifest.yaml.
// This is intentionally inline (not reusing inspect) — will be DRYed up in Task 5.
func readManifest(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return map[string]string{}, err
	}
	defer file.Close()

	meta := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key != "" {
			meta[key] = val
		}
	}
	return meta, scanner.Err()
}

// readRecipes parses .svalbard/recipes.json into a map keyed by recipe ID.
func readRecipes(path string) (map[string]RecipeMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]RecipeMeta{}, err
	}

	var list []RecipeMeta
	if err := json.Unmarshal(data, &list); err != nil {
		return map[string]RecipeMeta{}, err
	}

	out := make(map[string]RecipeMeta, len(list))
	for _, r := range list {
		out[r.ID] = r
	}
	return out, nil
}
