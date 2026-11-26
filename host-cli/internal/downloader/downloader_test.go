package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestDownloadIdentifiesItself(t *testing.T) {
	var agent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get("User-Agent")
		w.Write([]byte("content"))
	}))
	defer ts.Close()

	if _, err := Download(context.Background(), ts.URL+"/file.txt", filepath.Join(t.TempDir(), "file.txt"), ""); err != nil {
		t.Fatal(err)
	}
	// Hosts that refuse the default Go client agent must not see it.
	if agent != "svalbard" {
		t.Fatalf("User-Agent = %q, want svalbard", agent)
	}
}

func TestDownloadResumesPartialFile(t *testing.T) {
	content := "hello world test content"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
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
	// Write partial content with the transfer identity recorded.
	os.WriteFile(dest+".part", []byte("hello"), 0644)
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL + "/file.txt", ETag: `"v1"`}); err != nil {
		t.Fatal(err)
	}

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

func TestDownloadResumesChecksumPinnedPartialWithoutValidator(t *testing.T) {
	content := []byte("hello world")
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=5-" {
			t.Fatalf("Range = %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", "bytes 5-10/11")
		w.WriteHeader(http.StatusPartialContent)
		w.Write(content[5:])
	}))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest+".part", content[:5], 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, ExpectedSHA256: expected}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), ts.URL, dest, expected); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadResumesLastModifiedWithIdentityEncoding(t *testing.T) {
	content := []byte("hello world")
	modified := "Mon, 02 Jan 2006 15:04:05 GMT"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=5-" || r.Header.Get("If-Range") != modified || r.Header.Get("Accept-Encoding") != "identity" {
			t.Fatalf("unexpected resume headers: %#v", r.Header)
		}
		w.Header().Set("Last-Modified", modified)
		w.Header().Set("Content-Range", "bytes 5-10/11")
		w.WriteHeader(http.StatusPartialContent)
		w.Write(content[5:])
	}))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest+".part", content[:5], 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, LastModified: modified}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), ts.URL, dest, ""); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadIgnoredRangeRestartsPartial(t *testing.T) {
	content := []byte("new complete body")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(content) }))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest+".part", []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, ETag: `"v1"`}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), ts.URL, dest, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(content) {
		t.Fatalf("final = %q", got)
	}
}

func TestDownloadUnsafe416MakesOneFreshRequest(t *testing.T) {
	content := []byte("fresh body")
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Write(content)
	}))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest+".part", []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, ETag: `"v1"`}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), ts.URL, dest, ""); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want 2", requests)
	}
}

func TestDownloadSafe416PromotesChecksumValidatedPartial(t *testing.T) {
	content := []byte("complete")
	h := sha256.Sum256(content)
	checksum := hex.EncodeToString(h[:])
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes */8")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
	}))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest+".part", content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, ExpectedSHA256: checksum}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), ts.URL, dest, checksum); err != nil {
		t.Fatal(err)
	}
}

func TestComputeSHA256ContextStopsWhenCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large")
	if err := os.WriteFile(path, make([]byte, hashChunkSz), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := computeSHA256Context(ctx, path); err == nil {
		t.Fatal("hash succeeded after cancellation")
	}
}

func TestDownloadCancellationKeepsBytesOutsideFinalName(t *testing.T) {
	started := make(chan struct{})
	written := make(chan struct{})
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("prefix"))
		w.(http.Flusher).Flush()
		close(started)
		<-release
		w.Write([]byte("suffix"))
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "file.txt")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Download(ctx, ts.URL, dest, "", func(downloaded, _ int64) {
			if downloaded > 0 {
				select {
				case <-written:
				default:
					close(written)
				}
			}
		})
		done <- err
	}()
	<-started
	<-written
	cancel()
	close(release)
	if err := <-done; err == nil {
		t.Fatal("Download succeeded after cancellation")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("final path exists after cancelled download: %v", err)
	}
	if info, err := os.Stat(dest + ".part"); err != nil || info.Size() == 0 {
		t.Fatalf("partial file missing after cancellation: info=%v err=%v", info, err)
	}
}

func TestDownloadRejectsChangedETagBeforeAppendingPartial(t *testing.T) {
	content := []byte("hello world")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=5-" {
			t.Fatalf("Range = %q", r.Header.Get("Range"))
		}
		w.Header().Set("ETag", `"new"`)
		w.Header().Set("Content-Range", "bytes 5-10/11")
		w.WriteHeader(http.StatusPartialContent)
		w.Write(content[5:])
	}))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest+".part", content[:5], 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, ETag: `"old"`}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), ts.URL, dest, ""); err == nil {
		t.Fatal("changed ETag partial response succeeded")
	}
	if got, _ := os.ReadFile(dest + ".part"); string(got) != string(content[:5]) {
		t.Fatalf("partial changed to %q", got)
	}
}

