package search

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	drivebinary "github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/server"
)

// SemanticContract is the reader-side form of the host semantic contract.
type SemanticContract struct {
	Version     int    `json:"version"`
	Model       string `json:"model"`
	ModelSHA256 string `json:"model_sha256"`
	DocPrefix   string `json:"doc_prefix"`
	QueryPrefix string `json:"query_prefix"`
	Dimensions  int    `json:"dimensions"`
	Matryoshka  bool   `json:"matryoshka"`
}

type semanticQuery interface{ QueryRow(string, ...any) *sql.Row }

func LoadCompleteContract(q semanticQuery) (SemanticContract, bool) {
	var state, raw string
	if err := q.QueryRow("SELECT value FROM meta WHERE key='semantic_state'").Scan(&state); err != nil || state != "complete" {
		return SemanticContract{}, false
	}
	if err := q.QueryRow("SELECT value FROM meta WHERE key='semantic_contract'").Scan(&raw); err != nil {
		return SemanticContract{}, false
	}
	var contract SemanticContract
	if err := json.Unmarshal([]byte(raw), &contract); err != nil || !contract.valid() {
		return SemanticContract{}, false
	}
	return contract, true
}

func (c SemanticContract) valid() bool {
	if c.Version != 1 || c.Dimensions <= 0 || c.ModelSHA256 == "" || c.Model == "" {
		return false
	}
	if len(c.ModelSHA256) != sha256.Size*2 {
		return false
	}
	if _, err := hex.DecodeString(c.ModelSHA256); err != nil {
		return false
	}
	clean := filepath.Clean(c.Model)
	return !filepath.IsAbs(c.Model) && clean == c.Model && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) && strings.HasPrefix(filepath.ToSlash(clean), "models/embed/")
}

// SemanticBackend is the shared retained llama-server lifecycle used by native
// and MCP search. Requests can be cancelled without transferring ownership of
// a successfully healthy child to that request.
type SemanticBackend struct {
	driveRoot string
	mu        sync.Mutex
	cmd       *exec.Cmd
	port      int
	contract  SemanticContract
	done      chan struct{}
}

var (
	semanticResolveBinary  = drivebinary.Resolve
	semanticStartEmbedding = server.StartEmbedding
	semanticQueryTimeout   = 30 * time.Second
)

func NewSemanticBackend(driveRoot string) *SemanticBackend {
	return &SemanticBackend{driveRoot: driveRoot}
}

func (b *SemanticBackend) Embed(ctx context.Context, contract SemanticContract, text string) ([]float32, error) {
	if !contract.valid() {
		return nil, fmt.Errorf("invalid semantic contract")
	}
	if err := b.ensure(ctx, contract); err != nil {
		return nil, err
	}
	b.mu.Lock()
	port := b.port
	b.mu.Unlock()
	v, err := embedQuery(ctx, contract.QueryPrefix+text, port)
	if err != nil {
		return nil, err
	}
	return shapeQuery(v, contract)
}

func (b *SemanticBackend) ensure(ctx context.Context, contract SemanticContract) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cmd != nil {
		if b.contract == contract {
			select {
			case <-b.done:
				b.cmd, b.port, b.done = nil, 0, nil
			default:
				return nil
			}
		}
		if b.cmd != nil {
			// A new completed generation selects a new exact model. Reap the old
			// child before replacing it; the waiter owns Wait in both paths.
			cmd, done := b.cmd, b.done
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-done
			b.cmd, b.port, b.done = nil, 0, nil
		}
	}
	model, err := resolveRecordedModel(ctx, b.driveRoot, contract)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	bin, err := semanticResolveBinary("llama-server", b.driveRoot)
	if err != nil {
		return err
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", 8085)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd, err := semanticStartEmbedding(ctx, bin, model, port)
	if err != nil {
		return err
	}
	done := make(chan struct{})
	b.cmd, b.port, b.contract, b.done = cmd, port, contract, done
	go b.reap(cmd, done)
	return nil
}

func (b *SemanticBackend) reap(cmd *exec.Cmd, done chan struct{}) {
	_ = cmd.Wait()
	close(done)
	b.mu.Lock()
	if b.cmd == cmd {
		b.cmd = nil
		b.port = 0
		b.done = nil
	}
	b.mu.Unlock()
}

