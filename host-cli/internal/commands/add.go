package commands

import (
	"fmt"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// ExpandItemRefs replaces each preset or pack name in refs with the recipe ids
// it resolves to. Recipe ids, local ids and unknown refs pass through
// unchanged; a recipe id wins over a preset of the same name. Naming a user
// preset that was skipped at load is an error that says why.
func ExpandItemRefs(cat *catalog.Catalog, refs []string) ([]string, error) {
	var ids []string
	for _, ref := range refs {
		if _, ok := cat.RecipeByID(ref); ok {
			ids = append(ids, ref)
			continue
		}
		if _, ok := cat.PresetByName(ref); !ok {
			if skipped, ok := cat.SkippedUserPreset(ref); ok {
				return nil, skipped.Err()
			}
			ids = append(ids, ref)
			continue
		}
		preset, err := cat.ResolvePreset(ref)
		if err != nil {
			return nil, err
		}
		for _, item := range preset.Items {
			ids = append(ids, item.ID)
		}
	}
	return ids, nil
}

// AddItems appends ids to m.Desired.Items, skipping any that are already present.
// This is a metadata-only operation with no side effects.
func AddItems(m *manifest.Manifest, ids []string) error {
	existing := make(map[string]struct{}, len(m.Desired.Items))
	for _, item := range m.Desired.Items {
		existing[item] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			m.Desired.Items = append(m.Desired.Items, id)
			existing[id] = struct{}{}
		}
	}
	return nil
}

// DrafterWarnings describes each selected drafter whose target models are all
// unselected, so the user can add a target or drop the drafter.
func DrafterWarnings(cat *catalog.Catalog, ids []string) []string {
	var warnings []string
	for _, id := range cat.UnpairedDrafters(ids) {
		item, _ := cat.RecipeByID(id)
		warnings = append(warnings, fmt.Sprintf("warning: %s only speeds up %s, and none is selected; it will download unused", id, strings.Join(item.DraftFor, ", ")))
	}
	return warnings
}
