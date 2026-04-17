package apply

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/builder"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/mapview"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"github.com/pkronstrom/svalbard/host-cli/internal/resolver"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
	"github.com/pkronstrom/svalbard/tui"
)

const maxWorkers = 4

// ProgressEvent reports per-item progress during apply.
type ProgressEvent struct {
	ID         string
	Status     string // tui.Status* constants
	Step       string // current build step (e.g. "wget", "warc2zim")
	Downloaded int64  // bytes so far (only meaningful during StatusActive)
	Total      int64  // total bytes (-1 if unknown)
	Error      string
}

// ProgressFunc reports per-item progress during apply.
type ProgressFunc func(ProgressEvent)

// downloadJob is a unit of work for the parallel download pool.
type downloadJob struct {
	id     string
	recipe catalog.Item
}

// downloadResult collects the outcome of a single download job.
type downloadResult struct {
	id      string
	entries []manifest.RealizedEntry
	err     error
}

// Run executes a reconciliation plan against the vault at root, mutating the
// manifest's realized state to reflect what was applied.
// The optional onProgress callback reports per-item status.
func Run(ctx context.Context, root string, m *manifest.Manifest, plan planner.Plan, cat *catalog.Catalog, onProgress ...ProgressFunc) error {
	progress := func(ev ProgressEvent) {}
	if len(onProgress) > 0 && onProgress[0] != nil {
		progress = onProgress[0]
	}

	slog.Info("apply started", "downloads", len(plan.ToDownload), "removals", len(plan.ToRemove), "vault", root)

	// Process removals first (free up space).
	removeSet := make(map[string]struct{}, len(plan.ToRemove))
	for _, id := range plan.ToRemove {
		removeSet[id] = struct{}{}
		progress(ProgressEvent{ID: id, Status: tui.StatusActive})
	}
	for _, e := range m.Realized.Entries {
		if _, ok := removeSet[e.ID]; ok {
			os.RemoveAll(filepath.Join(root, e.RelativePath))
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
		progress(ProgressEvent{ID: id, Status: tui.StatusDone})
	}

	// Build the desired ID set for native builders to filter against.
	desiredIDs := make(map[string]bool, len(plan.ToDownload))
	for _, id := range plan.ToDownload {
		desiredIDs[id] = true
	}

	// Partition downloads into parallel-safe (HTTP) and sequential (build) jobs.
	var httpJobs []downloadJob
	var buildJobs []downloadJob

	for _, id := range plan.ToDownload {
		recipe, ok := cat.RecipeByID(id)
		if !ok {
			progress(ProgressEvent{ID: id, Status: tui.StatusFailed, Error: fmt.Sprintf("recipe %q not found", id)})
			continue
		}
		if recipe.Type == "python-package" {
			continue // installed by python-venv builder
		}

		switch {
		case recipe.URL != "" || recipe.URLPattern != "" || len(recipe.Platforms) > 0:
			httpJobs = append(httpJobs, downloadJob{id: id, recipe: recipe})
		case recipe.Strategy == "build" && recipe.Build != nil:
			buildJobs = append(buildJobs, downloadJob{id: id, recipe: recipe})
		default:
			progress(ProgressEvent{ID: id, Status: tui.StatusFailed, Error: "no acquisition strategy"})
			slog.Warn("no acquisition strategy", "id", id)
		}
	}

	// Run all jobs (downloads + builds) in a shared worker pool.
	allJobs := append(httpJobs, buildJobs...)

	var mu sync.Mutex
	var applyErrors []string

	results := make(chan downloadResult, len(allJobs))
	jobs := make(chan downloadJob, len(allJobs))

	var wg sync.WaitGroup
	workerCount := maxWorkers
	if workerCount > len(allJobs) {
		workerCount = len(allJobs)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				progress(ProgressEvent{ID: job.id, Status: tui.StatusActive})

				var entries []manifest.RealizedEntry
				var err error

				// Throttled progress: report at most every 250ms to avoid flooding TUI.
				var lastReport time.Time
				dlProgress := func(downloaded, total int64) {
					now := time.Now()
					if now.Sub(lastReport) < 250*time.Millisecond {
						return
					}
					lastReport = now
					progress(ProgressEvent{
						ID: job.id, Status: tui.StatusActive,
						Downloaded: downloaded, Total: total,
					})
				}

				switch {
				case job.recipe.URL != "" || job.recipe.URLPattern != "":
					var entry manifest.RealizedEntry
					entry, err = downloadItem(ctx, root, job.id, job.recipe, dlProgress)
					if err == nil {
						entries = []manifest.RealizedEntry{entry}
					}

				case len(job.recipe.Platforms) > 0:
					entries, err = downloadPlatformItems(ctx, root, job.id, job.recipe, m.Desired.Options.HostPlatforms, dlProgress)

				case job.recipe.Strategy == "build" && job.recipe.Build != nil:
					jobID := job.id
					if nativeFn, ok := builder.Dispatch(job.recipe); ok {
						entries, err = nativeFn(root, job.recipe, cat, builder.Options{
							Ctx:        ctx,
							Platforms:  m.Desired.Options.HostPlatforms,
							DesiredIDs: desiredIDs,
							OnStatus: func(step string) {
								progress(ProgressEvent{ID: jobID, Status: tui.StatusActive, Step: step})
							},
						})
					} else {
						var entry manifest.RealizedEntry
						entry, err = buildItem(ctx, root, job.id, job.recipe)
						if err == nil {
							entries = []manifest.RealizedEntry{entry}
						}
					}
				}

				results <- downloadResult{id: job.id, entries: entries, err: err}
			}
		}()
	}

	for _, job := range allJobs {
		jobs <- job
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			progress(ProgressEvent{ID: res.id, Status: tui.StatusFailed, Error: res.err.Error()})
			slog.Warn("job failed", "id", res.id, "error", res.err)
			mu.Lock()
			applyErrors = append(applyErrors, fmt.Sprintf("%s: %s", res.id, res.err))
			mu.Unlock()
			continue
		}
		mu.Lock()
		m.Realized.Entries = append(m.Realized.Entries, res.entries...)
		mu.Unlock()
		progress(ProgressEvent{ID: res.id, Status: tui.StatusDone})
	}

	// Collect env vars and menu specs from all realized recipes.
	envVars := make(map[string]string)
	menus := make(map[string]catalog.MenuSpec)
	for _, e := range m.Realized.Entries {
		if recipe, ok := cat.RecipeByID(e.ID); ok {
			for k, v := range recipe.Env {
				envVars[k] = v
			}
			if recipe.Menu != nil {
				spec := *recipe.Menu
				// Use recipe description as label fallback when menu has no label.
				if spec.Label == "" && recipe.Description != "" {
					spec.Label = recipe.Description
				}
				menus[e.ID] = spec
			} else if recipe.Description != "" {
				menus[e.ID] = catalog.MenuSpec{Label: recipe.Description}
			}
		}
	}

	// Regenerate runtime assets.
	presetName := ""
	if len(m.Desired.Presets) > 0 {
		presetName = m.Desired.Presets[0]
	}
	if err := toolkit.Generate(root, m.Realized.Entries, presetName, toolkit.GenerateOpts{EnvVars: envVars, Menus: menus}); err != nil {
		return fmt.Errorf("generating toolkit: %w", err)
	}
	if err := syncMapViewer(root, m.Realized.Entries); err != nil {
		return fmt.Errorf("generating map viewer: %w", err)
	}

	m.Realized.AppliedAt = time.Now().UTC().Format(time.RFC3339)

	if len(applyErrors) > 0 {
		slog.Warn("apply completed with errors", "errors", len(applyErrors), "realized", len(m.Realized.Entries))
		return fmt.Errorf("%d item(s) failed", len(applyErrors))
	}

	slog.Info("apply completed", "realized", len(m.Realized.Entries))
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

