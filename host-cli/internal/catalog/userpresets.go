package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// UserRegion is the region given to a user preset that neither declares one
// nor inherits one from its extends chain.
const UserRegion = "custom"

// SkippedPreset is a user preset file that LoadUserPresets left out of the
// catalog because it could not be read or failed validation.
type SkippedPreset struct {
	Name   string // preset name; the file name without .yaml if it did not parse
	Path   string // the preset file (or directory, if that could not be read)
	Reason string
}

// String is the warning shown to the user: it names the file and the reason.
func (s SkippedPreset) String() string {
	return fmt.Sprintf("skipped user preset %s: %s", s.Path, s.Reason)
}

// Err explains why a command that names the skipped preset cannot use it.
func (s SkippedPreset) Err() error {
	return fmt.Errorf("user preset %q was skipped (%s): %s", s.Name, s.Path, s.Reason)
}

// SkippedUserPresets returns the user preset files LoadUserPresets skipped,
// in the order they were found.
func (c *Catalog) SkippedUserPresets() []SkippedPreset {
	return append([]SkippedPreset(nil), c.skipped...)
}

// SkippedUserPreset reports whether a user preset named name was skipped.
func (c *Catalog) SkippedUserPreset(name string) (SkippedPreset, bool) {
	for _, s := range c.skipped {
		if s.Name == name {
			return s, true
		}
	}
	return SkippedPreset{}, false
}

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
	// Test binaries skip the repo's local/presets, so a developer's own files
	// cannot change test results.
	if root, err := devRepoRoot(); err == nil && !testing.Testing() {
		dirs = append(dirs, filepath.Join(root, "local", "presets"))
	}
	return dirs
}

// LoadUserPresets adds the *.yaml presets found in dirs (missing dirs are
// ignored) to the catalog. User presets resolve exactly like built-in ones but
// are validated strictly: a name may not shadow an existing preset, and every
// extends entry and source id (including "-id" removals) must exist. A preset
// that fails is skipped rather than failing the load, and so is any user
// preset that extends it; the skipped files are returned and kept for
// SkippedUserPresets.
func (c *Catalog) LoadUserPresets(dirs ...string) []SkippedPreset {
	var skipped []SkippedPreset
	skip := func(name, path, reason string) {
		skipped = append(skipped, SkippedPreset{Name: name, Path: path, Reason: reason})
	}

	var added []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			skip("", dir, fmt.Sprintf("reading directory: %v", err))
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			fileName := strings.TrimSuffix(e.Name(), ".yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				skip(fileName, path, err.Error())
				continue
			}
			var preset Preset
			if err := yaml.Unmarshal(data, &preset); err != nil {
				skip(fileName, path, fmt.Sprintf("invalid YAML: %v", err))
				continue
			}
			if preset.Name == "" {
				preset.Name = fileName
			}
			if existing, ok := c.presets[preset.Name]; ok {
				if existing.IsUser() {
					skip(preset.Name, path, fmt.Sprintf("name %q is already defined in %s", preset.Name, existing.Path))
				} else {
					skip(preset.Name, path, fmt.Sprintf("name %q shadows a built-in preset; pick a different name", preset.Name))
				}
				continue
			}
			preset.Path = path
			c.presets[preset.Name] = preset
			added = append(added, preset.Name)
		}
	}

	// Dropping a preset can break a user preset that extends it, so repeat
	// until a pass finds nothing new to drop.
	sort.Strings(added)
	dropped := map[string]bool{}
	for {
		var bad []SkippedPreset
		for _, name := range added {
			if dropped[name] {
				continue
			}
			if reason := c.userPresetProblem(name, dropped); reason != "" {
				bad = append(bad, SkippedPreset{Name: name, Path: c.presets[name].Path, Reason: reason})
			}
		}
		if len(bad) == 0 {
			break
		}
		for _, s := range bad {
			dropped[s.Name] = true
			delete(c.presets, s.Name)
		}
		skipped = append(skipped, bad...)
	}

	for _, name := range added {
		if dropped[name] {
			continue
		}
		p := c.presets[name]
		if p.Region == "" {
			p.Region = c.inheritedRegion(name, map[string]bool{})
			if p.Region == "" {
				p.Region = UserRegion
			}
			c.presets[name] = p
		}
	}
	c.skipped = append(c.skipped, skipped...)
	return skipped
}

// userPresetProblem returns why the named user preset cannot be used, or ""
// if it is valid. dropped holds user presets already skipped.
func (c *Catalog) userPresetProblem(name string, dropped map[string]bool) string {
	p := c.presets[name]
	for _, parent := range p.Extends {
		if dropped[parent] {
			return fmt.Sprintf("extends user preset %q, which was skipped", parent)
		}
		if _, ok := c.presets[parent]; !ok {
			return fmt.Sprintf("extends unknown preset %q (see `svalbard preset list`)", parent)
		}
	}
	for _, src := range p.Sources {
		id := strings.TrimPrefix(src, "-")
		if _, ok := c.recipes[id]; !ok {
			return fmt.Sprintf("unknown source id %q", id)
		}
	}
	if _, err := c.ResolvePreset(name); err != nil {
		return err.Error()
	}
	return ""
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
