package pdfext

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/responses"
)

// One compiled module serves every test; compiling it takes about a second.
var testExtractor *Extractor

func TestMain(m *testing.M) {
	var err error
	if testExtractor, err = Open(); err != nil {
		panic(err)
	}
	code := m.Run()
	testExtractor.Close()
	os.Exit(code)
}

func extract(t *testing.T, name string) Doc {
	t.Helper()
	doc, err := testExtractor.Document(context.Background(), filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return doc
}

func TestDocumentReadsTextTitleAndSections(t *testing.T) {
	doc := extract(t, "text.pdf")
	if doc.Title != "Fixture Owner's Manual" {
		t.Errorf("Title = %q", doc.Title)
	}
	if len(doc.Pages) != 3 {
		t.Fatalf("pages = %d, want 3", len(doc.Pages))
	}
	first := doc.Pages[0]
	if first.Number != 1 || first.Section != "Cooling system" || first.Images != 0 {
		t.Errorf("page 1 = %+v", first)
	}
	if want := "Cooling system\nCoolant capacity incl. heater: 9.5 litres.\nUse only type C coolant and check the level\nwhen the engine is cold."; first.Text != want {
		t.Errorf("page 1 text = %q, want %q", first.Text, want)
	}
	// Finnish letters survive, and a page inside a section keeps its name.
	second := doc.Pages[1]
	if !strings.Contains(second.Text, "Jäähdytysnesteen määrä lämmityslaitteineen on 9,5 litraa.") || !strings.Contains(second.Text, "öljy") {
		t.Errorf("page 2 text = %q", second.Text)
	}
	if second.Section != "Cooling system" || doc.Pages[2].Section != "Brakes" {
		t.Errorf("sections = %q, %q", second.Section, doc.Pages[2].Section)
	}
	if note := Classify(doc); note != "" {
		t.Errorf("Classify = %q, want a fully searchable document", note)
	}
}

func TestDocumentReportsScannedPages(t *testing.T) {
	scan := extract(t, "scan.pdf")
	if len(scan.Pages) != 2 || !scan.Pages[0].ImageOnly() || scan.Pages[0].Text != "" || scan.Pages[0].Images != 1 {
		t.Fatalf("scan pages = %+v", scan.Pages)
	}
	if note := Classify(scan); note != "scanned: no text layer, needs OCR" {
		t.Errorf("Classify(scan) = %q", note)
	}

	mixed := extract(t, "mixed.pdf")
	if len(mixed.Pages) != 4 || mixed.Pages[0].ImageOnly() || !mixed.Pages[1].ImageOnly() {
		t.Fatalf("mixed pages = %+v", mixed.Pages)
	}
	if note := Classify(mixed); note != "2 of 4 pages are scanned images without text" {
		t.Errorf("Classify(mixed) = %q", note)
	}
}

func TestDocumentEncryption(t *testing.T) {
	// A user password blocks reading; an owner password only restricts
	// editing and printing, so the text is still read.
	if _, err := testExtractor.Document(context.Background(), filepath.Join("testdata", "enc-user.pdf")); !errors.Is(err, ErrPassword) {
		t.Fatalf("user-password file: err = %v, want ErrPassword", err)
	}
	doc := extract(t, "enc-owner.pdf")
	if len(doc.Pages) != 3 || !strings.Contains(doc.Pages[0].Text, "Coolant capacity") {
		t.Fatalf("owner-password file pages = %+v", doc.Pages)
	}
}

func TestDocumentRejectsFilesThatAreNotPDFs(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"page.pdf": "<html>Please sign in</html>", "empty.pdf": "", "cut.pdf": "%PDF-1.4\n1 0 obj\n<< /Type /Catalog"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := testExtractor.Document(context.Background(), path); err == nil || errors.Is(err, ErrPassword) {
			t.Errorf("%s: err = %v, want a read error", name, err)
		}
	}
	// A broken file does not spoil the extractor for the next one.
	if doc := extract(t, "text.pdf"); len(doc.Pages) != 3 {
		t.Fatalf("pages after broken files = %d", len(doc.Pages))
	}
}

func TestDocumentHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testExtractor.Document(ctx, filepath.Join("testdata", "text.pdf")); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if doc := extract(t, "text.pdf"); len(doc.Pages) != 3 {
		t.Fatalf("pages after a cancelled run = %d", len(doc.Pages))
	}
}

