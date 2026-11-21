package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

func TestApplyVaultCheckpointsCompletedPeerOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("completed peer"))
	}))
	defer server.Close()

	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"complete.yaml": &fstest.MapFile{Data: []byte("id: complete\ntype: zim\nstrategy: download\nurl: " + server.URL + "/complete.zim\n")},
		"broken.yaml":   &fstest.MapFile{Data: []byte("id: broken\ntype: zim\nstrategy: download\nurl: http://127.0.0.1:1/broken.zim\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := manifest.New("test")
	m.Desired.Items = []string{"complete", "broken"}
	if err := manifest.Save(filepath.Join(root, "manifest.yaml"), m); err != nil {
		t.Fatal(err)
	}

	if err := ApplyVault(context.Background(), root, cat); err == nil {
		t.Fatal("ApplyVault succeeded despite broken peer")
	}
	reloaded, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Realized.Entries) != 1 || reloaded.Realized.Entries[0].ID != "complete" {
		t.Fatalf("realized entries = %+v, want completed peer", reloaded.Realized.Entries)
	}
	if reloaded.Realized.AppliedAt != "" {
		t.Errorf("AppliedAt = %q, want empty for incomplete apply", reloaded.Realized.AppliedAt)
	}
}

func TestApplyVaultResumesOnlyPendingRecipeAfterCancellation(t *testing.T) {
	fast := []byte("already complete")
	slow := []byte(strings.Repeat("slow-transfer-", 8192))
	fastSHA := checksum(fast)
	slowSHA := checksum(slow)
	started := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	requests := map[string]int{}
	var slowRange string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/fast.zim":
			_, _ = w.Write(fast)
		case "/slow.zim":
			if got := r.Header.Get("Range"); got != "" {
				mu.Lock()
				slowRange = got
				mu.Unlock()
				start, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(got, "bytes="), "-"), 10, 64)
				if err != nil || start < 0 || start >= int64(len(slow)) {
					http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
					return
				}
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(slow)-1, len(slow)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(slow[start:])
				return
			}
			_, _ = w.Write(slow[:4096])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			once.Do(func() { close(started) })
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cat, err := catalog.LoadFromFS(fstest.MapFS{
		"fast.yaml": &fstest.MapFile{Data: []byte("id: fast\ntype: zim\nstrategy: download\nurl: " + server.URL + "/fast.zim\nsha256: " + fastSHA + "\n")},
		"slow.yaml": &fstest.MapFile{Data: []byte("id: slow\ntype: zim\nstrategy: download\nurl: " + server.URL + "/slow.zim\nsha256: " + slowSHA + "\n")},
	}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := InitVaultWithOptions(root, []string{"fast", "slow"}, "chosen", "fi", []string{"linux-arm64"}); err != nil {
		t.Fatal(err)
	}
	before, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	slowReceived := make(chan struct{})
	var receivedOnce sync.Once
	fastDone := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	applyDone := make(chan error, 1)
	go func() {
		applyDone <- ApplyVault(ctx, root, cat, func(event apply.ProgressEvent) {
			if event.ID == "slow" && event.Downloaded > 0 {
				receivedOnce.Do(func() { close(slowReceived) })
			}
			if event.ID == "fast" && event.Status == "done" {
				select {
				case <-fastDone:
				default:
					close(fastDone)
				}
			}
		})
	}()
	<-started
	<-fastDone
	<-slowReceived
	during, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil || len(during.Realized.Entries) != 1 || during.Realized.Entries[0].ID != "fast" {
		t.Fatalf("peer was not saved while other transfer blocked: %+v error=%v", during.Realized, err)
	}
	cancel()
	// The first call returns only after the completed peer is checkpointed.
	if err = <-applyDone; err == nil {
		t.Fatal("cancelled apply unexpectedly succeeded")
	}

	reloaded, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := planner.Build(reloaded).ToDownload; len(got) != 1 || got[0] != "slow" {
		t.Fatalf("pending plan = %v, want [slow]", got)
	}
	if !reflect.DeepEqual(reloaded.Desired, before.Desired) || reloaded.Vault != before.Vault {
		t.Fatalf("initial selections/metadata changed: %+v", reloaded)
	}
	if reloaded.Desired.Presets[0] != "chosen" || reloaded.Desired.Options.Region != "fi" || reloaded.Desired.Options.HostPlatforms[0] != "linux-arm64" {
		t.Fatalf("desired selection changed: %+v", reloaded.Desired)
	}
	if reloaded.Realized.AppliedAt != "" {
		t.Fatalf("AppliedAt = %q after incomplete apply", reloaded.Realized.AppliedAt)
	}
	partInfo, err := os.Stat(filepath.Join(root, "zim", "slow.zim.part"))
	if err != nil {
		t.Fatal(err)
	}

	if err := ApplyVault(context.Background(), root, cat); err != nil {
		t.Fatalf("resume apply: %v", err)
	}
	mu.Lock()
	gotFast, gotSlow, gotRange := requests["/fast.zim"], requests["/slow.zim"], slowRange
	mu.Unlock()
	if gotFast != 1 || gotSlow != 2 || gotRange != fmt.Sprintf("bytes=%d-", partInfo.Size()) {
		t.Fatalf("requests fast=%d slow=%d range=%q, want 1, 2, bytes=%d-", gotFast, gotSlow, gotRange, partInfo.Size())
	}
	final, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if final.Realized.AppliedAt == "" || len(planner.Build(final).ToDownload) != 0 {
		t.Fatalf("resume did not complete manifest: %+v", final.Realized)
	}
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestApplyVaultRetainsApplyAndSaveErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "manifest.yaml")
	if err := InitVaultWithOptions(root, []string{"missing"}, "chosen", "fi", nil); err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	err = ApplyVault(context.Background(), root, cat, func(ev apply.ProgressEvent) {
		if ev.ID == "missing" && ev.Status == "failed" {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
			if err := os.Mkdir(path, 0755); err != nil {
				t.Error(err)
			}
		}
	})
	var pathErr *os.LinkError
	if err == nil || !strings.Contains(err.Error(), `recipe "missing" not found`) || !errors.As(err, &pathErr) {
		t.Fatalf("apply/save causes not retained: %v", err)
	}
}

