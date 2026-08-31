package tui

import "strings"

// PaletteEntry is one indexable item in the command palette.
type PaletteEntry struct {
	ID      string
	Label   string
	Aliases []string
}

// PaletteResult is a matched entry.
type PaletteResult struct {
	PaletteEntry
}

// Palette provides matching over a set of entries.
type Palette struct {
	Entries []PaletteEntry
}

// An empty query returns every entry. For non-empty queries the search is
// case-insensitive across each entry's Label, ID, and Aliases.
func (p *Palette) Match(query string) []PaletteResult {
	if query == "" {
		results := make([]PaletteResult, len(p.Entries))
		for i, e := range p.Entries {
			results[i] = PaletteResult{PaletteEntry: e}
		}
		return results
	}

	q := strings.ToLower(query)
	var results []PaletteResult
	for _, e := range p.Entries {
		if containsLower(e.Label, q) || containsLower(e.ID, q) {
			results = append(results, PaletteResult{PaletteEntry: e})
			continue
		}
		for _, alias := range e.Aliases {
			if containsLower(alias, q) {
				results = append(results, PaletteResult{PaletteEntry: e})
				break
			}
		}
	}

	if results == nil {
		return []PaletteResult{}
	}
	return results
}

// containsLower reports whether s contains substr using a case-insensitive comparison.
func containsLower(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), substr)
}
