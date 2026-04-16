package apply

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

		// Route by acquisition strategy.
		switch {
		case recipe.URL != "" || recipe.URLPattern != "":
			// Direct download or dated-pattern resolution.
			entry, err := downloadItem(root, id, recipe)
			if err != nil {
				return err
			}
			m.Realized.Entries = append(m.Realized.Entries, entry)

		case len(recipe.Platforms) > 0:
			// Platform-specific binary download.
			entry, err := downloadPlatformItem(root, id, recipe)
			if err != nil {
				// Platform not available is a warning, not fatal.
				// Vaults often target a different platform than the build host.
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", id, err)
				continue
			}
			m.Realized.Entries = append(m.Realized.Entries, entry)

		case recipe.Strategy == "build" && recipe.Build != nil:
			// Build via Docker (svalbard-tools container).
			entry, err := buildItem(root, id, recipe)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s: build failed: %v\n", id, err)
				continue
			}
			m.Realized.Entries = append(m.Realized.Entries, entry)

		default:
			fmt.Fprintf(os.Stderr, "skip %s: no download URL, platforms, or build config\n", id)
		}
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

// hostPlatform returns the svalbard platform string for the current machine.
func hostPlatform() string {
	os := runtime.GOOS   // "darwin", "linux"
	arch := runtime.GOARCH // "arm64", "amd64"
	osName := os
	if os == "darwin" {
		osName = "macos"
	}
	archName := arch
	if arch == "amd64" {
		archName = "x86_64"
	}
	return osName + "-" + archName
}

// downloadItem handles url/url_pattern recipes.
func downloadItem(root, id string, recipe catalog.Item) (manifest.RealizedEntry, error) {
	resolvedURL, err := resolver.Resolve(recipe.URL, recipe.URLPattern)
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("resolving URL for %s: %w", id, err)
	}
	return fetchAndRecord(root, id, recipe, resolvedURL)
}

// downloadPlatformItem handles platform-specific binary downloads.
// Tries multiple platform key conventions since recipes may use either
// "darwin-arm64" (Go convention) or "macos-arm64" (Svalbard convention).
func downloadPlatformItem(root, id string, recipe catalog.Item) (manifest.RealizedEntry, error) {
	candidates := hostPlatformCandidates()
	for _, platform := range candidates {
		if platformURL, ok := recipe.Platforms[platform]; ok {
			return fetchAndRecord(root, id, recipe, platformURL)
		}
	}
	return manifest.RealizedEntry{}, fmt.Errorf("no download URL for %s on platform %v (available: %v)", id, candidates, platformKeys(recipe.Platforms))
}

// hostPlatformCandidates returns platform keys to try, in preference order.
// Handles both naming conventions: "macos-arm64" and "darwin-arm64".
func hostPlatformCandidates() []string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	archName := goarch
	if goarch == "amd64" {
		archName = "x86_64"
	}

	candidates := []string{goos + "-" + archName}
	if goos == "darwin" {
		candidates = append(candidates, "macos-"+archName)
	} else if goos == "linux" {
		// Also try aarch64 alias for arm64
		if goarch == "arm64" {
			candidates = append(candidates, goos+"-aarch64")
		}
	}
	return candidates
}

func platformKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// fetchAndRecord downloads a file and returns a RealizedEntry.
func fetchAndRecord(root, id string, recipe catalog.Item, dlURL string) (manifest.RealizedEntry, error) {
	typeDir := toolkit.TypeDirs[recipe.Type]
	filename := recipe.Filename
	if filename == "" {
		filename = filenameFromURL(dlURL)
	}
	destPath := filepath.Join(root, typeDir, filename)

	result, err := downloader.Download(context.Background(), dlURL, destPath, "")
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("downloading %s: %w", id, err)
	}

	fileInfo, err := os.Stat(destPath)
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("stat %s: %w", destPath, err)
	}

	return manifest.RealizedEntry{
		ID:             id,
		Type:           recipe.Type,
		Filename:       filename,
		RelativePath:   filepath.Join(typeDir, filename),
		SizeBytes:      fileInfo.Size(),
		ChecksumSHA256: result.SHA256,
		SourceStrategy: recipe.Strategy,
	}, nil
}

// buildItem runs a Docker-based builder for strategy=build recipes.
func buildItem(root, id string, recipe catalog.Item) (manifest.RealizedEntry, error) {
	family := recipe.Build.Family
	typeDir := toolkit.TypeDirs[recipe.Type]
	filename := recipe.Filename
	if filename == "" && recipe.Build != nil {
		filename = id + "." + recipe.Type
	}
	destPath := filepath.Join(root, typeDir, filename)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return manifest.RealizedEntry{}, err
	}

	fmt.Fprintf(os.Stderr, "building %s (family=%s) via docker...\n", id, family)

	// Find the builder script path — embedded recipes include builders/
	builderScript := fmt.Sprintf("recipes/builders/%s.py", strings.ReplaceAll(family, "-", "_"))

	// Run the builder in the svalbard-tools container.
	// Mount the vault root and the recipe builders.
	args := []string{
		"docker", "run", "--rm",
		"-v", root + ":/vault",
		"-v", filepath.Join(root, "..") + "/recipes/builders:/builders:ro",
		"svalbard-tools",
		"python3", "/builders/" + filepath.Base(builderScript),
		"--output", "/vault/" + filepath.Join(typeDir, filename),
	}

	// Pass build-specific args from the recipe.
	// This is builder-family specific — for now pass the recipe ID.
	// Individual builders parse their own args.

	cmd := execCommand(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	if err := cmd.Run(); err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("building %s: %w", id, err)
	}

	fileInfo, err := os.Stat(destPath)
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("stat built artifact %s: %w", destPath, err)
	}

	sha256, _ := downloader.ComputeSHA256(destPath)

	return manifest.RealizedEntry{
		ID:             id,
		Type:           recipe.Type,
		Filename:       filename,
		RelativePath:   filepath.Join(typeDir, filename),
		SizeBytes:      fileInfo.Size(),
		ChecksumSHA256: sha256,
		SourceStrategy: "build",
	}, nil
}

// execCommand is a variable for testing — defaults to exec.Command.
var execCommand = exec.Command

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
