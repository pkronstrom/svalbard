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
			Desc: "List all content sources with descriptions and tags. CALL THIS FIRST to understand what is available before using search or query tools. Returns archive names, types, descriptions, topics, and sizes. A book (a PDF document) also has its path, which is its source name for search, its title, how many of its pages are searchable, and a note when some or all of it is not.",
			Params: []ParamDef{
				{Name: "type", Type: "string", Desc: "Optional source type filter", Enum: []string{"zim", "database", "map", "book"}},
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
			Desc:   "Get a high-level summary of the drive, including preset, region, creation date, and content counts.",
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
	// Enrich with recipe metadata
	for i := range sources {
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

	return ActionResult{Data: sources}, nil
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
