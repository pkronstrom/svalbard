package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	searchserver "github.com/pkronstrom/svalbard/drive-runtime/internal/search/server"
)

// SearchCapability exposes search and read functionality via the MCP "search" tool.
// Uses ncruces/go-sqlite3 directly for FTS5 queries (no sqlite3 binary needed).
// Uses kiwix-serve only for browse/read with full HTML content.
type SearchCapability struct {
	driveRoot string
	meta      DriveMetadata

	dbOnce sync.Once
	db     *sql.DB
	dbErr  error
	eng    *engine.Engine

	embedBackend *search.SemanticBackend

	kiwixOnce sync.Once
	kiwixCmd  *exec.Cmd
	kiwixPort int
	kiwixErr  error
}

// NewSearchCapability creates a search capability for the given drive.
func NewSearchCapability(driveRoot string, meta DriveMetadata) *SearchCapability {
	return &SearchCapability{driveRoot: driveRoot, meta: meta, embedBackend: search.NewSemanticBackend(driveRoot)}
}

func (c *SearchCapability) Tool() string { return "search" }

func (c *SearchCapability) Actions() []ActionDef {
	return []ActionDef{
		{
			Name: "search",
			Desc: "Search packaged ZIM archives by keyword. Call vault_sources first to see available archives and their topics. Use specific terms relevant to the archive content (e.g. 'acetaminophen dosage' not 'medicine'). Results include exact source and path — pass these to search_read unchanged. Do not use for SQLite data; use query_sql instead.",
			Params: []ParamDef{
				{Name: "query", Type: "string", Required: true, Desc: "Keyword query to search for, for example: nmap, grep, package manager"},
				{Name: "source", Type: "string", Desc: "Optional ZIM source name to restrict results to one archive, for example: wikipedia_en_100_mini_2026-04"},
				{Name: "detail", Type: "string", Desc: "How much result content to return: link, snippet, or full article text", Default: "snippet", Enum: []string{"link", "snippet", "full"}},
				{Name: "limit", Type: "integer", Desc: "Maximum number of results to return, from 1 to 50", Default: 10},
			},
		},
		{
			Name: "read",
			Desc: "Read or browse a ZIM archive. Omit path to get the main page with navigable links (use this to discover what's inside an archive). Include path to read a specific article. Use the exact path from search results or from links returned by a previous read.",
			Params: []ParamDef{
				{Name: "source", Type: "string", Required: true, Desc: "ZIM source name without the .zim extension, for example: wikipedia_en_100_mini_2026-04"},
				{Name: "path", Type: "string", Desc: "Article path inside the ZIM. Omit to browse the main page and see available categories/links."},
			},
		},
	}
}

func (c *SearchCapability) Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return ActionResult{}, err
		}
	}
	switch action {
	case "search":
		return c.handleSearch(ctx, params)
	case "read":
		return c.handleRead(ctx, params)
	default:
		return ActionResult{}, fmt.Errorf("unknown search action: %s", action)
	}
}

func (c *SearchCapability) Close() error {
	if c.db != nil {
		c.db.Close()
	}
	if c.embedBackend != nil {
		_ = c.embedBackend.Close()
	}
	if c.kiwixCmd != nil && c.kiwixCmd.Process != nil {
		_ = c.kiwixCmd.Process.Kill()
		_ = c.kiwixCmd.Wait()
	}
	return nil
}

// searchResultItem is the JSON shape returned for search results.
type searchResultItem struct {
	Source   string            `json:"source"`
	Path     string            `json:"path"`
	Title    string            `json:"title"`
	ReadHint string            `json:"read_hint,omitempty"`
	Snippet  string            `json:"snippet,omitempty"`
	Body     string            `json:"body,omitempty"`
	Links    []search.PageLink `json:"links,omitempty"`
}

func (c *SearchCapability) getDB() (*sql.DB, *engine.Engine, error) {
	c.dbOnce.Do(func() {
		c.db, c.dbErr = openSearchDB(c.driveRoot)
		if c.dbErr == nil {
			c.eng = engine.New(c.db)
		}
	})
	return c.db, c.eng, c.dbErr
}

