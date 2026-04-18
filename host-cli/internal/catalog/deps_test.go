package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDepDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dep-defaults.yaml")
	os.WriteFile(path, []byte("gguf: [llama-server]\nzim: [kiwix-serve]\n"), 0644)

	defaults, err := LoadDepDefaults(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults["gguf"]) != 1 || defaults["gguf"][0] != "llama-server" {
		t.Errorf("expected [llama-server], got %v", defaults["gguf"])
	}
	if len(defaults["zim"]) != 1 || defaults["zim"][0] != "kiwix-serve" {
		t.Errorf("expected [kiwix-serve], got %v", defaults["zim"])
	}
}

func TestLoadDepDefaultsMissing(t *testing.T) {
	defaults, err := LoadDepDefaults("/nonexistent/path.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 0 {
		t.Errorf("expected empty, got %v", defaults)
	}
}

func TestResolveDepsTypeDefault(t *testing.T) {
	c := &Catalog{recipes: map[string]Item{
		"my-model":     {Type: "gguf"},
		"llama-server": {Type: "binary"},
	}}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	selected := map[string]bool{"my-model": true}

	auto := c.ResolveDeps(selected, defaults)

	if !auto["llama-server"] {
		t.Error("llama-server should be auto-dep")
	}
	if auto["my-model"] {
		t.Error("my-model should not be auto-dep")
	}
}

func TestResolveDepsExplicitOverride(t *testing.T) {
	c := &Catalog{recipes: map[string]Item{
		"custom-model": {Type: "gguf", Deps: []string{"other-tool"}},
		"llama-server": {Type: "binary"},
		"other-tool":   {Type: "binary"},
	}}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	selected := map[string]bool{"custom-model": true}

	auto := c.ResolveDeps(selected, defaults)

	if auto["llama-server"] {
		t.Error("llama-server should NOT be pulled in (explicit deps override)")
	}
	if !auto["other-tool"] {
		t.Error("other-tool should be auto-dep")
	}
}

func TestResolveDepsExplicitEmpty(t *testing.T) {
	c := &Catalog{recipes: map[string]Item{
		"standalone": {Type: "gguf", Deps: []string{}},
	}}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	selected := map[string]bool{"standalone": true}

	auto := c.ResolveDeps(selected, defaults)

	if len(auto) != 0 {
		t.Errorf("expected no auto-deps, got %v", auto)
	}
}

func TestResolveDepsTransitive(t *testing.T) {
	c := &Catalog{recipes: map[string]Item{
		"my-model":     {Type: "gguf"},
		"llama-server": {Type: "binary", Deps: []string{"zstd"}},
		"zstd":         {Type: "binary"},
	}}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	selected := map[string]bool{"my-model": true}

	auto := c.ResolveDeps(selected, defaults)

	if !auto["llama-server"] {
		t.Error("llama-server should be auto-dep")
	}
	if !auto["zstd"] {
		t.Error("zstd should be transitive auto-dep")
	}
}

func TestResolveDepsCycle(t *testing.T) {
	c := &Catalog{recipes: map[string]Item{
		"my-model":     {Type: "gguf"},
		"llama-server": {Type: "binary", Deps: []string{"my-model"}},
	}}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	selected := map[string]bool{"my-model": true}

	auto := c.ResolveDeps(selected, defaults)
	if !auto["llama-server"] {
		t.Error("llama-server should be auto-dep despite cycle")
	}
}

func TestResolveDepsUserSelectedNotAuto(t *testing.T) {
	c := &Catalog{recipes: map[string]Item{
		"my-model":     {Type: "gguf"},
		"llama-server": {Type: "binary"},
	}}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	selected := map[string]bool{"my-model": true, "llama-server": true}

	auto := c.ResolveDeps(selected, defaults)
	if auto["llama-server"] {
		t.Error("user-selected should not be auto-dep")
	}
}

func TestDepsForItemExplicitNil(t *testing.T) {
	// Item with no deps key (nil Deps) should use type defaults
	item := Item{Type: "gguf"}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	deps := depsForItem(item, defaults)
	if len(deps) != 1 || deps[0] != "llama-server" {
		t.Errorf("expected [llama-server], got %v", deps)
	}
}

func TestDepsForItemExplicitEmpty(t *testing.T) {
	// Item with deps: [] (non-nil empty) should return empty, not type defaults
	item := Item{Type: "gguf", Deps: []string{}}
	defaults := DepDefaults{"gguf": {"llama-server"}}
	deps := depsForItem(item, defaults)
	if len(deps) != 0 {
		t.Errorf("expected empty, got %v", deps)
	}
}
