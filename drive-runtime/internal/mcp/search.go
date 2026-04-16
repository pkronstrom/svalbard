package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

// SearchCapability exposes search and read functionality via the MCP "search" tool.
type SearchCapability struct {
	driveRoot   string
	meta        DriveMetadata
	sessionOnce sync.Once
	session     *search.Session
	sessionErr  error
}

// NewSearchCapability creates a search capability for the given drive.
func NewSearchCapability(driveRoot string, meta DriveMetadata) *SearchCapability {
	return &SearchCapability{driveRoot: driveRoot, meta: meta}
}

func (c *SearchCapability) Tool() string        { return "search" }
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
	if c.session != nil {
		return c.session.Close()
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

func (c *SearchCapability) getSession() (*search.Session, error) {
	c.sessionOnce.Do(func() {
		c.session, c.sessionErr = search.NewSession(c.driveRoot, nil)
	})
	return c.session, c.sessionErr
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

	// Clamp limit.
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	if detail == "full" && limit > 5 {
		limit = 5
	}

	session, err := c.getSession()
	if err != nil {
		return ActionResult{}, err
	}

	mode := search.ModeKeyword
	if info := session.Info(); info.SemanticEnabled {
		mode = info.BestMode
	}

	fetchLimit := limit
	if sourceFilter != "" && fetchLimit < 50 {
		fetchLimit = 50
	}

	resp, err := session.Search(ctx, mode, query, fetchLimit)
	if err != nil {
		return ActionResult{}, fmt.Errorf("search failed: %w", err)
	}

	if sourceFilter != "" {
		filtered := make([]search.Result, 0, len(resp.Results))
		for _, r := range resp.Results {
			if normalizeSourceName(r.Filename) == sourceFilter {
				filtered = append(filtered, r)
			}
		}
		resp.Results = filtered
	}
	if len(resp.Results) > limit {
		resp.Results = resp.Results[:limit]
	}

	items := make([]searchResultItem, 0, len(resp.Results))
	for _, r := range resp.Results {
		source := normalizeSourceName(r.Filename)
		item := newSearchResultItem(r)

		switch detail {
		case "snippet":
			item.Snippet = r.Snippet
		case "full":
			page, fetchErr := c.fetchPage(ctx, session, source, r.Path)
			if fetchErr != nil {
				// Fallback to snippet on error.
				item.Snippet = r.Snippet
			} else {
				item.Body = page.Body
				item.Links = page.Links
			}
		}
		// "link" detail level: source + path + title only (nothing extra).

		items = append(items, item)
	}

	return ActionResult{Data: items}, nil
}

func (c *SearchCapability) handleRead(ctx context.Context, params map[string]any) (ActionResult, error) {
	source, _ := params["source"].(string)
	if source == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: source")
	}
	path, _ := params["path"].(string)
	// path is optional — omit to browse the main page

	session, err := c.getSession()
	if err != nil {
		return ActionResult{}, err
	}

	page, err := c.fetchPage(ctx, session, source, path)
	if err != nil {
		return ActionResult{}, err
	}

	item := searchResultItem{
		Source: source,
		Path:   path,
		Title:  page.Title,
		Body:   page.Body,
		Links:  page.Links,
	}
	return ActionResult{Data: item}, nil
}

func getString(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return value
}

func newSearchResultItem(result search.Result) searchResultItem {
	return searchResultItem{
		Source:   normalizeSourceName(result.Filename),
		Path:     result.Path,
		Title:    result.Title,
		ReadHint: "Use search_read with this exact source and path",
	}
}

func normalizeSourceName(source string) string {
	return strings.TrimSuffix(source, ".zim")
}

func (c *SearchCapability) fetchPage(ctx context.Context, session *search.Session, source, path string) (search.Page, error) {
	if err := session.EnsureKiwix(ctx); err != nil {
		return search.Page{}, fmt.Errorf("kiwix-serve unavailable: %w", err)
	}
	port := session.KiwixPort()

	// Source and path are used as-is — they come from search.db or the AI
	// passing back values from vault_sources/search results. Kiwix expects
	// the raw path (e.g. "A/Article_Name"), not URL-encoded.
	pageURL := fmt.Sprintf("http://127.0.0.1:%d/content/%s/%s", port, source, path)

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
