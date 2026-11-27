package catalog

import (
	"strings"
	"testing"
	"testing/fstest"
)

func libRecipe(id string, packages ...string) Item {
	item := Item{ID: id, Type: LibraryType}
	for _, pkg := range packages {
		item.Bundle = append(item.Bundle, BundleSource{URL: "https://registry.npmjs.org/" + pkg + ".tgz", SHA256: "aa", Strip: "package", Dest: "node_modules/" + pkg})
		item.Libraries = append(item.Libraries, Library{Package: pkg, Version: "1.0.0", Quickstart: LibraryDocName(pkg)})
	}
	return item
}

func TestValidateLibrariesRejectsDuplicateTopLevel(t *testing.T) {
	a := libRecipe("js-a", "chart.js", "d3")
	b := libRecipe("js-b", "d3")
	err := ValidateLibraries([]Item{a, b})
	if err == nil || !strings.Contains(err.Error(), "d3") || !strings.Contains(err.Error(), "js-a") || !strings.Contains(err.Error(), "js-b") {
		t.Fatalf("err = %v, want d3 named with both recipes", err)
	}
	// A nested dependency in two recipes is allowed.
	b = libRecipe("js-b", "pmtiles")
	b.Bundle = append(b.Bundle, BundleSource{URL: "u", SHA256: "s", Dest: "node_modules/pmtiles/node_modules/d3"})
	if err := ValidateLibraries([]Item{a, b}); err != nil {
		t.Fatalf("nested dependency refused: %v", err)
	}
}

func TestValidateLibrariesRejectsUnknownPortability(t *testing.T) {
	a := libRecipe("js-a", "chart.js")
	a.Libraries[0].Portability = "portable"
	if err := ValidateLibraries([]Item{a}); err == nil || !strings.Contains(err.Error(), "portable") {
		t.Fatalf("err = %v, want the unknown level named", err)
	}
	for _, level := range []string{"", PortabilitySingleFile, PortabilityServed, PortabilityDirectory} {
		a.Libraries[0].Portability = level
		if err := ValidateLibraries([]Item{a}); err != nil {
			t.Errorf("portability %q refused: %v", level, err)
		}
	}
}

func TestValidateLibrariesNeedsTheLibrarySource(t *testing.T) {
	a := libRecipe("js-a", "chart.js")
	a.Libraries = append(a.Libraries, Library{Package: "three", Quickstart: "three.md"})
	if err := ValidateLibraries([]Item{a}); err == nil || !strings.Contains(err.Error(), "node_modules/three") {
		t.Fatalf("err = %v, want the missing source named", err)
	}
}

func TestLibraryDocName(t *testing.T) {
	for pkg, want := range map[string]string{"chart.js": "chart.js.md", "@jscad/modeling": "@jscad__modeling.md", "three": "three.md"} {
		if got := LibraryDocName(pkg); got != want {
			t.Errorf("LibraryDocName(%q) = %q, want %q", pkg, got, want)
		}
	}
	if got := LibraryReferenceName("@jscad/modeling"); got != "@jscad__modeling.reference.md" {
		t.Errorf("reference name = %q", got)
	}
}

func TestCatalogLoadValidatesLibraries(t *testing.T) {
	recipe := func(id string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("id: " + id + "\ntype: js-lib\nlibraries:\n  - package: d3\n    quickstart: d3.md\nbundle:\n  - url: https://x/d3.tgz\n    sha256: aa\n    dest: node_modules/d3\n")}
	}
	_, err := LoadFromFS(fstest.MapFS{"a.yaml": recipe("js-a"), "b.yaml": recipe("js-b")}, fstest.MapFS{})
	if err == nil || !strings.Contains(err.Error(), "d3") {
		t.Fatalf("err = %v, want the duplicate library refused at load", err)
	}
}

func embeddedRecipe(t *testing.T, cat *Catalog, id string) Item {
	t.Helper()
	item, ok := cat.RecipeByID(id)
	if !ok {
		t.Fatalf("catalog has no %s", id)
	}
	return item
}

// bundleVersion returns the version a recipe's source for pkg pins, from its
// npm tarball URL (<name>-<version>.tgz).
func bundleVersion(item Item, pkg string) string {
	base := pkg[strings.LastIndex(pkg, "/")+1:]
	for _, source := range item.Bundle {
		marker := "/" + pkg + "/-/" + base + "-"
		if at := strings.Index(source.URL, marker); at >= 0 {
			return strings.TrimSuffix(source.URL[at+len(marker):], ".tgz")
		}
	}
	return ""
}

func TestMapLibreVersionsMatch(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	vendor, maps := embeddedRecipe(t, cat, "maplibre-vendor"), embeddedRecipe(t, cat, "js-maps")
	for _, pkg := range []string{"maplibre-gl", "pmtiles"} {
		v, m := bundleVersion(vendor, pkg), bundleVersion(maps, pkg)
		if v == "" || v != m {
			t.Errorf("%s: maplibre-vendor pins %q, js-maps %q; they must match", pkg, v, m)
		}
	}
	if v := bundleVersion(vendor, "maplibre-gl"); !strings.HasPrefix(v, "5.") {
		t.Errorf("maplibre-gl %s: the viewer loads it as a classic script, which 6.x (ESM only) breaks", v)
	}
}

func TestCatalogHasNoUnpinnedJS(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range cat.AllRecipes() {
		check := func(url, sha string) {
			if (strings.Contains(url, "unpkg.com") || strings.Contains(url, "jsdelivr") || strings.Contains(url, "registry.npmjs.org")) && sha == "" {
				t.Errorf("%s fetches %s without a sha256 pin", item.ID, url)
			}
		}
		check(item.URL, item.SHA256)
		for _, source := range item.Bundle {
			check(source.URL, source.SHA256)
		}
		if item.Build != nil {
			for _, asset := range item.Build.Assets {
				check(asset.URL, "")
			}
		}
	}
}
