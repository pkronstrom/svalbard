package runtimesearch

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebinary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebrowser"
)

type Mode string

const (
	ModeKeyword  Mode = "keyword"
	ModeSemantic Mode = "semantic"
)

type Capabilities struct {
	HasEmbeddings    bool
	HasEmbeddingData bool
	HasLlamaServer   bool
	EmbeddingModel   string
}

type Result struct {
	ID       int
	Filename string
	Path     string
	Title    string
	Snippet  string
}

func BuildFTSQuery(query string) string {
	parts := strings.Fields(query)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(part, `"`, `""`)
		part = strings.ReplaceAll(part, `'`, `''`)
		out = append(out, fmt.Sprintf(`"%s"*`, part))
	}
	return strings.Join(out, " ")
}

func DecodeVectorHex(value string) ([]float32, error) {
	raw, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if len(raw)%4 != 0 {
		return nil, fmt.Errorf("invalid vector length")
	}
	out := make([]float32, 0, len(raw)/4)
	for i := 0; i < len(raw); i += 4 {
		out = append(out, math.Float32frombits(binary.LittleEndian.Uint32(raw[i:i+4])))
	}
	return out, nil
}

func BestMode(c Capabilities) Mode {
	if c.HasEmbeddings && c.HasEmbeddingData && c.HasLlamaServer && c.EmbeddingModel != "" {
		return ModeSemantic
	}
	return ModeKeyword
}

func RenderResults(w io.Writer, results []Result) {
	for i, result := range results {
		label := strings.TrimSuffix(result.Filename, ".zim")
		fmt.Fprintf(w, "  %d. [%s] %s\n", i+1, label, result.Title)
		if result.Snippet != "" {
			fmt.Fprintf(w, "     %s\n", result.Snippet)
		}
	}
}

