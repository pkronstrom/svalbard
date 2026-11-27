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
	ID          string `yaml:"id"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	// SizeGB is the download size in decimal GB. For a per-platform recipe (a
	// platforms map, or a python-package/python-venv built per target) it is
	// one platform's download, using the largest platform asset; estimates
	// multiply it by the number of the vault's platforms the recipe supports
	// (see SizeGBFor). For any other recipe it is the single asset's size.
	SizeGB float64 `yaml:"size_gb"`
	// FileCount is how many files and directories the item puts on the
	// vault, per platform as SizeGB is. It is set, from a measured install,
	// for the items that are trees of many small files, whose space on a
	// stick with large clusters is several times their size (see
	// FileCountFor).
	FileCount  int               `yaml:"file_count,omitempty"`
	Strategy   string            `yaml:"strategy"`
	URL        string            `yaml:"url,omitempty"`
	SHA256     string            `yaml:"sha256,omitempty"`
	URLPattern string            `yaml:"url_pattern,omitempty"`
	Filename   string            `yaml:"filename,omitempty"`
	Platforms  map[string]string `yaml:"platforms,omitempty"`
	// PlatformSHA256 pins the archive selected from Platforms for each target.
	// It is optional for existing recipes; new multi-platform closures use it.
	PlatformSHA256 map[string]string `yaml:"platform_sha256,omitempty"`
	Build          *BuildSpec        `yaml:"build,omitempty"`
	Viewer         *ViewerSpec       `yaml:"viewer,omitempty"`
	License        *LicenseSpec      `yaml:"license,omitempty"`
	Tags           []string          `yaml:"tags,omitempty"`
	Menu           *MenuSpec         `yaml:"menu,omitempty"`
	Env            map[string]string `yaml:"env,omitempty"`

	// Settings are the choices the recipe lets the user make on the drive,
	// each with a default (see SettingSpec). Apply writes those of the
	// installed recipes to .svalbard/settings.schema.json.
	Settings []SettingSpec `yaml:"settings,omitempty"`

	// Bundle lists pinned downloads assembled into the item's directory
	// (<type dir>/<id>/), for example npm package tarballs for a web app.
	Bundle []BundleSource `yaml:"bundle,omitempty"`

	// Libraries describes the browser libraries of a js-lib recipe, one per
	// package a page imports (see Library and ValidateLibraries).
	Libraries []Library `yaml:"libraries,omitempty"`
	// Archive is what a js-lib recipe's archive holds, recorded by the pin
	// generator (build-tools/cmd/js-lock) for the size budgets.
	Archive *ArchiveSize `yaml:"archive,omitempty"`

	// Documents lists the files of a user document collection, staged into
	// <type dir>/<id>/ (see LoadUserRecipes and ExpectedDocuments).
	Documents []DocumentSource `yaml:"documents,omitempty"`
	// Path is the file a user recipe was loaded from; empty for built-ins.
	Path string `yaml:"-"`
	// VaultCopy marks a user recipe loaded from the copy a vault keeps of it
	// (see LoadVaultRecipes) because this computer has no definition.
	VaultCopy bool `yaml:"-"`

	// Recipe dependency declarations.
	Deps []string `yaml:"deps,omitempty"`

	// DraftFor lists the model recipe IDs a drafter (gguf-mtp) speeds up.
	DraftFor []string `yaml:"draft_for,omitempty"`

	// Embedding model params (gguf-embed only).
	Embedding *EmbeddingSpec `yaml:"embedding,omitempty"`

	// Python venv/package fields
	Python      string   `yaml:"python,omitempty"`
	Venv        string   `yaml:"venv,omitempty"`
	Packages    []string `yaml:"packages,omitempty"`
	EntryPoints []string `yaml:"entry_points,omitempty"`
	// RequirementsFrom is a pip requirements file inside the installed
	// packages, relative to the install root. When set, the packages are
	// installed without their declared dependencies and this file is installed
	// in their place, for a package that ships a smaller dependency set than
	// its metadata declares.
	RequirementsFrom string `yaml:"requirements_from,omitempty"`
	// PythonLauncher names a command written to bin/<platform>/ that runs the
	// drive's Python with the package tree on PYTHONPATH, for packages used
	// as libraries by scripts rather than through entry points. A package
	// with a launcher must install on every platform of the vault.
	PythonLauncher string `yaml:"python_launcher,omitempty"`
}

// BundleSource is one pinned download of an item bundle. An archive (.zip, .tgz
// or .tar.gz) is unpacked into Dest; any other file is saved there under the
// last segment of its URL. Apply refuses a source without a SHA-256 pin.
type BundleSource struct {
	URL    string `yaml:"url"`
	SHA256 string `yaml:"sha256"`
	// Dest is the subdirectory of the item directory to write into.
	Dest string `yaml:"dest,omitempty"`
	// Strip is the archive directory to unpack from, such as "package/dist" in
	// an npm tarball. Entries outside it are skipped. Empty means the root.
	Strip string `yaml:"strip,omitempty"`
	// Files limits extraction to these archive paths below Strip; each must
	// exist. Empty extracts every regular file.
	Files []string `yaml:"files,omitempty"`
	// Include limits extraction to the paths below Strip that match one of
	// these patterns: path.Match segment by segment, and a final "/**" for
	// everything below. Every pattern must match a file. Not with Files.
	Include []string `yaml:"include,omitempty" json:"Include,omitempty"`
	// Main, in a js-lib recipe, is the file below Strip that a bare import of
	// the package resolves to in place of its package.json entry: the
	// self-contained build when only that one is installed (d3's
	// dist/d3.min.js).
	Main string `yaml:"main,omitempty" json:"Main,omitempty"`
	// Extracted is what the source installs once trimmed, recorded by the
	// pin generator for the size budgets. It does not decide what is
	// installed, so it is left out of the acquisition hash.
	Extracted *ExtractedSize `yaml:"extracted,omitempty" json:"-"`
}

// EmbeddingSpec describes how an embedding model should be used at index and
// search time.  Stored in the recipe YAML and written as a sidecar JSON next
// to the downloaded GGUF so the indexer and search runtime can read it without
// loading the full catalog.
type EmbeddingSpec struct {
	DocPrefix      string `yaml:"doc_prefix,omitempty"       json:"doc_prefix,omitempty"`
	QueryPrefix    string `yaml:"query_prefix,omitempty"     json:"query_prefix,omitempty"`
	Dims           int    `yaml:"dims,omitempty"             json:"dims,omitempty"`
	Matryoshka     bool   `yaml:"matryoshka,omitempty"       json:"matryoshka,omitempty"`
	MaxInputTokens int    `yaml:"max_input_tokens,omitempty" json:"max_input_tokens,omitempty"`
}

// ViewerSpec describes how a recipe should be presented in a viewer.
type ViewerSpec struct {
	Style    map[string]map[string]any `yaml:"style,omitempty"`
	Name     string                    `yaml:"name"`
	Category string                    `yaml:"category"`
}

// LicenseSpec captures the licensing information for a recipe.
type LicenseSpec struct {
	ID          string `yaml:"id"`
	Attribution string `yaml:"attribution"`
	URL         string `yaml:"url"`
}

// MenuSpec defines how a recipe appears in the user-facing menu.
type MenuSpec struct {
	Group       string   `yaml:"group"`
	Subheader   string   `yaml:"subheader"`
	Label       string   `yaml:"label"`
	Description string   `yaml:"description"`
	Aliases     []string `yaml:"aliases,omitempty"`
	Order       int      `yaml:"order"`
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
	Path         string   `yaml:"-"` // source file of a user preset; empty for built-ins
}

// IsUser reports whether the preset was loaded from a user preset file.
func (p Preset) IsUser() bool { return p.Path != "" }

// ContentSizeGB estimates the resolved Items' size on a vault that targets
// platforms (see TotalSizeGB).
func (p Preset) ContentSizeGB(platforms []string) float64 {
	return TotalSizeGB(p.Items, platforms)
}

// Catalog holds all known recipes and presets.
type Catalog struct {
	recipes map[string]Item
	presets map[string]Preset
	skipped []SkippedPreset // user presets left out by LoadUserPresets

	skippedRecipes []SkippedRecipe // user recipes left out by LoadUserRecipes
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
	if err := c.validateDraftTargets(); err != nil {
		return nil, err
	}
	if err := c.validateSettings(); err != nil {
		return nil, err
	}
	if err := ValidateLibraries(c.AllRecipes()); err != nil {
		return nil, err
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
		if s, skipped := c.SkippedUserPreset(name); skipped {
			return Preset{}, s.Err()
		}
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

	preset.Sources = sources
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

// PresetByName looks up a single unresolved preset by name.
func (c *Catalog) PresetByName(name string) (Preset, bool) {
	p, ok := c.presets[name]
	return p, ok
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
