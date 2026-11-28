package searchdb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ncruces/go-sqlite3"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

func seedSource(t *testing.T, db *DB, filename, title string, articles []Article) int64 {
	t.Helper()
	if err := db.ReplaceSource(context.Background(), Source{Filename: filename, Title: title, UUID: "test-" + filename, Size: 1, ModTimeNS: 1}, articles); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := db.db.QueryRow("SELECT id FROM sources WHERE filename = ?", filename).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func cancelOnUpdate(t *testing.T, db *DB, table string, cancel context.CancelFunc) func() {
	t.Helper()
	db.db.SetMaxOpenConns(1)
	conn, err := db.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Raw(func(raw any) error {
		raw.(sqliteDriver.Conn).Raw().UpdateHook(func(_ sqlite3.AuthorizerActionCode, _, updatedTable string, _ int64) {
			if updatedTable == table {
				cancel()
			}
		})
		return nil
	}); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	return func() {
		conn, err := db.db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if err := conn.Raw(func(raw any) error {
			raw.(sqliteDriver.Conn).Raw().UpdateHook(nil)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

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

	seedSource(t, db, "wiki.zim", "Wikipedia", []Article{
		{Path: "A/Linux", Title: "Linux", Body: "Linux is an operating system kernel"},
		{Path: "A/Go", Title: "Go programming", Body: "Go is a programming language created by Google"},
	})

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

	seedSource(t, db, "wiki.zim", "Wikipedia", []Article{
		{Path: "A/Test", Title: "Test", Body: "test content"},
	})

	_, count, err := db.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("article count = %d", count)
	}

	err = db.PruneSources(context.Background(), map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	_, count, err = db.Stats()
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

	seedSource(t, db, "a.zim", "A", nil)
	seedSource(t, db, "b.zim", "B", nil)

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

	seedSource(t, db, "wiki.zim", "Wikipedia", nil)
	seedSource(t, db, "ifixit.zim", "iFixit", nil)

	fns, err := db.IndexedFilenames()
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 2 {
		t.Errorf("indexed filenames = %d", len(fns))
	}
}

func TestReplaceSourceUpdatesTitle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sid1 := seedSource(t, db, "wiki.zim", "Wikipedia", nil)
	sid2 := seedSource(t, db, "wiki.zim", "Wikipedia Updated", nil)

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

	sid := seedSource(t, db, "test.zim", "Test", []Article{{Path: "/A/One", Title: "One", Body: "body"}})

	// 384-dim embedding = 384 * 4 bytes = 1536 bytes.
	vec := make([]byte, 384*4)
	db.InsertChunkEmbeddings([]ChunkEmbedding{{ArticleID: sid, ChunkIndex: 0, Header: "One", Vector: vec}})

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

	sid := seedSource(t, db, "wiki.zim", "Wikipedia", []Article{
		{Path: "/A/One", Title: "One", Body: "first article"},
		{Path: "/A/Two", Title: "Two", Body: "second article"},
		{Path: "/A/Three", Title: "Three", Body: "third article"},
	})

	// Initially no embeddings.
	count, err := db.EmbeddingCountBySource(sid)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("initial embedding count = %d", count)
	}

	// Get unembedded articles.
	unembedded, err := db.UnembeddedArticlesBySource(sid, 0, 10)
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

	count, err = db.EmbeddingCountBySource(sid)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("embedding count = %d, want 2", count)
	}

	// Only one article should remain unembedded.
	remaining, err := db.UnembeddedArticlesBySource(sid, 0, 10)
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

	sid := seedSource(t, db, "test.zim", "Test", []Article{
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
	var count int64
	err = db.db.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("embedding count = %d, want 3", count)
	}

	// Article should NOT appear as unembedded (it has chunks).
	unembedded, err := db.UnembeddedArticlesBySource(sid, 0, 10)
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

	sid := seedSource(t, db, "test.zim", "Test", []Article{
		{Path: "/A/One", Title: "One", Body: "first article"},
	})

	vec1 := make([]byte, 8)
	vec1[0] = 0x01
	vec2 := make([]byte, 8)
	vec2[0] = 0x02

	// Insert a chunk.
	err = db.InsertChunkEmbeddings([]ChunkEmbedding{
		{ArticleID: sid, ChunkIndex: 0, Header: "Original", Vector: vec1},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Replace with different header and vector.
	err = db.InsertChunkEmbeddings([]ChunkEmbedding{
		{ArticleID: sid, ChunkIndex: 0, Header: "Updated", Vector: vec2},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should still be exactly 1 row (replaced, not duplicated).
	var count int64
	err = db.db.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&count)
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

	sid := seedSource(t, db, "test.zim", "Test", []Article{
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

	sectionsJSON := `[{"heading":"Intro","body":"hello"},{"heading":"Details","body":"world"}]`
	sid := seedSource(t, db, "test.zim", "Test", []Article{
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

func TestReplaceSourceReplacesArticlesAndLegacyEmbeddingsAtomically(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seedSource(t, db, "wiki.zim", "Old", []Article{{Path: "/old", Title: "Old", Body: "old token"}})
	if _, err := db.db.Exec("DROP TABLE embeddings"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`CREATE TABLE embeddings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		article_id INTEGER NOT NULL REFERENCES articles(id),
		chunk_index INTEGER NOT NULL,
		chunk_header TEXT NOT NULL,
		vector BLOB NOT NULL,
		UNIQUE(article_id, chunk_index)
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec("INSERT INTO embeddings (article_id, chunk_index, chunk_header, vector) VALUES (1, 0, 'Old', X'00000000')"); err != nil {
		t.Fatal(err)
	}

	source := Source{Filename: "wiki.zim", Title: "New", UUID: "new-uuid", Size: 123, ModTimeNS: 456}
	if err := db.ReplaceSource(context.Background(), source, []Article{{Path: "/new", Title: "New", Body: "new token"}}); err != nil {
		t.Fatal(err)
	}
	if hits, err := db.Search("old", 10); err != nil || len(hits) != 0 {
		t.Fatalf("old hits = %#v, %v", hits, err)
	}
	if hits, err := db.Search("new", 10); err != nil || len(hits) != 1 {
		t.Fatalf("new hits = %#v, %v", hits, err)
	}
	var embeddings int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&embeddings); err != nil {
		t.Fatal(err)
	}
	if embeddings != 0 {
		t.Fatalf("embeddings = %d, want 0", embeddings)
	}
}

func TestReplaceSourceStreamRollsBackPartialArticles(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedSource(t, db, "wiki.zim", "Old", []Article{{Path: "/old", Title: "Old", Body: "original"}})
	interrupted := errors.New("archive changed during extraction")
	err = db.ReplaceSourceStream(context.Background(), Source{Filename: "wiki.zim", Title: "New"}, func(yield func(Article, error) bool) {
		if yield(Article{Path: "/new", Title: "New", Body: "replacement"}, nil) {
			yield(Article{}, interrupted)
		}
	})
	if !errors.Is(err, interrupted) {
		t.Fatalf("replace returned %v", err)
	}
	if hits, err := db.Search("original", 10); err != nil || len(hits) != 1 {
		t.Fatalf("original hits = %v, %v", hits, err)
	}
	if hits, err := db.Search("replacement", 10); err != nil || len(hits) != 0 {
		t.Fatalf("partial replacement hits = %v, %v", hits, err)
	}
}

func TestPruneSourcesRemovesAbsentSources(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.ReplaceSource(context.Background(), Source{Filename: "keep.zim", Title: "Keep", UUID: "keep", Size: 1, ModTimeNS: 1}, []Article{{Path: "/keep", Title: "Keep", Body: "keep"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceSource(context.Background(), Source{Filename: "remove.zim", Title: "Remove", UUID: "remove", Size: 1, ModTimeNS: 1}, []Article{{Path: "/remove", Title: "Remove", Body: "remove"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.PruneSources(context.Background(), map[string]bool{"keep.zim": true}); err != nil {
		t.Fatal(err)
	}
	filenames, err := db.IndexedFilenames()
	if err != nil {
		t.Fatal(err)
	}
	if len(filenames) != 1 || filenames[0] != "keep.zim" {
		t.Fatalf("filenames = %v", filenames)
	}
}

func TestReplaceSourceMarksCompleteSemanticIndexBuilding(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SetMeta("semantic_state", "complete"); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceSource(context.Background(), Source{Filename: "wiki.zim", Title: "Wiki", UUID: "uuid", Size: 1, ModTimeNS: 1}, []Article{{Path: "/a", Title: "A", Body: "body"}}); err != nil {
		t.Fatal(err)
	}
	state, err := db.SemanticState()
	if err != nil {
		t.Fatal(err)
	}
	if state != "building" {
		t.Fatalf("semantic state = %q, want building", state)
	}
}

func TestReplaceSourceCancellationRollsBackTransaction(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.ReplaceSource(context.Background(), Source{Filename: "wiki.zim", Title: "Old", UUID: "old", Size: 1, ModTimeNS: 1}, []Article{{Path: "/old", Title: "Old", Body: "old"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	clearHook := cancelOnUpdate(t, db, "articles", cancel)
	defer clearHook()
	err = db.ReplaceSource(ctx, Source{Filename: "wiki.zim", Title: "New", UUID: "new", Size: 2, ModTimeNS: 2}, []Article{{Path: "/new", Title: "New", Body: "new"}, {Path: "/newer", Title: "Newer", Body: "newer"}})
	if err == nil {
		t.Fatal("ReplaceSource succeeded after cancellation")
	}
	if hits, err := db.Search("old", 10); err != nil || len(hits) != 1 {
		t.Fatalf("old source after cancelled replacement = %#v, %v", hits, err)
	}
	if hits, err := db.Search("new", 10); err != nil || len(hits) != 0 {
		t.Fatalf("new source after cancelled replacement = %#v, %v", hits, err)
	}
}

func TestSetMetaContextCancellationPreservesPreviousValue(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SetMeta("indexed_at", "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`CREATE TABLE cancellation_barrier (value TEXT);
		CREATE TRIGGER cancel_before_meta BEFORE UPDATE ON meta BEGIN
			INSERT INTO cancellation_barrier VALUES (NEW.value);
		END`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	clearHook := cancelOnUpdate(t, db, "cancellation_barrier", cancel)
	defer clearHook()
	if err := db.SetMetaContext(ctx, "indexed_at", "new"); err == nil {
		t.Fatal("SetMetaContext succeeded after cancellation")
	}
	value, err := db.GetMeta("indexed_at")
	if err != nil {
		t.Fatal(err)
	}
	if value != "old" {
		t.Fatalf("indexed_at = %q, want old", value)
	}
}

func TestSemanticContractClearsIncompatibleVectorsAndRequiresCoverage(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sourceID := seedSource(t, db, "wiki.zim", "Wiki", []Article{{Path: "/a", Title: "A", Body: "body"}})
	if err := db.InsertChunkEmbeddings([]ChunkEmbedding{{ArticleID: sourceID, ChunkIndex: 0, Header: "A", Vector: make([]byte, 8)}}); err != nil {
		t.Fatal(err)
	}
	old := SemanticContract{Version: 1, Model: "models/embed/old.gguf", ModelSHA256: "old", Dimensions: 2}
	if _, err := db.BeginSemanticContract(context.Background(), old, false); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertChunkEmbeddings([]ChunkEmbedding{{ArticleID: sourceID, ChunkIndex: 0, Header: "A", Vector: make([]byte, 8)}}); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteSemanticContract(context.Background(), old, SemanticCompletionMetadata{}); err != nil {
		t.Fatal(err)
	}
	newContract := SemanticContract{Version: 1, Model: "models/embed/new.gguf", ModelSHA256: "new", Dimensions: 2}
	cleared, err := db.BeginSemanticContract(context.Background(), newContract, false)
	if err != nil || !cleared {
		t.Fatalf("BeginSemanticContract = %v, %v", cleared, err)
	}
	if count, err := db.EmbeddingCountBySource(sourceID); err != nil || count != 0 {
		t.Fatalf("vectors after incompatible contract = %d, %v", count, err)
	}
	if err := db.CompleteSemanticContract(context.Background(), newContract, SemanticCompletionMetadata{}); err == nil {
		t.Fatal("completed contract without article coverage")
	}
	if state, err := db.SemanticState(); err != nil || state != "building" {
		t.Fatalf("semantic state = %q, %v", state, err)
	}
}

func TestSemanticContractResumesMatchingBuildingWork(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	contract := SemanticContract{Version: 1, Model: "models/embed/model.gguf", ModelSHA256: "hash", Dimensions: 2}
	if cleared, err := db.BeginSemanticContract(context.Background(), contract, false); err != nil || !cleared {
		t.Fatalf("first begin = %v, %v", cleared, err)
	}
	if cleared, err := db.BeginSemanticContract(context.Background(), contract, false); err != nil || cleared {
		t.Fatalf("resume begin = %v, %v", cleared, err)
	}
}

func TestSemanticCompletionCancellationRollsBackAllPublicationMetadata(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	articleID := seedSource(t, db, "wiki.zim", "Wiki", []Article{{Path: "/a", Title: "A", Body: "body"}})
	contract := SemanticContract{Version: 1, Model: "models/embed/model.gguf", ModelSHA256: "hash", Dimensions: 2}
	if _, err := db.BeginSemanticContract(context.Background(), contract, false); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertChunkEmbeddings([]ChunkEmbedding{{ArticleID: articleID, ChunkIndex: 0, Header: "A", Vector: make([]byte, 8)}}); err != nil {
		t.Fatal(err)
	}
	old := map[string]string{
		"semantic_indexed_at": "old-time", "embedding_model": "old-model",
		"embedding_doc_prefix": "old-doc", "embedding_query_prefix": "old-query", "embedding_dims": "2",
	}
	for key, value := range old {
		if err := db.SetMeta(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.db.Exec(`CREATE TABLE completion_barrier (value TEXT);
		CREATE TRIGGER cancel_completion BEFORE UPDATE ON meta BEGIN
			INSERT INTO completion_barrier VALUES (NEW.key);
		END`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	clearHook := cancelOnUpdate(t, db, "completion_barrier", cancel)
	defer clearHook()
	if err := db.CompleteSemanticContract(ctx, contract, SemanticCompletionMetadata{IndexedAt: "new-time", Model: "new-model"}); err == nil {
		t.Fatal("completion succeeded after cancellation")
	}
	if state, err := db.SemanticState(); err != nil || state != "building" {
		t.Fatalf("semantic state after cancelled completion = %q, %v", state, err)
	}
	for key, want := range old {
		got, err := db.GetMeta(key)
		if err != nil || got != want {
			t.Fatalf("%s after cancelled completion = %q, %v; want %q", key, got, err, want)
		}
	}
}

func TestLegacySourceWithoutIdentityIsNotCurrent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.db.Exec("INSERT INTO sources (filename, title, indexed_at) VALUES ('legacy.zim', 'Legacy', 'now')"); err != nil {
		t.Fatal(err)
	}
	sources, err := db.IndexedSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].IdentityCurrent(Source{Filename: "legacy.zim", UUID: "uuid", Size: 1, ModTimeNS: 1}) {
		t.Fatalf("legacy source unexpectedly current: %#v", sources)
	}
}

func TestSourceNoteIsStoredReplacedAndMigrated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search.db")
	// A database from before documents has no note column.
	legacy, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`CREATE TABLE sources (id INTEGER PRIMARY KEY AUTOINCREMENT, filename TEXT UNIQUE NOT NULL, title TEXT NOT NULL, indexed_at TEXT, source_uuid TEXT, source_size INTEGER, source_mtime_ns INTEGER);
		INSERT INTO sources (filename, title, source_uuid, source_size, source_mtime_ns) VALUES ('wiki.zim', 'Wiki', 'uuid', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	legacy.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatalf("opening a database without the note column: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	scanned := Source{Filename: "books/manuals/trailer.pdf", Title: "Trailer", UUID: "sha256:1", Size: 2, ModTimeNS: 3, Note: "scanned: no text layer, needs OCR"}
	if err := db.ReplaceSource(ctx, scanned, nil); err != nil {
		t.Fatal(err)
	}
	sources, err := db.IndexedSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0] != scanned || sources[1].Note != "" || !sources[1].IdentityCurrent(Source{UUID: "uuid", Size: 1, ModTimeNS: 1}) {
		t.Fatalf("sources = %+v", sources)
	}

	// Indexing the document again after OCR clears the note.
	scanned.Note, scanned.UUID = "", "sha256:2"
	if err := db.ReplaceSource(ctx, scanned, []Article{{Path: "page/1", Title: "Trailer, p. 1", Body: "towing capacity"}}); err != nil {
		t.Fatal(err)
	}
	if sources, _ = db.IndexedSources(); sources[0] != scanned {
		t.Fatalf("source after replacement = %+v", sources[0])
	}
}
