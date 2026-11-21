package catalog

import (
	"fmt"
	"sort"
)

// validateDraftTargets rejects a drafter that names a model the catalog lacks.
func (c *Catalog) validateDraftTargets() error {
	for id, item := range c.recipes {
		for _, target := range item.DraftFor {
			if _, ok := c.recipes[target]; !ok {
				return fmt.Errorf("recipe %s: draft_for target %q not found", id, target)
			}
		}
	}
	return nil
}

// UnpairedDrafters returns the selected drafters whose target models are all
// unselected; such a drafter downloads but is never used.
func (c *Catalog) UnpairedDrafters(selected []string) []string {
	chosen := make(map[string]bool, len(selected))
	for _, id := range selected {
		chosen[id] = true
	}
	var unpaired []string
	for _, id := range selected {
		item, ok := c.recipes[id]
		if !ok || len(item.DraftFor) == 0 {
			continue
		}
		paired := false
		for _, target := range item.DraftFor {
			if chosen[target] {
				paired = true
				break
			}
		}
		if !paired {
			unpaired = append(unpaired, id)
		}
	}
	sort.Strings(unpaired)
	return unpaired
}
