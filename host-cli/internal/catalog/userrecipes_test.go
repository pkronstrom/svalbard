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
		"dot-dot url":         {"c.yaml", "documents:\n  - url: https://example.org/files/..\n", "set filename:"},
		"unportable url name": {"c.yaml", "documents:\n  - url: https://example.org/report%3Afinal.pdf\n", "cannot be stored on every drive"},
		"unportable filename": {"c.yaml", "documents:\n  - url: https://example.org/a.pdf\n    filename: 'what?.pdf'\n", "plain file name"},
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

// On an exFAT vault books/Manuals and books/manuals are one folder, so two
// collections whose ids differ only in case would overwrite and delete each
// other's files.
func TestLoadUserRecipesRejectsIDsThatDifferOnlyInCase(t *testing.T) {
	cat := NewTestCatalog(t)
	dir := t.TempDir()
	body := "documents:\n  - url: https://example.org/a.pdf\n"
	writeUserRecipe(t, dir, "a-manuals.yaml", body)
	writeUserRecipe(t, dir, "b.yaml", "id: A-Manuals\n"+body)
	writeUserRecipe(t, dir, "c.yaml", "id: IFIXIT\n"+body)
	skipped := cat.LoadUserRecipes(dir)
	if len(skipped) != 2 {
		t.Fatalf("skipped = %v, want the two case twins", skipped)
	}
	if !strings.Contains(skipped[0].Reason, `differs only in letter case from the recipe "a-manuals"`) {
		t.Errorf("user twin reason = %q", skipped[0].Reason)
	}
	if !strings.Contains(skipped[1].Reason, `differs only in letter case from the recipe "ifixit"`) {
		t.Errorf("built-in twin reason = %q", skipped[1].Reason)
	}
	if _, ok := cat.RecipeByID("A-Manuals"); ok {
		t.Error("the case twin was added to the catalog")
	}
	if _, ok := cat.RecipeByID("a-manuals"); !ok {
		t.Error("the first definition was dropped")
	}
}

