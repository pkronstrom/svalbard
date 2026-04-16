package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// WritePresetList writes the sorted list of preset names to w, one per line.
func WritePresetList(w io.Writer, c *catalog.Catalog) error {
	for _, name := range c.PresetNames() {
		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}
	return nil
}

// CopyPreset resolves a preset from the catalog and writes it to targetPath as
// a YAML file that can later be loaded back as a preset.
func CopyPreset(cat *catalog.Catalog, sourceName string, targetPath string) error {
	preset, err := cat.ResolvePreset(sourceName)
	if err != nil {
		return fmt.Errorf("resolving preset %q: %w", sourceName, err)
	}

	// Create target directory if needed.
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	data, err := yaml.Marshal(preset)
	if err != nil {
		return fmt.Errorf("marshalling preset: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return fmt.Errorf("writing preset file: %w", err)
	}

	return nil
}
