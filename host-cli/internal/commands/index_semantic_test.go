package commands

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

type fakeEmbeddingServer struct {
	dimensions int
	err        error
}

func (s *fakeEmbeddingServer) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	vectors := make([][]float32, len(texts))
	for i := range vectors {
		vectors[i] = make([]float32, s.dimensions)
	}
	return vectors, nil
}

func (s *fakeEmbeddingServer) Stop() {}

func installFakeEmbeddingServer(t *testing.T, server embeddingServer, err error) {
	t.Helper()
	old := startEmbeddingServer
	startEmbeddingServer = func(context.Context, string, string, int) (embeddingServer, error) { return server, err }
	t.Cleanup(func() { startEmbeddingServer = old })
}

func semanticCommandFixture(t *testing.T) (string, *searchdb.DB, searchdb.SemanticContract, int64) {
	t.Helper()
	root := t.TempDir()
	modelPath := filepath.Join(root, "models", "embed", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modelPath, []byte("test embedding model"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data"), 0755); err != nil {
		t.Fatal(err)
	}
	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ReplaceSource(context.Background(), searchdb.Source{Filename: "wiki.zim", Title: "Wiki", UUID: "wiki", Size: 1, ModTimeNS: 1}, []searchdb.Article{{Path: "/a", Title: "Article", Body: "semantic body"}}); err != nil {
		t.Fatal(err)
	}
	sources, err := db.Sources()
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources = %#v, %v", sources, err)
	}
	hash, err := hashEmbeddingModel(context.Background(), modelPath)
	if err != nil {
		t.Fatal(err)
	}
	return root, db, searchdb.SemanticContract{Version: 1, Model: "models/embed/model.gguf", ModelSHA256: hash, Dimensions: 2}, sources[0].ID
}

func completeOldSemanticContract(t *testing.T, db *searchdb.DB, contract searchdb.SemanticContract, articleID int64) {
	t.Helper()
	if _, err := db.BeginSemanticContract(context.Background(), contract, false); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertChunkEmbeddings([]searchdb.ChunkEmbedding{{ArticleID: articleID, ChunkIndex: 0, Header: "Article", Vector: make([]byte, 8)}}); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteSemanticContract(context.Background(), contract, semanticMetadata("old")); err != nil {
		t.Fatal(err)
	}
}

func TestIndexSemanticStartupFailurePreservesCompleteContract(t *testing.T) {
	root, db, old, articleID := semanticCommandFixture(t)
	completeOldSemanticContract(t, db, old, articleID)
	installFakeEmbeddingServer(t, nil, errors.New("server unavailable"))

	if err := IndexSemantic(context.Background(), root, false, io.Discard, nil); err == nil {
		t.Fatal("IndexSemantic succeeded after startup failure")
	}
	contract, state, err := db.LoadSemanticContract()
	if err != nil || state != "complete" || contract != old {
		t.Fatalf("contract after failed startup = %#v, %q, %v", contract, state, err)
	}
	if count, err := db.EmbeddingCountBySource(articleID); err != nil || count != 1 {
		t.Fatalf("vectors after failed startup = %d, %v", count, err)
	}
}

