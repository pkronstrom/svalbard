package commands

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/builder"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// DefaultZimName derives an item id from the site hostname:
// https://www.opensourcelowtech.org/ → "opensourcelowtech".
func DefaultZimName(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("invalid URL %q", rawURL)
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	name := strings.Split(host, ".")[0]
	if name == "" {
		return "", fmt.Errorf("cannot derive a name from %q", rawURL)
	}
	return name, nil
}

// zimRecipe synthesizes an in-memory zimit recipe for a one-off site crawl.
// Same shape as a catalog zimit recipe, so the standard pipeline executor
// handles docker, path translation, verify, and caching.
func zimRecipe(name, sourceURL string) catalog.Item {
	return catalog.Item{
		ID:          name,
		Type:        "zim",
		Strategy:    "build",
		Description: "Web archive of " + sourceURL,
		Build: &catalog.BuildSpec{
			Family:    "zimit",
			SourceURL: sourceURL,
			Output:    name + ".zim",
			Steps: []catalog.BuildStep{
				{
					Exec:        "zimit",
					DockerImage: "ghcr.io/openzim/zimit:latest",
					Args: []string{
						"--seeds", "{source_url}",
						"--name", name,
						// Pin the exact filename; zimit otherwise appends a date suffix.
						"--zim-file", name + ".zim",
						"--output", "{vault}/zim",
						"--waitUntil", "domcontentloaded",
					},
				},
				{Verify: "{output}", MinSize: 1_000_000},
			},
		},
	}
}

// ZimOptions tunes how a site is turned into a ZIM.
type ZimOptions struct {
	Name    string // item id / output name; derived from the URL when empty
	Videos  bool   // mirror the site and pull embedded videos local
	Quality string // max video height for Videos builds, e.g. "480p"
}

// BuildZim crawls a website into a ZIM in the vault and records it in the
// manifest as a desired + realized item, so plan/apply treat it as already
// reconciled.
func BuildZim(ctx context.Context, vaultRoot, sourceURL string, opts ZimOptions, onStatus func(string)) (string, error) {
	name := opts.Name
	if name == "" {
		var err error
		if name, err = DefaultZimName(sourceURL); err != nil {
			return "", err
		}
	}

	var entries []manifest.RealizedEntry
	var err error
	if opts.Videos {
		entries, err = buildWithVideos(ctx, vaultRoot, sourceURL, name, opts.Quality, onStatus)
	} else {
		item := zimRecipe(name, sourceURL)
		fn, _ := builder.Dispatch(item) // always dispatches: item has explicit steps
		entries, err = fn(vaultRoot, item, nil, builder.Options{Ctx: ctx, OnStatus: onStatus})
	}
	if err != nil {
		return "", err
	}

	mPath := filepath.Join(vaultRoot, "manifest.yaml")
	m, err := manifest.Load(mPath)
	if err != nil {
		return "", err
	}
	_ = AddItems(&m, []string{name})
	upsertRealized(&m, entries...)
	if err := manifest.Save(mPath, m); err != nil {
		return "", err
	}
	return name, nil
}

// buildWithVideos runs the embedded web-video-zim builder in the tools
// container: it mirrors the site, downloads embedded videos, rewrites the
// embeds to play locally, and packs the result into one ZIM.
func buildWithVideos(ctx context.Context, vaultRoot, sourceURL, name, quality string, onStatus func(string)) ([]manifest.RealizedEntry, error) {
	if quality == "" {
		quality = "480p"
	}
	script, err := catalog.BuilderScript(builderScriptName)
	if err != nil {
		return nil, fmt.Errorf("loading builder: %w", err)
	}

	// The workdir holds the builder and the site mirror; it is mounted as /work.
	workdir, err := os.MkdirTemp("", "svalbard-webvideo-"+name+"-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workdir)
	if err := os.WriteFile(filepath.Join(workdir, builderScriptName), script, 0o755); err != nil {
		return nil, err
	}

	outFile := name + ".zim"
	destPath := filepath.Join(vaultRoot, "zim", outFile)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return nil, err
	}

	if onStatus != nil {
		onStatus("mirroring site and downloading videos")
	}
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", vaultRoot+":/vault",
		"-v", workdir+":/work",
		builder.DefaultDockerImage,
		"python3", "/work/"+builderScriptName,
		"--source-url", sourceURL,
		"--output", "/vault/zim/"+outFile,
		"--workdir", "/work",
		"--title", name,
		"--quality", quality,
	)
	// Stream builder progress; it reports per-video and per-phase lines.
	if onStatus != nil {
		cmd.Stdout = statusWriter{onStatus}
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("web-video build: %w\n%s", err, tailOf(errBuf.String(), 500))
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return nil, fmt.Errorf("no ZIM produced at %s", destPath)
	}
	sha, _ := downloader.ComputeSHA256(destPath)
	return []manifest.RealizedEntry{{
		ID:             name,
		Type:           "zim",
		Filename:       outFile,
		RelativePath:   filepath.Join("zim", outFile),
		SizeBytes:      info.Size(),
		ChecksumSHA256: sha,
		SourceStrategy: "build",
	}}, nil
}

const builderScriptName = "web-video-zim.py"

// statusWriter forwards each line the builder prints to the progress callback.
type statusWriter struct{ onStatus func(string) }

func (w statusWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			w.onStatus(line)
		}
	}
	return len(p), nil
}

func tailOf(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
