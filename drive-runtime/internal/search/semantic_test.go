package search

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	drivebinary "github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/server"
)

func TestLongLivedSessionRefreshesCompletedContractAndUsesRecordedQueryShape(t *testing.T) {
	root := newSearchDrive(t)
	dbPath := filepath.Join(root, "data", "search.db")
	writer, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.Exec("CREATE TABLE embeddings (article_id INTEGER NOT NULL, chunk_header TEXT, vector BLOB NOT NULL)"); err != nil {
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
	contract := SemanticContract{Version: 1, Model: "models/embed/recorded.gguf", ModelSHA256: fmt.Sprintf("%x", hash), QueryPrefix: "search_query: ", Dimensions: 256, Matryoshka: true}
	sessionDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{db: sessionDB, eng: engine.New(sessionDB), embedBackend: NewSemanticBackend(root)}
	t.Cleanup(func() { _ = session.Close() })
	before, err := session.Search(context.Background(), ModeHybrid, "water", 10)
	if err != nil {
		t.Fatal(err)
	}
	if before.EffectiveMode != ModeKeyword || len(before.Results) != 1 {
		t.Fatalf("incomplete contract search = %#v, want keyword result", before)
	}
	raw, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	vector := make([]float32, 256)
	for i := range vector {
		vector[i] = 1.0 / 16
	}
	if _, err := writer.Exec("INSERT INTO meta (key, value) VALUES ('semantic_state', 'complete'), ('semantic_contract', ?)", string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec("INSERT INTO embeddings (article_id, chunk_header, vector) VALUES (1, '', ?)", float32Blob(vector)); err != nil {
		t.Fatal(err)
	}
	if loaded, ok := LoadCompleteContract(sessionDB); !ok || loaded != contract {
		t.Fatalf("long-lived session did not refresh completed contract: %#v, %v", loaded, ok)
	}
	var observedQuery string
	semanticResolveBinary = func(string, string) (string, error) { return "fake", nil }
	semanticStartEmbedding = func(_ context.Context, _, _ string, port int) (*exec.Cmd, error) {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return nil, err
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/embedding" {
				var request struct {
					Content []string `json:"content"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				observedQuery = request.Content[0]
				response := make([]float32, 768)
				for i := range response[:256] {
					response[i] = 1
				}
				_ = json.NewEncoder(w).Encode([]map[string][]float32{{"embedding": response}})
				return
			}
			w.WriteHeader(http.StatusOK)
		})}
		go server.Serve(listener)
		t.Cleanup(func() { _ = server.Close() })
		cmd := exec.Command("sleep", "30")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
	t.Cleanup(resetSemanticBackendSeams)
	after, err := session.Search(context.Background(), ModeHybrid, "water", 10)
	if err != nil {
		t.Fatal(err)
	}
	if after.EffectiveMode != ModeHybrid || len(after.Results) != 1 {
		t.Fatalf("completed contract search = %#v, embedded query = %q, want hybrid result", after, observedQuery)
	}
	if observedQuery != "search_query: water" {
		t.Fatalf("embedded query = %q", observedQuery)
	}
}

func float32Blob(vector []float32) []byte {
	buf := make([]byte, len(vector)*4)
	for i, value := range vector {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(value))
	}
	return buf
}

func TestSemanticBackendRetainsHealthyChildAfterCompletedRequest(t *testing.T) {
	root, contract := backendFixture(t)
	var starts atomic.Int32
	semanticResolveBinary = func(string, string) (string, error) { return "fake", nil }
	semanticStartEmbedding = fakeEmbeddingStarter(t, &starts)
	t.Cleanup(resetSemanticBackendSeams)
	b := NewSemanticBackend(root)
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := b.Embed(ctx, contract, "first"); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := b.Embed(context.Background(), contract, "second"); err != nil {
		t.Fatal(err)
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("backend starts = %d, want 1", got)
	}
}

func TestSemanticBackendRetriesAfterTransientStartupFailure(t *testing.T) {
	root, contract := backendFixture(t)
	var starts atomic.Int32
	semanticResolveBinary = func(string, string) (string, error) { return "fake", nil }
	starter := fakeEmbeddingStarter(t, &starts)
	semanticStartEmbedding = func(ctx context.Context, bin, model string, port int) (*exec.Cmd, error) {
		if starts.Load() == 0 {
			starts.Add(1)
			return nil, fmt.Errorf("transient startup failure")
		}
		return starter(ctx, bin, model, port)
	}
	t.Cleanup(resetSemanticBackendSeams)
	b := NewSemanticBackend(root)
	defer b.Close()
	if _, err := b.Embed(context.Background(), contract, "first"); err == nil {
		t.Fatal("transient startup failure accepted")
	}
	if _, err := b.Embed(context.Background(), contract, "second"); err != nil {
		t.Fatal(err)
	}
}

func TestSemanticBackendKeepsHealthyChildAfterCanceledRequest(t *testing.T) {
	root, contract := backendFixture(t)
	var starts atomic.Int32
	semanticResolveBinary = func(string, string) (string, error) { return "fake", nil }
	entered := make(chan struct{})
	semanticStartEmbedding = blockingEmbeddingStarter(t, &starts, entered)
	t.Cleanup(resetSemanticBackendSeams)
	b := NewSemanticBackend(root)
	defer b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { _, err := b.Embed(ctx, contract, "cancelled"); errCh <- err }()
	select {
	case <-entered:
	case err := <-errCh:
		t.Fatalf("Embed() failed before its request entered: %v", err)
	case <-time.After(time.Second):
		t.Fatal("embedding request did not enter server")
	}
	cancel()
	if err := <-errCh; err == nil {
		t.Fatal("cancelled request succeeded")
	}
	if _, err := b.Embed(context.Background(), contract, "second"); err != nil {
		t.Fatal(err)
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("backend starts = %d, want retained 1", got)
	}
}

func TestHashModelStopsAfterReaderSynchronizesCancellation(t *testing.T) {
	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := hashModel(ctx, reader)
		errCh <- err
	}()
	if _, err := writer.Write(make([]byte, 64*1024)); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != context.Canceled {
		t.Fatalf("hashModel() error = %v, want context.Canceled", err)
	}
}

func TestSemanticBackendRestartsForChangedContract(t *testing.T) {
	root, contract := backendFixture(t)
	second := contract
	second.Model = "models/embed/second.gguf"
	bytes := []byte("second")
	if err := os.WriteFile(filepath.Join(root, "models", "embed", "second.gguf"), bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(bytes)
	second.ModelSHA256 = fmt.Sprintf("%x", h)
	var starts atomic.Int32
	semanticResolveBinary = func(string, string) (string, error) { return "fake", nil }
	semanticStartEmbedding = fakeEmbeddingStarter(t, &starts)
	t.Cleanup(resetSemanticBackendSeams)
	b := NewSemanticBackend(root)
	defer b.Close()
	if _, err := b.Embed(context.Background(), contract, "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Embed(context.Background(), second, "second"); err != nil {
		t.Fatal(err)
	}
	if got := starts.Load(); got != 2 {
		t.Fatalf("backend starts = %d, want 2", got)
	}
}

func TestSemanticBackendRestartsAfterExitedChild(t *testing.T) {
	root, contract := backendFixture(t)
	var starts atomic.Int32
	semanticResolveBinary = func(string, string) (string, error) { return "fake", nil }
	semanticStartEmbedding = fakeEmbeddingStarter(t, &starts)
	t.Cleanup(resetSemanticBackendSeams)
	b := NewSemanticBackend(root)
	defer b.Close()
	if _, err := b.Embed(context.Background(), contract, "first"); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	cmd, done := b.cmd, b.done
	b.mu.Unlock()
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-done
	if _, err := b.Embed(context.Background(), contract, "second"); err != nil {
		t.Fatal(err)
	}
	if got := starts.Load(); got != 2 {
		t.Fatalf("backend starts = %d, want 2", got)
	}
}

func backendFixture(t *testing.T) (string, SemanticContract) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "models", "embed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bytes := []byte("recorded")
	if err := os.WriteFile(filepath.Join(dir, "recorded.gguf"), bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(bytes)
	return root, SemanticContract{Version: 1, Model: "models/embed/recorded.gguf", ModelSHA256: fmt.Sprintf("%x", h), Dimensions: 2}
}

func fakeEmbeddingStarter(t *testing.T, starts *atomic.Int32) func(context.Context, string, string, int) (*exec.Cmd, error) {
	t.Helper()
	return func(_ context.Context, _, _ string, port int) (*exec.Cmd, error) {
		starts.Add(1)
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return nil, err
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/embedding" {
				_, _ = w.Write([]byte(`[{"embedding":[3,4]}]`))
				return
			}
			w.WriteHeader(http.StatusOK)
		})}
		go server.Serve(listener)
		t.Cleanup(func() { _ = server.Close() })
		cmd := exec.Command("sleep", "30")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
}

func blockingEmbeddingStarter(t *testing.T, starts *atomic.Int32, entered chan<- struct{}) func(context.Context, string, string, int) (*exec.Cmd, error) {
	t.Helper()
	var requests atomic.Int32
	return func(_ context.Context, _, _ string, port int) (*exec.Cmd, error) {
		starts.Add(1)
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return nil, err
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/embedding" {
				if requests.Add(1) == 1 {
					select {
					case entered <- struct{}{}:
					default:
					}
					<-r.Context().Done()
					return
				}
				_, _ = w.Write([]byte(`[{"embedding":[3,4]}]`))
				return
			}
			w.WriteHeader(http.StatusOK)
		})}
		go server.Serve(listener)
		t.Cleanup(func() { _ = server.Close() })
		cmd := exec.Command("sleep", "30")
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
}

func resetSemanticBackendSeams() {
	semanticResolveBinary = drivebinary.Resolve
	semanticStartEmbedding = server.StartEmbedding
}

func TestResolveModelUsesOnlyRecordedGGUFAndHash(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "models", "embed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.gguf"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "recorded.gguf")
	if err := os.WriteFile(model, []byte("recorded"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("recorded"))
	contract := SemanticContract{Version: 1, Model: "models/embed/recorded.gguf", ModelSHA256: fmt.Sprintf("%x", hash), Dimensions: 2}
	if got, err := resolveRecordedModel(context.Background(), root, contract); err != nil || got != model {
		t.Fatalf("resolve = %q, %v", got, err)
	}
	if err := os.WriteFile(model, []byte("replaced"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveRecordedModel(context.Background(), root, contract); err == nil {
		t.Fatal("replaced recorded model was accepted")
	}
}

func semanticSnapshotDB(t *testing.T, contract SemanticContract) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO meta (key, value) VALUES ('semantic_state', 'complete'), ('semantic_contract', ?)", string(raw)); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHybridSnapshotFallsBackWhenWriterSwitchesToBuilding(t *testing.T) {
	contract := SemanticContract{Version: 1, Model: "models/embed/model.gguf", ModelSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Dimensions: 2}
	db := semanticSnapshotDB(t, contract)
	loaded, ok := LoadCompleteContract(db)
	if !ok || loaded != contract {
		t.Fatalf("initial contract = %#v, %v", loaded, ok)
	}
	// This simulates a long-lived native or MCP request that loaded complete,
	// then spent time embedding while an incompatible writer began rebuilding.
	if _, err := db.Exec("UPDATE meta SET value = 'building' WHERE key = 'semantic_state'"); err != nil {
		t.Fatal(err)
	}
	if _, usable, err := HybridSnapshot(context.Background(), db, loaded, "article", []float32{1, 0}, 10); err != nil || usable {
		t.Fatalf("building generation snapshot = usable %v, err %v", usable, err)
	}
}

func TestHybridSnapshotFallsBackWhenContractGenerationChanges(t *testing.T) {
	contract := SemanticContract{Version: 1, Model: "models/embed/model.gguf", ModelSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Dimensions: 2}
	db := semanticSnapshotDB(t, contract)
	loaded, ok := LoadCompleteContract(db)
	if !ok {
		t.Fatal("missing initial complete contract")
	}
	replacement := contract
	replacement.ModelSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	raw, err := json.Marshal(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE meta SET value = ? WHERE key = 'semantic_contract'", string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, usable, err := HybridSnapshot(context.Background(), db, loaded, "article", []float32{1, 0}, 10); err != nil || usable {
		t.Fatalf("changed generation snapshot = usable %v, err %v", usable, err)
	}
}

func TestHybridSnapshotRetainsCapturedBodyAfterWriterReplacement(t *testing.T) {
	contract := SemanticContract{Version: 1, Model: "models/embed/model.gguf", ModelSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Dimensions: 2}
	db, dbPath := semanticSearchDB(t, contract)
	metadata, usable, err := HybridSnapshot(context.Background(), db, contract, "water", []float32{1, 0}, 1)
	if err != nil || !usable || len(metadata) != 1 {
		t.Fatalf("HybridSnapshot() = %#v, usable %v, err %v", metadata, usable, err)
	}
	results, usable, err := HybridSnapshotFull(context.Background(), db, contract, "water", []float32{1, 0}, 10, "")
	if err != nil || !usable || len(results) != 1 {
		t.Fatalf("HybridSnapshot() = %#v, usable %v, err %v", results, usable, err)
	}
	if results[0].ArticleID != metadata[0].ArticleID || results[0].Score != metadata[0].Score {
		t.Fatalf("full hybrid changed global result selection: metadata=%#v full=%#v", metadata[0], results[0])
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.Exec("UPDATE articles SET body = 'replacement body' WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if results[0].Body != "original full body" {
		t.Fatalf("snapshot body = %q, want original full body", results[0].Body)
	}
}

func semanticSearchDB(t *testing.T, contract SemanticContract) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "search.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	raw, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE sources (id INTEGER PRIMARY KEY, filename TEXT NOT NULL)`,
		`CREATE TABLE articles (id INTEGER PRIMARY KEY, source_id INTEGER NOT NULL, path TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '')`,
		`CREATE VIRTUAL TABLE articles_fts USING fts5(title, body, content='articles', content_rowid='id')`,
		`CREATE TABLE embeddings (article_id INTEGER NOT NULL, chunk_header TEXT, vector BLOB NOT NULL)`,
		`INSERT INTO sources VALUES (1, 'wiki.zim')`,
		`INSERT INTO articles VALUES (1, 1, '/Water', 'Water purification', 'original full body')`,
		`INSERT INTO articles_fts(rowid, title, body) VALUES (1, 'Water purification', 'original full body')`,
		`INSERT INTO embeddings VALUES (1, '', X'0000803f00000000')`} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO meta VALUES ('semantic_state', 'complete'), ('semantic_contract', ?)", string(raw)); err != nil {
		t.Fatal(err)
	}
	return db, path
}
