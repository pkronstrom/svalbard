package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// DocumentsDir is the drive directory whose files are indexed as documents.
// A document's source name is its drive-relative path (books/manuals/a.pdf),
// with one article per page at the path page/N. A ZIM source name never holds
// a "/", so the two kinds cannot be confused.
const DocumentsDir = "books"

// IsDocumentFile reports whether a file below DocumentsDir is a document by
// its name, and not something a download left beside one (a
// ".download.json" record, a partial file). The extensions are those of the
// recipe types the host stores there: keep in sync with TypeDirs in
// host-cli/internal/toolkit.
func IsDocumentFile(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf", ".epub":
		return true
	}
	return false
}

// ErrNoPage is returned for a page a document's index does not hold: it is
// out of range, blank, or an image without a text layer.
var ErrNoPage = errors.New("page not indexed")

// IsDocumentSource reports whether an indexed source is a document rather
// than a ZIM archive.
func IsDocumentSource(filename string) bool {
	return strings.HasPrefix(filename, DocumentsDir+"/")
}

// InScope reports whether an indexed source lies in a search scope: the empty
// scope is every source, a scope ending in "/" is every source below that
// folder (a document collection), and any other is one source by filename.
func InScope(filename, scope string) bool {
	switch {
	case scope == "":
		return true
	case strings.HasSuffix(scope, "/"):
		return strings.HasPrefix(filename, scope)
	}
	return filename == scope
}

// PagePath is the article path of a document's page.
func PagePath(number int) string { return "page/" + strconv.Itoa(number) }

// PageNumber reads the page out of a document article path. It accepts the
// forms a path comes back in from a model: "page/12", "/page/12", "12",
// "p. 12", "page=12" and "#page=12".
func PageNumber(path string) (int, bool) {
	path = strings.ToLower(strings.TrimSpace(path))
	path = strings.TrimLeft(path, "/#")
	for _, prefix := range []string{"page", "p"} {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			path = strings.TrimLeft(rest, "/=.: ")
			break
		}
	}
	number, err := strconv.Atoi(path)
	return number, err == nil && number > 0
}

// Document is what the index holds about one document.
type Document struct {
	Filename string
	Title    string
	// Pages counts the pages that have text and are searchable.
	Pages int
	// Note says why the document is not, or not fully, searchable.
	Note string
}

// Documents lists the indexed documents. An index written before documents
// had notes is read without them.
func (e *Engine) Documents(ctx context.Context) ([]Document, error) {
	query := func(note string) (*sql.Rows, error) {
		return e.db.QueryContext(ctx,
			`SELECT src.filename, src.title, `+note+`, COUNT(a.id) FROM sources src
             LEFT JOIN articles a ON a.source_id = src.id
             WHERE src.filename LIKE ? GROUP BY src.id ORDER BY src.filename`, DocumentsDir+"/%")
	}
	rows, err := query("COALESCE(src.note, '')")
	if err != nil {
		if rows, err = query("''"); err != nil {
			return nil, fmt.Errorf("engine: documents: %w", err)
		}
	}
	defer rows.Close()
	var documents []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.Filename, &doc.Title, &doc.Note, &doc.Pages); err != nil {
			return nil, err
		}
		documents = append(documents, doc)
	}
	return documents, rows.Err()
}

// Document returns one indexed document.
func (e *Engine) Document(ctx context.Context, filename string) (Document, error) {
	documents, err := e.Documents(ctx)
	if err != nil {
		return Document{}, err
	}
	for _, doc := range documents {
		if doc.Filename == filename {
			return doc, nil
		}
	}
	return Document{}, fmt.Errorf("document not indexed: %s", filename)
}

// pageOrder sorts a document's articles by page number; the path is text.
const pageOrder = `CAST(substr(a.path, 6) AS INTEGER)`

// DocumentPages lists a document's indexed pages in page order, up to limit.
func (e *Engine) DocumentPages(ctx context.Context, filename string, limit int) ([]IndexedPath, error) {
	rows, err := e.db.QueryContext(ctx,
		`SELECT a.path, a.title FROM articles a
         JOIN sources src ON src.id = a.source_id
         WHERE src.filename = ? AND a.path LIKE 'page/%'
         ORDER BY `+pageOrder+` LIMIT ?`, filename, limit)
	if err != nil {
		return nil, fmt.Errorf("engine: document pages: %w", err)
	}
	defer rows.Close()
	var pages []IndexedPath
	for rows.Next() {
		var page IndexedPath
		if err := rows.Scan(&page.Path, &page.Title); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, rows.Err()
}

// DocumentPage is one indexed page with its neighbours.
type DocumentPage struct {
	Number int
	Title  string
	// Text keeps the page's lines, so a table row stays on one line.
	Text string
	// Previous and Next are the nearest indexed pages, 0 when there is none.
	// Pages without text are not indexed, so these are not always one apart.
	Previous, Next int
}

// DocumentPage reads one page of a document from the index. When the page is
// not indexed it returns ErrNoPage together with the nearest pages that are.
func (e *Engine) DocumentPage(ctx context.Context, filename string, number int) (DocumentPage, error) {
	page := DocumentPage{Number: number}
	neighbour := func(comparison, order string) int {
		var found sql.NullInt64
		_ = e.db.QueryRow(
			`SELECT `+pageOrder+` FROM articles a
             JOIN sources src ON src.id = a.source_id
             WHERE src.filename = ? AND a.path LIKE 'page/%' AND `+pageOrder+comparison+` ?
             ORDER BY `+pageOrder+order+` LIMIT 1`, filename, number).Scan(&found)
		return int(found.Int64)
	}
	page.Previous, page.Next = neighbour(" <", " DESC"), neighbour(" >", " ASC")

	var body, sections sql.NullString
	if err := ctx.Err(); err != nil {
		return page, err
	}
	err := e.db.QueryRow(
		`SELECT a.title, a.body, a.sections FROM articles a
         JOIN sources src ON src.id = a.source_id
         WHERE src.filename = ? AND a.path = ?`, filename, PagePath(number)).Scan(&page.Title, &body, &sections)
	if errors.Is(err, sql.ErrNoRows) {
		return page, ErrNoPage
	}
	if err != nil {
		return page, fmt.Errorf("engine: document page: %w", err)
	}
	page.Text = body.String
	// The indexed body is one line for keyword search; the page's own lines
	// are kept in its section.
	var parts []struct {
		Body string `json:"body"`
	}
	if sections.Valid && json.Unmarshal([]byte(sections.String), &parts) == nil && len(parts) > 0 && strings.TrimSpace(parts[0].Body) != "" {
		page.Text = parts[0].Body
	}
	return page, nil
}