func TestIndexSemanticInvalidConfiguredDimensionsPreservesCompleteContract(t *testing.T) {
	root, db, old, articleID := semanticCommandFixture(t)
	completeOldSemanticContract(t, db, old, articleID)
	if err := os.WriteFile(filepath.Join(root, "models", "embed", "model.embedding.json"), []byte(`{"dims":3,"matryoshka":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	installFakeEmbeddingServer(t, &fakeEmbeddingServer{dimensions: 2}, nil)

	if err := IndexSemantic(context.Background(), root, false, io.Discard, nil); err == nil {
		t.Fatal("IndexSemantic accepted configured dimensions above native output")
	}
	contract, state, err := db.LoadSemanticContract()
	if err != nil || state != "complete" || contract != old {
		t.Fatalf("contract after invalid dimensions = %#v, %q, %v", contract, state, err)
	}
	if count, err := db.EmbeddingCountBySource(articleID); err != nil || count != 1 {
		t.Fatalf("vectors after invalid dimensions = %d, %v", count, err)
	}
}

func TestIndexSemanticResumeCompletesCancelledCommittedBatch(t *testing.T) {
	root, db, contract, articleID := semanticCommandFixture(t)
	installFakeEmbeddingServer(t, &fakeEmbeddingServer{dimensions: 2}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := IndexSemantic(ctx, root, false, io.Discard, func(p SemanticProgress) {
		if p.Status == "embedding" && p.Current == 1 {
			cancel()
		}
	}); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled indexing error = %v, want context.Canceled", err)
	}
	stored, state, err := db.LoadSemanticContract()
	if err != nil || state != "building" || stored != contract {
		t.Fatalf("state after committed batch cancellation = %#v, %q, %v", stored, state, err)
	}
	if count, err := db.EmbeddingCountBySource(articleID); err != nil || count != 1 {
		t.Fatalf("retained vectors = %d, %v", count, err)
	}
	if err := IndexSemantic(context.Background(), root, false, io.Discard, nil); err != nil {
		t.Fatalf("ordinary resume: %v", err)
	}
	stored, state, err = db.LoadSemanticContract()
	if err != nil || state != "complete" || stored != contract {
		t.Fatalf("state after no-work resume = %#v, %q, %v", stored, state, err)
	}
}

func TestIndexSemanticSelectedPreservesCoverageOnResume(t *testing.T) {
	root, db, _, excludedID := semanticCommandFixture(t)
	ctx := context.Background()
	if err := db.ReplaceSource(ctx, searchdb.Source{Filename: "books/guide.pdf", Title: "Guide"}, []searchdb.Article{
		{Path: "page/1", Title: "Preserving food", Body: "A handbook for preserving food."},
	}); err != nil {
		t.Fatal(err)
	}
	installFakeEmbeddingServer(t, &fakeEmbeddingServer{dimensions: 2}, nil)
	if err := IndexSemanticSelected(ctx, root, false, io.Discard, nil, []string{"books/"}, "model.gguf"); err != nil {
		t.Fatal(err)
	}
	if err := IndexSemantic(ctx, root, false, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	contract, state, err := db.LoadSemanticContract()
	if err != nil || state != "complete" || contract.SourceSelectors != `["books/"]` {
		t.Fatalf("selection after resume = %q, %q, %v", contract.SourceSelectors, state, err)
	}
	if count, err := db.EmbeddingCountBySource(excludedID); err != nil || count != 0 {
		t.Fatalf("unselected Wiki articles with embeddings = %d, %v", count, err)
	}
}

func TestPrepareEmbeddingTextTruncatesLongBody(t *testing.T) {
	body := strings.Repeat("token ", 800)

	// With prefix (nomic-style).
	text := prepareEmbeddingText("search_document: ", "Permacomputing", body)
	if !strings.HasPrefix(text, "search_document: ") {
		t.Fatalf("missing search_document prefix: %q", text[:min(len(text), 32)])
	}
	if !strings.Contains(text, "Permacomputing") {
		t.Fatalf("title missing from prepared text: %q", text)
	}
	if got := len([]rune(text)); got > embeddingTextMaxRunes {
		t.Fatalf("prepared text length = %d, want <= %d", got, embeddingTextMaxRunes)
	}
	if !strings.HasSuffix(text, "...") {
		t.Fatalf("expected truncated text to end with ellipsis: %q", text[len(text)-16:])
	}

	// Without prefix (no task-prefix model).
	textNoPrefix := prepareEmbeddingText("", "Permacomputing", body)
	if strings.HasPrefix(textNoPrefix, "search_document: ") {
		t.Fatalf("should not have prefix when empty: %q", textNoPrefix[:min(len(textNoPrefix), 32)])
	}
	if !strings.HasPrefix(textNoPrefix, "Permacomputing") {
		t.Fatalf("title should be first: %q", textNoPrefix[:min(len(textNoPrefix), 32)])
	}
}

func TestPrepareEmbeddingTextCapsWordCount(t *testing.T) {
	body := strings.Repeat("a ", 1200)

	// With prefix.
	text := prepareEmbeddingText("search_document: ", "Short Tokens", body)
	maxWords := embeddingTextMaxWords + len(strings.Fields("search_document: ")) + len(strings.Fields("Short Tokens"))
	if got := len(strings.Fields(text)); got > maxWords {
		t.Fatalf("prepared text word count = %d, want <= %d", got, maxWords)
	}

	// Without prefix.
	textNoPrefix := prepareEmbeddingText("", "Short Tokens", body)
	maxWordsNoPrefix := embeddingTextMaxWords + len(strings.Fields("Short Tokens"))
	if got := len(strings.Fields(textNoPrefix)); got > maxWordsNoPrefix {
		t.Fatalf("no-prefix word count = %d, want <= %d", got, maxWordsNoPrefix)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- buildChunks tests ---

func sectionsJSON(sections []struct{ Heading, Body string }) string {
	type sec struct {
		Heading string `json:"heading"`
		Body    string `json:"body"`
	}
	out := make([]sec, len(sections))
	for i, s := range sections {
		out[i] = sec{Heading: s.Heading, Body: s.Body}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func TestBuildChunksMultiSection(t *testing.T) {
	secs := []struct{ Heading, Body string }{
		{"Introduction", strings.Repeat("word ", 100)},
		{"History", strings.Repeat("word ", 120)},
		{"Usage", strings.Repeat("word ", 90)},
	}
	chunks := buildChunks("search_document: ", "Water Filter", sectionsJSON(secs))

	if len(chunks) == 0 {
		t.Fatal("expected multiple chunks, got nil")
	}

	// Each chunk should have the correct header format.
	for _, c := range chunks {
		if !strings.HasPrefix(c.Header, "Water Filter") {
			t.Errorf("chunk header should start with article title, got %q", c.Header)
		}
		if !strings.HasPrefix(c.Text, "search_document: ") {
			t.Errorf("chunk text should start with doc prefix, got %q", c.Text[:min(len(c.Text), 40)])
		}
	}

	// At least one chunk should have a " > " separator in the header.
	found := false
	for _, c := range chunks {
		if strings.Contains(c.Header, " > ") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one chunk with ' > ' separator in header")
	}
}

func TestBuildChunksSingleShortSection(t *testing.T) {
	secs := []struct{ Heading, Body string }{
		{"Overview", "A short body with just a few words."},
	}
	chunks := buildChunks("", "Compass", sectionsJSON(secs))

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Header != "Compass > Overview" {
		t.Errorf("header = %q, want %q", chunks[0].Header, "Compass > Overview")
	}
	if !strings.Contains(chunks[0].Text, "A short body") {
		t.Errorf("text should contain body content, got %q", chunks[0].Text)
	}
}

func TestBuildChunksEmptyJSON(t *testing.T) {
	if chunks := buildChunks("prefix: ", "Title", ""); chunks != nil {
		t.Fatalf("expected nil for empty string, got %d chunks", len(chunks))
	}
	if chunks := buildChunks("prefix: ", "Title", "[]"); chunks != nil {
		t.Fatalf("expected nil for empty array, got %d chunks", len(chunks))
	}
	if chunks := buildChunks("prefix: ", "Title", "invalid json"); chunks != nil {
		t.Fatalf("expected nil for invalid JSON, got %d chunks", len(chunks))
	}
}

func TestBuildChunksMergesSmallSections(t *testing.T) {
	// Each section has < 80 words, so adjacent ones should be merged.
	secs := []struct{ Heading, Body string }{
		{"Intro", "First short section."},
		{"Note", "Second short section."},
		{"More", "Third short section."},
	}
	chunks := buildChunks("", "Article", sectionsJSON(secs))

	// All three are tiny (<80 words each), so the second and third should
	// merge into the first, yielding a single chunk.
	if len(chunks) != 1 {
		t.Fatalf("expected 1 merged chunk, got %d", len(chunks))
	}
	// The merged chunk should contain content from all sections.
	if !strings.Contains(chunks[0].Text, "First short section") {
		t.Error("merged chunk missing first section body")
	}
	if !strings.Contains(chunks[0].Text, "Second short section") {
		t.Error("merged chunk missing second section body")
	}
	if !strings.Contains(chunks[0].Text, "Third short section") {
		t.Error("merged chunk missing third section body")
	}
}

func TestBuildChunksSplitsLargeSections(t *testing.T) {
	// Build a section with >500 words using multiple paragraphs.
	var paras []string
	for i := 0; i < 6; i++ {
		paras = append(paras, strings.Repeat("word ", 100))
	}
	largeBody := strings.Join(paras, "\n\n")

	secs := []struct{ Heading, Body string }{
		{"Details", largeBody},
	}
	chunks := buildChunks("search_document: ", "Big Article", sectionsJSON(secs))

	if len(chunks) < 2 {
		t.Fatalf("expected large section to be split into >= 2 chunks, got %d", len(chunks))
	}

	// All chunks should share the same header.
	for _, c := range chunks {
		if c.Header != "Big Article > Details" {
			t.Errorf("chunk header = %q, want %q", c.Header, "Big Article > Details")
		}
	}

	// Each chunk body should be <= ~500 words (with some tolerance for the
	// last paragraph that pushed it over).
	for i, c := range chunks {
		// Extract body after the header prefix.
		bodyStart := strings.Index(c.Text, ": ")
		if bodyStart < 0 {
			t.Fatalf("chunk %d text has no ': ' separator", i)
		}
		body := c.Text[bodyStart+2:]
		wc := wordCount(body)
		if wc > 600 { // generous upper bound: 500 + one full paragraph
			t.Errorf("chunk %d has %d words, expected <= ~500", i, wc)
		}
	}
}

func TestBuildChunksEmptyHeadingUsesTitle(t *testing.T) {
	secs := []struct{ Heading, Body string }{
		{"", "Some content without a heading."},
	}
	chunks := buildChunks("", "My Article", sectionsJSON(secs))

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	// When heading is empty, header should just be the title (no " > ").
	if chunks[0].Header != "My Article" {
		t.Errorf("header = %q, want %q", chunks[0].Header, "My Article")
	}
}

func TestBuildChunksSingleOversizedParagraph(t *testing.T) {
	// 100 sentences of 20 words each — 2000 words total — all on one line
	// (no "\n\n" breaks). The paragraph-split path must fall through to
	// sentence splitting so no single chunk exceeds the limit.
	var sentences []string
	for i := 0; i < 100; i++ {
		sentences = append(sentences, strings.Repeat("word ", 19)+"end.")
	}
	single := strings.Join(sentences, " ")

	secs := []struct{ Heading, Body string }{{"Overview", single}}
	chunks := buildChunks("search_document: ", "Monolith", sectionsJSON(secs))

	if len(chunks) < 4 {
		t.Fatalf("expected ≥4 chunks for 2000-word single paragraph, got %d", len(chunks))
	}
	for i, c := range chunks {
		bodyStart := strings.Index(c.Text, ": ")
		if bodyStart < 0 {
			t.Fatalf("chunk %d has no ': ' separator", i)
		}
		body := c.Text[bodyStart+2:]
		if wc := wordCount(body); wc > 600 {
			t.Errorf("chunk %d has %d words, expected ≤600", i, wc)
		}
	}
}

func TestBuildChunksOversizedNoSentences(t *testing.T) {
	// 2000 words, no punctuation, no "\n\n" — forces hard word-count split.
	body := strings.Repeat("word ", 2000)
	secs := []struct{ Heading, Body string }{{"Dump", body}}
	chunks := buildChunks("", "Bare", sectionsJSON(secs))

	if len(chunks) < 3 {
		t.Fatalf("expected ≥3 chunks for hard-split fallback, got %d", len(chunks))
	}
	for i, c := range chunks {
		bodyStart := strings.Index(c.Text, ": ")
		if bodyStart < 0 {
			t.Fatalf("chunk %d has no ': ' separator", i)
		}
		body := c.Text[bodyStart+2:]
		if wc := wordCount(body); wc > chunkWordLimit+1 {
			t.Errorf("chunk %d has %d words, expected ≤%d", i, wc, chunkWordLimit)
		}
	}
}

// --- preflightEmbeddingSpec tests ---

func TestPreflightEmbeddingSpecNomic(t *testing.T) {
	spec := catalog.EmbeddingSpec{
		DocPrefix:      "search_document: ",
		QueryPrefix:    "search_query: ",
		Dims:           256,
		Matryoshka:     true,
		MaxInputTokens: 2048,
	}
	if err := preflightEmbeddingSpec(spec, "nomic-embed-text-v1.5"); err != nil {
		t.Fatalf("unexpected error for valid Nomic spec: %v", err)
	}
}

func TestPreflightEmbeddingSpecContextTooSmall(t *testing.T) {
	// MiniLM-like: 512 tokens is below chunkWordLimit (500) × 1.5 = 750.
	spec := catalog.EmbeddingSpec{MaxInputTokens: 512}
	err := preflightEmbeddingSpec(spec, "all-minilm-l6-v2")
	if err == nil {
		t.Fatal("expected error for undersized context window")
	}
	if !strings.Contains(err.Error(), "all-minilm-l6-v2") {
		t.Errorf("error should name the model: %v", err)
	}
	if !strings.Contains(err.Error(), "512") {
		t.Errorf("error should mention token count: %v", err)
	}
}

func TestPreflightEmbeddingSpecDimsWithoutMatryoshka(t *testing.T) {
	// Truncating a non-matryoshka model's vector destroys its semantics.
	spec := catalog.EmbeddingSpec{
		Dims:           256,
		Matryoshka:     false,
		MaxInputTokens: 2048,
	}
	err := preflightEmbeddingSpec(spec, "some-model")
	if err == nil {
		t.Fatal("expected error for dims>0 without matryoshka")
	}
	if !strings.Contains(err.Error(), "matryoshka") {
		t.Errorf("error should mention matryoshka: %v", err)
	}
}

func TestPreflightEmbeddingSpecLegacyEmpty(t *testing.T) {
	// A recipe without the new fields (pre-this-change) should still load.
	// No dims → no truncation to guard. No MaxInputTokens → skip context check.
	spec := catalog.EmbeddingSpec{
		DocPrefix:   "search_document: ",
		QueryPrefix: "search_query: ",
	}
	if err := preflightEmbeddingSpec(spec, "legacy"); err != nil {
		t.Fatalf("unexpected error for legacy spec: %v", err)
	}
}

func TestPreflightEmbeddingSpecMatryoshkaNoDims(t *testing.T) {
	// Matryoshka flag set but no dims → fine, truncation simply won't run.
	spec := catalog.EmbeddingSpec{
		Matryoshka:     true,
		MaxInputTokens: 2048,
	}
	if err := preflightEmbeddingSpec(spec, "matryoshka-native"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWordCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"one two three", 3},
		{"  spaced   out  ", 2},
	}
	for _, tc := range tests {
		if got := wordCount(tc.input); got != tc.want {
			t.Errorf("wordCount(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
