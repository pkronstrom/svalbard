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
	Name         string   `yaml:"name"`
	Kind         string   `yaml:"kind,omitempty"`
	Region       string   `yaml:"region"`
	DisplayGroup string   `yaml:"display_group,omitempty"`
	Description  string   `yaml:"description,omitempty"`
	TargetSizeGB float64  `yaml:"target_size_gb,omitempty"`
	Extends      []string `yaml:"extends,omitempty"`
	Sources      []string `yaml:"sources"`
	Items        []Item   `yaml:"-"` // populated by ResolvePreset
}

// ContentSizeGB returns the sum of SizeGB across all resolved Items.
func (p Preset) ContentSizeGB() float64 {
	var total float64
	for _, item := range p.Items {
		total += item.SizeGB
	}
	return total
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

// ResolvePreset looks up a preset by name, recursively flattens its extends
// chain, deduplicates sources, and populates Items by resolving each source ID
// against the recipes map. Sources prefixed with "-" remove that recipe from the
// inherited set. Missing recipes are tolerated (skipped, not errored).
// Circular extends chains are detected and produce an error.
func (c *Catalog) ResolvePreset(name string) (Preset, error) {
	preset, ok := c.presets[name]
	if !ok {
		return Preset{}, fmt.Errorf("preset %q not found", name)
	}

	// Collect all sources from the extends chain (parents first) then own.
	sources, err := c.flattenSources(name, make(map[string]bool))
	if err != nil {
		return Preset{}, err
	}

	// Resolve source IDs to Items, skipping unknown recipes.
	items := make([]Item, 0, len(sources))
	for _, src := range sources {
		item, found := c.recipes[src]
		if !found {
			continue // tolerate missing recipes
		}
		items = append(items, item)
	}

	preset.Items = items
	return preset, nil
}

// flattenSources recursively walks the extends chain for the given preset,
// collecting sources in order (parents first). It deduplicates and handles
// "-source" removal entries. The stack map tracks the current recursion path
// to detect circular extends while allowing diamond-shaped inheritance.
func (c *Catalog) flattenSources(name string, stack map[string]bool) ([]string, error) {
	if stack[name] {
		return nil, fmt.Errorf("circular extends detected: %q", name)
	}
	stack[name] = true
	defer func() { stack[name] = false }()

	preset, ok := c.presets[name]
	if !ok {
		// If an extended preset doesn't exist, tolerate it (return empty).
		return nil, nil
	}

	// Collect sources from parents first.
	var inherited []string
	for _, parent := range preset.Extends {
		parentSources, err := c.flattenSources(parent, stack)
		if err != nil {
			return nil, err
		}
		inherited = append(inherited, parentSources...)
	}

	// Build the final source list: start with inherited, then apply own sources.
	// A source prefixed with "-" removes it; otherwise it's added.
	seen := make(map[string]bool, len(inherited)+len(preset.Sources))
	result := make([]string, 0, len(inherited)+len(preset.Sources))

	// Add inherited (dedup).
	for _, src := range inherited {
		if !seen[src] {
			seen[src] = true
			result = append(result, src)
		}
	}

	// Apply own sources.
	for _, src := range preset.Sources {
		if strings.HasPrefix(src, "-") {
			// Removal: strip the "-" and remove from result.
			remove := src[1:]
			seen[remove] = true // prevent re-adding
			filtered := result[:0]
			for _, s := range result {
				if s != remove {
					filtered = append(filtered, s)
				}
			}
			result = filtered
		} else if !seen[src] {
			seen[src] = true
			result = append(result, src)
		}
	}

	return result, nil
}

// Packs returns all presets with Kind=="pack", sorted by DisplayGroup then Name.
func (c *Catalog) Packs() []Preset {
	var packs []Preset
	for _, p := range c.presets {
		if p.Kind == "pack" {
			packs = append(packs, p)
		}
	}
	sort.Slice(packs, func(i, j int) bool {
		if packs[i].DisplayGroup != packs[j].DisplayGroup {
			return packs[i].DisplayGroup < packs[j].DisplayGroup
		}
		return packs[i].Name < packs[j].Name
	})
	return packs
}

// PresetsForRegion returns non-pack presets for the given region, excluding
// presets whose name starts with "test-", sorted by TargetSizeGB ascending.
func (c *Catalog) PresetsForRegion(region string) []Preset {
	var result []Preset
	for _, p := range c.presets {
		if p.Kind == "pack" {
			continue
		}
		if strings.HasPrefix(p.Name, "test-") {
			continue
		}
		if p.Region != region {
			continue
		}
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TargetSizeGB < result[j].TargetSizeGB
	})
	return result
}

// Regions returns the distinct sorted region strings from non-pack, non-test presets.
func (c *Catalog) Regions() []string {
	seen := make(map[string]bool)
	for _, p := range c.presets {
		if p.Kind == "pack" {
			continue
		}
		if strings.HasPrefix(p.Name, "test-") {
			continue
		}
		if p.Region != "" {
			seen[p.Region] = true
		}
	}
	regions := make([]string, 0, len(seen))
	for r := range seen {
		regions = append(regions, r)
	}
	sort.Strings(regions)
	return regions
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
