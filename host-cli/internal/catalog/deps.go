package catalog

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// DepDefaults maps recipe type to default dep IDs.
type DepDefaults map[string][]string

// LoadDepDefaults reads dep-defaults.yaml from the given path.
func LoadDepDefaults(path string) (DepDefaults, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DepDefaults{}, nil
		}
		return nil, err
	}
	var defaults DepDefaults
	if err := yaml.Unmarshal(data, &defaults); err != nil {
		return nil, err
	}
	return defaults, nil
}

// depsForItem returns the dep IDs for an item.
// If the item has an explicit deps field (non-nil), use it.
// Otherwise fall back to type-level defaults.
func depsForItem(item Item, defaults DepDefaults) []string {
	if item.Deps != nil {
		return item.Deps
	}
	return defaults[item.Type]
}

// ResolveDeps takes user-selected IDs and returns the set of auto-dep IDs.
// IDs already in selectedIDs are not returned as auto-deps.
func (c *Catalog) ResolveDeps(selectedIDs map[string]bool, defaults DepDefaults) map[string]bool {
	autoDeps := make(map[string]bool)
	visited := make(map[string]bool)

	var visit func(id string, isAuto bool)
	visit = func(id string, isAuto bool) {
		if visited[id] {
			return
		}
		item, ok := c.recipes[id]
		if !ok {
			if isAuto {
				slog.Warn("dep not found in recipe index", "dep", id)
			}
			return
		}
		visited[id] = true
		if isAuto && !selectedIDs[id] {
			autoDeps[id] = true
		}
		for _, depID := range depsForItem(item, defaults) {
			visit(depID, true)
		}
	}

	for id := range selectedIDs {
		visit(id, false)
	}

	return autoDeps
}
