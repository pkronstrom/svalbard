package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

// documentFixture is a drive with one archive (wiki.zim, from
// searchFixtureDrive) and a collection of PDFs indexed by page. There is no
// kiwix-serve on it or on the host: every document read must come from the
// index.
type documentFixture struct {
	t    *testing.T
	root string
	db   *sql.DB
	cap  *SearchCapability
}

func newDocumentFixture(t *testing.T) *documentFixture {
	t.Helper()
	root := searchFixtureDrive(t)
	t.Setenv("PATH", t.TempDir())
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, statement := range []string{
		`ALTER TABLE sources ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN note TEXT`,
		`ALTER TABLE articles ADD COLUMN sections TEXT`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	f := &documentFixture{t: t, root: root, db: db}

	volvo := f.document("books/car-manuals/volvo-240.pdf", "Volvo 240 Owner's Manual (1990)", "")
	f.page(volvo, 111, "Volvo 240 Owner's Manual (1990), p. 111 · Cooling system", "Checking the coolant level")
	f.page(volvo, 112, "Volvo 240 Owner's Manual (1990), p. 112 · Cooling system", "Coolant capacity incl. heater:\n9.5 litres\nUse only type C coolant")
	f.page(volvo, 114, "Volvo 240 Owner's Manual (1990), p. 114 · Brakes", "Brake fluid DOT 4")
	hmmwv := f.document("books/car-manuals/workshop/hmmwv.pdf", "HMMWV Operator's Manual", "3 of 8 pages are scanned images without text")
	f.page(hmmwv, 1, "HMMWV Operator's Manual, p. 1", "Coolant capacity 26 quarts")
	f.document("books/car-manuals/trailer.pdf", "Trailer manual", "scanned: no text layer, needs OCR")
	who := f.document("books/WHO_Basic_Emergency_Care.pdf", "WHO Basic Emergency Care", "")
	f.page(who, 5, "WHO Basic Emergency Care, p. 5", "Naloxone for opioid overdose")

	manifest := "version: 2\nrealized:\n    entries:\n"
	for _, entry := range [][2]string{
		{"car-manuals", "books/car-manuals/volvo-240.pdf"}, {"car-manuals", "books/car-manuals/workshop/hmmwv.pdf"},
		{"car-manuals", "books/car-manuals/trailer.pdf"}, {"who-basic-emergency-care", "books/WHO_Basic_Emergency_Care.pdf"},
	} {
		manifest += fmt.Sprintf("        - id: %s\n          type: pdf\n          filename: %s\n          relative_path: %s\n", entry[0], filepath.Base(entry[1]), entry[1])
		f.file(entry[1], "%PDF-1.4")
	}
	f.file("manifest.yaml", manifest)
	f.file(".svalbard/recipes.json", `{
		"car-manuals": {"id": "car-manuals", "type": "pdf", "description": "Owner and workshop manuals for our vehicles", "tags": ["vehicles", "repair"]},
		"who-basic-emergency-care": {"id": "who-basic-emergency-care", "type": "pdf", "description": "WHO Basic Emergency Care (open-access triage)", "tags": ["medicine"]}
	}`)
	meta, err := LoadMetadata(root)
	if err != nil {
		t.Fatal(err)
	}
	f.cap = NewSearchCapability(root, meta)
	t.Cleanup(func() { _ = f.cap.Close() })
	return f
}

func (f *documentFixture) file(rel, body string) {
	f.t.Helper()
	path := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *documentFixture) document(filename, title, note string) int64 {
	f.t.Helper()
	res, err := f.db.Exec(`INSERT INTO sources (filename, title, note) VALUES (?, ?, NULLIF(?, ''))`, filename, title, note)
	if err != nil {
		f.t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// page indexes a page as the host does: the body on one line, the page's own
// lines in its section.
func (f *documentFixture) page(source int64, number int, title, lines string) {
	f.t.Helper()
	body := strings.Join(strings.Fields(lines), " ")
	res, err := f.db.Exec(`INSERT INTO articles (source_id, path, title, body, sections) VALUES (?, ?, ?, ?, json_array(json_object('heading', '', 'body', ?)))`,
		source, fmt.Sprintf("page/%d", number), title, body, lines)
	if err != nil {
		f.t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := f.db.Exec(`INSERT INTO articles_fts(rowid, title, body) VALUES (?, ?, ?)`, id, title, body); err != nil {
		f.t.Fatal(err)
	}
}

func (f *documentFixture) search(params map[string]any) ([]searchResultItem, string) {
	f.t.Helper()
	result, err := f.cap.Handle(context.Background(), "search", params)
	if err != nil {
		f.t.Fatalf("search %v: %v", params, err)
	}
	return result.Data.([]searchResultItem), result.Text
}

func (f *documentFixture) read(params map[string]any) (readResult, error) {
	f.t.Helper()
	result, err := f.cap.Handle(context.Background(), "read", params)
	if err != nil {
		return readResult{}, err
	}
	return result.Data.(readResult), nil
}

func sources(items []searchResultItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Source + " " + item.Path
	}
	return out
}

func TestSearchHitInADocumentNamesItsPageAndFile(t *testing.T) {
	f := newDocumentFixture(t)
	items, _ := f.search(map[string]any{"query": "coolant capacity heater"})
	if len(items) == 0 {
		t.Fatal("no results")
	}
	want := searchResultItem{
		Source:  "books/car-manuals/volvo-240.pdf",
		Path:    "page/112",
		Title:   "Volvo 240 Owner's Manual (1990), p. 112 · Cooling system",
		Snippet: items[0].Snippet,
		Page:    112,
		File:    "books/car-manuals/volvo-240.pdf#page=112",
	}
	if !reflect.DeepEqual(items[0], want) {
		t.Errorf("hit = %+v, want %+v", items[0], want)
	}
	if !strings.Contains(items[0].Snippet, "9.5 litres") {
		t.Errorf("snippet = %q", items[0].Snippet)
	}
	// An archive hit has neither field.
	items, _ = f.search(map[string]any{"query": "water"})
	if len(items) != 1 || items[0].Source != "wiki" || items[0].Page != 0 || items[0].File != "" {
		t.Errorf("archive hit = %+v", items)
	}
	// Full detail reads the page from the index too.
	items, _ = f.search(map[string]any{"query": "naloxone", "detail": "full"})
	if len(items) != 1 || items[0].Body != "Naloxone for opioid overdose" || items[0].Page != 5 {
		t.Errorf("full-detail hit = %+v", items)
	}
}

func TestSearchCanBeLimitedToADocumentOrACollection(t *testing.T) {
	f := newDocumentFixture(t)
	for source, want := range map[string][]string{
		"books/car-manuals":               {"books/car-manuals/volvo-240.pdf page/112", "books/car-manuals/volvo-240.pdf page/111", "books/car-manuals/workshop/hmmwv.pdf page/1"},
		"car-manuals/":                    nil,
		"books/car-manuals/volvo-240.pdf": {"books/car-manuals/volvo-240.pdf page/112", "books/car-manuals/volvo-240.pdf page/111"},
		"car-manuals/volvo-240":           nil,
		"hmmwv":                           {"books/car-manuals/workshop/hmmwv.pdf page/1"},
		"WHO_Basic_Emergency_Care":        {},
	} {
		items, _ := f.search(map[string]any{"query": "coolant", "source": source})
		got := sources(items)
		if want == nil {
			continue // same scope under another spelling, checked below
		}
		if len(got) != len(want) {
			t.Errorf("source %q found %v, want %v", source, got, want)
			continue
		}
		for _, hit := range want {
			if !contains(got, hit) {
				t.Errorf("source %q found %v, want %v", source, got, want)
			}
		}
	}
	a, _ := f.search(map[string]any{"query": "coolant", "source": "car-manuals/"})
	b, _ := f.search(map[string]any{"query": "coolant", "source": "books/car-manuals"})
	if len(a) != 3 || !reflect.DeepEqual(sources(a), sources(b)) {
		t.Errorf("collection named without books/ found %v, want the same as %v", sources(a), sources(b))
	}
	a, _ = f.search(map[string]any{"query": "coolant", "source": "car-manuals/volvo-240"})
	if len(a) != 2 {
		t.Errorf("document named without books/ and extension found %v", sources(a))
	}
	// The advisory names the scope the way the caller can use it again.
	_, note := f.search(map[string]any{"query": "naloxone", "source": "books/car-manuals"})
	if !strings.Contains(note, "in books/car-manuals mentions: naloxone") {
		t.Errorf("advisory = %q", note)
	}

	_, err := f.cap.Handle(context.Background(), "search", map[string]any{"query": "coolant", "source": "tractor-manuals"})
	if err == nil || !strings.Contains(err.Error(), "books/car-manuals (3 documents)") || !strings.Contains(err.Error(), "books/WHO_Basic_Emergency_Care.pdf") || !strings.Contains(err.Error(), "wiki") {
		t.Errorf("unknown source error = %v, want the sources with the collection as one entry", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestReadDocumentPageComesFromTheIndex(t *testing.T) {
	f := newDocumentFixture(t)
	want := readResult{
		Source: "books/car-manuals/volvo-240.pdf",
		Path:   "page/112",
		Title:  "Volvo 240 Owner's Manual (1990), p. 112 · Cooling system",
		Body:   "Coolant capacity incl. heater:\n9.5 litres\nUse only type C coolant",
		Links: []search.PageLink{
			{Path: "page/111", Label: "previous page"},
			{Path: "page/114", Label: "next page"},
		},
		Page: 112,
		File: "books/car-manuals/volvo-240.pdf#page=112",
	}
	for _, params := range []map[string]any{
		{"source": "books/car-manuals/volvo-240.pdf", "path": "page/112"},
		{"source": "car-manuals/volvo-240", "path": "/page/112"},
		{"source": "volvo-240", "path": "112"},
		{"source": "BOOKS/CAR-MANUALS/VOLVO-240.PDF", "path": "p. 112"},
		{"source": "books/car-manuals/volvo-240.pdf", "path": "#page=112"},
	} {
		got, err := f.read(params)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("read %v = %+v, %v; want %+v", params, got, err, want)
		}
	}
	// kiwix-serve was never started for any of it.
	if len(f.cap.kiwix) != 0 {
		t.Error("a document read started kiwix-serve")
	}

	// A long page comes in parts like any article.
	long := f.document("books/long.pdf", "Long", "")
	f.page(long, 1, "Long, p. 1", strings.Repeat("A line of the table of torque values.\n", 400))
	first, err := f.read(map[string]any{"source": "books/long.pdf", "path": "page/1"})
	if err != nil || first.Continues == "" || len([]rune(first.Body)) > readChars {
		t.Fatalf("first part = %d characters, continues %q, %v", len([]rune(first.Body)), first.Continues, err)
	}
	var offset int
	if _, err := fmt.Sscanf(first.Continues[strings.LastIndex(first.Continues, "offset ")+7:], "%d", &offset); err != nil {
		t.Fatalf("continues = %q", first.Continues)
	}
	second, err := f.read(map[string]any{"source": "books/long.pdf", "path": "page/1", "offset": offset})
	if err != nil || second.Body == "" || len(second.Links) != 0 || second.Page != 1 {
		t.Fatalf("second part = %+v, %v", second, err)
	}
}

func TestReadDocumentWithoutPathListsItsPages(t *testing.T) {
	f := newDocumentFixture(t)
	overview, err := f.read(map[string]any{"source": "books/car-manuals/workshop/hmmwv.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Title != "HMMWV Operator's Manual" || overview.File != "books/car-manuals/workshop/hmmwv.pdf" || overview.Page != 0 {
		t.Errorf("overview = %+v", overview)
	}
	if !strings.Contains(overview.Body, "1 page of text") || !strings.Contains(overview.Note, "3 of 8 pages are scanned images without text") {
		t.Errorf("overview body %q, note %q", overview.Body, overview.Note)
	}
	if want := []search.PageLink{{Path: "page/1", Label: "HMMWV Operator's Manual, p. 1"}}; !reflect.DeepEqual(overview.Links, want) {
		t.Errorf("links = %+v, want %+v", overview.Links, want)
	}

	// A document with more pages than an overview lists says so.
	big := f.document("books/big.pdf", "Big", "")
	for number := 1; number <= overviewPages+5; number++ {
		f.page(big, number, fmt.Sprintf("Big, p. %d", number), "text")
	}
	overview, err = f.read(map[string]any{"source": "books/big.pdf"})
	if err != nil || len(overview.Links) != overviewPages || overview.Links[9].Path != "page/10" || !strings.Contains(overview.Note, "5 more follow") {
		t.Errorf("big overview: %d links, tenth %+v, note %q, %v", len(overview.Links), overview.Links[9], overview.Note, err)
	}

	// A scanned document reads as what it is.
	scanned, err := f.read(map[string]any{"source": "books/car-manuals/trailer.pdf"})
	if err != nil || !strings.Contains(scanned.Body, "cannot be searched or read here: scanned: no text layer, needs OCR") || len(scanned.Links) != 0 {
		t.Errorf("scanned overview = %+v, %v", scanned, err)
	}
}

func TestReadDocumentErrorsSayWhereToGo(t *testing.T) {
	f := newDocumentFixture(t)
	for name, tc := range map[string]struct {
		params map[string]any
		want   []string
	}{
		"image page":    {map[string]any{"source": "books/car-manuals/volvo-240.pdf", "path": "page/113"}, []string{"no text on page 113", "page/112, page/114"}},
		"past the end":  {map[string]any{"source": "books/car-manuals/volvo-240.pdf", "path": "page/900"}, []string{"no text on page 900", "Nearest pages with text: page/114"}},
		"not a page":    {map[string]any{"source": "books/car-manuals/volvo-240.pdf", "path": "Cooling_system"}, []string{"read by page", "page/12"}},
		"scanned":       {map[string]any{"source": "books/car-manuals/trailer.pdf", "path": "page/1"}, []string{"cannot be read here: scanned: no text layer, needs OCR"}},
		"a collection":  {map[string]any{"source": "books/car-manuals", "path": "page/1"}, []string{"is a folder of 3 documents", "books/car-manuals/volvo-240.pdf", "search with source books/car-manuals"}},
		"unknown":       {map[string]any{"source": "books/nope.pdf", "path": "page/1"}, []string{"unknown source", "books/car-manuals (3 documents)"}},
		"past the page": {map[string]any{"source": "books/car-manuals/volvo-240.pdf", "path": "page/112", "offset": 5000}, []string{"offset 5000 is past the end of the page"}},
	} {
		_, err := f.read(tc.params)
		if err == nil {
			t.Errorf("%s: read succeeded", name)
			continue
		}
		for _, want := range tc.want {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error %q does not contain %q", name, err, want)
			}
		}
	}
	if len(f.cap.kiwix) != 0 {
		t.Error("a failed document read started kiwix-serve")
	}
}

func TestSearchSaysWhenADocumentHasMoreMatchingPages(t *testing.T) {
	f := newDocumentFixture(t)
	manual := f.document("books/car-manuals/saab-900.pdf", "Saab 900 Workshop Manual", "")
	for number := 1; number <= 12; number++ {
		f.page(manual, number, fmt.Sprintf("Saab 900 Workshop Manual, p. %d", number), "Torque values for the workshop.")
	}
	items, note := f.search(map[string]any{"query": "saab workshop manual"})
	if len(items) != 3 {
		t.Fatalf("results = %v, want the three best pages of the manual", sources(items))
	}
	if want := "More pages match in: books/car-manuals/saab-900.pdf. Search with one of them as source to see them."; !strings.Contains(note, want) {
		t.Errorf("advisory = %q, want it to contain %q", note, want)
	}
	// Following the advice lists more of them.
	items, note = f.search(map[string]any{"query": "saab workshop manual", "source": "books/car-manuals/saab-900.pdf"})
	if len(items) != defaultSearchLimit || strings.Contains(note, "More pages match") {
		t.Errorf("document scope: %d results, advisory %q", len(items), note)
	}
}

func TestEmptySearchListsACollectionOnceWithItsDescription(t *testing.T) {
	f := newDocumentFixture(t)
	_, note := f.search(map[string]any{"query": "zeppelin"})
	for _, want := range []string{
		"books/car-manuals (Owner and workshop manuals for our vehicles; 3 documents)",
		"books/WHO_Basic_Emergency_Care.pdf (WHO Basic Emergency Care)",
		"wiki",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("holdings %q do not contain %q", note, want)
		}
	}
	if strings.Count(note, "car-manuals") != 1 {
		t.Errorf("holdings name the collection more than once: %q", note)
	}
}

func TestVaultSourcesDescribesEachDocument(t *testing.T) {
	f := newDocumentFixture(t)
	// A book outside the index, and one whose name another collection shares.
	f.file("books/unindexed.pdf", "%PDF-1.4")
	f.file("books/tractor-manuals/trailer.pdf", "%PDF-1.4")
	meta, err := LoadMetadata(f.root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewVaultCapability(f.root, meta).Handle(context.Background(), "sources", map[string]any{"type": "book"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]inspect.SourceInfo{}
	for _, source := range result.Data.([]inspect.SourceInfo) {
		byPath[source.Path] = source
	}
	if len(byPath) != 6 {
		t.Fatalf("books = %v, want six, each with its own path", byPath)
	}
	pages := func(source inspect.SourceInfo) int {
		if source.PagesIndexed == nil {
			return -1
		}
		return *source.PagesIndexed
	}
	volvo := byPath["books/car-manuals/volvo-240.pdf"]
	if volvo.Title != "Volvo 240 Owner's Manual (1990)" || pages(volvo) != 3 || volvo.Note != "" ||
		volvo.Description != "Owner and workshop manuals for our vehicles" || !reflect.DeepEqual(volvo.Tags, []string{"vehicles", "repair"}) || volvo.Type != "book" {
		t.Errorf("volvo = %+v (pages %d)", volvo, pages(volvo))
	}
	if hmmwv := byPath["books/car-manuals/workshop/hmmwv.pdf"]; pages(hmmwv) != 1 || hmmwv.Note != "3 of 8 pages are scanned images without text" || hmmwv.Description != volvo.Description {
		t.Errorf("hmmwv = %+v (pages %d)", hmmwv, pages(hmmwv))
	}
	if trailer := byPath["books/car-manuals/trailer.pdf"]; pages(trailer) != 0 || trailer.Note != "scanned: no text layer, needs OCR" {
		t.Errorf("scanned trailer = %+v (pages %d), want 0 pages and the note", trailer, pages(trailer))
	}
	if who := byPath["books/WHO_Basic_Emergency_Care.pdf"]; who.Description != "WHO Basic Emergency Care (open-access triage)" || pages(who) != 1 {
		t.Errorf("who = %+v", who)
	}
	// The same file name in another folder is not this collection's document.
	if other := byPath["books/tractor-manuals/trailer.pdf"]; other.Description != "" || other.PagesIndexed != nil {
		t.Errorf("a same-named book of another folder = %+v, want no description", other)
	}
	if unindexed := byPath["books/unindexed.pdf"]; unindexed.PagesIndexed != nil || unindexed.Title != "" || unindexed.Description != "" {
		t.Errorf("unindexed book = %+v, want no index fields", unindexed)
	}
}