func (c *SearchCapability) handleSearch(ctx context.Context, params map[string]any) (ActionResult, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: query")
	}
	sourceFilter := normalizeSourceName(getString(params, "source"))

	detail := "snippet"
	if d, ok := params["detail"].(string); ok && d != "" {
		detail = d
	}

	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := params["limit"].(int); ok {
		limit = l
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	if detail == "full" && limit > 5 {
		limit = 5
	}

	db, eng, err := c.getDB()
	if err != nil {
		return ActionResult{}, err
	}

	// Fetch more results when filtering by source, then trim.
	// Use 200 to avoid missing source-specific hits that rank below global top-N.
	fetchLimit := limit
	if sourceFilter != "" && fetchLimit < 200 {
		fetchLimit = 200
	}

	// Building, legacy, and malformed semantic contracts remain keyword-only.
	contract, semanticComplete := search.LoadCompleteContract(db)
	// Try hybrid search if a completed semantic contract is available.
	var results []engine.Result
	hybridUsed := false
	advisory := ""
	if !semanticComplete {
		advisory = "Hybrid unavailable, fell back to keyword"
	}
	if semanticComplete {
		queryVec, embedErr := c.embedBackend.Embed(ctx, contract, query)
		if embedErr == nil {
			var usable bool
			if detail == "full" {
				results, usable, embedErr = search.HybridSnapshotFull(ctx, db, contract, query, queryVec, limit, sourceFilter)
			} else {
				results, usable, embedErr = search.HybridSnapshot(ctx, db, contract, query, queryVec, fetchLimit)
			}
			if !usable {
				results = nil
			} else {
				hybridUsed = true
			}
		}
		if embedErr != nil || results == nil {
			advisory = "Hybrid unavailable, fell back to keyword"
		}
	}
	if err := ctx.Err(); err != nil {
		return ActionResult{}, err
	}

	// Fallback to keyword search.
	if results == nil {
		hybridUsed = false
		results, err = eng.KeywordContext(ctx, query, fetchLimit)
		if err != nil {
			return ActionResult{}, fmt.Errorf("search failed: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return ActionResult{}, err
		}
	}

	if sourceFilter != "" {
		filtered := make([]engine.Result, 0, len(results))
		for _, r := range results {
			if normalizeSourceName(r.Filename) == sourceFilter {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}
	if len(results) > limit {
		results = results[:limit]
	}

	items := make([]searchResultItem, 0, len(results))
	for _, r := range results {
		item := searchResultItem{
			Source:   normalizeSourceName(r.Filename),
			Path:     r.Path,
			Title:    r.Title,
			ReadHint: "Use search_read with this exact source and path",
		}

		// Prepend chunk header for section context (e.g., "Water Purification > Solar disinfection")
		if r.ChunkHeader != "" && r.ChunkHeader != r.Title {
			item.Title = r.ChunkHeader
		}

		switch detail {
		case "snippet":
			item.Snippet = r.Snippet
			// If FTS snippet is empty, try article body from DB
			if !hybridUsed && item.Snippet == "" {
				if body, err := eng.ReadArticle(r.Filename, r.Path); err == nil && body != "" {
					if len(body) > 200 {
						body = body[:200] + "..."
					}
					item.Snippet = body
				}
			}
		case "full":
			// Try reading from search.db first (fast, no kiwix needed)
			if hybridUsed {
				// HybridSnapshot captured this body in its verified read snapshot.
				item.Body = r.Body
			} else if r.Body != "" {
				item.Body = r.Body
			} else if body, err := eng.ReadArticle(r.Filename, r.Path); err == nil && body != "" {
				item.Body = body
			} else {
				// Fallback to kiwix for full HTML content
				if page, fetchErr := c.fetchPage(normalizeSourceName(r.Filename), r.Path); fetchErr == nil {
					item.Body = page.Body
					item.Links = page.Links
				} else {
					item.Snippet = r.Snippet // last resort
				}
			}
		}

		items = append(items, item)
	}

	return ActionResult{Data: items, Text: advisory}, nil
}

func (c *SearchCapability) handleRead(_ context.Context, params map[string]any) (ActionResult, error) {
	source, _ := params["source"].(string)
	if source == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: source")
	}
	path, _ := params["path"].(string)

	// Try Kiwix first — gives full HTML with links (best experience)
	if page, err := c.fetchPage(source, path); err == nil {
		return ActionResult{Data: searchResultItem{
			Source: source,
			Path:   path,
			Title:  page.Title,
			Body:   page.Body,
			Links:  page.Links,
		}}, nil
	}

	// Fallback to search.db via engine (partial body, no links, but no Kiwix needed)
	if path != "" {
		_, eng, err := c.getDB()
		if err == nil {
			if body, err := eng.ReadArticle(source, path); err == nil && body != "" {
				return ActionResult{Data: searchResultItem{
					Source: source,
					Path:   path,
					Title:  path,
					Body:   body,
				}}, nil
			}
		}
	}

	return ActionResult{}, fmt.Errorf("article not found: %s/%s", source, path)
}

func getString(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return value
}

func normalizeSourceName(source string) string {
	return strings.TrimSuffix(source, ".zim")
}

// ensureKiwix starts kiwix-serve lazily.
func (c *SearchCapability) ensureKiwix() error {
	c.kiwixOnce.Do(func() {
		kiwixBin, err := binary.Resolve("kiwix-serve", c.driveRoot)
		if err != nil {
			c.kiwixErr = fmt.Errorf("kiwix-serve not found: %w", err)
			return
		}

		zims, err := filepath.Glob(filepath.Join(c.driveRoot, "zim", "*.zim"))
		if err != nil {
			c.kiwixErr = err
			return
		}

		port, err := netutil.FindAvailablePort("127.0.0.1", 8080)
		if err != nil {
			c.kiwixErr = err
			return
		}
		cmd, err := searchserver.StartKiwix(context.Background(), kiwixBin, zims, port)
		if err != nil {
			c.kiwixErr = err
			return
		}
		c.kiwixCmd = cmd
		c.kiwixPort = port
	})
	return c.kiwixErr
}

func (c *SearchCapability) fetchPage(source, path string) (search.Page, error) {
	if err := c.ensureKiwix(); err != nil {
		return search.Page{}, fmt.Errorf("kiwix-serve unavailable: %w", err)
	}

	pageURL := fmt.Sprintf("http://127.0.0.1:%d/content/%s/%s", c.kiwixPort, source, path)

	resp, err := http.Get(pageURL)
	if err != nil {
		return search.Page{}, fmt.Errorf("fetching page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return search.Page{}, fmt.Errorf("kiwix returned status %d for %s/%s", resp.StatusCode, source, path)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return search.Page{}, fmt.Errorf("reading page body: %w", err)
	}

	return search.ExtractText(string(body))
}
