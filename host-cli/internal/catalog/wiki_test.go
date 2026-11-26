package catalog

import (
	"strings"
	"testing"
)

func TestRealCatalogPinsTheWikiForEveryPlatform(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item, ok := cat.RecipeByID("silverbullet")
	if !ok {
		t.Fatal("silverbullet recipe missing")
	}
	// The drive runtime writes this server's state files for one release.
	for _, platform := range []string{"macos-arm64", "macos-x86_64", "linux-arm64", "linux-x86_64"} {
		url := item.Platforms[platform]
		if !strings.Contains(url, "/releases/download/3.0.1/silverbullet-server-") || !strings.HasSuffix(url, ".zip") {
			t.Errorf("%s url = %q, want the 3.0.1 server zip", platform, url)
		}
		if len(item.PlatformSHA256[platform]) != 64 {
			t.Errorf("%s has no sha256 pin", platform)
		}
	}
	if len(item.Platforms) != 4 {
		t.Errorf("platforms = %v, want the four the drive supports", item.Platforms)
	}
	if item.Type != "binary" || item.License == nil || item.License.ID != "MIT" {
		t.Errorf("type = %q, license = %+v; want a binary under MIT", item.Type, item.License)
	}

	if len(item.Settings) != 2 {
		t.Fatalf("silverbullet settings = %+v, want open_browser and fs_watch", item.Settings)
	}
	if s := item.Settings[0]; s.Name != "open_browser" || s.Type != "bool" || s.Default != true {
		t.Errorf("setting 0 = %+v, want open_browser as a bool defaulting to true", s)
	}
	if s := item.Settings[1]; s.Name != "fs_watch" || s.Type != "choice" || s.Default != "auto" || strings.Join(s.Choices, ",") != "auto,poll" {
		t.Errorf("setting 1 = %+v, want fs_watch as auto or poll, defaulting to auto", s)
	}
}

func TestEveryDefaultPresetHasTheWiki(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range cat.PresetNames() {
		p, _ := cat.PresetByName(name)
		if p.Kind == "pack" || strings.HasPrefix(name, "test-") {
			continue
		}
		resolved, err := cat.ResolvePreset(name)
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, item := range resolved.Items {
			if item.ID == "silverbullet" {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%s has the wiki %d times, want once", name, found)
		}
		checked++
	}
	if checked < 16 {
		t.Errorf("checked %d presets, want the default and finland tiers", checked)
	}
}