func TestApplyVaultFinalSaveDoesNotRealizeRejectedCheckpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("complete")) }))
	defer server.Close()
	cat, err := catalog.LoadFromFS(fstest.MapFS{"item.yaml": &fstest.MapFile{Data: []byte("id: item\ntype: zim\nurl: " + server.URL + "/item.zim\n")}}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "manifest.yaml")
	backup := path + ".backup"
	if err := InitVaultWithOptions(root, []string{"item"}, "chosen", "fi", nil); err != nil {
		t.Fatal(err)
	}
	var blocked, restored bool
	err = ApplyVault(context.Background(), root, cat, func(ev apply.ProgressEvent) {
		if ev.ID == "item" && ev.Status == "active" && !blocked {
			blocked = true
			if err := os.Rename(path, backup); err != nil {
				t.Error(err)
			}
			if err := os.Mkdir(path, 0755); err != nil {
				t.Error(err)
			}
		}
		if ev.ID == "item" && ev.Status == "failed" {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
			if err := os.Rename(backup, path); err != nil {
				t.Error(err)
			}
			restored = true
		}
	})
	if err == nil || !strings.Contains(err.Error(), "checkpointing") || !restored {
		t.Fatalf("expected rejected checkpoint and restored final save: %v", err)
	}
	saved, err := manifest.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Realized.Entries) != 0 || saved.Realized.AppliedAt != "" {
		t.Fatalf("final save accepted rejected checkpoint: %+v", saved.Realized)
	}
}

func TestEmptyApplyRepairsRuntimeAfterAllRecipesWereCheckpointed(t *testing.T) {
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		_, _ = w.Write([]byte("pmtiles fixture"))
	}))
	defer server.Close()
	cat, err := catalog.LoadFromFS(fstest.MapFS{"map.yaml": &fstest.MapFile{Data: []byte("id: map\ntype: pmtiles\nurl: " + server.URL + "/map.pmtiles\n")}}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := InitVaultWithOptions(root, []string{"map"}, "chosen", "fi", nil); err != nil {
		t.Fatal(err)
	}
	if err := ApplyVault(context.Background(), root, cat); err == nil || !strings.Contains(err.Error(), "generating map viewer") {
		t.Fatalf("expected late runtime generation failure: %v", err)
	}
	saved, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planner.Build(saved).ToDownload) != 0 || saved.Realized.AppliedAt != "" {
		t.Fatalf("unexpected post-checkpoint state: %+v", saved.Realized)
	}
	vendor := filepath.Join(root, "apps", "maplibre-vendor", "vendor")
	if err := os.MkdirAll(vendor, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"maplibre-gl.js", "maplibre-gl.css", "pmtiles.js"} {
		if err := os.WriteFile(filepath.Join(vendor, name), []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := ApplyVault(context.Background(), root, cat); err != nil {
		t.Fatal(err)
	}
	final, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if final.Realized.AppliedAt == "" || downloads.Load() != 1 {
		t.Fatalf("empty-plan repair did not finish without redownload: %+v count=%d", final.Realized, downloads.Load())
	}
	if info, err := os.Stat(filepath.Join(root, "apps", "map", "index.html")); err != nil || info.Size() == 0 {
		t.Fatalf("viewer not repaired: %v", err)
	}
}
