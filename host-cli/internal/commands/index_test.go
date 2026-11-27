package commands

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

func testSmallZIMPath(t *testing.T) string {
	t.Helper()

	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}

	path := filepath.Join(strings.TrimSpace(string(out)), "github.com", "stazelabs", "gozim@v0.1.0", "testdata", "small.zim")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat test zim %s: %v", path, err)
	}
	return path
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func TestScanZIMFiles(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	os.WriteFile(filepath.Join(zimDir, "wiki.zim"), []byte("fake"), 0o644)
	os.WriteFile(filepath.Join(zimDir, "ifixit.zim"), []byte("fake"), 0o644)
	os.WriteFile(filepath.Join(zimDir, "readme.txt"), []byte("not a zim"), 0o644)
	// What macOS leaves beside an archive on an exFAT stick.
	os.WriteFile(filepath.Join(zimDir, "._wiki.zim"), []byte("\x00\x05\x16\x07"), 0o644)

	files, err := ScanZIMFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0] != "ifixit.zim" || files[1] != "wiki.zim" {
		t.Errorf("files = %v", files)
	}
}

func TestScanZIMFilesEmptyDir(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)

	files, err := ScanZIMFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
}

func TestScanZIMFilesMissingDir(t *testing.T) {
	root := t.TempDir()

	_, err := ScanZIMFiles(root)
	if err == nil {
		t.Fatal("expected error for missing zim directory")
	}
}

func TestIndexVaultCreatesSearchDB(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	copyFile(t, testSmallZIMPath(t), filepath.Join(zimDir, "wiki.zim"))

	var buf bytes.Buffer
	if err := IndexVault(root, false, &buf, nil); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(root, "data", "search.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("search.db not created: %v", err)
	}

	db, err := searchdb.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	fns, err := db.IndexedFilenames()
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 1 || fns[0] != "wiki.zim" {
		t.Errorf("indexed filenames = %v", fns)
	}

	sc, ac, err := db.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if sc != 1 {
		t.Fatalf("source count = %d, want 1", sc)
	}
	if ac != 1 {
		t.Fatalf("article count = %d, want 1", ac)
	}

	results, err := db.Search("test", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected indexed article search result")
	}
	if results[0].Title != "Test ZIM file" {
		t.Fatalf("search title = %q, want %q", results[0].Title, "Test ZIM file")
	}
	if strings.Contains(results[0].Body, "<html") {
		t.Fatalf("search body should be plain text, got %q", results[0].Body)
	}
}

func TestIndexVaultProcessesSmallerZIMsFirst(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(testSmallZIMPath(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string][]byte{
		"a-large.zim": append(bytes.Clone(fixture), make([]byte, 4096)...),
		"z-small.zim": fixture,
	} {
		if err := os.WriteFile(filepath.Join(root, "zim", name), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var order []string
	if err := IndexVault(root, false, io.Discard, func(p IndexProgress) {
		if p.Status == "done" {
			order = append(order, p.File)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "z-small.zim,a-large.zim" {
		t.Fatalf("completed in order %q", got)
	}
}

func TestIndexVaultSkipsAlreadyIndexed(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	copyFile(t, testSmallZIMPath(t), filepath.Join(zimDir, "wiki.zim"))

	var buf1 bytes.Buffer
	if err := IndexVault(root, false, &buf1, nil); err != nil {
		t.Fatal(err)
	}

	var skipped bool
	if err := IndexVault(root, false, io.Discard, func(p IndexProgress) {
		if p.Status == "skip" {
			skipped = true
		}
	}); err != nil {
		t.Fatal(err)
	}

	if !skipped {
		t.Error("expected skip callback for already-indexed file")
	}

	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sc, ac, err := db.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if sc != 1 {
		t.Errorf("source count = %d, should still be 1 after re-index", sc)
	}
	if ac != 1 {
		t.Errorf("article count = %d, should still be 1 after re-index", ac)
	}
}

func TestIndexVaultRefreshesWhenSourceMtimeChanges(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	path := filepath.Join(zimDir, "wiki.zim")
	copyFile(t, testSmallZIMPath(t), path)
	if err := IndexVault(root, false, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := time.Unix(0, info.ModTime().UnixNano()+time.Second.Nanoseconds())
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatal(err)
	}
	var done bool
	if err := IndexVault(root, false, io.Discard, func(p IndexProgress) { done = done || p.Status == "done" }); err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("changed source was not refreshed")
	}
}

func TestIndexVaultPrunesRemovedSource(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	path := filepath.Join(zimDir, "wiki.zim")
	copyFile(t, testSmallZIMPath(t), path)
	if err := IndexVault(root, false, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := IndexVault(root, false, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if hits, err := db.Search("test", 10); err != nil || len(hits) != 0 {
		t.Fatalf("removed-source hits = %#v, %v", hits, err)
	}
}

func TestIndexVaultFailurePreservesExistingSource(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	path := filepath.Join(zimDir, "wiki.zim")
	copyFile(t, testSmallZIMPath(t), path)
	if err := IndexVault(root, false, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := IndexVault(root, false, io.Discard, nil); err == nil {
		t.Fatal("expected corrupt replacement error")
	}
	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if hits, err := db.Search("test", 10); err != nil || len(hits) != 1 {
		t.Fatalf("preserved hits = %#v, %v", hits, err)
	}
}

func TestIndexVaultContextCancellationLeavesNoCurrentSource(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	copyFile(t, testSmallZIMPath(t), filepath.Join(zimDir, "wiki.zim"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := IndexVaultContext(ctx, root, false, io.Discard, nil); err != context.Canceled {
		t.Fatalf("IndexVaultContext error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "search.db")); !os.IsNotExist(err) {
		t.Fatalf("cancelled indexing created search database: %v", err)
	}
}

func TestIndexVaultContextCancellationBeforeCommitPreservesSource(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0o755)
	copyFile(t, testSmallZIMPath(t), filepath.Join(zimDir, "wiki.zim"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := IndexVaultContext(ctx, root, false, io.Discard, func(p IndexProgress) {
		if p.Status == "extracting" && strings.HasPrefix(p.Detail, "inserting") {
			cancel()
		}
	})
	if err != context.Canceled {
		t.Fatalf("IndexVaultContext error = %v, want context.Canceled", err)
	}
	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if sources, err := db.IndexedFilenames(); err != nil || len(sources) != 0 {
		t.Fatalf("cancelled replacement sources = %v, %v", sources, err)
	}
}
