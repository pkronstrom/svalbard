package commands

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/builder"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
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

// BuildZim crawls a website into a ZIM in the vault via the zimit container
// and records it in the manifest as a desired + realized item, so plan/apply
// treat it as already reconciled.
func BuildZim(ctx context.Context, vaultRoot, sourceURL, name string, onStatus func(string)) (string, error) {
	if name == "" {
		var err error
		if name, err = DefaultZimName(sourceURL); err != nil {
			return "", err
		}
	}

	item := zimRecipe(name, sourceURL)
	fn, _ := builder.Dispatch(item) // always dispatches: item has explicit steps
	entries, err := fn(vaultRoot, item, nil, builder.Options{Ctx: ctx, OnStatus: onStatus})
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
