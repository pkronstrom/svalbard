package embedder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

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
