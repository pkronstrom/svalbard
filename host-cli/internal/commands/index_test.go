package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
)

func TestScanZIMFiles(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0755)
	os.WriteFile(filepath.Join(zimDir, "wiki.zim"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(zimDir, "ifixit.zim"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(zimDir, "readme.txt"), []byte("not a zim"), 0644)

	files, err := ScanZIMFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	// Should be sorted
	if files[0] != "ifixit.zim" || files[1] != "wiki.zim" {
		t.Errorf("files = %v", files)
	}
}

func TestScanZIMFilesEmptyDir(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0755)

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
	os.MkdirAll(zimDir, 0755)
	os.WriteFile(filepath.Join(zimDir, "wiki.zim"), []byte("fake"), 0644)

	var buf bytes.Buffer
	if err := IndexVault(root, &buf); err != nil {
		t.Fatal(err)
	}

	// search.db should exist
	dbPath := filepath.Join(root, "data", "search.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("search.db not created: %v", err)
	}

	// Should have indexed the ZIM
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
}

func TestIndexVaultSkipsAlreadyIndexed(t *testing.T) {
	root := t.TempDir()
	zimDir := filepath.Join(root, "zim")
	os.MkdirAll(zimDir, 0755)
	os.WriteFile(filepath.Join(zimDir, "wiki.zim"), []byte("fake"), 0644)

	// Index once
	var buf1 bytes.Buffer
	if err := IndexVault(root, &buf1); err != nil {
		t.Fatal(err)
	}

	// Index again — should skip
	var buf2 bytes.Buffer
	if err := IndexVault(root, &buf2); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf2.String(), "skip") {
		t.Errorf("expected skip message, got: %s", buf2.String())
	}

	db, err := searchdb.Open(filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sc, _, err := db.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if sc != 1 {
		t.Errorf("source count = %d, should still be 1 after re-index", sc)
	}
}
