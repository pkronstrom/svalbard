package mcp

import (
	"context"
	"fmt"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/query"
)

// QueryCapability exposes SQLite database querying via the MCP "query" tool.
type QueryCapability struct {
	driveRoot string
	meta      DriveMetadata
}

// NewQueryCapability creates a query capability for the given drive.
func NewQueryCapability(driveRoot string, meta DriveMetadata) *QueryCapability {
	return &QueryCapability{driveRoot: driveRoot, meta: meta}
}

func (c *QueryCapability) Tool() string { return "query" }
func (c *QueryCapability) Description() string {
	return "Query structured SQLite databases on this drive (pharmaceutical registries, nutrition data, etc.)"
}

func (c *QueryCapability) Actions() []ActionDef {
	return []ActionDef{
		{
			Name: "describe",
			Desc: "Inspect a packaged SQLite database schema. Use this before query_sql to discover tables, columns, FTS support, and sample rows.",
			Params: []ParamDef{
				{Name: "database", Type: "string", Required: true, Desc: "Database filename in the drive data directory, for example: fimea.sqlite"},
				{Name: "table", Type: "string", Desc: "Optional table name to inspect. Omit to describe all tables."},
			},
		},
		{
			Name: "sql",
			Desc: "Run a read-only SQL query against a packaged SQLite database. Use only for SELECT-style queries; write statements are rejected.",
			Params: []ParamDef{
				{Name: "database", Type: "string", Required: true, Desc: "Database filename in the drive data directory, for example: fimea.sqlite"},
				{Name: "sql", Type: "string", Required: true, Desc: "Read-only SQL query to execute, typically a SELECT statement"},
			},
		},
	}
}

func (c *QueryCapability) Handle(_ context.Context, action string, params map[string]any) (ActionResult, error) {
	switch action {
	case "describe":
		return c.handleDescribe(params)
	case "sql":
		return c.handleSQL(params)
	default:
		return ActionResult{}, fmt.Errorf("unknown query action: %s", action)
	}
}

func (c *QueryCapability) Close() error { return nil }

func (c *QueryCapability) handleDescribe(params map[string]any) (ActionResult, error) {
	database, _ := params["database"].(string)
	if database == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: database")
	}
	table, _ := params["table"].(string)

	info, err := query.Describe(c.driveRoot, database, table)
	if err != nil {
		return ActionResult{}, fmt.Errorf("describe failed: %w", err)
	}

	return ActionResult{Data: info}, nil
}

func (c *QueryCapability) handleSQL(params map[string]any) (ActionResult, error) {
	database, _ := params["database"].(string)
	if database == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: database")
	}
	sqlQuery, _ := params["sql"].(string)
	if sqlQuery == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: sql")
	}

	rows, err := query.Execute(c.driveRoot, database, sqlQuery)
	if err != nil {
		return ActionResult{}, fmt.Errorf("query failed: %w", err)
	}

	return ActionResult{Data: rows}, nil
}
