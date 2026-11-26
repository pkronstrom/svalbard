package mcp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func TestMain(m *testing.M) {
	if os.Getenv("SVALBARD_TEST_MCP_LLAMA") == "1" {
		for i, arg := range os.Args {
			if arg == "--port" && i+1 < len(os.Args) {
				if err := http.ListenAndServe("127.0.0.1:"+os.Args[i+1], http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/embedding" {
						var request struct {
							Content []string `json:"content"`
						}
						if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Content) != 1 {
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						if log := os.Getenv("SVALBARD_TEST_MCP_LLAMA_LOG"); log != "" {
							if err := os.WriteFile(log, []byte(request.Content[0]), 0o644); err != nil {
								w.WriteHeader(http.StatusInternalServerError)
								return
							}
						}
						vector := make([]float32, 768)
						for i := range vector[:256] {
							vector[i] = 1
						}
						_ = json.NewEncoder(w).Encode([]map[string][]float32{{"embedding": vector}})
						return
					}
					w.WriteHeader(http.StatusOK)
				})); err != nil {
					os.Exit(1)
				}
			}
		}
		os.Exit(1)
	}
	if os.Getenv("SVALBARD_TEST_MCP_KIWIX") != "1" {
		kiwixBasePort = 18480
		os.Exit(m.Run())
	}
	for i, arg := range os.Args {
		if arg == "--port" && i+1 < len(os.Args) {
			if err := http.ListenAndServe("127.0.0.1:"+os.Args[i+1], http.HandlerFunc(fakeKiwix)); err != nil {
				os.Exit(1)
			}
		}
	}
	os.Exit(1)
}

// fakeKiwix stands in for kiwix-serve. With SVALBARD_TEST_MCP_KIWIX_PAGES it
// serves /content/<book>/<path> from the file <dir>/<book>/<escaped path>,
// answers 404 for anything else, and redirects a book's root to the page
// named in <dir>/<book>/.main, as kiwix-serve does. Without it every request
// succeeds with an empty page.
func fakeKiwix(w http.ResponseWriter, r *http.Request) {
	pages := os.Getenv("SVALBARD_TEST_MCP_KIWIX_PAGES")
	rest, isContent := strings.CutPrefix(r.URL.Path, "/content/")
	if pages == "" || !isContent {
		w.WriteHeader(http.StatusOK)
		return
	}
	book, path, _ := strings.Cut(rest, "/")
	if path == "" {
		if main, err := os.ReadFile(filepath.Join(pages, book, ".main")); err == nil {
			http.Redirect(w, r, "/content/"+book+"/"+string(main), http.StatusFound)
			return
		}
	}
	// Listed, not opened by name: paths are case-sensitive in an archive,
	// whatever the file system under the test does.
	entries, _ := os.ReadDir(filepath.Join(pages, book))
	for _, entry := range entries {
		if entry.Name() == url.PathEscape(path) {
			page, _ := os.ReadFile(filepath.Join(pages, book, entry.Name()))
			_, _ = w.Write(page)
			return
		}
	}
	http.Error(w, "<html><body>Not Found</body></html>", http.StatusNotFound)
}

func TestSearchFallbackMetadataSerializesJSONThenAdvisory(t *testing.T) {
	for _, tc := range []struct {
		name string
		meta []string
	}{
		{name: "legacy", meta: []string{"INSERT INTO meta VALUES ('semantic_state', 'complete')"}},
		{name: "building", meta: []string{"INSERT INTO meta VALUES ('semantic_state', 'building')"}},
		{name: "malformed", meta: []string{"INSERT INTO meta VALUES ('semantic_state', 'complete')", "INSERT INTO meta VALUES ('semantic_contract', '{')"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := searchFixtureDrive(t, tc.meta...)
			cap := NewSearchCapability(root, DriveMetadata{})
			defer cap.Close()
			srv := NewServer(cap)
			result, err := srv.makeHandler(cap, "search")(context.Background(), gomcp.CallToolRequest{Params: gomcp.CallToolParams{Arguments: map[string]any{"query": "water"}}})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Content) != 2 {
				t.Fatalf("content blocks = %d, want JSON results then fallback advisory", len(result.Content))
			}
			first, ok := result.Content[0].(gomcp.TextContent)
			if !ok || !strings.Contains(first.Text, "Water purification") {
				t.Fatalf("first content = %#v, want JSON results", result.Content[0])
			}
			second, ok := result.Content[1].(gomcp.TextContent)
			if !ok || !strings.Contains(second.Text, "fell back to keyword") {
				t.Fatalf("second content = %#v, want fallback advisory", result.Content[1])
			}
		})
	}
}

