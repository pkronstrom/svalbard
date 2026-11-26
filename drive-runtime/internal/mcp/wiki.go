package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/wiki"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

const (
	// maxWikiPages caps the pages one call walks. A personal wiki is a few
	// hundred pages; the cap keeps a folder that is not one from stalling a
	// tool call.
	maxWikiPages = 5000
	maxWikiDepth = 12

	maxWikiNameBytes = 240

	defaultWikiHits = 8
	maxWikiHits     = 20
	// wikiExcerpts is how many matching lines one search result quotes, and
	// wikiExcerptRunes how long each may be.
	wikiExcerpts     = 3
	wikiExcerptRunes = 160

	// wikiHoldings caps the page names an empty search lists.
	wikiHoldings = 12
	// wikiLinkers caps the pages a move names as still linking to the old name.
	wikiLinkers = 5

	pageExample = "Projects/Garden plan"
)

// WikiCapability exposes the drive's wiki via the MCP "wiki" tools: list,
// read, search, create, append, edit, move and delete pages. The pages are
// the Markdown files in <drive>/wiki, which SilverBullet watches, so a page
// written here shows up in the browser by itself. Every client of the
// drive's server gets these tools, on a drive that has the wiki installed.
type WikiCapability struct {
	driveRoot string

	// The two folders are opened on first use: listing the server's tools
	// must not create a wiki folder.
	mu    sync.Mutex
	pages *workspace.Workspace
	trash *workspace.Workspace

	// running returns the address of the wiki when its launcher is up, for
	// the link a write returns. now stamps a deleted page's name in the
	// trash. Both are fields so tests can set them.
	running func() string
	now     func() time.Time
}

// NewWikiCapability creates the wiki tools for the drive at driveRoot.
func NewWikiCapability(driveRoot string) *WikiCapability {
	return &WikiCapability{
		driveRoot: driveRoot,
		running:   func() string { return wiki.RunningURL(driveRoot) },
		now:       time.Now,
	}
}

func (c *WikiCapability) Tool() string { return "wiki" }

func (c *WikiCapability) Actions() []ActionDef {
	const pageDesc = `Page name, for example: ` + pageExample + `. No ".md"; "/" puts the page in a folder.`
	return []ActionDef{
		{
			Name: "list",
			Desc: "List the pages of the wiki, the user's own notebook on this drive. Returns each page's name, size in bytes and modified time.",
			Params: []ParamDef{
				{Name: "folder", Type: "string", Desc: "Only list the pages in this folder, for example: Projects. Omit to list the whole wiki."},
				{Name: "depth", Type: "integer", Desc: "How many folder levels to go down, from 1 to 5", Default: defaultListDepth},
			},
		},
		{
			Name: "read",
			Desc: "Read a wiki page. Give only the page: a normal page comes back whole. A long page comes in parts: when the result has continues, call again with the offset it names. Call this before wiki_edit to copy the exact text you want to replace.",
			Params: []ParamDef{
				{Name: "page", Type: "string", Required: true, Desc: pageDesc},
				{Name: "offset", Type: "integer", Desc: "Only for a long page: the character to start from, as continues says", Default: 0},
			},
		},
		{
			Name: "search",
			// Without the second sentence a small model looks for vault
			// articles here and for its own notes in the archives.
			Desc: "Find wiki pages by keyword. This looks only in the user's wiki; the drive's reference archives are searched with search. Every word must be in the page's name or text. Returns the best pages with the lines that match.",
			Params: []ParamDef{
				{Name: "query", Type: "string", Required: true, Desc: "One to three specific words, not a sentence"},
				{Name: "limit", Type: "integer", Desc: "Most pages to return, from 1 to 20", Default: defaultWikiHits},
			},
		},
		{
			Name:   "create",
			Writes: true,
			Desc:   "Create a new wiki page. Fails if the page exists: then use wiki_append or wiki_edit. Returns the page's name and, when the wiki is open in the browser, a Markdown link to it: put that link in your reply exactly as returned.",
			Params: []ParamDef{
				{Name: "page", Type: "string", Required: true, Desc: pageDesc},
				{Name: "content", Type: "string", Required: true, Desc: "The page's text in Markdown. Link to another page with [[Page name]]."},
			},
		},
		{
			Name:   "append",
			Writes: true,
			Desc:   "Add text to the end of a wiki page, after a blank line. Creates the page if it does not exist.",
			Params: []ParamDef{
				{Name: "page", Type: "string", Required: true, Desc: pageDesc},
				{Name: "content", Type: "string", Required: true, Desc: "The Markdown text to add"},
			},
		},
		{
			Name:   "edit",
			Writes: true,
			Desc:   "Change part of a wiki page: replace one exact piece of text with another. old_text must match the page exactly and occur once; call wiki_read first and copy it from there.",
			Params: []ParamDef{
				{Name: "page", Type: "string", Required: true, Desc: pageDesc},
				{Name: "old_text", Type: "string", Required: true, Desc: "Exact text to replace, copied from the page"},
				{Name: "new_text", Type: "string", Required: true, Desc: "Text to put in its place; an empty string deletes old_text"},
			},
		},
		{
			Name:   "move",
			Writes: true,
			Desc:   "Rename a wiki page or move it to another folder. Fails if the new name is taken. Links to the old name in other pages are not changed.",
			Params: []ParamDef{
				{Name: "page", Type: "string", Required: true, Desc: "The page's current name"},
				{Name: "new_name", Type: "string", Required: true, Desc: "The new page name, for example: Archive/Garden plan"},
			},
		},
		{
			Name:   "delete",
			Writes: true,
			Desc:   "Delete a wiki page. The page is moved to a trash folder on the drive, not erased, and the result says where it went.",
			Params: []ParamDef{
				{Name: "page", Type: "string", Required: true, Desc: pageDesc},
			},
		},
	}
}

