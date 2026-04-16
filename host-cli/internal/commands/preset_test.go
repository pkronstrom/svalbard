package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestPresetListWritesKnownPresets(t *testing.T) {
	cat := catalog.NewTestCatalog(t)

	var buf bytes.Buffer
	if err := WritePresetList(&buf, cat); err != nil {
		t.Fatalf("WritePresetList: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "default-32") {
		t.Errorf("expected output to contain %q, got %q", "default-32", output)
	}
	if !strings.Contains(output, "default-128") {
		t.Errorf("expected output to contain %q, got %q", "default-128", output)
	}

	// Verify sorted order and at least 2 presets present.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %v", len(lines), lines)
	}
	// Alphabetical: default-128 before default-32
	idx128, idx32 := -1, -1
	for i, line := range lines {
		if line == "default-128" {
			idx128 = i
		}
		if line == "default-32" {
			idx32 = i
		}
	}
	if idx128 < 0 || idx32 < 0 {
		t.Fatalf("expected default-128 and default-32 in output, got %v", lines)
	}
	if idx128 > idx32 {
		t.Errorf("expected default-128 before default-32, got indices %d and %d", idx128, idx32)
	}
}

func TestCopyPresetWritesYAMLFile(t *testing.T) {
	c := catalog.NewTestCatalog(t)
	target := filepath.Join(t.TempDir(), "my-custom.yaml")

	if err := CopyPreset(c, "default-32", target); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, "default-32") || !strings.Contains(content, "sources") {
		t.Errorf("copied preset missing expected content: %q", content)
	}
}

func TestCopyPresetErrorsOnUnknown(t *testing.T) {
	c := catalog.NewTestCatalog(t)
	target := filepath.Join(t.TempDir(), "bad.yaml")

	err := CopyPreset(c, "nonexistent-preset", target)
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
}
