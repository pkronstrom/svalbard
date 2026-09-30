package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
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

func TestPresetListMarksUserPresets(t *testing.T) {
	cat := catalog.NewTestCatalog(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mine.yaml"), []byte("extends: [default-32]\nsources: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cat.LoadUserPresets(dir); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := WritePresetList(&buf, cat); err != nil {
		t.Fatal(err)
	}
	want := "mine  (user: " + filepath.Join(dir, "mine.yaml") + ")"
	if !strings.Contains(buf.String(), want) {
		t.Errorf("expected %q in output, got %q", want, buf.String())
	}
}

func TestCopyPresetWritesExtendsStubThatLoadsBack(t *testing.T) {
	c := catalog.NewTestCatalog(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "my-custom.yaml")

	if err := CopyPreset(c, "default-32", target, false); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	for _, want := range []string{"# Custom svalbard preset", "name: my-custom", "extends:\n  - default-32", "sources: []"} {
		if !strings.Contains(content, want) {
			t.Errorf("stub missing %q:\n%s", want, content)
		}
	}

	if err := c.LoadUserPresets(dir); err != nil {
		t.Fatalf("stub does not load back: %v", err)
	}
	mine, err := c.ResolvePreset("my-custom")
	if err != nil {
		t.Fatal(err)
	}
	orig, _ := c.ResolvePreset("default-32")
	if !reflect.DeepEqual(mine.Sources, orig.Sources) {
		t.Errorf("stub resolves to %v, want %v", mine.Sources, orig.Sources)
	}
}

func TestCopyPresetFlattenListsResolvedSources(t *testing.T) {
	c := catalog.NewTestCatalog(t)
	target := filepath.Join(t.TempDir(), "flat.yaml")

	if err := CopyPreset(c, "default-128", target, true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if strings.Contains(content, "\nextends:") || !strings.Contains(content, "- wikipedia-en-nopic") || !strings.Contains(content, "name: flat") {
		t.Errorf("flattened preset unexpected:\n%s", content)
	}
}

func TestCopyPresetRefusesNameCollisionAndOverwrite(t *testing.T) {
	c := catalog.NewTestCatalog(t)
	dir := t.TempDir()

	if err := CopyPreset(c, "default-32", filepath.Join(dir, "default-128.yaml"), false); err == nil {
		t.Error("expected error when target name matches an existing preset")
	}
	existing := filepath.Join(dir, "taken.yaml")
	if err := os.WriteFile(existing, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyPreset(c, "default-32", existing, false); err == nil {
		t.Error("expected error when target file exists")
	}
	if raw, _ := os.ReadFile(existing); string(raw) != "keep" {
		t.Error("existing file was overwritten")
	}
}

func TestCopyPresetErrorsOnUnknown(t *testing.T) {
	c := catalog.NewTestCatalog(t)
	target := filepath.Join(t.TempDir(), "bad.yaml")

	err := CopyPreset(c, "nonexistent-preset", target, false)
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
}

func TestPresetTargetPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	cases := map[string]string{
		"my-256":       filepath.Join("/cfg", "svalbard", "presets", "my-256.yaml"),
		"my-256.yaml":  "my-256.yaml",
		"./out/my-256": "./out/my-256",
		"/abs/x.yaml":  "/abs/x.yaml",
	}
	for in, want := range cases {
		got, err := PresetTargetPath(in)
		if err != nil || got != want {
			t.Errorf("PresetTargetPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
