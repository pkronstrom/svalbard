// Package embedder manages a llama-server subprocess for generating text
// embeddings via the nomic-embed-text-v1.5 model (or compatible).
package embedder

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPort    = 8085
	defaultHost    = "127.0.0.1"
	healthTimeout  = 30 * time.Second
	requestTimeout = 120 * time.Second
)

// Server manages a llama-server subprocess running in embedding mode.
type Server struct {
	proc *exec.Cmd
	host string
	port int
}

// StartServer launches llama-server with the given model in embedding mode.
// It searches for the binary in driveRoot/bin/ (extracting archives if needed),
// then falls back to system PATH.
// It waits up to 30 seconds for the /health endpoint to return 200.
func StartServer(ctx context.Context, modelPath, driveRoot string) (*Server, error) {
	bin, err := resolveBinary("llama-server", driveRoot)
	if err != nil {
		return nil, err
	}

	port, err := findFreePort()
	if err != nil {
		return nil, fmt.Errorf("embedder: find free port: %w", err)
	}

	s := &Server{host: defaultHost, port: port}
	s.proc = exec.CommandContext(ctx, bin,
		"--model", modelPath,
		"--port", fmt.Sprintf("%d", s.port),
		"--host", s.host,
		"--embedding",
		"--ubatch-size", "4096",
		"--batch-size", "4096",
	)
	s.proc.Stdout = nil
	s.proc.Stderr = nil

	if err := s.proc.Start(); err != nil {
		return nil, fmt.Errorf("embedder: start llama-server: %w", err)
	}

	if err := s.waitHealthy(); err != nil {
		s.Stop()
		return nil, err
	}
	return s, nil
}

// Stop kills the llama-server process.
func (s *Server) Stop() {
	if s.proc != nil && s.proc.Process != nil {
		_ = s.proc.Process.Kill()
		_ = s.proc.Wait()
	}
}

// EmbedBatch sends texts to the llama-server /embedding endpoint and returns
// one float32 vector per input text.
func (s *Server) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	url := fmt.Sprintf("http://%s:%d/embedding", s.host, s.port)

	payload, err := json.Marshal(map[string]any{"content": texts})
	if err != nil {
		return nil, fmt.Errorf("embedder: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("embedder: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedder: POST /embedding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embedder: /embedding returned %d: %s", resp.StatusCode, body)
	}

	var items []struct {
		Embedding json.RawMessage `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("embedder: decode response: %w", err)
	}

	vectors := make([][]float32, 0, len(items))
	for i, item := range items {
		vec, err := parseEmbedding(item.Embedding)
		if err != nil {
			return nil, fmt.Errorf("embedder: parse embedding[%d]: %w", i, err)
		}
		vectors = append(vectors, vec)
	}
	return vectors, nil
}

// VectorToBlob packs a float32 vector as a little-endian binary blob.
func VectorToBlob(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// BlobToVector unpacks a little-endian float32 blob into a vector.
func BlobToVector(blob []byte) []float32 {
	n := len(blob) / 4
	vec := make([]float32, n)
	for i := range n {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return vec
}

// --- internal helpers ---

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

func (s *Server) waitHealthy() error {
	url := fmt.Sprintf("http://%s:%d/health", s.host, s.port)
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(healthTimeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("embedder: llama-server not healthy after %s", healthTimeout)
}


// parseEmbedding handles both flat vectors and the nested [[...]] format
// that some llama-server versions produce.
func parseEmbedding(raw json.RawMessage) ([]float32, error) {
	// Try flat array first: [0.1, 0.2, ...]
	var flat []float32
	if err := json.Unmarshal(raw, &flat); err == nil {
		return flat, nil
	}

	// Try nested: [[0.1, 0.2, ...]]
	var nested [][]float32
	if err := json.Unmarshal(raw, &nested); err == nil && len(nested) > 0 {
		return nested[0], nil
	}

	return nil, fmt.Errorf("unexpected embedding format: %s", truncate(string(raw), 100))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// FindEmbeddingModel returns the path to the first .gguf file in models/embed/.
func FindEmbeddingModel(driveRoot string) (string, error) {
	embedDir := filepath.Join(driveRoot, "models", "embed")
	matches, err := filepath.Glob(filepath.Join(embedDir, "*.gguf"))
	if err != nil {
		return "", fmt.Errorf("embedder: glob models/embed: %w", err)
	}
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), "._") {
			return m, nil
		}
	}
	return "", fmt.Errorf("embedder: no embedding model found in %s", embedDir)
}
