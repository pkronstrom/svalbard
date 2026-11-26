package commands

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	_ "github.com/ncruces/go-sqlite3/driver"
)

func fixtureZIM(name string) string { return filepath.Join("..", "zimext", "testdata", name) }

// zimVault is a vault with documents.zim in zim/ and a manifest.
func zimVault(t *testing.T) string {
	t.Helper()
	root := documentsVault(t)
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, fixtureZIM("documents.zim"), filepath.Join(root, "zim", "documents.zim"))
	return root
}

// countReads wraps every document reader so the test can count the
// documents read.
func countReads(t *testing.T) *atomic.Int64 {
	t.Helper()
	var reads atomic.Int64
	old := documentKinds
	wrapped := make(map[string]documentKind, len(old))
	for ext, kind := range old {
		open := kind.open
		kind.open = func() (documentReader, error) {
			reader, err := open()
			if err != nil {
				return nil, err
			}
			return countingReader{reader, &reads}, nil
		}
		wrapped[ext] = kind
	}
	documentKinds = wrapped
	t.Cleanup(func() { documentKinds = old })
	return &reads
}

type countingReader struct {
	documentReader
	reads *atomic.Int64
}

func (r countingReader) Read(ctx context.Context, src documentSource) (documentText, error) {
	r.reads.Add(1)
	return r.documentReader.Read(ctx, src)
}

func sourcesWithPrefix(t *testing.T, root, prefix string) map[string]searchdb.Source {
	t.Helper()
	sources, err := openIndex(t, root).IndexedSources()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]searchdb.Source{}
	for _, source := range sources {
		if strings.HasPrefix(source.Filename, prefix) {
			found[source.Filename] = source
		}
	}
	return found
}

func TestIndexZIMIndexesEmbeddedDocuments(t *testing.T) {
	root := zimVault(t)
	out := index(t, root)

	counts := indexedCounts(t, root)
	want := map[string]int64{
		"documents.zim":                    3, // /Novel.123, /about, /home
		"documents.zim/files/guide.pdf":    3,
		"documents.zim/files/water.pdf":    3,
		"documents.zim/files/scan (1).pdf": 0,
		"documents.zim/files/book.epub":    5,
	}
	for name, n := range want {
		if counts[name] != n {
			t.Errorf("%s: %d articles, want %d", name, counts[name], n)
		}
	}
	if _, ok := counts["documents.zim/Novel.123.epub"]; ok {
		t.Error("the EPUB copy of an HTML book was indexed")
	}
	for name, source := range sourcesWithPrefix(t, root, "documents.zim") {
		if source.ReaderVersion != zimReaderVersion {
			t.Errorf("%s: ReaderVersion %d", name, source.ReaderVersion)
		}
	}
	if note := indexedSource(t, root, "documents.zim/files/scan (1).pdf").Note; note != "scanned: no text layer, needs OCR" {
		t.Errorf("scan note = %q", note)
	}
	hits, err := openIndex(t, root).Search("kettle", 5)
	if err != nil || len(hits) == 0 || hits[0].Title != "Fixture Book, ch. 1 · Beginnings" {
		t.Errorf("kettle hits = %+v, %v", hits, err)
	}
	guide := indexedSource(t, root, "documents.zim/files/guide.pdf")
	if guide.Title != "Fixture Library, Part 2: Fruit and Fruit Products" {
		t.Errorf("guide title = %q", guide.Title)
	}
	for _, line := range []string{"documents: 4 documents, 11 pages; 1 not searchable", "files/scan (1).pdf  not searchable: scanned: no text layer, needs OCR"} {
		if !strings.Contains(out, line) {
			t.Errorf("report misses %q:\n%s", line, out)
		}
	}
}

func TestIndexZIMSecondRunReadsNothing(t *testing.T) {
	root := zimVault(t)
	reads := countReads(t)
	index(t, root)
	if reads.Load() != 4 {
		t.Fatalf("first run read %d documents, want 4", reads.Load())
	}
	reads.Store(0)
	index(t, root)
	if reads.Load() != 0 {
		t.Fatalf("second run read %d documents", reads.Load())
	}
}

