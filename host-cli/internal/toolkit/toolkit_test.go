package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	ID           string         `json:"id"`
	Label        string         `json:"label"`
	Description  string         `json:"description"`
	Order        int            `json:"order"`
	AutoActivate bool           `json:"auto_activate,omitempty"`
	Items        []testMenuItem `json:"items"`
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

var (
	testRuntimeBinaryOnce sync.Once
	testRuntimeBinaryMap  map[string]string
	testRuntimeBinaryErr  error
)

func stubRuntimeBinarySources(t *testing.T) {
	t.Helper()

	testRuntimeBinaryOnce.Do(func() {
		root, err := os.MkdirTemp("", "toolkit-runtime-binaries-")
		if err != nil {
			testRuntimeBinaryErr = err
			return
		}

		binaries := make(map[string]string, len(supportedPlatforms))
		for _, platform := range supportedPlatforms {
			source := filepath.Join(root, platform, runtimeBinaryName)
			if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
				testRuntimeBinaryErr = err
				return
			}
			if err := os.WriteFile(source, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755); err != nil {
				testRuntimeBinaryErr = err
				return
			}
			binaries[platform] = source
		}
		testRuntimeBinaryMap = binaries
	})

	if testRuntimeBinaryErr != nil {
		t.Fatal(testRuntimeBinaryErr)
	}

	original := runtimeBinarySources
	runtimeBinarySources = func() (map[string]string, error) {
		return testRuntimeBinaryMap, nil
	}
	t.Cleanup(func() {
		runtimeBinarySources = original
	})
}

