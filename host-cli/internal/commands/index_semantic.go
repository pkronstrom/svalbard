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

// SemanticProgress reports embedding progress.
type SemanticProgress struct {
	Phase    string // "starting", "embedding", "done"
	Embedded int64
	Total    int64
}

// IndexSemantic generates embeddings for all unembedded articles in the search
// database. It starts a llama-server subprocess, batches articles through
// the embedding API, and stores the resulting vectors.
//
// The keyword index must already exist (run IndexVault first).
func IndexSemantic(ctx context.Context, root string, w io.Writer, onProgress func(SemanticProgress)) error {
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

	// Check how many articles need embedding.
	totalArticles, err := db.ArticleCount()
	if err != nil {
		return fmt.Errorf("counting articles: %w", err)
	}
	alreadyEmbedded, err := db.EmbeddingCount()
	if err != nil {
		return fmt.Errorf("counting embeddings: %w", err)
	}
	remaining := totalArticles - alreadyEmbedded
	if remaining <= 0 {
		fmt.Fprintf(w, "All %d articles already embedded\n", totalArticles)
		if onProgress != nil {
			onProgress(SemanticProgress{Phase: "done", Embedded: alreadyEmbedded, Total: totalArticles})
		}
		return nil
	}

	// Find the embedding model.
	modelPath, err := embedder.FindEmbeddingModel(root)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Starting embedding server (%s)...\n", filepath.Base(modelPath))
	if onProgress != nil {
		onProgress(SemanticProgress{Phase: "starting", Total: totalArticles})
	}

	server, err := embedder.StartServer(ctx, modelPath, root)
	if err != nil {
		return fmt.Errorf("starting embedding server: %w", err)
	}
	defer server.Stop()

	fmt.Fprintf(w, "Embedding %d articles (batch size %d)...\n", remaining, embeddingBatchSize)

	embedded := alreadyEmbedded
	var afterID int64

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		batch, err := db.UnembeddedArticles(afterID, embeddingBatchSize)
		if err != nil {
			return fmt.Errorf("fetching unembedded articles: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		// Build text inputs: "title body"
		texts := make([]string, len(batch))
		for i, a := range batch {
			texts[i] = a.Title + " " + a.Body
		}

		vectors, err := server.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("embedding batch at article %d: %w", batch[0].ID, err)
		}

		pairs := make([]searchdb.EmbeddingPair, len(batch))
		for i, a := range batch {
			pairs[i] = searchdb.EmbeddingPair{
				ArticleID: a.ID,
				Vector:    embedder.VectorToBlob(vectors[i]),
			}
		}

		if err := db.InsertEmbeddings(pairs); err != nil {
			return fmt.Errorf("storing embeddings: %w", err)
		}

		embedded += int64(len(batch))
		afterID = batch[len(batch)-1].ID

		if onProgress != nil {
			onProgress(SemanticProgress{Phase: "embedding", Embedded: embedded, Total: totalArticles})
		}
	}

	if err := db.SetMeta("semantic_indexed_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting semantic_indexed_at: %w", err)
	}

	fmt.Fprintf(w, "Semantic indexing complete: %d embeddings\n", embedded)
	if onProgress != nil {
		onProgress(SemanticProgress{Phase: "done", Embedded: embedded, Total: totalArticles})
	}
	return nil
}
