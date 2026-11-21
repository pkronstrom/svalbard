package search

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEmbeddingQueryTimesOutWhenHealthyServerStalls(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	previous := semanticQueryTimeout
	semanticQueryTimeout = 200 * time.Millisecond
	defer func() { semanticQueryTimeout = previous }()
	start := time.Now()
	_, err := embedQuery(context.Background(), "query", srv.Listener.Addr().(*net.TCPAddr).Port)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stalled embedding error = %v, want deadline exceeded", err)
	}
	select {
	case <-entered:
	default:
		t.Fatal("timeout did not exercise an accepted embedding request")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("stalled embedding request took %s", elapsed)
	}
}
