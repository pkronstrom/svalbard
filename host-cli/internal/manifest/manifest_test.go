package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManifestUsesDesiredAndRealizedSections(t *testing.T) {
	m := New("test-vault")

	if m.Version != 2 {
		t.Fatalf("expected Version=2, got %d", m.Version)
	}
	if m.Vault.Name != "test-vault" {
		t.Fatalf("expected vault name %q, got %q", "test-vault", m.Vault.Name)
	}
	if m.Vault.CreatedAt == "" {
		t.Fatal("expected CreatedAt to be set")
	}
	if m.Desired.Items == nil {
		t.Fatal("expected Desired.Items to be initialized (non-nil)")
	}
	if m.Desired.Presets == nil {
		t.Fatal("expected Desired.Presets to be initialized (non-nil)")
	}
	if m.Realized.Entries == nil {
		t.Fatal("expected Realized.Entries to be initialized (non-nil)")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")

	original := New("round-trip-vault")
	original.Desired.Items = []string{"wiki/golang", "wiki/rust"}
	original.Desired.Presets = []string{"dev-essentials"}
	original.Desired.Options.Region = "eu-north-1"
	original.Desired.Options.HostPlatforms = []string{"linux-arm64", "darwin-arm64"}
	original.Desired.Options.IndexStrategy = "full-text"
	original.Realized.AppliedAt = "2026-04-16T12:00:00Z"
	original.Realized.Entries = []RealizedEntry{
		{
			ID:             "abc123",
			Type:           "wiki",
			Filename:       "golang.zim",
			RelativePath:   "content/golang.zim",
			SizeBytes:      1024,
			ChecksumSHA256: "deadbeef",
			SourceStrategy: "direct-download",
		},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file exists on disk.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest file not found: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != original.Version {
		t.Fatalf("Version: got %d, want %d", loaded.Version, original.Version)
	}
	if loaded.Vault.Name != original.Vault.Name {
		t.Fatalf("Vault.Name: got %q, want %q", loaded.Vault.Name, original.Vault.Name)
	}
	if loaded.Vault.CreatedAt != original.Vault.CreatedAt {
		t.Fatalf("Vault.CreatedAt: got %q, want %q", loaded.Vault.CreatedAt, original.Vault.CreatedAt)
	}
	if len(loaded.Desired.Items) != len(original.Desired.Items) {
		t.Fatalf("Desired.Items length: got %d, want %d", len(loaded.Desired.Items), len(original.Desired.Items))
	}
	for i, item := range loaded.Desired.Items {
		if item != original.Desired.Items[i] {
			t.Fatalf("Desired.Items[%d]: got %q, want %q", i, item, original.Desired.Items[i])
		}
	}
	if loaded.Desired.Options.Region != original.Desired.Options.Region {
		t.Fatalf("Region: got %q, want %q", loaded.Desired.Options.Region, original.Desired.Options.Region)
	}
	if len(loaded.Desired.Options.HostPlatforms) != len(original.Desired.Options.HostPlatforms) {
		t.Fatalf("HostPlatforms length: got %d, want %d",
			len(loaded.Desired.Options.HostPlatforms), len(original.Desired.Options.HostPlatforms))
	}
	if loaded.Desired.Options.IndexStrategy != original.Desired.Options.IndexStrategy {
		t.Fatalf("IndexStrategy: got %q, want %q",
			loaded.Desired.Options.IndexStrategy, original.Desired.Options.IndexStrategy)
	}
	if loaded.Realized.AppliedAt != original.Realized.AppliedAt {
		t.Fatalf("AppliedAt: got %q, want %q", loaded.Realized.AppliedAt, original.Realized.AppliedAt)
	}
	if len(loaded.Realized.Entries) != 1 {
		t.Fatalf("Entries length: got %d, want 1", len(loaded.Realized.Entries))
	}
	e := loaded.Realized.Entries[0]
	if e.ID != "abc123" || e.Filename != "golang.zim" || e.SizeBytes != 1024 {
		t.Fatalf("Entry mismatch: %+v", e)
	}
}
