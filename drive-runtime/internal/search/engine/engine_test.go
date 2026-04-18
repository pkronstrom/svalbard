package engine

import (
	"database/sql"
	"encoding/binary"
	"math"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// setupTestDB creates an in-memory SQLite database with the search.db schema
// and returns the db connection and a cleanup function.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Force single connection so all goroutines share the same in-memory database.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	schema := `
		CREATE TABLE sources (
			id INTEGER PRIMARY KEY,
			filename TEXT NOT NULL
		);

		CREATE TABLE articles (
			id INTEGER PRIMARY KEY,
			source_id INTEGER NOT NULL REFERENCES sources(id),
			path TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT ''
		);

		CREATE VIRTUAL TABLE articles_fts USING fts5(
			title, body,
			content='articles',
			content_rowid='id'
		);

		CREATE TABLE embeddings (
			id INTEGER PRIMARY KEY,
			article_id INTEGER NOT NULL REFERENCES articles(id),
			chunk_header TEXT NOT NULL DEFAULT '',
			vector BLOB NOT NULL
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// float32ToBlob converts a float32 slice to a little-endian byte blob.
func float32ToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// insertSource inserts a source and returns its ID.
func insertSource(t *testing.T, db *sql.DB, filename string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO sources (filename) VALUES (?)", filename)
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

// insertArticle inserts an article, populates FTS, and returns its ID.
func insertArticle(t *testing.T, db *sql.DB, sourceID int, path, title, body string) int {
	t.Helper()
	res, err := db.Exec(
		"INSERT INTO articles (source_id, path, title, body) VALUES (?, ?, ?, ?)",
		sourceID, path, title, body,
	)
	if err != nil {
		t.Fatalf("insert article: %v", err)
	}
	id, _ := res.LastInsertId()

	// Populate FTS index
	if _, err := db.Exec(
		"INSERT INTO articles_fts (rowid, title, body) VALUES (?, ?, ?)",
		id, title, body,
	); err != nil {
		t.Fatalf("insert fts: %v", err)
	}
	return int(id)
}

// insertEmbedding inserts an embedding for an article.
func insertEmbedding(t *testing.T, db *sql.DB, articleID int, header string, vec []float32) {
	t.Helper()
	blob := float32ToBlob(vec)
	if _, err := db.Exec(
		"INSERT INTO embeddings (article_id, chunk_header, vector) VALUES (?, ?, ?)",
		articleID, header, blob,
	); err != nil {
		t.Fatalf("insert embedding: %v", err)
	}
}

func TestKeywordSearch(t *testing.T) {
	db := setupTestDB(t)
	srcID := insertSource(t, db, "wiki.zim")

	insertArticle(t, db, srcID, "/Water_purification", "Water purification",
		"Water purification is the process of removing contaminants from water.")
	insertArticle(t, db, srcID, "/Fire_starting", "Fire starting",
		"Fire starting techniques include friction, flint and steel.")
	insertArticle(t, db, srcID, "/Shelter_building", "Shelter building",
		"Building a shelter protects against wind, rain, and cold.")

	eng := New(db)

	results, err := eng.Keyword("water", 10)
	if err != nil {
		t.Fatalf("keyword search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Water purification" {
		t.Errorf("expected title 'Water purification', got %q", results[0].Title)
	}
	if results[0].Filename != "wiki.zim" {
		t.Errorf("expected filename 'wiki.zim', got %q", results[0].Filename)
	}
	if results[0].Path != "/Water_purification" {
		t.Errorf("expected path '/Water_purification', got %q", results[0].Path)
	}

	// Search for "fire" should return fire starting article
	results, err = eng.Keyword("fire", 10)
	if err != nil {
		t.Fatalf("keyword search fire: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for fire, got %d", len(results))
	}
	if results[0].Title != "Fire starting" {
		t.Errorf("expected 'Fire starting', got %q", results[0].Title)
	}

	// Search matching multiple articles via body text
	results, err = eng.Keyword("building", 10)
	if err != nil {
		t.Fatalf("keyword search building: %v", err)
	}
	if len(results) < 1 {
		t.Fatalf("expected at least 1 result for 'building', got %d", len(results))
	}
}

func TestRRFMerge(t *testing.T) {
	ftsResults := []Result{
		{ArticleID: 1, Title: "Article 1"},
		{ArticleID: 2, Title: "Article 2"},
		{ArticleID: 3, Title: "Article 3"},
	}
	vecResults := []scoredChunk{
		{articleID: 2, header: "Section A", score: 0.95},
		{articleID: 4, header: "Section B", score: 0.90},
		{articleID: 1, header: "Section C", score: 0.85},
	}

	merged := rrfMerge(ftsResults, vecResults, 60)

	// Articles 1 and 2 appear in both lists, so they should have higher RRF scores
	if len(merged) != 4 {
		t.Fatalf("expected 4 merged entries, got %d", len(merged))
	}

	// Article 2 is rank 0 in vec (1/60) + rank 1 in FTS (1/61) = highest combined
	// Article 1 is rank 0 in FTS (1/60) + rank 2 in vec (1/62)
	// Both should be in top 2
	topIDs := map[int]bool{merged[0].articleID: true, merged[1].articleID: true}
	if !topIDs[1] || !topIDs[2] {
		t.Errorf("expected articles 1 and 2 in top 2, got %d and %d", merged[0].articleID, merged[1].articleID)
	}

	// Article 2 should be #1 (highest combined score)
	// FTS rank 1: 1/61, Vec rank 0: 1/60 => 1/61 + 1/60
	// Article 1: FTS rank 0: 1/60, Vec rank 2: 1/62 => 1/60 + 1/62
	// 1/61 + 1/60 > 1/60 + 1/62
	if merged[0].articleID != 2 {
		t.Errorf("expected article 2 as top result, got %d", merged[0].articleID)
	}

	// Verify chunk header propagated from vec results
	for _, e := range merged {
		if e.articleID == 2 && e.header != "Section A" {
			t.Errorf("expected header 'Section A' for article 2, got %q", e.header)
		}
	}
}

func TestApplyMMR(t *testing.T) {
	// Create candidates with known scores and vectors
	// Vectors are normalized so dot product = cosine similarity
	candidates := []rrfEntry{
		{articleID: 1, score: 1.0},
		{articleID: 2, score: 0.9},
		{articleID: 3, score: 0.8},
		{articleID: 4, score: 0.7},
	}

	// Article 1 and 2 have very similar vectors, 3 and 4 are different
	vectors := map[int][]float32{
		1: {1.0, 0.0, 0.0},
		2: {0.99, 0.1, 0.0},  // very similar to 1
		3: {0.0, 1.0, 0.0},   // orthogonal to 1
		4: {0.0, 0.0, 1.0},   // orthogonal to both
	}

	result := applyMMR(candidates, vectors, 0.7, 3)

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	// First should be article 1 (highest RRF score, selected first)
	if result[0].articleID != 1 {
		t.Errorf("expected article 1 first, got %d", result[0].articleID)
	}

	// Article 2 is similar to 1, so MMR should prefer 3 or 4 over 2 for second slot
	// Article 3: rel=0.8, maxSim~0 => mmr = 0.7*0.8 - 0.3*0 = 0.56
	// Article 2: rel=0.9, maxSim~0.99 => mmr = 0.7*0.9 - 0.3*0.99 = 0.63 - 0.297 = 0.333
	// Article 4: rel=0.7, maxSim~0 => mmr = 0.7*0.7 - 0.3*0 = 0.49
	// So order should be: 1, 3, 2 or 1, 3, 4
	if result[1].articleID == 2 {
		t.Errorf("MMR should have penalized article 2 (similar to 1), but it was selected second")
	}
}

func TestDedupByArticle(t *testing.T) {
	entries := []rrfEntry{
		{articleID: 1, header: "Intro", score: 0.9},
		{articleID: 2, header: "A", score: 0.8},
		{articleID: 1, header: "Details", score: 0.7},
		{articleID: 3, header: "B", score: 0.6},
		{articleID: 2, header: "C", score: 0.5},
	}

	deduped := dedupByArticle(entries)

	if len(deduped) != 3 {
		t.Fatalf("expected 3 deduped entries, got %d", len(deduped))
	}

	// Should keep first occurrence of each article
	if deduped[0].articleID != 1 || deduped[0].header != "Intro" {
		t.Errorf("expected first entry to be article 1 'Intro', got %d %q", deduped[0].articleID, deduped[0].header)
	}
	if deduped[1].articleID != 2 || deduped[1].header != "A" {
		t.Errorf("expected second entry to be article 2 'A', got %d %q", deduped[1].articleID, deduped[1].header)
	}
	if deduped[2].articleID != 3 {
		t.Errorf("expected third entry to be article 3, got %d", deduped[2].articleID)
	}
}

func TestBuildFTSQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"water purification", `"water"* "purification"*`},
		{"fire", `"fire"*`},
		{"", ""},
		{`word "quoted"`, `"word"* """quoted"""*`},
	}

	for _, tc := range tests {
		got := BuildFTSQuery(tc.input)
		if got != tc.expected {
			t.Errorf("BuildFTSQuery(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float32
		expected float64
	}{
		{
			name:     "simple",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{4.0, 5.0, 6.0},
			expected: 32.0, // 1*4 + 2*5 + 3*6
		},
		{
			name:     "orthogonal",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "same direction",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 2.0, 3.0, 4.0},
			b:        []float32{1.0, 1.0},
			expected: 3.0, // 1*1 + 2*1, stops at shorter
		},
		{
			name:     "empty",
			a:        []float32{},
			b:        []float32{1.0, 2.0},
			expected: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DotProduct(tc.a, tc.b)
			if math.Abs(got-tc.expected) > 1e-6 {
				t.Errorf("DotProduct = %f, want %f", got, tc.expected)
			}
		})
	}
}

func TestBlobToFloat32(t *testing.T) {
	original := []float32{1.0, 2.5, -3.0, 0.0}
	blob := float32ToBlob(original)
	restored := blobToFloat32(blob)

	if len(restored) != len(original) {
		t.Fatalf("expected %d floats, got %d", len(original), len(restored))
	}
	for i := range original {
		if restored[i] != original[i] {
			t.Errorf("index %d: expected %f, got %f", i, original[i], restored[i])
		}
	}

	// Invalid blob length
	if result := blobToFloat32([]byte{1, 2, 3}); result != nil {
		t.Errorf("expected nil for invalid blob, got %v", result)
	}
}

func TestReadArticle(t *testing.T) {
	db := setupTestDB(t)
	srcID := insertSource(t, db, "survival.zim")
	insertArticle(t, db, srcID, "/Water", "Water", "How to find and purify water in the wild.")

	eng := New(db)

	// With .zim suffix
	body, err := eng.ReadArticle("survival.zim", "/Water")
	if err != nil {
		t.Fatalf("ReadArticle: %v", err)
	}
	if body != "How to find and purify water in the wild." {
		t.Errorf("unexpected body: %q", body)
	}

	// Without .zim suffix (should auto-append)
	body, err = eng.ReadArticle("survival", "/Water")
	if err != nil {
		t.Fatalf("ReadArticle without suffix: %v", err)
	}
	if body != "How to find and purify water in the wild." {
		t.Errorf("unexpected body: %q", body)
	}

	// Not found
	_, err = eng.ReadArticle("survival", "/Nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent article")
	}
}

func TestListSources(t *testing.T) {
	db := setupTestDB(t)
	src1 := insertSource(t, db, "wiki.zim")
	src2 := insertSource(t, db, "survival.zim")
	insertArticle(t, db, src1, "/A", "A", "a")
	insertArticle(t, db, src1, "/B", "B", "b")
	insertArticle(t, db, src2, "/C", "C", "c")

	eng := New(db)
	sources, err := eng.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}

	// Sources ordered by filename
	if sources[0].Filename != "survival.zim" || sources[0].ArticleCount != 1 {
		t.Errorf("source 0: got %+v", sources[0])
	}
	if sources[1].Filename != "wiki.zim" || sources[1].ArticleCount != 2 {
		t.Errorf("source 1: got %+v", sources[1])
	}
}

