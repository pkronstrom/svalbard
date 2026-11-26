package apply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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

// ProgressEvent is JSON Lines-compatible and reports item or procedure progress.
type ProgressEvent struct {
	ID         string `json:"id"`
	Procedure  string `json:"procedure_id,omitempty"`
	State      string `json:"state,omitempty"`
	Status     string `json:"status,omitempty"`
	Step       string `json:"step,omitempty"`
	Message    string `json:"message,omitempty"`
	Downloaded int64  `json:"downloaded,omitempty"`
	Total      int64  `json:"total,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ProgressFunc reports per-item progress during apply.
type ProgressFunc func(ProgressEvent)

// downloadJob is a unit of work for the parallel download pool.
type downloadJob struct {
	id     string
	recipe catalog.Item
	// realized holds a document collection's current entries, so its job
	// fetches only the documents that are missing or changed.
	realized []manifest.RealizedEntry
}

// downloadResult collects the outcome of a single download job.
type downloadResult struct {
	id      string
	entries []manifest.RealizedEntry
	err     error
	// partial marks entries as the item's complete realized set even though
	// err is set: a document collection keeps the documents that arrived.
	partial bool
	ack     chan struct{}
}

// CheckpointFunc durably stores a manifest snapshot before a completed recipe
// is reported to callers.
type CheckpointFunc func(manifest.Manifest) error

// Run executes a reconciliation plan against the vault at root, mutating the
// manifest's realized state to reflect what was applied.
// The optional onProgress callback reports per-item status.
func Run(ctx context.Context, root string, m *manifest.Manifest, plan planner.Plan, cat *catalog.Catalog, onProgress ...ProgressFunc) error {
	return RunWithCheckpoint(ctx, root, m, plan, cat, nil, onProgress...)
}

// RunWithCheckpoint executes a reconciliation plan and checkpoints each
// completed recipe before emitting its done event.
func RunWithCheckpoint(ctx context.Context, root string, m *manifest.Manifest, plan planner.Plan, cat *catalog.Catalog, checkpoint CheckpointFunc, onProgress ...ProgressFunc) error {
	applyCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	progress := func(ev ProgressEvent) {}
	if len(onProgress) > 0 && onProgress[0] != nil {
		progress = onProgress[0]
	}

	slog.Info("apply started", "downloads", len(plan.ToDownload), "removals", len(plan.ToRemove), "vault", root)

	checkpointManifest := func(snapshot manifest.Manifest) error {
		canonicalizeEntries(&snapshot)
		if checkpoint != nil {
			if err := checkpoint(snapshot); err != nil {
				cancel()
				return err
			}
		}
		*m = snapshot
		return nil
	}

	recoverAsides(root, m.Realized.Entries)
	acquire := slices.Concat(plan.ToDownload, plan.ToUpdate)
	// The installed entries of each item being updated, to set aside while
	// its job runs.
	previous := make(map[string][]manifest.RealizedEntry, len(plan.ToUpdate))
	for _, id := range plan.ToUpdate {
		for _, e := range m.Realized.Entries {
			if e.ID == id {
				previous[id] = append(previous[id], e)
			}
		}
	}

	var applyErrors []error

	// Process removals first (free up space), checkpointing before completion.
	removeSet := make(map[string]struct{}, len(plan.ToRemove))
	for _, id := range plan.ToRemove {
		removeSet[id] = struct{}{}
		progress(ProgressEvent{ID: id, Status: tui.StatusActive, Step: "removing"})
	}
	for _, e := range m.Realized.Entries {
		if _, ok := removeSet[e.ID]; ok {
			path := filepath.Join(root, e.RelativePath)
			os.RemoveAll(path)
			// The downloader's record of where the file came from.
			os.Remove(path + ".download.json")
		}
	}
	var filtered, removed []manifest.RealizedEntry
	for _, e := range m.Realized.Entries {
		if _, ok := removeSet[e.ID]; !ok {
			filtered = append(filtered, e)
		} else {
			removed = append(removed, e)
		}
	}
	removeCollectionDirs(root, removed)
	snapshot := *m
	snapshot.Realized.Entries = filtered
	if len(plan.ToRemove) > 0 {
		if err := checkpointManifest(snapshot); err != nil {
			return fmt.Errorf("checkpointing removals: %w", err)
		}
	}
	for _, id := range plan.ToRemove {
		progress(ProgressEvent{ID: id, Status: tui.StatusDone})
	}

	// Build the desired ID set for native builders to filter against.
	desiredIDs := make(map[string]bool, len(m.Desired.Items))
	for _, id := range m.Desired.Items {
		desiredIDs[id] = true
	}
	if len(desiredIDs) == 0 {
		for _, id := range plan.ToDownload {
			desiredIDs[id] = true
		}
	}

	// Partition downloads into parallel-safe (HTTP) and sequential (build) jobs.
	var httpJobs []downloadJob
	var buildJobs []downloadJob
	buildQueued := make(map[string]bool)

	for _, id := range acquire {
		recipe, ok := cat.RecipeByID(id)
		if !ok {
			err := fmt.Errorf("recipe %q not found", id)
			progress(ProgressEvent{ID: id, Status: tui.StatusFailed, Error: err.Error()})
			applyErrors = append(applyErrors, err)
			continue
		}
		if recipe.Type == "python-package" {
			venv, ok := cat.RecipeByID(recipe.Venv)
			if !ok {
				err := fmt.Errorf("%s: python venv %q not found", id, recipe.Venv)
				progress(ProgressEvent{ID: id, Status: tui.StatusFailed, Error: err.Error()})
				applyErrors = append(applyErrors, err)
				continue
			}
			// The planner regards an existing venv as complete. Re-run it when a
			// newly desired package needs a tree below that venv.
			if !buildQueued[venv.ID] {
				buildJobs = append(buildJobs, downloadJob{id: venv.ID, recipe: venv})
				buildQueued[venv.ID] = true
			}
			continue
		}

		switch {
		case recipe.IsCollection():
			httpJobs = append(httpJobs, downloadJob{id: id, recipe: recipe, realized: collectionEntries(m.Realized.Entries, id)})
		case recipe.URL != "" || recipe.URLPattern != "" || len(recipe.Platforms) > 0 || len(recipe.Bundle) > 0:
			httpJobs = append(httpJobs, downloadJob{id: id, recipe: recipe})
		case recipe.Strategy == "build" && recipe.Build != nil:
			if !buildQueued[id] {
				buildJobs = append(buildJobs, downloadJob{id: id, recipe: recipe})
				buildQueued[id] = true
			}
		default:
			err := fmt.Errorf("%s: no acquisition strategy", id)
			progress(ProgressEvent{ID: id, Status: tui.StatusFailed, Error: err.Error()})
			applyErrors = append(applyErrors, err)
			slog.Warn("no acquisition strategy", "id", id)
		}
	}

	allJobs := append(httpJobs, buildJobs...)
	// Python packages are materialized by their venv aggregate rather than a
	// standalone job. Replace explicit package dependencies with that queued
	// aggregate so a platform archive cannot run before its Core tree exists.
	for i := range allJobs {
		allJobs[i].recipe.Deps = schedulerDependencies(allJobs[i].recipe, cat)
	}
	levels, err := jobLevels(allJobs, m.Realized.Entries)
	if err != nil {
		return fmt.Errorf("scheduling jobs: %w", err)
	}

	for _, level := range levels {
		levelFailed := false
		for res := range runJobBatch(applyCtx, root, level, cat, m.Desired.Options.HostPlatforms, desiredIDs, updateSwap{previous: previous, replace: plan.Replace}, progress) {
			func() {
				defer close(res.ack)
				if res.err != nil {
					progress(ProgressEvent{ID: res.id, Status: tui.StatusFailed, Error: res.err.Error()})
					slog.Warn("job failed", "id", res.id, "error", res.err)
					applyErrors = append(applyErrors, fmt.Errorf("%s: %w", res.id, res.err))
					levelFailed = true
					if res.partial {
						snapshot := *m
						snapshot.Realized.Entries = replaceEntriesForID(snapshot.Realized.Entries, res.id, res.entries)
						if err := checkpointManifest(snapshot); err != nil {
							applyErrors = append(applyErrors, fmt.Errorf("%s: checkpointing: %w", res.id, err))
						} else {
							recordCollection(root, m.Realized.Entries, cat, res.id)
						}
					} else if plan.Replace && len(previous[res.id]) > 0 {
						// Its files were deleted before the failed attempt.
						snapshot := *m
						snapshot.Realized.Entries = slices.DeleteFunc(slices.Clone(snapshot.Realized.Entries), func(e manifest.RealizedEntry) bool {
							return e.ID == res.id
						})
						if err := checkpointManifest(snapshot); err != nil {
							applyErrors = append(applyErrors, fmt.Errorf("%s: checkpointing: %w", res.id, err))
						}
					}
					return
				}
				for i := range res.entries {
					if recipe, ok := cat.RecipeByID(res.entries[i].ID); ok {
						res.entries[i].AcquisitionHash = recipe.AcquisitionHash()
					}
				}
				snapshot := *m
				snapshot.Realized.Entries = replaceEntries(snapshot.Realized.Entries, res.entries)
				if err := checkpointManifest(snapshot); err != nil {
					progress(ProgressEvent{ID: res.id, Status: tui.StatusFailed, Error: fmt.Sprintf("checkpointing: %v", err)})
					applyErrors = append(applyErrors, fmt.Errorf("%s: checkpointing: %w", res.id, err))
					levelFailed = true
					return
				}
				recordCollection(root, m.Realized.Entries, cat, res.id)
				for _, id := range plannedIDsCoveredBy(res.entries, acquire) {
					progress(ProgressEvent{ID: id, Status: tui.StatusDone})
				}
			}()
		}
		if levelFailed {
			break
		}
	}
	if err := applyCtx.Err(); err != nil {
		applyErrors = append(applyErrors, err)
	}
	for _, id := range acquire {
		if !realizedID(m.Realized.Entries, id) {
			applyErrors = append(applyErrors, fmt.Errorf("incomplete apply: %s was not realized", id))
		}
	}
	if len(applyErrors) > 0 {
		return fmt.Errorf("apply incomplete: %w", errors.Join(applyErrors...))
	}

	// Collect env vars and menu specs from all realized recipes.
	envVars := make(map[string]string)
	platformIORecipes := make(map[string]catalog.Item)
	recipes := make(map[string]catalog.Item)
	for _, e := range m.Realized.Entries {
		if recipe, ok := cat.RecipeByID(e.ID); ok {
			recipes[e.ID] = recipe
			if desiredIDs[recipe.ID] && isPlatformIORecipe(recipe) {
				platformIORecipes[recipe.ID] = recipe
			}
			for k, v := range recipe.Env {
				envVars[k] = v
			}
		}
	}
	menus := toolkit.MenuSpecs(recipes)

	// Regenerate runtime assets.
	presetName := ""
	if len(m.Desired.Presets) > 0 {
		presetName = m.Desired.Presets[0]
	}
	platforms := m.Desired.Options.HostPlatforms
	if len(platforms) == 0 {
		platforms = []string{hostPlatform()}
	}
	if err := toolkit.Generate(root, m.Realized.Entries, presetName, toolkit.GenerateOpts{EnvVars: envVars, Menus: menus, PlatformIORecipes: platformIORecipes, PlatformIOPlatforms: platforms, Recipes: recipes}); err != nil {
		return fmt.Errorf("generating toolkit: %w", err)
	}
	if err := toolkit.WriteDocumentsMetadata(root, m.Realized.Entries, recipes); err != nil {
		return fmt.Errorf("writing document metadata: %w", err)
	}
	if err := syncRecipeCopies(root, m, cat); err != nil {
		return fmt.Errorf("writing the vault's recipe copies: %w", err)
	}
	if err := syncMapViewer(root, m.Realized.Entries); err != nil {
		return fmt.Errorf("generating map viewer: %w", err)
	}

	snapshot = *m
	snapshot.Realized.AppliedAt = time.Now().UTC().Format(time.RFC3339)
	if err := checkpointManifest(snapshot); err != nil {
		return fmt.Errorf("checkpointing completed apply: %w", err)
	}

	slog.Info("apply completed", "realized", len(m.Realized.Entries))
	return nil
}

func schedulerDependencies(recipe catalog.Item, cat *catalog.Catalog) []string {
	deps := append([]string(nil), recipeDependencies(recipe)...)
	for i, dep := range deps {
		item, ok := cat.RecipeByID(dep)
		if ok && item.Type == "python-package" && item.Venv != "" {
			deps[i] = item.Venv
		}
	}
	return slices.Compact(deps)
}

func runJobBatch(
	ctx context.Context,
	root string,
	batch []downloadJob,
	cat *catalog.Catalog,
	platforms []string,
	desiredIDs map[string]bool,
	swap updateSwap,
	progress ProgressFunc,
) <-chan downloadResult {
	results := make(chan downloadResult)
	jobs := make(chan downloadJob)
	var wg sync.WaitGroup
	workerCount := min(maxWorkers, len(batch))
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				var result downloadResult
				select {
				case <-ctx.Done():
					result = downloadResult{id: job.id, err: ctx.Err()}
				default:
					result = swap.run(root, job.id, func() downloadResult {
						return executeJob(ctx, root, job, cat, platforms, desiredIDs, progress)
					})
				}
				result.ack = make(chan struct{})
				results <- result
				<-result.ack
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, job := range batch {
			select {
			case <-ctx.Done():
				return
			case jobs <- job:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}

// asideSuffix marks an installed file or directory moved out of the way while
// its item is updated.
const asideSuffix = ".svalbard-old"

// updateSwap keeps the installed copy of an item until its update succeeds.
type updateSwap struct {
	previous map[string][]manifest.RealizedEntry // installed entries by item id
	replace  bool                                // delete them up front instead
}

// run executes job for item id. If the item is being updated, its installed
// files are first moved aside, so the job cannot reuse them, and afterwards
// deleted on success or moved back on failure.
func (s updateSwap) run(root, id string, job func() downloadResult) downloadResult {
	old := s.previous[id]
	for _, e := range old {
		path := filepath.Join(root, e.RelativePath)
		var err error
		if s.replace {
			err = os.RemoveAll(path)
		} else if err = os.Rename(path, path+asideSuffix); os.IsNotExist(err) {
			err = nil // already missing; nothing to keep
		}
		if err != nil {
			recoverAsides(root, old)
			return downloadResult{id: id, err: fmt.Errorf("setting aside the installed copy: %w", err)}
		}
	}
	result := job()
	if result.err != nil {
		recoverAsides(root, old)
		return result
	}
	for _, e := range old {
		if err := os.RemoveAll(filepath.Join(root, e.RelativePath) + asideSuffix); err != nil {
			slog.Warn("removing the previous copy", "id", id, "error", err)
		}
	}
	return result
}

// recoverAsides moves back every entry's set-aside copy. The manifest still
// describes that copy, so it replaces whatever an unfinished update left in
// its place. Apply runs it over the whole vault first, which repairs an
// update that was interrupted.
func recoverAsides(root string, entries []manifest.RealizedEntry) {
	for _, e := range entries {
		path := filepath.Join(root, e.RelativePath)
		if _, err := os.Lstat(path + asideSuffix); err != nil {
			continue
		}
		if err := os.RemoveAll(path); err == nil {
			err = os.Rename(path+asideSuffix, path)
			if err != nil {
				slog.Warn("restoring the previous copy", "id", e.ID, "error", err)
			}
		}
	}
}

func replaceEntries(existing, additions []manifest.RealizedEntry) []manifest.RealizedEntry {
	ids := make(map[string]struct{}, len(additions))
	for _, entry := range additions {
		ids[entry.ID] = struct{}{}
	}
	entries := make([]manifest.RealizedEntry, 0, len(existing)+len(additions))
	for _, entry := range existing {
		if _, replace := ids[entry.ID]; !replace {
			entries = append(entries, entry)
		}
	}
	entries = append(entries, additions...)
	return entries
}

func canonicalizeEntries(m *manifest.Manifest) {
	slices.SortFunc(m.Realized.Entries, func(a, b manifest.RealizedEntry) int {
		if byID := strings.Compare(a.ID, b.ID); byID != 0 {
			return byID
		}
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
}

func plannedIDsCoveredBy(entries []manifest.RealizedEntry, planned []string) []string {
	covered := make(map[string]bool, len(entries))
	for _, entry := range entries {
		covered[entry.ID] = true
	}
	ids := make([]string, 0, len(entries))
	for _, id := range planned {
		if covered[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

func realizedID(entries []manifest.RealizedEntry, id string) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func executeJob(
	ctx context.Context,
	root string,
	job downloadJob,
	cat *catalog.Catalog,
	platforms []string,
	desiredIDs map[string]bool,
	progress ProgressFunc,
) downloadResult {
	progress(ProgressEvent{ID: job.id, Status: tui.StatusActive})
	var lastReport time.Time
	dlProgress := func(downloaded, total int64) {
		now := time.Now()
		if now.Sub(lastReport) < 250*time.Millisecond {
			return
		}
		lastReport = now
		progress(ProgressEvent{ID: job.id, Status: tui.StatusActive, Downloaded: downloaded, Total: total})
	}

	var entries []manifest.RealizedEntry
	var err error
	switch {
	case job.recipe.IsCollection():
		entries, err = assembleDocuments(ctx, root, job.id, job.recipe, job.realized, dlProgress)
		if err != nil {
			return downloadResult{id: job.id, entries: entries, err: err, partial: true}
		}
	case len(job.recipe.Bundle) > 0:
		var entry manifest.RealizedEntry
		entry, err = assembleBundle(ctx, root, job.id, job.recipe, dlProgress)
		if err == nil {
			entries = []manifest.RealizedEntry{entry}
		}
	case job.recipe.URL != "" || job.recipe.URLPattern != "":
		var entry manifest.RealizedEntry
		entry, err = downloadItem(ctx, root, job.id, job.recipe, dlProgress)
		if err == nil {
			entries = []manifest.RealizedEntry{entry}
			writeEmbeddingSidecar(root, entry, job.recipe)
		}
	case len(job.recipe.Platforms) > 0:
		entries, err = downloadPlatformItems(ctx, root, job.id, job.recipe, platforms, dlProgress)
	case job.recipe.Strategy == "build" && job.recipe.Build != nil:
		if nativeFn, ok := builder.Dispatch(job.recipe); ok {
			entries, err = nativeFn(root, job.recipe, cat, builder.Options{
				Ctx:        ctx,
				Platforms:  platforms,
				DesiredIDs: desiredIDs,
				OnEvent: func(event builder.BuildEvent) {
					status := tui.StatusActive
					if event.State == builder.EventFailed {
						status = tui.StatusFailed
					}
					message := event.Message
					if message == "" {
						message = event.Procedure
					}
					progress(ProgressEvent{
						ID: job.id, Procedure: event.Procedure, State: string(event.State),
						Status: status, Step: message, Message: event.Message, Error: event.Error,
					})
				},
			})
		} else {
			err = fmt.Errorf("unsupported build family %q for recipe %s", job.recipe.Build.Family, job.id)
		}
	default:
		err = fmt.Errorf("no acquisition strategy")
	}
	return downloadResult{id: job.id, entries: entries, err: err}
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

// HostPlatform names the platform this process runs on, e.g. "macos-arm64".
func HostPlatform() string { return hostPlatform() }

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
		candidates := catalog.PlatformCandidates(platform)
		var downloaded bool
		for _, candidate := range candidates {
			if platformURL, ok := recipe.Platforms[candidate]; ok {
				platformRecipe := recipe
				if checksum := recipe.PlatformSHA256[candidate]; checksum != "" {
					platformRecipe.SHA256 = checksum
				}
				entry, err := fetchAndRecordForPlatform(ctx, root, id, platformRecipe, platformURL, platform, onProgress)
				if err != nil {
					return nil, err
				}
				entries = append(entries, entry)
				downloaded = true
				break
			}
		}
		if !downloaded {
			if isPlatformIORecipe(recipe) {
				return nil, fmt.Errorf("PlatformIO closure %s has no archive for required target %s; reprovision with supported targets", id, platform)
			}
			slog.Debug("no platform URL", "id", id, "platform", platform, "available", slices.Collect(maps.Keys(recipe.Platforms)))
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no download URL for %s on any target platform %v (available: %v)", id, targets, slices.Collect(maps.Keys(recipe.Platforms)))
	}
	return entries, nil
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

// fetchAndRecord downloads a file and returns a RealizedEntry.
func fetchAndRecord(ctx context.Context, root, id string, recipe catalog.Item, dlURL string, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	return fetchAndRecordAt(ctx, root, id, recipe, dlURL, "", onProgress)
}

func fetchAndRecordForPlatform(ctx context.Context, root, id string, recipe catalog.Item, dlURL, platform string, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	return fetchAndRecordAt(ctx, root, id, recipe, dlURL, platform, onProgress)
}

func fetchAndRecordAt(ctx context.Context, root, id string, recipe catalog.Item, dlURL, platform string, onProgress downloader.ProgressFunc) (manifest.RealizedEntry, error) {
	typeDir := toolkit.TypeDirs[recipe.Type]
	if isPlatformIORecipe(recipe) {
		var err error
		typeDir, err = platformIODestination(dlURL, platform)
		if err != nil {
			return manifest.RealizedEntry{}, fmt.Errorf("%s: %w", id, err)
		}
	}
	filename := recipe.Filename
	if filename == "" {
		filename = filenameFromURL(dlURL)
	}
	cacheName := filename
	// Per-platform binaries live in bin/<platform>/, where the drive runtime
	// unpacks and resolves them. In flat bin/ the runtime unpacked every
	// platform's archive into one directory and could run another platform's
	// llama-server. A bare executable (omp-darwin-arm64, jq-linux-amd64) is
	// stored under the recipe id, ready to run.
	platformBinary := recipe.Type == "binary" && platform != ""
	bareBinary := platformBinary && recipe.Filename == "" && filepath.Ext(filename) == ""
	if platformBinary {
		typeDir = filepath.Join(typeDir, drivePlatform(platform))
	}
	if bareBinary {
		filename = id
	}
	destPath := filepath.Join(root, typeDir, filename)

	result, err := fetchFile(ctx, id, dlURL, destPath, cacheName, recipe.SHA256, onProgress)
	if err != nil {
		return manifest.RealizedEntry{}, err
	}
	if bareBinary {
		if err := os.Chmod(destPath, 0o755); err != nil {
			return manifest.RealizedEntry{}, fmt.Errorf("mark %s executable: %w", destPath, err)
		}
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

// drivePlatform returns the name the drive runtime uses for a target platform:
// "darwin-arm64" and "macos-aarch64" are both "macos-arm64" on the drive.
func drivePlatform(platform string) string {
	osName, archName, ok := strings.Cut(platform, "-")
	if !ok {
		return platform
	}
	if osName == "darwin" {
		osName = "macos"
	}
	switch archName {
	case "aarch64":
		archName = "arm64"
	case "amd64":
		archName = "x86_64"
	}
	return osName + "-" + archName
}

// fetchFile downloads dlURL to destPath, reusing a locally cached copy
// (SVALBARD_CACHE_DIRS) when one matches filename.
func fetchFile(ctx context.Context, id, dlURL, destPath, filename, sha256 string, onProgress downloader.ProgressFunc) (downloader.Result, error) {
	if dirs := downloader.CacheDirs(); len(dirs) > 0 {
		if cached, ok := downloader.FindInCache(dirs, filename); ok {
			res, used, err := downloader.ReuseFromCacheWithSHA256(ctx, cached, dlURL, destPath, sha256, onProgress)
			if err != nil {
				return downloader.Result{}, fmt.Errorf("reusing cached %s: %w", id, err)
			}
			if used {
				slog.Info("reused from cache", "id", id, "src", cached, "dest", destPath, "sha256", res.SHA256)
				return res, nil
			}
		}
	}
	result, err := downloader.Download(ctx, dlURL, destPath, sha256, onProgress)
	if err != nil {
		return downloader.Result{}, fmt.Errorf("downloading %s: %w", id, err)
	}
	slog.Info("downloaded", "id", id, "path", destPath, "sha256", result.SHA256, "cached", result.Cached)
	return result, nil
}

func isPlatformIORecipe(recipe catalog.Item) bool {
	if _, ok := parsePlatformIOArchive(recipe.URL); ok {
		return true
	}
	for _, raw := range recipe.Platforms {
		if _, ok := parsePlatformIOArchive(raw); ok {
			return true
		}
	}
	return false
}

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

// writeEmbeddingSidecar writes a .embedding.json next to a downloaded gguf-embed
// model so the indexer and search runtime can discover embedding params without
// loading the full catalog.
func writeEmbeddingSidecar(root string, entry manifest.RealizedEntry, recipe catalog.Item) {
	if recipe.Type != "gguf-embed" || recipe.Embedding == nil {
		return
	}
	ggufPath := filepath.Join(root, entry.RelativePath)
	sidecarPath := strings.TrimSuffix(ggufPath, filepath.Ext(ggufPath)) + ".embedding.json"
	data, err := json.MarshalIndent(recipe.Embedding, "", "  ")
	if err != nil {
		slog.Warn("failed to marshal embedding sidecar", "id", entry.ID, "error", err)
		return
	}
	if err := os.WriteFile(sidecarPath, data, 0o644); err != nil {
		slog.Warn("failed to write embedding sidecar", "path", sidecarPath, "error", err)
	}
}

func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		parts := strings.Split(rawURL, "/")
		return parts[len(parts)-1]
	}
	segments := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	return segments[len(segments)-1]
}
