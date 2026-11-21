package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeUserPreset(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadUserPresetsComposesBuiltins(t *testing.T) {
	cat := NewTestCatalog(t)
	dir := t.TempDir()
	writeUserPreset(t, dir, "mine.yaml", `
description: finland-32 without ifixit, plus test-pack
extends: [finland-32, test-pack]
sources: [-ifixit]
`)
	if skipped := cat.LoadUserPresets(dir, filepath.Join(dir, "missing")); len(skipped) > 0 {
		t.Fatalf("LoadUserPresets skipped %v", skipped)
	}

	p, err := cat.ResolvePreset("mine")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"wikipedia-en-nopic"}; !reflect.DeepEqual(p.Sources, want) {
		t.Errorf("Sources = %v, want %v", p.Sources, want)
	}
	if !p.IsUser() || p.Path != filepath.Join(dir, "mine.yaml") {
		t.Errorf("Path = %q, want user preset path", p.Path)
	}
	if p.Region != "finland" {
		t.Errorf("Region = %q, want inherited %q", p.Region, "finland")
	}
	if builtin, _ := cat.PresetByName("default-32"); builtin.IsUser() {
		t.Error("built-in preset reported as user preset")
	}
	found := false
	for _, fp := range cat.PresetsForRegion("finland") {
		found = found || fp.Name == "mine"
	}
	if !found {
		t.Error("user preset missing from PresetsForRegion(finland)")
	}
}

func TestLoadUserPresetsCanExtendEachOther(t *testing.T) {
	cat := NewTestCatalog(t)
	dir := t.TempDir()
	writeUserPreset(t, dir, "a.yaml", "extends: [b]\nsources: [ifixit]\n")
	writeUserPreset(t, dir, "b.yaml", "sources: [wikipedia-en-nopic]\n")
	if skipped := cat.LoadUserPresets(dir); len(skipped) > 0 {
		t.Fatal(skipped)
	}
	p, err := cat.ResolvePreset("a")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"wikipedia-en-nopic", "ifixit"}; !reflect.DeepEqual(p.Sources, want) {
		t.Errorf("Sources = %v, want %v", p.Sources, want)
	}
	if p.Region != UserRegion {
		t.Errorf("Region = %q, want %q", p.Region, UserRegion)
	}
}

func TestLoadUserPresetsSkipsInvalid(t *testing.T) {
	cases := map[string]struct {
		files      map[string]string
		wantName   string // preset whose lookup must explain the skip
		wantReason string
	}{
		"shadows built-in":  {map[string]string{"x.yaml": "name: default-32\nsources: [ifixit]\n"}, "", "shadows a built-in preset"},
		"duplicate user":    {map[string]string{"x.yaml": "name: dup\nsources: []\n", "y.yaml": "name: dup\nsources: []\n"}, "", "already defined in"},
		"unknown extends":   {map[string]string{"x.yaml": "extends: [nope-256]\nsources: []\n"}, "x", `extends unknown preset "nope-256"`},
		"unknown source":    {map[string]string{"x.yaml": "sources: [not-a-recipe]\n"}, "x", `unknown source id "not-a-recipe"`},
		"unknown removal":   {map[string]string{"x.yaml": "extends: [default-32]\nsources: [-typo]\n"}, "x", `unknown source id "typo"`},
		"circular extends":  {map[string]string{"x.yaml": "extends: [y]\nsources: []\n", "y.yaml": "extends: [x]\nsources: []\n"}, "x", "circular"},
		"invalid yaml file": {map[string]string{"x.yaml": "sources: [\n"}, "x", "invalid YAML"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cat := NewTestCatalog(t)
			dir := t.TempDir()
			writeUserPreset(t, dir, "good.yaml", "extends: [default-32]\nsources: [-ifixit]\n")
			for f, body := range tc.files {
				writeUserPreset(t, dir, f, body)
			}

			skipped := cat.LoadUserPresets(dir)
			if len(skipped) == 0 {
				t.Fatal("nothing skipped")
			}
			for _, s := range skipped {
				if !strings.Contains(s.Reason, tc.wantReason) {
					t.Errorf("reason = %q, want containing %q", s.Reason, tc.wantReason)
				}
				if filepath.Dir(s.Path) != dir || !strings.Contains(s.String(), s.Path) {
					t.Errorf("warning %q should name the offending file in %s", s, dir)
				}
			}
			if !reflect.DeepEqual(cat.SkippedUserPresets(), skipped) {
				t.Errorf("SkippedUserPresets = %v, want %v", cat.SkippedUserPresets(), skipped)
			}

			good, err := cat.ResolvePreset("good")
			if err != nil {
				t.Fatalf("valid preset next to a broken one: %v", err)
			}
			if len(good.Items) == 0 {
				t.Error("valid preset resolved to no items")
			}
			if tc.wantName != "" {
				if _, ok := cat.PresetByName(tc.wantName); ok {
					t.Fatalf("skipped preset %q is still in the catalog", tc.wantName)
				}
				_, err := cat.ResolvePreset(tc.wantName)
				if err == nil || !strings.Contains(err.Error(), "was skipped") || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("ResolvePreset(%q) err = %v, want the skip reason", tc.wantName, err)
				}
			}
		})
	}
}

func TestLoadUserPresetsSkipsPresetsExtendingASkippedOne(t *testing.T) {
	cat := NewTestCatalog(t)
	dir := t.TempDir()
	writeUserPreset(t, dir, "a.yaml", "extends: [b]\nsources: []\n")
	writeUserPreset(t, dir, "b.yaml", "sources: [not-a-recipe]\n")

	skipped := cat.LoadUserPresets(dir)
	reasons := map[string]string{}
	for _, s := range skipped {
		reasons[s.Name] = s.Reason
	}
	if !strings.Contains(reasons["b"], "unknown source id") {
		t.Errorf("b reason = %q", reasons["b"])
	}
	if !strings.Contains(reasons["a"], `extends user preset "b", which was skipped`) {
		t.Errorf("a reason = %q", reasons["a"])
	}
}

func TestUserPresetDirHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	dir, err := UserPresetDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/cfg", "svalbard", "presets"); dir != want {
		t.Errorf("UserPresetDir = %q, want %q", dir, want)
	}
}

func TestLoadCatalogIncludesUserPresetDir(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "svalbard", "presets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserPreset(t, dir, "my-32.yaml", "extends: [default-32]\nsources: []\n")

	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	p, ok := cat.PresetByName("my-32")
	if !ok || !p.IsUser() {
		t.Fatalf("my-32 not loaded as user preset: %+v", p)
	}
}

func TestLoadCatalogSkipsBrokenUserPreset(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "svalbard", "presets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeUserPreset(t, dir, "my-32.yaml", "extends: [default-32]\nsources: []\n")
	writeUserPreset(t, dir, "broken.yaml", "extends: [default-32]\nsources: [no-such-recipe]\n")

	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("a broken user preset failed the catalog load: %v", err)
	}
	if _, err := cat.ResolvePreset("my-32"); err != nil {
		t.Fatalf("valid user preset: %v", err)
	}
	var warnings []string
	for _, s := range cat.SkippedUserPresets() {
		warnings = append(warnings, s.String())
	}
	broken := filepath.Join(dir, "broken.yaml")
	if len(warnings) != 1 || !strings.Contains(warnings[0], broken) || !strings.Contains(warnings[0], "no-such-recipe") {
		t.Fatalf("warnings = %q, want one naming %s and the reason", warnings, broken)
	}
}
