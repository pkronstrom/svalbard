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

	body, next, total := pageSlice(page.Text, offset, c.read.Chars)
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

// archiveListed caps the documents the note on an archive's main page names.
const archiveListed = 20

// archiveDocumentsNote names the documents indexed from inside an archive,
// for the reader of its main page: the main page of an archive that is a
// set of PDFs is often an empty web app. "" for an archive that holds none.
func (c *SearchCapability) archiveDocumentsNote(ctx context.Context, archive string) string {
	_, eng, err := c.getDB()
	if err != nil {
		return ""
	}
	documents, err := eng.Documents(ctx)
	if err != nil {
		return ""
	}
	var held []engine.Document
	for _, doc := range documents {
		if inside, _, ok := engine.EmbeddedArchive(doc.Filename); ok && inside == archive {
			held = append(held, doc)
		}
	}
	if len(held) == 0 {
		return ""
	}
	listed := make([]string, 0, min(len(held), archiveListed))
	for _, doc := range held[:min(len(held), archiveListed)] {
		detail := countPages(doc.Pages)
		if doc.Pages == 0 {
			reason := doc.Note
			if reason == "" {
				reason = "no text"
			}
			detail = "not searchable: " + reason
		}
		listed = append(listed, fmt.Sprintf("%s (%s, %s)", doc.Filename, doc.Title, detail))
	}
	note := fmt.Sprintf("This archive also holds %s, read from the index with search_read: %s.", countDocuments(len(held)), strings.Join(listed, "; "))
	if extra := len(held) - len(listed); extra > 0 {
		note += fmt.Sprintf(" And %d more; search with this source to find a page.", extra)
	}
	return note
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
// A document inside an archive may also be named under the archive's name
// without ".zim", or without the language and date kiwix appends to it:
// "usda-2015/files/guide.pdf" for "usda-2015_en_2025-04.zim/files/guide.pdf".
func documentNames(filename string) []string {
	full := strings.ToLower(filename)
	bare := strings.TrimSuffix(full, strings.ToLower(extension(filename)))
	if archive, entry, ok := engine.EmbeddedArchive(full); ok {
		names := []string{full, bare}
		entryBare := strings.TrimSuffix(entry, extension(entry))
		for _, name := range []string{strings.TrimSuffix(archive, ".zim"), archiveShortName(archive)} {
			names = append(names, name+"/"+entry, name+"/"+entryBare)
		}
		return names
	}
	short := strings.TrimPrefix(full, engine.DocumentsDir+"/")
	return []string{full, bare, short, strings.TrimPrefix(bare, engine.DocumentsDir+"/")}
}

// archiveShortName is an archive's name before the first "_", where kiwix
// names put the language, selection and date: "usda-2015" for
// "usda-2015_en_2025-04.zim".
func archiveShortName(archive string) string {
	name := strings.TrimSuffix(archive, ".zim")
	if cut := strings.Index(name, "_"); cut > 0 {
		return name[:cut]
	}
	return name
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
// name may come with or without the books/ prefix and a trailing slash. A
// skill root (skills, lib/js) or a folder below one is a scope as well, when
// it is named with its root.
func documentFolder(name string, filenames []string) (string, int) {
	name = strings.Trim(strings.TrimSpace(name), "/")
	for _, root := range []string{engine.SkillsDir, engine.LibraryDocsDir} {
		if strings.EqualFold(name, root) || len(name) > len(root) && strings.EqualFold(name[:len(root)+1], root+"/") {
			// With nothing there, the name may still be a document
			// collection's: one called "skills" is books/skills/.
			if folder, count := folderScope(name+"/", filenames, engine.IsSkillSource); count > 0 {
				return folder, count
			}
		}
	}
	name = strings.TrimPrefix(name, engine.DocumentsDir+"/")
	if name == "" || name == engine.DocumentsDir {
		return "", 0
	}
	return folderScope(engine.DocumentsDir+"/"+name+"/", filenames, engine.IsDocumentSource)
}

// folderScope finds the indexed sources of one kind below prefix, compared
// without regard to case, and returns the prefix as the index spells it.
func folderScope(prefix string, filenames []string, kind func(string) bool) (string, int) {
	count := 0
	folder := ""
	for _, filename := range filenames {
		if kind(filename) && len(filename) > len(prefix) && strings.EqualFold(filename[:len(prefix)], prefix) {
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
		if engine.IsEmbeddedSource(filename) {
			// Named under its archive, which is listed on its own.
			continue
		}
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
