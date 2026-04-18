package searchdb

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabase(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
}

func TestUpsertAndSearchArticles(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, err := db.UpsertSource("wiki.zim", "Wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	err = db.InsertArticles(sid, []Article{
		{Path: "A/Linux", Title: "Linux", Body: "Linux is an operating system kernel"},
		{Path: "A/Go", Title: "Go programming", Body: "Go is a programming language created by Google"},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, err := db.Search("linux kernel", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Title != "Linux" {
		t.Errorf("first result title = %q", results[0].Title)
	}
}

func TestDeleteSourceRemovesArticles(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, err := db.UpsertSource("wiki.zim", "Wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	err = db.InsertArticles(sid, []Article{
		{Path: "A/Test", Title: "Test", Body: "test content"},
	})
	if err != nil {
		t.Fatal(err)
	}

	count, err := db.ArticleCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("article count = %d", count)
	}

	err = db.DeleteSourceArticles(sid)
	if err != nil {
		t.Fatal(err)
	}
	count, err = db.ArticleCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("after delete, article count = %d", count)
	}
}

func TestMetaSetGet(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.SetMeta("tier", "standard")
	if err != nil {
		t.Fatal(err)
	}
	val, err := db.GetMeta("tier")
	if err != nil {
		t.Fatal(err)
	}
	if val != "standard" {
		t.Errorf("meta value = %q", val)
	}

	// Missing key returns empty string
	val, err = db.GetMeta("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("missing key should return empty, got %q", val)
	}
}

func TestStats(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.UpsertSource("a.zim", "A")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.UpsertSource("b.zim", "B")
	if err != nil {
		t.Fatal(err)
	}

	sc, ac, err := db.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if sc != 2 {
		t.Errorf("source count = %d", sc)
	}
	if ac != 0 {
		t.Errorf("article count = %d", ac)
	}
}

