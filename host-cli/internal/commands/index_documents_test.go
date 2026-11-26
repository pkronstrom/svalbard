package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

// fakeDocumentReader stands in for PDFium: it returns the text registered for
// a file name and records every file it was asked to read.
type fakeDocumentReader struct {
	texts  map[string]documentText
	reads  []string
	opens  int
	onRead func(name string)
}

func (r *fakeDocumentReader) Read(ctx context.Context, path string) (documentText, error) {
	name := filepath.Base(path)
	r.reads = append(r.reads, name)
	if r.onRead != nil {
		r.onRead(name)
	}
	if err := ctx.Err(); err != nil {
		return documentText{}, err
	}
	return r.texts[name], nil
}

func (r *fakeDocumentReader) Close() error { return nil }

// takeReads returns the files read since the last call.
func (r *fakeDocumentReader) takeReads() []string {
	reads := r.reads
	r.reads = nil
	return reads
}

func pages(texts ...string) documentText {
	var doc documentText
	for i, text := range texts {
		doc.Parts = append(doc.Parts, documentPart{Path: "page/" + string(rune('1'+i)), Label: "p. " + string(rune('1'+i)), Text: text})
	}
	return doc
}

func useFakeDocumentReader(t *testing.T, texts map[string]documentText) *fakeDocumentReader {
	t.Helper()
	reader := &fakeDocumentReader{texts: texts}
	old := documentKinds
	documentKinds = map[string]documentKind{".pdf": {unit: "page", open: func() (documentReader, error) {
		reader.opens++
		return reader, nil
	}}}
	t.Cleanup(func() { documentKinds = old })
	return reader
}

func writeVaultFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// documentsVault is a vault that has never held a ZIM archive: a manifest
// and no zim/ directory.
func documentsVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeVaultFile(t, root, "manifest.yaml", "version: 2\nrealized:\n  entries: []\n")
	return root
}

func touchLater(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	later := info.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
}

func openIndex(t *testing.T, root string) *searchdb.DB {
	t.Helper()
	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// indexedCounts maps each indexed source to its number of articles.
func indexedCounts(t *testing.T, root string) map[string]int64 {
	t.Helper()
	sources, err := openIndex(t, root).Sources()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, source := range sources {
		counts[source.Filename] = source.ArticleCount
	}
	return counts
}

func indexedSource(t *testing.T, root, filename string) searchdb.Source {
	t.Helper()
	sources, err := openIndex(t, root).IndexedSources()
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if source.Filename == filename {
			return source
		}
	}
	t.Fatalf("source %s is not indexed; have %+v", filename, sources)
	return searchdb.Source{}
}

func index(t *testing.T, root string) string {
	t.Helper()
	var out bytes.Buffer
	if err := IndexVault(root, false, &out, nil); err != nil {
		t.Fatalf("IndexVault: %v\n%s", err, out.String())
	}
	return out.String()
}