func TestDownloadRejectsMalformedRangeBeforeAppendingPartial(t *testing.T) {
	content := []byte("hello world")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Range", "bytes 5-10/11 trailing")
		w.WriteHeader(http.StatusPartialContent)
		w.Write(content[5:])
	}))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest+".part", content[:5], 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, ETag: `"v1"`}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), ts.URL, dest, ""); err == nil {
		t.Fatal("malformed partial response succeeded")
	}
	if got, _ := os.ReadFile(dest + ".part"); string(got) != string(content[:5]) {
		t.Fatalf("partial changed to %q", got)
	}
}

func TestDownloadReusesCompletedSidecarAfterPromotion(t *testing.T) {
	content := []byte("completed artifact")
	h := sha256.Sum256(content)
	checksum := hex.EncodeToString(h[:])
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write(content)
	}))
	defer ts.Close()
	dest := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(dest, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: ts.URL, Size: int64(len(content)), SHA256: checksum, Complete: true}); err != nil {
		t.Fatal(err)
	}
	result, err := Download(context.Background(), ts.URL, dest, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cached || requests != 0 {
		t.Fatalf("result=%+v requests=%d, want completed reuse", result, requests)
	}
}

func TestDownloadPromotesCompletedSidecarBeforeRename(t *testing.T) {
	content := []byte("completed partial")
	h := sha256.Sum256(content)
	sum := hex.EncodeToString(h[:])
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest+".part", content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: "http://example.test/x", Size: int64(len(content)), SHA256: sum, Complete: true}); err != nil {
		t.Fatal(err)
	}
	result, err := Download(context.Background(), "http://example.test/x", dest, "")
	if err != nil || !result.Cached {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatalf("partial remains: %v", err)
	}
}

