package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

const (
	protomapsBuildsIndex = "https://build-metadata.protomaps.dev/builds.json"
	protomapsBuildsBase  = "https://build.protomaps.com/"
)

// buildOSMExtract cuts an OpenStreetMap basemap for the recipe's bbox out of
// a Protomaps planet build with `pmtiles extract`, which fetches only the
// tiles it needs over HTTP range requests. The source is build.source_url
// when the recipe pins one, otherwise the newest daily build listed in the
// Protomaps build index.
func buildOSMExtract(root string, recipe catalog.Item, _ *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil {
		return nil, fmt.Errorf("osm-extract %s: build settings required", recipe.ID)
	}
	config := recipe.Build.Config
	if _, err := parseBBox(config["bbox"]); err != nil {
		return nil, fmt.Errorf("osm-extract %s: %w", recipe.ID, err)
	}
	ctx := builderContext(opts)
	source := recipe.Build.SourceURL
	if source == "" {
		index, base := config["builds_index"], config["builds_base"]
		if index == "" {
			index = protomapsBuildsIndex
		}
		if base == "" {
			base = protomapsBuildsBase
		}
		latest, err := latestProtomapsBuild(ctx, index)
		if err != nil {
			return nil, fmt.Errorf("osm-extract %s: %w", recipe.ID, err)
		}
		source = strings.TrimSuffix(base, "/") + "/" + latest
	}

	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}
	// Extract into staging and move the finished archive into place, so an
	// interrupted extract never leaves a partial map on the vault.
	partial := filepath.Join(workdir, recipe.ID+".pmtiles")
	args := []string{"extract", source, partial, "--bbox=" + config["bbox"]}
	if maxZoom := config["maxzoom"]; maxZoom != "" {
		if _, err := strconv.Atoi(maxZoom); err != nil {
			return nil, fmt.Errorf("osm-extract %s: maxzoom %q: %w", recipe.ID, maxZoom, err)
		}
		args = append(args, "--maxzoom="+maxZoom)
	}
	emit(opts, BuildEvent{RecipeID: recipe.ID, Procedure: "extract", State: EventStarted,
		Message: "extracting " + config["bbox"] + " from " + filepath.Base(source)})
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "pmtiles", args); err != nil {
		return nil, err
	}
	output := filepath.Join(root, toolkit.TypeDirs[recipe.Type], recipe.ID+".pmtiles")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(partial, output); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(workdir); err != nil {
		slog.Warn("osm-extract: could not remove build files", "dir", workdir, "err", err)
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	checksum, _ := downloader.ComputeSHA256(output)
	return []manifest.RealizedEntry{{
		ID: recipe.ID, Type: recipe.Type, Filename: filepath.Base(output),
		RelativePath: filepath.Join(toolkit.TypeDirs[recipe.Type], filepath.Base(output)),
		SizeBytes:    info.Size(), ChecksumSHA256: checksum, SourceStrategy: "build",
	}}, nil
}

// latestProtomapsBuild returns the newest planet build file named in the
// Protomaps build index, a JSON list of {"key": "YYYYMMDD.pmtiles", ...}.
func latestProtomapsBuild(ctx context.Context, index string) (string, error) {
	client := &http.Client{Timeout: time.Minute}
	body, err := fetchBytes(ctx, client, index, 0)
	if err != nil {
		return "", err
	}
	var builds []struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &builds); err != nil {
		return "", fmt.Errorf("build index %s: %w", index, err)
	}
	var keys []string
	for _, build := range builds {
		if strings.HasSuffix(build.Key, ".pmtiles") {
			keys = append(keys, build.Key)
		}
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("build index %s lists no .pmtiles builds", index)
	}
	// Keys are dated YYYYMMDD.pmtiles, so the newest sorts last.
	sort.Strings(keys)
	return keys[len(keys)-1], nil
}
