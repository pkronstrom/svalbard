package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportLocalFileCopiesIntoWorkspaceLibrary(t *testing.T) {
	workspace := t.TempDir()

	// Create a source file outside the workspace.
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "my-notes.pdf")
	if err := os.WriteFile(srcPath, []byte("pdf-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := ImportLocalFile(workspace, srcPath, "")
	if err != nil {
		t.Fatalf("ImportLocalFile returned error: %v", err)
	}

	if !strings.HasPrefix(id, "local:") {
		t.Errorf("expected id to start with %q, got %q", "local:", id)
	}

	destPath := filepath.Join(workspace, "library", "my-notes.pdf")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatalf("expected file at %s but it does not exist", destPath)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "pdf-content" {
		t.Errorf("expected file content %q, got %q", "pdf-content", string(data))
	}
}

func TestImportLocalFileUsesCustomName(t *testing.T) {
	workspace := t.TempDir()

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "original.txt")
	if err := os.WriteFile(srcPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := ImportLocalFile(workspace, srcPath, "my-doc")
	if err != nil {
		t.Fatalf("ImportLocalFile returned error: %v", err)
	}

	if id != "local:my-doc" {
		t.Errorf("expected id %q, got %q", "local:my-doc", id)
	}
}
