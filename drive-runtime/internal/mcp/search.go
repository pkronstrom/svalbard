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
// Uses kiwix-serve for native full-text search and for reading archive pages.
// Document pages and fallback archive search use SQLite.
type SearchCapability struct {
	driveRoot string
	meta      DriveMetadata

	dbOnce sync.Once
	db     *sql.DB
	dbErr  error
	eng    *engine.Engine

	embedBackend *search.SemanticBackend

	// read is the size of one search_read.
	read ReadLimits

	kiwixMu sync.Mutex
	kiwix   map[string]kiwixArchive
}

type kiwixArchive struct {
	cmd  *exec.Cmd
	port int
}

// NewSearchCapability creates a search capability for the given drive.
func NewSearchCapability(driveRoot string, meta DriveMetadata) *SearchCapability {
	return &SearchCapability{driveRoot: driveRoot, meta: meta, embedBackend: search.NewSemanticBackend(driveRoot), read: ReadLimitsFor(0)}
}

func (c *SearchCapability) Tool() string { return "search" }

func (c *SearchCapability) Actions() []ActionDef {
	return []ActionDef{
		{
			Name: "search",
			Desc: "Search the drive's archives and documents by keyword. Use a specific subject such as \"growing potatoes\" and start with about five results. Read a promising result before broadening the search. Pass its source and path to search_read or search_link. Use vault_sources for source names and query_sql for SQLite data.",
			Params: []ParamDef{
				{Name: "query", Type: "string", Required: true, Desc: "Keywords, for example: nmap port scan"},
				{Name: "source", Type: "string", Desc: "Restrict to one archive or document, by its name from vault_sources, or to a folder of documents such as books/car-manuals"},
				{Name: "detail", Type: "string", Desc: "link: titles only. snippet: a matching line. full: the opening of up to 5 articles", Default: "snippet", Enum: []string{"link", "snippet", "full"}},
				{Name: "limit", Type: "integer", Desc: "Maximum results, 1 to 50", Default: defaultSearchLimit},
			},
		},
		{
			Name: "link",
			Desc: "Serve a ZIM article on this computer and get a clickable browser URL. Pass the source and path from search or read.",
			Params: []ParamDef{
				{Name: "source", Type: "string", Required: true, Desc: "ZIM source name from search or read"},
				{Name: "path", Type: "string", Required: true, Desc: "Article path from search or read"},
			},
		},
		{
			Name: "read",
			Desc: "Read an archive article or a document page. When a result includes continues, use its offset with that same source and path. Omit path for the archive's main page or document overview. Get article paths from search results or a page's links. Document paths look like page/1.",
			Params: []ParamDef{
				{Name: "source", Type: "string", Required: true, Desc: "Archive or document name, as in search results"},
				{Name: "path", Type: "string", Desc: "Article path, as in search results; page/N for a document. Omit for the main page."},
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
	case "link":
		return c.handleArchiveLink(ctx, params)
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
	c.kiwixMu.Lock()
	defer c.kiwixMu.Unlock()
	for _, archive := range c.kiwix {
		if cmd := archive.cmd; cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}
	clear(c.kiwix)
	return nil
}

const (
	defaultSearchLimit = 8

	// fullDetailChars is how much of each article a full-detail search
	// returns. Five of them stay below a third of a 16K context; search_read
	// has the rest. smallFullDetailChars is the opening below the Balanced
	// tier, where five full openings (8,300 characters on WikEM) made pi
	// compact after each search: five of them are about one shorter read.
	fullDetailChars      = 1500
	smallFullDetailChars = 600
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
	// Page and File are set for a hit in a document: the page number, and
	// where the page is on the drive ("books/manuals/volvo.pdf#page=12").
	Page int    `json:"page,omitempty"`
	File string `json:"file,omitempty"`
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

	limit := intParam(params, "limit", defaultSearchLimit)
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	if detail == "full" && limit > 5 {
		limit = 5
	}

	// source is the scope searched: an indexed filename, a folder ending in
	// "/", or empty for every knowledge source (the skills are searched only
	// when the scope names them). sourceFilter is the name tools know it by.
	source, sourceFilter := "", ""
	if requested := getString(params, "source"); requested != "" {
		var err error
		if source, err = c.resolveScope(ctx, requested, false); err != nil {
			archive, lookupErr := c.resolveScope(ctx, requested, true)
			if lookupErr != nil {
				return ActionResult{}, lookupErr
			}
			if !strings.EqualFold(filepath.Ext(archive), ".zim") {
				return ActionResult{}, err
			}
			return c.searchArchive(ctx, sourceName(archive), query, detail, limit)
		}
		sourceFilter = strings.TrimSuffix(sourceName(source), "/")
	}
	db, eng, err := c.getDB()
	if err != nil {
		return ActionResult{}, err
	}

	// Building, legacy, and malformed semantic contracts remain keyword-only.
	contract, semanticComplete := search.LoadCompleteContract(db)
	// Try hybrid search if a completed semantic contract is available.
	var results []engine.Result
	var matches engine.KeywordMatches
	hybridUsed := false
	var advisory []string
	// A drive indexed for keywords only has nothing to fall back from.
	if !semanticComplete && semanticRecorded(db) {
		advisory = append(advisory, "Hybrid unavailable, fell back to keyword.")
	}
	if semanticComplete {
		queryVec, embedErr := c.embedBackend.Embed(ctx, contract, query)
		if embedErr == nil {
			var usable bool
			if detail == "full" {
				results, usable, embedErr = search.HybridSnapshotFull(ctx, db, contract, query, queryVec, limit, source)
			} else {
				results, usable, embedErr = search.HybridSnapshot(ctx, db, contract, query, queryVec, limit, source)
			}
			if !usable {
				results = nil
			} else {
				hybridUsed = true
			}
		}
		if embedErr != nil || results == nil {
			advisory = append(advisory, "Hybrid unavailable, fell back to keyword.")
		}
	}
	if err := ctx.Err(); err != nil {
		return ActionResult{}, err
	}

	if hybridUsed {
		// The engine kept to the scope, in its keyword candidates and in the
		// vectors it scored.
		if len(results) > limit {
			results = results[:limit]
		}
	} else {
		// Fallback to keyword search.
		matches, err = eng.Search(ctx, query, engine.KeywordOptions{Source: source, Limit: limit})
		if err != nil {
			return ActionResult{}, fmt.Errorf("search failed: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return ActionResult{}, err
		}
		results = matches.Results
	}

	nativePort := func(ctx context.Context) (int, error) {
		key := ""
		if strings.EqualFold(filepath.Ext(source), ".zim") {
			key = sourceName(source)
		}
		return c.startKiwix(ctx, key)
	}
	results, native, nativeErr := search.MergeZIMKeywords(ctx, eng, nativePort, query, source, limit, results)
	if err := ctx.Err(); err != nil {
		return ActionResult{}, err
	}
	if nativeErr != nil {
		advisory = append(advisory, "Some embedded ZIM indexes were unavailable: "+nativeErr.Error()+".")
	}
	if !hybridUsed && !native {
		scope := "on the drive"
		if sourceFilter != "" {
			scope = "in " + sourceFilter
		}
		if note := keywordAdvisory(matches, scope); note != "" {
			advisory = append(advisory, note)
		}
	}
	if len(matches.MorePages) > 0 {
		advisory = append(advisory, "Only the best pages of each document are listed. More pages match in: "+strings.Join(matches.MorePages, ", ")+". Search with one of them as source to see them.")
	}
	if len(results) == 0 {
		if native && nativeErr == nil {
			advisory = append(advisory, "No matches in the searched archives or documents.")
		}
		advisory = append(advisory, c.holdings(ctx))
	}

	items := make([]searchResultItem, 0, len(results))
	for _, r := range results {
		item := searchResultItem{
			Source:  sourceName(r.Filename),
			Path:    r.Path,
			Title:   r.Title,
			Matched: r.Matched,
		}
		if page, ok := engine.PageNumber(r.Path); ok && engine.IsDocumentSource(r.Filename) {
			item.Page, item.File = page, search.DocumentFile(r.Filename, page)
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
			if r.Native {
				if page, _, fetchErr := c.fetchPage(ctx, sourceName(r.Filename), strings.TrimLeft(r.Path, "/")); fetchErr == nil {
					item.Body = articleOpening(page.Body, c.read.Opening)
				} else {
					item.Snippet = r.Snippet
				}
			} else if hybridUsed {
				// HybridSnapshot captured this body in its verified read snapshot.
				item.Body = articleOpening(r.Body, c.read.Opening)
			} else if r.Body != "" {
				item.Body = articleOpening(r.Body, c.read.Opening)
			} else if body, err := eng.ReadArticle(r.Filename, r.Path); err == nil && body != "" {
				item.Body = articleOpening(body, c.read.Opening)
			} else if engine.IsDocumentSource(r.Filename) {
				item.Snippet = html.UnescapeString(r.Snippet) // a document has no page outside the index
			} else {
				// Fallback to kiwix for full HTML content
				if page, _, fetchErr := c.fetchPage(ctx, sourceName(r.Filename), strings.TrimLeft(r.Path, "/")); fetchErr == nil {
					item.Body = articleOpening(page.Body, c.read.Opening)
				} else {
					item.Snippet = html.UnescapeString(r.Snippet) // last resort
				}
			}
		}

		items = append(items, item)
	}

	return ActionResult{Data: items, Text: strings.Join(advisory, " ")}, nil
}

// holdingsListed caps the sources an empty search names.
const holdingsListed = 12

// holdings says what the drive does hold, for a search that found nothing. A
// model that knows the drive is two medical references stops looking for
// tyre pressures sooner than one told only that its words were not found.
func (c *SearchCapability) holdings(ctx context.Context) string {
	_, eng, err := c.getDB()
	if err != nil {
		return ""
	}
	indexed, err := eng.SourceFilenames(ctx)
	filenames := knowledgeSources(indexed)
	if err != nil || len(filenames) == 0 {
		return ""
	}
	// A folder of documents is one holding, described by its collection:
	// fifty manuals must not crowd the archives out of the list.
	type holding struct {
		name      string
		recipe    RecipeMeta
		documents int
	}
	var holdings []*holding
	folders := map[string]*holding{}
	archives := map[string]*holding{}
	for _, filename := range filenames {
		if archive, _, ok := engine.EmbeddedArchive(filename); ok {
			// Counted under its archive, which sorts before it.
			if held := archives[archive]; held != nil {
				held.documents++
			}
			continue
		}
		recipe, _ := c.meta.RecipeForFile(filename)
		if rest, isDocument := strings.CutPrefix(filename, engine.DocumentsDir+"/"); isDocument {
			if cut := strings.Index(rest, "/"); cut > 0 {
				folder := engine.DocumentsDir + "/" + rest[:cut]
				if folders[folder] == nil {
					folders[folder] = &holding{name: folder, recipe: recipe}
					holdings = append(holdings, folders[folder])
				}
				folders[folder].documents++
				continue
			}
		}
		held := &holding{name: sourceName(filename), recipe: recipe}
		archives[filename] = held
		holdings = append(holdings, held)
	}
	listed := make([]string, 0, holdingsListed)
	for _, held := range holdings[:min(len(holdings), holdingsListed)] {
		entry := held.name
		var details []string
		if held.recipe.Description != "" {
			topic, _, _ := strings.Cut(held.recipe.Description, " (")
			details = append(details, opening(topic, 90))
		}
		if held.documents > 0 {
			details = append(details, countDocuments(held.documents))
		}
		if len(details) > 0 {
			entry += " (" + strings.Join(details, "; ") + ")"
		}
		listed = append(listed, entry)
	}
	more := ""
	if extra := len(holdings) - len(listed); extra > 0 {
		more = fmt.Sprintf("; and %d more, listed by vault_sources", extra)
	}
	return "The drive holds: " + strings.Join(listed, "; ") + more + ". A subject outside these is not on the drive."
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

// articleOpening is an article's body in a full-detail result, at most size
// characters. A model takes a bare "..." for the end of the article, so a cut
// body says where the rest is.
func articleOpening(body string, size int) string {
	body = html.UnescapeString(body)
	runes := []rune(body)
	if len(runes) <= size {
		return body
	}
	return strings.TrimSpace(string(runes[:size])) + "... [opening only: search_read has the whole article]"
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

// ensureKiwix opens just the requested archive. Keep each listener for the
// tool session so links already given to the user can still be opened.
func (c *SearchCapability) ensureKiwix(book string) (int, error) {
	return c.startKiwix(context.Background(), book)
}

// startKiwix keeps a shared listener for global native search. A scoped
// search can use it too, or start just one archive when it is the first call.
func (c *SearchCapability) startKiwix(ctx context.Context, book string) (int, error) {
	c.kiwixMu.Lock()
	defer c.kiwixMu.Unlock()
	if archive, ok := c.kiwix[book]; ok {
		return archive.port, nil
	}
	if archive, ok := c.kiwix[""]; ok {
		return archive.port, nil
	}
	kiwixBin, err := binary.Resolve("kiwix-serve", c.driveRoot)
	if err != nil {
		return 0, fmt.Errorf("kiwix-serve not found: %w", err)
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", kiwixBasePort)
	if err != nil {
		return 0, err
	}
	archives := []string{filepath.Join(c.driveRoot, "zim", book+".zim")}
	if book == "" {
		archives, err = searchserver.ZIMFiles(c.driveRoot)
		if err != nil {
			return 0, err
		}
	}
	var languages map[string]string
	if _, eng, dbErr := c.getDB(); dbErr == nil {
		languages, err = eng.NativeZIMLanguages(ctx)
		if err != nil {
			return 0, err
		}
	}
	cmd, err := searchserver.StartKiwixForSearch(ctx, kiwixBin, archives, port, languages)
	if err != nil {
		return 0, err
	}
	if c.kiwix == nil {
		c.kiwix = make(map[string]kiwixArchive)
	}
	c.kiwix[book] = kiwixArchive{cmd: cmd, port: port}
	return port, nil
}
