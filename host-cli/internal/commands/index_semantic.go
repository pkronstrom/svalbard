package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/embedder"
	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

const embeddingBatchSize = 32

const (
	embeddingTextMaxRunes = 1800
	embeddingTextMaxWords = 220
)

// loadEmbeddingSpec reads the .embedding.json sidecar next to the GGUF model.
// Returns an empty spec (no prefixes) if the sidecar doesn't exist.
func loadEmbeddingSpec(modelPath string) catalog.EmbeddingSpec {
	sidecar := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".embedding.json"
	data, err := os.ReadFile(sidecar)
	if err != nil {
		return catalog.EmbeddingSpec{}
	}
	var spec catalog.EmbeddingSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return catalog.EmbeddingSpec{}
	}
	return spec
}

// SemanticProgress reports per-source embedding progress.
type SemanticProgress struct {
	File    string // ZIM filename (or "" for global events)
	Status  string // "starting", "embedding", "skip", "done", "failed"
	Detail  string // human-readable detail
	Current int64  // articles embedded so far for this source
	Total   int64  // total articles for this source
}

// IndexSemantic generates embeddings for all unembedded articles in the search
// database, reporting progress per source (ZIM file).
//
// The keyword index must already exist (run IndexVault first).
func IndexSemantic(ctx context.Context, root string, force bool, w io.Writer, onProgress func(SemanticProgress)) error {
	dataDir := filepath.Join(root, "data")
	dbPath := filepath.Join(dataDir, "search.db")
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("search database not found — run keyword indexing first")
	}

	db, err := searchdb.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening search database: %w", err)
	}
	defer db.Close()

	sources, err := db.Sources()
	if err != nil {
		return fmt.Errorf("listing sources: %w", err)
	}
	if len(sources) == 0 {
		return fmt.Errorf("no indexed sources — run keyword indexing first")
	}

	if force {
		if err := db.DeleteAllEmbeddings(); err != nil {
			return fmt.Errorf("clearing embeddings: %w", err)
		}
	}

	notify := func(p SemanticProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	// Report all sources upfront so progress view populates immediately.
	anyWork := false
	for _, src := range sources {
		embedded, _ := db.EmbeddingCountBySource(src.ID)
		if embedded >= src.ArticleCount {
			notify(SemanticProgress{
				File: src.Filename, Status: "skip",
				Detail:  "already embedded",
				Current: src.ArticleCount, Total: src.ArticleCount,
			})
		} else {
			anyWork = true
			notify(SemanticProgress{File: src.Filename, Status: "queued"})
		}
	}
	if !anyWork {
		return nil
	}

	// Find the embedding model.
	modelPath, err := embedder.FindEmbeddingModel(root)
	if err != nil {
		return err
	}

	// Derive stable model ID from filename.
	modelID := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))

	// Detect model change — force re-embedding if model switched.
	previousModel, _ := db.GetMeta("embedding_model")
	if previousModel != "" && previousModel != modelID && !force {
		notify(SemanticProgress{Status: "starting", Detail: fmt.Sprintf("Model changed (%s → %s), re-embedding...", previousModel, modelID)})
		if err := db.DeleteAllEmbeddings(); err != nil {
			return fmt.Errorf("clearing old embeddings: %w", err)
		}
	}

	// Load embedding params from recipe sidecar.
	embSpec := loadEmbeddingSpec(modelPath)

	notify(SemanticProgress{Status: "starting", Detail: "Starting embedding server..."})

	server, err := embedder.StartServer(ctx, modelPath, root)
	if err != nil {
		return fmt.Errorf("starting embedding server: %w", err)
	}
	defer server.Stop()

	dims := embSpec.Dims // 0 if not set → no truncation

	// Embed each source separately for per-file progress.
	for _, src := range sources {
		if err := embedSource(ctx, db, server, src, embSpec.DocPrefix, dims, notify); err != nil {
			return err
		}
	}

	if err := db.SetMeta("semantic_indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting semantic_indexed_at: %w", err)
	}
	if err := db.SetMeta("embedding_model", modelID); err != nil {
		return fmt.Errorf("setting embedding_model: %w", err)
	}
	if err := db.SetMeta("embedding_doc_prefix", embSpec.DocPrefix); err != nil {
		return fmt.Errorf("setting embedding_doc_prefix: %w", err)
	}
	if err := db.SetMeta("embedding_query_prefix", embSpec.QueryPrefix); err != nil {
		return fmt.Errorf("setting embedding_query_prefix: %w", err)
	}
	if dims > 0 {
		db.SetMeta("embedding_dims", fmt.Sprintf("%d", dims))
	} else if dimCount, err := db.EmbeddingDims(); err == nil && dimCount > 0 {
		db.SetMeta("embedding_dims", fmt.Sprintf("%d", dimCount))
	}

	return nil
}

