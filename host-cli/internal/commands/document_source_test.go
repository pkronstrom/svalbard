package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPDFReaderReadsAnInMemorySource(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "pdfext", "testdata", "text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := openPDFReader()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	text, err := reader.Read(context.Background(), documentSource{Name: "x.zim/files/a.pdf", Data: bytes.NewReader(data), Size: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, part := range text.Parts {
		paths = append(paths, part.Path)
	}
	if want := []string{"page/1", "page/2", "page/3"}; !slices.Equal(paths, want) {
		t.Fatalf("parts = %v, want %v", paths, want)
	}
}

func TestMarkdownReaderTitleFallsBackToSourceName(t *testing.T) {
	body := []byte("no heading here, only text\n")
	reader, _ := openMarkdownReader()
	text, err := reader.Read(context.Background(), documentSource{Name: "lib/js/three/docs/getting-started.md", Data: bytes.NewReader(body), Size: int64(len(body))})
	if err != nil {
		t.Fatal(err)
	}
	if text.Title != "getting-started" {
		t.Fatalf("title = %q, want getting-started", text.Title)
	}
}

func TestDocumentTitleRejectsOfficeDefaults(t *testing.T) {
	if got := documentTitle("", "PowerPoint Presentation", "books/x/antibiogram.pdf"); got != "antibiogram" {
		t.Fatalf("documentTitle = %q, want the file name", got)
	}
}
