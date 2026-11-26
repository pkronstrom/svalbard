package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// InitVault creates a new vault directory at path, seeded from the named preset.
//
// It resolves the preset via the catalog, creates a new v2 manifest populated
// with the preset's source IDs, writes it to path/manifest.yaml, and ensures
// the directory exists on disk.
func InitVault(path, presetName string, c *catalog.Catalog) error {
	slog.Info("init vault", "path", path, "preset", presetName)
	if err := rejectExistingManifest(filepath.Join(path, "manifest.yaml")); err != nil {
		return err
	}
	preset, err := c.ResolvePreset(presetName)
	if err != nil {
		return fmt.Errorf("resolving preset: %w", err)
	}

	m := manifest.New(filepath.Base(path))
	m.Desired.Presets = []string{presetName}

	items := make([]string, 0, len(preset.Items))
	for _, item := range preset.Items {
		items = append(items, item.ID)
	}
	m.Desired.Items = items

	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("creating vault directory: %w", err)
	}

	manifestPath := filepath.Join(path, "manifest.yaml")
	if err := manifest.Save(manifestPath, m); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// InitVaultWithOptions creates a vault with explicit items and platform options.
// Used by the TUI wizard which provides custom selections instead of a preset name.
func InitVaultWithOptions(path string, items []string, presetName string, region string, hostPlatforms []string) error {
	slog.Info("init vault", "path", path, "preset", presetName, "items", len(items), "platforms", hostPlatforms)
	if err := rejectExistingManifest(filepath.Join(path, "manifest.yaml")); err != nil {
		return err
	}
	m := manifest.New(filepath.Base(path))
	if presetName != "" {
		m.Desired.Presets = []string{presetName}
	}
	m.Desired.Items = items
	m.Desired.Options.HostPlatforms = hostPlatforms
	if region != "" {
		m.Desired.Options.Region = region
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("creating vault directory: %w", err)
	}

	manifestPath := filepath.Join(path, "manifest.yaml")
	if err := manifest.Save(manifestPath, m); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

func rejectExistingManifest(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return fmt.Errorf("vault already has a manifest; reopen this vault to resume")
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("checking existing manifest: %w", err)
	}
	return nil
}

// ResolveInitSelection builds the item list for a vault created without the
// wizard: the preset's sources, plus add, minus remove, plus the dependencies
// the result requires. add and remove take recipe ids and preset or pack
// names. It returns the sorted ids and the preset's region.
func ResolveInitSelection(cat *catalog.Catalog, defaults catalog.DepDefaults, presetName string, add, remove []string) ([]string, string, error) {
	selected := make(map[string]bool)
	region := ""
	if presetName != "" {
		base, ok := cat.PresetByName(presetName)
		if !ok {
			if skipped, ok := cat.SkippedUserPreset(presetName); ok {
				return nil, "", skipped.Err()
			}
			return nil, "", fmt.Errorf("unknown preset %q; run \"svalbard preset list\"", presetName)
		}
		region = base.Region
		resolved, err := cat.ResolvePreset(presetName)
		if err != nil {
			return nil, "", fmt.Errorf("resolving preset: %w", err)
		}
		for _, item := range resolved.Items {
			selected[item.ID] = true
		}
	}

	added, err := ExpandItemRefs(cat, add)
	if err != nil {
		return nil, "", err
	}
	if unknown := UnknownItems(cat, added); len(unknown) > 0 {
		return nil, "", fmt.Errorf("not in the catalog: %s; run \"svalbard catalog\"", strings.Join(unknown, ", "))
	}
	for _, id := range added {
		selected[id] = true
	}
	removed, err := ExpandItemRefs(cat, remove)
	if err != nil {
		return nil, "", err
	}
	for _, id := range removed {
		delete(selected, id)
	}

	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	deps, missing := RequiredDependencies(cat, defaults, ids)
	if len(missing) > 0 {
		return nil, "", fmt.Errorf("missing catalog recipes: %s", strings.Join(missing, ", "))
	}
	ids = append(ids, deps...)
	sort.Strings(ids)
	return ids, region, nil
}