func TestIndexDocumentsOnlyVaultWithoutZIMDirectory(t *testing.T) {
	root := t.TempDir()
	writeVaultFile(t, root, "manifest.yaml", "version: 2\nrealized:\n  entries:\n    - id: manuals\n      type: pdf\n      relative_path: books/manuals/volvo.pdf\n")
	writeVaultFile(t, root, "books/manuals/volvo.pdf", "volvo")
	writeVaultFile(t, root, "books/manuals/workshop/engine/b20.pdf", "b20")
	writeVaultFile(t, root, "books/manuals/._volvo.pdf", "macOS sidecar")
	writeVaultFile(t, root, "books/manuals/notes.epub", "not an indexed kind")
	writeVaultFile(t, root, ".svalbard/documents.json", `{"books/manuals/volvo.pdf": {"title": "Volvo 240 Owner's Manual", "collection": "manuals"}}`)
	reader := useFakeDocumentReader(t, map[string]documentText{
		"volvo.pdf": {Title: "volvo240_final.indd", Parts: []documentPart{
			{Path: "page/1", Label: "p. 1", Text: "Introduction\nRead this manual."},
			{Path: "page/112", Label: "p. 112", Section: "Cooling system", Text: "Coolant capacity incl. heater:\n9.5 litres."},
		}},
		"b20.pdf": {Title: "B20 Engine", Parts: []documentPart{{Path: "page/1", Label: "p. 1", Section: "B20 engine", Text: "Valve clearance 0.40 mm"}}},
	})

	out := index(t, root)
	want := "Indexing documents:\n" +
		"  manuals/volvo.pdf                2 pages\n" +
		"  manuals/workshop/engine/b20.pdf  1 page\n"
	if out != want {
		t.Fatalf("report = %q, want %q", out, want)
	}
	if got, want := reader.takeReads(), []string{"volvo.pdf", "b20.pdf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files read = %v, want %v", got, want)
	}

	volvo := indexedSource(t, root, "books/manuals/volvo.pdf")
	if volvo.Title != "Volvo 240 Owner's Manual" || !strings.HasPrefix(volvo.UUID, "sha256:") || volvo.Note != "" {
		t.Errorf("volvo source = %+v", volvo)
	}
	// A file's own title is used when the collection gives none.
	if b20 := indexedSource(t, root, "books/manuals/workshop/engine/b20.pdf"); b20.Title != "B20 Engine" {
		t.Errorf("b20 title = %q", b20.Title)
	}
	hits, err := openIndex(t, root).Search("coolant", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits = %+v, %v", hits, err)
	}
	if hits[0].Path != "page/112" || hits[0].Title != "Volvo 240 Owner's Manual, p. 112 · Cooling system" || hits[0].Body != "Coolant capacity incl. heater: 9.5 litres." {
		t.Errorf("hit = %+v", hits[0])
	}
	// The section equal to the document title is not repeated.
	if hits, _ := openIndex(t, root).Search("valve", 10); len(hits) != 1 || hits[0].Title != "B20 Engine, p. 1" {
		t.Errorf("b20 hit = %+v", hits)
	}
}

func TestIndexDocumentsSkipsUnchangedAndReadsTouched(t *testing.T) {
	root := documentsVault(t)
	a := writeVaultFile(t, root, "books/manuals/a.pdf", "a")
	writeVaultFile(t, root, "books/manuals/b.pdf", "b")
	reader := useFakeDocumentReader(t, map[string]documentText{"a.pdf": pages("alpha text"), "b.pdf": pages("bravo text", "more bravo")})
	index(t, root)
	if reads := reader.takeReads(); len(reads) != 2 {
		t.Fatalf("first run read %v", reads)
	}

	// Nothing changed: no file is opened, the reader is not even started.
	opens := reader.opens
	if out := index(t, root); out != "" {
		t.Errorf("unchanged run reported %q", out)
	}
	if reads := reader.takeReads(); len(reads) != 0 || reader.opens != opens {
		t.Fatalf("unchanged run read %v (reader started %d more times)", reads, reader.opens-opens)
	}

	// Only the touched file is read again.
	touchLater(t, a)
	reader.texts["a.pdf"] = pages("alpha text, revised", "a second page")
	out := index(t, root)
	if want := "Indexing documents:\n  manuals/a.pdf  2 pages\n  1 document unchanged\n"; out != want {
		t.Errorf("report = %q, want %q", out, want)
	}
	if reads := reader.takeReads(); !reflect.DeepEqual(reads, []string{"a.pdf"}) {
		t.Fatalf("after touching a.pdf read %v, want only a.pdf", reads)
	}
	if got, want := indexedCounts(t, root), map[string]int64{"books/manuals/a.pdf": 2, "books/manuals/b.pdf": 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("indexed = %v, want %v", got, want)
	}

	// A title given in the collection file later reaches the index although
	// the file itself is unchanged.
	writeVaultFile(t, root, ".svalbard/documents.json", `{"books/manuals/b.pdf": {"title": "Bravo Manual", "collection": "manuals"}}`)
	index(t, root)
	if reads := reader.takeReads(); !reflect.DeepEqual(reads, []string{"b.pdf"}) {
		t.Fatalf("after a title change read %v, want only b.pdf", reads)
	}
	if hits, _ := openIndex(t, root).Search("bravo", 1); len(hits) != 1 || !strings.HasPrefix(hits[0].Title, "Bravo Manual, p. ") {
		t.Fatalf("hit after title change = %+v", hits)
	}
}