func (b *SemanticBackend) Close() error {
	b.mu.Lock()
	cmd, done := b.cmd, b.done
	b.mu.Unlock()
	if cmd == nil {
		return nil
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-done
	return nil
}

// resolveRecordedModel accepts only the model named by a completed contract
// and verifies its current bytes before a backend is opened.
func resolveRecordedModel(ctx context.Context, root string, contract SemanticContract) (string, error) {
	if !contract.valid() {
		return "", fmt.Errorf("invalid semantic contract")
	}
	path := filepath.Join(root, filepath.FromSlash(contract.Model))
	rootClean, pathClean := filepath.Clean(root), filepath.Clean(path)
	if pathClean != rootClean && !strings.HasPrefix(pathClean, rootClean+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid model path")
	}
	f, err := os.Open(pathClean)
	if err != nil {
		return "", fmt.Errorf("recorded embedding model unavailable: %w", err)
	}
	defer f.Close()
	digest, err := hashModel(ctx, f)
	if err != nil {
		return "", err
	}
	if digest != strings.ToLower(contract.ModelSHA256) {
		return "", fmt.Errorf("recorded embedding model hash changed")
	}
	return pathClean, nil
}

func hashModel(ctx context.Context, r io.Reader) (string, error) {
	h := sha256.New()
	buf := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := r.Read(buf)
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if n > 0 {
			if _, err := h.Write(buf[:n]); err != nil {
				return "", err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("hashing recorded embedding model: %w", readErr)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func shapeQuery(vector []float32, contract SemanticContract) ([]float32, error) {
	if len(vector) < contract.Dimensions {
		return nil, fmt.Errorf("query vector dimensions %d below contract %d", len(vector), contract.Dimensions)
	}
	if len(vector) != contract.Dimensions && !contract.Matryoshka {
		return nil, fmt.Errorf("query vector dimensions %d differ from contract %d", len(vector), contract.Dimensions)
	}
	v := append([]float32(nil), vector[:contract.Dimensions]...)
	var sum float64
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return nil, fmt.Errorf("query vector contains non-finite value")
		}
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return nil, fmt.Errorf("query vector has zero norm")
	}
	scale := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= scale
	}
	return v, nil
}

func embedQuery(ctx context.Context, query string, port int) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, semanticQueryTimeout)
	defer cancel()
	payload, _ := json.Marshal(map[string][]string{"content": {query}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/embedding", port), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embed query: server returned %d: %s", resp.StatusCode, body)
	}
	var data []struct {
		Embedding json.RawMessage `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("embed query: decode response: %w", err)
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("embed query: expected one response")
	}
	var nested [][]float32
	if err := json.Unmarshal(data[0].Embedding, &nested); err == nil && len(nested) == 1 {
		return nested[0], nil
	}
	var vector []float32
	if err := json.Unmarshal(data[0].Embedding, &vector); err != nil {
		return nil, fmt.Errorf("embed query: unmarshal: %w", err)
	}
	return vector, nil
}

// HybridSnapshot scores vectors only when the same complete contract remains
// visible in one read transaction after query embedding.
func HybridSnapshot(ctx context.Context, db *sql.DB, contract SemanticContract, query string, vector []float32, limit int) ([]Result, bool, error) {
	return hybridSnapshot(ctx, db, contract, query, vector, limit, limit, "", false)
}

// HybridSnapshotFull captures complete bodies only after hybrid ranking, source
// filtering, and the MCP result limit have been decided in one read snapshot.
func HybridSnapshotFull(ctx context.Context, db *sql.DB, contract SemanticContract, query string, vector []float32, limit int, source string) ([]Result, bool, error) {
	candidateLimit := limit
	if source != "" && candidateLimit < 200 {
		candidateLimit = 200
	}
	return hybridSnapshot(ctx, db, contract, query, vector, candidateLimit, limit, source, true)
}

func hybridSnapshot(ctx context.Context, db *sql.DB, contract SemanticContract, query string, vector []float32, candidateLimit, finalLimit int, source string, fullBodies bool) ([]Result, bool, error) {
	if !contract.valid() || len(vector) != contract.Dimensions {
		return nil, false, fmt.Errorf("semantic query is incompatible with completed contract")
	}
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, false, fmt.Errorf("semantic query contains non-finite value")
		}
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	snapshot, ok := LoadCompleteContract(tx)
	if !ok || snapshot != contract {
		return nil, false, nil
	}
	results, err := engine.New(tx).HybridContext(ctx, query, vector, candidateLimit)
	if err != nil {
		return nil, false, err
	}
	if !fullBodies {
		return results, true, nil
	}
	filtered := results[:0]
	for _, result := range results {
		if source == "" || strings.TrimSuffix(result.Filename, ".zim") == source {
			filtered = append(filtered, result)
		}
	}
	if len(filtered) > finalLimit {
		filtered = filtered[:finalLimit]
	}
	for i := range filtered {
		var body sql.NullString
		if err := tx.QueryRow("SELECT body FROM articles WHERE id = ?", filtered[i].ArticleID).Scan(&body); err != nil {
			return nil, false, err
		}
		filtered[i].Body = body.String
	}
	return filtered, true, nil
}
