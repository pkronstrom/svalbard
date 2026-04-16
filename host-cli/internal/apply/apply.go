package apply

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/mapview"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"github.com/pkronstrom/svalbard/host-cli/internal/resolver"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// Run executes a reconciliation plan against the vault at root, mutating the
// manifest's realized state to reflect what was applied.
func Run(root string, m *manifest.Manifest, plan planner.Plan, cat *catalog.Catalog) error {
	// Process downloads: resolve URLs, download files, add realized entries.
	for _, id := range plan.ToDownload {
		recipe, ok := cat.RecipeByID(id)
		if !ok {
			return fmt.Errorf("recipe %q not found in catalog", id)
		}

		resolvedURL, err := resolver.Resolve(recipe.URL, recipe.URLPattern)
		if err != nil {
			return fmt.Errorf("resolving URL for %s: %w", id, err)
		}

		typeDir := toolkit.TypeDirs[recipe.Type]

		filename := recipe.Filename
		if filename == "" {
			filename = filenameFromURL(resolvedURL)
		}

		destPath := filepath.Join(root, typeDir, filename)

		result, err := downloader.Download(context.Background(), resolvedURL, destPath, "")
		if err != nil {
			return fmt.Errorf("downloading %s: %w", id, err)
		}

		fileInfo, err := os.Stat(destPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", destPath, err)
		}

		m.Realized.Entries = append(m.Realized.Entries, manifest.RealizedEntry{
			ID:             id,
			Type:           recipe.Type,
			Filename:       filename,
			RelativePath:   filepath.Join(typeDir, filename),
			SizeBytes:      fileInfo.Size(),
			ChecksumSHA256: result.SHA256,
			SourceStrategy: recipe.Strategy,
		})
	}

	// Process removals: remove realized entries and delete artifact files.
	removeSet := make(map[string]struct{}, len(plan.ToRemove))
	for _, id := range plan.ToRemove {
		removeSet[id] = struct{}{}
	}
	for _, e := range m.Realized.Entries {
		if _, ok := removeSet[e.ID]; ok {
			// Best-effort file deletion.
			os.Remove(filepath.Join(root, e.RelativePath))
		}
	}
	var filtered []manifest.RealizedEntry
	for _, e := range m.Realized.Entries {
		if _, ok := removeSet[e.ID]; !ok {
			filtered = append(filtered, e)
		}
	}
	m.Realized.Entries = filtered

	// Update applied timestamp.
	// Regenerate runtime assets.
	presetName := ""
	if len(m.Desired.Presets) > 0 {
		presetName = m.Desired.Presets[0]
	}
	if err := toolkit.Generate(root, m.Realized.Entries, presetName); err != nil {
		return fmt.Errorf("generating toolkit: %w", err)
	}
	if err := syncMapViewer(root, m.Realized.Entries); err != nil {
		return fmt.Errorf("generating map viewer: %w", err)
	}

	// Update applied timestamp only after all runtime assets have been regenerated.
	m.Realized.AppliedAt = time.Now().UTC().Format(time.RFC3339)

	return nil
}

func syncMapViewer(root string, entries []manifest.RealizedEntry) error {
	viewerPath := filepath.Join(root, "apps", "map", "index.html")
	layers := pmtilesLayers(entries)
	if len(layers) == 0 {
		if err := os.Remove(viewerPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return mapview.Generate(root, layers)
}

func pmtilesLayers(entries []manifest.RealizedEntry) []mapview.Layer {
	layers := make([]mapview.Layer, 0)
	for _, e := range entries {
		if e.Type != "pmtiles" {
			continue
		}
		layers = append(layers, mapview.Layer{
			Name:     e.ID,
			Filename: e.Filename,
			Category: "basemap",
		})
	}
	return layers
}

// filenameFromURL extracts the last path segment from a URL, stripping any
// query string or fragment.
func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Fallback: split on "/" and take the last segment.
		parts := strings.Split(rawURL, "/")
		return parts[len(parts)-1]
	}
	segments := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	return segments[len(segments)-1]
}
