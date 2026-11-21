package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type registryTestTransport struct{ server *url.URL }

func (rt registryTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host == "dead.test" {
		return nil, errors.New("mirror connection refused")
	}
	clone := r.Clone(r.Context())
	u := *r.URL
	u.Scheme = rt.server.Scheme
	u.Host = rt.server.Host
	clone.URL = &u
	return http.DefaultTransport.RoundTrip(clone)
}

func registryFixture(t *testing.T, content string, wrongDigest, failAlternate bool) (string, *atomic.Int32) {
	t.Helper()
	heads := &atomic.Int32{}
	hash := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(hash[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			heads.Add(1)
			mirror := "dead"
			location := "https://dead.test/archive"
			if r.URL.Query().Get("bypass") == "dead" {
				mirror = "good"
				location = "https://good.test/archive"
			}
			advertised := digest
			if wrongDigest {
				advertised = strings.Repeat("0", 64)
			}
			w.Header().Set("Location", location)
			w.Header().Set("X-PIO-Mirror", mirror)
			w.Header().Set("X-PIO-Content-SHA256", advertised)
			w.WriteHeader(302)
			return
		}
		if r.Host == "good.test" {
			if r.Header.Get("Range") != "" || r.Header.Get("If-Range") != "" {
				t.Error("alternate inherited original transfer validators")
			}
			if failAlternate {
				w.WriteHeader(503)
				return
			}
			w.Write([]byte(content))
			return
		}
		w.Header().Set("Location", "https://dead.test/archive")
		w.WriteHeader(302)
	}))
	t.Cleanup(server.Close)
	u, _ := url.Parse(server.URL)
	old := httpClient
	httpClient = &http.Client{Transport: registryTestTransport{u}}
	t.Cleanup(func() { httpClient = old })
	return "https://dl.registry.platformio.org/download/platformio/tool/example/1.0/example.tar.gz", heads
}

func TestRegistryDownloadFallsBackWithoutSplicingPartial(t *testing.T) {
	content := "official pinned archive bytes"
	source, heads := registryFixture(t, content, false, false)
	sum := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(sum[:])
	dest := filepath.Join(t.TempDir(), "archive")
	os.WriteFile(dest+".part", []byte("old partial"), 0600)
	saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: source, ExpectedSHA256: expected, ETag: `"old"`})
	result, err := Download(context.Background(), source, dest, expected)
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != expected {
		t.Fatal(result)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != content {
		t.Fatalf("download %q", data)
	}
	state, ok := loadState(dest + ".download.json")
	if !ok || state.URL != source || !state.Complete {
		t.Fatalf("canonical state %#v", state)
	}
	if heads.Load() != 2 {
		t.Fatalf("mirror HEAD requests=%d", heads.Load())
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal("stale canonical partial after success")
	}
}

func TestRegistryDownloadRejectsAdvertisedChecksumMismatch(t *testing.T) {
	source, heads := registryFixture(t, "bytes", true, false)
	dest := filepath.Join(t.TempDir(), "archive")
	expected := strings.Repeat("1", 64)
	_, err := Download(context.Background(), source, dest, expected)
	if err == nil || !strings.Contains(err.Error(), "registry checksum") {
		t.Fatalf("expected checksum rejection, got %v", err)
	}
	if heads.Load() != 1 {
		t.Fatalf("requests %d", heads.Load())
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("promoted mismatched archive")
	}
}

func TestRegistryFallbackFailurePreservesOriginalPartial(t *testing.T) {
	source, _ := registryFixture(t, "bytes", false, true)
	dest := filepath.Join(t.TempDir(), "archive")
	old := []byte("partial from preferred mirror")
	os.WriteFile(dest+".part", old, 0600)
	saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: source, ETag: `"old"`})
	_, err := Download(context.Background(), source, dest, "")
	if err == nil {
		t.Fatal("expected failure")
	}
	data, _ := os.ReadFile(dest + ".part")
	if string(data) != string(old) {
		t.Fatalf("destroyed original partial: %q", data)
	}
	state, _ := loadState(dest + ".download.json")
	if state.ETag != `"old"` || state.URL != source {
		t.Fatalf("destroyed original identity: %#v", state)
	}
}

func TestRegistryDownloadCancelledDoesNotTryMirrors(t *testing.T) {
	source, heads := registryFixture(t, "bytes", false, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Download(ctx, source, filepath.Join(t.TempDir(), "archive"), "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if heads.Load() != 0 {
		t.Fatal("tried mirrors after cancellation")
	}
}