// A drive indexed for keywords only was never meant to answer with hybrid
// search, so its results carry no fallback notice.
func TestSearchWithoutSemanticIndexSerializesOnlyResults(t *testing.T) {
	root := searchFixtureDrive(t)
	cap := NewSearchCapability(root, DriveMetadata{})
	defer cap.Close()
	result, err := NewServer(cap).makeHandler(cap, "search")(context.Background(), gomcp.CallToolRequest{Params: gomcp.CallToolParams{Arguments: map[string]any{"query": "water"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content blocks = %d, want the JSON results alone: %#v", len(result.Content), result.Content)
	}
	if first, ok := result.Content[0].(gomcp.TextContent); !ok || !strings.Contains(first.Text, "Water purification") {
		t.Fatalf("content = %#v, want JSON results", result.Content[0])
	}
}

// hybridFixtureDrive is searchFixtureDrive with a completed semantic index:
// a recorded model, one vector for the wiki article, and a stand-in
// llama-server that embeds every query as that same vector and writes the
// text it was asked to embed to the returned log.
func hybridFixtureDrive(t *testing.T) (root string, db *sql.DB, queryLog string) {
	t.Helper()
	root = searchFixtureDrive(t)
	dbPath := filepath.Join(root, "data", "search.db")
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec("CREATE TABLE embeddings (article_id INTEGER NOT NULL, chunk_header TEXT, vector BLOB NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	modelBytes := []byte("recorded model")
	modelPath := filepath.Join(root, "models", "embed", "recorded.gguf")
	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modelPath, modelBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(modelBytes)
	contract := map[string]any{
		"version": 1, "model": "models/embed/recorded.gguf", "model_sha256": fmt.Sprintf("%x", hash),
		"query_prefix": "search_query: ", "dimensions": 256, "matryoshka": true,
	}
	raw, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO meta VALUES ('semantic_state', 'complete'), ('semantic_contract', ?)", string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO embeddings VALUES (1, '', ?)", mcpFloat32Blob(256, 1.0/16)); err != nil {
		t.Fatal(err)
	}
	platformName, err := platform.Detect()
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "bin", platformName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(os.Args[0], filepath.Join(binDir, "llama-server")); err != nil {
		t.Fatal(err)
	}
	queryLog = filepath.Join(t.TempDir(), "embedded-query")
	t.Setenv("SVALBARD_TEST_MCP_LLAMA", "1")
	t.Setenv("SVALBARD_TEST_MCP_LLAMA_LOG", queryLog)
	return root, db, queryLog
}

func TestSearchCapabilityHybridUsesRecordedQueryShapeAndSerializesOnlyResults(t *testing.T) {
	root, _, queryLog := hybridFixtureDrive(t)

	cap := NewSearchCapability(root, DriveMetadata{})
	defer cap.Close()
	server := NewServer(cap)
	result, err := server.makeHandler(cap, "search")(context.Background(), gomcp.CallToolRequest{Params: gomcp.CallToolParams{Arguments: map[string]any{"query": "water"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("serialized content blocks = %d, want JSON results only", len(result.Content))
	}
	first, ok := result.Content[0].(gomcp.TextContent)
	if !ok || !strings.Contains(first.Text, "Water purification") {
		t.Fatalf("first content = %#v, want JSON result array", result.Content[0])
	}
	if strings.Contains(first.Text, "fell back to keyword") {
		t.Fatalf("hybrid result serialized fallback advisory: %q", first.Text)
	}
	gotQuery, err := os.ReadFile(queryLog)
	if err != nil {
		t.Fatalf("read observed embedding query: %v", err)
	}
	if string(gotQuery) != "search_query: water" {
		t.Fatalf("embedded query = %q", gotQuery)
	}
}

func TestSearchCapabilityFallbackFullResultReadsKeywordBody(t *testing.T) {
	root := searchFixtureDrive(t)
	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	contract := `{"version":1,"model":"models/embed/recorded.gguf","model_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","dimensions":256,"matryoshka":true}`
	if _, err := db.Exec("INSERT INTO meta VALUES ('semantic_state', 'complete'), ('semantic_contract', ?)", contract); err != nil {
		t.Fatal(err)
	}
	cap := NewSearchCapability(root, DriveMetadata{})
	defer cap.Close()
	result, err := cap.Handle(context.Background(), "search", map[string]any{"query": "water", "detail": "full"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Fatal("missing hybrid fallback advisory")
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Water is essential.") {
		t.Fatalf("keyword fallback did not read article body: %s", raw)
	}
}

func mcpFloat32Blob(dimensions int, value float32) []byte {
	buf := make([]byte, dimensions*4)
	for i := 0; i < dimensions; i++ {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(value))
	}
	return buf
}

func searchFixtureDrive(t *testing.T, metaStatements ...string) string {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(data, "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE sources (id INTEGER PRIMARY KEY, filename TEXT NOT NULL);
		CREATE TABLE articles (id INTEGER PRIMARY KEY, source_id INTEGER NOT NULL, path TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '');
		CREATE VIRTUAL TABLE articles_fts USING fts5(title, body, content='articles', content_rowid='id');
		CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO sources VALUES (1, 'wiki.zim');
		INSERT INTO articles VALUES (1, 1, '/Water', 'Water purification', 'Water is essential.');
		INSERT INTO articles_fts(rowid, title, body) VALUES (1, 'Water purification', 'Water is essential.');`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range metaStatements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSearchCapabilityReusesAndClosesKiwix(t *testing.T) {
	root := t.TempDir()
	platformName, err := platform.Detect()
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "bin", platformName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(os.Args[0], filepath.Join(binDir, "kiwix-serve")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SVALBARD_TEST_MCP_KIWIX", "1")

	capability := NewSearchCapability(root, DriveMetadata{})
	if err := capability.ensureKiwix(); err != nil {
		t.Fatalf("first ensureKiwix() error = %v", err)
	}
	first := capability.kiwixCmd
	if err := capability.ensureKiwix(); err != nil {
		t.Fatalf("second ensureKiwix() error = %v", err)
	}
	if capability.kiwixCmd != first {
		t.Fatal("ensureKiwix() did not reuse the cached process")
	}
	if err := capability.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if first.ProcessState == nil {
		t.Fatal("Close() did not wait for the cached Kiwix process")
	}
}
