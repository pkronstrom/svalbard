package catalog

import (
	"strings"
	"testing"
	"testing/fstest"
)

func settingsCatalog(recipe string) (*Catalog, error) {
	return LoadFromFS(fstest.MapFS{"tools/radio-tool.yaml": {Data: []byte(recipe)}}, fstest.MapFS{})
}

func TestItemParsesSettings(t *testing.T) {
	cat, err := settingsCatalog(`id: radio-tool
type: binary
settings:
  - name: autostart
    type: bool
    default: true
    label: Start on launch
    description: Start scanning at once.
  - name: band
    type: choice
    default: vhf
    choices: [vhf, uhf]
    label: Band
    description: Band to scan.
  - name: gain
    type: number
    default: 5
    min: 1
    max: 10
    label: Gain
    description: Receiver gain.
  - name: station
    type: text
    default: ""
    label: Station
    description: Station name.
  - name: token
    type: secret
    default: ""
    label: Token
    description: Access token.
`)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := cat.RecipeByID("radio-tool")
	if len(item.Settings) != 5 {
		t.Fatalf("Settings = %+v, want 5", item.Settings)
	}
	if s := item.Settings[0]; s.Name != "autostart" || s.Type != "bool" || s.Default != true || s.Label != "Start on launch" || s.Description != "Start scanning at once." {
		t.Errorf("autostart = %+v", s)
	}
	if s := item.Settings[1]; s.Default != "vhf" || len(s.Choices) != 2 {
		t.Errorf("band = %+v", s)
	}
	gain := item.Settings[2]
	if n, ok := gain.NumberDefault(); !ok || n != 5 || gain.Min == nil || *gain.Min != 1 || gain.Max == nil || *gain.Max != 10 {
		t.Errorf("gain = %+v", gain)
	}
	if s := item.Settings[4]; s.Type != "secret" || s.Default != "" {
		t.Errorf("token = %+v", s)
	}
}

func TestLoadRejectsInvalidSettings(t *testing.T) {
	setting := func(body string) string {
		return "id: radio-tool\ntype: binary\nsettings:\n" + body
	}
	for name, tc := range map[string]struct{ recipe, want string }{
		"reserved recipe id":                   {"id: ai\ntype: binary\n", "reserved"},
		"reserved recipe id declaring nothing": {"id: network\ntype: zim\n", "reserved"},
		"bad name":                             {setting("  - {name: Open-Browser, type: bool, default: true, label: L, description: D}\n"), "[a-z][a-z0-9_]*"},
		"missing name":                         {setting("  - {type: bool, default: true, label: L, description: D}\n"), "[a-z][a-z0-9_]*"},
		"name declared twice": {setting("  - {name: a, type: bool, default: true, label: L, description: D}\n" +
			"  - {name: a, type: text, default: x, label: L, description: D}\n"), "declared twice"},
		"unknown type":           {setting("  - {name: a, type: integer, default: 1, label: L, description: D}\n"), `unknown type "integer"`},
		"missing type":           {setting("  - {name: a, default: 1, label: L, description: D}\n"), "unknown type"},
		"missing default":        {setting("  - {name: a, type: bool, label: L, description: D}\n"), "default is required"},
		"bool default is text":   {setting("  - {name: a, type: bool, default: \"yes\", label: L, description: D}\n"), "not true or false"},
		"number default is text": {setting("  - {name: a, type: number, default: five, label: L, description: D}\n"), "not a number"},
		"number default is bool": {setting("  - {name: a, type: number, default: true, label: L, description: D}\n"), "not a number"},
		"text default is number": {setting("  - {name: a, type: text, default: 5, label: L, description: D}\n"), "not text"},
		"secret default is bool": {setting("  - {name: a, type: secret, default: false, label: L, description: D}\n"), "not text"},
		"choice without choices": {setting("  - {name: a, type: choice, default: x, label: L, description: D}\n"), "needs choices"},
		"default not a choice":   {setting("  - {name: a, type: choice, default: c, choices: [a, b], label: L, description: D}\n"), "not one of the choices"},
		"repeated choice":        {setting("  - {name: a, type: choice, default: a, choices: [a, a], label: L, description: D}\n"), "repeated"},
		"choices on a bool":      {setting("  - {name: a, type: bool, default: true, choices: [a], label: L, description: D}\n"), "choices are for type choice"},
		"range on text":          {setting("  - {name: a, type: text, default: x, min: 1, label: L, description: D}\n"), "min and max are for type number"},
		"default below min":      {setting("  - {name: a, type: number, default: 0, min: 1, label: L, description: D}\n"), "outside min and max"},
		"default above max":      {setting("  - {name: a, type: number, default: 11, max: 10, label: L, description: D}\n"), "outside min and max"},
		"min above max":          {setting("  - {name: a, type: number, default: 5, min: 9, max: 1, label: L, description: D}\n"), "above max"},
		"missing label":          {setting("  - {name: a, type: bool, default: true, description: D}\n"), "label is required"},
		"missing description":    {setting("  - {name: a, type: bool, default: true, label: L}\n"), "description is required"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := settingsCatalog(tc.recipe)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadFromFS() error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestRealCatalogDeclaresOpenWebUIBrowserSetting(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, ok := cat.RecipeByID("open-webui")
	if !ok {
		t.Fatal("open-webui recipe missing")
	}
	if len(item.Settings) != 1 {
		t.Fatalf("open-webui settings = %+v, want open_browser only", item.Settings)
	}
	if s := item.Settings[0]; s.Name != "open_browser" || s.Type != "bool" || s.Default != true {
		t.Errorf("open_browser = %+v, want a bool defaulting to true", s)
	}
	// No recipe may take a built-in owner's place.
	for _, owner := range ReservedSettingOwners {
		if _, taken := cat.RecipeByID(owner); taken {
			t.Errorf("recipe id %q is reserved for built-in settings", owner)
		}
	}
}
