package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/tui"
)

// CatalogEntry is one recipe as the catalog command lists it.
type CatalogEntry struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	SizeGB      float64  `json:"size_gb"`
	Description string   `json:"description"`
	Packs       []string `json:"packs"`
	Tags        []string `json:"tags,omitempty"`
}

// CatalogEntries lists the recipes matching query, sorted by id and sized for
// platforms. Every word of query must appear in the recipe's id, type,
// description, tags or packs; an empty query matches all.
func CatalogEntries(cat *catalog.Catalog, platforms []string, query string) []CatalogEntry {
	packsOf := make(map[string][]string)
	for _, pack := range cat.Packs() {
		resolved, err := cat.ResolvePreset(pack.Name)
		if err != nil {
			continue
		}
		for _, item := range resolved.Items {
			packsOf[item.ID] = append(packsOf[item.ID], pack.Name)
		}
	}

	words := strings.Fields(strings.ToLower(query))
	entries := []CatalogEntry{}
	for _, item := range cat.AllRecipes() {
		packs := packsOf[item.ID]
		if packs == nil {
			packs = []string{}
		}
		sort.Strings(packs)
		entry := CatalogEntry{
			ID:          item.ID,
			Type:        item.Type,
			SizeGB:      item.SizeGBFor(platforms),
			Description: item.Description,
			Packs:       packs,
			Tags:        item.Tags,
		}
		haystack := strings.ToLower(strings.Join([]string{
			entry.ID, entry.Type, entry.Description,
			strings.Join(entry.Tags, " "), strings.Join(entry.Packs, " "),
		}, " "))
		matches := true
		for _, word := range words {
			matches = matches && strings.Contains(haystack, word)
		}
		if matches {
			entries = append(entries, entry)
		}
	}
	return entries
}

// WriteCatalog writes entries as an aligned table.
func WriteCatalog(w io.Writer, entries []CatalogEntry) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "ID\tType\tSize\tPacks\tDescription\n")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.ID, e.Type, tui.FormatSize(e.SizeGB), strings.Join(e.Packs, ","), e.Description)
	}
	return tw.Flush()
}

// PresetSummary is one preset or pack as "preset list --json" reports it.
type PresetSummary struct {
	Name        string  `json:"name"`
	Kind        string  `json:"kind"` // "preset" or "pack"
	Region      string  `json:"region,omitempty"`
	Description string  `json:"description,omitempty"`
	SizeGB      float64 `json:"size_gb"`
	Items       int     `json:"items"`
	UserPath    string  `json:"user_path,omitempty"`
}

// PresetSummaries describes every preset and pack, sorted by name and sized
// for platforms.
func PresetSummaries(cat *catalog.Catalog, platforms []string) []PresetSummary {
	summaries := []PresetSummary{}
	for _, name := range cat.PresetNames() {
		p, _ := cat.PresetByName(name)
		resolved, err := cat.ResolvePreset(name)
		if err != nil {
			continue
		}
		kind := "preset"
		if p.Kind == "pack" {
			kind = "pack"
		}
		summaries = append(summaries, PresetSummary{
			Name:        name,
			Kind:        kind,
			Region:      p.Region,
			Description: p.Description,
			SizeGB:      resolved.ContentSizeGB(platforms),
			Items:       len(resolved.Items),
			UserPath:    p.Path,
		})
	}
	return summaries
}

// PresetEntries lists the recipes preset name resolves to, as catalog
// entries.
func PresetEntries(cat *catalog.Catalog, platforms []string, name string) ([]CatalogEntry, error) {
	if _, ok := cat.PresetByName(name); !ok {
		if skipped, ok := cat.SkippedUserPreset(name); ok {
			return nil, skipped.Err()
		}
		return nil, fmt.Errorf("unknown preset %q; run \"svalbard preset list\"", name)
	}
	resolved, err := cat.ResolvePreset(name)
	if err != nil {
		return nil, err
	}
	in := make(map[string]bool, len(resolved.Items))
	for _, item := range resolved.Items {
		in[item.ID] = true
	}
	entries := []CatalogEntry{}
	for _, e := range CatalogEntries(cat, platforms, "") {
		if in[e.ID] {
			entries = append(entries, e)
		}
	}
	return entries, nil
}
