package toolkit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

const recipesFile = "recipes.json"

// recipeMetadata is one entry of .svalbard/recipes.json, read by the
// drive-runtime MCP server to describe and tag vault sources.
type recipeMetadata struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Viewer      map[string]string `json:"viewer,omitempty"`
	Preview     *recipePreview    `json:"preview,omitempty"`
}

// writeRecipesMetadata writes catalog metadata for realized entries, keyed by
// recipe ID. Without any, a stale file from an earlier apply is removed.
func writeRecipesMetadata(ctx context.Context, root string, entries []manifest.RealizedEntry, recipes map[string]catalog.Item, notice func(string)) error {
	path := filepath.Join(root, svalbardDir, recipesFile)
	out := make(map[string]recipeMetadata)
	var previous map[string]recipeMetadata
	previousLoaded := false
	for _, entry := range entries {
		if _, listed := out[entry.ID]; listed {
			continue
		}
		recipe, ok := recipes[entry.ID]
		if !ok {
			continue
		}
		meta := recipeMetadata{ID: recipe.ID, Type: recipe.Type, Description: recipe.Description, Tags: recipe.Tags}
		if meta.Tags == nil {
			meta.Tags = []string{}
		}
		if recipe.Viewer != nil {
			meta.Viewer = map[string]string{"name": recipe.Viewer.Name, "category": recipe.Viewer.Category}
		}
		if recipe.Preview != nil {
			if !previousLoaded {
				if data, err := os.ReadFile(path); err == nil {
					_ = json.Unmarshal(data, &previous)
				}
				previousLoaded = true
			}
			meta.Preview = prepareRecipePreview(ctx, filepath.Join(root, svalbardDir, previewsDir), recipe, previous[entry.ID].Preview, notice)
		}
		out[entry.ID] = meta
	}
	if len(out) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".recipes-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