func TestClassify(t *testing.T) {
	text := Page{Text: strings.Repeat("word ", 30)}
	scan := Page{Images: 1}
	stamped := Page{Images: 1, Text: "Page 12"}
	figure := Page{Images: 2, Text: strings.Repeat("caption ", 10)}
	blank := Page{}
	cases := map[string]struct {
		pages []Page
		want  string
	}{
		"all text":                 {[]Page{text, text, figure}, ""},
		"blank separators":         {[]Page{text, blank, text, blank}, ""},
		"all scanned":              {[]Page{scan, stamped, scan}, "scanned: no text layer, needs OCR"},
		"scan with blank pages":    {[]Page{scan, blank, scan}, "scanned: no text layer, needs OCR"},
		"no text and no images":    {[]Page{blank, blank}, "no text found"},
		"a quarter scanned":        {[]Page{text, text, text, scan}, "1 of 4 pages are scanned images without text"},
		"fewer than a quarter":     {[]Page{text, text, text, text, scan}, ""},
		"illustrated pages":        {[]Page{figure, figure}, ""},
		"zero pages":               {nil, "no text found"},
		"mostly scanned with text": {[]Page{text, scan, scan}, "2 of 3 pages are scanned images without text"},
	}
	for name, tc := range cases {
		if got := Classify(Doc{Pages: tc.pages}); got != tc.want {
			t.Errorf("%s: Classify = %q, want %q", name, got, tc.want)
		}
	}
}

func TestCleanText(t *testing.T) {
	cases := map[string]struct{ raw, want string }{
		"line ends":       {"first line\nsecond  line\n", "first line\nsecond line"},
		"broken word":     {"the cool\uFFFEant level", "the coolant level"},
		"older marker":    {"the cool\x02\nant level", "the coolant level"},
		"number range":    {"doses of 10\uFFFE20 mg and type\x02 C", "doses of 10-20 mg and type-C"},
		"ocr hyphen":      {"the necessary calcu¬\nlations and co\u00AD\nolant, soft\u00ADhyphen", "the necessary calculations and coolant, softhyphen"},
		"paragraph break": {"one\n\n\n\ntwo", "one\n\ntwo"},
		"unmapped glyphs": {"a\uFFFF c\x00d", "a cd"},
		"only space":      {" \n\t\n", ""},
		"finnish":         {"Jäähdytysneste  ja öljy", "Jäähdytysneste ja öljy"},
	}
	for name, tc := range cases {
		if got := cleanText(tc.raw, buildVocabulary([]string{tc.raw})); got != tc.want {
			t.Errorf("%s: cleanText(%q) = %q, want %q", name, tc.raw, got, tc.want)
		}
	}
}

// A hyphen at a line end may belong to the word or only break it, and PDFium
// marks both alike. What the rest of the document writes decides, so neither
// "medications" nor "acting" is lost to search.
func TestBrokenWordsFollowTheDocumentsOwnSpelling(t *testing.T) {
	elsewhere := "Give medications by mouth. A longer course of acting on symptoms. Other long-acting drugs. " +
		"Formation of information. Tämä jäähdytysneste ja järjestelmä sekä linja-auto."
	cases := []struct{ raw, want, why string }{
		{"the medica\uFFFEtions", "the medications", "written unbroken elsewhere"},
		{"are longer\uFFFEacting", "are longer-acting", "two words of their own, never written as one"},
		{"a long\uFFFEacting agent", "a long-acting agent", "written with a hyphen elsewhere"},
		{"respira\uFFFEtory", "respiratory", "neither half is a word"},
		{"in\uFFFEformation", "information", "written unbroken elsewhere"},
		{"The Anglo\uFFFESaxon chart", "The Anglo-Saxon chart", "the second half is capitalised"},
		{"jäähdytys\uFFFEneste", "jäähdytysneste", "Finnish, written unbroken elsewhere"},
		{"linja\uFFFEauto", "linja-auto", "Finnish compound written with a hyphen elsewhere"},
		{"be\uFFFEacting", "beacting", "a short first half is a syllable"},
	}
	for _, tc := range cases {
		vocab := buildVocabulary([]string{tc.raw, elsewhere})
		if got := cleanText(tc.raw, vocab); got != tc.want {
			t.Errorf("cleanText(%q) = %q, want %q (%s)", tc.raw, got, tc.want, tc.why)
		}
	}
	// The halves of a broken word are not evidence that they are words.
	only := "medica\uFFFEtions and medica\uFFFEtions"
	if got := cleanText(only, buildVocabulary([]string{only})); got != "medications and medications" {
		t.Errorf("a word broken twice = %q", got)
	}
}

