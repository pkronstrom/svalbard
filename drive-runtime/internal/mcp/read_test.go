package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

// readFixture is a drive with one indexed archive, wiki.zim, and a stand-in
// kiwix-serve that serves the pages added with page().
type readFixture struct {
	t     *testing.T
	root  string
	pages string
	db    *sql.DB
	cap   *SearchCapability
}

func newReadFixture(t *testing.T, withKiwix bool) *readFixture {
	t.Helper()
	root := searchFixtureDrive(t)
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pages := t.TempDir()
	if withKiwix {
		platformName, err := platform.Detect()
		if err != nil {
			t.Skip(err)
		}
		binDir := filepath.Join(root, "bin", platformName)
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(os.Args[0], filepath.Join(binDir, "kiwix-serve")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("SVALBARD_TEST_MCP_KIWIX", "1")
		t.Setenv("SVALBARD_TEST_MCP_KIWIX_PAGES", pages)
	} else {
		// No kiwix-serve on the drive, and none found on the host either.
		t.Setenv("PATH", t.TempDir())
	}
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	capability := NewSearchCapability(root, DriveMetadata{})
	t.Cleanup(func() { _ = capability.Close() })
	return &readFixture{t: t, root: root, pages: pages, db: db, cap: capability}
}

// page makes kiwix serve html at path (no leading slash) and indexes the
// article under "/"+path with the first 40 characters of text as its body.
func (f *readFixture) page(path, title, html, indexedBody string) {
	f.t.Helper()
	dir := filepath.Join(f.pages, "wiki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, url.PathEscape(path)), []byte(html), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.index(path, title, indexedBody)
}

func (f *readFixture) index(path, title, body string) {
	f.t.Helper()
	res, err := f.db.Exec(`INSERT INTO articles (source_id, path, title, body) VALUES (1, ?, ?, ?)`, "/"+path, title, body)
	if err != nil {
		f.t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := f.db.Exec(`INSERT INTO articles_fts(rowid, title, body) VALUES (?, ?, ?)`, id, title, body); err != nil {
		f.t.Fatal(err)
	}
}

func (f *readFixture) read(params map[string]any) (readResult, error) {
	f.t.Helper()
	result, err := f.cap.Handle(context.Background(), "read", params)
	if err != nil {
		return readResult{}, err
	}
	return result.Data.(readResult), nil
}

func article(title string, paragraphs ...string) string {
	var body strings.Builder
	for _, paragraph := range paragraphs {
		body.WriteString("<p>" + paragraph + "</p>")
	}
	return "<html><head><title>" + title + "</title></head><body>" + body.String() + "</body></html>"
}

func TestReadReturnsTheWholeArticleInPieces(t *testing.T) {
	f := newReadFixture(t, true)
	var paragraphs []string
	for i := 0; i < 150; i++ {
		paragraphs = append(paragraphs, fmt.Sprintf("Paragraph %03d of the article, long enough to fill a line of about a hundred characters or so.", i))
	}
	f.page("Long_article", "Long article", article("Long article", paragraphs...), "Paragraph 000")

	var pieces []string
	offset, calls := 0, 0
	for {
		params := map[string]any{"source": "wiki", "path": "/Long_article"}
		if calls > 0 {
			params["offset"] = float64(offset)
		}
		got, err := f.read(params)
		if err != nil {
			t.Fatal(err)
		}
		calls++
		if n := len([]rune(got.Body)); n > readChars {
			t.Fatalf("piece %d has %d characters, want at most %d", calls, n, readChars)
		}
		if !strings.HasPrefix(got.Body, "Paragraph") || !strings.HasSuffix(got.Body, "so.") {
			t.Fatalf("piece %d is not cut at paragraph breaks: %q ... %q", calls, got.Body[:20], got.Body[len(got.Body)-20:])
		}
		pieces = append(pieces, got.Body)
		if got.Continues == "" {
			break
		}
		// The line names the next offset; a model follows it literally.
		if _, err := fmt.Sscanf(got.Continues[strings.Index(got.Continues, "offset "):], "offset %d", &offset); err != nil {
			t.Fatalf("continues = %q does not name the next offset: %v", got.Continues, err)
		}
		if calls > 10 {
			t.Fatal("paging does not end")
		}
	}
	if calls < 3 {
		t.Fatalf("a %d-character article came in %d pieces, want several", 150*98, calls)
	}
	if got, want := strings.Join(pieces, "\n"), strings.Join(paragraphs, "\n"); got != want {
		t.Fatalf("pieces do not add up to the article: got %d characters, want %d", len(got), len(want))
	}

	if _, err := f.read(map[string]any{"source": "wiki", "path": "/Long_article", "offset": float64(1_000_000)}); err == nil || !strings.Contains(err.Error(), "past the end") {
		t.Fatalf("offset past the end: err = %v", err)
	}
}

func TestReadToleratesNearMissSourcesAndPaths(t *testing.T) {
	f := newReadFixture(t, true)
	f.page("Water_purification", "Water purification", article("Water purification", "Boil it."), "Boil it.")
	f.page("www.example.org/medicines/aspirin/", "Aspirin", article("Aspirin", "With food."), "With food.")
	f.page("What is it?", "What is it?", article("What is it?", "A question."), "A question.")

	for _, tc := range []struct {
		source, path, want string
	}{
		{"wiki", "/Water_purification", "/Water_purification"},
		{"wiki", "Water_purification", "/Water_purification"},
		{"wiki.zim", "//Water_purification/", "/Water_purification"},
		{"WIKI", "Water purification", "/Water_purification"},
		{"wik", "Water%20purification", "/Water_purification"},
		{"wiki", "water_PURIFICATION", "/Water_purification"},
		{"wiki", "/Water_purification#Boiling", "/Water_purification"},
		{"wiki", "/www.example.org/medicines/aspirin", "/www.example.org/medicines/aspirin/"},
		{"wiki", "https://www.example.org/medicines/aspirin/", "/www.example.org/medicines/aspirin/"},
		// Without its leading directories, when only one article ends so.
		{"wiki", "/aspirin", "/www.example.org/medicines/aspirin/"},
		// A path with characters that mean something in a URL.
		{"wiki", "/What is it?", "/What is it?"},
		{"wiki", "What%20is%20it%3F", "/What is it?"},
	} {
		got, err := f.read(map[string]any{"source": tc.source, "path": tc.path})
		if err != nil {
			t.Errorf("read(%q, %q) error = %v", tc.source, tc.path, err)
			continue
		}
		if got.Path != tc.want || got.Source != "wiki" || got.Body == "" || got.Note != "" {
			t.Errorf("read(%q, %q) = source %q path %q body %q note %q, want the full page at %q", tc.source, tc.path, got.Source, got.Path, got.Body, got.Note, tc.want)
		}
	}
}

func TestReadNamesNearestPathsWhenNothingResolves(t *testing.T) {
	f := newReadFixture(t, true)
	f.page("Shelter_building", "Shelter building", article("Shelter building", "Start with the frame."), "Start with the frame.")
	f.page("Shelter_materials", "Shelter materials", article("Shelter materials", "Branches and tarp."), "Branches and tarp.")
	f.page("Fire", "Fire", article("Fire", "Tinder first."), "Tinder first.")

	_, err := f.read(map[string]any{"source": "wiki", "path": "/Shelter"})
	if err == nil {
		t.Fatal("read of a path that only resembles two articles succeeded")
	}
	for _, want := range []string{`no "/Shelter"`, "/Shelter_building", "/Shelter_materials"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "/Fire") {
		t.Errorf("error %q offers an unrelated path", err)
	}

	// Nothing spelled alike: the words of the path are searched instead.
	_, err = f.read(map[string]any{"source": "wiki", "path": "/building_a_shelter"})
	if err == nil || !strings.Contains(err.Error(), "/Shelter_building") {
		t.Errorf("error = %v, want the article about the same words offered", err)
	}

	_, err = f.read(map[string]any{"source": "wiki", "path": "/Toyota"})
	if err == nil || !strings.Contains(err.Error(), "Use search") {
		t.Errorf("error = %v, want advice to search when nothing is near", err)
	}

	_, err = f.read(map[string]any{"source": "maps", "path": "/Water"})
	if err == nil || !strings.Contains(err.Error(), `unknown source "maps"`) || !strings.Contains(err.Error(), "wiki") {
		t.Errorf("error = %v, want the drive's sources named", err)
	}
}

func TestReadResolvesAndCapsLinks(t *testing.T) {
	f := newReadFixture(t, true)
	var links strings.Builder
	links.WriteString(`<nav><a href="/content/wiki/Site_menu">Menu</a></nav>`)
	links.WriteString(`<a href="../warfarin/">warfarin</a> <a href="dose/">dose</a> <a href="#top">top</a> <a href="./">here</a>`)
	links.WriteString(`<a href="https://example.com/">out</a> <a href="../../logo.png">logo</a> <a href="../warfarin/#uses">warfarin again</a>`)
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&links, `<a href="../drug-%02d/">Drug %d</a>`, i, i)
	}
	f.page("site/medicines/aspirin/", "Aspirin", article("Aspirin", "With food. "+links.String()), "With food.")
	f.page("site/medicines/warfarin/", "Warfarin", article("Warfarin", "A blood thinner."), "A blood thinner.")

	got, err := f.read(map[string]any{"source": "wiki", "path": "/site/medicines/aspirin/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != readLinks {
		t.Fatalf("links = %d, want the cap of %d", len(got.Links), readLinks)
	}
	if got.Links[0].Path != "/site/medicines/warfarin/" || got.Links[1].Path != "/site/medicines/aspirin/dose/" {
		t.Fatalf("links = %+v, want paths resolved against the page, navigation and self links left out", got.Links[:3])
	}
	if !strings.Contains(got.Note, "12 more links") {
		t.Errorf("note = %q, want the number of links left out", got.Note)
	}
	// A listed link reads as it is.
	if linked, err := f.read(map[string]any{"source": "wiki", "path": got.Links[0].Path}); err != nil || linked.Title != "Warfarin" {
		t.Errorf("reading a listed link: %+v, %v", linked, err)
	}
}

func TestReadWithoutPathOpensTheMainPage(t *testing.T) {
	f := newReadFixture(t, true)
	f.page("Main_Page", "Main Page", article("Main Page", `Welcome. <a href="Fire">Fire</a>`), "Welcome.")
	if err := os.WriteFile(filepath.Join(f.pages, "wiki", ".main"), []byte("Main_Page"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := f.read(map[string]any{"source": "wiki"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/Main_Page" || !strings.Contains(got.Body, "Welcome.") || len(got.Links) != 1 || got.Links[0].Path != "/Fire" {
		t.Fatalf("main page = %+v", got)
	}
}

func TestReadFallsBackToTheIndexedExtractAndSaysSo(t *testing.T) {
	for _, withKiwix := range []bool{true, false} {
		t.Run(fmt.Sprintf("kiwix=%v", withKiwix), func(t *testing.T) {
			f := newReadFixture(t, withKiwix)
			// Indexed, but kiwix has no such page.
			f.index("Only_indexed", "Only indexed", "The index&#x27;s extract.")

			got, err := f.read(map[string]any{"source": "wiki", "path": "only_indexed"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Body != "The index's extract." || got.Title != "Only indexed" || got.Path != "/Only_indexed" {
				t.Fatalf("read = %+v, want the indexed article", got)
			}
			if !strings.Contains(got.Note, "indexed extract") {
				t.Errorf("note = %q, want it to say the text is the index's extract", got.Note)
			}
		})
	}
}

func TestReadReturnsCancellation(t *testing.T) {
	f := newReadFixture(t, true)
	f.page("Fire", "Fire", article("Fire", "Tinder first."), "Tinder first.")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.cap.Handle(ctx, "read", map[string]any{"source": "wiki", "path": "/Fire"}); err == nil {
		t.Fatal("read with a cancelled context returned no error")
	}
}

func TestSearchFallsBackToPartialMatchesAndSaysSo(t *testing.T) {
	f := newReadFixture(t, false)
	f.index("Desalination", "Desalination", "Removing salt from sea water by distillation.")
	f.index("Distillation", "Distillation", "A still separates liquids > 78 degrees.")
	server := NewServer(f.cap)
	call := func(arguments map[string]any) (string, string) {
		t.Helper()
		result, err := server.makeHandler(f.cap, "search")(context.Background(), gomcp.CallToolRequest{Params: gomcp.CallToolParams{Arguments: arguments}})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError {
			t.Fatalf("search error: %#v", result.Content)
		}
		advisory := ""
		if len(result.Content) > 1 {
			advisory = result.Content[1].(gomcp.TextContent).Text
		}
		return result.Content[0].(gomcp.TextContent).Text, advisory
	}

	raw, advisory := call(map[string]any{"query": "sea water distillation by sunlight"})
	var items []searchResultItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("results are not JSON: %v\n%s", err, raw)
	}
	if len(items) == 0 || items[0].Path != "/Desalination" || !slices.Equal(items[0].Matched, []string{"sea", "water", "distillation"}) {
		t.Fatalf("results = %+v, want the article with most of the words first, naming them", items)
	}
	if advisory != `No article has all of the words. No article on the drive mentions: sunlight. These have most of them; "matched" lists which.` {
		t.Errorf("advisory = %q, want it to say the results are partial matches and which word is nowhere", advisory)
	}

	// A complete match needs no advice, and "<" and ">" arrive unescaped.
	raw, advisory = call(map[string]any{"query": "still liquids"})
	if advisory != "" || !strings.Contains(raw, "> 78") {
		t.Errorf("results = %s, advisory = %q", raw, advisory)
	}

	raw, advisory = call(map[string]any{"query": "tyre pressure"})
	if raw != "[]" || advisory != "Nothing found. No article on the drive mentions: tyre, pressure." {
		t.Errorf("results = %s, advisory = %q, want an empty list and the absent words", raw, advisory)
	}
	// Both words exist, in different articles.
	if raw, advisory = call(map[string]any{"query": "salt degrees", "source": "wiki"}); raw != "[]" || advisory != "Nothing found: no article in wiki has these words together." {
		t.Errorf("results = %s, advisory = %q", raw, advisory)
	}

	if _, err := f.cap.Handle(context.Background(), "search", map[string]any{"query": "water", "source": "maps"}); err == nil || !strings.Contains(err.Error(), `unknown source "maps"`) {
		t.Errorf("search of an unknown source: err = %v", err)
	}
	if result, err := f.cap.Handle(context.Background(), "search", map[string]any{"query": "salt", "source": "wiki.zim"}); err != nil || len(result.Data.([]searchResultItem)) != 1 {
		t.Errorf("search restricted to wiki.zim = %+v, %v", result.Data, err)
	}
}

// The defaults of both tools have to leave a 16K-context model room to work:
// a search and a read together stay near 2,500 tokens.
func TestDefaultResponsesStayWithinASmallContext(t *testing.T) {
	f := newReadFixture(t, true)
	long := strings.Repeat("Water treatment words fill this article to the length the index keeps. ", 55)
	var paragraphs []string
	for i := 0; i < 400; i++ {
		paragraphs = append(paragraphs, "Water treatment paragraph with a link to <a href=\"Topic_"+fmt.Sprint(i)+"\">another topic of some length</a>.")
	}
	for i := 0; i < 40; i++ {
		f.page(fmt.Sprintf("www.example.org/a/rather/long/path/to/Water_treatment_%02d/", i), "Water treatment methods and their long title", article("Water treatment", paragraphs...), long)
	}
	size := func(action string, params map[string]any) int {
		t.Helper()
		result, err := f.cap.Handle(context.Background(), action, params)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(result.Data)
		if err != nil {
			t.Fatal(err)
		}
		return len(raw) + len(result.Text)
	}

	if n := size("search", map[string]any{"query": "water treatment"}); n > 3000 {
		t.Errorf("default search response is %d characters, want at most 3000", n)
	}
	if n := size("search", map[string]any{"query": "water treatment", "detail": "full"}); n > 9500 {
		t.Errorf("full-detail search response is %d characters, want at most 9500", n)
	}
	if n := size("read", map[string]any{"source": "wiki", "path": "/www.example.org/a/rather/long/path/to/Water_treatment_00/"}); n > 9000 {
		t.Errorf("default read response is %d characters, want at most 9000", n)
	}
}

func TestPathCandidates(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []string
	}{
		{"", []string{""}},
		{"/Lyme_disease", []string{"Lyme_disease", "Lyme_disease/"}},
		{"Lyme disease/", []string{"Lyme disease/", "Lyme disease", "Lyme_disease/", "Lyme_disease"}},
		{"https://www.nhs.uk/medicines/x/", []string{"https://www.nhs.uk/medicines/x/", "https://www.nhs.uk/medicines/x", "www.nhs.uk/medicines/x/", "www.nhs.uk/medicines/x"}},
	} {
		if got := pathCandidates(tc.raw); !slices.Equal(got, tc.want) {
			t.Errorf("pathCandidates(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
	// The path as given always comes first: it is what an exact caller means.
	if got := pathCandidates("a%20b#c?d"); got[0] != "a%20b#c?d" || !slices.Contains(got, "a b") || !slices.Contains(got, "a_b") {
		t.Errorf("pathCandidates = %q", got)
	}
}

func TestPageSlice(t *testing.T) {
	body := strings.Repeat("ääää ", 30) // 150 characters, 270 bytes
	piece, next, total := pageSlice(body, 0, 100)
	if total != 150 || next != 100 || len([]rune(piece)) != 99 {
		t.Fatalf("first piece: %d characters, next %d, total %d", len([]rune(piece)), next, total)
	}
	piece, next, _ = pageSlice(body, next, 100)
	if next != 150 || len([]rune(piece)) != 49 {
		t.Fatalf("last piece: %d characters, next %d", len([]rune(piece)), next)
	}
	// No break near the end of the piece: cut at the size, never mid-character.
	if piece, next, _ := pageSlice(strings.Repeat("ä", 300), 0, 100); next != 100 || piece != strings.Repeat("ä", 100) {
		t.Fatalf("unbroken text: next %d", next)
	}
	if piece, next, total := pageSlice("short", 9, 100); piece != "" || next != 5 || total != 5 {
		t.Fatalf("past the end: %q, %d, %d", piece, next, total)
	}
}
