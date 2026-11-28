package toolkit

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestGenerateWritesAppPageIntoRealizedApp(t *testing.T) {
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "duckdb-wasm")
	if err := os.MkdirAll(filepath.Join(appDir, "duckdb"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A stale page from an earlier CLI is replaced.
	if err := os.WriteFile(filepath.Join(appDir, "index.html"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	otherDir := filepath.Join(root, "apps", "other-app")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []manifest.RealizedEntry{
		{ID: "duckdb-wasm", Type: "app", Filename: "duckdb-wasm", RelativePath: "apps/duckdb-wasm"},
		{ID: "other-app", Type: "app", Filename: "other-app", RelativePath: "apps/other-app"},
	}
	if err := Generate(root, entries, "default", GenerateOpts{}); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"index.html", "arrow-shim.mjs"} {
		want, err := appPages.ReadFile(path.Join("apppages", "duckdb-wasm", name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(appDir, name))
		if err != nil {
			t.Fatalf("page file %s not written: %v", name, err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s differs from the embedded page", name)
		}
	}
	if left, _ := os.ReadDir(otherDir); len(left) != 0 {
		t.Fatalf("app without a svalbard page got files: %v", left)
	}
}

func TestGenerateSkipsAppPageWhenAppDirectoryIsMissing(t *testing.T) {
	stubRuntimeBinarySources(t)
	root := t.TempDir()
	entries := []manifest.RealizedEntry{{ID: "duckdb-wasm", Type: "app", Filename: "duckdb-wasm", RelativePath: "apps/duckdb-wasm"}}
	if err := Generate(root, entries, "default", GenerateOpts{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "apps", "duckdb-wasm")); !os.IsNotExist(err) {
		t.Fatalf("page created an app directory the apply did not realize: %v", err)
	}
}

func TestOfflineAppPagesMatchTheirRecipeFiles(t *testing.T) {
	cat, err := catalog.NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"cyberchef", "mml-map-sheets"} {
		t.Run(id, func(t *testing.T) {
			recipe, ok := cat.RecipeByID(id)
			if !ok {
				t.Fatal("recipe missing")
			}
			page, err := appPages.ReadFile(path.Join("apppages", id, "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			if id == "cyberchef" {
				if len(recipe.Bundle) != 1 || recipe.Bundle[0].SHA256 == "" ||
					!strings.Contains(string(page), `href="CyberChef_v11.6.0.html"`) {
					t.Fatal("CyberChef needs an unpacked, pinned bundle and its HTML entry point")
				}
				return
			}
			for _, asset := range recipe.Build.Assets {
				if !strings.Contains(string(page), `href="`+asset.Dest+`"`) {
					t.Errorf("map sheet %s is missing from the page", asset.Dest)
				}
			}
		})
	}
}

// The DuckDB page loads files by relative path; the catalog recipe's bundle
// must provide each of them, so bumping one without the other fails here.
func TestDuckDBPageFilesComeFromRecipeBundle(t *testing.T) {
	cat, err := catalog.NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	recipe, ok := cat.RecipeByID("duckdb-wasm")
	if !ok {
		t.Fatal("duckdb-wasm recipe missing")
	}
	provided := map[string]bool{}
	for _, source := range recipe.Bundle {
		if source.SHA256 == "" {
			t.Errorf("%s is not pinned", source.URL)
		}
		for _, file := range source.Files {
			provided[path.Join(source.Dest, file)] = true
		}
		if len(source.Files) == 0 {
			provided[path.Join(source.Dest, path.Base(source.URL))] = true
		}
	}
	page, err := appPages.ReadFile("apppages/duckdb-wasm/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []struct{ inPage, file string }{
		{`"arrow/Arrow.esnext.min.js"`, "arrow/Arrow.esnext.min.js"},
		{`"./duckdb/duckdb-browser.mjs"`, "duckdb/duckdb-browser.mjs"},
		{`"duckdb-browser-eh.worker.js"`, "duckdb/duckdb-browser-eh.worker.js"},
		{`"duckdb-eh.wasm"`, "duckdb/duckdb-eh.wasm"},
	} {
		if !strings.Contains(string(page), ref.inPage) {
			t.Errorf("index.html no longer references %s; update this test", ref.inPage)
		}
		if !provided[ref.file] {
			t.Errorf("recipe bundle does not provide %s", ref.file)
		}
	}
	for _, ext := range []string{"json", "parquet"} {
		found := false
		for file := range provided {
			if strings.HasPrefix(file, "extensions/") && strings.HasSuffix(file, "/"+ext+".duckdb_extension.wasm") {
				found = true
			}
		}
		if !found {
			t.Errorf("recipe bundle has no %s extension under extensions/", ext)
		}
	}
}