func TestLoadVaultRecipesFillsInOnlyWhatThisComputerLacks(t *testing.T) {
	cat := NewTestCatalog(t)
	config, vault := t.TempDir(), t.TempDir()
	local := filepath.Join(t.TempDir(), "trailer.pdf")
	writeDocument(t, local, 1)
	writeUserRecipe(t, config, "here.yaml", "description: edited here\ndocuments:\n  - url: https://example.org/new.pdf\n")
	writeUserRecipe(t, vault, "here.yaml", "description: stale copy\ndocuments:\n  - url: https://example.org/old.pdf\n")
	writeUserRecipe(t, vault, "elsewhere.yaml", "description: from another computer\ndocuments:\n"+
		"  - title: Volvo\n    url: https://example.org/volvo.pdf\n"+
		"  - title: Trailer\n    path: "+local+"\n"+
		"  - path: ~/Documents/manuals/workshop/\n")
	writeUserRecipe(t, vault, "broken.yaml", "description: no documents\n")
	cat.LoadUserRecipes(config)

	if used := cat.LoadVaultRecipes(vault); !reflect.DeepEqual(used, []string{"elsewhere"}) {
		t.Fatalf("vault copies used = %v, want only the one this computer lacks", used)
	}
	if cat.LoadVaultRecipes(filepath.Join(vault, "missing")) != nil {
		t.Error("a vault without copies added recipes")
	}
	if here, _ := cat.RecipeByID("here"); here.Description != "edited here" || here.VaultCopy {
		t.Errorf("the definition on this computer lost to the vault's copy: %+v", here)
	}
	if skipped := cat.SkippedUserRecipes(); len(skipped) != 1 || skipped[0].ID != "broken" {
		t.Errorf("skipped = %v, want the broken copy reported", skipped)
	}

	item, _ := cat.RecipeByID("elsewhere")
	if !item.VaultCopy || item.Path != filepath.Join(vault, "elsewhere.yaml") {
		t.Fatalf("item = %+v", item)
	}
	// Paths stay as written, and are not read even when one exists here: a
	// vault from elsewhere must not get this computer's files copied onto it.
	if got := item.Documents[2].Path; got != "~/Documents/manuals/workshop/" {
		t.Errorf("path = %q, want it as written", got)
	}
	set, err := ExpectedDocuments(item)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Documents) != 1 || set.Documents[0].URL != "https://example.org/volvo.pdf" {
		t.Fatalf("documents = %+v, want only the URL document", set.Documents)
	}
	want := []UnreadableSource{
		{Path: local, Name: "trailer.pdf", Reason: "local source, not refreshed from the vault's copy", Foreign: true},
		{Path: "~/Documents/manuals/workshop/", Name: "workshop", Reason: "local source, not refreshed from the vault's copy", Foreign: true},
	}
	if !reflect.DeepEqual(set.Unreadable, want) {
		t.Fatalf("unreadable = %+v, want %+v", set.Unreadable, want)
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
	set, err := ExpectedDocuments(item)
	if err != nil || len(set.Unreadable) != 0 {
		t.Fatalf("ExpectedDocuments: %v, unreadable %+v", err, set.Unreadable)
	}
	docs := set.Documents
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

// A local source that cannot be read, and a folder with nothing in it, say
// nothing about the documents they held: they must not shrink the set.
func TestExpectedDocumentsKeepsUnreadableSourcesApart(t *testing.T) {
	src := t.TempDir()
	writeDocument(t, filepath.Join(src, "trailer.pdf"), 1)
	if err := os.MkdirAll(filepath.Join(src, "unsynced", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	item := Item{ID: "c", Documents: []DocumentSource{
		{Path: filepath.Join(src, "trailer.pdf")},
		{Path: filepath.Join(src, "unsynced")},
		{Path: filepath.Join(src, "unplugged") + "/"},
		{Path: filepath.Join(src, "gone.pdf"), Filename: "renamed.pdf"},
	}}
	set, err := ExpectedDocuments(item)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Documents) != 1 || set.Documents[0].RelPath != "trailer.pdf" {
		t.Fatalf("documents = %+v", set.Documents)
	}
	want := []UnreadableSource{
		{Path: filepath.Join(src, "unsynced"), Name: "unsynced", Reason: "no documents found in it"},
		{Path: filepath.Join(src, "unplugged") + "/", Name: "unplugged", Reason: "no such file or directory"},
		{Path: filepath.Join(src, "gone.pdf"), Name: "renamed.pdf", Reason: "no such file or directory"},
	}
	if !reflect.DeepEqual(set.Unreadable, want) {
		t.Fatalf("unreadable = %+v, want %+v", set.Unreadable, want)
	}
	for rel, held := range map[string]bool{
		"unsynced/a.pdf": true, "unsynced/sub/b.pdf": true, "unplugged/c.pdf": true, "renamed.pdf": true,
		"trailer.pdf": false, "unsynced.pdf": false, "renamed.pdf.bak": false,
	} {
		if _, got := set.Holds(rel); got != held {
			t.Errorf("Holds(%q) = %v, want %v", rel, got, held)
		}
	}
}

func TestExpectedDocumentsRejectsUnusableDefinitions(t *testing.T) {
	src := t.TempDir()
	writeDocument(t, filepath.Join(src, "a", "manual.pdf"), 1)
	writeDocument(t, filepath.Join(src, "manual.pdf"), 1)
	cases := map[string]struct {
		docs []DocumentSource
		want string
	}{
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

func TestLoadUserRecipesAddsJSLibraries(t *testing.T) {
	cat := NewTestCatalog(t)
	recipes := t.TempDir()
	pin := strings.Repeat("ab", 32)
	writeUserRecipe(t, recipes, "js-mine.yaml", `
type: js-lib
description: My charts
libraries:
  - package: uplot
    version: 1.6.32
    entry: uplot
    quickstart: uplot.md
bundle:
  - url: https://registry.npmjs.org/uplot/-/uplot-1.6.32.tgz
    sha256: `+pin+`
    strip: package
    dest: node_modules/uplot
    include: [package.json, LICENSE, README.md, dist/uPlot.esm.js]
`)
	writeUserRecipe(t, recipes, "js-same.yaml", `
type: js-lib
libraries:
  - package: uplot
    quickstart: uplot.md
bundle:
  - url: https://registry.npmjs.org/uplot/-/uplot-1.6.31.tgz
    sha256: `+pin+`
    dest: node_modules/uplot
`)
	writeUserRecipe(t, recipes, "js-unpinned.yaml", `
type: js-lib
bundle:
  - url: https://registry.npmjs.org/a/-/a-1.0.0.tgz
    dest: node_modules/a
`)
	skipped := cat.LoadUserRecipes(recipes)
	item, ok := cat.RecipeByID("js-mine")
	if !ok || item.Type != LibraryType || item.Strategy != "download" || len(item.Libraries) != 1 {
		t.Fatalf("js-mine = %+v (loaded %v), want a js-lib recipe", item, ok)
	}
	reasons := map[string]string{}
	for _, s := range skipped {
		reasons[s.ID] = s.Reason
	}
	if !strings.Contains(reasons["js-same"], "uplot") || !strings.Contains(reasons["js-same"], "js-mine") {
		t.Errorf("js-same skipped for %q, want the library it shares with js-mine", reasons["js-same"])
	}
	if !strings.Contains(reasons["js-unpinned"], "sha256") {
		t.Errorf("js-unpinned skipped for %q, want the missing pin", reasons["js-unpinned"])
	}
}
