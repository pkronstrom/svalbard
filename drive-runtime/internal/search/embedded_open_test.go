package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedURL(t *testing.T) {
	for _, tc := range []struct {
		filename string
		page     int
		want     string
	}{
		{"zimgit-post-disaster_en_2024-05.zim/files/Emergency Preparedness (10).pdf", 7, "http://localhost:9000/content/zimgit-post-disaster_en_2024-05/files/Emergency%20Preparedness%20%2810%29.pdf#page=7"},
		{"usda.zim/files/book.epub", 3, "http://localhost:9000/content/usda/files/book.epub"},
		{"usda.zim/files/ä.pdf", 0, "http://localhost:9000/content/usda/files/%C3%A4.pdf"},
	} {
		if got := embeddedURL("http://localhost:9000", tc.filename, tc.page); got != tc.want {
			t.Errorf("embeddedURL(%q, %d) = %q, want %q", tc.filename, tc.page, got, tc.want)
		}
	}
}

func TestSessionOpensAnEmbeddedDocumentThroughKiwix(t *testing.T) {
	root := newSearchDrive(t)
	writeTestKiwix(t, root)
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "usda.zim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SVALBARD_TEST_KIWIX", "1")
	var opened string
	session, err := NewSession(root, func(address string) error {
		opened = address
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.OpenResult(Result{Filename: "usda.zim/files/guide (2).pdf", Path: "page/12"}); err != nil {
		t.Fatalf("OpenResult: %v", err)
	}
	if !strings.HasSuffix(opened, "/content/usda/files/guide%20%282%29.pdf#page=12") || !strings.HasPrefix(opened, "http://localhost:") {
		t.Fatalf("opened %q", opened)
	}
	if label := SourceLabel("usda.zim/files/guide (2).pdf"); label != "usda/files/guide (2).pdf" {
		t.Errorf("SourceLabel = %q", label)
	}
	if file := DocumentFile("usda.zim/files/guide (2).pdf", 12); file != "" {
		t.Errorf("DocumentFile = %q, want none for a document inside an archive", file)
	}
}

func TestSessionOpensALooseEPUBWithoutAPage(t *testing.T) {
	root := newSearchDrive(t)
	t.Setenv("PATH", t.TempDir())
	addDocument(t, root, "books/novels/book.epub", 2, "Fixture Book, ch. 2", "The long road")
	var opened string
	session, err := NewSession(root, func(address string) error {
		opened = address
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	response, err := session.Search(context.Background(), ModeKeyword, "long road", 5)
	if err != nil || len(response.Results) != 1 {
		t.Fatalf("Search = %+v, %v", response.Results, err)
	}
	if err := session.OpenResult(response.Results[0]); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(opened, "#") || !strings.HasSuffix(opened, "/novels/book.epub") {
		t.Fatalf("opened %q, want the EPUB without a page", opened)
	}
}
