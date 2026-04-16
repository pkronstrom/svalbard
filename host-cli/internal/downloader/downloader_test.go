package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadNewFile(t *testing.T) {
	content := "hello world test content for download"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "file.txt")
	result, err := Download(context.Background(), ts.URL+"/file.txt", dest, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Cached {
		t.Error("should not be cached")
	}
	if result.SHA256 == "" {
		t.Error("missing sha256")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != content {
		t.Errorf("content = %q", got)
	}
}

func TestDownloadResumesPartialFile(t *testing.T) {
	content := "hello world test content"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" {
			var start int
			fmt.Sscanf(rangeHdr, "bytes=%d-", &start)
			if start >= len(content) {
				w.WriteHeader(416) // Range not satisfiable
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(content)-1, len(content)))
			w.WriteHeader(206)
			w.Write([]byte(content[start:]))
			return
		}
		w.Write([]byte(content))
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "file.txt")
	// Write partial content
	os.WriteFile(dest, []byte("hello"), 0644)

	result, err := Download(context.Background(), ts.URL+"/file.txt", dest, "")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != content {
		t.Errorf("content after resume = %q", got)
	}
	_ = result
}

func TestDownloadSkipsWhenCached(t *testing.T) {
	content := "cached content"
	h := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(h[:])

	dest := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(dest, []byte(content), 0644)

	requestMade := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		w.Write([]byte(content))
	}))
	defer ts.Close()

	result, err := Download(context.Background(), ts.URL+"/file.txt", dest, expected)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cached {
		t.Error("should be cached")
	}
	if requestMade {
		t.Error("should not have made HTTP request")
	}
}

func TestDownloadRejectsBadSHA256(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("actual content"))
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "file.txt")
	_, err := Download(context.Background(), ts.URL+"/file.txt", dest, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected SHA256 mismatch error")
	}
}

func TestComputeSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	content := "test data"
	os.WriteFile(path, []byte(content), 0644)

	got, err := ComputeSHA256(path)
	if err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256([]byte(content))
	want := hex.EncodeToString(h[:])
	if got != want {
		t.Errorf("sha256 = %q, want %q", got, want)
	}
}