func TestReuseFromCacheWithSHA256RejectsMismatch(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cached.gguf")
	if err := os.WriteFile(cache, []byte("wrong"), 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out.gguf")
	if err := os.WriteFile(dest, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	_, used, err := ReuseFromCacheWithSHA256(context.Background(), cache, "http://127.0.0.1:1/x", dest, "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil || used {
		t.Fatalf("reuse = used=%v err=%v, want unsuitable cache", used, err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "old" {
		t.Fatalf("final changed to %q", got)
	}
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

func TestDownloadRejectsInconsistentRangeLengths(t *testing.T) {
	for _, tc := range []struct {
		name, contentRange, body string
		chunked                  bool
	}{
		{"wrong offset", "bytes 4-10/11", " world", false},
		{"wrong declared length", "bytes 5-10/11", " worl", false},
		{"incomplete total", "bytes 5-7/11", " wo", false},
		{"chunked body exceeds range", "bytes 5-6/11", " world", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"same"`)
				w.Header().Set("Content-Range", tc.contentRange)
				w.WriteHeader(http.StatusPartialContent)
				if tc.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(dest+".part", []byte("hello"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: server.URL, ETag: `"same"`, Size: 11}); err != nil {
				t.Fatal(err)
			}
			if _, err := Download(context.Background(), server.URL, dest, ""); err == nil {
				t.Fatal("inconsistent range promoted as complete")
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("final exists: %v", err)
			}
		})
	}
}

func TestInterruptedFreshTransferRetainsItsCurrentIdentity(t *testing.T) {
	for _, oldPartial := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-partial=%v", oldPartial), func(t *testing.T) {
			content := strings.Repeat("b", 2*chunkSize)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"new"`)
				if requests.Add(1) == 1 {
					w.Header().Set("Content-Length", fmt.Sprint(len(content)))
					_, _ = w.Write([]byte(content[:chunkSize]))
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				if r.Header.Get("If-Range") != `"new"` {
					t.Errorf("resume validator=%q", r.Header.Get("If-Range"))
				}
				var offset int
				if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-", &offset); err != nil {
					t.Error(err)
					return
				}
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, len(content)-1, len(content)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte(content[offset:]))
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "file")
			if oldPartial {
				if err := os.WriteFile(dest+".part", []byte("old-prefix"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: server.URL, ETag: `"old"`, Size: int64(len(content))}); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err := Download(ctx, server.URL, dest, "", func(n, total int64) {
				if n > 0 {
					cancel()
				}
			})
			if err == nil {
				t.Fatal("cancelled transfer succeeded")
			}
			state, ok := loadState(dest + ".download.json")
			if !ok || state.ETag != `"new"` || state.Complete {
				t.Fatalf("invalid recovery identity: %+v valid=%v", state, ok)
			}
			if _, err := Download(context.Background(), server.URL, dest, ""); err != nil {
				t.Fatal(err)
			}
			if got, err := os.ReadFile(dest); err != nil || string(got) != content {
				t.Fatalf("mixed or incomplete bytes; error=%v length=%d", err, len(got))
			}
		})
	}
}

func TestCompletedRecoverySurvivesCancellationAndRenameFailure(t *testing.T) {
	for _, failure := range []string{"cancel", "rename"} {
		t.Run(failure, func(t *testing.T) {
			content := []byte("verified completed transfer")
			hash := sha256.Sum256(content)
			dest := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(dest+".part", content, 0644); err != nil {
				t.Fatal(err)
			}
			if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: "http://example.test/file", Size: int64(len(content)), SHA256: hex.EncodeToString(hash[:]), Complete: true}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancel" {
				cancel()
			} else if err := os.Mkdir(dest, 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := Download(ctx, "http://example.test/file", dest, ""); err == nil {
				t.Fatal("recovery failure ignored")
			}
			if got, err := os.ReadFile(dest + ".part"); err != nil || string(got) != string(content) {
				t.Fatalf("validated partial lost: %v", err)
			}
			if state, ok := loadState(dest + ".download.json"); !ok || !state.Complete {
				t.Fatal("completed recovery state lost")
			}
			if failure == "rename" {
				if err := os.Remove(dest); err != nil {
					t.Fatal(err)
				}
			}
			if result, err := Download(context.Background(), "http://example.test/file", dest, ""); err != nil || !result.Cached {
				t.Fatalf("offline recovery failed: result=%+v error=%v", result, err)
			}
		})
	}
}

func TestOverlongBrokenChunkedResponseCannotBeRecoveredAsComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"same"`)
		w.Header().Set("Content-Range", "bytes 5-6/11")
		w.WriteHeader(http.StatusPartialContent)
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte(" world"))
		w.(http.Flusher).Flush()
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest+".part", []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: server.URL, ETag: `"same"`, Size: 11}); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), server.URL, dest, ""); err == nil {
		t.Fatal("broken response accepted")
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatalf("rejected overlong bytes remain recoverable: %v", err)
	}
}

func TestWeakOrMalformedETagCannotAuthorizeResume(t *testing.T) {
	for _, etag := range []string{`W/"weak"`, `not-quoted`, `"broken"tag"`} {
		t.Run(etag, func(t *testing.T) {
			var requests atomic.Int32
			observedRange := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", etag)
				if requests.Add(1) == 1 {
					_, _ = w.Write([]byte("prefix"))
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				observedRange <- r.Header.Get("Range")
				_, _ = w.Write([]byte("replacement"))
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "file")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if _, err := Download(ctx, server.URL, dest, "", func(n, _ int64) {
				if n > 0 {
					cancel()
				}
			}); err == nil {
				t.Fatal("cancelled transfer accepted")
			}
			if _, err := Download(context.Background(), server.URL, dest, ""); err != nil {
				t.Fatal(err)
			}
			if got := <-observedRange; got != "" {
				t.Fatalf("untrusted ETag authorized range %q", got)
			}
		})
	}
}

type validationTransport func(*http.Request) (*http.Response, error)

func (f validationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCancelledSafe416ValidationRetainsRecoverableBytes(t *testing.T) {
	content := []byte("complete but not promoted")
	dest := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dest+".part", content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveState(dest+".download.json", transferState{Version: sidecarVersion, URL: "http://fixture.test/file", ETag: `"same"`, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests atomic.Int32
	original := httpClient
	httpClient = &http.Client{Transport: validationTransport(func(r *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			cancel()
		}
		return &http.Response{StatusCode: 416, Header: http.Header{"Content-Range": []string{fmt.Sprintf("bytes */%d", len(content))}, "Etag": []string{`"same"`}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	t.Cleanup(func() { httpClient = original })
	if _, err := Download(ctx, "http://fixture.test/file", dest, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("validation cancellation lost: %v", err)
	}
	if got, err := os.ReadFile(dest + ".part"); err != nil || string(got) != string(content) {
		t.Fatalf("cancelled validation destroyed partial: %v", err)
	}
	if _, ok := loadState(dest + ".download.json"); !ok {
		t.Fatal("cancelled validation destroyed identity")
	}
	if _, err := Download(context.Background(), "http://fixture.test/file", dest, ""); err != nil {
		t.Fatalf("later promotion failed: %v", err)
	}
}