func TestHybridSearch(t *testing.T) {
	db := setupTestDB(t)
	srcID := insertSource(t, db, "wiki.zim")

	id1 := insertArticle(t, db, srcID, "/Water", "Water purification",
		"Water purification removes contaminants from water sources.")
	id2 := insertArticle(t, db, srcID, "/Fire", "Fire starting",
		"Fire starting with friction and flint.")
	id3 := insertArticle(t, db, srcID, "/Shelter", "Shelter building",
		"Building a shelter for protection.")

	// Add embeddings - water-related vector points in one direction,
	// fire in another, shelter in a third
	insertEmbedding(t, db, id1, "Water intro", []float32{0.9, 0.1, 0.0})
	insertEmbedding(t, db, id2, "Fire intro", []float32{0.1, 0.9, 0.0})
	insertEmbedding(t, db, id3, "Shelter intro", []float32{0.0, 0.1, 0.9})

	eng := New(db)

	// Query with both keyword "water" and a vector pointing toward water
	queryVec := []float32{0.9, 0.1, 0.0}
	results, err := eng.Hybrid("water", queryVec, 3)
	if err != nil {
		t.Fatalf("Hybrid: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result from hybrid search")
	}

	// Water should be the top result since both FTS and vector agree
	if results[0].Title != "Water purification" {
		t.Errorf("expected 'Water purification' as top result, got %q", results[0].Title)
	}
}
