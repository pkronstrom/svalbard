package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

// VaultCapability exposes drive inventory via the MCP "vault" tool.
type VaultCapability struct {
	driveRoot string
	meta      DriveMetadata
}

// NewVaultCapability creates a vault capability for the given drive.
func NewVaultCapability(driveRoot string, meta DriveMetadata) *VaultCapability {
	return &VaultCapability{driveRoot: driveRoot, meta: meta}
}

func (c *VaultCapability) Tool() string { return "vault" }

func (c *VaultCapability) Actions() []ActionDef {
	return []ActionDef{
		{
			Name: "sources",
			Desc: "Find particular content sources by name or topic, or list a page of sources. Use vault_stats for a broad overview and search for a known topic. Returns source names, descriptions, sizes, and any document indexing notes.",
			Params: []ParamDef{
				{Name: "type", Type: "string", Desc: "Optional source type filter", Enum: []string{"zim", "database", "map", "book"}},
				{Name: "match", Type: "string", Desc: "Filter by part of a source name, title, description or tag"},
				{Name: "limit", Type: "integer", Desc: "Sources per page, up to 50", Default: 12},
				{Name: "offset", Type: "integer", Desc: "Start at this zero-based position in the filtered list", Default: 0},
			},
		},
		{
			Name:   "databases",
			Desc:   "List packaged SQLite databases and their tables. Use this before query_describe or query_sql when you need to discover structured data sources.",
			Params: nil,
		},
		{
			Name:   "maps",
			Desc:   "List packaged PMTiles map files available on the drive.",
			Params: nil,
		},
		{
			Name:   "stats",
			Desc:   "Give a broad overview of what is available on the drive. Includes the preset, region, file counts and sizes by directory, and the combined size of those directories.",
			Params: nil,
		},
	}
}

func (c *VaultCapability) Handle(_ context.Context, action string, params map[string]any) (ActionResult, error) {
	switch action {
	case "sources":
		return c.handleSources(params)
	case "databases":
		return c.handleDatabases()
	case "maps":
		return c.handleMaps()
	case "stats":
		return c.handleStats()
	default:
		return ActionResult{}, fmt.Errorf("unknown vault action: %s", action)
	}
}

func (c *VaultCapability) Close() error { return nil }

func (c *VaultCapability) handleSources(params map[string]any) (ActionResult, error) {
	filter, _ := params["type"].(string)
	sources, err := inspect.Sources(c.driveRoot, filter)
	if err != nil {
		return ActionResult{}, fmt.Errorf("listing sources: %w", err)
	}

	documents := c.indexedDocuments()
	inArchives := map[string]int{}
	for filename := range documents {
		if archive, _, ok := engine.EmbeddedArchive(filename); ok {
			inArchives[archive]++
		}
	}
	// Enrich with recipe metadata
	for i := range sources {
		if count, ok := inArchives[sources[i].Name]; ok && sources[i].Type == "zim" {
			sources[i].Documents = &count
		}
		if doc, ok := documents[sources[i].Path]; ok {
			pages := doc.Pages
			sources[i].Title, sources[i].PagesIndexed, sources[i].Note = doc.Title, &pages, doc.Note
		}
		// A book in a folder is matched by its path only: every document of
		// a collection is an entry of the same recipe in the manifest, and
		// two collections may hold a file of the same name.
		recipe, ok := c.meta.RecipeForFile(sources[i].Path)
		if !ok && strings.Count(sources[i].Path, "/") <= 1 {
			recipe, ok = c.meta.RecipeForFile(sources[i].Name)
		}
		if ok {
			if sources[i].Description == "" && recipe.Description != "" {
				sources[i].Description = recipe.Description
			}
			if len(sources[i].Tags) == 0 && len(recipe.Tags) > 0 {
				sources[i].Tags = recipe.Tags
			}
		}
	}

	if match := strings.ToLower(strings.TrimSpace(getString(params, "match"))); match != "" {
		filtered := sources[:0]
		for _, source := range sources {
			text := strings.Join([]string{source.Name, source.Title, source.Description, strings.Join(source.Tags, " ")}, " ")
			if strings.Contains(strings.ToLower(text), match) {
				filtered = append(filtered, source)
			}
		}
		sources = filtered
	}
	offset := min(len(sources), max(0, intParam(params, "offset", 0)))
	limit := min(50, max(1, intParam(params, "limit", 12)))
	end := min(len(sources), offset+limit)
	note := ""
	if end < len(sources) {
		note = fmt.Sprintf("Showing %d–%d of %d sources. Use offset %d for the next page, or narrow with match.", offset+1, end, len(sources), end)
	}
	return ActionResult{Data: sources[offset:end], Text: note}, nil
}

// indexedDocuments returns what the search index holds about each document,
// by its path on the drive. A drive without an index has none.
func (c *VaultCapability) indexedDocuments() map[string]engine.Document {
	db, err := openSearchDB(c.driveRoot)
	if err != nil {
		return nil
	}
	defer db.Close()
	documents, err := engine.New(db).Documents(context.Background())
	if err != nil {
		return nil
	}
	byPath := make(map[string]engine.Document, len(documents))
	for _, doc := range documents {
		byPath[doc.Filename] = doc
	}
	return byPath
}

func (c *VaultCapability) handleDatabases() (ActionResult, error) {
	dbs, err := inspect.Databases(c.driveRoot)
	if err != nil {
		return ActionResult{}, fmt.Errorf("listing databases: %w", err)
	}
	return ActionResult{Data: dbs}, nil
}

func (c *VaultCapability) handleMaps() (ActionResult, error) {
	maps, err := inspect.Maps(c.driveRoot)
	if err != nil {
		return ActionResult{}, fmt.Errorf("listing maps: %w", err)
	}

	// Enrich with recipe metadata
	for i := range maps {
		if recipe, ok := c.meta.RecipeForFile(maps[i].Name); ok {
			if maps[i].Category == "" {
				if cat, ok := recipe.Viewer["category"].(string); ok {
					maps[i].Category = cat
				}
			}
			if maps[i].Coverage == "" {
				if cov, ok := recipe.Viewer["coverage"].(string); ok {
					maps[i].Coverage = cov
				}
			}
		}
	}

	return ActionResult{Data: maps}, nil
}

func (c *VaultCapability) handleStats() (ActionResult, error) {
	stats, err := inspect.Stats(c.driveRoot)
	if err != nil {
		return ActionResult{}, fmt.Errorf("computing stats: %w", err)
	}
	return ActionResult{Data: stats}, nil
}
