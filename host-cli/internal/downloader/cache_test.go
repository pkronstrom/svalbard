package downloader

import (
	"context"
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
