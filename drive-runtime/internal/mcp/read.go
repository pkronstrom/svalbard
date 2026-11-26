package mcp

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

const (
	// readChars is one piece of an article: about 1,500 tokens, so a model
	// with a 16K context can read several articles in one conversation.
	readChars = 6000

	// readLinks caps the links listed with the first piece.
	readLinks = 20

	// nearPaths is how many existing paths a failed read names.
	nearPaths = 5
)

// errPageNotFound is kiwix-serve answering 404, as opposed to not answering.
var errPageNotFound = errors.New("page not found")

// readResult is the JSON shape of a read.
type readResult struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	// Continues tells the caller how to get the next piece.
	Continues string            `json:"continues,omitempty"`
	Note      string            `json:"note,omitempty"`
	Links     []search.PageLink `json:"links,omitempty"`
	// Page and File are set for a document: the page number, and where the
	// page is on the drive ("books/manuals/volvo.pdf#page=12").
	Page int    `json:"page,omitempty"`
	File string `json:"file,omitempty"`
}

func (c *SearchCapability) handleRead(ctx context.Context, params map[string]any) (ActionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	requested := getString(params, "source")
	if requested == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: source")
	}
	source, err := c.resolveSource(ctx, requested, true)
	if err != nil {
		return ActionResult{}, err
	}
	offset := intParam(params, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	rawPath := getString(params, "path")
	if engine.IsDocumentSource(source) {
		return c.readDocument(ctx, source, rawPath, offset)
	}
	candidates := pathCandidates(rawPath)

	page, pagePath, err := c.readFullPage(ctx, source, candidates)
	note := ""
	if err != nil && rawPath != "" {
		// The index knows which paths exist, so it can correct a near miss
		// and, failing that, say what is nearby.
		match, near := c.nearPaths(ctx, source, candidates)
		if match.Path != "" {
			if full, fullPath, fullErr := c.readFullPage(ctx, source, []string{strings.TrimLeft(match.Path, "/")}); fullErr == nil {
				page, pagePath, err = full, fullPath, nil
			} else if body, readErr := c.indexedBody(source, match.Path); readErr == nil && body != "" {
				page, pagePath, err = search.Page{Title: match.Title, Body: html.UnescapeString(body)}, strings.TrimLeft(match.Path, "/"), nil
				note = "The full page could not be loaded. This is the indexed extract, which holds only the start of a long article, and it has no links."
			}
		}
		if err != nil {
			return ActionResult{}, notFoundError(sourceName(source), rawPath, near, errors.Is(err, errPageNotFound))
		}
	}
	if err != nil {
		return ActionResult{}, fmt.Errorf("cannot open %s: %w", sourceName(source), err)
	}

	body, next, total := pageSlice(page.Body, offset, readChars)
	if offset > 0 && offset >= total {
		return ActionResult{}, fmt.Errorf("offset %d is past the end of the article (%d characters)", offset, total)
	}
	result := readResult{
		Source: sourceName(source),
		Path:   "/" + pagePath,
		Title:  page.Title,
		Body:   body,
		Note:   note,
	}
	if next < total {
		result.Continues = fmt.Sprintf("Characters %d to %d of %d. Call search_read with the same source and path and offset %d for the next part.", offset, next, total, next)
	}
	if offset == 0 {
		result.Links = page.Links
		if extra := len(page.Links) - readLinks; extra > 0 {
			result.Links = page.Links[:readLinks]
			result.Note = strings.TrimSpace(result.Note + fmt.Sprintf(" %d more links are not listed; use search to find a page by name.", extra))
		}
		for i := range result.Links {
			result.Links[i].Path = "/" + result.Links[i].Path
		}
	}
	return ActionResult{Data: result}, nil
}

// readFullPage loads the first candidate path the source has, as text. Only
// ZIM archives are served by kiwix-serve; any other kind of source has no
// full-page reader here and is read from the index by the caller.
func (c *SearchCapability) readFullPage(ctx context.Context, source string, candidates []string) (search.Page, string, error) {
	if !strings.EqualFold(filepath.Ext(source), ".zim") {
		return search.Page{}, "", fmt.Errorf("%s has no page reader", source)
	}
	var lastErr error
	for _, candidate := range candidates {
		page, pagePath, err := c.fetchPage(ctx, sourceName(source), candidate)
		if err == nil {
			return page, pagePath, nil
		}
		lastErr = err
		if !errors.Is(err, errPageNotFound) {
			break
		}
	}
	return search.Page{}, "", lastErr
}

