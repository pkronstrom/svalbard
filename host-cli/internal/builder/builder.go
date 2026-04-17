// Package builder provides Go-native build handlers for recipe families
// that don't require Docker. Complex pipelines (geodata, ZIM scraping)
// stay Docker-based in apply.go; simple tasks live here.
package builder

import (
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// Func is the signature for a native builder. It receives the vault root,
// the recipe to build, the full catalog (for cross-recipe lookups like
// python-package collection), and the target platforms to build for.
type Func func(root string, recipe catalog.Item, cat *catalog.Catalog, platforms []string) ([]manifest.RealizedEntry, error)

// Native maps build family names to their Go-native handlers.
var Native = map[string]Func{
	"app-bundle":  buildAppBundle,
	"python-venv": buildPythonVenv,
}

// Dispatch returns the native builder for a family, if one exists.
func Dispatch(family string) (Func, bool) {
	fn, ok := Native[family]
	return fn, ok
}
