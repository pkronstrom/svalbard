package catalog

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Item represents a single recipe (content source) in the catalog.
type Item struct {
	ID          string            `yaml:"id"`
	Type        string            `yaml:"type"`
	Description string            `yaml:"description"`
	SizeGB      float64           `yaml:"size_gb"`
	Strategy    string            `yaml:"strategy"`
	URL         string            `yaml:"url,omitempty"`
	URLPattern  string            `yaml:"url_pattern,omitempty"`
	Filename    string            `yaml:"filename,omitempty"`
	Platforms   map[string]string `yaml:"platforms,omitempty"`
	Build       *BuildSpec        `yaml:"build,omitempty"`
	Viewer      *ViewerSpec       `yaml:"viewer,omitempty"`
	License     *LicenseSpec      `yaml:"license,omitempty"`
	Tags        []string          `yaml:"tags,omitempty"`
	Menu        *MenuSpec         `yaml:"menu,omitempty"`
}

// BuildSpec describes how to build a recipe from source data.
type BuildSpec struct {
	Family string `yaml:"family"`
}

// ViewerSpec describes how a recipe should be presented in a viewer.
type ViewerSpec struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
}

// LicenseSpec captures the licensing information for a recipe.
type LicenseSpec struct {
	ID          string `yaml:"id"`
	Attribution string `yaml:"attribution"`
	URL         string `yaml:"url"`
}

// MenuSpec defines how a recipe appears in the user-facing menu.
type MenuSpec struct {
	Group       string `yaml:"group"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
}

// Preset represents a named collection of source references.
type Preset struct {
	Name    string   `yaml:"name"`
	Region  string   `yaml:"region"`
	Sources []string `yaml:"sources"`
	Items   []Item   `yaml:"-"` // populated by ResolvePreset
}

// Catalog holds all known recipes and presets.
type Catalog struct {
	recipes map[string]Item
	presets map[string]Preset
}

// LoadFromFS walks both FS trees, parsing .yaml files into recipes and presets maps.
func LoadFromFS(recipesFS fs.FS, presetsFS fs.FS) (*Catalog, error) {
	c := &Catalog{
		recipes: make(map[string]Item),
		presets: make(map[string]Preset),
	}

	if err := fs.WalkDir(recipesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, readErr := fs.ReadFile(recipesFS, path)
		if readErr != nil {
			return fmt.Errorf("reading recipe %s: %w", path, readErr)
		}
		var item Item
		if parseErr := yaml.Unmarshal(data, &item); parseErr != nil {
			return fmt.Errorf("parsing recipe %s: %w", path, parseErr)
		}
		if item.ID == "" {
			base := filepath.Base(path)
			item.ID = strings.TrimSuffix(base, ".yaml")
		}
		c.recipes[item.ID] = item
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walking recipes: %w", err)
	}

	if err := fs.WalkDir(presetsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, readErr := fs.ReadFile(presetsFS, path)
		if readErr != nil {
			return fmt.Errorf("reading preset %s: %w", path, readErr)
		}
		var preset Preset
		if parseErr := yaml.Unmarshal(data, &preset); parseErr != nil {
			return fmt.Errorf("parsing preset %s: %w", path, parseErr)
		}
		if preset.Name == "" {
			base := filepath.Base(path)
			preset.Name = strings.TrimSuffix(base, ".yaml")
		}
		c.presets[preset.Name] = preset
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walking presets: %w", err)
	}

	return c, nil
}

// ResolvePreset looks up a preset by name and populates its Items by resolving
// each source ID against the recipes map.
func (c *Catalog) ResolvePreset(name string) (Preset, error) {
	preset, ok := c.presets[name]
	if !ok {
		return Preset{}, fmt.Errorf("preset %q not found", name)
	}

	items := make([]Item, 0, len(preset.Sources))
	for _, src := range preset.Sources {
		item, found := c.recipes[src]
		if !found {
			return Preset{}, fmt.Errorf("preset %q references unknown recipe %q", name, src)
		}
		items = append(items, item)
	}

	preset.Items = items
	return preset, nil
}

// PresetNames returns a sorted list of all preset names.
func (c *Catalog) PresetNames() []string {
	names := make([]string, 0, len(c.presets))
	for name := range c.presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RecipeByID looks up a single recipe by its ID.
func (c *Catalog) RecipeByID(id string) (Item, bool) {
	item, ok := c.recipes[id]
	return item, ok
}

// AllRecipes returns all recipes as a slice, sorted by ID for deterministic ordering.
func (c *Catalog) AllRecipes() []Item {
	items := make([]Item, 0, len(c.recipes))
	for _, item := range c.recipes {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}
