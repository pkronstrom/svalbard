package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestImportAndMaybeAddWithoutAdd(t *testing.T) {
	workspace := t.TempDir()
	vaultRoot := t.TempDir()

	// Create a manifest in the vault root.
	m := manifest.New("test")
	mPath := filepath.Join(vaultRoot, "manifest.yaml")
	if err := manifest.Save(mPath, m); err != nil {
		t.Fatal(err)
	}

	// Create a source file.
	srcPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := ImportAndMaybeAdd(workspace, srcPath, "", false, vaultRoot)
	if err != nil {
		t.Fatalf("ImportAndMaybeAdd returned error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	// Manifest should NOT have been modified.
	reloaded, err := manifest.Load(mPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Desired.Items) != 0 {
		t.Errorf("expected 0 items in manifest, got %d: %v", len(reloaded.Desired.Items), reloaded.Desired.Items)
	}
}

func TestImportAndMaybeAddWithAdd(t *testing.T) {
	workspace := t.TempDir()
	vaultRoot := t.TempDir()

	// Create a manifest in the vault root.
	m := manifest.New("test")
	mPath := filepath.Join(vaultRoot, "manifest.yaml")
	if err := manifest.Save(mPath, m); err != nil {
		t.Fatal(err)
	}

	// Create a source file.
	srcPath := filepath.Join(t.TempDir(), "guide.pdf")
	if err := os.WriteFile(srcPath, []byte("pdf-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := ImportAndMaybeAdd(workspace, srcPath, "", true, vaultRoot)
	if err != nil {
		t.Fatalf("ImportAndMaybeAdd returned error: %v", err)
	}
	if id != "local:guide" {
		t.Errorf("expected id %q, got %q", "local:guide", id)
	}

	// Manifest SHOULD contain the new id.
	reloaded, err := manifest.Load(mPath)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range reloaded.Desired.Items {
		if item == "local:guide" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected manifest items to contain %q, got %v", "local:guide", reloaded.Desired.Items)
	}

	// The import must also be realized, otherwise plan keeps it in ToDownload
	// and every apply fails with "recipe not found".
	if len(reloaded.Realized.Entries) != 1 {
		t.Fatalf("expected 1 realized entry, got %d", len(reloaded.Realized.Entries))
	}
	e := reloaded.Realized.Entries[0]
	if e.ID != "local:guide" || e.Type != "pdf" || e.RelativePath != filepath.Join("library", "guide.pdf") {
		t.Errorf("unexpected realized entry: %+v", e)
	}
	if e.SizeBytes != int64(len("pdf-data")) || e.SourceStrategy != "local" {
		t.Errorf("unexpected realized entry metadata: %+v", e)
	}
}

func TestImportAndAddZIMUsesSearchableArchiveDirectory(t *testing.T) {
	root := t.TempDir()
	if err := manifest.Save(filepath.Join(root, "manifest.yaml"), manifest.New("test")); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "videos.zim")
	if err := os.WriteFile(source, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportAndMaybeAdd(root, source, "", true, root); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Realized.Entries) != 1 || m.Realized.Entries[0].RelativePath != "zim/videos.zim" {
		t.Fatalf("realized import = %+v", m.Realized.Entries)
	}
	if data, err := os.ReadFile(filepath.Join(root, "zim", "videos.zim")); err != nil || string(data) != "fixture" {
		t.Fatalf("installed archive = %q, %v", data, err)
	}
}