// Chunk represents a single embedding unit derived from an article section.
type Chunk struct {
	Header string // "Article Title" or "Article Title > Section Heading"
	Text   string // full embedding input with doc prefix
}

// buildChunks creates multiple chunks from an article's sections JSON.
// Returns nil if sectionsJSON is empty or invalid (caller should fall back to
// single-chunk mode using title+body).
func buildChunks(docPrefix, title, sectionsJSON string) []Chunk {
	if sectionsJSON == "" {
		return nil
	}

	var sections []struct {
		Heading string `json:"heading"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal([]byte(sectionsJSON), &sections); err != nil {
		return nil
	}
	if len(sections) == 0 {
		return nil
	}

	// Merge adjacent small sections (<80 words).
	type merged struct {
		heading string
		body    string
	}
	var groups []merged
	for _, s := range sections {
		body := strings.TrimSpace(s.Body)
		if len(groups) > 0 && wordCount(groups[len(groups)-1].body) < 80 {
			prev := &groups[len(groups)-1]
			if prev.body == "" {
				prev.body = body
			} else {
				prev.body += "\n\n" + body
			}
			// Keep the first non-empty heading.
			if prev.heading == "" && s.Heading != "" {
				prev.heading = s.Heading
			}
		} else {
			groups = append(groups, merged{heading: s.Heading, body: body})
		}
	}

	// Split large sections (>500 words) at paragraph boundaries.
	var chunks []Chunk
	for _, g := range groups {
		header := title
		if g.heading != "" {
			header = title + " > " + g.heading
		}

		if wordCount(g.body) <= 500 {
			text := docPrefix + header + ": " + g.body
			chunks = append(chunks, Chunk{Header: header, Text: text})
			continue
		}

		// Split on double-newline paragraph boundaries.
		paragraphs := strings.Split(g.body, "\n\n")
		var buf strings.Builder
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if buf.Len() > 0 && wordCount(buf.String()+"\n\n"+p) > 500 {
				// Flush current buffer as a chunk.
				text := docPrefix + header + ": " + buf.String()
				chunks = append(chunks, Chunk{Header: header, Text: text})
				buf.Reset()
			}
			if buf.Len() > 0 {
				buf.WriteString("\n\n")
			}
			buf.WriteString(p)
		}
		if buf.Len() > 0 {
			text := docPrefix + header + ": " + buf.String()
			chunks = append(chunks, Chunk{Header: header, Text: text})
		}
	}

	if len(chunks) == 0 {
		return nil
	}
	return chunks
}

// wordCount returns the number of whitespace-delimited words in s.
func wordCount(s string) int {
	return len(strings.Fields(s))
}

// embedSource embeds all unembedded articles for a single source, using a
// pipeline: the next DB batch is pre-fetched while the current one is being
// embedded by llama-server.
func embedSource(
	ctx context.Context,
	db *searchdb.DB,
	server *embedder.Server,
	src searchdb.SourceInfo,
	docPrefix string,
	dims int,
	notify func(SemanticProgress),
) error {
	embedded, _ := db.EmbeddingCountBySource(src.ID)
	if embedded >= src.ArticleCount {
		notify(SemanticProgress{
			File: src.Filename, Status: "skip",
			Detail:  "already embedded",
			Current: src.ArticleCount, Total: src.ArticleCount,
		})
		return nil
	}

	notify(SemanticProgress{
		File: src.Filename, Status: "embedding",
		Detail:  fmt.Sprintf("%d / %d", embedded, src.ArticleCount),
		Current: embedded, Total: src.ArticleCount,
	})

	// Pipeline: pre-fetch the next batch while embedding the current one.
	type fetchResult struct {
		articles []searchdb.UnembeddedArticle
		err      error
	}

	var afterID int64
	prefetch := func(after int64) <-chan fetchResult {
		ch := make(chan fetchResult, 1)
		go func() {
			arts, err := db.UnembeddedArticlesBySource(src.ID, after, embeddingBatchSize)
			ch <- fetchResult{arts, err}
		}()
		return ch
	}

	// Kick off the first fetch.
	nextCh := prefetch(afterID)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Wait for the pre-fetched batch.
		result := <-nextCh
		if result.err != nil {
			return fmt.Errorf("fetching unembedded articles for %s: %w", src.Filename, result.err)
		}
		batch := result.articles
		if len(batch) == 0 {
			break
		}

		// Start pre-fetching the next batch while we embed this one.
		afterID = batch[len(batch)-1].ID
		nextCh = prefetch(afterID)

		// Build chunks for each article in the batch.
		type articleChunks struct {
			articleID int64
			chunks    []Chunk
		}
		var allArticleChunks []articleChunks
		var allTexts []string

		for _, a := range batch {
			chunks := buildChunks(docPrefix, a.Title, a.Sections)
			if len(chunks) == 0 {
				// Fallback: single chunk from title+body (old behavior).
				chunks = []Chunk{{
					Header: a.Title,
					Text:   prepareEmbeddingText(docPrefix, a.Title, a.Body),
				}}
			}
			allArticleChunks = append(allArticleChunks, articleChunks{
				articleID: a.ID,
				chunks:    chunks,
			})
			for _, c := range chunks {
				allTexts = append(allTexts, c.Text)
			}
		}

		vectors, err := server.EmbedBatch(ctx, allTexts)
		if err != nil {
			notify(SemanticProgress{
				File: src.Filename, Status: "failed",
				Detail: err.Error(), Current: embedded, Total: src.ArticleCount,
			})
			return fmt.Errorf("embedding %s: %w", src.Filename, err)
		}

		// Truncate dims if configured.
		if dims > 0 {
			for i := range vectors {
				vectors[i] = embedder.TruncateDims(vectors[i], dims)
			}
		}

		// Map vectors back to chunk embeddings.
		var chunkEmbeddings []searchdb.ChunkEmbedding
		vecIdx := 0
		for _, ac := range allArticleChunks {
			for ci, c := range ac.chunks {
				chunkEmbeddings = append(chunkEmbeddings, searchdb.ChunkEmbedding{
					ArticleID:  ac.articleID,
					ChunkIndex: ci,
					Header:     c.Header,
					Vector:     embedder.VectorToBlob(vectors[vecIdx]),
				})
				vecIdx++
			}
		}

		if err := db.InsertChunkEmbeddings(chunkEmbeddings); err != nil {
			return fmt.Errorf("storing embeddings for %s: %w", src.Filename, err)
		}

		embedded += int64(len(batch))

		pct := int(100 * embedded / src.ArticleCount)
		notify(SemanticProgress{
			File: src.Filename, Status: "embedding",
			Detail:  fmt.Sprintf("%d%% (%d / %d)", pct, embedded, src.ArticleCount),
			Current: embedded, Total: src.ArticleCount,
		})
	}

	notify(SemanticProgress{
		File: src.Filename, Status: "done",
		Detail:  fmt.Sprintf("%d articles", src.ArticleCount),
		Current: src.ArticleCount, Total: src.ArticleCount,
	})
	return nil
}

func prepareEmbeddingText(prefix, title, body string) string {
	title = strings.TrimSpace(title)
	body = strings.Join(strings.Fields(body), " ")

	base := prefix + title
	if body == "" {
		return truncateRunes(base, embeddingTextMaxRunes)
	}

	body, wordTruncated := truncateWords(body, embeddingTextMaxWords)

	full := base + " " + body
	if len([]rune(full)) <= embeddingTextMaxRunes && !wordTruncated {
		return full
	}

	if len([]rune(base)) >= embeddingTextMaxRunes-3 {
		return truncateRunes(base, embeddingTextMaxRunes-3) + "..."
	}

	remaining := embeddingTextMaxRunes - len([]rune(base)) - 1
	if remaining <= 3 {
		return truncateRunes(base, embeddingTextMaxRunes-3) + "..."
	}

	suffix := ""
	if wordTruncated || len([]rune(full)) > embeddingTextMaxRunes {
		suffix = "..."
	}
	if suffix == "" {
		return full
	}

	return base + " " + truncateRunes(body, remaining-len([]rune(suffix))) + suffix
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

func truncateWords(s string, limit int) (string, bool) {
	if limit <= 0 {
		return "", len(strings.Fields(s)) > 0
	}
	words := strings.Fields(s)
	if len(words) <= limit {
		return s, false
	}
	return strings.Join(words[:limit], " "), true
}
