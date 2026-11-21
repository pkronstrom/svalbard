package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestInitSeedsManifestFromPreset(t *testing.T) {
	cat := catalog.NewTestCatalog(t)
	dir := filepath.Join(t.TempDir(), "my-vault")

	if err := InitVault(dir, "default-32", cat); err != nil {
		t.Fatalf("InitVault: %v", err)
	}

	m, err := manifest.Load(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("Load manifest: %v", err)
	}

	if m.Version != 2 {
		t.Errorf("expected version 2, got %d", m.Version)
	}
	if m.Vault.Name != "my-vault" {
		t.Errorf("expected vault name %q, got %q", "my-vault", m.Vault.Name)
	}
	if len(m.Desired.Presets) != 1 || m.Desired.Presets[0] != "default-32" {
		t.Errorf("expected presets [default-32], got %v", m.Desired.Presets)
	}

	// The default-32 preset references wikipedia-en-nopic and ifixit.
	wantItems := map[string]bool{
		"wikipedia-en-nopic": true,
		"ifixit":             true,
	}
	if len(m.Desired.Items) != len(wantItems) {
		t.Fatalf("expected %d items, got %d: %v", len(wantItems), len(m.Desired.Items), m.Desired.Items)
	}
	for _, id := range m.Desired.Items {
		if !wantItems[id] {
			t.Errorf("unexpected item %q", id)
		}
	}
}

func TestInitVaultDoesNotOverwriteExistingManifest(t *testing.T) {
	cat := catalog.NewTestCatalog(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	before := []byte("existing manifest bytes\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InitVault(dir, "default-32", cat); err == nil {
		t.Fatal("InitVault succeeded with an existing manifest")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("manifest was overwritten: %q", after)
	}
}

func TestInitVaultWithOptionsDoesNotOverwriteExistingManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	before := []byte("existing manifest bytes\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InitVaultWithOptions(dir, []string{"item"}, "preset", "eu", []string{"linux-arm64"}); err == nil {
		t.Fatal("InitVaultWithOptions succeeded with an existing manifest")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("manifest was overwritten: %q", after)
	}
}
