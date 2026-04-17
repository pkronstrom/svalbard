// Package builder provides Go-native build handlers for recipe families
// that don't require Docker. Complex pipelines (geodata, ZIM scraping)
// stay Docker-based in apply.go; simple tasks live here.
//
// Build dispatch order:
//  1. If recipe.Build.Steps is non-empty → pipeline executor
//  2. If family matches a native handler → special handler (python-venv)
//  3. If family is "app-bundle" → converted to pipeline internally
//  4. Otherwise → not handled (caller falls back to Docker)
package builder

import (
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// Func is the signature for a native builder. It receives the vault root,
// the recipe to build, the full catalog (for cross-recipe lookups like
// python-package collection), and the target platforms to build for.
type Func func(root string, recipe catalog.Item, cat *catalog.Catalog, platforms []string) ([]manifest.RealizedEntry, error)

// special maps build family names to handlers that need custom orchestration
// beyond the generic pipeline (e.g. python-venv collects sibling recipes).
var special = map[string]Func{
	"python-venv": buildPythonVenv,
}

// Dispatch returns a native builder for the recipe, if one can handle it.
// Priority: explicit steps → special handler → app-bundle conversion.
func Dispatch(recipe catalog.Item) (Func, bool) {
	if recipe.Build == nil {
		return nil, false
	}

	// 1. Explicit pipeline steps.
	if len(recipe.Build.Steps) > 0 {
		return buildPipeline, true
	}

	// 2. Special handlers (python-venv, etc.).
	if fn, ok := special[recipe.Build.Family]; ok {
		return fn, ok
	}

	// 3. App-bundle: convert to pipeline internally.
	if recipe.Build.Family == "app-bundle" {
		return buildAppBundleAsPipeline, true
	}

	return nil, false
}

// buildAppBundleAsPipeline converts an app-bundle recipe into pipeline steps
// and executes it. This provides backward compatibility for existing recipes
// that use family: app-bundle with source_url or assets.
func buildAppBundleAsPipeline(root string, recipe catalog.Item, cat *catalog.Catalog, platforms []string) ([]manifest.RealizedEntry, error) {
	if recipe.Build.SourceURL != "" {
		recipe.Build.Steps = []catalog.BuildStep{
			{Download: "{source_url}", Dest: "{workdir}/archive"},
			{Extract: "{workdir}/archive", Dest: "{output_dir}"},
			{Verify: "{output_dir}", NotEmpty: true},
		}
	}
	return buildPipeline(root, recipe, cat, platforms)
}
