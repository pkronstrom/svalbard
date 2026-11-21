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

func TestCacheDirsParsing(t *testing.T) {
	t.Setenv(CacheEnvVar, "")
	if got := CacheDirs(); got != nil {
		t.Errorf("empty env → %v, want nil", got)
	}

	home, _ := os.UserHomeDir()
	t.Setenv(CacheEnvVar, "/a/b: : ~/models ")
	got := CacheDirs()
	want := []string{"/a/b", filepath.Join(home, "models")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("CacheDirs() = %v, want %v", got, want)
	}
}

func TestFindInCache(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "unsloth", "sub")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(nested, "model-Q4_K_XL.gguf")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// empty file should be ignored
	if err := os.WriteFile(filepath.Join(root, "empty.gguf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if got, ok := FindInCache([]string{root}, "model-Q4_K_XL.gguf"); !ok || got != target {
		t.Errorf("FindInCache = %q,%v, want %q,true", got, ok, target)
	}
	if _, ok := FindInCache([]string{root}, "empty.gguf"); ok {
		t.Error("empty file should not match")
	}
	if _, ok := FindInCache([]string{root}, "missing.gguf"); ok {
		t.Error("missing file should not match")
	}
}

func TestReuseFromCacheSizeMatch(t *testing.T) {
	body := []byte("hello-model-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "cached.gguf")
	if err := os.WriteFile(cache, body, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "models", "out.gguf")

	res, used, err := ReuseFromCache(context.Background(), cache, srv.URL, dest)
	if err != nil || !used {
		t.Fatalf("ReuseFromCache used=%v err=%v, want used=true", used, err)
	}
	if res.SHA256 == "" || !res.Cached {
		t.Errorf("result = %+v, want hashed + cached", res)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(body) {
		t.Errorf("dest contents = %q, want %q", got, body)
	}
}

func TestReuseFromCacheSizeMismatchFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "99999") // differs from cache size
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "cached.gguf")
	if err := os.WriteFile(cache, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out.gguf")

	_, used, err := ReuseFromCache(context.Background(), cache, srv.URL, dest)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if used {
		t.Error("size mismatch should not reuse cache (should fall back to download)")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dest should not be written on mismatch")
	}
}

func TestReuseFromCacheOfflineTrustsCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cached.gguf")
	if err := os.WriteFile(cache, []byte("offline-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out.gguf")

	// Unreachable URL → remoteSize fails → trust the cached file.
	_, used, err := ReuseFromCache(context.Background(), cache, "http://127.0.0.1:1/x.gguf", dest)
	if err != nil || !used {
		t.Fatalf("offline ReuseFromCache used=%v err=%v, want used=true", used, err)
	}
}

func TestReuseFromCacheCancellationLeavesFinalAndHTTPPartialUntouched(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cached.gguf")
	if err := os.WriteFile(cache, make([]byte, 2*chunkSize), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out.gguf")
	if err := os.WriteFile(dest, []byte("old-final"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest+".part", []byte("http-partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, used, err := ReuseFromCache(ctx, cache, "http://127.0.0.1:1/x.gguf", dest, func(_, _ int64) { cancel() })
	if err == nil || used {
		t.Fatalf("cancelled reuse = used=%v err=%v, want unused cancellation", used, err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "old-final" {
		t.Fatalf("final was replaced with %q", got)
	}
	if got, _ := os.ReadFile(dest + ".part"); string(got) != "http-partial" {
		t.Fatalf("HTTP partial was changed to %q", got)
	}
	if _, err := os.Stat(dest + ".cache.part"); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestRejectedCachePreservesHTTPResumeOffset(t *testing.T) {
	content := []byte("hello world")
	hash := sha256.Sum256(content)
	expected := hex.EncodeToString(hash[:])
	var requestedRange string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"version-one"`)
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "11")
			return
		}
		requestedRange = r.Header.Get("Range")
		w.Header().Set("Content-Range", "bytes 5-10/11")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[5:])
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest+".part", content[:5], 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: server.URL, ExpectedSHA256: expected, ETag: `"version-one"`, Size: 11}); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cached-file")
	if err := os.WriteFile(cache, []byte("wrong bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, used, err := ReuseFromCacheWithSHA256(context.Background(), cache, server.URL, dest, expected); err != nil || used {
		t.Fatalf("cache used=%v error=%v", used, err)
	}
	if _, err := Download(context.Background(), server.URL, dest, expected); err != nil {
		t.Fatal(err)
	}
	if requestedRange != "bytes=5-" {
		t.Fatalf("range=%q, want original offset", requestedRange)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != string(content) {
		t.Fatalf("content=%q error=%v", got, err)
	}
}

func TestCompletedCacheArtifactIsReusableWithoutCacheOrNetwork(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cached")
	content := []byte("complete offline cache")
	if err := os.WriteFile(cache, content, 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "file")
	source := "http://127.0.0.1:1/unavailable"
	if _, used, err := ReuseFromCache(context.Background(), cache, source, dest); err != nil || !used {
		t.Fatalf("cache promotion: used=%v error=%v", used, err)
	}
	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	if result, err := Download(context.Background(), source, dest, ""); err != nil || !result.Cached {
		t.Fatalf("post-promotion offline restart lost completion: %+v %v", result, err)
	}
}

func TestCompletedCachePartialRecoversBeforePromotion(t *testing.T) {
	content := []byte("complete offline cache")
	hash := sha256.Sum256(content)
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest+".cache.part", content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest+".part", []byte("older HTTP prefix"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: "http://127.0.0.1:1/unavailable", Size: int64(len(content)), SHA256: hex.EncodeToString(hash[:]), Complete: true}); err != nil {
		t.Fatal(err)
	}
	if result, err := Download(context.Background(), "http://127.0.0.1:1/unavailable", dest, ""); err != nil || !result.Cached {
		t.Fatalf("pre-promotion cache recovery failed: %+v %v", result, err)
	}
}
