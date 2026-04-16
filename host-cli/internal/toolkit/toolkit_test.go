package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// Local structs matching the drive-runtime RuntimeConfig JSON format,
// so we can verify the generated file without importing drive-runtime.
type testRuntimeConfig struct {
	Version int             `json:"version"`
	Preset  string          `json:"preset"`
	Groups  []testMenuGroup `json:"groups"`
}

type testMenuGroup struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Order       int            `json:"order"`
	Items       []testMenuItem `json:"items"`
}

type testMenuItem struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Order       int            `json:"order"`
	Action      testActionSpec `json:"action"`
}

type testActionSpec struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

type testBuiltinConfig struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args,omitempty"`
}

func TestGenerateCreatesActionsJSON(t *testing.T) {
	root := t.TempDir()
	entries := []manifest.RealizedEntry{
		{ID: "wikipedia-en-nopic", Type: "zim", Filename: "wikipedia-en-nopic.zim", RelativePath: "zim/wikipedia-en-nopic.zim"},
		{ID: "ifixit", Type: "zim", Filename: "ifixit.zim", RelativePath: "zim/ifixit.zim"},
	}
	if err := Generate(root, entries, "default-32"); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	if err != nil {
		t.Fatal(err)
	}

	var cfg testRuntimeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Version != 2 {
		t.Errorf("version = %d", cfg.Version)
	}
	if cfg.Preset != "default-32" {
		t.Errorf("preset = %q", cfg.Preset)
	}

	// Should have library group with 2 items.
	var library *testMenuGroup
	for i := range cfg.Groups {
		if cfg.Groups[i].ID == "library" {
			library = &cfg.Groups[i]
		}
	}
	if library == nil {
		t.Fatal("missing library group")
	}
	if len(library.Items) != 2 {
		t.Errorf("library items = %d, want 2", len(library.Items))
	}

	// Verify first library item (ordered by order field).
	item0 := library.Items[0]
	if item0.ID != "browse-wikipedia-en-nopic" {
		t.Errorf("item[0].ID = %q", item0.ID)
	}
	if item0.Action.Type != "builtin" {
		t.Errorf("item[0].action.type = %q", item0.Action.Type)
	}
	var bc testBuiltinConfig
	if err := json.Unmarshal(item0.Action.Config, &bc); err != nil {
		t.Fatalf("unmarshal action config: %v", err)
	}
	if bc.Name != "browse" {
		t.Errorf("action config name = %q, want browse", bc.Name)
	}
	if bc.Args["zim"] != "wikipedia-en-nopic.zim" {
		t.Errorf("action config args[zim] = %q", bc.Args["zim"])
	}

	// Should have tools group.
	hasTools := false
	for _, g := range cfg.Groups {
		if g.ID == "tools" {
			hasTools = true
		}
	}
	if !hasTools {
		t.Error("missing tools group")
	}
}

func TestGenerateWithPMTilesCreatesMapGroup(t *testing.T) {
	root := t.TempDir()
	entries := []manifest.RealizedEntry{
		{ID: "osm-finland", Type: "pmtiles", Filename: "osm-finland.pmtiles", RelativePath: "maps/osm-finland.pmtiles"},
	}
	if err := Generate(root, entries, "test"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	var cfg testRuntimeConfig
	json.Unmarshal(raw, &cfg)

	hasMaps := false
	for _, g := range cfg.Groups {
		if g.ID == "maps" {
			hasMaps = true
		}
	}
	if !hasMaps {
		t.Error("missing maps group for pmtiles entries")
	}
}

func TestGenerateToolsAlwaysPresent(t *testing.T) {
	root := t.TempDir()
	// No entries at all — tools group should still be present.
	if err := Generate(root, nil, "empty"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	var cfg testRuntimeConfig
	json.Unmarshal(raw, &cfg)

	if len(cfg.Groups) != 1 {
		t.Fatalf("expected 1 group (tools only), got %d", len(cfg.Groups))
	}
	if cfg.Groups[0].ID != "tools" {
		t.Errorf("expected tools group, got %q", cfg.Groups[0].ID)
	}
	if len(cfg.Groups[0].Items) != 2 {
		t.Errorf("tools items = %d, want 2", len(cfg.Groups[0].Items))
	}
}

func TestGenerateGroupsOrderedCorrectly(t *testing.T) {
	root := t.TempDir()
	entries := []manifest.RealizedEntry{
		{ID: "wikipedia-en", Type: "zim", Filename: "wikipedia-en.zim", RelativePath: "zim/wikipedia-en.zim"},
		{ID: "osm-world", Type: "pmtiles", Filename: "osm-world.pmtiles", RelativePath: "maps/osm-world.pmtiles"},
	}
	if err := Generate(root, entries, "full"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	var cfg testRuntimeConfig
	json.Unmarshal(raw, &cfg)

	if len(cfg.Groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(cfg.Groups))
	}
	// Should be ordered: library (200), maps (300), tools (900).
	if cfg.Groups[0].ID != "library" {
		t.Errorf("groups[0] = %q, want library", cfg.Groups[0].ID)
	}
	if cfg.Groups[1].ID != "maps" {
		t.Errorf("groups[1] = %q, want maps", cfg.Groups[1].ID)
	}
	if cfg.Groups[2].ID != "tools" {
		t.Errorf("groups[2] = %q, want tools", cfg.Groups[2].ID)
	}
}

func TestGenerateHumanizesLabels(t *testing.T) {
	root := t.TempDir()
	entries := []manifest.RealizedEntry{
		{ID: "wikipedia-en-nopic", Type: "zim", Filename: "wikipedia-en-nopic.zim", RelativePath: "zim/wikipedia-en-nopic.zim"},
	}
	if err := Generate(root, entries, "test"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	var cfg testRuntimeConfig
	json.Unmarshal(raw, &cfg)

	var library *testMenuGroup
	for i := range cfg.Groups {
		if cfg.Groups[i].ID == "library" {
			library = &cfg.Groups[i]
		}
	}
	if library == nil {
		t.Fatal("missing library group")
	}
	if library.Items[0].Label != "Wikipedia En Nopic" {
		t.Errorf("label = %q, want %q", library.Items[0].Label, "Wikipedia En Nopic")
	}
}

func TestTypeDirsExported(t *testing.T) {
	// Verify TypeDirs contains expected mappings.
	expected := map[string]string{
		"zim": "zim", "pmtiles": "maps", "pdf": "books", "epub": "books",
		"gguf": "models", "binary": "bin", "app": "apps",
	}
	for k, v := range expected {
		got, ok := TypeDirs[k]
		if !ok {
			t.Errorf("TypeDirs missing key %q", k)
		} else if got != v {
			t.Errorf("TypeDirs[%q] = %q, want %q", k, got, v)
		}
	}
}
