package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWaitHealthyCancelsInFlightProbeAndStopsProcess(t *testing.T) {
	requested := make(chan struct{})
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requested)
		<-r.Context().Done()
	}))
	defer health.Close()
	endpoint, err := url.Parse(health.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(endpoint.Port())
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &Server{proc: cmd, host: endpoint.Hostname(), port: port, stderr: &bytes.Buffer{}, done: make(chan struct{})}
	go func() { err := cmd.Wait(); s.waitMu.Lock(); s.waitErr = err; s.waitMu.Unlock(); close(s.done) }()
	t.Cleanup(s.Stop)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.waitHealthy(ctx) }()
	select {
	case <-requested:
	case <-time.After(3 * time.Second):
		t.Fatal("health probe did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("startup cancellation cause = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt health request")
	}
	s.Stop()
}

func TestWaitHealthyReportsEarlyExitAndPreservesCancellation(t *testing.T) {
	cmd := exec.Command("false")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &Server{proc: cmd, stderr: &bytes.Buffer{}, done: make(chan struct{})}
	go func() { err := cmd.Wait(); s.waitMu.Lock(); s.waitErr = err; s.waitMu.Unlock(); close(s.done) }()
	<-s.done
	if err := s.waitHealthy(context.Background()); err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("early exit diagnostic = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.waitHealthy(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled early exit cause = %v", err)
	}
	s.Stop()
}

func TestStopWaitsForSingleProcessReaper(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &Server{proc: cmd, done: make(chan struct{})}
	go func() { err := cmd.Wait(); s.waitMu.Lock(); s.waitErr = err; s.waitMu.Unlock(); close(s.done) }()
	done := make(chan struct{})
	go func() { s.Stop(); s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop hung waiting for process reaper")
	}
}

func TestEmbedBatchRetriesLongInputIndividually(t *testing.T) {
	t.Helper()

	var batchRequests int
	var singleRequests int
	var truncatedLongSeen bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embedding" {
			http.NotFound(w, r)
			return
		}

		var payload struct {
			Content []string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if len(payload.Content) > 1 {
			batchRequests++
			http.Error(w, `{"error":{"code":400,"message":"input (608 tokens) is larger than the max context size (512 tokens).\nskipping","type":"exceed_context_size_error","n_prompt_tokens":608,"n_ctx":512}}`, http.StatusBadRequest)
			return
		}

		singleRequests++
		text := payload.Content[0]
		if strings.Contains(text, "longword") && len(strings.Fields(text)) > 120 {
			http.Error(w, `{"error":{"code":400,"message":"input (608 tokens) is larger than the max context size (512 tokens).\nskipping","type":"exceed_context_size_error","n_prompt_tokens":608,"n_ctx":512}}`, http.StatusBadRequest)
			return
		}
		if strings.Contains(text, "longword") && strings.HasSuffix(text, "...") {
			truncatedLongSeen = true
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"embedding":[0.1,0.2,0.3]}]`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	server := &Server{host: u.Hostname(), port: port}
	texts := []string{
		"short input",
		"search_document: " + strings.Repeat("longword ", 300),
	}

	vectors, err := server.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch returned error: %v", err)
	}
	if len(vectors) != len(texts) {
		t.Fatalf("vector count = %d, want %d", len(vectors), len(texts))
	}
	if batchRequests != 1 {
		t.Fatalf("batch requests = %d, want 1", batchRequests)
	}
	if singleRequests < 3 {
		t.Fatalf("single requests = %d, want at least 3", singleRequests)
	}
	if !truncatedLongSeen {
		t.Fatal("expected truncated long input to be retried successfully")
	}
}

func TestEmbedBatchRejectsShortOrMixedDimensionResponse(t *testing.T) {
	for name, body := range map[string]string{
		"short": `[{"embedding":[1,2]}]`,
		"mixed": `[{"embedding":[1,2]},{"embedding":[1,2,3]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			u, _ := url.Parse(srv.URL)
			port, _ := strconv.Atoi(u.Port())
			server := &Server{host: u.Hostname(), port: port}
			if _, err := server.EmbedBatch(context.Background(), []string{"one", "two"}); err == nil {
				t.Fatal("EmbedBatch accepted malformed response")
			}
		})
	}
}