func TestIndexedFilenames(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.UpsertSource("wiki.zim", "Wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.UpsertSource("ifixit.zim", "iFixit")
	if err != nil {
		t.Fatal(err)
	}

	fns, err := db.IndexedFilenames()
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 2 {
		t.Errorf("indexed filenames = %d", len(fns))
	}
}

func TestDeleteSource(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, err := db.UpsertSource("wiki.zim", "Wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	err = db.InsertArticles(sid, []Article{
		{Path: "A/Test", Title: "Test", Body: "test content"},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.DeleteSource(sid)
	if err != nil {
		t.Fatal(err)
	}

	// Source should be gone
	fns, err := db.IndexedFilenames()
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 0 {
		t.Errorf("expected 0 filenames after delete, got %d", len(fns))
	}
}

func TestUpsertSourceUpdatesTitle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid1, err := db.UpsertSource("wiki.zim", "Wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	sid2, err := db.UpsertSource("wiki.zim", "Wikipedia Updated")
	if err != nil {
		t.Fatal(err)
	}

	if sid1 != sid2 {
		t.Errorf("upsert returned different IDs: %d vs %d", sid1, sid2)
	}
}

func TestEmbeddingDims(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, _ := db.UpsertSource("test.zim", "Test")
	db.InsertArticles(sid, []Article{{Path: "/A/One", Title: "One", Body: "body"}})

	// 384-dim embedding = 384 * 4 bytes = 1536 bytes.
	vec := make([]byte, 384*4)
	db.InsertChunkEmbeddings([]ChunkEmbedding{{ArticleID: 1, ChunkIndex: 0, Header: "One", Vector: vec}})

	dims, err := db.EmbeddingDims()
	if err != nil {
		t.Fatal(err)
	}
	if dims != 384 {
		t.Errorf("dims = %d, want 384", dims)
	}
}

func TestChunkEmbeddings(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, err := db.UpsertSource("wiki.zim", "Wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	err = db.InsertArticles(sid, []Article{
		{Path: "/A/One", Title: "One", Body: "first article"},
		{Path: "/A/Two", Title: "Two", Body: "second article"},
		{Path: "/A/Three", Title: "Three", Body: "third article"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initially no embeddings.
	count, err := db.EmbeddingCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("initial embedding count = %d", count)
	}

	// Get unembedded articles.
	unembedded, err := db.UnembeddedArticles(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(unembedded) != 3 {
		t.Fatalf("unembedded = %d, want 3", len(unembedded))
	}

	// Insert chunk embeddings for first two articles (one chunk each).
	fakeVec := make([]byte, 8) // 2 floats
	err = db.InsertChunkEmbeddings([]ChunkEmbedding{
		{ArticleID: unembedded[0].ID, ChunkIndex: 0, Header: "One", Vector: fakeVec},
		{ArticleID: unembedded[1].ID, ChunkIndex: 0, Header: "Two", Vector: fakeVec},
	})
	if err != nil {
		t.Fatal(err)
	}

	count, err = db.EmbeddingCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("embedding count = %d, want 2", count)
	}

	// Only one article should remain unembedded.
	remaining, err := db.UnembeddedArticles(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Errorf("remaining unembedded = %d, want 1", len(remaining))
	}
	if remaining[0].Title != "Three" {
		t.Errorf("remaining title = %q, want Three", remaining[0].Title)
	}
}

func TestInsertChunkEmbeddingsMultipleChunksPerArticle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, _ := db.UpsertSource("test.zim", "Test")
	db.InsertArticles(sid, []Article{
		{Path: "/A/One", Title: "One", Body: "first article"},
	})

	fakeVec := make([]byte, 16) // 4 floats

	// Insert 3 chunks for the same article.
	err = db.InsertChunkEmbeddings([]ChunkEmbedding{
		{ArticleID: 1, ChunkIndex: 0, Header: "Intro", Vector: fakeVec},
		{ArticleID: 1, ChunkIndex: 1, Header: "Section A", Vector: fakeVec},
		{ArticleID: 1, ChunkIndex: 2, Header: "Section B", Vector: fakeVec},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Total rows in embeddings should be 3.
	count, err := db.EmbeddingCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("embedding count = %d, want 3", count)
	}

	// Article should NOT appear as unembedded (it has chunks).
	unembedded, err := db.UnembeddedArticles(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(unembedded) != 0 {
		t.Errorf("unembedded = %d, want 0", len(unembedded))
	}
}

func TestInsertChunkEmbeddingsInsertOrReplace(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, _ := db.UpsertSource("test.zim", "Test")
	db.InsertArticles(sid, []Article{
		{Path: "/A/One", Title: "One", Body: "first article"},
	})

	vec1 := make([]byte, 8)
	vec1[0] = 0x01
	vec2 := make([]byte, 8)
	vec2[0] = 0x02

	// Insert a chunk.
	err = db.InsertChunkEmbeddings([]ChunkEmbedding{
		{ArticleID: 1, ChunkIndex: 0, Header: "Original", Vector: vec1},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Replace with different header and vector.
	err = db.InsertChunkEmbeddings([]ChunkEmbedding{
		{ArticleID: 1, ChunkIndex: 0, Header: "Updated", Vector: vec2},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should still be exactly 1 row (replaced, not duplicated).
	count, err := db.EmbeddingCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("embedding count after replace = %d, want 1", count)
	}
}

func TestEmbeddingCountBySourceDistinct(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, _ := db.UpsertSource("test.zim", "Test")
	db.InsertArticles(sid, []Article{
		{Path: "/A/One", Title: "One", Body: "first"},
		{Path: "/A/Two", Title: "Two", Body: "second"},
	})

	fakeVec := make([]byte, 8)

	// Insert 3 chunks for article 1, 2 chunks for article 2.
	err = db.InsertChunkEmbeddings([]ChunkEmbedding{
		{ArticleID: 1, ChunkIndex: 0, Header: "1-A", Vector: fakeVec},
		{ArticleID: 1, ChunkIndex: 1, Header: "1-B", Vector: fakeVec},
		{ArticleID: 1, ChunkIndex: 2, Header: "1-C", Vector: fakeVec},
		{ArticleID: 2, ChunkIndex: 0, Header: "2-A", Vector: fakeVec},
		{ArticleID: 2, ChunkIndex: 1, Header: "2-B", Vector: fakeVec},
	})
	if err != nil {
		t.Fatal(err)
	}

	// EmbeddingCountBySource should return 2 (distinct articles), not 5 (chunks).
	count, err := db.EmbeddingCountBySource(sid)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("EmbeddingCountBySource = %d, want 2 (distinct articles)", count)
	}
}

func TestUnembeddedArticlesBySourceReturnsSections(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, _ := db.UpsertSource("test.zim", "Test")
	sectionsJSON := `[{"heading":"Intro","body":"hello"},{"heading":"Details","body":"world"}]`
	db.InsertArticles(sid, []Article{
		{Path: "/A/One", Title: "One", Body: "first", Sections: sectionsJSON},
		{Path: "/A/Two", Title: "Two", Body: "second"},
	})

	articles, err := db.UnembeddedArticlesBySource(sid, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Fatalf("unembedded articles = %d, want 2", len(articles))
	}

	// First article should have sections JSON.
	if articles[0].Sections != sectionsJSON {
		t.Errorf("article 1 sections = %q, want %q", articles[0].Sections, sectionsJSON)
	}

	// Second article should have empty sections.
	if articles[1].Sections != "" {
		t.Errorf("article 2 sections = %q, want empty", articles[1].Sections)
	}
}

func TestUnembeddedArticlesReturnsSections(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid, _ := db.UpsertSource("test.zim", "Test")
	sectionsJSON := `[{"heading":"A","body":"aaa"}]`
	db.InsertArticles(sid, []Article{
		{Path: "/A/One", Title: "One", Body: "first", Sections: sectionsJSON},
	})

	articles, err := db.UnembeddedArticles(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("unembedded = %d, want 1", len(articles))
	}
	if articles[0].Sections != sectionsJSON {
		t.Errorf("sections = %q, want %q", articles[0].Sections, sectionsJSON)
	}
}
