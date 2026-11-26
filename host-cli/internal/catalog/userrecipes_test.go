package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeUserRecipe(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDocument(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("%PDF-"+strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadUserRecipesAddsCollectionAPresetCanList(t *testing.T) {
	cat := NewTestCatalog(t)
	recipes, presets := t.TempDir(), t.TempDir()
	writeUserRecipe(t, recipes, "car-manuals.yaml", `
description: Owner and workshop manuals
tags: [vehicles]
documents:
  - title: Volvo 240
    url: https://example.org/manuals/volvo-240.pdf
  - url: https://example.org/dl/tm9.pdf
    filename: hmmwv.pdf
    sha256: `+strings.Repeat("AB", 32)+`
  - path: local/trailer.pdf
`)
	if skipped := cat.LoadUserRecipes(recipes, filepath.Join(recipes, "missing")); len(skipped) > 0 {
		t.Fatalf("LoadUserRecipes skipped %v", skipped)
	}
	item, ok := cat.RecipeByID("car-manuals")
	if !ok || !item.IsCollection() {
		t.Fatalf("car-manuals not loaded as a collection: %+v", item)
	}
	if item.Type != "pdf" || item.Strategy != "download" {
		t.Errorf("type/strategy = %q/%q, want pdf/download defaults", item.Type, item.Strategy)
	}
	if want := filepath.Join(recipes, "local", "trailer.pdf"); item.Documents[2].Path != want {
		t.Errorf("relative path = %q, want it anchored at the recipe dir %q", item.Documents[2].Path, want)
	}

	writeUserPreset(t, presets, "mine.yaml", "extends: [finland-32]\nsources: [car-manuals]\n")
	if skipped := cat.LoadUserPresets(presets); len(skipped) > 0 {
		t.Fatalf("user preset listing a user recipe was skipped: %v", skipped)
	}
	preset, err := cat.ResolvePreset("mine")
	if err != nil {
		t.Fatal(err)
	}
	if got := preset.Sources[len(preset.Sources)-1]; got != "car-manuals" {
		t.Errorf("preset sources = %v, want car-manuals last", preset.Sources)
	}
}

func TestLoadUserRecipesSkipsInvalid(t *testing.T) {
	cases := map[string]struct {
		file, body, wantReason string
	}{
		"shadows a built-in":  {"ifixit.yaml", "documents:\n  - url: https://example.org/a.pdf\n", "shadows a built-in recipe"},
		"url and path":        {"c.yaml", "documents:\n  - url: https://example.org/a.pdf\n    path: /tmp/a.pdf\n", "set url or path, not both"},
		"neither":             {"c.yaml", "documents:\n  - title: nothing\n", "needs a url or a path"},
		"non-http url":        {"c.yaml", "documents:\n  - url: ftp://example.org/a.pdf\n", "must start with http:// or https://"},
		"duplicate filenames": {"c.yaml", "documents:\n  - url: https://a.example/x/Manual.pdf\n  - url: https://b.example/y/manual.pdf\n", "set filename:"},
		"missing documents":   {"c.yaml", "description: empty\n", "missing documents"},
		"other acquisition":   {"c.yaml", "url: https://example.org/a.zim\ndocuments:\n  - url: https://example.org/a.pdf\n", "documents only"},
		"other type":          {"c.yaml", "type: zim\ndocuments:\n  - url: https://example.org/a.pdf\n", "use type: pdf"},
		"nameless url":        {"c.yaml", "documents:\n  - url: https://example.org/download?id=5\n", "set filename:"},
		"filename with dir":   {"c.yaml", "documents:\n  - url: https://example.org/a.pdf\n    filename: sub/a.pdf\n", "plain file name"},
		"bad pin":             {"c.yaml", "documents:\n  - url: https://example.org/a.pdf\n    sha256: abc\n", "64 hex"},
		"bad id":              {"c.yaml", "id: ../escape\ndocuments:\n  - url: https://example.org/a.pdf\n", "may only hold"},
		"bad yaml":            {"c.yaml", "documents: [\n", "invalid YAML"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cat := NewTestCatalog(t)
			builtin, _ := cat.RecipeByID("ifixit")
			dir := t.TempDir()
			writeUserRecipe(t, dir, tc.file, tc.body)
			skipped := cat.LoadUserRecipes(dir)
			if len(skipped) != 1 || !strings.Contains(skipped[0].Reason, tc.wantReason) {
				t.Fatalf("skipped = %v, want one with reason containing %q", skipped, tc.wantReason)
			}
			if !strings.Contains(skipped[0].String(), filepath.Join(dir, tc.file)) {
				t.Errorf("warning %q does not name the file", skipped[0])
			}
			if got, _ := cat.RecipeByID("ifixit"); !reflect.DeepEqual(got, builtin) {
				t.Error("built-in recipe was replaced")
			}
			if _, ok := cat.RecipeByID("c"); ok {
				t.Error("invalid recipe was added to the catalog")
			}
			if tc.file == "c.yaml" && tc.wantReason != "may only hold" {
				if s, ok := cat.SkippedUserRecipe("c"); !ok || s.Err() == nil {
					t.Error("SkippedUserRecipe does not explain the skip")
				}
			}
		})
	}
}

func TestLoadUserRecipesSkipsSecondDefinitionOfAnID(t *testing.T) {
	cat := NewTestCatalog(t)
	first, second := t.TempDir(), t.TempDir()
	writeUserRecipe(t, first, "docs.yaml", "documents:\n  - url: https://example.org/a.pdf\n")
	writeUserRecipe(t, second, "docs.yaml", "documents:\n  - url: https://example.org/b.pdf\n")
	skipped := cat.LoadUserRecipes(first, second)
	if len(skipped) != 1 || !strings.Contains(skipped[0].Reason, "already defined in "+filepath.Join(first, "docs.yaml")) {
		t.Fatalf("skipped = %v", skipped)
	}
	item, _ := cat.RecipeByID("docs")
	if item.Documents[0].URL != "https://example.org/a.pdf" {
		t.Errorf("second definition replaced the first: %+v", item.Documents)
	}
}

func TestLoadCatalogLoadsUserRecipesBeforeUserPresets(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	writeUserRecipe(t, filepath.Join(cfg, "svalbard", "recipes"), "car-manuals.yaml", "documents:\n  - url: https://example.org/a.pdf\n")
	writeUserRecipe(t, filepath.Join(cfg, "svalbard", "recipes"), "broken.yaml", "description: none\n")
	writeUserRecipe(t, filepath.Join(cfg, "svalbard", "presets"), "mine.yaml", "sources: [car-manuals]\n")

	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("a broken user recipe failed the catalog load: %v", err)
	}
	if _, ok := cat.RecipeByID("car-manuals"); !ok {
		t.Fatal("user recipe not loaded")
	}
	if skipped := cat.SkippedUserPresets(); len(skipped) > 0 {
		t.Fatalf("preset listing a user recipe was skipped: %v", skipped)
	}
	if skipped := cat.SkippedUserRecipes(); len(skipped) != 1 || skipped[0].ID != "broken" {
		t.Fatalf("skipped recipes = %v, want broken", skipped)
	}
}

func TestExpectedDocumentsMirrorsFoldersAndNamesFiles(t *testing.T) {
	cat := NewTestCatalog(t)
	dir, src := t.TempDir(), t.TempDir()
	writeDocument(t, filepath.Join(src, "trailer.pdf"), 10)
	writeDocument(t, filepath.Join(src, "workshop", "brakes.pdf"), 20)
	writeDocument(t, filepath.Join(src, "workshop", "engine", "b20", "Timing.PDF"), 30)
	writeDocument(t, filepath.Join(src, "workshop", "engine", "notes.txt"), 1)
	writeDocument(t, filepath.Join(src, "workshop", "._brakes.pdf"), 1)
	writeDocument(t, filepath.Join(src, "workshop", ".trash", "old.pdf"), 1)
	writeUserRecipe(t, dir, "car-manuals.yaml", `
documents:
  - title: Volvo 240
    url: https://example.org/manuals/volvo%20240.pdf
  - url: https://example.org/dl/tm9.pdf
    filename: hmmwv.pdf
  - title: Trailer
    path: `+filepath.Join(src, "trailer.pdf")+`
  - path: `+filepath.Join(src, "workshop")+`/
`)
	if skipped := cat.LoadUserRecipes(dir); len(skipped) > 0 {
		t.Fatal(skipped)
	}
	item, _ := cat.RecipeByID("car-manuals")
	docs, err := ExpectedDocuments(item)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, doc := range docs {
		got = append(got, doc.RelPath)
	}
	want := []string{"volvo 240.pdf", "hmmwv.pdf", "trailer.pdf", "workshop/brakes.pdf", "workshop/engine/b20/Timing.PDF"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("documents = %v, want %v", got, want)
	}
	if docs[0].Title != "Volvo 240" || docs[0].URL == "" || docs[2].Title != "Trailer" || docs[2].Size != 15 || docs[4].Source == "" {
		t.Errorf("document details lost: %+v", docs)
	}
}

func TestExpectedDocumentsReportsUnreadableAndCollidingSources(t *testing.T) {
	src := t.TempDir()
	writeDocument(t, filepath.Join(src, "a", "manual.pdf"), 1)
	writeDocument(t, filepath.Join(src, "manual.pdf"), 1)
	if err := os.MkdirAll(filepath.Join(src, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		docs []DocumentSource
		want string
	}{
		"missing folder":   {[]DocumentSource{{Path: filepath.Join(src, "gone")}}, "cannot read " + filepath.Join(src, "gone")},
		"empty folder":     {[]DocumentSource{{Path: filepath.Join(src, "empty")}}, "no documents found"},
		"folder with pin":  {[]DocumentSource{{Path: filepath.Join(src, "a"), SHA256: strings.Repeat("a", 64)}}, "is a folder"},
		"case collision":   {[]DocumentSource{{Path: filepath.Join(src, "manual.pdf")}, {URL: "https://example.org/Manual.pdf"}}, "would both be saved as Manual.pdf; set filename:"},
		"folder collision": {[]DocumentSource{{Path: filepath.Join(src, "a")}, {URL: "https://example.org/x.pdf", Filename: "A"}}, "which is also the folder of"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ExpectedDocuments(Item{ID: "c", Documents: tc.docs})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
