package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
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
	Aliases     []string       `json:"aliases,omitempty"`
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
	if err := Generate(root, entries, "default-32", GenerateOpts{Menus: map[string]catalog.MenuSpec{
		"ifixit": {Aliases: []string{"repair", "fix"}},
	}}); err != nil {
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
	var ifixit *testMenuItem
	for i := range library.Items {
		if library.Items[i].ID == "ifixit" {
			ifixit = &library.Items[i]
			break
		}
	}
	if ifixit == nil {
		t.Fatal("missing ifixit item")
	}
	if got, want := ifixit.Aliases, []string{"repair", "fix"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ifixit aliases = %v, want %v", got, want)
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

func TestGenerateWritesPlatformIOInventory(t *testing.T) {
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	entries := []manifest.RealizedEntry{
		{ID: "platformio-espressif32", Filename: "espressif32-7.1.3.tar.gz", RelativePath: "tools/platformio/packages/espressif32-7.1.3.tar.gz", ChecksumSHA256: "shared"},
		{ID: "tool-cmake", Filename: "tool-cmake.tar.gz", RelativePath: "tools/platformio/packages/macos-arm64/tool-cmake.tar.gz", ChecksumSHA256: "target"},
		{ID: "platformio-core", RelativePath: "runtime/python/macos-arm64/tools/platformio-core"},
		{ID: "espidf-python", RelativePath: "runtime/python/macos-arm64/tools/espidf-python"},
	}
	for _, e := range entries {
		if e.ID == "platformio-core" || e.ID == "espidf-python" {
			if err := os.MkdirAll(filepath.Join(root, e.RelativePath), 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, e.RelativePath)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, e.RelativePath), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	recipes := map[string]catalog.Item{
		"platformio-espressif32": {URL: "https://dl.registry.platformio.org/download/platformio/platform/espressif32/7.1.3/espressif32-7.1.3.tar.gz"},
		"tool-cmake": {Platforms: map[string]string{
			"macos-arm64":  "https://dl.registry.platformio.org/download/platformio/tool/tool-cmake/3.30.2/tool-cmake.tar.gz",
			"macos-x86_64": "https://dl.registry.platformio.org/download/platformio/tool/tool-cmake/3.30.2/tool-cmake-x86.tar.gz",
			"linux-arm64":  "https://dl.registry.platformio.org/download/platformio/tool/tool-cmake/3.30.2/tool-cmake-linux-arm.tar.gz",
			"linux-x86_64": "https://dl.registry.platformio.org/download/platformio/tool/tool-cmake/3.30.2/tool-cmake-linux-x86.tar.gz",
		}},
	}
	if err := Generate(root, entries, "esp32-dev", GenerateOpts{PlatformIORecipes: recipes, PlatformIOPlatforms: []string{"macos-arm64"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".svalbard", "platformio.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory platformIOInventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.Version != 1 || len(inventory.Packages) != 2 {
		t.Fatalf("inventory = %+v", inventory)
	}
	if inventory.Packages[1].Owner != "platformio" || inventory.Packages[1].Platform != "macos-arm64" {
		t.Fatalf("target record = %+v", inventory.Packages[1])
	}
	if err := os.RemoveAll(filepath.Join(root, "runtime", "python", "macos-arm64", "tools", "espidf-python")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(root, entries, "esp32-dev", GenerateOpts{PlatformIORecipes: recipes, PlatformIOPlatforms: []string{"macos-arm64"}}); err == nil || !strings.Contains(err.Error(), "Python tree") {
		t.Fatalf("Generate after deleted Python tree = %v", err)
	}
}

func TestGenerateSearchCapabilityHasSearchAlias(t *testing.T) {
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	entries := []manifest.RealizedEntry{{ID: "wiki", Type: "zim", Filename: "wiki.zim", RelativePath: "zim/wiki.zim"}}
	if err := Generate(root, entries, "default", GenerateOpts{}); err != nil {
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
	for _, group := range cfg.Groups {
		for _, item := range group.Items {
			if item.ID == "search-all" {
				if !reflect.DeepEqual(item.Aliases, []string{"search"}) {
					t.Fatalf("search aliases = %v", item.Aliases)
				}
				return
			}
		}
	}
	t.Fatal("search capability missing")
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
	// No entries at all — the tools and settings groups should still be present.
	if err := Generate(root, nil, "empty"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	var cfg testRuntimeConfig
	json.Unmarshal(raw, &cfg)

	if len(cfg.Groups) != 2 {
		t.Fatalf("expected 2 groups (tools and settings), got %d", len(cfg.Groups))
	}
	if cfg.Groups[0].ID != "tools" {
		t.Errorf("expected tools group, got %q", cfg.Groups[0].ID)
	}
	if cfg.Groups[1].ID != "settings" {
		t.Errorf("expected settings group, got %q", cfg.Groups[1].ID)
	}
	var ids []string
	for _, it := range cfg.Groups[0].Items {
		ids = append(ids, it.ID)
	}
	want := []string{"mcp-serve", "drive-frontpage", "inspect-drive", "activate-shell"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Errorf("tools items = %v, want %v", ids, want)
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

	if len(cfg.Groups) != 5 {
		t.Fatalf("expected 5 groups, got %d", len(cfg.Groups))
	}
	// Should be ordered: search (100), library (200), maps (300), tools (500),
	// settings (1000).
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
	if cfg.Groups[4].ID != "settings" {
		t.Errorf("groups[4] = %q, want settings", cfg.Groups[4].ID)
	}
}

func TestGenerateEndsTheMenuWithSettings(t *testing.T) {
	stubRuntimeBinarySources(t)

	root := t.TempDir()
	// A recipe menu group the registry does not know sorts at 999; Settings
	// must still come after it.
	entries := []manifest.RealizedEntry{{ID: "radio-tool", Type: "binary", Filename: "radio-tool"}}
	if err := Generate(root, entries, "test", GenerateOpts{Menus: map[string]catalog.MenuSpec{
		"radio-tool": {Group: "radio", Label: "Radio tool"},
	}}); err != nil {
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
	last := cfg.Groups[len(cfg.Groups)-1]
	if last.ID != "settings" || last.Label != "Settings" || !last.AutoActivate || len(last.Items) != 1 {
		t.Fatalf("last group = %+v, want the auto-activated Settings group with one item", last)
	}
	var action testBuiltinConfig
	if err := json.Unmarshal(last.Items[0].Action.Config, &action); err != nil {
		t.Fatal(err)
	}
	if last.Items[0].Action.Type != "builtin" || action.Name != "settings" {
		t.Errorf("action = %s %+v, want builtin settings", last.Items[0].Action.Type, action)
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

func TestDraftModelsAreInstalledSeparatelyWithoutChatActions(t *testing.T) {
	if got := TypeDirs["gguf-mtp"]; got != "models/draft" {
		t.Fatalf("draft directory = %q, want models/draft", got)
	}
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	entries := []manifest.RealizedEntry{{ID: "qwen-27b-mtp", Type: "gguf-mtp", Filename: "mtp-Qwen3.8-27B-Q4_0.gguf"}}
	if err := Generate(root, entries, "ai/mtp"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg testRuntimeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, group := range cfg.Groups {
		for _, item := range group.Items {
			if item.ID == "qwen-27b-mtp" || item.ID == "chat-default" {
				t.Fatalf("drafter must not appear as a selectable chat model: %s", item.ID)
			}
		}
	}
}

func TestGenerateOffersOpenWebUIOnlyWhenInstalled(t *testing.T) {
	stubRuntimeBinarySources(t)

	find := func(entries []manifest.RealizedEntry) (testMenuItem, string, bool) {
		t.Helper()
		root := t.TempDir()
		if err := Generate(root, entries, "test"); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cfg testRuntimeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		for _, group := range cfg.Groups {
			for _, item := range group.Items {
				if item.ID == "open-webui" {
					return item, group.ID, true
				}
			}
		}
		return testMenuItem{}, "", false
	}

	model := manifest.RealizedEntry{ID: "qwen-4b", Type: "gguf", Filename: "qwen.gguf"}
	if _, _, ok := find([]manifest.RealizedEntry{model, {ID: "sqlmap", Type: "python-package"}}); ok {
		t.Fatal("Open WebUI offered without its recipe installed")
	}

	// One realized entry per platform must still give one menu entry.
	item, group, ok := find([]manifest.RealizedEntry{
		model,
		{ID: "open-webui", Type: "python-package", RelativePath: "runtime/python/macos-arm64/tools/open-webui"},
		{ID: "open-webui", Type: "python-package", RelativePath: "runtime/python/linux-x86_64/tools/open-webui"},
	})
	if !ok {
		t.Fatal("Open WebUI missing from the menu")
	}
	if group != "local-ai" {
		t.Errorf("group = %q, want local-ai", group)
	}
	// The licence's branding clause forbids any other name.
	if item.Label != "Open WebUI" {
		t.Errorf("label = %q, want Open WebUI", item.Label)
	}
	if !slices.Contains(item.Aliases, "open-webui") {
		t.Errorf("aliases = %v, want open-webui", item.Aliases)
	}
	var action testBuiltinConfig
	if err := json.Unmarshal(item.Action.Config, &action); err != nil {
		t.Fatal(err)
	}
	if item.Action.Type != "builtin" || action.Name != "open-webui" || len(action.Args) != 0 {
		t.Errorf("action = %s %+v, want builtin open-webui without args", item.Action.Type, action)
	}
}

func TestGenerateOffersTheWikiOnceWhenInstalled(t *testing.T) {
	stubRuntimeBinarySources(t)

	find := func(entries []manifest.RealizedEntry) (items []testMenuItem, groups []string) {
		t.Helper()
		root := t.TempDir()
		if err := Generate(root, entries, "test"); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, ".svalbard", "actions.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cfg testRuntimeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		for _, group := range cfg.Groups {
			for _, item := range group.Items {
				if item.ID == "silverbullet" || strings.Contains(strings.ToLower(item.Label), "wiki") || strings.Contains(strings.ToLower(item.Label), "silverbullet") {
					items = append(items, item)
					groups = append(groups, group.ID)
				}
			}
		}
		return items, groups
	}

	archive := manifest.RealizedEntry{ID: "ifixit", Type: "zim", Filename: "ifixit.zim"}
	if items, _ := find([]manifest.RealizedEntry{archive, {ID: "dufs", Type: "binary", RelativePath: "bin/macos-arm64/dufs.tar.gz"}}); len(items) != 0 {
		t.Fatalf("the wiki is offered without its recipe installed: %+v", items)
	}

	// One realized entry per platform must still give one menu entry, and
	// none of the generated per-platform tool entries.
	items, groups := find([]manifest.RealizedEntry{
		archive,
		{ID: "silverbullet", Type: "binary", RelativePath: "bin/macos-arm64/silverbullet-server-darwin-aarch64.zip"},
		{ID: "silverbullet", Type: "binary", RelativePath: "bin/macos-x86_64/silverbullet-server-darwin-x86_64.zip"},
		{ID: "silverbullet", Type: "binary", RelativePath: "bin/linux-arm64/silverbullet-server-linux-aarch64.zip"},
		{ID: "silverbullet", Type: "binary", RelativePath: "bin/linux-x86_64/silverbullet-server-linux-x86_64.zip"},
	})
	if len(items) != 1 {
		t.Fatalf("wiki menu entries = %+v, want exactly one", items)
	}
	item := items[0]
	if groups[0] != "apps" || item.Label != "Wiki" {
		t.Errorf("entry = %q in group %q, want Wiki in apps", item.Label, groups[0])
	}
	// `./run wiki` starts it.
	if !slices.Contains(item.Aliases, "wiki") {
		t.Errorf("aliases = %v, want wiki", item.Aliases)
	}
	var action testBuiltinConfig
	if err := json.Unmarshal(item.Action.Config, &action); err != nil {
		t.Fatal(err)
	}
	if item.Action.Type != "builtin" || action.Name != "wiki" || len(action.Args) != 0 {
		t.Errorf("action = %s %+v, want builtin wiki without args", item.Action.Type, action)
	}

	// A drive with nothing but the wiki still gets the entry.
	if items, _ := find([]manifest.RealizedEntry{{ID: "silverbullet", Type: "binary"}}); len(items) != 1 {
		t.Errorf("wiki menu entries on a wiki-only drive = %+v, want one", items)
	}
}

func TestGenerateWritesRecipeMetadataForRealizedEntries(t *testing.T) {
	stubRuntimeBinarySources(t)

	root := t.TempDir()
	entries := []manifest.RealizedEntry{
		{ID: "ifixit", Type: "zim", Filename: "ifixit.zim", RelativePath: "zim/ifixit.zim"},
		{ID: "osm-finland", Type: "pmtiles", Filename: "osm-finland.pmtiles", RelativePath: "maps/osm-finland.pmtiles"},
	}
	recipes := map[string]catalog.Item{
		"ifixit":      {ID: "ifixit", Type: "zim", Description: "Repair guides", Tags: []string{"repair"}},
		"osm-finland": {ID: "osm-finland", Type: "pmtiles", Description: "Finland basemap", Viewer: &catalog.ViewerSpec{Name: "OSM Finland", Category: "basemap"}},
		"unrealized":  {ID: "unrealized", Type: "zim", Description: "Not on the drive"},
	}
	if err := Generate(root, entries, "test", GenerateOpts{Recipes: recipes}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".svalbard", "recipes.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The drive-runtime MCP reader expects an object keyed by recipe ID.
	var got map[string]struct {
		ID          string            `json:"id"`
		Type        string            `json:"type"`
		Description string            `json:"description"`
		Tags        []string          `json:"tags"`
		Viewer      map[string]string `json:"viewer"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("recipes.json is not an object keyed by ID: %v\n%s", err, data)
	}
	if len(got) != 2 {
		t.Fatalf("recipes.json has %d entries, want only the 2 realized: %s", len(got), data)
	}
	if r := got["ifixit"]; r.ID != "ifixit" || r.Type != "zim" || r.Description != "Repair guides" || len(r.Tags) != 1 || r.Tags[0] != "repair" {
		t.Errorf("ifixit = %+v", r)
	}
	if r := got["osm-finland"]; r.Viewer["name"] != "OSM Finland" || r.Viewer["category"] != "basemap" {
		t.Errorf("osm-finland viewer = %+v", r.Viewer)
	}
}

func TestGenerateWithoutRecipeMetadataRemovesStaleFile(t *testing.T) {
	stubRuntimeBinarySources(t)

	root := t.TempDir()
	stale := filepath.Join(root, ".svalbard", "recipes.json")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte(`{"old":{"id":"old"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(root, nil, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale recipes.json still present: %v", err)
	}
}

func TestInstallRuntimeBinariesRemovesItsScratchDirectory(t *testing.T) {
	scratch, err := newRuntimeScratch("toolkit-scratch-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(scratch) })
	kept := t.TempDir() // a directory this package did not create, as a stub's is

	sources := make(map[string]string, len(supportedPlatforms))
	for i, platform := range supportedPlatforms {
		base := scratch
		if i == 0 {
			base = kept
		}
		source := filepath.Join(base, platform, runtimeBinaryName)
		if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(source, []byte("binary "+platform), 0o755); err != nil {
			t.Fatal(err)
		}
		sources[platform] = source
	}
	original := runtimeBinarySources
	runtimeBinarySources = func() (map[string]string, error) { return sources, nil }
	t.Cleanup(func() { runtimeBinarySources = original })

	root := t.TempDir()
	if err := installRuntimeBinaries(root); err != nil {
		t.Fatal(err)
	}
	for _, platform := range supportedPlatforms {
		if _, err := os.Stat(filepath.Join(root, svalbardDir, "runtime", platform, runtimeBinaryName)); err != nil {
			t.Errorf("runtime binary for %s not installed: %v", platform, err)
		}
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Errorf("scratch directory %s was left behind (err %v)", scratch, err)
	}
	if _, err := os.Stat(filepath.Join(kept, supportedPlatforms[0], runtimeBinaryName)); err != nil {
		t.Errorf("a source directory the package did not create was removed: %v", err)
	}
}
