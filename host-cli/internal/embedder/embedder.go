// Package embedder manages a llama-server subprocess for generating text
// embeddings via a GGUF embedding model (e.g. nomic-embed-text-v1.5).
package embedder

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	proc   *exec.Cmd
	host   string
	port   int
	stderr *bytes.Buffer
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

	var stderrBuf bytes.Buffer
	s := &Server{host: defaultHost, port: port, stderr: &stderrBuf}
	s.proc = exec.CommandContext(ctx, bin,
		"--model", modelPath,
		"--port", fmt.Sprintf("%d", s.port),
		"--host", s.host,
		"--embedding",
		"--ubatch-size", "4096",
		"--batch-size", "4096",
	)
	s.proc.Stdout = nil
	s.proc.Stderr = &stderrBuf

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
		// Wait may have already been called by waitHealthy's exit detector;
		// calling it again is harmless (returns "wait: already waited").
		_ = s.proc.Wait()
	}
}

// EmbedBatch sends texts to the llama-server /embedding endpoint and returns
// one float32 vector per input text.
func (s *Server) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors, err := s.embedBatchOnce(ctx, texts)
	if err == nil {
		return vectors, nil
	}

	var limitErr *contextLimitError
	if !errors.As(err, &limitErr) {
		return nil, err
	}

	// A single oversized document should not fail the whole indexing batch.
	vectors = make([][]float32, 0, len(texts))
	for _, text := range texts {
		vec, err := s.embedSingleWithRetry(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vec)
	}
	return vectors, nil
}

func (s *Server) embedSingleWithRetry(ctx context.Context, text string) ([]float32, error) {
	current := text
	for range 6 {
		vectors, err := s.embedBatchOnce(ctx, []string{current})
		if err == nil {
			return vectors[0], nil
		}

		var limitErr *contextLimitError
		if !errors.As(err, &limitErr) {
			return nil, err
		}

		next := shrinkTextForContext(current, limitErr.PromptTokens, limitErr.ContextSize)
		if next == current {
			return nil, err
		}
		current = next
	}
	return nil, fmt.Errorf("embedder: exceeded retry budget while shrinking oversized input")
}

func (s *Server) embedBatchOnce(ctx context.Context, texts []string) ([][]float32, error) {
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
		if limitErr := parseContextLimitError(body); limitErr != nil {
			return nil, limitErr
		}
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

type contextLimitError struct {
	Message      string
	PromptTokens int
	ContextSize  int
}

func (e *contextLimitError) Error() string {
	return fmt.Sprintf("embedder: %s", e.Message)
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

// TruncateDims truncates a vector to the first dims elements and L2-normalizes.
// If the vector is shorter than dims, it is normalized in place but not grown.
func TruncateDims(vec []float32, dims int) []float32 {
	if dims > 0 && dims < len(vec) {
		vec = vec[:dims]
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
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

	// Channel to detect early process exit.
	exited := make(chan error, 1)
	go func() { exited <- s.proc.Wait() }()

	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			detail := strings.TrimSpace(s.stderr.String())
			if len(detail) > 300 {
				detail = detail[len(detail)-300:]
			}
			if detail != "" {
				return fmt.Errorf("embedder: llama-server exited (%v): %s", err, detail)
			}
			return fmt.Errorf("embedder: llama-server exited: %v", err)
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	detail := strings.TrimSpace(s.stderr.String())
	if len(detail) > 300 {
		detail = detail[len(detail)-300:]
	}
	if detail != "" {
		return fmt.Errorf("embedder: llama-server not healthy after %s: %s", healthTimeout, detail)
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

func parseContextLimitError(body []byte) *contextLimitError {
	var payload struct {
		Error struct {
			Message       string `json:"message"`
			Type          string `json:"type"`
			PromptTokens  int    `json:"n_prompt_tokens"`
			ContextWindow int    `json:"n_ctx"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	if payload.Error.Type != "exceed_context_size_error" {
		return nil
	}
	return &contextLimitError{
		Message:      payload.Error.Message,
		PromptTokens: payload.Error.PromptTokens,
		ContextSize:  payload.Error.ContextWindow,
	}
}

func shrinkTextForContext(text string, promptTokens, contextSize int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	safeCtx := contextSize - 64
	if safeCtx < 64 {
		safeCtx = contextSize / 2
	}
	if safeCtx <= 0 || promptTokens <= 0 {
		safeCtx = 256
		promptTokens = 512
	}

	ratio := float64(safeCtx) / float64(promptTokens)
	if ratio > 0.85 {
		ratio = 0.85
	}
	if ratio <= 0 {
		ratio = 0.5
	}

	targetWords := int(float64(len(words)) * ratio)
	if targetWords >= len(words) {
		targetWords = len(words) - 1
	}
	if targetWords < 32 {
		targetWords = 32
	}
	if targetWords >= len(words) {
		targetWords = len(words) / 2
		if targetWords < 1 {
			targetWords = 1
		}
	}

	shrunk := strings.Join(words[:targetWords], " ")
	if shrunk == text {
		runes := []rune(text)
		limit := int(float64(len(runes)) * ratio)
		if limit >= len(runes) {
			limit = len(runes) - 1
		}
		if limit < 16 {
			limit = 16
		}
		if limit >= len(runes) {
			return text
		}
		shrunk = string(runes[:limit])
	}
	if !strings.HasSuffix(shrunk, "...") {
		shrunk += "..."
	}
	return shrunk
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
