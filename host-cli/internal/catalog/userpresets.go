package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// UserRegion is the region given to a user preset that neither declares one
// nor inherits one from its extends chain.
const UserRegion = "custom"

// UserPresetDir is where user-defined presets live:
// $XDG_CONFIG_HOME/svalbard/presets, defaulting to ~/.config/svalbard/presets.
func UserPresetDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locating home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "svalbard", "presets"), nil
}

// UserPresetDirs returns every directory LoadCatalog reads user presets from:
// the user preset dir and, in development builds, the repo's gitignored
// local/presets.
func UserPresetDirs() []string {
	var dirs []string
	if dir, err := UserPresetDir(); err == nil {
		dirs = append(dirs, dir)
	}
	if root, err := devRepoRoot(); err == nil {
		dirs = append(dirs, filepath.Join(root, "local", "presets"))
	}
	return dirs
}

// LoadUserPresets adds the *.yaml presets found in dirs (missing dirs are
// skipped) to the catalog. User presets resolve exactly like built-in ones but
// are validated strictly: a name may not shadow an existing preset, and every
// extends entry and source id (including "-id" removals) must exist.
func (c *Catalog) LoadUserPresets(dirs ...string) error {
	var added []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading user presets: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading user preset %s: %w", path, err)
			}
			var preset Preset
			if err := yaml.Unmarshal(data, &preset); err != nil {
				return fmt.Errorf("parsing user preset %s: %w", path, err)
			}
			if preset.Name == "" {
				preset.Name = strings.TrimSuffix(e.Name(), ".yaml")
			}
			if existing, ok := c.presets[preset.Name]; ok {
				if existing.IsUser() {
					return fmt.Errorf("user preset %s: name %q is already defined in %s", path, preset.Name, existing.Path)
				}
				return fmt.Errorf("user preset %s: name %q shadows a built-in preset; pick a different name", path, preset.Name)
			}
			preset.Path = path
			c.presets[preset.Name] = preset
			added = append(added, preset.Name)
		}
	}

	sort.Strings(added)
	for _, name := range added {
		if err := c.validateUserPreset(name); err != nil {
			return err
		}
	}
	for _, name := range added {
		p := c.presets[name]
		if p.Region == "" {
			p.Region = c.inheritedRegion(name, map[string]bool{})
			if p.Region == "" {
				p.Region = UserRegion
			}
			c.presets[name] = p
		}
	}
	return nil
}

func (c *Catalog) validateUserPreset(name string) error {
	p := c.presets[name]
	for _, parent := range p.Extends {
		if _, ok := c.presets[parent]; !ok {
			return fmt.Errorf("user preset %s: extends unknown preset %q (see `svalbard preset list`)", p.Path, parent)
		}
	}
	for _, src := range p.Sources {
		id := strings.TrimPrefix(src, "-")
		if _, ok := c.recipes[id]; !ok {
			return fmt.Errorf("user preset %s: unknown source id %q", p.Path, id)
		}
	}
	if _, err := c.ResolvePreset(name); err != nil {
		return fmt.Errorf("user preset %s: %w", p.Path, err)
	}
	return nil
}

// inheritedRegion returns the first region found depth-first along the
// extends chain, so a user preset extending finland-256 is listed with the
// finland presets.
func (c *Catalog) inheritedRegion(name string, seen map[string]bool) string {
	if seen[name] {
		return ""
	}
	seen[name] = true
	for _, parent := range c.presets[name].Extends {
		if r := c.presets[parent].Region; r != "" {
			return r
		}
		if r := c.inheritedRegion(parent, seen); r != "" {
			return r
		}
	}
	return ""
}
