package mcp

import (
	"context"
	"fmt"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
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

func (c *VaultCapability) Tool() string        { return "vault" }
func (c *VaultCapability) Description() string {
	return "Discover what content is available on this offline drive. Call vault_sources FIRST before searching — it shows all archives with descriptions and tags so you know what topics are searchable."
}

func (c *VaultCapability) Actions() []ActionDef {
	return []ActionDef{
		{
			Name: "sources",
			Desc: "List all content sources with descriptions and tags. CALL THIS FIRST to understand what is available before using search or query tools. Returns archive names, types, descriptions, topics, and sizes.",
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
	var filterArgs []string
	if t, ok := params["type"].(string); ok && t != "" {
		filterArgs = append(filterArgs, t)
	}

	sources, err := inspect.Sources(c.driveRoot, filterArgs...)
	if err != nil {
		return ActionResult{}, fmt.Errorf("listing sources: %w", err)
	}

	// Enrich with recipe metadata
	for i := range sources {
		if recipe, ok := c.meta.Recipes[sources[i].ID]; ok {
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
		id := maps[i].Name
		// Try without extension
		if idx := len(id) - len(".pmtiles"); idx > 0 {
			id = id[:idx]
		}
		if recipe, ok := c.meta.Recipes[id]; ok {
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
