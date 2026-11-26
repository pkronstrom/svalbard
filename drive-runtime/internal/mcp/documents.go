package mcp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

// Documents (PDFs under books/) are read from search.db alone: the index
// holds the text of every page that has any, so no kiwix-serve and no PDF
// reader is needed on the drive. A document's source name is its path on the
// drive, its articles are its pages (path page/N), and a folder of documents
// (a collection, books/<id>/) can be named as the source of a search.

// overviewPages caps the pages a document overview lists.
const overviewPages = 100

// readDocument serves search_read for a document: one page, in parts like
// any article and with links to the neighbouring pages, or, without a path,
// an overview of the document with its pages.
func (c *SearchCapability) readDocument(ctx context.Context, source, rawPath string, offset int) (ActionResult, error) {
	_, eng, err := c.getDB()
	if err != nil {
		return ActionResult{}, err
	}
	doc, err := eng.Document(ctx, source)
	if err != nil {
		return ActionResult{}, fmt.Errorf("%s is on the drive but not in the search index; run 'svalbard index' on the computer that manages the drive", source)
	}
	if strings.TrimSpace(rawPath) == "" {
		return c.documentOverview(ctx, eng, doc)
	}
	number, ok := engine.PageNumber(rawPath)
	if !ok {
		return ActionResult{}, fmt.Errorf("%s is read by page: %q is not a page. Use a path like page/12, or omit path for the list of pages.", source, rawPath)
	}
	page, err := eng.DocumentPage(ctx, source, number)
	if errors.Is(err, engine.ErrNoPage) {
		return ActionResult{}, noPageError(doc, page)
	}
	if err != nil {
		return ActionResult{}, err
	}

	body, next, total := pageSlice(page.Text, offset, readChars)
	if offset > 0 && offset >= total {
		return ActionResult{}, fmt.Errorf("offset %d is past the end of the page (%d characters)", offset, total)
	}
	result := readResult{
		Source: source,
		Path:   engine.PagePath(number),
		Title:  page.Title,
		Body:   body,
		Page:   number,
		File:   search.DocumentFile(source, number),
	}
	if next < total {
		result.Continues = fmt.Sprintf("Characters %d to %d of %d. Call search_read with the same source and path and offset %d for the next part.", offset, next, total, next)
	}
	if offset == 0 {
		if page.Previous > 0 {
			result.Links = append(result.Links, search.PageLink{Path: engine.PagePath(page.Previous), Label: "previous page"})
		}
		if page.Next > 0 {
			result.Links = append(result.Links, search.PageLink{Path: engine.PagePath(page.Next), Label: "next page"})
		}
	}
	return ActionResult{Data: result}, nil
}

func (c *SearchCapability) documentOverview(ctx context.Context, eng *engine.Engine, doc engine.Document) (ActionResult, error) {
	pages, err := eng.DocumentPages(ctx, doc.Filename, overviewPages)
	if err != nil {
		return ActionResult{}, err
	}
	result := readResult{Source: doc.Filename, Title: doc.Title, File: doc.Filename}
	switch {
	case doc.Pages == 0 && doc.Note != "":
		result.Body = fmt.Sprintf("%s cannot be searched or read here: %s.", doc.Title, doc.Note)
	case doc.Pages == 0:
		result.Body = fmt.Sprintf("%s has no text that can be searched or read here.", doc.Title)
	default:
		result.Body = fmt.Sprintf("%s is a document with %s of text. Read a page with path page/N, or search with this source to find the page on a subject.", doc.Title, countPages(doc.Pages))
		if doc.Note != "" {
			result.Note = "Not all of it is text: " + doc.Note + "."
		}
	}
	for _, page := range pages {
		result.Links = append(result.Links, search.PageLink{Path: page.Path, Label: page.Title})
	}
	if extra := doc.Pages - len(pages); extra > 0 {
		result.Note = strings.TrimSpace(result.Note + fmt.Sprintf(" The first %d pages are listed; %d more follow.", len(pages), extra))
	}
	return ActionResult{Data: result}, nil
}

func noPageError(doc engine.Document, page engine.DocumentPage) error {
	if doc.Pages == 0 {
		reason := "it has no text"
		if doc.Note != "" {
			reason = doc.Note
		}
		return fmt.Errorf("%s cannot be read here: %s", doc.Filename, reason)
	}
	var near []string
	for _, number := range []int{page.Previous, page.Next} {
		if number > 0 {
			near = append(near, engine.PagePath(number))
		}
	}
	return fmt.Errorf("%s has no text on page %d (the page is blank, an image, or past the end). Nearest pages with text: %s", doc.Filename, page.Number, strings.Join(near, ", "))
}

func countPages(n int) string {
	if n == 1 {
		return "1 page"
	}
	return fmt.Sprintf("%d pages", n)
}

// documentNames are the forms a document may be named by, lowercased: its
// path on the drive, with or without the extension and the books/ prefix.
func documentNames(filename string) []string {
	full := strings.ToLower(filename)
	bare := strings.TrimSuffix(full, strings.ToLower(extension(filename)))
	short := strings.TrimPrefix(full, engine.DocumentsDir+"/")
	return []string{full, bare, short, strings.TrimPrefix(bare, engine.DocumentsDir+"/")}
}

// extension is the last ".ext" of a path's final segment.
func extension(filename string) string {
	base := filename[strings.LastIndex(filename, "/")+1:]
	if dot := strings.LastIndex(base, "."); dot > 0 {
		return base[dot:]
	}
	return ""
}

// documentFolder returns the scope of a folder of documents, "books/<name>/",
// when name is one that holds indexed documents, and how many it holds. The
// name may come with or without the books/ prefix and a trailing slash.
func documentFolder(name string, filenames []string) (string, int) {
	name = strings.Trim(strings.TrimSpace(name), "/")
	name = strings.TrimPrefix(name, engine.DocumentsDir+"/")
	if name == "" || name == engine.DocumentsDir {
		return "", 0
	}
	count := 0
	folder := ""
	for _, filename := range filenames {
		if !engine.IsDocumentSource(filename) {
			continue
		}
		prefix := engine.DocumentsDir + "/" + name + "/"
		if len(filename) > len(prefix) && strings.EqualFold(filename[:len(prefix)], prefix) {
			folder = filename[:len(prefix)]
			count++
		}
	}
	return folder, count
}

// sourceNames lists sources for a message, with the documents of one folder
// folded into a single entry so a collection of fifty manuals stays one name.
func sourceNames(filenames []string) []string {
	var names []string
	folders := map[string]int{}
	for _, filename := range filenames {
		if !engine.IsDocumentSource(filename) {
			names = append(names, sourceName(filename))
			continue
		}
		rest := strings.TrimPrefix(filename, engine.DocumentsDir+"/")
		if cut := strings.Index(rest, "/"); cut > 0 {
			folders[engine.DocumentsDir+"/"+rest[:cut]]++
			continue
		}
		names = append(names, filename)
	}
	var grouped []string
	for folder, count := range folders {
		grouped = append(grouped, fmt.Sprintf("%s (%s)", folder, countDocuments(count)))
	}
	sort.Strings(grouped)
	return append(names, grouped...)
}

func countDocuments(n int) string {
	if n == 1 {
		return "1 document"
	}
	return fmt.Sprintf("%d documents", n)
}
