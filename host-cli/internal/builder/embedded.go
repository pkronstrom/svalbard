package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// embeddedMarker marks a directory whose contents host-cli writes from files
// it embeds (toolkit.Generate fills it on every apply).
const embeddedMarker = ".svalbard-embedded"

// buildEmbedded realizes a recipe whose contents ship inside host-cli: it
// creates <type dir>/<id>/ with a marker and downloads nothing.
func buildEmbedded(root string, recipe catalog.Item, _ *catalog.Catalog, _ Options) ([]manifest.RealizedEntry, error) {
	typeDir, ok := toolkit.TypeDirs[recipe.Type]
	if !ok {
		return nil, fmt.Errorf("embedded %s: type %q has no vault directory", recipe.ID, recipe.Type)
	}
	rel := filepath.Join(typeDir, recipe.ID)
	dir := filepath.Join(root, rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, embeddedMarker), []byte(recipe.ID+"\n"), 0o644); err != nil {
		return nil, err
	}
	return []manifest.RealizedEntry{{
		ID: recipe.ID, Type: recipe.Type, Filename: recipe.ID,
		RelativePath: rel, SourceStrategy: "build",
	}}, nil
}
