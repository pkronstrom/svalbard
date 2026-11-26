package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
)

const usdaZIM = "usda-2015_en_2025-04.zim"

// embeddedFixture adds to the document fixture an archive whose content is
// two PDFs, indexed as the host indexes documents inside a ZIM.
func embeddedFixture(t *testing.T) *documentFixture {
	t.Helper()
	f := newDocumentFixture(t)
	f.document(usdaZIM, "USDA Guide to Home Canning", "")
	guide := f.document(usdaZIM+"/files/GUIDE02_HomeCan_rev0715.pdf", "USDA Guide to Home Canning, Part 2: Fruit and Fruit Products", "")
	for page := 1; page <= 3; page++ {
		f.page(guide, page, "USDA Guide to Home Canning, Part 2: Fruit and Fruit Products, p. "+string(rune('0'+page)), "Pressure canning of fruit, page "+string(rune('0'+page)))
	}
	intro := f.document(usdaZIM+"/files/INTRO_HomeCanrev0715.pdf", "USDA Guide to Home Canning, Introduction", "")
	f.page(intro, 1, "USDA Guide to Home Canning, Introduction, p. 1", "Principles of home canning")
	f.file("zim/"+usdaZIM, "")
	return f
}

func TestSearchReadOfEmbeddedPage(t *testing.T) {
	f := embeddedFixture(t)
	got, err := f.read(map[string]any{"source": usdaZIM + "/files/GUIDE02_HomeCan_rev0715.pdf", "path": "page/2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "page 2") || got.Page != 2 || got.File != "" {
		t.Fatalf("read = %+v", got)
	}
	if len(got.Links) != 2 || got.Links[0].Path != "page/1" || got.Links[1].Path != "page/3" {
		t.Fatalf("links = %+v", got.Links)
	}
	items, _ := f.search(map[string]any{"query": "pressure canning"})
	for _, item := range items {
		if strings.HasPrefix(item.Source, usdaZIM) && (item.Page == 0 || item.File != "") {
			t.Errorf("search item %+v: want a page and no file", item)
		}
	}
}

func TestResolveEmbeddedDocumentByLooseNames(t *testing.T) {
	f := embeddedFixture(t)
	want := usdaZIM + "/files/GUIDE02_HomeCan_rev0715.pdf"
	for _, name := range []string{
		want,
		"usda-2015/files/GUIDE02_HomeCan_rev0715.pdf",
		"usda-2015_en_2025-04/files/GUIDE02_HomeCan_rev0715",
		"usda-2015/files/GUIDE02_HomeCan_rev0715",
	} {
		if got, err := f.cap.resolveSource(context.Background(), name, true); err != nil || got != want {
			t.Errorf("resolveSource(%q) = %q, %v", name, got, err)
		}
	}
	if got, err := f.cap.resolveScope(context.Background(), "usda-2015", true); err != nil || got != usdaZIM {
		t.Errorf("resolveScope(usda-2015) = %q, %v; want the archive", got, err)
	}
}

func TestEmptySearchFoldsEmbeddedDocumentsIntoTheirArchive(t *testing.T) {
	f := embeddedFixture(t)
	held := f.cap.holdings(context.Background())
	if !strings.Contains(held, "usda-2015_en_2025-04 (2 documents)") {
		t.Errorf("holdings = %q", held)
	}
	if strings.Contains(held, "GUIDE02") {
		t.Errorf("holdings list an embedded document on its own: %q", held)
	}
	names := sourceNames(f.cap.knownSources(context.Background(), false))
	for _, name := range names {
		if strings.Contains(name, "/files/") {
			t.Errorf("sourceNames lists %q", name)
		}
	}
}

func TestVaultSourcesCountsDocumentsInArchives(t *testing.T) {
	f := embeddedFixture(t)
	meta, _ := LoadMetadata(f.root)
	result, err := NewVaultCapability(f.root, meta).Handle(context.Background(), "sources", map[string]any{"type": "zim"})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range result.Data.([]inspect.SourceInfo) {
		if source.Name == usdaZIM {
			if source.Documents == nil || *source.Documents != 2 {
				t.Fatalf("documents = %v, want 2", source.Documents)
			}
			return
		}
	}
	t.Fatal("the archive is not listed")
}

func TestSearchReadOfArchiveNamesItsDocuments(t *testing.T) {
	f := newReadFixture(t, true)
	f.page("home", "USDA Guide to Home Canning", article("USDA Guide to Home Canning", "Loading..."), "Loading")
	if err := os.WriteFile(filepath.Join(f.pages, "wiki", ".main"), []byte("home"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`ALTER TABLE sources ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN note TEXT`,
		`ALTER TABLE articles ADD COLUMN sections TEXT`,
		`INSERT INTO sources (id, filename, title) VALUES (10, 'wiki.zim/files/guide.pdf', 'Part 2: Fruit'), (11, 'wiki.zim/files/scan.pdf', 'Scanned guide')`,
		`UPDATE sources SET note = 'scanned: no text layer, needs OCR' WHERE id = 11`,
		`INSERT INTO articles (source_id, path, title, body) VALUES (10, 'page/1', 'Part 2: Fruit, p. 1', 'fruit'), (10, 'page/2', 'Part 2: Fruit, p. 2', 'more fruit')`,
	} {
		if _, err := f.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.read(map[string]any{"source": "wiki"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"also holds 2 documents", "wiki.zim/files/guide.pdf (Part 2: Fruit, 2 pages)", "wiki.zim/files/scan.pdf (Scanned guide, not searchable: scanned: no text layer, needs OCR)"} {
		if !strings.Contains(got.Note, want) {
			t.Errorf("note misses %q: %q", want, got.Note)
		}
	}
}
