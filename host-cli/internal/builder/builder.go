// Package builder provides Go-native build handlers for recipe families
// that don't require Docker. Complex pipelines (geodata, ZIM scraping)
// stay Docker-based in apply.go; simple tasks live here.
//
// Build dispatch order:
//  1. If recipe.Build.Steps is non-empty → pipeline executor
//  2. If family is "python-venv" → custom native handler
//  3. If family is "app-bundle" → converted to pipeline internally
//  4. Otherwise → not handled (caller falls back to Docker)
package builder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

const (
	ToolsImageVersion  = "0.2.0"
	BaseToolsImage     = "ghcr.io/pkronstrom/svalbard-tools:" + ToolsImageVersion
	BrowserToolsImage  = "ghcr.io/pkronstrom/svalbard-tools:" + ToolsImageVersion + "-browser"
	DefaultDockerImage = BaseToolsImage
)

// Options provides context from the apply layer to builders.
type Options struct {
	Ctx        context.Context
	Platforms  []string
	DesiredIDs map[string]bool
	OnStatus   func(step string) // legacy status adapter
	OnEvent    func(BuildEvent)
}

// Func is the signature for a native builder.
type Func func(root string, recipe catalog.Item, cat *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error)

// Dispatch returns a native builder for the recipe, if one can handle it.
// Priority: explicit steps → python-venv handler → app-bundle conversion.
func Dispatch(recipe catalog.Item) (Func, bool) {
	if recipe.Build == nil {
		return nil, false
	}

	// 1. Explicit pipeline steps.
	if len(recipe.Build.Steps) > 0 {
		return buildPipeline, true
	}

	// 2. python-venv needs custom orchestration to collect sibling recipes.
	if recipe.Build.Family == "python-venv" {
		return buildPythonVenv, true
	}

	if recipe.Build.Family == "reference-static" {
		return buildReferenceStatic, true
	}
	if recipe.Build.Family == "zim-compact" {
		return buildZIMCompact, true
	}
	switch recipe.Build.Family {
	case "vector-static":
		return buildVectorStatic, true
	case "vector-service":
		return buildVectorService, true
	}
	// 3. App bundles use the shared Go download/extract pipeline.
	if recipe.Build.Family == "app-bundle" {
		return buildAppBundleAsPipeline, true
	}

	return nil, false
}

// buildAppBundleAsPipeline converts an app-bundle recipe into pipeline steps
// and executes it. This provides backward compatibility for existing recipes
// that use family: app-bundle with source_url or assets.
func buildAppBundleAsPipeline(root string, recipe catalog.Item, cat *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	switch {
	case recipe.Build.SourceURL != "":
		recipe.Build.Steps = []catalog.BuildStep{
			{Download: "{source_url}", Dest: "{workdir}/archive"},
			{Extract: "{workdir}/archive", Dest: "{output_dir}"},
			{Verify: "{output_dir}", NotEmpty: true},
		}
	case len(recipe.Build.Assets) > 0:
		steps := make([]catalog.BuildStep, 0, len(recipe.Build.Assets)+1)
		for _, asset := range recipe.Build.Assets {
			dest, err := safeAssetDestination(asset.Dest)
			if err != nil {
				return nil, fmt.Errorf("app-bundle %s: %w", recipe.ID, err)
			}
			steps = append(steps, catalog.BuildStep{
				Download: asset.URL,
				Dest:     filepath.Join("{output_dir}", dest),
			})
		}
		recipe.Build.Steps = append(steps, catalog.BuildStep{Verify: "{output_dir}", NotEmpty: true})
	default:
		return nil, fmt.Errorf("app-bundle %s: source_url or assets required", recipe.ID)
	}
	return buildPipeline(root, recipe, cat, opts)
}

func safeAssetDestination(dest string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(dest))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid asset destination %q", dest)
	}
	return clean, nil
}
