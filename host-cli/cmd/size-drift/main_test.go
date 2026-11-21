package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestCheckRecipeMeasuresLargestPlatformAndFlagsDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s, want HEAD", r.Method)
		}
		sizes := map[string]string{"/small": "100000000", "/large": "300000000", "/close": "110000000"}
		w.Header().Set("Content-Length", sizes[r.URL.Path])
	}))
	defer server.Close()

	drifted := checkRecipe(context.Background(), server.Client(), catalog.Item{
		ID: "tool", SizeGB: 0.1,
		Platforms: map[string]string{"linux-x86_64": server.URL + "/small", "macos-arm64": server.URL + "/large"},
	}, 0.3, 0.01)
	if drifted.Err != nil || drifted.ActualGB != 0.3 || !drifted.Drifted {
		t.Fatalf("multi-platform result = %+v, want largest asset 0.3 GB flagged", drifted)
	}

	close := checkRecipe(context.Background(), server.Client(), catalog.Item{ID: "zim", SizeGB: 0.1, URL: server.URL + "/close"}, 0.3, 0.01)
	if close.Err != nil || close.Drifted {
		t.Fatalf("10%% drift result = %+v, want within threshold", close)
	}
}

func TestCheckRecipeIgnoresDriftBelowMinimumSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3000000")
	}))
	defer server.Close()

	// 1 MB declared, 3 MB upstream: +200% but only 2 MB.
	result := checkRecipe(context.Background(), server.Client(), catalog.Item{ID: "tiny", SizeGB: 0.001, URL: server.URL + "/t"}, 0.3, 0.01)
	if result.Err != nil || result.Drifted {
		t.Fatalf("result = %+v, want small absolute drift ignored", result)
	}
}

func TestCheckRecipeReportsUnknownSizeWithoutContentLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Transfer-Encoding", "chunked")
	}))
	defer server.Close()

	result := checkRecipe(context.Background(), server.Client(), catalog.Item{ID: "stream", SizeGB: 0.1, URL: server.URL + "/x"}, 0.3, 0.01)
	if result.Err == nil {
		t.Fatalf("result = %+v, want unknown-size error", result)
	}
}
