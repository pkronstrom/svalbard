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

// ProgressFunc reports per-item progress during apply. May be nil.
type ProgressFunc func(id, status string)

// Run executes a reconciliation plan against the vault at root, mutating the
// manifest's realized state to reflect what was applied.
// The optional onProgress callback reports per-item status ("active", "done", "failed").
func Run(root string, m *manifest.Manifest, plan planner.Plan, cat *catalog.Catalog, onProgress ...ProgressFunc) error {
	progress := func(id, status string) {}
	if len(onProgress) > 0 && onProgress[0] != nil {
		progress = onProgress[0]
	}

	// Process removals first (free up space).
	removeSet := make(map[string]struct{}, len(plan.ToRemove))
	for _, id := range plan.ToRemove {
		removeSet[id] = struct{}{}
		progress(id, "active")
	}
	for _, e := range m.Realized.Entries {
		if _, ok := removeSet[e.ID]; ok {
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
	for _, id := range plan.ToRemove {
		progress(id, "done")
	}

	// Process downloads: resolve URLs, download files, add realized entries.
	for _, id := range plan.ToDownload {
		progress(id, "active")
		recipe, ok := cat.RecipeByID(id)
		if !ok {
			return fmt.Errorf("recipe %q not found in catalog", id)
		}

		// Route by acquisition strategy.
		switch {
		case recipe.URL != "" || recipe.URLPattern != "":
			entry, err := downloadItem(root, id, recipe)
			if err != nil {
				progress(id, "failed")
				return err
			}
			m.Realized.Entries = append(m.Realized.Entries, entry)
			progress(id, "done")

		case len(recipe.Platforms) > 0:
			entries, err := downloadPlatformItems(root, id, recipe, m.Desired.Options.HostPlatforms)
			if err != nil {
				progress(id, "failed")
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", id, err)
				continue
			}
			m.Realized.Entries = append(m.Realized.Entries, entries...)
			progress(id, "done")

		case recipe.Strategy == "build" && recipe.Build != nil:
			entry, err := buildItem(root, id, recipe)
			if err != nil {
				progress(id, "failed")
				fmt.Fprintf(os.Stderr, "skip %s: build failed: %v\n", id, err)
				continue
			}
			m.Realized.Entries = append(m.Realized.Entries, entry)
			progress(id, "done")

		default:
			progress(id, "failed")
			fmt.Fprintf(os.Stderr, "skip %s: no download URL, platforms, or build config\n", id)
		}
	}

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

// downloadPlatformItems downloads platform-specific binaries for all target platforms.
// If targetPlatforms is empty, falls back to the current host platform only.
func downloadPlatformItems(root, id string, recipe catalog.Item, targetPlatforms []string) ([]manifest.RealizedEntry, error) {
	targets := targetPlatforms
	if len(targets) == 0 {
		targets = hostPlatformCandidates()
	}

	var entries []manifest.RealizedEntry
	for _, platform := range targets {
		// Try the platform key directly, then alias conventions
		candidates := platformCandidates(platform)
		var downloaded bool
		for _, candidate := range candidates {
			if platformURL, ok := recipe.Platforms[candidate]; ok {
				entry, err := fetchAndRecord(root, id, recipe, platformURL)
				if err != nil {
					return nil, err
				}
				entries = append(entries, entry)
				downloaded = true
				break
			}
		}
		if !downloaded {
			fmt.Fprintf(os.Stderr, "skip %s for %s: no download URL (available: %v)\n", id, platform, platformKeys(recipe.Platforms))
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no download URL for %s on any target platform %v (available: %v)", id, targets, platformKeys(recipe.Platforms))
	}
	return entries, nil
}

// platformCandidates returns platform keys to try for a given target platform,
// handling naming convention aliases (macos↔darwin, arm64↔aarch64).
func platformCandidates(platform string) []string {
	candidates := []string{platform}
	parts := strings.SplitN(platform, "-", 2)
	if len(parts) != 2 {
		return candidates
	}
	osName, archName := parts[0], parts[1]

	// OS aliases
	switch osName {
	case "macos":
		candidates = append(candidates, "darwin-"+archName)
	case "darwin":
		candidates = append(candidates, "macos-"+archName)
	}
	// Arch aliases
	if archName == "arm64" {
		candidates = append(candidates, osName+"-aarch64")
	}
	if archName == "x86_64" {
		candidates = append(candidates, osName+"-amd64")
	}
	return candidates
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
