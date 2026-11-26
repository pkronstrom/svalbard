package commands

import (
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// RemoveItems filters m.Desired.Items to exclude any ids in the given list.
// This is a metadata-only operation with no side effects.
func RemoveItems(m *manifest.Manifest, ids []string) error {
	toRemove := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		toRemove[id] = struct{}{}
	}
	filtered := make([]string, 0, len(m.Desired.Items))
	for _, item := range m.Desired.Items {
		if _, ok := toRemove[item]; !ok {
			filtered = append(filtered, item)
		}
	}
	m.Desired.Items = filtered
	return nil
}

// StillRequired returns the removed ids that an item left in remaining
// depends on, sorted.
func StillRequired(cat *catalog.Catalog, defaults catalog.DepDefaults, remaining, removed []string) []string {
	deps, _ := RequiredDependencies(cat, defaults, remaining)
	wasRemoved := make(map[string]bool, len(removed))
	for _, id := range removed {
		wasRemoved[id] = true
	}
	var required []string
	for _, id := range deps {
		if wasRemoved[id] {
			required = append(required, id)
		}
	}
	return required
}
