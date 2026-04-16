package vault_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-tui/internal/vault"
)

func TestResolveFindsManifestInCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := vault.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks so macOS /private/var/... matches /var/...
	want, _ := filepath.EvalSymlinks(dir)
	got, _ = filepath.EvalSymlinks(got)

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveFindsManifestInParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "manifest.yaml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := vault.Resolve(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want, _ := filepath.EvalSymlinks(parent)
	got, _ = filepath.EvalSymlinks(got)

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveReturnsErrorWhenNoVault(t *testing.T) {
	dir := t.TempDir()

	_, err := vault.Resolve(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, vault.ErrNoVault) {
		t.Errorf("expected ErrNoVault, got %v", err)
	}
}

func TestResolveReturnsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := vault.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}
