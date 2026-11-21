package catalog

import (
	"strings"
	"testing"
)

func TestSizeGBForCountsPerPlatformPayloadOncePerSupportedPlatform(t *testing.T) {
	all := []string{"macos-arm64", "macos-x86_64", "linux-arm64", "linux-x86_64"}
	single := Item{ID: "zim", SizeGB: 2}
	binary := Item{ID: "tool", SizeGB: 0.5, Platforms: map[string]string{
		"darwin-arm64": "u", "linux-arm64": "u", "linux-x86_64": "u",
	}}
	python := Item{ID: "pkg", Type: "python-package", SizeGB: 0.25}

	cases := []struct {
		name      string
		item      Item
		platforms []string
		want      float64
	}{
		{"single asset ignores platforms", single, all, 2},
		{"no platforms assumes the host", binary, nil, 0.5},
		{"one per selected platform, aliases count", binary, []string{"macos-arm64", "linux-x86_64"}, 1},
		{"unsupported platforms do not count", binary, all, 1.5},
		{"python packages build for every platform", python, all, 1},
		{"python packages on one platform", python, []string{"linux-arm64"}, 0.25},
	}
	for _, tc := range cases {
		if got := tc.item.SizeGBFor(tc.platforms); got != tc.want {
			t.Errorf("%s: SizeGBFor(%v) = %v, want %v", tc.name, tc.platforms, got, tc.want)
		}
	}

	p := Preset{Items: []Item{single, binary, python}}
	if got, want := p.ContentSizeGB(all), 2+1.5+1.0; got != want {
		t.Errorf("ContentSizeGB(all) = %v, want %v", got, want)
	}
}

// The default tiers must fit their drive size with the wizard's default of
// one platform.
func TestBuiltinPresetsFitTargetSize(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range cat.PresetNames() {
		p, _ := cat.PresetByName(name)
		if p.Kind == "pack" || p.TargetSizeGB <= 0 || strings.HasPrefix(name, "test-") {
			continue
		}
		resolved, err := cat.ResolvePreset(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := resolved.ContentSizeGB(nil); got > p.TargetSizeGB {
			t.Errorf("%s: content %.2f GB exceeds target %.0f GB", name, got, p.TargetSizeGB)
		}
	}
}
