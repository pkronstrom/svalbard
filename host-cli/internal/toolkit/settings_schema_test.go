package toolkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func readSettingsSchema(t *testing.T, root string) settingsSchema {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".svalbard", "settings.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema settingsSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("settings.schema.json: %v\n%s", err, data)
	}
	return schema
}

func TestGenerateNamesTheWikiSettingsAfterItsMenuEntry(t *testing.T) {
	stubRuntimeBinarySources(t)

	recipes := map[string]catalog.Item{
		"silverbullet": {ID: "silverbullet", Type: "binary", Settings: []catalog.SettingSpec{
			{Name: "open_browser", Type: "bool", Default: true, Label: "Open browser on start", Description: "Open the browser when the wiki is ready."},
			{Name: "fs_watch", Type: "choice", Default: "auto", Choices: []string{"auto", "poll"}, Label: "Notice changed pages", Description: "How the wiki sees pages changed on the drive."},
		}},
	}
	// One realized entry per platform must still give one group.
	entries := []manifest.RealizedEntry{
		{ID: "silverbullet", Type: "binary", RelativePath: "bin/macos-arm64/silverbullet-server-darwin-aarch64.zip"},
		{ID: "silverbullet", Type: "binary", RelativePath: "bin/linux-x86_64/silverbullet-server-linux-x86_64.zip"},
	}
	root := t.TempDir()
	if err := Generate(root, entries, "test", GenerateOpts{Recipes: recipes}); err != nil {
		t.Fatal(err)
	}
	schema := readSettingsSchema(t, root)
	// The user starts "Wiki" from the menu; the settings are found under
	// the same name, not under the server's.
	if len(schema.Groups) != 1 || schema.Groups[0].Owner != "silverbullet" || schema.Groups[0].Label != "Wiki" {
		t.Fatalf("groups = %+v, want one group Wiki owned by silverbullet", schema.Groups)
	}
	if settings := schema.Groups[0].Settings; len(settings) != 2 || settings[1].Name != "fs_watch" || len(settings[1].Choices) != 2 {
		t.Errorf("settings = %+v, want open_browser and fs_watch with its two choices", settings)
	}
}

func TestGenerateWritesSettingsSchemaForInstalledRecipes(t *testing.T) {
	stubRuntimeBinarySources(t)

	one, ten := 1.0, 10.0
	recipes := map[string]catalog.Item{
		"open-webui": {ID: "open-webui", Type: "python-package", Settings: []catalog.SettingSpec{
			{Name: "open_browser", Type: "bool", Default: true, Label: "Open browser on start", Description: "Open the browser when Open WebUI is ready."},
		}},
		"radio-tool": {ID: "radio-tool", Type: "binary", Menu: &catalog.MenuSpec{Label: "Radio tool"}, Settings: []catalog.SettingSpec{
			{Name: "band", Type: "choice", Default: "vhf", Label: "Band", Description: "Band to scan.", Choices: []string{"vhf", "uhf"}},
			{Name: "gain", Type: "number", Default: 5, Label: "Gain", Description: "Receiver gain.", Min: &one, Max: &ten},
			{Name: "token", Type: "secret", Default: "", Label: "Token", Description: "Access token."},
		}},
		"plain-tool":    {ID: "plain-tool", Type: "binary", Settings: []catalog.SettingSpec{{Name: "note", Type: "text", Default: "", Label: "Note", Description: "A note."}}},
		"not-installed": {ID: "not-installed", Type: "binary", Settings: []catalog.SettingSpec{{Name: "x", Type: "bool", Default: false, Label: "X", Description: "X."}}},
		"wikipedia":     {ID: "wikipedia", Type: "zim"},
	}
	entries := []manifest.RealizedEntry{
		{ID: "wikipedia", Type: "zim", Filename: "wikipedia.zim"},
		{ID: "radio-tool", Type: "binary", Filename: "radio-tool"},
		{ID: "plain-tool", Type: "binary", Filename: "plain-tool"},
		// One realized entry per platform must still give one group.
		{ID: "open-webui", Type: "python-package", RelativePath: "runtime/python/macos-arm64/tools/open-webui"},
		{ID: "open-webui", Type: "python-package", RelativePath: "runtime/python/linux-x86_64/tools/open-webui"},
	}
	root := t.TempDir()
	if err := Generate(root, entries, "test", GenerateOpts{Recipes: recipes}); err != nil {
		t.Fatal(err)
	}

	schema := readSettingsSchema(t, root)
	if schema.Version != 1 {
		t.Errorf("version = %d, want 1", schema.Version)
	}
	// Group label: the recipe's menu label, the fixed built-in entry's label
	// for Open WebUI, else the recipe ID. Not-installed recipes are left out.
	var got [][2]string
	for _, group := range schema.Groups {
		got = append(got, [2]string{group.Owner, group.Label})
	}
	want := [][2]string{{"open-webui", "Open WebUI"}, {"plain-tool", "plain-tool"}, {"radio-tool", "Radio tool"}}
	if len(got) != len(want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("groups[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	browser := schema.Groups[0].Settings[0]
	if browser.Name != "open_browser" || browser.Type != "bool" || browser.Default != true ||
		browser.Label != "Open browser on start" || browser.Description != "Open the browser when Open WebUI is ready." {
		t.Errorf("open_browser = %+v", browser)
	}
	radio := schema.Groups[2].Settings
	if len(radio) != 3 {
		t.Fatalf("radio-tool settings = %+v, want 3 in declared order", radio)
	}
	if radio[0].Default != "vhf" || len(radio[0].Choices) != 2 || radio[0].Choices[1] != "uhf" {
		t.Errorf("band = %+v", radio[0])
	}
	if radio[1].Default != float64(5) || radio[1].Min == nil || *radio[1].Min != 1 || radio[1].Max == nil || *radio[1].Max != 10 {
		t.Errorf("gain = %+v", radio[1])
	}
	if radio[2].Type != "secret" || radio[2].Default != "" {
		t.Errorf("token = %+v", radio[2])
	}
}

func TestGenerateRemovesStaleSettingsSchemaButKeepsValues(t *testing.T) {
	stubRuntimeBinarySources(t)

	root := t.TempDir()
	dir := filepath.Join(root, ".svalbard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.schema.json"), []byte(`{"version":1,"groups":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	values := `{"ai": {"context": "balanced"}, "open-webui": {"open_browser": false}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(values), 0o644); err != nil {
		t.Fatal(err)
	}

	// No installed recipe declares a setting any more.
	if err := Generate(root, []manifest.RealizedEntry{{ID: "wikipedia", Type: "zim", Filename: "wikipedia.zim"}}, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.schema.json")); !os.IsNotExist(err) {
		t.Fatalf("stale settings.schema.json still present: %v", err)
	}
	// The user's values survive every apply, including those of a removed recipe.
	if got, err := os.ReadFile(filepath.Join(dir, "settings.json")); err != nil || string(got) != values {
		t.Fatalf("settings.json = %q (%v), want it untouched", got, err)
	}
}
