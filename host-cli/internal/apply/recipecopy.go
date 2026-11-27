package apply

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// A built-in recipe is embedded in every svalbard binary; a user recipe
// exists only in one computer's config folder. The vault therefore keeps a
// copy of each user recipe it holds, in .svalbard/recipes/<id>.yaml, which
// catalog.LoadVaultRecipes reads on a computer that has no definition. The
// config file stays the one the user edits; the copy follows it at every
// apply and is byte for byte the file, local paths as written.

// saveRecipeCopy writes the vault's copy of a user recipe. A recipe that was
// itself loaded from the vault's copy has nothing newer to write.
func saveRecipeCopy(root string, recipe catalog.Item) error {
	if recipe.Path == "" || recipe.VaultCopy {
		return nil
	}
	data, err := os.ReadFile(recipe.Path)
	if err != nil {
		return err
	}
	dir := catalog.VaultRecipeDir(root)
	dest := filepath.Join(dir, recipe.ID+".yaml")
	if existing, err := os.ReadFile(dest); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".recipe-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

func removeRecipeCopy(root, id string) error {
	err := os.Remove(filepath.Join(catalog.VaultRecipeDir(root), id+".yaml"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// syncRecipeCopies brings the vault's recipe copies in line with a completed
// apply: every desired or realized user recipe defined on this computer has a
// current copy, and a copy whose recipe the vault no longer wants or holds is
// removed.
func syncRecipeCopies(root string, m *manifest.Manifest, cat *catalog.Catalog) error {
	keep := make(map[string]bool)
	for _, id := range m.Desired.Items {
		keep[id] = true
	}
	for _, entry := range m.Realized.Entries {
		keep[entry.ID] = true
	}
	for id := range keep {
		// Built-in recipes have no Path and are skipped by saveRecipeCopy.
		if recipe, ok := cat.RecipeByID(id); ok && (recipe.IsCollection() || recipe.Type == catalog.LibraryType) {
			if err := saveRecipeCopy(root, recipe); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
		}
	}
	copies, err := os.ReadDir(catalog.VaultRecipeDir(root))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, file := range copies {
		id, isCopy := strings.CutSuffix(file.Name(), ".yaml")
		if file.IsDir() || !isCopy || keep[id] {
			continue
		}
		if err := removeRecipeCopy(root, id); err != nil {
			return err
		}
	}
	return nil
}

// recordCollection writes what the vault remembers about one collection as
// soon as its entries are saved: the recipe copy and the document titles.
// Both are written again when an apply completes; this covers the apply that
// fails on one document, after which the vault may be indexed by hand or
// carried to another computer. It is best effort: the index falls back to a
// file's own title, and the next apply here writes the copy.
func recordCollection(root string, entries []manifest.RealizedEntry, cat *catalog.Catalog, id string) {
	recipe, ok := cat.RecipeByID(id)
	if !ok || !recipe.IsCollection() {
		return
	}
	if err := saveRecipeCopy(root, recipe); err != nil {
		slog.Warn("writing the vault's recipe copy", "id", id, "error", err)
	}
	writeCollectionTitles(root, entries, recipe)
}