func (c *WikiCapability) Handle(_ context.Context, action string, params map[string]any) (ActionResult, error) {
	switch action {
	case "list":
		return c.handleList(params)
	case "read":
		return c.handleRead(params)
	case "search":
		return c.handleSearch(params)
	case "create":
		return c.handleCreate(params)
	case "append":
		return c.handleAppend(params)
	case "edit":
		return c.handleEdit(params)
	case "move":
		return c.handleMove(params)
	case "delete":
		return c.handleDelete(params)
	default:
		return ActionResult{}, fmt.Errorf("unknown wiki action: %s", action)
	}
}

func (c *WikiCapability) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var firstErr error
	for _, folder := range []**workspace.Workspace{&c.pages, &c.trash} {
		if *folder == nil {
			continue
		}
		if err := (*folder).Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		*folder = nil
	}
	return firstErr
}

// openPages returns the confined page folder, creating it and its start page
// on a drive that has neither. The launcher seeds the same page; whichever
// touches the wiki first leaves the user a start page.
func (c *WikiCapability) openPages() (*workspace.Workspace, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pages != nil {
		return c.pages, nil
	}
	ws, err := workspace.OpenWith(wiki.Dir(c.driveRoot), workspace.Options{Place: "wiki", Example: pageExample + ".md"})
	if err != nil {
		return nil, fmt.Errorf("the wiki folder cannot be opened: %v", unwrapPath(err))
	}
	// A drive that cannot be written still lets its pages be read.
	_ = wiki.Seed(ws.Dir())
	c.pages = ws
	return ws, nil
}

func (c *WikiCapability) openTrash() (*workspace.Workspace, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.trash != nil {
		return c.trash, nil
	}
	ws, err := workspace.OpenWith(wiki.TrashDir(c.driveRoot), workspace.Options{Place: "wiki trash", Example: pageExample + ".md"})
	if err != nil {
		return nil, fmt.Errorf("the wiki's trash folder cannot be opened, so nothing was deleted: %v", unwrapPath(err))
	}
	c.trash = ws
	return ws, nil
}

// unwrapPath drops the host path an *fs.PathError carries.
func unwrapPath(err error) error {
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		return pathError.Err
	}
	return err
}

// ---------------------------------------------------------------------------
// Page names
// ---------------------------------------------------------------------------

