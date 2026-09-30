package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// WritePresetList writes the sorted list of preset names to w, one per line.
// User presets are marked with the file they were loaded from.
func WritePresetList(w io.Writer, c *catalog.Catalog) error {
	for _, name := range c.PresetNames() {
		line := name
		if p, _ := c.PresetByName(name); p.IsUser() {
			line = fmt.Sprintf("%s  (user: %s)", name, p.Path)
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// PresetTargetPath maps a bare preset name ("my-256") to its file in the user
// preset dir; anything that looks like a path is returned unchanged.
func PresetTargetPath(target string) (string, error) {
	if strings.ContainsRune(target, filepath.Separator) || strings.HasSuffix(target, ".yaml") {
		return target, nil
	}
	dir, err := catalog.UserPresetDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, target+".yaml"), nil
}

const presetStubHeader = `# Custom svalbard preset. Resolves like the built-in presets:
#   extends: presets/packs whose sources are included (see "svalbard preset list")
#   sources: extra recipe ids; prefix an id with "-" to drop an inherited one
# Loaded from ~/.config/svalbard/presets/*.yaml ($XDG_CONFIG_HOME respected).
`

// CopyPreset writes a new preset file at targetPath named after the file.
// By default it is a stub that extends sourceName with no extra sources; with
// flatten it lists every resolved source instead. It refuses to overwrite an
// existing file or to reuse an existing preset name.
func CopyPreset(cat *catalog.Catalog, sourceName, targetPath string, flatten bool) error {
	resolved, err := cat.ResolvePreset(sourceName)
	if err != nil {
		return fmt.Errorf("resolving preset %q: %w", sourceName, err)
	}

	name := strings.TrimSuffix(filepath.Base(targetPath), ".yaml")
	if _, exists := cat.PresetByName(name); exists {
		return fmt.Errorf("preset %q already exists; choose a different target name", name)
	}
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("%s already exists", targetPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	out := catalog.Preset{
		Name:        name,
		Description: fmt.Sprintf("Custom preset based on %s", sourceName),
		Extends:     []string{sourceName},
		Sources:     []string{},
	}
	if flatten {
		out = resolved
		out.Name = name
		out.Kind = ""
		out.Extends = nil
	}

	var buf bytes.Buffer
	buf.WriteString(presetStubHeader)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("marshalling preset: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}
	if err := os.WriteFile(targetPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing preset file: %w", err)
	}
	return nil
}
