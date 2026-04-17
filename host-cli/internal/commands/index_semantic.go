package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/embedder"
	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

const embeddingBatchSize = 32

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

	// Check if any work is needed.
	anyWork := false
	for _, src := range sources {
		embedded, _ := db.EmbeddingCountBySource(src.ID)
		if embedded < src.ArticleCount {
			anyWork = true
			break
		}
	}
	if !anyWork {
		for _, src := range sources {
			notify(SemanticProgress{
				File: src.Filename, Status: "skip",
				Detail:  "already embedded",
				Current: src.ArticleCount, Total: src.ArticleCount,
			})
		}
		return nil
	}

	// Find the embedding model.
	modelPath, err := embedder.FindEmbeddingModel(root)
	if err != nil {
		return err
	}

	notify(SemanticProgress{Status: "starting", Detail: "Starting embedding server..."})

	server, err := embedder.StartServer(ctx, modelPath, root)
	if err != nil {
		return fmt.Errorf("starting embedding server: %w", err)
	}
	defer server.Stop()

	// Embed each source separately for per-file progress.
	for _, src := range sources {
		if err := embedSource(ctx, db, server, src, notify); err != nil {
			return err
		}
	}

	if err := db.SetMeta("semantic_indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting semantic_indexed_at: %w", err)
	}

	return nil
}

// embedSource embeds all unembedded articles for a single source, using a
// pipeline: the next DB batch is pre-fetched while the current one is being
// embedded by llama-server.
func embedSource(
	ctx context.Context,
	db *searchdb.DB,
	server *embedder.Server,
	src searchdb.SourceInfo,
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

		texts := make([]string, len(batch))
		for i, a := range batch {
			texts[i] = "search_document: " + a.Title + " " + a.Body
		}

		vectors, err := server.EmbedBatch(ctx, texts)
		if err != nil {
			notify(SemanticProgress{
				File: src.Filename, Status: "failed",
				Detail: err.Error(), Current: embedded, Total: src.ArticleCount,
			})
			return fmt.Errorf("embedding %s: %w", src.Filename, err)
		}

		pairs := make([]searchdb.EmbeddingPair, len(batch))
		for i, a := range batch {
			pairs[i] = searchdb.EmbeddingPair{
				ArticleID: a.ID,
				Vector:    embedder.VectorToBlob(vectors[i]),
			}
		}

		if err := db.InsertEmbeddings(pairs); err != nil {
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
