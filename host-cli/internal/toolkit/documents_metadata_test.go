package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestGenerateOffersSearchOnDocumentsOnlyVault(t *testing.T) {
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	entries := []manifest.RealizedEntry{{ID: "manuals", Type: "pdf", Filename: "a.pdf", RelativePath: "books/manuals/a.pdf"}}
	if err := Generate(root, entries, "default", GenerateOpts{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg testRuntimeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, group := range cfg.Groups {
		for _, item := range group.Items {
			if item.ID == "search-all" {
				return
			}
		}
	}
	t.Fatal("a vault that holds only documents does not offer Search")
}

func TestWriteDocumentsMetadata(t *testing.T) {
	root := t.TempDir()
	recipe := catalog.Item{ID: "manuals", Type: "pdf", Documents: []catalog.DocumentSource{
		{Title: "Alpha Manual", URL: "https://example.test/a.pdf"},
		{URL: "https://example.test/b.pdf"},
	}}
	entries := []manifest.RealizedEntry{
		{ID: "manuals", Type: "pdf", RelativePath: filepath.Join("books", "manuals", "a.pdf")},
		{ID: "manuals", Type: "pdf", RelativePath: filepath.Join("books", "manuals", "b.pdf")},
		{ID: "who", Type: "pdf", RelativePath: filepath.Join("books", "WHO.pdf")},
		{ID: "wiki", Type: "zim", RelativePath: filepath.Join("zim", "wiki.zim")},
	}
	if err := WriteDocumentsMetadata(root, entries, map[string]catalog.Item{"manuals": recipe}); err != nil {
		t.Fatal(err)
	}
	want := map[string]DocumentMetadata{
		"books/manuals/a.pdf": {Title: "Alpha Manual", Collection: "manuals", URL: "https://example.test/a.pdf"},
		"books/manuals/b.pdf": {Collection: "manuals", URL: "https://example.test/b.pdf"},
	}
	got, err := ReadDocumentsMetadata(root)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata = %+v, %v; want %+v", got, err, want)
	}

	// Without the definition (another computer) the recorded titles stay,
	// and a document that left the vault leaves the file.
	if err := WriteDocumentsMetadata(root, entries[:1], nil); err != nil {
		t.Fatal(err)
	}
	if got, _ = ReadDocumentsMetadata(root); !reflect.DeepEqual(got, map[string]DocumentMetadata{"books/manuals/a.pdf": want["books/manuals/a.pdf"]}) {
		t.Fatalf("metadata without the definition = %+v", got)
	}

	// No collection documents: no file.
	if err := WriteDocumentsMetadata(root, entries[2:], nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".svalbard", "documents.json")); !os.IsNotExist(err) {
		t.Fatalf("stale documents.json left behind: %v", err)
	}
	if got, err := ReadDocumentsMetadata(root); err != nil || len(got) != 0 {
		t.Fatalf("metadata of a vault without documents = %+v, %v", got, err)
	}
}
