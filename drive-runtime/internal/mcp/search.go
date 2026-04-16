package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

// SearchCapability exposes search and read functionality via the MCP "search" tool.
// Uses modernc.org/sqlite directly for FTS5 queries (no sqlite3 binary needed).
// Uses kiwix-serve only for browse/read with full HTML content.
type SearchCapability struct {
	driveRoot string
	meta      DriveMetadata

	dbOnce sync.Once
	db     *searchDB
	dbErr  error

	kiwixOnce sync.Once
	kiwixCmd  *exec.Cmd
	kiwixPort int
	kiwixErr  error
}

// NewSearchCapability creates a search capability for the given drive.
func NewSearchCapability(driveRoot string, meta DriveMetadata) *SearchCapability {
	return &SearchCapability{driveRoot: driveRoot, meta: meta}
}

func (c *SearchCapability) Tool() string { return "search" }
func (c *SearchCapability) Description() string {
	return "Search and read offline ZIM archives on this drive. IMPORTANT: First call vault_sources to see what archives are available — searches only work against indexed content. Use specific terms matching the archive topics, not generic web-style queries."
}

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
	if c.kiwixCmd != nil && c.kiwixCmd.Process != nil {
		_ = c.kiwixCmd.Process.Kill()
		_, _ = c.kiwixCmd.Process.Wait()
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

func (c *SearchCapability) getDB() (*searchDB, error) {
	c.dbOnce.Do(func() {
		c.db, c.dbErr = openSearchDB(c.driveRoot)
	})
	return c.db, c.dbErr
}

func (c *SearchCapability) handleSearch(_ context.Context, params map[string]any) (ActionResult, error) {
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

	db, err := c.getDB()
	if err != nil {
		return ActionResult{}, err
	}

	// Fetch more results when filtering by source, then trim.
	fetchLimit := limit
	if sourceFilter != "" && fetchLimit < 50 {
		fetchLimit = 50
	}

	results, err := db.keywordSearch(query, fetchLimit)
	if err != nil {
		return ActionResult{}, fmt.Errorf("search failed: %w", err)
	}

	if sourceFilter != "" {
		filtered := make([]search.Result, 0, len(results))
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

		switch detail {
		case "snippet":
			item.Snippet = r.Snippet
			// If FTS snippet is empty, try article body from DB
			if item.Snippet == "" {
				if body, err := db.readArticle(r.Filename, r.Path); err == nil && body != "" {
					if len(body) > 200 {
						body = body[:200] + "..."
					}
					item.Snippet = body
				}
			}
		case "full":
			// Try reading from search.db first (fast, no kiwix needed)
			if body, err := db.readArticle(r.Filename, r.Path); err == nil && body != "" {
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

	return ActionResult{Data: items}, nil
}

func (c *SearchCapability) handleRead(_ context.Context, params map[string]any) (ActionResult, error) {
	source, _ := params["source"].(string)
	if source == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: source")
	}
	path, _ := params["path"].(string)

	// Try search.db first (has article body text, fast)
	if path != "" {
		db, err := c.getDB()
		if err == nil {
			if body, err := db.readArticle(source, path); err == nil && body != "" {
				return ActionResult{Data: searchResultItem{
					Source: source,
					Path:   path,
					Title:  path, // best we have from DB
					Body:   body,
				}}, nil
			}
		}
	}

	// Fallback to kiwix for full HTML with links (needed for browse/main page)
	page, err := c.fetchPage(source, path)
	if err != nil {
		return ActionResult{}, err
	}

	return ActionResult{Data: searchResultItem{
		Source: source,
		Path:   path,
		Title:  page.Title,
		Body:   page.Body,
		Links:  page.Links,
	}}, nil
}

func getString(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return value
}

func normalizeSourceName(source string) string {
	return strings.TrimSuffix(source, ".zim")
}

// ensureKiwix starts kiwix-serve lazily. Handles the case where the binary
// is inside a subdirectory (e.g. bin/macos-arm64/kiwix-serve/kiwix-serve).
func (c *SearchCapability) ensureKiwix() error {
	c.kiwixOnce.Do(func() {
		kiwixBin, err := resolveKiwixBinary(c.driveRoot)
		if err != nil {
			c.kiwixErr = fmt.Errorf("kiwix-serve not found: %w", err)
			return
		}

		zims, _ := filepath.Glob(filepath.Join(c.driveRoot, "zim", "*.zim"))
		if len(zims) == 0 {
			c.kiwixErr = fmt.Errorf("no ZIM files found")
			return
		}

		port, err := netutil.FindAvailablePort("127.0.0.1", 8080)
		if err != nil {
			c.kiwixErr = err
			return
		}

		args := []string{"--port", fmt.Sprintf("%d", port), "--address", "127.0.0.1"}
		args = append(args, zims...)

		cmd := exec.Command(kiwixBin, args...)
		if err := cmd.Start(); err != nil {
			c.kiwixErr = fmt.Errorf("starting kiwix-serve: %w", err)
			return
		}

		// Health check
		healthURL := fmt.Sprintf("http://127.0.0.1:%d/", port)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					c.kiwixCmd = cmd
					c.kiwixPort = port
					return
				}
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Timeout — kill orphan
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		c.kiwixErr = fmt.Errorf("kiwix-serve did not become healthy on port %d", port)
	})
	return c.kiwixErr
}

// resolveKiwixBinary handles the case where bin/platform/kiwix-serve is a
// directory containing the actual binary (common after archive extraction).
func resolveKiwixBinary(driveRoot string) (string, error) {
	path, err := binary.Resolve("kiwix-serve", driveRoot, platform.Detect)
	if err != nil {
		return "", err
	}
	// Check if resolved path is a directory (extracted archive)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		// Look for the binary inside the directory
		inner := filepath.Join(path, "kiwix-serve")
		if fi, err := os.Stat(inner); err == nil && !fi.IsDir() {
			return inner, nil
		}
		// Try to find any executable named kiwix-serve inside subdirs
		matches, _ := filepath.Glob(filepath.Join(path, "*", "kiwix-serve"))
		if len(matches) > 0 {
			return matches[0], nil
		}
		return "", fmt.Errorf("kiwix-serve directory found but no binary inside: %s", path)
	}
	return path, nil
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
