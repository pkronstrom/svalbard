package zimext

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const documentsZIM = "testdata/documents.zim"

func TestExtractArticlesSkipsJunkAndTemplates(t *testing.T) {
	articles, _, _, err := ExtractArticlesContext(context.Background(), documentsZIM)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, article := range articles {
		paths = append(paths, article.Path)
	}
	slices.Sort(paths)
	if want := []string{"/Escaping_braces", "/Node.js", "/Novel.123", "/about", "/home"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestExtractArticlesReportsAnArchiveWithNothingIndexable(t *testing.T) {
	_, _, _, err := ExtractArticlesContext(context.Background(), "testdata/empty.zim")
	if !errors.Is(err, ErrNoIndexableArticles) {
		t.Fatalf("err = %v, want ErrNoIndexableArticles", err)
	}
}

func openDocuments(t *testing.T) *Archive {
	t.Helper()
	archive, err := OpenArchive(documentsZIM)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { archive.Close() })
	return archive
}

func TestDocumentsListsPDFsAndEPUBsWithoutDuplicates(t *testing.T) {
	documents, err := openDocuments(t).Documents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, doc := range documents {
		paths = append(paths, doc.Path)
		if doc.Size <= 0 {
			t.Errorf("%s: Size = %d", doc.Path, doc.Size)
		}
	}
	want := []string{"files/book.epub", "files/guide.pdf", "files/scan (1).pdf", "files/water.pdf"}
	if !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	if documents[0].MIME != "application/epub+zip" || documents[1].MIME != "application/pdf" {
		t.Fatalf("MIME types = %q, %q", documents[0].MIME, documents[1].MIME)
	}
}

func TestDocumentsTakeNautilusTitles(t *testing.T) {
	documents, err := openDocuments(t).Documents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]string{}
	for _, doc := range documents {
		titles[doc.Path] = doc.Title
	}
	want := map[string]string{
		"files/guide.pdf":    "Fixture Library, Part 2: Fruit and Fruit Products",
		"files/water.pdf":    "Water purification",
		"files/book.epub":    "",
		"files/scan (1).pdf": "",
	}
	for path, title := range want {
		if titles[path] != title {
			t.Errorf("%s: title = %q, want %q", path, titles[path], title)
		}
	}
}

func TestNautilusCatalogueThatCannotBeRead(t *testing.T) {
	for _, js := range []string{"", "var DATABASE = [ not a list", "{'ti': 'No file'}"} {
		if titles := parseNautilusTitles(js, "Library"); len(titles) != 0 {
			t.Errorf("parseNautilusTitles(%q) = %v, want none", js, titles)
		}
	}
}

func TestReadEntryEnforcesLimit(t *testing.T) {
	archive := openDocuments(t)
	want, err := os.ReadFile(filepath.Join("..", "pdfext", "testdata", "text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := archive.ReadEntry("files/guide.pdf", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ReadEntry returned %d bytes, want text.pdf's %d", len(got), len(want))
	}
	if _, err := archive.ReadEntry("files/guide.pdf", 10); !errors.Is(err, ErrEntryTooLarge) {
		t.Fatalf("err = %v, want ErrEntryTooLarge", err)
	}
}

func TestHoldsDocuments(t *testing.T) {
	if !openDocuments(t).HoldsDocuments() {
		t.Fatal("documents.zim: HoldsDocuments = false")
	}
	small, err := OpenArchive(testSmallZIMPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer small.Close()
	if small.HoldsDocuments() {
		t.Fatal("small.zim: HoldsDocuments = true")
	}
}

func TestJunkAndTemplateRules(t *testing.T) {
	for _, path := range []string{"vendors/x.css.map", "display_rows.handlebars", "a/b.js", "style.css", "data.json", "logo.svg", "m.wasm", "t.mustache"} {
		if !IsJunkEntry("text/plain", path) {
			t.Errorf("IsJunkEntry(text/plain, %q) = false", path)
		}
	}
	for _, path := range []string{"home", "Novel.123", "files/guide.pdf", "A/Water_purification"} {
		if IsJunkEntry("text/plain", path) {
			t.Errorf("IsJunkEntry(text/plain, %q) = true", path)
		}
	}
	// An article is never junk by its name: Wikipedia keeps "Node.js" there.
	for _, path := range []string{"Node.js", "questions/tagged/node.js", "D3.js", "Chart.json"} {
		if IsJunkEntry("text/html", path) {
			t.Errorf("IsJunkEntry(text/html, %q) = true", path)
		}
	}
	if !IsTemplateTitle("{{ title }}") || !IsTemplateTitle("  {{title}} ") {
		t.Fatal("a placeholder title is not a template")
	}
	for _, title := range []string{"Fixture Library", "How to escape {{ in Jinja", "{{ a }} and {{ b }} in Go templates"} {
		if IsTemplateTitle(title) {
			t.Errorf("IsTemplateTitle(%q) = true", title)
		}
	}
}

func TestNautilusSectionTitlesNameTheirArchive(t *testing.T) {
	js := "var DATABASE = [\n" +
		"{'_id': '00000', 'ti': 'Introduction', 'dsc': '', 'fp': ['INTRO.pdf']},\n" +
		"{'_id': '00001', 'ti': 'Appendix B', 'dsc': 'Altitude tables', 'fp': ['APPB.pdf']},\n" +
		"{'_id': '00002', 'ti': 'Introduction to knots', 'dsc': 'Basics', 'fp': ['knots.pdf']},\n];"
	titles := parseNautilusTitles(js, "USDA Guide to Home Canning")
	want := map[string]string{
		"INTRO.pdf": "USDA Guide to Home Canning, Introduction",
		"APPB.pdf":  "USDA Guide to Home Canning, Appendix B: Altitude tables",
		"knots.pdf": "Introduction to knots",
	}
	for file, title := range want {
		if titles[file] != title {
			t.Errorf("%s: %q, want %q", file, titles[file], title)
		}
	}
}

func TestNautilusTitlesThatAreSlugsOrNumbers(t *testing.T) {
	js := "var DATABASE = [\n" +
		"{'_id': '0', 'ti': 'Wilderness-Survival-FM-2176', 'dsc': '', 'fp': ['a.pdf']},\n" +
		"{'_id': '1', 'ti': '_173165', 'dsc': 'Nuclear Threats', 'fp': ['b.pdf']},\n" +
		"{'_id': '2', 'ti': 'fema_emergency- preparedness-guidelines', 'dsc': '', 'fp': ['c.pdf']},\n];"
	titles := parseNautilusTitles(js, "Post Disaster")
	if got := titles["a.pdf"]; got != "Wilderness Survival FM 2176" {
		t.Errorf("slug title = %q", got)
	}
	if got, ok := titles["b.pdf"]; ok {
		t.Errorf("a title without letters was kept: %q", got)
	}
	if got := titles["c.pdf"]; got != "fema_emergency- preparedness-guidelines" {
		t.Errorf("a title with spaces was changed: %q", got)
	}
}
