package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"html"
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
			Desc: "Search the drive's archives by keyword. Use two to four specific words (\"paracetamol dose\", not a sentence). Articles with every word come first; if there are few, articles with most of the words follow, marked with the words they matched. Pass a result's source and path to search_read. vault_sources lists the archives. Not for SQLite data: use query_sql.",
			Params: []ParamDef{
				{Name: "query", Type: "string", Required: true, Desc: "Keywords, for example: nmap port scan"},
				{Name: "source", Type: "string", Desc: "Restrict to one archive, by its name from vault_sources"},
				{Name: "detail", Type: "string", Desc: "link: titles only. snippet: a matching line. full: the opening of up to 5 articles", Default: "snippet", Enum: []string{"link", "snippet", "full"}},
				{Name: "limit", Type: "integer", Desc: "Maximum results, 1 to 50", Default: defaultSearchLimit},
			},
		},
		{
			Name: "read",
			Desc: "Read an article from an archive. Long articles come in parts: when the result has \"continues\", call again with the offset it names. Omit path for the archive's main page. Paths come from search results and from the links of a page already read.",
			Params: []ParamDef{
				{Name: "source", Type: "string", Required: true, Desc: "Archive name, as in search results"},
				{Name: "path", Type: "string", Desc: "Article path, as in search results. Omit for the main page."},
				{Name: "offset", Type: "integer", Desc: "Character to start from, for the next part of a long article", Default: 0},
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

const (
	defaultSearchLimit = 8

	// fullDetailChars is how much of each article a full-detail search
	// returns. Five of them stay below a third of a 16K context; search_read
	// has the rest.
	fullDetailChars = 1500
)

// searchResultItem is the JSON shape returned for search results.
type searchResultItem struct {
	Source  string `json:"source"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Snippet string `json:"snippet,omitempty"`
	Body    string `json:"body,omitempty"`
	// Matched lists the query words a partial match contains.
	Matched []string `json:"matched,omitempty"`
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

	detail := "snippet"
	if d, ok := params["detail"].(string); ok && d != "" {
		detail = d
	}

	limit := defaultSearchLimit
	if l, ok := intParam(params, "limit"); ok {
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
	// source is the indexed filename; sourceFilter is the name tools use.
	source, sourceFilter := "", ""
	if requested := getString(params, "source"); requested != "" {
		if source, err = c.resolveSource(ctx, requested, false); err != nil {
			return ActionResult{}, err
		}
		sourceFilter = sourceName(source)
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
	var advisory []string
	// A drive indexed for keywords only has nothing to fall back from.
	if !semanticComplete && semanticRecorded(db) {
		advisory = append(advisory, "Hybrid unavailable, fell back to keyword")
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
			advisory = append(advisory, "Hybrid unavailable, fell back to keyword")
		}
	}
	if err := ctx.Err(); err != nil {
		return ActionResult{}, err
	}

	if hybridUsed {
		if sourceFilter != "" {
			filtered := make([]engine.Result, 0, len(results))
			for _, r := range results {
				if sourceName(r.Filename) == sourceFilter {
					filtered = append(filtered, r)
				}
			}
			results = filtered
		}
		if len(results) > limit {
			results = results[:limit]
		}
	} else {
		// Fallback to keyword search.
		matches, err := eng.Search(ctx, query, engine.KeywordOptions{Source: source, Limit: limit})
		if err != nil {
			return ActionResult{}, fmt.Errorf("search failed: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return ActionResult{}, err
		}
		results = matches.Results
		scope := "on the drive"
		if sourceFilter != "" {
			scope = "in " + sourceFilter
		}
		if note := keywordAdvisory(matches, scope); note != "" {
			advisory = append(advisory, note)
		}
	}

	items := make([]searchResultItem, 0, len(results))
	for _, r := range results {
		item := searchResultItem{
			Source:  sourceName(r.Filename),
			Path:    r.Path,
			Title:   r.Title,
			Matched: r.Matched,
		}

		// Prepend chunk header for section context (e.g., "Water Purification > Solar disinfection")
		if r.ChunkHeader != "" && r.ChunkHeader != r.Title {
			item.Title = r.ChunkHeader
		}

		switch detail {
		case "snippet":
			item.Snippet = html.UnescapeString(r.Snippet)
			// If FTS snippet is empty, try article body from DB
			if !hybridUsed && item.Snippet == "" {
				if body, err := eng.ReadArticle(r.Filename, r.Path); err == nil && body != "" {
					item.Snippet = opening(body, 200)
				}
			}
		case "full":
			// Try reading from search.db first (fast, no kiwix needed)
			if hybridUsed {
				// HybridSnapshot captured this body in its verified read snapshot.
				item.Body = opening(r.Body, fullDetailChars)
			} else if r.Body != "" {
				item.Body = opening(r.Body, fullDetailChars)
			} else if body, err := eng.ReadArticle(r.Filename, r.Path); err == nil && body != "" {
				item.Body = opening(body, fullDetailChars)
			} else {
				// Fallback to kiwix for full HTML content
				if page, _, fetchErr := c.fetchPage(ctx, sourceName(r.Filename), strings.TrimLeft(r.Path, "/")); fetchErr == nil {
					item.Body = opening(page.Body, fullDetailChars)
				} else {
					item.Snippet = html.UnescapeString(r.Snippet) // last resort
				}
			}
		}

		items = append(items, item)
	}

	return ActionResult{Data: items, Text: strings.Join(advisory, ". ")}, nil
}

// keywordAdvisory says, for the model reading the results, how far they are
// from what was asked. It states what the index knows and leaves the next
// step to the model: told to "try other words", a small model keeps trying.
func keywordAdvisory(matches engine.KeywordMatches, scope string) string {
	absent := ""
	if len(matches.Missing) > 0 {
		absent = "No article " + scope + " mentions: " + strings.Join(matches.Missing, ", ") + "."
	}
	partial := len(matches.Results) - matches.Exact
	switch {
	case len(matches.Results) == 0 && absent != "":
		return "Nothing found. " + absent
	case len(matches.Results) == 0:
		return "Nothing found: no article " + scope + " has these words together."
	case matches.Exact == 0:
		return strings.TrimSpace("No article has all of the words. " + absent + " These have most of them; \"matched\" lists which.")
	case partial > 0:
		return strings.TrimSpace(fmt.Sprintf("The first %d have every word. The rest have only the words in \"matched\". ", matches.Exact) + absent)
	}
	return ""
}

// semanticRecorded reports whether the index was ever built for hybrid
// search, complete or not.
func semanticRecorded(db *sql.DB) bool {
	var state string
	return db.QueryRow("SELECT value FROM meta WHERE key='semantic_state'").Scan(&state) == nil
}

// opening returns the start of an indexed text, cut at a character boundary.
func opening(text string, limit int) string {
	text = html.UnescapeString(text)
	if runes := []rune(text); len(runes) > limit {
		return strings.TrimSpace(string(runes[:limit])) + "..."
	}
	return text
}

func getString(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return value
}

// kiwixBasePort is where the search for a free kiwix-serve port starts. It is
// a variable so this package's tests can stay clear of other packages' tests,
// which run at the same time and start their own stand-in servers from 8080.
var kiwixBasePort = 8080

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

		port, err := netutil.FindAvailablePort("127.0.0.1", kiwixBasePort)
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
