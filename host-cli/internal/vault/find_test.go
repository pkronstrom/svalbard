package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootWalksUpToManifest(t *testing.T) {
	// Create:  root/manifest.yaml
	//          root/a/b/c/  (nested working directory)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.yaml"), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot("", nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks so macOS /private/var/... matches /var/...
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("got %q, want %q", gotResolved, wantResolved)
	}
}

func TestFindRootReturnsErrorWhenNoManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := FindRoot("", dir)
	if err == nil {
		t.Fatal("expected error when no manifest.yaml exists")
	}
}

func TestFindRootUsesExplicitPath(t *testing.T) {
	dir := t.TempDir()

	got, err := FindRoot(dir, "/some/other/cwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks for macOS temp dir comparison.
	wantResolved, _ := filepath.EvalSymlinks(dir)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("got %q, want %q", gotResolved, wantResolved)
	}
}
