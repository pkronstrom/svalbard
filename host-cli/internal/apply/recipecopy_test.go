package apply

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// userCatalog loads the user recipes of dir and then, like the CLI, the
// copies the vault keeps for ids dir does not define.
func userCatalog(t *testing.T, dir, vaultRoot string) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.LoadFromFS(fstest.MapFS{}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if skipped := cat.LoadUserRecipes(dir); len(skipped) > 0 {
		t.Fatalf("recipes skipped: %v", skipped)
	}
	cat.LoadVaultRecipes(catalog.VaultRecipeDir(vaultRoot))
	if skipped := cat.SkippedUserRecipes(); len(skipped) > 0 {
		t.Fatalf("vault copies skipped: %v", skipped)
	}
	return cat
}

func TestRecipeCopyFollowsTheDefinition(t *testing.T) {
	config, root := t.TempDir(), t.TempDir()
	definition := "# our manuals\ndescription: Manuals\ndocuments:\n  - url: https://example.test/a.pdf\n  - path: ~/Documents/manuals/workshop/\n  - path: notes/trailer.pdf\n"
	writeFile(t, filepath.Join(config, "manuals.yaml"), definition)
	writeFile(t, filepath.Join(config, "other.yaml"), "documents:\n  - url: https://example.test/o.pdf\n")
	copyPath := filepath.Join(catalog.VaultRecipeDir(root), "manuals.yaml")
	m := manifest.New("test")
	m.Desired.Items = []string{"manuals"}

	if err := syncRecipeCopies(root, &m, userCatalog(t, config, root)); err != nil {
		t.Fatal(err)
	}
	// The copy is the file, comments and local paths as written.
	if got, err := os.ReadFile(copyPath); err != nil || string(got) != definition {
		t.Fatalf("copy = %q, %v; want the definition byte for byte", got, err)
	}
	if got := vaultFiles(t, root, ".svalbard"); !reflect.DeepEqual(got, []string{".svalbard/recipes/manuals.yaml"}) {
		t.Fatalf("vault files = %v, want a copy of the desired recipe only", got)
	}

	// An edit on the computer reaches the copy at the next apply.
	edited := definition + "  - url: https://example.test/b.pdf\n"
	writeFile(t, filepath.Join(config, "manuals.yaml"), edited)
	if err := syncRecipeCopies(root, &m, userCatalog(t, config, root)); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(copyPath); string(got) != edited {
		t.Fatalf("copy after the edit = %q", got)
	}

	// On a computer without the definition the copy is used and left as is.
	elsewhere := t.TempDir()
	cat := userCatalog(t, elsewhere, root)
	recipe, ok := cat.RecipeByID("manuals")
	if !ok || !recipe.VaultCopy {
		t.Fatalf("recipe on another computer = %+v, %v", recipe, ok)
	}
	if err := syncRecipeCopies(root, &m, cat); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(copyPath); string(got) != edited {
		t.Fatalf("copy after an apply elsewhere = %q", got)
	}

	// A recipe the vault neither wants nor holds has no copy.
	m.Desired.Items = nil
	if err := syncRecipeCopies(root, &m, cat); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
		t.Fatalf("copy of a removed collection left behind: %v", err)
	}
}