func TestIndexDocumentsAndArchivesChangeIndependently(t *testing.T) {
	root := t.TempDir()
	zim := filepath.Join(root, "zim", "wiki.zim")
	if err := os.MkdirAll(filepath.Dir(zim), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, testSmallZIMPath(t), zim)
	doc := writeVaultFile(t, root, "books/manuals/a.pdf", "a")
	reader := useFakeDocumentReader(t, map[string]documentText{"a.pdf": pages("alpha text")})
	index(t, root)
	before := indexedCounts(t, root)
	if before["wiki.zim"] == 0 || before["books/manuals/a.pdf"] != 1 || len(before) != 2 {
		t.Fatalf("indexed = %v, want the archive and the document", before)
	}
	reader.takeReads()

	// A changed archive leaves the document alone.
	touchLater(t, zim)
	var refreshed []string
	if err := IndexVault(root, false, io.Discard, func(p IndexProgress) {
		if p.Status == "done" {
			refreshed = append(refreshed, p.File)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(refreshed, []string{"wiki.zim"}) || len(reader.takeReads()) != 0 {
		t.Fatalf("after touching the archive refreshed %v", refreshed)
	}
	// A changed document leaves the archive alone.
	touchLater(t, doc)
	refreshed = nil
	if err := IndexVault(root, false, io.Discard, func(p IndexProgress) {
		if p.Status == "done" {
			refreshed = append(refreshed, p.File)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(refreshed, []string{"books/manuals/a.pdf"}) {
		t.Fatalf("after touching the document refreshed %v", refreshed)
	}
	if after := indexedCounts(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("indexed = %v, want %v", after, before)
	}

	// A removed document is pruned; the archive stays. Then the reverse.
	if err := os.Remove(doc); err != nil {
		t.Fatal(err)
	}
	index(t, root)
	if got := indexedCounts(t, root); len(got) != 1 || got["wiki.zim"] != before["wiki.zim"] {
		t.Fatalf("after removing the document indexed = %v", got)
	}
	writeVaultFile(t, root, "books/manuals/a.pdf", "a")
	if err := os.Remove(zim); err != nil {
		t.Fatal(err)
	}
	index(t, root)
	if got := indexedCounts(t, root); !reflect.DeepEqual(got, map[string]int64{"books/manuals/a.pdf": 1}) {
		t.Fatalf("after removing the archive indexed = %v", got)
	}
}

func TestIndexMissingBooksDirectoryIsNeverAPrune(t *testing.T) {
	// Realized documents but no books/: the vault is not all there.
	recorded := t.TempDir()
	writeVaultFile(t, recorded, "manifest.yaml", "version: 2\nrealized:\n  entries:\n    - id: manuals\n      type: pdf\n      relative_path: books/manuals/a.pdf\n")
	if err := os.MkdirAll(filepath.Join(recorded, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	useFakeDocumentReader(t, map[string]documentText{"a.pdf": pages("alpha text")})
	if err := IndexVault(recorded, false, io.Discard, nil); err == nil || !strings.Contains(err.Error(), "books") {
		t.Fatalf("err = %v, want the missing books directory", err)
	}

	// Indexed documents but no books/, whatever the manifest says.
	indexed := t.TempDir()
	writeVaultFile(t, indexed, "manifest.yaml", "version: 2\nrealized:\n  entries: []\n")
	writeVaultFile(t, indexed, "books/manuals/a.pdf", "a")
	if err := os.MkdirAll(filepath.Join(indexed, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	index(t, indexed)
	if err := os.RemoveAll(filepath.Join(indexed, "books")); err != nil {
		t.Fatal(err)
	}
	if err := IndexVault(indexed, false, io.Discard, nil); err == nil || !strings.Contains(err.Error(), "not pruning") {
		t.Fatalf("err = %v, want a refusal to prune", err)
	}
	if got := indexedCounts(t, indexed); got["books/manuals/a.pdf"] != 1 {
		t.Fatalf("indexed after the refused run = %v, want the document kept", got)
	}

	// The same guard for archives on a vault that also holds documents.
	archives := t.TempDir()
	writeVaultFile(t, archives, "manifest.yaml", "version: 2\nrealized:\n  entries: []\n")
	writeVaultFile(t, archives, "books/manuals/a.pdf", "a")
	zim := filepath.Join(archives, "zim", "wiki.zim")
	if err := os.MkdirAll(filepath.Dir(zim), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, testSmallZIMPath(t), zim)
	index(t, archives)
	if err := os.RemoveAll(filepath.Join(archives, "zim")); err != nil {
		t.Fatal(err)
	}
	if err := IndexVault(archives, false, io.Discard, nil); err == nil || !strings.Contains(err.Error(), "not pruning") {
		t.Fatalf("err = %v, want a refusal to prune archives", err)
	}
}

func TestIndexDocumentsRecordsUnsearchableFilesOnce(t *testing.T) {
	root := documentsVault(t)
	writeVaultFile(t, root, "books/manuals/trailer.pdf", "scan")
	writeVaultFile(t, root, "books/manuals/secret.pdf", "locked")
	writeVaultFile(t, root, "books/manuals/wiring.pdf", "half scanned")
	writeVaultFile(t, root, "books/manuals/volvo.pdf", "text")
	partly := pages("fuse table")
	partly.Note = "3 of 4 pages are scanned images without text"
	reader := useFakeDocumentReader(t, map[string]documentText{
		"trailer.pdf": {Note: "scanned: no text layer, needs OCR"},
		"secret.pdf":  {Note: "password-protected"},
		"wiring.pdf":  partly,
		"volvo.pdf":   pages("coolant"),
	})

	out := index(t, root)
	want := "Indexing documents:\n" +
		"  manuals/secret.pdf   not searchable: password-protected\n" +
		"  manuals/trailer.pdf  not searchable: scanned: no text layer, needs OCR\n" +
		"  manuals/volvo.pdf    1 page\n" +
		"  manuals/wiring.pdf   1 page; 3 of 4 pages are scanned images without text\n"
	if out != want {
		t.Fatalf("report = %q, want %q", out, want)
	}
	counts := indexedCounts(t, root)
	if len(counts) != 4 || counts["books/manuals/trailer.pdf"] != 0 || counts["books/manuals/secret.pdf"] != 0 || counts["books/manuals/wiring.pdf"] != 1 {
		t.Fatalf("indexed = %v, want every file recorded as a source", counts)
	}
	if note := indexedSource(t, root, "books/manuals/trailer.pdf").Note; note != "scanned: no text layer, needs OCR" {
		t.Errorf("trailer note = %q", note)
	}
	if note := indexedSource(t, root, "books/manuals/secret.pdf").Note; note != "password-protected" {
		t.Errorf("secret note = %q", note)
	}
	reader.takeReads()

	// The files are not read again, but what is wrong with them is still said.
	out = index(t, root)
	want = "Indexing documents:\n" +
		"  manuals/secret.pdf   not searchable: password-protected\n" +
		"  manuals/trailer.pdf  not searchable: scanned: no text layer, needs OCR\n" +
		"  manuals/wiring.pdf   1 page; 3 of 4 pages are scanned images without text\n" +
		"  1 document unchanged\n"
	if out != want {
		t.Fatalf("second report = %q, want %q", out, want)
	}
	if reads := reader.takeReads(); len(reads) != 0 {
		t.Fatalf("unsearchable files were read again: %v", reads)
	}
}

func TestIndexDocumentsCancellationKeepsPreviousSource(t *testing.T) {
	root := documentsVault(t)
	path := writeVaultFile(t, root, "books/manuals/a.pdf", "a")
	reader := useFakeDocumentReader(t, map[string]documentText{"a.pdf": pages("first edition")})
	index(t, root)

	touchLater(t, path)
	reader.texts["a.pdf"] = pages("second edition")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader.onRead = func(string) { cancel() }
	if err := IndexVaultContext(ctx, root, false, io.Discard, nil); err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	hits, err := openIndex(t, root).Search("edition", 10)
	if err != nil || len(hits) != 1 || hits[0].Body != "first edition" {
		t.Fatalf("hits after cancellation = %+v, %v; want the previous text", hits, err)
	}

	// The interrupted document is picked up by the next run.
	reader.onRead = nil
	index(t, root)
	if hits, _ := openIndex(t, root).Search("edition", 10); len(hits) != 1 || hits[0].Body != "second edition" {
		t.Fatalf("hits after the retry = %+v", hits)
	}
}

func TestDocumentArticlesCapPageText(t *testing.T) {
	long := strings.Repeat("word ", 5000) // 25,000 characters
	articles := documentArticles("Manual", []documentPart{{Path: "page/1", Label: "p. 1", Text: long}, {Path: "page/2", Label: "p. 2", Text: " \n "}})
	if len(articles) != 1 {
		t.Fatalf("articles = %d, want the empty page dropped", len(articles))
	}
	if n := len([]rune(articles[0].Body)); n > maxDocumentPartChars || n < maxDocumentPartChars-10 {
		t.Errorf("body length = %d, want capped at %d", n, maxDocumentPartChars)
	}
	var sections []struct{ Heading, Body string }
	if err := json.Unmarshal([]byte(articles[0].Sections), &sections); err != nil || len(sections) != 1 || sections[0].Heading != "" {
		t.Fatalf("sections = %+v, %v", sections, err)
	}
	// Semantic chunks are headed by the page's citation, once.
	chunks := buildChunks("", articles[0].Title, articles[0].Sections)
	if len(chunks) == 0 || chunks[0].Header != "Manual, p. 1" {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func TestDocumentTitle(t *testing.T) {
	cases := []struct{ given, own, rel, want string }{
		{"Volvo 240 Owner's Manual", "volvo.indd", "books/m/volvo.pdf", "Volvo 240 Owner's Manual"},
		{"", "Machinery's Handbook", "books/m/mh.pdf", "Machinery's Handbook"},
		{"", "Microsoft Word - Service Notes", "books/m/notes.pdf", "Service Notes"},
		{"", "Microsoft Word - notes_v3.docx", "books/m/notes.pdf", "notes"},
		{"", "untitled", "books/m/workshop/hmmwv-operator.pdf", "hmmwv-operator"},
		{"", "", "books/m/Timing.PDF", "Timing"},
	}
	for _, tc := range cases {
		if got := documentTitle(tc.given, tc.own, tc.rel); got != tc.want {
			t.Errorf("documentTitle(%q, %q, %q) = %q, want %q", tc.given, tc.own, tc.rel, got, tc.want)
		}
	}
}

// TestIndexRealPDFs runs the document pass with PDFium on the pdfext
// fixtures, from collection metadata to search hits.
func TestIndexRealPDFs(t *testing.T) {
	root := documentsVault(t)
	fixtures := filepath.Join("..", "pdfext", "testdata")
	for name, rel := range map[string]string{
		"text.pdf":      "books/manuals/owner.pdf",
		"scan.pdf":      "books/manuals/scans/trailer.pdf",
		"mixed.pdf":     "books/manuals/wiring.pdf",
		"enc-user.pdf":  "books/manuals/locked.pdf",
		"enc-owner.pdf": "books/manuals/print-protected.pdf",
	} {
		copyFile(t, filepath.Join(fixtures, name), writeVaultFile(t, root, rel, ""))
	}
	writeVaultFile(t, root, "books/manuals/broken.pdf", "%PDF-1.4\nnot really")
	entries := `{"books/manuals/print-protected.pdf": {"title": "Käyttöohjekirja", "collection": "manuals"}}`
	writeVaultFile(t, root, ".svalbard/documents.json", entries)

	out := index(t, root)
	for _, line := range []string{
		"  manuals/owner.pdf            3 pages\n",
		"  manuals/print-protected.pdf  3 pages\n",
		"  manuals/scans/trailer.pdf    not searchable: scanned: no text layer, needs OCR\n",
		"  manuals/wiring.pdf           2 pages; 2 of 4 pages are scanned images without text\n",
		"  manuals/locked.pdf           not searchable: password-protected\n",
		"  manuals/broken.pdf           not searchable: unreadable PDF: ",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("report misses %q:\n%s", line, out)
		}
	}
	db := openIndex(t, root)
	// The PDF's own title and outline make the citation.
	hits, err := db.Search(`"coolant capacity"`, 10)
	if err != nil || len(hits) != 2 {
		t.Fatalf("hits = %+v, %v; want the page in both copies", hits, err)
	}
	titles := []string{hits[0].Title, hits[1].Title}
	if !(contains(titles, "Fixture Owner's Manual, p. 1 · Cooling system") && contains(titles, "Käyttöohjekirja, p. 1 · Cooling system")) {
		t.Errorf("hit titles = %q", titles)
	}
	// Finnish words are found with and without their diacritics folded by
	// the tokenizer, and come back intact.
	for _, query := range []string{"jäähdytysnesteen", "määrä", "öljy"} {
		hits, err := db.Search(query, 10)
		if err != nil || len(hits) != 2 || hits[0].Path != "page/2" || !strings.Contains(hits[0].Body, "Jäähdytysnesteen määrä lämmityslaitteineen on 9,5 litraa.") {
			t.Errorf("search %q = %+v, %v", query, hits, err)
		}
	}
	if note := indexedSource(t, root, "books/manuals/locked.pdf").Note; note != "password-protected" {
		t.Errorf("locked note = %q", note)
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

func TestConfiguredIndexCoversDocumentsOnlyVault(t *testing.T) {
	root := t.TempDir()
	writeVaultFile(t, root, "manifest.yaml", "version: 2\nrealized:\n  entries:\n    - id: manuals\n      type: pdf\n      relative_path: books/manuals/a.pdf\n")
	writeVaultFile(t, root, "books/manuals/a.pdf", "a")
	writeVaultFile(t, root, "models/embed/model.gguf", "test embedding model")
	useFakeDocumentReader(t, map[string]documentText{"a.pdf": pages("coolant capacity", "brake fluid")})
	oldFind := findEmbeddingModel
	findEmbeddingModel = func(root string) (string, error) {
		return filepath.Join(root, "models", "embed", "model.gguf"), nil
	}
	t.Cleanup(func() { findEmbeddingModel = oldFind })
	installFakeEmbeddingServer(t, &fakeEmbeddingServer{dimensions: 2}, nil)

	var out bytes.Buffer
	if err := RunConfiguredIndex(context.Background(), root, "semantic", &out, nil, nil); err != nil {
		t.Fatalf("RunConfiguredIndex: %v\n%s", err, out.String())
	}
	db := openIndex(t, root)
	if state, err := db.SemanticState(); err != nil || state != "complete" {
		t.Fatalf("semantic state = %q, %v; want complete", state, err)
	}
	sources, err := db.Sources()
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources = %+v, %v", sources, err)
	}
	if embedded, err := db.EmbeddingCountBySource(sources[0].ID); err != nil || embedded != 2 {
		t.Fatalf("embedded pages = %d, %v; want 2", embedded, err)
	}
}

func TestConfiguredIndexSkipsVaultWithoutArchivesOrDocuments(t *testing.T) {
	root := t.TempDir()
	writeVaultFile(t, root, "manifest.yaml", "version: 2\nrealized:\n  entries: []\n")
	if err := os.MkdirAll(filepath.Join(root, "books"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RunConfiguredIndex(context.Background(), root, "", &out, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "search.db")); !os.IsNotExist(err) {
		t.Fatalf("created an unneeded search database: %v", err)
	}
	if out.String() != "No ZIM archives or documents to index.\n" {
		t.Errorf("output = %q", out.String())
	}
}

// books/ may be a symlink (a vault whose documents live on another volume).
// WalkDir does not follow a symlinked root: read naively it lists nothing,
// and every document source would be pruned from the index.
func TestIndexFollowsSymlinkedBooksDirectory(t *testing.T) {
	root := documentsVault(t)
	elsewhere := t.TempDir()
	writeVaultFile(t, elsewhere, "manuals/a.pdf", "a")
	writeVaultFile(t, elsewhere, "manuals/workshop/b.pdf", "b")
	if err := os.Symlink(elsewhere, filepath.Join(root, "books")); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	reader := useFakeDocumentReader(t, map[string]documentText{"a.pdf": pages("alpha text"), "b.pdf": pages("bravo text")})

	documents, err := ScanDocuments(root)
	if want := []string{"books/manuals/a.pdf", "books/manuals/workshop/b.pdf"}; err != nil || !reflect.DeepEqual(documents, want) {
		t.Fatalf("ScanDocuments = %v, %v; want %v", documents, err, want)
	}
	index(t, root)
	index(t, root)
	if got, want := indexedCounts(t, root), map[string]int64{"books/manuals/a.pdf": 1, "books/manuals/workshop/b.pdf": 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("indexed = %v, want %v", got, want)
	}
	if reads := reader.takeReads(); len(reads) != 2 {
		t.Fatalf("files read over two runs = %v, want each once", reads)
	}

	// A books/ that is a file, or a link to nowhere, is not an empty one.
	for name, replace := range map[string]func(string) error{
		"dangling link": func(path string) error { return os.Symlink(filepath.Join(elsewhere, "gone"), path) },
		"a file":        func(path string) error { return os.WriteFile(path, []byte("x"), 0o644) },
	} {
		books := filepath.Join(root, "books")
		if err := os.Remove(books); err != nil {
			t.Fatal(err)
		}
		if err := replace(books); err != nil {
			t.Fatal(err)
		}
		if err := IndexVault(root, false, io.Discard, nil); err == nil {
			t.Fatalf("books/ as %s: the index ran", name)
		}
		if got := indexedCounts(t, root); len(got) != 2 {
			t.Fatalf("books/ as %s: indexed = %v, want both documents kept", name, got)
		}
	}
}

// One file that cannot be read must not stop the other documents, and must
// not cost it what the index already held.
func TestIndexDocumentsSkipsUnreadableFileAndGoesOn(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads every file")
	}
	root := documentsVault(t)
	locked := writeVaultFile(t, root, "books/manuals/b-locked.pdf", "b")
	writeVaultFile(t, root, "books/manuals/a.pdf", "a")
	writeVaultFile(t, root, "books/manuals/c.pdf", "c")
	reader := useFakeDocumentReader(t, map[string]documentText{"a.pdf": pages("alpha text"), "b-locked.pdf": pages("bravo text"), "c.pdf": pages("charlie text")})
	index(t, root)
	reader.takeReads()

	// Every file changed; the middle one can no longer be opened.
	for _, name := range []string{"a.pdf", "b-locked.pdf", "c.pdf"} {
		touchLater(t, filepath.Join(root, "books", "manuals", name))
	}
	reader.texts["a.pdf"], reader.texts["c.pdf"] = pages("alpha revised"), pages("charlie revised")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })

	var out bytes.Buffer
	if err := IndexVault(root, false, &out, nil); err != nil {
		t.Fatalf("one unreadable file failed the index: %v\n%s", err, out.String())
	}
	want := "Indexing documents:\n" +
		"  manuals/a.pdf         1 page\n" +
		"  manuals/b-locked.pdf  skipped: cannot read file: permission denied\n" +
		"  manuals/c.pdf         1 page\n"
	if out.String() != want {
		t.Fatalf("report = %q, want %q", out.String(), want)
	}
	db := openIndex(t, root)
	for query, body := range map[string]string{"alpha": "alpha revised", "charlie": "charlie revised", "bravo": "bravo text"} {
		if hits, err := db.Search(query, 5); err != nil || len(hits) != 1 || hits[0].Body != body {
			t.Errorf("search %q = %+v, %v; want %q", query, hits, err, body)
		}
	}

	// Readable again, it is picked up by the next run.
	if err := os.Chmod(locked, 0o644); err != nil {
		t.Fatal(err)
	}
	reader.texts["b-locked.pdf"] = pages("bravo revised")
	index(t, root)
	if hits, _ := db.Search("bravo", 5); len(hits) != 1 || hits[0].Body != "bravo revised" {
		t.Fatalf("after the file became readable: %+v", hits)
	}
}
