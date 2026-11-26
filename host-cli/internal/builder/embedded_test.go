package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestBuildEmbeddedCreatesDirectoryEntry(t *testing.T) {
	root := t.TempDir()
	recipe := catalog.Item{ID: "skills-core", Type: "skills", Strategy: "build", Build: &catalog.BuildSpec{Family: "embedded"}}
	fn, ok := Dispatch(recipe)
	if !ok {
		t.Fatal("no builder for family embedded")
	}
	entries, err := fn(root, recipe, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v, want one", entries)
	}
	e := entries[0]
	if e.ID != "skills-core" || e.Type != "skills" || filepath.ToSlash(e.RelativePath) != "skills/skills-core" || e.SourceStrategy != "build" {
		t.Fatalf("entry = %+v", e)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "skills-core", ".svalbard-embedded")); err != nil {
		t.Fatalf("marker: %v", err)
	}
	// A second build over the same directory keeps it.
	if _, err := fn(root, recipe, nil, Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestBuildEmbeddedRefusesTypeWithoutDirectory(t *testing.T) {
	recipe := catalog.Item{ID: "x", Type: "nonsense", Strategy: "build", Build: &catalog.BuildSpec{Family: "embedded"}}
	if _, err := buildEmbedded(t.TempDir(), recipe, nil, Options{}); err == nil {
		t.Fatal("built a recipe whose type has no directory")
	}
}