func TestDocumentJoinsWordsBrokenAcrossLines(t *testing.T) {
	doc := extract(t, "hyphens.pdf")
	if len(doc.Pages) != 1 {
		t.Fatalf("pages = %d", len(doc.Pages))
	}
	text := doc.Pages[0].Text
	for _, want := range []string{
		"medications are longer-acting and may need",
		"Repeat the medications as needed",
		"for respiratory depression",
		"use a long-acting agent, as with other long-acting drugs",
		"The Anglo-Saxon chart lists doses of 10-\n20 mg.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text misses %q:\n%s", want, text)
		}
	}
}

func TestPageSections(t *testing.T) {
	at := func(title string, page int, children ...responses.GetBookmarksBookmark) responses.GetBookmarksBookmark {
		return responses.GetBookmarksBookmark{Title: title, DestInfo: &responses.DestInfo{PageIndex: page}, Children: children}
	}
	outline := []responses.GetBookmarksBookmark{
		at("Engine", 1, at("Cooling", 1), at("Lubrication", 3)),
		{Title: "No destination"},
		at("Out of range", 99),
		at(strings.Repeat("long ", 40), 4),
	}
	got := pageSections(outline, 6)
	want := []string{"", "Engine", "Cooling", "Lubrication"}
	for page, section := range want {
		if got[page] != section {
			t.Errorf("page %d section = %q, want %q", page+1, got[page], section)
		}
	}
	if len([]rune(got[4])) != maxSectionRunes || got[5] != got[4] {
		t.Errorf("long title = %q (%d runes), next page %q", got[4], len([]rune(got[4])), got[5])
	}
	if pageSections(nil, 3) != nil {
		t.Error("a document without an outline got sections")
	}
}

// Cancelling in the middle of a document kills its instance from another
// goroutine. Document must not return before that is finished: the caller
// reads the next document, or closes the extractor, straight away.
func TestDocumentWaitsForTheKillOfACancelledInstance(t *testing.T) {
	extractor := testExtractor
	var started, finished atomic.Int32
	kill := killInstance
	killInstance = func(instance pdfium.Pdfium) {
		started.Add(1)
		time.Sleep(20 * time.Millisecond) // a kill that takes its time
		kill(instance)
		finished.Add(1)
	}
	t.Cleanup(func() { killInstance = kill })

	path := filepath.Join("testdata", "text.pdf")
	for i := 0; i < 400 && started.Load() < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		timer := time.AfterFunc(time.Duration(i%40)*100*time.Microsecond, cancel)
		_, err := extractor.Document(ctx, path)
		if s, f := started.Load(), finished.Load(); s != f {
			t.Fatalf("run %d: Document returned while a kill was still running (%d started, %d finished)", i, s, f)
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("run %d: err = %v, want success or context.Canceled", i, err)
		}
		timer.Stop()
		cancel()
		// The next document is read by a fresh instance.
		if doc, err := extractor.Document(context.Background(), path); err != nil || len(doc.Pages) != 3 {
			t.Fatalf("run %d: document after a cancelled one = %d pages, %v", i, len(doc.Pages), err)
		}
	}
	if started.Load() == 0 {
		t.Skip("no cancellation landed in the middle of a document")
	}
}

func TestDocumentFromBytesMatchesDocument(t *testing.T) {
	path := filepath.Join("testdata", "text.pdf")
	fromFile, err := testExtractor.Document(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fromBytes, err := testExtractor.DocumentFrom(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if fromBytes.Title != fromFile.Title || len(fromBytes.Pages) != len(fromFile.Pages) {
		t.Fatalf("from bytes: %q with %d pages, from file: %q with %d pages", fromBytes.Title, len(fromBytes.Pages), fromFile.Title, len(fromFile.Pages))
	}
	for i := range fromFile.Pages {
		if fromBytes.Pages[i] != fromFile.Pages[i] {
			t.Fatalf("page %d differs: %+v vs %+v", i+1, fromBytes.Pages[i], fromFile.Pages[i])
		}
	}
}
