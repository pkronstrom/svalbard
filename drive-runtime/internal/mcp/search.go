package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
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

	embedOnce sync.Once
	embedCmd  *exec.Cmd
	embedPort int
	embedErr  error
	queryPfx  string

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
	if c.embedCmd != nil && c.embedCmd.Process != nil {
		_ = c.embedCmd.Process.Kill()
		_, _ = c.embedCmd.Process.Wait()
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

func (c *SearchCapability) getDB() (*sql.DB, *engine.Engine, error) {
	c.dbOnce.Do(func() {
		c.db, c.dbErr = openSearchDB(c.driveRoot)
		if c.dbErr == nil {
			c.eng = engine.New(c.db)
		}
	})
	return c.db, c.eng, c.dbErr
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

	_, eng, err := c.getDB()
	if err != nil {
		return ActionResult{}, err
	}

	// Fetch more results when filtering by source, then trim.
	fetchLimit := limit
	if sourceFilter != "" && fetchLimit < 50 {
		fetchLimit = 50
	}

	// Try hybrid search if embedding server is available.
	var results []engine.Result
	if c.ensureEmbedServer() == nil {
		queryVec, embedErr := embedQuery(c.queryPfx+query, c.embedPort)
		if embedErr == nil {
			results, _ = eng.Hybrid(query, queryVec, fetchLimit)
		}
	}

	// Fallback to keyword search.
	if results == nil {
		results, err = eng.Keyword(query, fetchLimit)
		if err != nil {
			return ActionResult{}, fmt.Errorf("search failed: %w", err)
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

		switch detail {
		case "snippet":
			item.Snippet = r.Snippet
			// If FTS snippet is empty, try article body from DB
			if item.Snippet == "" {
				if body, err := eng.ReadArticle(r.Filename, r.Path); err == nil && body != "" {
					if len(body) > 200 {
						body = body[:200] + "..."
					}
					item.Snippet = body
				}
			}
		case "full":
			// Try reading from search.db first (fast, no kiwix needed)
			if body, err := eng.ReadArticle(r.Filename, r.Path); err == nil && body != "" {
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

// ensureEmbedServer starts llama-server for query embedding lazily on first
// hybrid search request. Uses the same sync.Once pattern as ensureKiwix.
func (c *SearchCapability) ensureEmbedServer() error {
	c.embedOnce.Do(func() {
		llamaBin, err := binary.Resolve("llama-server", c.driveRoot, platform.Detect)
		if err != nil {
			c.embedErr = fmt.Errorf("llama-server not found: %w", err)
			return
		}

		modelPath := findEmbeddingModel(c.driveRoot)
		if modelPath == "" {
			c.embedErr = fmt.Errorf("no embedding model found")
			return
		}

		// Read query prefix from DB meta.
		if db, _, err := c.getDB(); err == nil {
			_ = db.QueryRow("SELECT COALESCE((SELECT value FROM meta WHERE key='embedding_query_prefix'), '')").Scan(&c.queryPfx)
		}

		port, err := netutil.FindAvailablePort("127.0.0.1", 8085)
		if err != nil {
			c.embedErr = err
			return
		}

		cmd := exec.Command(llamaBin, "--model", modelPath, "--port", fmt.Sprintf("%d", port), "--host", "127.0.0.1", "--embedding")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			c.embedErr = fmt.Errorf("starting llama-server: %w", err)
			return
		}

		healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get(healthURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				c.embedCmd = cmd
				c.embedPort = port
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(500 * time.Millisecond)
		}

		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		c.embedErr = fmt.Errorf("llama-server did not become healthy on port %d", port)
	})
	return c.embedErr
}

func findEmbeddingModel(driveRoot string) string {
	matches, _ := filepath.Glob(filepath.Join(driveRoot, "models", "embed", "*.gguf"))
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), "._") {
			return m
		}
	}
	return ""
}

func embedQuery(query string, port int) ([]float32, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/embedding", port)
	payload := map[string][]string{"content": {query}}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data []struct {
		Embedding json.RawMessage `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	// Handle both nested [[...]] and flat [...] response formats.
	var nested [][]float32
	if err := json.Unmarshal(data[0].Embedding, &nested); err == nil && len(nested) > 0 {
		return nested[0], nil
	}
	var vec []float32
	if err := json.Unmarshal(data[0].Embedding, &vec); err != nil {
		return nil, err
	}
	return vec, nil
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
