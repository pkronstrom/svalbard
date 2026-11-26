package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// resolvedWithDeps is what selecting a preset installs: its items and their
// dependencies.
func resolvedWithDeps(t *testing.T, cat *Catalog, preset string) map[string]bool {
	t.Helper()
	p, err := cat.ResolvePreset(preset)
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := LoadDepDefaultsAuto()
	if err != nil {
		t.Fatal(err)
	}
	selected := make(map[string]bool)
	for _, item := range p.Items {
		selected[item.ID] = true
	}
	for id := range cat.ResolveDeps(selected, defaults) {
		selected[id] = true
	}
	return selected
}

func TestHarnessesPackIncludesSkills(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	pack, err := cat.ResolvePreset("ai/harnesses")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range pack.Sources {
		if src == "py-docs" {
			t.Error("ai/harnesses names py-docs; it should arrive through skills-core's deps so deselecting skills-core drops it")
		}
	}
	got := resolvedWithDeps(t, cat, "default-256")
	for _, id := range []string{"skills-core", "py-docs"} {
		if !got[id] {
			t.Errorf("default-256 does not install %s", id)
		}
	}
}

func TestSkillsCoreDeselectable(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	preset := "name: no-skills\nextends: [default-256]\nsources:\n- -skills-core\n"
	if err := os.WriteFile(filepath.Join(dir, "no-skills.yaml"), []byte(preset), 0o644); err != nil {
		t.Fatal(err)
	}
	if skipped := cat.LoadUserPresets(dir); len(skipped) != 0 {
		t.Fatalf("user preset skipped: %+v", skipped)
	}
	got := resolvedWithDeps(t, cat, "no-skills")
	for _, id := range []string{"skills-core", "py-docs"} {
		if got[id] {
			t.Errorf("no-skills still installs %s", id)
		}
	}
	if !got["pi"] || !got["llama-server"] {
		t.Error("deselecting skills-core dropped the harnesses")
	}
}