// A vault's recipe copy was written on another computer. Its path: entries
// name that computer's files: what was installed from them stays, and
// nothing is ever read from this computer on the copy's say-so.
func TestAssembleDocumentsFromVaultCopy(t *testing.T) {
	server := newDocumentServer(t, map[string]string{"/a.pdf": fakePDF("alpha"), "/b.pdf": fakePDF("bravo")})
	root, here := t.TempDir(), t.TempDir()
	private := filepath.Join(here, "private.txt")
	writeFile(t, private, "a file of this computer")
	writeFile(t, filepath.Join(here, "workshop", "brakes.pdf"), fakePDF("this computer's brakes"))
	writeFile(t, filepath.Join(catalog.VaultRecipeDir(root), "manuals.yaml"), "documents:\n"+
		"  - url: "+server.URL+"/a.pdf\n"+
		"  - url: "+server.URL+"/b.pdf\n"+
		"  - path: "+filepath.Join(here, "workshop")+"\n"+
		"  - path: "+private+"\n"+
		"  - path: ~/Documents/trailer.pdf\n")
	installed := map[string]string{"a.pdf": fakePDF("alpha"), "workshop/brakes.pdf": fakePDF("brakes"), "trailer.pdf": fakePDF("trailer")}
	var realized []manifest.RealizedEntry
	for rel, body := range installed {
		writeFile(t, filepath.Join(root, "books", "manuals", filepath.FromSlash(rel)), body)
		realized = append(realized, manifest.RealizedEntry{ID: "manuals", Type: "pdf", Filename: filepath.Base(rel),
			RelativePath: filepath.Join("books", "manuals", filepath.FromSlash(rel)), SizeBytes: int64(len(body)), ChecksumSHA256: sha(body)})
	}

	recipe, _ := userCatalog(t, t.TempDir(), root).RecipeByID("manuals")
	set, err := catalog.ExpectedDocuments(recipe)
	if err != nil {
		t.Fatal(err)
	}
	diff := DiffDocuments(recipe, set, realized)
	if diff.Added != 1 || diff.Removed != 0 || len(diff.Kept) != 2 || len(diff.Missing) != 1 || !diff.Any() {
		t.Fatalf("diff = %+v, want b.pdf to add, two documents kept, private.txt out of reach", diff)
	}
	entries, err := assembleDocuments(context.Background(), root, "manuals", recipe, realized, nil)
	if err != nil {
		t.Fatalf("a path of another computer failed the item: %v", err)
	}
	want := []string{"books/manuals/a.pdf", "books/manuals/b.pdf", "books/manuals/trailer.pdf", "books/manuals/workshop/brakes.pdf"}
	if got := vaultFiles(t, root, "books"); !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	if got := entryPaths(entries); !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	if got := server.takeRequests(); !reflect.DeepEqual(got, map[string]int{"/b.pdf": 1}) {
		t.Fatalf("requests = %v, want only the missing URL document", got)
	}
	// The installed copy is not replaced by this computer's file of the same
	// name, and this computer's private file is not carried onto the vault.
	if got, _ := os.ReadFile(filepath.Join(root, "books", "manuals", "workshop", "brakes.pdf")); string(got) != fakePDF("brakes") {
		t.Fatalf("installed document was replaced from this computer: %q", got)
	}
	// With only unreachable sources left to wait for, there is nothing to do.
	if diff := DiffDocuments(recipe, set, entries); diff.Any() {
		t.Fatalf("diff after the apply = %+v, want nothing to do", diff)
	}
}

func TestRunKeepsRecipeCopyOfAFailedCollectionAndRemovesItWithTheCollection(t *testing.T) {
	server := newDocumentServer(t, map[string]string{"/good.pdf": fakePDF("good")})
	config, root := t.TempDir(), t.TempDir()
	definition := "documents:\n  - url: " + server.URL + "/good.pdf\n  - url: " + server.URL + "/gone.pdf\n"
	writeFile(t, filepath.Join(config, "manuals.yaml"), definition)
	copyPath := filepath.Join(catalog.VaultRecipeDir(root), "manuals.yaml")
	m := manifest.New("test")
	m.Desired.Items = []string{"manuals"}

	// The apply fails on one document; the vault still gets the recipe, so
	// it can be finished from another computer.
	err := Run(context.Background(), root, &m, planner.Plan{ToDownload: []string{"manuals"}}, userCatalog(t, config, root))
	if err == nil || !strings.Contains(err.Error(), "/gone.pdf") {
		t.Fatalf("err = %v, want the dead URL", err)
	}
	if got, err := os.ReadFile(copyPath); err != nil || string(got) != definition {
		t.Fatalf("copy after the failed apply = %q, %v", got, err)
	}

	// Removing the collection removes its copy, also when a later step of
	// the same apply fails.
	m.Desired.Items = []string{"missing-recipe"}
	err = Run(context.Background(), root, &m, planner.Plan{ToRemove: []string{"manuals"}, ToDownload: []string{"missing-recipe"}}, userCatalog(t, t.TempDir(), root))
	if err == nil {
		t.Fatal("apply with an unknown recipe succeeded")
	}
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
		t.Fatalf("copy of the removed collection left behind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "books", "manuals")); !os.IsNotExist(err) {
		t.Fatalf("books/manuals left behind: %v", err)
	}
}