// fetchPage reads one page from kiwix-serve. path is relative to the archive
// root; an empty path is the archive's main page. It returns the path the
// page was finally served from, which differs after a redirect.
func (c *SearchCapability) fetchPage(ctx context.Context, book, path string) (search.Page, string, error) {
	if err := c.ensureKiwix(); err != nil {
		return search.Page{}, "", fmt.Errorf("kiwix-serve unavailable: %w", err)
	}
	// Escaped per segment: an indexed path may hold spaces, "?" or "%".
	prefix := "/content/" + book + "/"
	pageURL := url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", c.kiwixPort), Path: prefix + path}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return search.Page{}, "", err
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return search.Page{}, "", fmt.Errorf("fetching page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return search.Page{}, "", errPageNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return search.Page{}, "", fmt.Errorf("kiwix returned status %d for %s/%s", resp.StatusCode, book, path)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return search.Page{}, "", fmt.Errorf("reading page body: %w", err)
	}
	served := strings.TrimPrefix(resp.Request.URL.Path, prefix)
	page, err := search.ExtractPage(string(raw), served)
	page.Body = strings.TrimPrefix(page.Body, "\ufeff") // a byte order mark some archived pages start with
	return page, served, err
}

// resolveSource turns the name a caller gave into a source filename. The name
// may carry the file extension or not, differ in case, or be an unambiguous
// part of the filename; a document may be named with or without "books/".
// onDisk also accepts archives that are not indexed.
func (c *SearchCapability) resolveSource(ctx context.Context, name string, onDisk bool) (string, error) {
	scope, err := c.resolveScope(ctx, name, onDisk)
	if err != nil {
		return "", err
	}
	if folder, isFolder := strings.CutSuffix(scope, "/"); isFolder {
		var documents []string
		for _, filename := range c.knownSources(ctx, false) {
			if engine.InScope(filename, scope) {
				documents = append(documents, filename)
			}
		}
		if len(documents) == 1 {
			return documents[0], nil
		}
		listed := documents[:min(len(documents), holdingsListed)]
		more := ""
		if extra := len(documents) - len(listed); extra > 0 {
			more = fmt.Sprintf(", and %d more", extra)
		}
		return "", fmt.Errorf("%s is a folder of %s; pass one of them as source: %s%s. To find a page, search with source %s.",
			folder, countDocuments(len(documents)), strings.Join(listed, ", "), more, folder)
	}
	return scope, nil
}

