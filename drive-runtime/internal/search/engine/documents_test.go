package engine

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

// documentDB is an index as the host writes it for documents: sources carry
// a title and a note, articles keep the page's lines in sections.
func documentDB(t *testing.T) *sql.DB {
	t.Helper()
	db := setupTestDB(t)
	for _, statement := range []string{
		`ALTER TABLE sources ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN note TEXT`,
		`ALTER TABLE articles ADD COLUMN sections TEXT`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func insertDocument(t *testing.T, db *sql.DB, filename, title, note string) int {
	t.Helper()
	id := insertSource(t, db, filename)
	if _, err := db.Exec(`UPDATE sources SET title = ?, note = NULLIF(?, '') WHERE id = ?`, title, note, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// insertPage indexes one page: body is the one-line search text, lines the
// page as it reads.
func insertPage(t *testing.T, db *sql.DB, source, number int, title, body, lines string) {
	t.Helper()
	id := insertArticle(t, db, source, PagePath(number), title, body)
	if lines != "" {
		if _, err := db.Exec(`UPDATE articles SET sections = json_array(json_object('heading', '', 'body', ?)) WHERE id = ?`, lines, id); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadArticleFindsADocumentByItsPath(t *testing.T) {
	db := documentDB(t)
	insertPage(t, db, insertDocument(t, db, "books/manuals/volvo.pdf", "Volvo 240", ""), 112, "Volvo 240, p. 112", "Coolant capacity 9.5 litres.", "")
	insertArticle(t, db, insertSource(t, db, "wiki.zim"), "/Water", "Water", "Water is essential.")
	eng := New(db)

	// A document's source name ends in its own extension; ".zim" must not be
	// put behind it.
	if body, err := eng.ReadArticle("books/manuals/volvo.pdf", "page/112"); err != nil || body != "Coolant capacity 9.5 litres." {
		t.Fatalf("document page = %q, %v", body, err)
	}
	for _, name := range []string{"wiki", "wiki.zim"} {
		if body, err := eng.ReadArticle(name, "/Water"); err != nil || body != "Water is essential." {
			t.Errorf("ReadArticle(%q) = %q, %v", name, body, err)
		}
	}
	if _, err := eng.ReadArticle("books/manuals/volvo.pdf", "page/9"); err == nil {
		t.Error("a page that is not indexed was read")
	}
}

func TestDocumentPagesAndNeighbours(t *testing.T) {
	db := documentDB(t)
	volvo := insertDocument(t, db, "books/manuals/volvo.pdf", "Volvo 240", "2 of 12 pages are scanned images without text")
	// Pages 3 and 11 are images: not indexed. Page 10 sorts after 9, not after 1.
	for _, number := range []int{10, 1, 2, 9, 12} {
		insertPage(t, db, volvo, number, "Volvo 240, p. "+PagePath(number)[5:], "text of the page", "")
	}
	insertPage(t, db, volvo, 4, "Volvo 240, p. 4 · Fluids", "Coolant 9.5 litres Brake fluid DOT 4", "Coolant 9.5 litres\nBrake fluid DOT 4")
	insertDocument(t, db, "books/manuals/scan.pdf", "Trailer", "scanned: no text layer, needs OCR")
	insertArticle(t, db, insertSource(t, db, "wiki.zim"), "/page/1", "A wiki page", "not a document")
	eng := New(db)
	ctx := context.Background()

	documents, err := eng.Documents(ctx)
	want := []Document{
		{Filename: "books/manuals/scan.pdf", Title: "Trailer", Pages: 0, Note: "scanned: no text layer, needs OCR"},
		{Filename: "books/manuals/volvo.pdf", Title: "Volvo 240", Pages: 6, Note: "2 of 12 pages are scanned images without text"},
	}
	if err != nil || !reflect.DeepEqual(documents, want) {
		t.Fatalf("Documents = %+v, %v; want %+v", documents, err, want)
	}

	pages, err := eng.DocumentPages(ctx, "books/manuals/volvo.pdf", 4)
	if err != nil || len(pages) != 4 || pages[0].Path != "page/1" || pages[2].Path != "page/4" || pages[3].Path != "page/9" {
		t.Fatalf("DocumentPages = %+v, %v; want pages 1, 2, 4, 9 in order", pages, err)
	}

	page, err := eng.DocumentPage(ctx, "books/manuals/volvo.pdf", 4)
	if err != nil {
		t.Fatal(err)
	}
	// The page reads with its lines; its neighbours are the indexed pages.
	if page.Text != "Coolant 9.5 litres\nBrake fluid DOT 4" || page.Title != "Volvo 240, p. 4 · Fluids" || page.Previous != 2 || page.Next != 9 {
		t.Errorf("page 4 = %+v", page)
	}
	if page, _ := eng.DocumentPage(ctx, "books/manuals/volvo.pdf", 10); page.Previous != 9 || page.Next != 12 || page.Text != "text of the page" {
		t.Errorf("page 10 = %+v, want neighbours 9 and 12 and the indexed body", page)
	}
	if page, _ := eng.DocumentPage(ctx, "books/manuals/volvo.pdf", 12); page.Next != 0 {
		t.Errorf("last page has a next page: %+v", page)
	}
	// A page without text is named as missing, with where to go instead.
	missing, err := eng.DocumentPage(ctx, "books/manuals/volvo.pdf", 3)
	if !errors.Is(err, ErrNoPage) || missing.Previous != 2 || missing.Next != 4 {
		t.Errorf("page 3 = %+v, %v; want ErrNoPage between 2 and 4", missing, err)
	}
}

func TestDocumentsReadsAnIndexWithoutNotes(t *testing.T) {
	db := setupTestDB(t)
	if _, err := db.Exec(`ALTER TABLE sources ADD COLUMN title TEXT NOT NULL DEFAULT ''`); err != nil {
		t.Fatal(err)
	}
	insertArticle(t, db, insertSource(t, db, "books/a.pdf"), "page/1", "A, p. 1", "text")
	documents, err := New(db).Documents(context.Background())
	if err != nil || len(documents) != 1 || documents[0].Pages != 1 || documents[0].Note != "" {
		t.Fatalf("Documents = %+v, %v", documents, err)
	}
}

func TestSearchScopeCoversOneDocumentOrAFolderOfThem(t *testing.T) {
	db := documentDB(t)
	insertPage(t, db, insertDocument(t, db, "books/car-manuals/volvo.pdf", "Volvo 240", ""), 1, "Volvo 240, p. 1", "coolant capacity", "")
	insertPage(t, db, insertDocument(t, db, "books/car-manuals/workshop/b20.pdf", "B20", ""), 1, "B20, p. 1", "coolant drain plug", "")
	insertPage(t, db, insertDocument(t, db, "books/car-manuals-old/saab.pdf", "Saab", ""), 1, "Saab, p. 1", "coolant mix", "")
	insertPage(t, db, insertDocument(t, db, "books/100%_proof/x.pdf", "X", ""), 1, "X, p. 1", "coolant bottle", "")
	insertArticle(t, db, insertSource(t, db, "wiki.zim"), "/Coolant", "Coolant", "coolant is a fluid")

	for scope, want := range map[string]int{
		"":                            5,
		"books/car-manuals/volvo.pdf": 1,
		"books/car-manuals/":          2, // not the folder whose name merely starts the same
		"books/100%_proof/":           1, // a folder name is not a LIKE pattern
		"books/":                      4,
		"wiki.zim":                    1,
	} {
		matches := search(t, db, "coolant", KeywordOptions{Source: scope})
		if len(matches.Results) != want {
			t.Errorf("scope %q found %v, want %d results", scope, paths(matches.Results), want)
		}
		for _, result := range matches.Results {
			if !InScope(result.Filename, scope) {
				t.Errorf("scope %q returned %s", scope, result.Filename)
			}
		}
	}
	// A word that is on the drive but not in the scope is reported missing.
	if matches := search(t, db, "fluid coolant", KeywordOptions{Source: "books/car-manuals/"}); !reflect.DeepEqual(matches.Missing, []string{"fluid"}) {
		t.Errorf("missing = %v, want fluid", matches.Missing)
	}
}

func TestPageNumber(t *testing.T) {
	for path, want := range map[string]int{
		"page/12": 12, "/page/12": 12, "12": 12, "p. 12": 12, "page=12": 12, "#page=12": 12, "Page 12": 12, "p12": 12,
		"page/0": 0, "page/-3": 0, "page/twelve": 0, "": 0, "Water_purification": 0, "page/12/extra": 0,
	} {
		got, ok := PageNumber(path)
		if got != want && want != 0 || ok != (want != 0) {
			t.Errorf("PageNumber(%q) = %d, %v; want %d", path, got, ok, want)
		}
	}
}

// Every page of a document repeats the document's title, so a query that
// names it matches all of them in the weighted column. One document must not
// fill the results and bury what other sources have on the subject.
func TestSearchReturnsFewPagesPerDocumentUnlessLimitedToIt(t *testing.T) {
	db := documentDB(t)
	who := insertDocument(t, db, "books/who.pdf", "Basic Emergency Care", "")
	for number := 1; number <= 40; number++ {
		insertPage(t, db, who, number, "Basic Emergency Care, p. "+PagePath(number)[5:], "Assess the airway and give oxygen.", "")
	}
	other := insertDocument(t, db, "books/manuals/first-aid.pdf", "First Aid", "")
	insertPage(t, db, other, 1, "First Aid, p. 1", "Emergency care of burns.", "")
	wiki := insertSource(t, db, "wiki.zim")
	insertArticle(t, db, wiki, "/Emergency_medicine", "Emergency medicine", "The care of illness that needs immediate attention.")
	insertArticle(t, db, wiki, "/Triage", "Triage", "Sorting patients in emergency care by urgency.")

	count := func(matches KeywordMatches) map[string]int {
		perSource := map[string]int{}
		for _, result := range matches.Results {
			perSource[result.Filename]++
		}
		return perSource
	}
	all := search(t, db, "emergency care", KeywordOptions{Limit: 8})
	if got := count(all); got["books/who.pdf"] != pagesPerDocument || got["wiki.zim"] != 2 || got["books/manuals/first-aid.pdf"] != 1 {
		t.Fatalf("results per source = %v, want %d pages of the manual and every other match", got, pagesPerDocument)
	}
	if all.Exact != len(all.Results) || !reflect.DeepEqual(all.MorePages, []string{"books/who.pdf"}) {
		t.Errorf("exact %d of %d, more pages in %v; want the manual named as having more", all.Exact, len(all.Results), all.MorePages)
	}
	// The best pages of the manual still lead: the cap does not reorder.
	if all.Results[0].Filename != "books/who.pdf" {
		t.Errorf("first result = %+v", all.Results[0])
	}
	// A folder of documents is capped per document as well.
	if got := count(search(t, db, "emergency care", KeywordOptions{Source: "books/", Limit: 8})); got["books/who.pdf"] != pagesPerDocument || got["books/manuals/first-aid.pdf"] != 1 {
		t.Errorf("folder scope results = %v", got)
	}
	// Limited to the document itself, every page is available.
	one := search(t, db, "emergency care", KeywordOptions{Source: "books/who.pdf", Limit: 8})
	if len(one.Results) != 8 || len(one.MorePages) != 0 {
		t.Errorf("document scope returned %d results, more pages %v; want 8 and none", len(one.Results), one.MorePages)
	}
}
