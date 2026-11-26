package toolkit

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func libEntry(id string) manifest.RealizedEntry {
	return manifest.RealizedEntry{ID: id, Type: catalog.LibraryType, Filename: id + ".zip", RelativePath: filepath.Join("lib", "js", id+".zip")}
}

func libRecipe(id string, libs ...catalog.Library) catalog.Item {
	item := catalog.Item{ID: id, Type: catalog.LibraryType, Libraries: libs}
	for _, lib := range libs {
		item.Bundle = append(item.Bundle, catalog.BundleSource{URL: "u", SHA256: "s", Dest: "node_modules/" + lib.Package})
	}
	return item
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
}

var testLibDocs = fstest.MapFS{
	"js-core/chart.js.md":                     {Data: []byte("# Chart.js quickstart\n\nimport Chart from \"chart.js/auto\";\n")},
	"js-core/chart.js.reference.md":           {Data: []byte("# chart.js reference\n")},
	"js-3d-cad/@jscad__modeling.md":           {Data: []byte("# JSCAD quickstart\n")},
	"js-3d-cad/@jscad__modeling.reference.md": {Data: []byte("# JSCAD reference\n")},
}

func TestWriteLibraryDocsPerRecipe(t *testing.T) {
	root := t.TempDir()
	chart := catalog.Library{Package: "chart.js", Version: "4.6.2", License: "MIT", Description: "Charts on a canvas", Entry: "chart.js/auto", Portability: "single-file", Quickstart: "chart.js.md"}
	recipes := map[string]catalog.Item{"js-core": libRecipe("js-core", chart)}
	if err := writeLibraryDocs(root, []manifest.RealizedEntry{libEntry("js-core")}, recipes, testLibDocs); err != nil {
		t.Fatal(err)
	}
	quickstart, err := os.ReadFile(filepath.Join(root, "lib", "js", "js-core", "docs", "chart.js.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nname: \"chart.js\"\ndescription: \"Charts on a canvas\"\npackage: \"chart.js\"\nversion: \"4.6.2\"\nlicense: \"MIT\"\nportability: \"single-file\"\nentry: \"chart.js/auto\"\nrequires: [js-core]\n---\n# Chart.js quickstart\n"
	if !strings.HasPrefix(string(quickstart), want) {
		t.Errorf("quickstart =\n%s\nwant it to start with\n%s", quickstart, want)
	}
	if ref, _ := os.ReadFile(filepath.Join(root, "lib", "js", "js-core", "docs", "chart.js.reference.md")); string(ref) != "# chart.js reference\n" {
		t.Errorf("reference = %q", ref)
	}

	// Unchanged docs are not written again; a stray file goes.
	docs := filepath.Join(root, "lib", "js", "js-core", "docs")
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(docs, "chart.js.md"), old, old)
	os.WriteFile(filepath.Join(docs, "stray.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(docs, ".DS_Store"), []byte("x"), 0o644)
	if err := writeLibraryDocs(root, []manifest.RealizedEntry{libEntry("js-core")}, recipes, testLibDocs); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(filepath.Join(docs, "chart.js.md")); !info.ModTime().Equal(old.Truncate(time.Second)) && info.ModTime().After(old.Add(time.Second)) {
		t.Errorf("an unchanged quickstart was written again (mtime %v)", info.ModTime())
	}
	if _, err := os.Stat(filepath.Join(docs, "stray.md")); !os.IsNotExist(err) {
		t.Error("a file Svalbard did not write stayed in docs/")
	}
	if _, err := os.Stat(filepath.Join(docs, ".DS_Store")); err != nil {
		t.Error("a hidden file was removed")
	}

	// Deselected: its folder goes, lib/js stays.
	os.MkdirAll(filepath.Join(root, "lib", "js", "js-gone", "docs"), 0o755)
	if err := writeLibraryDocs(root, []manifest.RealizedEntry{libEntry("js-core")}, recipes, testLibDocs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "lib", "js", "js-gone")); !os.IsNotExist(err) {
		t.Error("the docs of a recipe no longer realized stayed")
	}
	if err := writeLibraryDocs(root, nil, nil, testLibDocs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "lib", "js")); err != nil {
		t.Error("lib/js was removed with its last recipe")
	}
}

func TestWriteLibraryDocsScopedNames(t *testing.T) {
	root := t.TempDir()
	lib := catalog.Library{Package: "@jscad/modeling", Version: "2.13.0", Entry: "@jscad/modeling"}
	recipes := map[string]catalog.Item{"js-3d-cad": libRecipe("js-3d-cad", lib)}
	if err := writeLibraryDocs(root, []manifest.RealizedEntry{libEntry("js-3d-cad")}, recipes, testLibDocs); err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(root, "lib", "js", "js-3d-cad", "docs")
	for _, name := range []string{"@jscad__modeling.md", "@jscad__modeling.reference.md"} {
		if _, err := os.Stat(filepath.Join(docs, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(docs, "@jscad__modeling.md"))
	if !strings.Contains(string(data), "name: \"@jscad/modeling\"") {
		t.Errorf("quickstart = %s", data)
	}
}

func TestWriteLibraryDocsFallsBackToReadme(t *testing.T) {
	root := t.TempDir()
	writeZip(t, filepath.Join(root, "lib", "js", "js-mine.zip"), map[string]string{
		"node_modules/uplot/package.json": `{"version":"1.6.32","license":"MIT","description":"Fast charts"}`,
		"node_modules/uplot/README.md":    "# uPlot\n\nSmall fast charts.\n",
	})
	// A user recipe with no libraries: block.
	recipe := catalog.Item{ID: "js-mine", Type: catalog.LibraryType, Bundle: []catalog.BundleSource{{URL: "u", SHA256: "s", Dest: "node_modules/uplot"}}}
	if err := writeLibraryDocs(root, []manifest.RealizedEntry{libEntry("js-mine")}, map[string]catalog.Item{"js-mine": recipe}, testLibDocs); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "lib", "js", "js-mine", "docs", "uplot.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"version: \"1.6.32\"", "license: \"MIT\"", "description: \"Fast charts\"", "requires: [js-mine]", "# uPlot\n\nSmall fast charts."} {
		if !strings.Contains(string(data), want) {
			t.Errorf("fallback quickstart lacks %q:\n%s", want, data)
		}
	}
}