// pageName checks a page name a model supplied and returns it cleaned. The
// rules are the file system's and SilverBullet's own: a page whose name
// SilverBullet cannot address would be written and never shown. The errors
// are written for the model: each says what to send instead.
func pageName(param, raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("missing required parameter: %s, for example: %s", param, pageExample)
	}
	if len(name) > 3 && strings.EqualFold(name[len(name)-3:], ".md") {
		// The file's name is a natural thing to send for a page.
		name = strings.TrimSpace(name[:len(name)-3])
	}
	switch {
	case strings.ContainsRune(name, 0):
		return "", fmt.Errorf("page name contains a NUL byte; use a plain name, for example: %s", pageExample)
	case strings.Contains(name, `\`):
		return "", fmt.Errorf(`page name %q uses "\"; separate folders with "/", for example: %s`, raw, pageExample)
	case strings.HasPrefix(name, "/"):
		if rest := strings.TrimLeft(name, "/"); rest != "" {
			return "", fmt.Errorf(`page name %q starts with "/"; send it without: %s`, raw, rest)
		}
		return "", fmt.Errorf(`page name %q is not a page; use a name, for example: %s`, raw, pageExample)
	case strings.HasPrefix(name, "~"):
		return "", fmt.Errorf("page name %q is a path on the computer; pages are named inside the wiki, for example: %s", raw, pageExample)
	case len(name) > maxWikiNameBytes:
		return "", fmt.Errorf("page name is %d bytes long, over the limit of %d; use a shorter name", len(name), maxWikiNameBytes)
	}
	segments := strings.Split(name, "/")
	for i, segment := range segments {
		switch {
		case segment == "":
			return "", fmt.Errorf(`page name %q has an empty part; write one "/" between folder and page, for example: %s`, raw, pageExample)
		case segment == "..":
			return "", fmt.Errorf(`page name %q contains ".."; name the page from the top of the wiki, for example: %s`, raw, pageExample)
		case strings.HasPrefix(segment, "."):
			return "", fmt.Errorf("page name %q has a part starting with a dot; choose a name without the leading dot", raw)
		case strings.HasPrefix(segment, "^"):
			return "", fmt.Errorf(`page name %q has a part starting with "^"; choose a name without it`, raw)
		case segment != strings.TrimSpace(segment):
			return "", fmt.Errorf("page name %q has a space next to a \"/\"; send it without: %s", raw, trimSegments(segments))
		case strings.HasSuffix(segment, "."):
			return "", fmt.Errorf("page name %q has a part ending in a dot; send it without: %s", raw, strings.TrimRight(segment, "."))
		}
		if bad := strings.IndexFunc(segment, unusableInPageName); bad >= 0 {
			char, _ := utf8.DecodeRuneInString(segment[bad:])
			return "", fmt.Errorf("page name %q contains %q, which a page name cannot hold; send it without, for example: %s",
				raw, string(char), replaceSegment(segments, i, strings.Join(strings.Fields(strings.Map(spaceIfUnusable, segment)), " ")))
		}
		if strings.Contains(segment, "[[") || strings.Contains(segment, "]]") {
			return "", fmt.Errorf(`page name %q contains "[[" or "]]", which mark a link; send the name alone`, raw)
		}
	}
	// SilverBullet reads a name that ends like a file type ("Version 1.2",
	// "notes.txt") as a file, not a page, and would never list this one.
	last := segments[len(segments)-1]
	if dot := strings.LastIndex(last, "."); dot >= 0 && strings.IndexFunc(last[dot+1:], notASCIIAlphanumeric) < 0 {
		return "", fmt.Errorf("page name %q ends in %q, which the wiki reads as a file type; send it without the dot, for example: %s",
			raw, last[dot:], replaceSegment(segments, len(segments)-1, last[:dot]+"-"+last[dot+1:]))
	}
	return strings.Join(segments, "/"), nil
}

// unusableInPageName reports the characters SilverBullet gives a meaning in a
// link (@ # | < > $) and the ones an exFAT stick cannot store.
func unusableInPageName(r rune) bool {
	return unicode.IsControl(r) || strings.ContainsRune(`@#|<>$"*:?`, r)
}

func spaceIfUnusable(r rune) rune {
	if unusableInPageName(r) {
		return ' '
	}
	return r
}

func notASCIIAlphanumeric(r rune) bool {
	return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
}

func trimSegments(segments []string) string {
	trimmed := make([]string, len(segments))
	for i, segment := range segments {
		trimmed[i] = strings.TrimSpace(segment)
	}
	return strings.Join(trimmed, "/")
}

func replaceSegment(segments []string, index int, with string) string {
	replaced := slices.Clone(segments)
	replaced[index] = strings.TrimSpace(with)
	return strings.Join(replaced, "/")
}

// page is one checked page: its name and its file inside the page folder.
type page struct {
	name string
	rel  string
}

// resolvePage checks the page name in params[param] against the name rules
// and the page folder's confinement.
func (c *WikiCapability) resolvePage(ws *workspace.Workspace, params map[string]any, param string) (page, error) {
	name, err := pageName(param, getString(params, param))
	if err != nil {
		return page{}, err
	}
	name = diskSpelling(ws, name)
	rel, err := ws.Resolve(name + ".md")
	if err != nil {
		return page{}, err
	}
	return page{name: name, rel: rel}, nil
}

// diskSpelling returns name with the letter case its folders and its page
// already have on the drive. A stick is exFAT, where "garden" and "Garden"
// are one file: the page is then the one that exists, under the name the
// wiki shows it by, on every file system.
func diskSpelling(ws *workspace.Workspace, name string) string {
	segments := strings.Split(name, "/")
	dir := "."
	for i, segment := range segments {
		want := segment
		if i == len(segments)-1 {
			want += ".md"
		}
		entries, err := ws.ReadDir(dir)
		if err != nil {
			// Nothing exists from here down, so the name is free as typed.
			break
		}
		exact, others := false, []string(nil)
		for _, entry := range entries {
			switch {
			case entry.Name() == want:
				exact = true
			case strings.EqualFold(entry.Name(), want):
				others = append(others, entry.Name())
			}
		}
		if !exact && len(others) == 1 {
			segments[i] = others[0][:len(others[0])-(len(want)-len(segment))]
		}
		dir = path.Join(dir, segments[i])
	}
	return strings.Join(segments, "/")
}