func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, driveRoot, initialQuery string, opener func(string) error) error {
	if opener == nil {
		opener = runtimebrowser.Open
	}
	dbPath := filepath.Join(driveRoot, "data", "search.db")
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("search index not found. run 'svalbard index' to build it")
	}
	sqliteBin, err := runtimebinary.Resolve("sqlite3", driveRoot, platform.Detect)
	if err != nil {
		return fmt.Errorf("sqlite3 not found")
	}
	caps, articleCount, sourceCount, err := detectCapabilities(driveRoot, dbPath, sqliteBin)
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
			fmt.Fprintf(stdout, "\n  [%s] Search (/fts /sem q): ", mode)
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
		case "/sem", "/semantic":
			mode = bestMode
			fmt.Fprintln(stdout, "  Switched to semantic search")
			query = ""
			continue
		}

		fmt.Fprintf(stdout, "Searching (%s): %s\n", mode, query)
		effectiveMode := mode
		var results []Result
		if effectiveMode == ModeSemantic {
			if embedServer == nil {
				if path, err := runtimebinary.Resolve("llama-server", driveRoot, platform.Detect); err == nil && caps.EmbeddingModel != "" {
					embedPort, _ = findAvailablePort("127.0.0.1", 8085)
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
			if effectiveMode == ModeSemantic {
				results, err = semanticSearch(sqliteBin, dbPath, query, articleCount, embedPort)
				if err != nil || len(results) == 0 {
					effectiveMode = ModeKeyword
				}
			}
		}
		if effectiveMode == ModeKeyword {
			results, err = keywordSearch(sqliteBin, dbPath, query)
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
				kiwixPort, _ = findAvailablePort("127.0.0.1", 8080)
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

func detectCapabilities(driveRoot, dbPath, sqliteBin string) (Capabilities, int, int, error) {
	var caps Capabilities
	sourceCount, err := scalarInt(sqliteBin, dbPath, "SELECT count(*) FROM sources;")
	if err != nil {
		return caps, 0, 0, err
	}
	articleCount, err := scalarInt(sqliteBin, dbPath, "SELECT count(*) FROM articles;")
	if err != nil {
		return caps, 0, 0, err
	}
	hasEmbeddings, err := scalarInt(sqliteBin, dbPath, "SELECT count(*) FROM sqlite_master WHERE name='embeddings';")
	if err == nil && hasEmbeddings == 1 {
		caps.HasEmbeddings = true
		embedCount, _ := scalarInt(sqliteBin, dbPath, "SELECT count(*) FROM embeddings;")
		caps.HasEmbeddingData = embedCount > 0
	}
	if _, err := runtimebinary.Resolve("llama-server", driveRoot, platform.Detect); err == nil {
		caps.HasLlamaServer = true
	}
	caps.EmbeddingModel = findEmbeddingModel(driveRoot)
	return caps, articleCount, sourceCount, nil
}

func findEmbeddingModel(driveRoot string) string {
	patterns := []string{
		filepath.Join(driveRoot, "models", "*nomic*embed*"),
		filepath.Join(driveRoot, "models", "*embed*"),
		filepath.Join(driveRoot, "models", "*bge*"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		sort.Strings(matches)
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && !info.IsDir() {
				return match
			}
		}
	}
	return ""
}

func scalarInt(sqliteBin, dbPath, sql string) (int, error) {
	out, err := exec.Command(sqliteBin, dbPath, sql).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func keywordSearch(sqliteBin, dbPath, query string) ([]Result, error) {
	ftsQuery := BuildFTSQuery(query)
	sql := fmt.Sprintf(
		"SELECT a.id, s.filename, a.path, a.title, snippet(articles_fts, 1, '»', '«', '...', 12) "+
			"FROM articles_fts JOIN articles a ON a.id = articles_fts.rowid "+
			"JOIN sources s ON s.id = a.source_id WHERE articles_fts MATCH '%s' ORDER BY rank LIMIT 20;",
		ftsQuery,
	)
	return queryResults(sqliteBin, dbPath, sql)
}

func semanticSearch(sqliteBin, dbPath, query string, articleCount, embedPort int) ([]Result, error) {
	var candidateIDs []int
	if articleCount >= 500000 {
		ftsQuery := BuildFTSQuery(query)
		sql := fmt.Sprintf(
			"SELECT a.id FROM articles_fts JOIN articles a ON a.id = articles_fts.rowid "+
				"WHERE articles_fts MATCH '%s' ORDER BY rank LIMIT 200;", ftsQuery,
		)
		out, err := exec.Command(sqliteBin, dbPath, sql).Output()
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Fields(string(out)) {
			id, err := strconv.Atoi(strings.TrimSpace(line))
			if err == nil {
				candidateIDs = append(candidateIDs, id)
			}
		}
	} else {
		out, err := exec.Command(sqliteBin, dbPath, "SELECT id FROM articles;").Output()
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Fields(string(out)) {
			id, err := strconv.Atoi(strings.TrimSpace(line))
			if err == nil {
				candidateIDs = append(candidateIDs, id)
			}
		}
	}
	if len(candidateIDs) == 0 {
		return nil, nil
	}

	queryVector, err := embedQuery(query, embedPort)
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf("SELECT article_id, hex(vector) FROM embeddings WHERE article_id IN (%s);", intsToCSV(candidateIDs))
	out, err := exec.Command(sqliteBin, "-separator", "\t", dbPath, sql).Output()
	if err != nil {
		return nil, err
	}
	type scored struct {
		id    int
		score float64
	}
	var scores []scored
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		vec, err := DecodeVectorHex(parts[1])
		if err != nil {
			continue
		}
		scores = append(scores, scored{id: id, score: dotProduct(queryVector, vec)})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	if len(scores) > 20 {
		scores = scores[:20]
	}
	if len(scores) == 0 {
		return nil, nil
	}

	valueRows := make([]string, 0, len(scores))
	for i, score := range scores {
		valueRows = append(valueRows, fmt.Sprintf("(%d,%d)", score.id, i))
	}
	detailSQL := fmt.Sprintf(
		"WITH ranked(aid, pos) AS (VALUES %s) "+
			"SELECT a.id, s.filename, a.path, a.title, substr(a.body, 1, 120) "+
			"FROM ranked r JOIN articles a ON a.id = r.aid JOIN sources s ON s.id = a.source_id ORDER BY r.pos;",
		strings.Join(valueRows, ","),
	)
	return queryResults(sqliteBin, dbPath, detailSQL)
}

func queryResults(sqliteBin, dbPath, sql string) ([]Result, error) {
	out, err := exec.Command(sqliteBin, "-separator", "\t", dbPath, sql).Output()
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 4 {
			continue
		}
		id, _ := strconv.Atoi(parts[0])
		snippet := ""
		if len(parts) == 5 {
			snippet = parts[4]
		}
		results = append(results, Result{
			ID:       id,
			Filename: parts[1],
			Path:     parts[2],
			Title:    parts[3],
			Snippet:  snippet,
		})
	}
	return results, nil
}

func embedQuery(query string, port int) ([]float32, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/embedding", port)
	payload := map[string][]string{"content": {"search_query: " + query}}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data []struct {
		Embedding json.RawMessage `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	var nested [][]float32
	if err := json.Unmarshal(data[0].Embedding, &nested); err == nil && len(nested) > 0 {
		return nested[0], nil
	}
	var vec []float32
	if err := json.Unmarshal(data[0].Embedding, &vec); err != nil {
		return nil, err
	}
	return vec, nil
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
	kiwixBin, err := runtimebinary.Resolve("kiwix-serve", driveRoot, platform.Detect)
	if err != nil {
		return nil, err
	}
	zims, err := filepath.Glob(filepath.Join(driveRoot, "zim", "*.zim"))
	if err != nil || len(zims) == 0 {
		return nil, fmt.Errorf("no ZIM files found")
	}
	args := []string{"--port", fmt.Sprintf("%d", port)}
	args = append(args, zims...)
	cmd := exec.CommandContext(ctx, kiwixBin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	time.Sleep(2 * time.Second)
	return cmd, nil
}

func intsToCSV(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func dotProduct(a []float32, b []float32) float64 {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	var sum float64
	for i := 0; i < limit; i++ {
		sum += float64(a[i] * b[i])
	}
	return sum
}

func findAvailablePort(host string, preferred int) (int, error) {
	for port := preferred; port < preferred+20; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