func hostPlatform() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
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
func downloadItem(ctx context.Context, root, id string, recipe catalog.Item, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	resolvedURL, err := resolver.Resolve(recipe.URL, recipe.URLPattern)
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("resolving URL for %s: %w", id, err)
	}
	slog.Debug("resolved URL", "id", id, "url", resolvedURL)
	return fetchAndRecord(ctx, root, id, recipe, resolvedURL, onProgress)
}

// downloadPlatformItems downloads platform-specific binaries for all target platforms.
func downloadPlatformItems(ctx context.Context, root, id string, recipe catalog.Item, targetPlatforms []string, onProgress downloader.ProgressFunc) ([]manifest.RealizedEntry, error) {
	targets := targetPlatforms
	if len(targets) == 0 {
		targets = hostPlatformCandidates()
	}

	var entries []manifest.RealizedEntry
	for _, platform := range targets {
		candidates := platformCandidates(platform)
		var downloaded bool
		for _, candidate := range candidates {
			if platformURL, ok := recipe.Platforms[candidate]; ok {
				entry, err := fetchAndRecord(ctx, root, id, recipe, platformURL, onProgress)
				if err != nil {
					return nil, err
				}
				entries = append(entries, entry)
				downloaded = true
				break
			}
		}
		if !downloaded {
			slog.Debug("no platform URL", "id", id, "platform", platform, "available", platformKeys(recipe.Platforms))
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no download URL for %s on any target platform %v (available: %v)", id, targets, platformKeys(recipe.Platforms))
	}
	return entries, nil
}

func platformCandidates(platform string) []string {
	candidates := []string{platform}
	parts := strings.SplitN(platform, "-", 2)
	if len(parts) != 2 {
		return candidates
	}
	osName, archName := parts[0], parts[1]

	switch osName {
	case "macos":
		candidates = append(candidates, "darwin-"+archName)
	case "darwin":
		candidates = append(candidates, "macos-"+archName)
	}
	if archName == "arm64" {
		candidates = append(candidates, osName+"-aarch64")
	}
	if archName == "x86_64" {
		candidates = append(candidates, osName+"-amd64")
	}
	return candidates
}

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
func fetchAndRecord(ctx context.Context, root, id string, recipe catalog.Item, dlURL string, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	typeDir := toolkit.TypeDirs[recipe.Type]
	filename := recipe.Filename
	if filename == "" {
		filename = filenameFromURL(dlURL)
	}
	destPath := filepath.Join(root, typeDir, filename)

	result, err := downloader.Download(ctx, dlURL, destPath, "", onProgress)
	if err != nil {
		return manifest.RealizedEntry{}, fmt.Errorf("downloading %s: %w", id, err)
	}
	slog.Info("downloaded", "id", id, "path", destPath, "sha256", result.SHA256, "cached", result.Cached)

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
func buildItem(ctx context.Context, root, id string, recipe catalog.Item) (manifest.RealizedEntry, error) {
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

	slog.Info("building via docker", "id", id, "family", family)

	builderScript := fmt.Sprintf("recipes/builders/%s.py", strings.ReplaceAll(family, "-", "_"))

	args := []string{
		"docker", "run", "--rm",
		"-v", root + ":/vault",
		"-v", filepath.Join(root, "..") + "/recipes/builders:/builders:ro",
		builder.DefaultDockerImage,
		"python3", "/builders/" + filepath.Base(builderScript),
		"--output", "/vault/" + filepath.Join(typeDir, filename),
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var outputBuf bytes.Buffer
	cmd.Stderr = &outputBuf
	cmd.Stdout = &outputBuf
	if err := cmd.Run(); err != nil {
		// Include first 200 chars of output for context.
		detail := outputBuf.String()
		if len(detail) > 200 {
			detail = detail[:200] + "..."
		}
		return manifest.RealizedEntry{}, fmt.Errorf("building %s: %w\n%s", id, err, strings.TrimSpace(detail))
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

var execCommand = exec.Command

func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		parts := strings.Split(rawURL, "/")
		return parts[len(parts)-1]
	}
	segments := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	return segments[len(segments)-1]
}