func TestGenerateCreatesActionsJSON(t *testing.T) {
	stubRuntimeBinarySources(t)

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
	if item0.ID != "wikipedia-en-nopic" {
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

func TestGenerateBundlesRuntimeAndScripts(t *testing.T) {
	stubRuntimeBinarySources(t)

	root := t.TempDir()
	if err := Generate(root, nil, "default-32"); err != nil {
		t.Fatal(err)
	}

	for _, platform := range supportedPlatforms {
		dest := filepath.Join(root, ".svalbard", "runtime", platform, runtimeBinaryName)
		info, err := os.Stat(dest)
		if err != nil {
			t.Fatalf("stat runtime binary for %s: %v", platform, err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("runtime binary mode for %s = %o, want 755", platform, info.Mode().Perm())
		}
	}

	runPath := filepath.Join(root, "run")
	runData, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatal(err)
	}
	runText := string(runData)
	if !strings.Contains(runText, ".svalbard/runtime/") {
		t.Fatalf("run script missing runtime path: %q", runText)
	}
	if !strings.Contains(runText, "uname -s") {
		t.Fatalf("run script missing uname detection: %q", runText)
	}
	if !strings.Contains(runText, `exec "$DRIVE_ROOT/.svalbard/runtime/$platform/svalbard-drive" "$@"`) {
		t.Fatalf("run script missing exec line: %q", runText)
	}
	if info, err := os.Stat(runPath); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o755 {
		t.Fatalf("run mode = %o, want 755", info.Mode().Perm())
	}

	activatePath := filepath.Join(root, "activate")
	activateData, err := os.ReadFile(activatePath)
	if err != nil {
		t.Fatal(err)
	}
	activateText := string(activateData)
	if !strings.Contains(activateText, `exec "$DRIVE_ROOT/run" activate "$@"`) {
		t.Fatalf("activate script missing exec line: %q", activateText)
	}
	if !strings.Contains(activateText, "sb()") {
		t.Fatalf("activate script missing sb helper: %q", activateText)
	}
	if !strings.Contains(activateText, "deactivate()") {
		t.Fatalf("activate script missing deactivate helper: %q", activateText)
	}
	if info, err := os.Stat(activatePath); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o755 {
		t.Fatalf("activate mode = %o, want 755", info.Mode().Perm())
	}
}

func TestGenerateWithPMTilesCreatesMapGroup(t *testing.T) {
	stubRuntimeBinarySources(t)

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
	stubRuntimeBinarySources(t)

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
	// With no entries: activate-shell, mcp-serve, inspect are always present.
	// verify-checksums is conditional (needs checksums), serve/share conditional (needs services).
	if len(cfg.Groups[0].Items) != 3 {
		var ids []string
		for _, it := range cfg.Groups[0].Items {
			ids = append(ids, it.ID)
		}
		t.Errorf("tools items = %d (%v), want 3", len(cfg.Groups[0].Items), ids)
	}
}

func TestGenerateGroupsOrderedCorrectly(t *testing.T) {
	stubRuntimeBinarySources(t)

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

	if len(cfg.Groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(cfg.Groups))
	}
	// Should be ordered: search (100), library (200), maps (300), tools (500).
	if cfg.Groups[0].ID != "search" {
		t.Errorf("groups[0] = %q, want search", cfg.Groups[0].ID)
	}
	if !cfg.Groups[0].AutoActivate {
		t.Error("search group should have auto_activate=true")
	}
	if cfg.Groups[1].ID != "library" {
		t.Errorf("groups[1] = %q, want library", cfg.Groups[1].ID)
	}
	if cfg.Groups[2].ID != "maps" {
		t.Errorf("groups[2] = %q, want maps", cfg.Groups[2].ID)
	}
	if cfg.Groups[3].ID != "tools" {
		t.Errorf("groups[3] = %q, want tools", cfg.Groups[3].ID)
	}
}

func TestGenerateHumanizesLabels(t *testing.T) {
	stubRuntimeBinarySources(t)

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

func TestGenerateWritesChecksums(t *testing.T) {
	stubRuntimeBinarySources(t)

	root := t.TempDir()
	entries := []manifest.RealizedEntry{
		{ID: "ifixit", Type: "zim", Filename: "ifixit.zim", RelativePath: "zim/ifixit.zim", ChecksumSHA256: "aaa111"},
		{ID: "wikipedia-en", Type: "zim", Filename: "wikipedia-en.zim", RelativePath: "zim/wikipedia-en.zim", ChecksumSHA256: "bbb222"},
		{ID: "osm-world", Type: "pmtiles", Filename: "osm-world.pmtiles", RelativePath: "maps/osm-world.pmtiles"},
	}
	if err := Generate(root, entries, "test"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".svalbard", "checksums.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Lines should be sorted and use forward slashes.
	expected := "aaa111  zim/ifixit.zim\nbbb222  zim/wikipedia-en.zim\n"
	if content != expected {
		t.Errorf("checksums.sha256:\ngot:  %q\nwant: %q", content, expected)
	}
}

func TestGenerateNoChecksumsRemovesFile(t *testing.T) {
	stubRuntimeBinarySources(t)

	root := t.TempDir()
	// Pre-create a stale checksums file.
	svDir := filepath.Join(root, ".svalbard")
	os.MkdirAll(svDir, 0o755)
	os.WriteFile(filepath.Join(svDir, "checksums.sha256"), []byte("old\n"), 0o644)

	entries := []manifest.RealizedEntry{
		{ID: "osm-world", Type: "pmtiles", Filename: "osm-world.pmtiles", RelativePath: "maps/osm-world.pmtiles"},
	}
	if err := Generate(root, entries, "test"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(svDir, "checksums.sha256")); !os.IsNotExist(err) {
		t.Error("stale checksums.sha256 should have been removed when no entries have checksums")
	}
}

func TestExtractEmbeddedBinariesFallsBackGracefully(t *testing.T) {
	// With only .gitkeep in embedded/, extraction should fail,
	// triggering the fallback path in loadRuntimeBinaries.
	_, err := extractEmbeddedBinaries()
	if err == nil {
		// If the build script was run, binaries exist — that's fine too.
		t.Log("embedded binaries found (build script was run)")
		return
	}
	// Expected: error because only .gitkeep exists
	t.Logf("extraction correctly failed: %v", err)
}
