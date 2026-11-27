package search

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"

	drivebinary "github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

type Mode string

const (
	ModeKeyword Mode = "keyword"
	ModeHybrid  Mode = "hybrid"
)

type Capabilities struct {
	HasEmbeddings    bool
	HasEmbeddingData bool
	HasLlamaServer   bool
	EmbeddingModel   string // file path from models/embed/
	EmbeddingModelID string // model ID from search.db meta
	QueryPrefix      string // prefix for query embedding (from recipe sidecar via meta)
	SemanticComplete bool   // only completed compatibility contracts may be queried
}

// Result is an alias for engine.Result so external consumers (menu, mcp) share
// a single result type across the codebase.
type Result = engine.Result

var resolveRuntimeBinary = drivebinary.Resolve

// BuildFTSQuery delegates to the engine package.
func BuildFTSQuery(query string) string {
	return engine.BuildFTSQuery(query)
}

func BestMode(c Capabilities) Mode {
	if c.SemanticComplete && c.HasEmbeddings && c.HasEmbeddingData && c.HasLlamaServer && c.EmbeddingModel != "" {
		return ModeHybrid
	}
	return ModeKeyword
}

func RenderResults(w io.Writer, results []Result) {
	for i, result := range results {
		label := SourceLabel(result.Filename)
		title := result.Title
		if result.ChunkHeader != "" && result.ChunkHeader != result.Title {
			title = result.ChunkHeader
		}
		fmt.Fprintf(w, "  %d. [%s] %s\n", i+1, label, title)
		if result.Snippet != "" {
			fmt.Fprintf(w, "     %s\n", result.Snippet)
		}
	}
}

func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, driveRoot, initialQuery string, opener func(string) error) error {
	session, err := newSession(ctx, driveRoot, opener)
	if err != nil {
		return err
	}
	defer session.Close()

	return runSession(ctx, stdin, stdout, initialQuery, session)
}

func runSession(ctx context.Context, stdin io.Reader, stdout io.Writer, initialQuery string, session *Session) error {
	info := session.Info()
	mode := info.BestMode

	fmt.Fprintln(stdout)
	if info.NativeZIMCount > 0 {
		fmt.Fprintf(stdout, "Search (%d sources, %d embedded ZIM indexes, %d SQLite articles or pages, %s)\n", info.SourceCount, info.NativeZIMCount, info.ArticleCount, mode)
	} else {
		fmt.Fprintf(stdout, "Search (%d sources, %d articles, %s)\n", info.SourceCount, info.ArticleCount, mode)
	}
	fmt.Fprintln(stdout, "────────────────────────────────")

	reader := bufio.NewReader(stdin)
	query := initialQuery
	for {
		if query == "" {
			fmt.Fprintf(stdout, "\n  [%s] Search (/fts /hybrid q): ", mode)
			line, err := reader.ReadString('\n')
			if err != nil && line == "" {
				return err
			}
			query = strings.TrimSpace(line)
		}
		if query == "" || strings.EqualFold(query, "q") {
			return nil
		}
		switch query {
		case "/fts", "/keyword":
			mode = ModeKeyword
			fmt.Fprintln(stdout, "  Switched to keyword search")
			query = ""
			continue
		case "/sem", "/semantic", "/hybrid", "/full":
			// A long-lived session may have opened while semantic indexing was
			// incomplete.  Ask Search to refresh its completed contract instead
			// of pinning this command to the startup capability snapshot.
			mode = ModeHybrid
			fmt.Fprintf(stdout, "  Switched to %s search\n", mode)
			query = ""
			continue
		}

		fmt.Fprintf(stdout, "Searching (%s): %s\n", mode, query)
		response, err := session.Search(ctx, mode, query, 20)
		if err != nil {
			return err
		}
		if response.Status != "" {
			fmt.Fprintf(stdout, "  %s\n", response.Status)
		}
		if len(response.Results) == 0 {
			fmt.Fprintf(stdout, "  No results for: %s\n", query)
			query = ""
			continue
		}

		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "────────────────────────────────")
		RenderResults(stdout, response.Results)
		fmt.Fprintln(stdout)
		fmt.Fprint(stdout, "  Open # (or new search, q to quit): ")
		choice, err := reader.ReadString('\n')
		if err != nil && choice == "" {
			return err
		}
		choice = strings.TrimSpace(choice)
		if choice == "" {
			query = ""
			continue
		}
		if strings.EqualFold(choice, "q") {
			return nil
		}
		if idx, err := strconv.Atoi(choice); err == nil && idx >= 1 && idx <= len(response.Results) {
			if chosen := response.Results[idx-1]; session.OpenResult(chosen) != nil {
				if IsDocumentSource(chosen.Filename) {
					fmt.Fprintf(stdout, "  Could not open the document. It is on the drive at %s, %s\n", chosen.Filename, chosen.Path)
				} else {
					fmt.Fprintf(stdout, "  kiwix-serve not available. Article: %s / %s\n", SourceLabel(chosen.Filename), chosen.Path)
				}
				query = ""
				continue
			}
			fmt.Fprintln(stdout, "  Opening result")
			query = ""
			continue
		}
		query = choice
	}
}

func detectCapabilities(ctx context.Context, driveRoot string, db *sql.DB) (Capabilities, int, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Capabilities{}, 0, 0, err
	}
	var caps Capabilities
	// Skills and library quickstarts are in the index too, but this search
	// does not cover them.
	sourceCount, articleCount := engine.New(db).KnowledgeCounts()

	var hasEmbeddings int
	_ = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='embeddings'").Scan(&hasEmbeddings)
	if hasEmbeddings > 0 {
		caps.HasEmbeddings = true
		var embedCount int
		_ = db.QueryRow("SELECT count(*) FROM embeddings").Scan(&embedCount)
		caps.HasEmbeddingData = embedCount > 0
	}

	var modelID sql.NullString
	_ = db.QueryRow("SELECT value FROM meta WHERE key='embedding_model'").Scan(&modelID)
	if modelID.Valid {
		caps.EmbeddingModelID = modelID.String
	}
	contract, complete := LoadCompleteContract(db)
	caps.SemanticComplete = complete

	if complete {
		caps.QueryPrefix = contract.QueryPrefix
	}

	if complete {
		if model, err := resolveRecordedModel(ctx, driveRoot, contract); err == nil {
			caps.EmbeddingModel = model
		} else if ctx.Err() != nil {
			return Capabilities{}, 0, 0, ctx.Err()
		} else {
			caps.SemanticComplete = false
		}
	}
	if err := ctx.Err(); err != nil {
		return Capabilities{}, 0, 0, err
	}
	if _, err := resolveRuntimeBinary("llama-server", driveRoot); err == nil {
		caps.HasLlamaServer = true
	}
	return caps, articleCount, sourceCount, nil
}
