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
	if err := cat.LoadUserPresets(dir, filepath.Join(dir, "missing")); err != nil {
		t.Fatalf("LoadUserPresets: %v", err)
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
	if err := cat.LoadUserPresets(dir); err != nil {
		t.Fatal(err)
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

func TestLoadUserPresetsRejectsInvalid(t *testing.T) {
	cases := map[string]struct {
		files   map[string]string
		wantErr string
	}{
		"shadows built-in":  {map[string]string{"x.yaml": "name: default-32\nsources: [ifixit]\n"}, "shadows a built-in preset"},
		"duplicate user":    {map[string]string{"x.yaml": "name: dup\nsources: []\n", "y.yaml": "name: dup\nsources: []\n"}, "already defined"},
		"unknown extends":   {map[string]string{"x.yaml": "extends: [nope-256]\nsources: []\n"}, `extends unknown preset "nope-256"`},
		"unknown source":    {map[string]string{"x.yaml": "sources: [not-a-recipe]\n"}, `unknown source id "not-a-recipe"`},
		"unknown removal":   {map[string]string{"x.yaml": "extends: [default-32]\nsources: [-typo]\n"}, `unknown source id "typo"`},
		"circular extends":  {map[string]string{"x.yaml": "extends: [y]\nsources: []\n", "y.yaml": "extends: [x]\nsources: []\n"}, "circular"},
		"invalid yaml file": {map[string]string{"x.yaml": "sources: [\n"}, "parsing user preset"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cat := NewTestCatalog(t)
			dir := t.TempDir()
			for f, body := range tc.files {
				writeUserPreset(t, dir, f, body)
			}
			err := cat.LoadUserPresets(dir)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), dir) {
				t.Errorf("error should name the offending file: %v", err)
			}
		})
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
