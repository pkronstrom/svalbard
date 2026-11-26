package engine

import (
	"context"
	"testing"
)

func TestEmbeddedSourceNames(t *testing.T) {
	archive, entry, ok := EmbeddedArchive("usda.zim/files/a b.pdf")
	if !ok || archive != "usda.zim" || entry != "files/a b.pdf" {
		t.Fatalf("EmbeddedArchive = %q, %q, %v", archive, entry, ok)
	}
	if !IsEmbeddedSource("usda.zim/files/a b.pdf") || !IsDocumentSource("usda.zim/files/a b.pdf") {
		t.Fatal("an embedded document is not recognised")
	}
	for _, name := range []string{"usda.zim", "books/x.pdf", "books/a.zim/b.pdf", "skills/a/SKILL.md", "lib/js/x.zim/docs/a.md"} {
		if IsEmbeddedSource(name) {
			t.Errorf("IsEmbeddedSource(%q) = true", name)
		}
		if _, _, ok := EmbeddedArchive(name); ok {
			t.Errorf("EmbeddedArchive(%q) ok", name)
		}
	}
}

func TestSearchScopeOfAnArchiveCoversItsDocuments(t *testing.T) {
	db := documentDB(t)
	insertArticle(t, db, insertSource(t, db, "usda.zim"), "/home", "USDA canning", "canning at home")
	insertPage(t, db, insertDocument(t, db, "usda.zim/files/guide.pdf", "Guide", ""), 1, "Guide, p. 1", "canning tomatoes", "")
	insertPage(t, db, insertDocument(t, db, "usda-2.zim/files/other.pdf", "Other", ""), 1, "Other, p. 1", "canning beans", "")
	insertPage(t, db, insertDocument(t, db, "books/canning.pdf", "Book", ""), 1, "Book, p. 1", "canning jam", "")

	for scope, want := range map[string]int{
		"":                         4,
		"usda.zim":                 2,
		"usda.zim/files/guide.pdf": 1,
		"usda-2.zim":               1,
	} {
		matches := search(t, db, "canning", KeywordOptions{Source: scope})
		if len(matches.Results) != want {
			t.Errorf("scope %q found %v, want %d", scope, paths(matches.Results), want)
		}
		for _, result := range matches.Results {
			if !InScope(result.Filename, scope) {
				t.Errorf("scope %q returned %s, which InScope rejects", scope, result.Filename)
			}
		}
	}
}

func TestSearchCapsPagesPerEmbeddedDocument(t *testing.T) {
	db := documentDB(t)
	guide := insertDocument(t, db, "usda.zim/files/guide.pdf", "Guide", "")
	for number := 1; number <= 6; number++ {
		insertPage(t, db, guide, number, "Guide, p. "+PagePath(number)[5:], "pressure canning altitude", "")
	}
	insertArticle(t, db, insertSource(t, db, "wiki.zim"), "/Canning", "Canning", "pressure canning at altitude")

	perSource := map[string]int{}
	for _, result := range search(t, db, "pressure canning", KeywordOptions{Limit: 10}).Results {
		perSource[result.Filename]++
	}
	if perSource["usda.zim/files/guide.pdf"] != pagesPerDocument || perSource["wiki.zim"] != 1 {
		t.Fatalf("unscoped results per source = %v", perSource)
	}
	if n := len(search(t, db, "pressure canning", KeywordOptions{Source: "usda.zim/files/guide.pdf", Limit: 10}).Results); n != 6 {
		t.Fatalf("scoped to the document: %d results, want 6", n)
	}
}

func TestDocumentsListsEmbedded(t *testing.T) {
	db := documentDB(t)
	insertPage(t, db, insertDocument(t, db, "books/a.pdf", "A", ""), 1, "A, p. 1", "x", "")
	insertDocument(t, db, "usda.zim/files/scan.pdf", "Scan", "scanned: no text layer, needs OCR")
	insertPage(t, db, insertDocument(t, db, "usda.zim/files/guide.pdf", "Guide", ""), 1, "Guide, p. 1", "y", "")
	insertSource(t, db, "usda.zim")

	documents, err := New(db).Documents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Document{}
	for _, doc := range documents {
		got[doc.Filename] = doc
	}
	if len(got) != 3 || got["usda.zim/files/guide.pdf"].Pages != 1 || got["usda.zim/files/scan.pdf"].Note == "" {
		t.Fatalf("documents = %+v", documents)
	}
	if _, ok := got["usda.zim"]; ok {
		t.Fatal("the archive itself is listed as a document")
	}
}
