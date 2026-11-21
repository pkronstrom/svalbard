package commands

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

func TestIndexWriterLockRejectsConcurrentWriter(t *testing.T) {
	root := t.TempDir()
	release, err := acquireIndexWriterLock(root)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	defer release()

	if _, err := acquireIndexWriterLock(root); err == nil {
		t.Fatal("second writer acquired lock")
	}
}

func TestIndexWriterLockReleasesOnClose(t *testing.T) {
	root := t.TempDir()
	release, err := acquireIndexWriterLock(root)
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	release()
	second, err := acquireIndexWriterLock(root)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	second()
}

func TestSemanticWriterFailsWhileKeywordWriterHoldsLock(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	if err := os.MkdirAll(zimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, testSmallZIMPath(t), filepath.Join(zimDir, "wiki.zim"))
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- IndexVaultContext(context.Background(), root, true, io.Discard, func(p IndexProgress) {
			if p.Status == "extracting" && p.Detail == "extracting articles..." {
				close(started)
				<-release
			}
		})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("keyword writer did not acquire lock")
	}
	if err := IndexSemantic(context.Background(), root, false, io.Discard, nil); err == nil {
		t.Fatal("semantic writer acquired lock while keyword writer held it")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("keyword writer: %v", err)
	}
	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if state, err := db.GetMeta("semantic_indexed_at"); err != nil || state != "" {
		t.Fatalf("contended semantic writer mutated database: %q, %v", state, err)
	}
}
