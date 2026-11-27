package commands

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func readEPUB(t *testing.T, data []byte) documentText {
	t.Helper()
	reader, err := openEPUBReader()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	text, err := reader.Read(context.Background(), documentSource{Name: "books/novels/book.epub", Data: bytes.NewReader(data), Size: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func epubFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "zimext", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestEPUBReaderReadsSpineInOrder(t *testing.T) {
	text := readEPUB(t, epubFixture(t, "book.epub"))
	if text.Title != "Fixture Book" || text.Note != "" {
		t.Fatalf("title %q, note %q", text.Title, text.Note)
	}
	want := []struct{ path, label, section string }{
		{"page/1", "ch. 1", "Beginnings"},
		{"page/2", "ch. 2", "The Long Road"},
		{"page/3", "ch. 2 (cont.)", "The Long Road"},
		{"page/4", "ch. 2 (cont.)", "The Long Road"},
		{"page/5", "ch. 2 (cont.)", "The Long Road"},
	}
	if len(text.Parts) != len(want) {
		t.Fatalf("parts = %+v, want %d", partPaths(text), len(want))
	}
	var chapter2 []string
	for i, w := range want {
		part := text.Parts[i]
		if part.Path != w.path || part.Label != w.label || part.Section != w.section {
			t.Errorf("part %d = %s %q %q, want %s %q %q", i, part.Path, part.Label, part.Section, w.path, w.label, w.section)
		}
		if n := utf8.RuneCountInString(part.Text); n > maxDocumentPartChars {
			t.Errorf("part %d holds %d characters", i, n)
		}
		if i > 0 {
			chapter2 = append(chapter2, part.Text)
		}
	}
	joined := strings.Join(chapter2, "\n")
	for n := 1; n < 1300; n++ {
		if c := strings.Count(joined, fmt.Sprintf("L%04d ", n)); c != 1 {
			t.Fatalf("line L%04d appears %d times in chapter 2", n, c)
		}
	}
}

func TestEPUBReaderNotesProblemsWithoutFailing(t *testing.T) {
	if text := readEPUB(t, epubFixture(t, "drm.epub")); text.Note != "DRM-protected" || len(text.Parts) != 0 {
		t.Errorf("drm.epub: note %q, %d parts", text.Note, len(text.Parts))
	}
	if text := readEPUB(t, []byte("not a zip at all")); !strings.HasPrefix(text.Note, "unreadable EPUB:") {
		t.Errorf("random bytes: note %q", text.Note)
	}
	var empty bytes.Buffer
	zipWith(t, &empty, map[string]string{"mimetype": "application/epub+zip"})
	if text := readEPUB(t, empty.Bytes()); text.Note != "unreadable EPUB: no package file" {
		t.Errorf("no container: note %q", text.Note)
	}
}

func TestLooseEPUBsUnderBooksAreScanned(t *testing.T) {
	root := documentsVault(t)
	copyFile(t, filepath.Join("..", "zimext", "testdata", "book.epub"), writeVaultFile(t, root, "books/novels/book.epub", ""))
	documents, err := ScanDocuments(root)
	if err != nil || len(documents) != 1 || documents[0] != "books/novels/book.epub" {
		t.Fatalf("ScanDocuments = %v, %v", documents, err)
	}
	out := index(t, root)
	if !strings.Contains(out, "novels/book.epub  5 parts") {
		t.Errorf("report:\n%s", out)
	}
	if counts := indexedCounts(t, root); counts["books/novels/book.epub"] != 5 {
		t.Fatalf("counts = %v", counts)
	}
	hits, err := openIndex(t, root).Search("kettle", 5)
	if err != nil || len(hits) != 1 || hits[0].Title != "Fixture Book, ch. 1 · Beginnings" {
		t.Fatalf("hits = %+v, %v", hits, err)
	}
}

func zipWith(t *testing.T, w *bytes.Buffer, files map[string]string) {
	t.Helper()
	z := zip.NewWriter(w)
	for name, body := range files {
		f, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
}