// resolveScope is resolveSource for a search, which may also be limited to a
// folder of documents: the scope returned for one ends in "/".
func (c *SearchCapability) resolveScope(ctx context.Context, name string, onDisk bool) (string, error) {
	known := c.knownSources(ctx, onDisk)
	given := strings.ToLower(strings.TrimSpace(name))
	want := strings.ToLower(sourceName(strings.TrimSpace(name)))
	var partial []string
	for _, filename := range known {
		if engine.IsDocumentSource(filename) {
			forms := documentNames(filename)
			if slices.Contains(forms, given) {
				return filename, nil
			}
			if strings.Contains(forms[0], given) {
				partial = append(partial, filename)
			}
			continue
		}
		have := strings.ToLower(sourceName(filename))
		if have == want {
			return filename, nil
		}
		if strings.Contains(have, want) {
			partial = append(partial, filename)
		}
	}
	if folder, count := documentFolder(name, known); count > 0 {
		return folder, nil
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	if len(known) == 0 {
		return "", fmt.Errorf("unknown source %q: the drive has no archives or documents", name)
	}
	return "", fmt.Errorf("unknown source %q. Sources on this drive: %s", name, strings.Join(sourceNames(known), ", "))
}

// knownSources lists the indexed sources and, with onDisk, the archives in
// zim/ that are not indexed.
func (c *SearchCapability) knownSources(ctx context.Context, onDisk bool) []string {
	var known []string
	listed := map[string]bool{}
	add := func(filename string) {
		if !listed[filename] {
			listed[filename] = true
			known = append(known, filename)
		}
	}
	if _, eng, err := c.getDB(); err == nil {
		indexed, _ := eng.SourceFilenames(ctx)
		for _, filename := range indexed {
			add(filename)
		}
	}
	if onDisk {
		zims, _ := filepath.Glob(filepath.Join(c.driveRoot, "zim", "*.zim"))
		for _, zim := range zims {
			add(filepath.Base(zim))
		}
	}
	return known
}

// sourceName is a source as tools name it: an archive's filename without
// extension, a document's path on the drive as it is.
func sourceName(filename string) string {
	if engine.IsDocumentSource(filename) {
		return filename
	}
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

var urlScheme = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// pathCandidates lists the archive paths a caller may have meant, most
// literal first and without a leading slash. Models hand back paths with the
// slashes changed, URL-encoded, with spaces for underscores, or as the page's
// original web address.
func pathCandidates(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{""}
	}
	forms := []string{raw}
	add := func(form string) {
		if form != "" {
			forms = append(forms, form)
		}
	}
	if decoded, err := url.PathUnescape(raw); err == nil {
		add(decoded)
	}
	for _, form := range forms {
		// A web archive stores a page under its address without the scheme.
		add(urlScheme.ReplaceAllString(form, ""))
	}
	for _, form := range forms {
		if cut := strings.IndexAny(form, "#?"); cut > 0 {
			add(form[:cut])
		}
	}
	for _, form := range forms {
		add(strings.ReplaceAll(form, " ", "_"))
	}

	seen := map[string]bool{}
	var candidates []string
	for _, form := range forms {
		form = strings.TrimLeft(form, "/")
		for _, candidate := range []string{form, toggleTrailingSlash(form)} {
			if candidate != "" && !seen[candidate] {
				seen[candidate] = true
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates
}

func toggleTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return strings.TrimRight(path, "/")
	}
	return path + "/"
}

// nearPaths asks the index for the article a failed read meant and for
// existing paths that resemble it.
func (c *SearchCapability) nearPaths(ctx context.Context, source string, candidates []string) (engine.IndexedPath, []engine.IndexedPath) {
	_, eng, err := c.getDB()
	if err != nil {
		return engine.IndexedPath{}, nil
	}
	segment := lastSegment(candidates[0])
	match, near, err := eng.NearPaths(ctx, source, candidates, segment, nearPaths)
	if err != nil {
		return engine.IndexedPath{}, nil
	}
	if len(near) < nearPaths {
		// Nothing spelled alike: offer what a search for its words finds.
		words := strings.Join(strings.FieldsFunc(segment, func(r rune) bool { return r == '_' || r == '-' || r == '.' || r == ' ' }), " ")
		found, _ := eng.Search(ctx, words, engine.KeywordOptions{Source: source, Limit: nearPaths})
		for _, result := range found.Results {
			if len(near) == nearPaths {
				break
			}
			duplicate := false
			for _, have := range near {
				duplicate = duplicate || have.Path == result.Path
			}
			if !duplicate {
				near = append(near, engine.IndexedPath{Path: result.Path, Title: result.Title})
			}
		}
	}
	return match, near
}

func lastSegment(path string) string {
	path = strings.TrimRight(path, "/")
	if cut := strings.LastIndex(path, "/"); cut >= 0 {
		path = path[cut+1:]
	}
	return path
}

func (c *SearchCapability) indexedBody(source, path string) (string, error) {
	_, eng, err := c.getDB()
	if err != nil {
		return "", err
	}
	return eng.ReadArticle(source, path)
}

func notFoundError(source, path string, near []engine.IndexedPath, served bool) error {
	message := fmt.Sprintf("article not found: %s has no %q.", source, path)
	if !served {
		message = fmt.Sprintf("article not readable: %s could not be opened and its index has no %q.", source, path)
	}
	if len(near) == 0 {
		return errors.New(message + " Use search to find the path.")
	}
	paths := make([]string, len(near))
	for i, found := range near {
		paths[i] = found.Path
	}
	return errors.New(message + " Nearest paths: " + strings.Join(paths, ", "))
}

// pageSlice returns the piece of body that starts at character offset and is
// about size characters long, ending at a line or word break when one is
// near, with the offset of the next piece and the body's length.
func pageSlice(body string, offset, size int) (piece string, next, total int) {
	runes := []rune(body)
	total = len(runes)
	if offset >= total {
		return "", total, total
	}
	next = min(offset+size, total)
	if next < total {
		for _, separator := range []rune{'\n', ' '} {
			cut := next
			for cut > next-size/5 && runes[cut-1] != separator {
				cut--
			}
			if cut > next-size/5 {
				next = cut
				break
			}
		}
	}
	return strings.TrimSpace(string(runes[offset:next])), next, total
}