// pageError turns a page folder error into one about the page: the folder
// speaks of files, the model of pages. A page that does not exist gets the
// names nearest to it.
func (c *WikiCapability) pageError(ws *workspace.Workspace, p page, err error) error {
	quoted := fmt.Sprintf("page %q", p.name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		near := nearestPages(p.name, c.pageNames(ws))
		if len(near) == 0 {
			return fmt.Errorf("%s does not exist. Call wiki_list to see the pages, or create it with wiki_create.", quoted)
		}
		return fmt.Errorf("%s does not exist. Nearest pages: %s", quoted, strings.Join(near, ", "))
	case errors.Is(err, workspace.ErrIsFolder):
		return fmt.Errorf("%s is a folder, not a page; list it with wiki_list", quoted)
	}
	return errors.New(strings.ReplaceAll(err.Error(), p.rel, quoted))
}

// readPage loads the text of the page p.
func (c *WikiCapability) readPage(ws *workspace.Workspace, p page) (string, error) {
	data, err := ws.ReadFile(p.rel, maxReadFileBytes)
	if err != nil {
		return "", c.pageError(ws, p, err)
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", fmt.Errorf("page %q is not text (it is binary or not UTF-8); these tools read text only", p.name)
	}
	return string(data), nil
}

func (c *WikiCapability) writePage(ws *workspace.Workspace, p page, content string) error {
	if err := ws.WriteFile(p.rel, []byte(content)); err != nil {
		return c.pageError(ws, p, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Walking the page folder
// ---------------------------------------------------------------------------

// wikiEntry is the JSON shape of one listed page, or of a folder the listing
// did not go into.
type wikiEntry struct {
	Page     string `json:"page,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
}

// walkPages calls visit for every page at or below the folder dir, in name
// order, down to depth folder levels; a folder at the limit goes to folder
// when that is set. visit returns false to stop the walk.
func walkPages(ws *workspace.Workspace, dir string, depth int, visit func(name string, info fs.FileInfo) bool, folder func(name string)) error {
	var walk func(dir string, level int) (bool, error)
	walk = func(dir string, level int) (bool, error) {
		children, err := ws.ReadDir(dir)
		if err != nil {
			return false, err
		}
		slices.SortFunc(children, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
		for _, child := range children {
			if strings.HasPrefix(child.Name(), ".") {
				continue
			}
			childPath := path.Join(dir, child.Name())
			// Stat follows a link that stays inside the folder and fails on
			// one that leaves it, which is then not listed.
			info, err := ws.Stat(childPath)
			switch {
			case err != nil:
			case info.IsDir() && level < depth:
				if more, err := walk(childPath, level+1); err != nil || !more {
					return more, err
				}
			case info.IsDir():
				if folder != nil {
					folder(childPath)
				}
			case info.Mode().IsRegular() && strings.HasSuffix(child.Name(), ".md") && len(child.Name()) > len(".md"):
				if !visit(strings.TrimSuffix(childPath, ".md"), info) {
					return false, nil
				}
			}
		}
		return true, nil
	}
	_, err := walk(dir, 1)
	return err
}

// pageNames lists every page of the wiki, up to maxWikiPages.
func (c *WikiCapability) pageNames(ws *workspace.Workspace) []string {
	var names []string
	_ = walkPages(ws, ".", maxWikiDepth, func(name string, _ fs.FileInfo) bool {
		names = append(names, name)
		return len(names) < maxWikiPages
	}, nil)
	return names
}

// foldPageName reduces a page name to what a near miss still has in common
// with it: letter case, and spaces, underscores and hyphens, are ignored.
func foldPageName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '_' || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, name)
}

// samePage returns the one existing page name means when it is spelled with
// other case or separators, or without its folder. An ambiguous name is none.
func samePage(name string, names []string) (string, bool) {
	want := foldPageName(name)
	for _, match := range []func(have string) bool{
		func(have string) bool { return foldPageName(have) == want },
		func(have string) bool { return foldPageName(path.Base(have)) == want },
	} {
		var found []string
		for _, have := range names {
			if match(have) {
				found = append(found, have)
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
		if len(found) > 1 {
			return "", false
		}
	}
	return "", false
}

// nearestPages names up to nearPaths existing pages that resemble name: the
// ones sharing most of its words, then the ones spelled most like it.
func nearestPages(name string, names []string) []string {
	words := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	type candidate struct {
		name     string
		shared   int
		distance int
	}
	var candidates []candidate
	folded := foldPageName(path.Base(name))
	for _, have := range names {
		lower := strings.ToLower(have)
		shared := 0
		for _, word := range words {
			if len(word) > 2 && strings.Contains(lower, word) {
				shared++
			}
		}
		distance := editDistance(folded, foldPageName(path.Base(have)))
		// A name that shares no word and is mostly different is not near.
		if shared == 0 && distance > max(2, len(folded)/3) {
			continue
		}
		candidates = append(candidates, candidate{have, shared, distance})
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if a.shared != b.shared {
			return b.shared - a.shared
		}
		return a.distance - b.distance
	})
	near := make([]string, 0, nearPaths)
	for _, found := range candidates[:min(len(candidates), nearPaths)] {
		near = append(near, found.name)
	}
	return near
}

// editDistance is the Levenshtein distance between a and b, in runes.
func editDistance(a, b string) int {
	left, right := []rune(a), []rune(b)
	row := make([]int, len(right)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(left); i++ {
		diagonal := row[0]
		row[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			diagonal, row[j] = row[j], min(row[j]+1, row[j-1]+1, diagonal+cost)
		}
	}
	return row[len(right)]
}

// ---------------------------------------------------------------------------
// list, read, search
// ---------------------------------------------------------------------------

func (c *WikiCapability) handleList(params map[string]any) (ActionResult, error) {
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}
	dir := "."
	if folder := strings.Trim(strings.TrimSpace(getString(params, "folder")), "/"); folder != "" && folder != "." {
		if dir, err = ws.Resolve(folder); err != nil {
			return ActionResult{}, err
		}
		info, err := ws.Stat(dir)
		switch {
		case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR), err == nil && !info.IsDir():
			return ActionResult{}, fmt.Errorf("the wiki has no folder %q; call wiki_list without folder to see all pages", folder)
		case err != nil:
			return ActionResult{}, err
		}
	}
	depth := min(max(intParam(params, "depth", defaultListDepth), 1), maxListDepth)

	var entries []wikiEntry
	truncated := false
	var folders []string
	add := func(entry wikiEntry) bool {
		if len(entries) == maxListEntries {
			truncated = true
			return false
		}
		entries = append(entries, entry)
		return true
	}
	err = walkPages(ws, dir, depth, func(name string, info fs.FileInfo) bool {
		return add(wikiEntry{Page: name, Size: info.Size(), Modified: info.ModTime().Format("2006-01-02 15:04")})
	}, func(name string) {
		if add(wikiEntry{Folder: name}) {
			folders = append(folders, name)
		}
	})
	if err != nil {
		return ActionResult{}, err
	}

	switch {
	case len(entries) == 0 && dir == ".":
		// An empty result reads as a failed call to a small model.
		return ActionResult{Text: "The wiki has no pages yet. Create one with wiki_create."}, nil
	case len(entries) == 0:
		return ActionResult{Text: fmt.Sprintf("The folder %s has no pages.", dir)}, nil
	case truncated:
		return ActionResult{Data: entries, Text: fmt.Sprintf("Only the first %d entries are shown. List one folder to see the rest, or find a page with wiki_search.", maxListEntries)}, nil
	case len(folders) > 0:
		return ActionResult{Data: entries, Text: fmt.Sprintf("A folder entry is listed without what is in it. Call wiki_list with folder=%s to see inside.", folders[0])}, nil
	}
	return ActionResult{Data: entries}, nil
}

// wikiPage is the JSON shape of a read.
type wikiPage struct {
	Page string `json:"page"`
	Body string `json:"body"`
	// Continues tells the caller how to get the next piece.
	Continues string `json:"continues,omitempty"`
	Note      string `json:"note,omitempty"`
}

func (c *WikiCapability) handleRead(params map[string]any) (ActionResult, error) {
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}
	p, err := c.resolvePage(ws, params, "page")
	if err != nil {
		return ActionResult{}, err
	}
	note := ""
	if _, statErr := ws.Stat(p.rel); errors.Is(statErr, fs.ErrNotExist) {
		// Reading the page a near miss meant is harmless; the writing tools
		// do not guess.
		if match, ok := samePage(p.name, c.pageNames(ws)); ok {
			note = fmt.Sprintf("There is no page %q; this is %q.", p.name, match)
			p = page{name: match, rel: match + ".md"}
		}
	}
	content, err := c.readPage(ws, p)
	if err != nil {
		return ActionResult{}, err
	}
	if strings.TrimSpace(content) == "" {
		return ActionResult{Text: fmt.Sprintf("[page %q is empty]", p.name)}, nil
	}

	offset := max(intParam(params, "offset", 0), 0)
	body, next, total := pageSlice(content, offset, readChars)
	if offset > 0 && offset >= total {
		return ActionResult{}, fmt.Errorf("offset %d is past the end of page %q (%d characters)", offset, p.name, total)
	}
	result := wikiPage{Page: p.name, Body: body, Note: note}
	if next < total {
		result.Continues = fmt.Sprintf("Characters %d to %d of %d. Call wiki_read with the same page and offset %d for the next part.", offset, next, total, next)
	}
	return ActionResult{Data: result}, nil
}

// wikiHit is the JSON shape of one search result.
type wikiHit struct {
	Page string `json:"page"`
	// Matches counts the query's words in the page's name and text.
	Matches  int      `json:"matches"`
	Excerpts []string `json:"excerpts,omitempty"`
}

func (c *WikiCapability) handleSearch(params map[string]any) (ActionResult, error) {
	terms := searchTerms(getString(params, "query"))
	if len(terms) == 0 {
		return ActionResult{}, fmt.Errorf("missing required parameter: query (one to three words to look for)")
	}
	limit := min(max(intParam(params, "limit", defaultWikiHits), 1), maxWikiHits)
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}

	type scored struct {
		hit   wikiHit
		score int
	}
	var hits []scored
	names := c.pageNames(ws)
	// seen records which words occur anywhere, so an empty result can name
	// the ones that occur nowhere.
	seen := make([]bool, len(terms))
	for _, name := range names {
		data, err := ws.ReadFile(name+".md", maxReadFileBytes)
		if err != nil || bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
			data = nil
		}
		lowerName, lowerText := strings.ToLower(name), strings.ToLower(string(data))
		matches, score, all := 0, 0, true
		for i, term := range terms {
			inName, inText := strings.Count(lowerName, term), strings.Count(lowerText, term)
			if inName+inText == 0 {
				all = false
				continue
			}
			seen[i] = true
			matches += inName + inText
			// A word in the name counts for more than a page that repeats
			// it, and one page full of a word must not bury the rest.
			score += 10*min(inName, 1) + min(inText, 10)
		}
		if all {
			hits = append(hits, scored{wikiHit{Page: name, Matches: matches, Excerpts: excerpts(string(data), terms)}, score})
		}
	}

	if len(hits) == 0 {
		return ActionResult{Text: emptyWikiSearch(terms, seen, names)}, nil
	}
	slices.SortStableFunc(hits, func(a, b scored) int { return b.score - a.score })
	shown := make([]wikiHit, 0, limit)
	for _, found := range hits[:min(len(hits), limit)] {
		shown = append(shown, found.hit)
	}
	if extra := len(hits) - len(shown); extra > 0 {
		return ActionResult{Data: shown, Text: fmt.Sprintf("These are the best %d of %d matching pages. Add a word to narrow the search.", len(shown), len(hits))}, nil
	}
	return ActionResult{Data: shown}, nil
}

// searchTerms splits a query into lower-case words without the punctuation
// around them, each once.
func searchTerms(query string) []string {
	var terms []string
	for _, word := range strings.Fields(strings.ToLower(query)) {
		word = strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if word != "" && !slices.Contains(terms, word) {
			terms = append(terms, word)
		}
	}
	return terms
}

// excerpts returns the lines of text that hold most of the terms, in page
// order, each cut to wikiExcerptRunes around its first match.
func excerpts(text string, terms []string) []string {
	type candidate struct {
		index int
		found int
		line  string
	}
	var candidates []candidate
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		found := 0
		for _, term := range terms {
			if strings.Contains(lower, term) {
				found++
			}
		}
		if found > 0 {
			candidates = append(candidates, candidate{index, found, line})
		}
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int { return b.found - a.found })
	candidates = candidates[:min(len(candidates), wikiExcerpts)]
	slices.SortFunc(candidates, func(a, b candidate) int { return a.index - b.index })

	lines := make([]string, 0, len(candidates))
	for _, found := range candidates {
		lines = append(lines, cutAround(found.line, terms, wikiExcerptRunes))
	}
	return lines
}

// cutAround shortens line to size runes, keeping the first term in view.
func cutAround(line string, terms []string, size int) string {
	runes := []rune(line)
	if len(runes) <= size {
		return line
	}
	// Lower-casing keeps the rune count for all but a few letters; where it
	// does not, the cut starts at the beginning of the line.
	lower := []rune(strings.ToLower(line))
	start := 0
	if len(lower) == len(runes) {
		first := -1
		for _, term := range terms {
			if at := strings.Index(string(lower), term); at >= 0 {
				if position := utf8.RuneCountInString(string(lower)[:at]); first < 0 || position < first {
					first = position
				}
			}
		}
		if first > size/3 {
			start = min(first-size/3, len(runes)-size)
		}
	}
	cut := string(runes[start : start+size])
	if start > 0 {
		cut = "…" + cut
	}
	if start+size < len(runes) {
		cut += "…"
	}
	return cut
}

// emptyWikiSearch says, for a search that found nothing, which words occur
// nowhere and what the wiki does hold. A model told only that nothing
// matched keeps trying other words.
func emptyWikiSearch(terms []string, seen []bool, names []string) string {
	if len(names) == 0 {
		return "The wiki has no pages yet, so there is nothing to find. Create a page with wiki_create."
	}
	var absent []string
	for i, term := range terms {
		if !seen[i] {
			absent = append(absent, term)
		}
	}
	message := "No wiki page has all of these words: " + strings.Join(terms, ", ") + "."
	if len(absent) > 0 {
		message = "No wiki page mentions: " + strings.Join(absent, ", ") + "."
	}
	listed := names[:min(len(names), wikiHoldings)]
	more := ""
	if extra := len(names) - len(listed); extra > 0 {
		more = fmt.Sprintf(", and %d more, listed by wiki_list", extra)
	}
	pages := "pages"
	if len(names) == 1 {
		pages = "page"
	}
	return fmt.Sprintf("%s The wiki holds %d %s: %s%s. A subject outside these is not in the wiki; the drive's reference archives are searched with search.",
		message, len(names), pages, strings.Join(listed, ", "), more)
}

// ---------------------------------------------------------------------------
// create, append, edit, move, delete
// ---------------------------------------------------------------------------

// wikiChange is the JSON shape a writing tool returns. The link is ready-made
// Markdown, as the file tools return it, and is there only while the wiki is
// open in a browser.
type wikiChange struct {
	Page  string `json:"page"`
	File  string `json:"file,omitempty"`
	Bytes int    `json:"bytes,omitempty"`
	Link  string `json:"link,omitempty"`
	// From is the name a moved page had, and Trash where a deleted one went.
	From  string `json:"from,omitempty"`
	Trash string `json:"trash,omitempty"`
}

// changed describes the page p after a write, with what the user does next
// to see it.
func (c *WikiCapability) changed(p page, size int) (wikiChange, string) {
	change := wikiChange{Page: p.name, File: path.Join(wiki.DirName, p.rel), Bytes: size}
	base := c.running()
	if base == "" {
		return change, "The wiki is not open in a browser now. The user opens it with Wiki in the drive's menu, or reads the file."
	}
	text := strings.NewReplacer("[", `\[`, "]", `\]`).Replace(p.name)
	change.Link = fmt.Sprintf("[%s](%s)", text, wiki.PageURL(base, p.name))
	return change, ""
}

func (c *WikiCapability) content(params map[string]any, what string) (string, error) {
	content, ok := params["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("missing required parameter: content (%s)", what)
	}
	return content, nil
}

func (c *WikiCapability) handleCreate(params map[string]any) (ActionResult, error) {
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}
	p, err := c.resolvePage(ws, params, "page")
	if err != nil {
		return ActionResult{}, err
	}
	content, err := c.content(params, "the page's text in Markdown")
	if err != nil {
		return ActionResult{}, err
	}
	if _, err := ws.Stat(p.rel); err == nil {
		return ActionResult{}, fmt.Errorf("page %q already exists. Add to it with wiki_append, change it with wiki_edit, or choose another name.", p.name)
	}
	content = endWithNewline(content)
	if err := c.writePage(ws, p, content); err != nil {
		return ActionResult{}, err
	}
	change, note := c.changed(p, len(content))
	return ActionResult{Data: change, Text: note}, nil
}

func (c *WikiCapability) handleAppend(params map[string]any) (ActionResult, error) {
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}
	p, err := c.resolvePage(ws, params, "page")
	if err != nil {
		return ActionResult{}, err
	}
	content, err := c.content(params, "the Markdown text to add")
	if err != nil {
		return ActionResult{}, err
	}
	existing := ""
	if _, statErr := ws.Stat(p.rel); !errors.Is(statErr, fs.ErrNotExist) {
		if existing, err = c.readPage(ws, p); err != nil {
			return ActionResult{}, err
		}
	}
	updated := endWithNewline(content)
	if before := strings.TrimRight(existing, "\n"); before != "" {
		updated = before + "\n\n" + updated
	}
	if err := c.writePage(ws, p, updated); err != nil {
		return ActionResult{}, err
	}
	change, note := c.changed(p, len(updated))
	return ActionResult{Data: change, Text: note}, nil
}

func endWithNewline(content string) string {
	return strings.TrimRight(content, "\n") + "\n"
}

func (c *WikiCapability) handleEdit(params map[string]any) (ActionResult, error) {
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}
	p, err := c.resolvePage(ws, params, "page")
	if err != nil {
		return ActionResult{}, err
	}
	content, err := c.readPage(ws, p)
	if err != nil {
		return ActionResult{}, err
	}
	oldText, _ := params["old_text"].(string)
	newText, ok := params["new_text"].(string)
	switch {
	case oldText == "":
		return ActionResult{}, fmt.Errorf("missing required parameter: old_text (the exact text to replace). To add to the end of the page use wiki_append.")
	case !ok:
		return ActionResult{}, fmt.Errorf("missing required parameter: new_text (use an empty string to delete old_text)")
	case oldText == newText:
		return ActionResult{}, fmt.Errorf("old_text and new_text are the same, so nothing would change")
	}
	switch count := strings.Count(content, oldText); {
	case count == 0:
		return ActionResult{}, fmt.Errorf("old_text was not found in page %q.%s It must match the page exactly, including spaces and line breaks. Call wiki_read and copy the text from there.", p.name, nearMiss(content, oldText))
	case count > 1:
		return ActionResult{}, fmt.Errorf("old_text occurs %d times in page %q. Add more of the surrounding text so it matches once.", count, p.name)
	}
	updated := strings.Replace(content, oldText, newText, 1)
	if err := c.writePage(ws, p, updated); err != nil {
		return ActionResult{}, err
	}
	change, note := c.changed(p, len(updated))
	return ActionResult{Data: change, Text: note}, nil
}

func (c *WikiCapability) handleMove(params map[string]any) (ActionResult, error) {
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}
	from, err := c.resolvePage(ws, params, "page")
	if err != nil {
		return ActionResult{}, err
	}
	to, err := c.resolvePage(ws, params, "new_name")
	if err != nil {
		return ActionResult{}, err
	}
	if from.rel == to.rel {
		return ActionResult{}, fmt.Errorf("page and new_name are the same, so nothing would move")
	}
	switch err := ws.Rename(from.rel, to.rel); {
	case errors.Is(err, fs.ErrExist):
		return ActionResult{}, fmt.Errorf("page %q already exists, and a move does not replace a page. Choose another new_name, or delete that page first with wiki_delete.", to.name)
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, workspace.ErrIsFolder):
		return ActionResult{}, c.pageError(ws, from, err)
	case err != nil:
		return ActionResult{}, errors.New(strings.NewReplacer(from.rel, fmt.Sprintf("page %q", from.name), to.rel, fmt.Sprintf("page %q", to.name)).Replace(err.Error()))
	}

	change, note := c.changed(to, 0)
	change.From = from.name
	if linkers := c.linkingPages(ws, from.name, to.name); len(linkers) > 0 {
		note = strings.TrimSpace(fmt.Sprintf("These pages still link to [[%s]]: %s. Links are not changed by a move; change them with wiki_edit if they matter. %s",
			from.name, strings.Join(linkers, ", "), note))
	}
	return ActionResult{Data: change, Text: note}, nil
}

// linkingPages names up to wikiLinkers pages that link to the page old. The
// page moved to is left out: a link to its own old name is its own business.
func (c *WikiCapability) linkingPages(ws *workspace.Workspace, old, moved string) []string {
	var linkers []string
	for _, name := range c.pageNames(ws) {
		if name == moved {
			continue
		}
		data, err := ws.ReadFile(name+".md", maxReadFileBytes)
		if err != nil {
			continue
		}
		text := string(data)
		for _, end := range []string{"]]", "|", "#", "@", "$"} {
			if strings.Contains(text, "[["+old+end) {
				linkers = append(linkers, name)
				break
			}
		}
		if len(linkers) == wikiLinkers {
			break
		}
	}
	return linkers
}

func (c *WikiCapability) handleDelete(params map[string]any) (ActionResult, error) {
	ws, err := c.openPages()
	if err != nil {
		return ActionResult{}, err
	}
	p, err := c.resolvePage(ws, params, "page")
	if err != nil {
		return ActionResult{}, err
	}
	if _, err := ws.Stat(p.rel); err != nil {
		return ActionResult{}, c.pageError(ws, p, err)
	}
	trash, err := c.openTrash()
	if err != nil {
		return ActionResult{}, err
	}

	// The time keeps a page deleted twice apart; the counter does when both
	// deletions fall in one second.
	stamp := c.now().Format("20060102-150405")
	var trashed string
	for attempt := 1; ; attempt++ {
		candidate := fmt.Sprintf("%s.%s.md", p.name, stamp)
		if attempt > 1 {
			candidate = fmt.Sprintf("%s.%s-%d.md", p.name, stamp, attempt)
		}
		if trashed, err = trash.Resolve(candidate); err != nil {
			return ActionResult{}, err
		}
		err = ws.MoveTo(p.rel, trash, trashed)
		if errors.Is(err, fs.ErrExist) && attempt < 100 {
			continue
		}
		if err != nil {
			return ActionResult{}, c.pageError(ws, p, err)
		}
		break
	}
	where := path.Join(wiki.TrashPath, trashed)
	return ActionResult{
		Data: wikiChange{Page: p.name, Trash: where},
		Text: fmt.Sprintf("Page %q was moved to the trash, not erased. To restore it the user moves %s on the drive back into the wiki folder as %s.", p.name, where, p.rel),
	}, nil
}
