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

	// Verify sorted order: default-128 should come before default-32.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "default-128" {
		t.Errorf("expected first line %q, got %q", "default-128", lines[0])
	}
	if lines[1] != "default-32" {
		t.Errorf("expected second line %q, got %q", "default-32", lines[1])
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
