package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
func (c *SearchCapability) Description() string { return "Search and read offline archives" }

func (c *SearchCapability) Actions() []ActionDef {
	return []ActionDef{
		{
			Name: "keyword",
			Desc: "Full-text keyword search across all ZIM archives",
			Params: []ParamDef{
				{Name: "query", Type: "string", Required: true, Desc: "Search query"},
				{Name: "detail", Type: "string", Desc: "Detail level: link, snippet, full", Default: "snippet", Enum: []string{"link", "snippet", "full"}},
				{Name: "limit", Type: "integer", Desc: "Max results (1-50, default 10)", Default: 10},
			},
		},
		{
			Name: "semantic",
			Desc: "Semantic similarity search using embeddings",
			Params: []ParamDef{
				{Name: "query", Type: "string", Required: true, Desc: "Search query"},
				{Name: "detail", Type: "string", Desc: "Detail level: link, snippet, full", Default: "snippet", Enum: []string{"link", "snippet", "full"}},
				{Name: "limit", Type: "integer", Desc: "Max results (1-50, default 10)", Default: 10},
			},
		},
		{
			Name: "read",
			Desc: "Read the full text of a specific article from a ZIM archive",
			Params: []ParamDef{
				{Name: "source", Type: "string", Required: true, Desc: "ZIM source name (without .zim extension)"},
				{Name: "path", Type: "string", Required: true, Desc: "Article path within the ZIM"},
			},
		},
	}
}

func (c *SearchCapability) Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error) {
	switch action {
	case "keyword":
		return c.handleSearch(ctx, search.ModeKeyword, params)
	case "semantic":
		return c.handleSearch(ctx, search.ModeSemantic, params)
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
	Source  string            `json:"source"`
	Path   string            `json:"path"`
	Title  string            `json:"title"`
	Snippet string           `json:"snippet,omitempty"`
	Body   string            `json:"body,omitempty"`
	Links  []search.PageLink `json:"links,omitempty"`
}

func (c *SearchCapability) getSession() (*search.Session, error) {
	c.sessionOnce.Do(func() {
		c.session, c.sessionErr = search.NewSession(c.driveRoot, nil)
	})
	return c.session, c.sessionErr
}

func (c *SearchCapability) handleSearch(ctx context.Context, mode search.Mode, params map[string]any) (ActionResult, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: query")
	}

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

	resp, err := session.Search(ctx, mode, query, limit)
	if err != nil {
		return ActionResult{}, fmt.Errorf("search failed: %w", err)
	}

	items := make([]searchResultItem, 0, len(resp.Results))
	for _, r := range resp.Results {
		source := strings.TrimSuffix(r.Filename, ".zim")
		item := searchResultItem{
			Source: source,
			Path:   r.Path,
			Title:  r.Title,
		}

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
	if path == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: path")
	}

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

func (c *SearchCapability) fetchPage(ctx context.Context, session *search.Session, source, path string) (search.Page, error) {
	if err := session.EnsureKiwix(ctx); err != nil {
		return search.Page{}, fmt.Errorf("kiwix-serve unavailable: %w", err)
	}
	port := session.KiwixPort()

	pageURL := fmt.Sprintf("http://127.0.0.1:%d/content/%s/%s",
		port,
		url.PathEscape(source),
		url.PathEscape(path),
	)

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
