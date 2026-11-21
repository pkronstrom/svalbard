package commands

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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