func TestIndexZIMUpgradesAnOldIndex(t *testing.T) {
	root := zimVault(t)
	copyFile(t, testSmallZIMPath(t), filepath.Join(root, "zim", "small.zim"))
	index(t, root)
	db := openIndex(t, root)
	ctx := context.Background()

	// An index written before this reader: junk in the nautilus ZIM, no
	// documents, no reader versions.
	old := indexedSource(t, root, "documents.zim")
	old.ReaderVersion = 0
	if err := db.ReplaceSource(ctx, old, []searchdb.Article{
		{Path: "/home", Title: "Fixture Library", Body: "A library of guides."},
		{Path: "/home.html", Title: "{{ title }}", Body: "{{ description }}"},
		{Path: "/vendors/x.css.map", Title: "x.css.map", Body: "version 3"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.PruneSources(ctx, map[string]bool{"documents.zim": true, "small.zim": true}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSourceReaderVersion(ctx, "small.zim", 0); err != nil {
		t.Fatal(err)
	}
	var smallIndexedAt string
	readIndexedAt := func() string {
		var at string
		if err := rawDB(t, root).QueryRow(`SELECT indexed_at FROM sources WHERE filename = 'small.zim'`).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	smallIndexedAt = readIndexedAt()

	index(t, root)
	heads, err := db.ArticleHeads("documents.zim")
	if err != nil {
		t.Fatal(err)
	}
	for _, head := range heads {
		if head.Path == "/home.html" || head.Path == "/vendors/x.css.map" {
			t.Errorf("junk article %s survived the upgrade", head.Path)
		}
	}
	if len(sourcesWithPrefix(t, root, "documents.zim/")) != 4 {
		t.Errorf("embedded sources after upgrade: %v", sourcesWithPrefix(t, root, "documents.zim/"))
	}
	for _, name := range []string{"documents.zim", "small.zim"} {
		if v := indexedSource(t, root, name).ReaderVersion; v != zimReaderVersion {
			t.Errorf("%s: ReaderVersion %d", name, v)
		}
	}
	if at := readIndexedAt(); at != smallIndexedAt {
		t.Errorf("small.zim was re-extracted: indexed_at %s, was %s", at, smallIndexedAt)
	}
}

func TestIndexZIMResumesAfterInterruption(t *testing.T) {
	root := zimVault(t)
	reads := countReads(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := 0
	err := IndexVaultContext(ctx, root, false, &bytes.Buffer{}, func(p IndexProgress) {
		if p.Status == "done" && strings.HasPrefix(p.File, "documents.zim/") {
			if done++; done == 1 {
				cancel()
			}
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if v := indexedSource(t, root, "documents.zim").ReaderVersion; v != 0 {
		t.Fatalf("an interrupted archive has ReaderVersion %d", v)
	}
	reads.Store(0)
	index(t, root)
	if reads.Load() != 3 {
		t.Fatalf("the resumed run read %d documents, want the 3 not yet done", reads.Load())
	}
	if v := indexedSource(t, root, "documents.zim").ReaderVersion; v != zimReaderVersion {
		t.Fatalf("ReaderVersion after resuming = %d", v)
	}
}

func TestIndexZIMPrunesWithTheArchive(t *testing.T) {
	root := zimVault(t)
	index(t, root)
	if err := os.Rename(filepath.Join(root, "zim", "documents.zim"), filepath.Join(root, "zim", "documents_2.zim")); err != nil {
		t.Fatal(err)
	}
	index(t, root)
	if left := sourcesWithPrefix(t, root, "documents.zim"); len(left) != 0 {
		t.Errorf("sources of the removed archive remain: %v", left)
	}
	if _, ok := indexedCounts(t, root)["documents_2.zim/files/guide.pdf"]; !ok {
		t.Error("the new archive's documents are not indexed")
	}
}

func TestIndexZIMSkipsOversizedEntries(t *testing.T) {
	root := zimVault(t)
	old := maxEmbeddedDocumentBytes
	maxEmbeddedDocumentBytes = 1000
	t.Cleanup(func() { maxEmbeddedDocumentBytes = old })
	index(t, root)
	guide := indexedSource(t, root, "documents.zim/files/guide.pdf")
	if !strings.HasPrefix(guide.Note, "too large to index") || indexedCounts(t, root)["documents.zim/files/guide.pdf"] != 0 {
		t.Fatalf("guide.pdf: note %q, %d articles", guide.Note, indexedCounts(t, root)["documents.zim/files/guide.pdf"])
	}
}

func TestIndexZIMWithNothingIndexableStillFails(t *testing.T) {
	root := documentsVault(t)
	os.MkdirAll(filepath.Join(root, "zim"), 0o755)
	copyFile(t, fixtureZIM("empty.zim"), filepath.Join(root, "zim", "empty.zim"))
	var out bytes.Buffer
	err := IndexVault(root, false, &out, nil)
	if err == nil || !strings.Contains(err.Error(), "empty.zim") {
		t.Fatalf("err = %v, want one naming empty.zim", err)
	}
}

// rawDB opens the vault's search.db directly, for columns the searchdb API
// does not return.
func rawDB(t *testing.T, root string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
