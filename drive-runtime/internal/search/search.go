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
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/server"
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
}

// Result is an alias for engine.Result so external consumers (menu, mcp) share
// a single result type across the codebase.
type Result = engine.Result

// BuildFTSQuery delegates to the engine package.
func BuildFTSQuery(query string) string {
	return engine.BuildFTSQuery(query)
}

func BestMode(c Capabilities) Mode {
	if c.HasEmbeddings && c.HasEmbeddingData && c.HasLlamaServer && c.EmbeddingModel != "" {
		return ModeHybrid
	}
	return ModeKeyword
}

func RenderResults(w io.Writer, results []Result) {
	for i, result := range results {
		label := strings.TrimSuffix(result.Filename, ".zim")
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
	session, err := NewSession(driveRoot, opener)
	if err != nil {
		return err
	}
	defer session.Close()

	return runSession(ctx, stdin, stdout, initialQuery, session)
}

func runSession(ctx context.Context, stdin io.Reader, stdout io.Writer, initialQuery string, session *Session) error {
	info := session.Info()
	mode := info.BestMode
	bestMode := mode

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Cross-ZIM Search (%d sources, %d articles, %s)\n", info.SourceCount, info.ArticleCount, mode)
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
			mode = bestMode
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
			if err := session.OpenResult(response.Results[idx-1]); err != nil {
				fmt.Fprintf(stdout, "  kiwix-serve not available. Article: %s / %s\n", strings.TrimSuffix(response.Results[idx-1].Filename, ".zim"), response.Results[idx-1].Path)
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

func detectCapabilities(driveRoot string, db *sql.DB) (Capabilities, int, int, error) {
	var caps Capabilities
	var sourceCount, articleCount int

	_ = db.QueryRow("SELECT count(*) FROM sources").Scan(&sourceCount)
	_ = db.QueryRow("SELECT count(*) FROM articles").Scan(&articleCount)

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

	var queryPrefix sql.NullString
	_ = db.QueryRow("SELECT value FROM meta WHERE key='embedding_query_prefix'").Scan(&queryPrefix)
	if queryPrefix.Valid {
		caps.QueryPrefix = queryPrefix.String
	}

	if _, err := drivebinary.Resolve("llama-server", driveRoot, platform.Detect); err == nil {
		caps.HasLlamaServer = true
	}
	caps.EmbeddingModel = server.FindEmbeddingModel(driveRoot)
	return caps, articleCount, sourceCount, nil
}
