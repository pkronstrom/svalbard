package tui

import "strings"

// PaletteEntry is one indexable item in the command palette.
type PaletteEntry struct {
	ID              string
	Label           string
	Aliases         []string
	Verbs           []string // optional verb prefixes (e.g., "browse", "add")
	AcceptsFreeform bool     // trailing text after verb becomes FreeformArg
}

// PaletteResult is a matched entry with optional parsed freeform argument.
type PaletteResult struct {
	PaletteEntry
	FreeformArg string
}

// Palette provides matching over a set of entries.
type Palette struct {
	Entries []PaletteEntry
}

// Match returns all palette entries that match the query string.
//
// An empty query returns every entry. For non-empty queries the search is
// case-insensitive and proceeds in three stages:
//
//  1. Direct match — the entry's Label or ID contains the query substring.
//  2. Alias match — any of the entry's Aliases contains the query substring.
//  3. Verb prefix match — the query is split on the first space into "verb rest".
//     If verb matches one of the entry's Verbs and either rest matches
//     Label/ID/Alias or the entry accepts freeform input, the entry matches
//     (with FreeformArg set to rest when AcceptsFreeform is true).
//
// All matching entries are returned, not just the first.
func (p *Palette) Match(query string) []PaletteResult {
	if query == "" {
		results := make([]PaletteResult, len(p.Entries))
		for i, e := range p.Entries {
			results[i] = PaletteResult{PaletteEntry: e}
		}
		return results
	}

	q := strings.ToLower(query)

	// Track which entries are already matched to avoid duplicates.
	seen := make(map[int]bool)
	var results []PaletteResult

	// Stage 1 & 2: direct and alias substring matching.
	for i, e := range p.Entries {
		if containsLower(e.Label, q) || containsLower(e.ID, q) {
			seen[i] = true
			results = append(results, PaletteResult{PaletteEntry: e})
			continue
		}
		for _, alias := range e.Aliases {
			if containsLower(alias, q) {
				seen[i] = true
				results = append(results, PaletteResult{PaletteEntry: e})
				break
			}
		}
	}

	// Stage 3: verb prefix matching.
	verb, rest, hasSpace := strings.Cut(q, " ")
	if hasSpace {
		for i, e := range p.Entries {
			if seen[i] {
				continue
			}
			if !matchesVerb(e.Verbs, verb) {
				continue
			}
			// Verb matched — check whether the rest matches label/id/alias.
			if containsLower(e.Label, rest) || containsLower(e.ID, rest) {
				seen[i] = true
				results = append(results, PaletteResult{PaletteEntry: e})
				continue
			}
			for _, alias := range e.Aliases {
				if containsLower(alias, rest) {
					seen[i] = true
					results = append(results, PaletteResult{PaletteEntry: e})
					break
				}
			}
			if seen[i] {
				continue
			}
			// If the entry accepts freeform input, match with rest as FreeformArg.
			if e.AcceptsFreeform {
				seen[i] = true
				results = append(results, PaletteResult{
					PaletteEntry: e,
					FreeformArg:  rest,
				})
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

// matchesVerb reports whether any verb in verbs equals v (case-insensitive).
func matchesVerb(verbs []string, v string) bool {
	for _, verb := range verbs {
		if strings.EqualFold(verb, v) {
			return true
		}
	}
	return false
}
