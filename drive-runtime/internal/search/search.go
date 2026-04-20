package search

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"

	drivebinary "github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
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
	if opener == nil {
		opener = browser.Open
	}
	dbPath := filepath.Join(driveRoot, "data", "search.db")
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("search index not found. run 'svalbard index' to build it")
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return fmt.Errorf("opening search.db: %w", err)
	}
	defer db.Close()

	eng := engine.New(db)

	caps, articleCount, sourceCount, err := detectCapabilities(driveRoot, db)
	if err != nil {
		return err
	}
	mode := BestMode(caps)
	bestMode := mode

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Cross-ZIM Search (%d sources, %d articles, %s)\n", sourceCount, articleCount, mode)
	fmt.Fprintln(stdout, "────────────────────────────────")

	reader := bufio.NewReader(stdin)
	query := initialQuery
	var embedServer *exec.Cmd
	var kiwixServer *exec.Cmd
	embedPort := 8085
	kiwixPort := 8080
	defer func() {
		if embedServer != nil && embedServer.Process != nil {
			_ = embedServer.Process.Kill()
		}
		if kiwixServer != nil && kiwixServer.Process != nil {
			_ = kiwixServer.Process.Kill()
		}
	}()

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
		effectiveMode := mode
		var results []Result
		if effectiveMode == ModeHybrid {
			if embedServer == nil {
				if path, err := drivebinary.Resolve("llama-server", driveRoot, platform.Detect); err == nil && caps.EmbeddingModel != "" {
					embedPort, _ = netutil.FindAvailablePort("127.0.0.1", 8085)
					embedServer, err = startEmbeddingServer(ctx, path, caps.EmbeddingModel, embedPort)
					if err != nil {
						embedServer = nil
						effectiveMode = ModeKeyword
						fmt.Fprintln(stdout, "  Embed server unavailable, falling back to keyword")
					}
				} else {
					effectiveMode = ModeKeyword
				}
			}
			if effectiveMode == ModeHybrid {
				queryVec, embedErr := engine.EmbedQuery(caps.QueryPrefix+query, embedPort)
				if embedErr == nil {
					results, err = eng.Hybrid(query, queryVec, 20)
				}
				if embedErr != nil || err != nil || len(results) == 0 {
					effectiveMode = ModeKeyword
				}
			}
		}
		if effectiveMode == ModeKeyword {
			results, err = eng.Keyword(query, 20)
			if err != nil {
				return err
			}
		}
		if len(results) == 0 {
			fmt.Fprintf(stdout, "  No results for: %s\n", query)
			query = ""
			continue
		}

		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "────────────────────────────────")
		RenderResults(stdout, results)
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
		if idx, err := strconv.Atoi(choice); err == nil && idx >= 1 && idx <= len(results) {
			if kiwixServer == nil {
				kiwixPort, _ = netutil.FindAvailablePort("127.0.0.1", 8080)
				kiwixServer, err = startKiwix(ctx, driveRoot, kiwixPort)
				if err != nil {
					fmt.Fprintf(stdout, "  kiwix-serve not available. Article: %s / %s\n", strings.TrimSuffix(results[idx-1].Filename, ".zim"), results[idx-1].Path)
					query = ""
					continue
				}
			}
			book := strings.TrimSuffix(results[idx-1].Filename, ".zim")
			url := fmt.Sprintf("http://localhost:%d/content/%s/%s", kiwixPort, book, results[idx-1].Path)
			fmt.Fprintf(stdout, "  Opening: %s\n", url)
			_ = opener(url)
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
	caps.EmbeddingModel = findEmbeddingModel(driveRoot)
	return caps, articleCount, sourceCount, nil
}

func findEmbeddingModel(driveRoot string) string {
	matches, _ := filepath.Glob(filepath.Join(driveRoot, "models", "embed", "*.gguf"))
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), "._") {
			return m
		}
	}
	return ""
}

func startEmbeddingServer(ctx context.Context, llamaBin, model string, port int) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, llamaBin, "--model", model, "--port", fmt.Sprintf("%d", port), "--host", "127.0.0.1", "--embedding")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return cmd, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return nil, fmt.Errorf("llama-server failed to become healthy")
}

func startKiwix(ctx context.Context, driveRoot string, port int) (*exec.Cmd, error) {
	kiwixBin, err := drivebinary.Resolve("kiwix-serve", driveRoot, platform.Detect)
	if err != nil {
		return nil, err
	}
	zims, err := filepath.Glob(filepath.Join(driveRoot, "zim", "*.zim"))
	if err != nil || len(zims) == 0 {
		return nil, fmt.Errorf("no ZIM files found")
	}
	args := []string{"--port", fmt.Sprintf("%d", port), "--address", "127.0.0.1"}
	args = append(args, zims...)
	cmd := exec.CommandContext(ctx, kiwixBin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/", port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return cmd, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Kill the process to prevent orphan on timeout.
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return nil, fmt.Errorf("kiwix-serve did not become healthy")
}
