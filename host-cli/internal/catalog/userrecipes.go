package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// collectionType is the recipe type of a user document collection. It decides
// the vault directory (books/) the collection is staged into.
const collectionType = "pdf"

var (
	userRecipeIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	sha256Re       = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// SkippedRecipe is a user recipe file that LoadUserRecipes left out of the
// catalog because it could not be read or failed validation.
type SkippedRecipe struct {
	ID     string // recipe id; the file name without .yaml if it did not parse
	Path   string // the recipe file (or directory, if that could not be read)
	Reason string
}

// String is the warning shown to the user: it names the file and the reason.
func (s SkippedRecipe) String() string {
	return fmt.Sprintf("skipped user recipe %s: %s", s.Path, s.Reason)
}

// Err explains why a command that names the skipped recipe cannot use it.
func (s SkippedRecipe) Err() error {
	return fmt.Errorf("user recipe %q was skipped (%s): %s", s.ID, s.Path, s.Reason)
}

// SkippedUserRecipes returns the user recipe files LoadUserRecipes skipped,
// in the order they were found.
func (c *Catalog) SkippedUserRecipes() []SkippedRecipe {
	return append([]SkippedRecipe(nil), c.skippedRecipes...)
}

// SkippedUserRecipe reports whether a user recipe with this id was skipped.
func (c *Catalog) SkippedUserRecipe(id string) (SkippedRecipe, bool) {
	for _, s := range c.skippedRecipes {
		if s.ID == id {
			return s, true
		}
	}
	return SkippedRecipe{}, false
}

// UserRecipeDir is where user-defined recipes live:
// $XDG_CONFIG_HOME/svalbard/recipes, defaulting to ~/.config/svalbard/recipes.
func UserRecipeDir() (string, error) {
	presets, err := UserPresetDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(presets), "recipes"), nil
}

// UserRecipeDirs returns every directory LoadCatalog reads user recipes from:
// the user recipe dir and, in development builds, the repo's gitignored
// local/recipes.
func UserRecipeDirs() []string {
	var dirs []string
	if dir, err := UserRecipeDir(); err == nil {
		dirs = append(dirs, dir)
	}
	if root, err := devRepoRoot(); err == nil {
		dirs = append(dirs, filepath.Join(root, "local", "recipes"))
	}
	return dirs
}

// VaultRecipeDir is where a vault keeps a copy of each user recipe it has
// realized, so the vault can still be planned and applied on a computer
// that does not have the definition.
func VaultRecipeDir(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".svalbard", "recipes")
}

// LoadUserRecipes adds the *.yaml recipes found in dirs (missing dirs are
// ignored) to the catalog. A user recipe is a document collection: it must
// list documents, and its id may not shadow a built-in recipe or another user
// recipe, also not by letter case, since both would share one folder on the
// vault. A recipe that fails is skipped rather than failing the load; the
// skipped files are returned and kept for SkippedUserRecipes. Local document
// paths are made absolute: "~" is the home directory and a relative path
// starts at the recipe file's directory.
func (c *Catalog) LoadUserRecipes(dirs ...string) []SkippedRecipe {
	var skipped []SkippedRecipe
	for _, dir := range dirs {
		_, bad := c.loadRecipeDir(dir, false)
		skipped = append(skipped, bad...)
	}
	c.skippedRecipes = append(c.skippedRecipes, skipped...)
	return skipped
}

// LoadVaultRecipes adds the recipe copies a vault keeps in dir for ids the
// catalog does not define, and returns those ids. A definition on this
// computer always wins over the vault's copy.
//
// A copy's local path: entries are kept as written and never read, not even
// when the path exists here. They name files of the computer that wrote the
// copy, and a vault is something people hand to each other: if apply read
// them, a prepared stick could list path: ~/.ssh/id_rsa and have this
// computer's file copied onto it. What was installed from such an entry
// stays on the vault and is reported; see ExpectedDocuments.
func (c *Catalog) LoadVaultRecipes(dir string) []string {
	used, skipped := c.loadRecipeDir(dir, true)
	c.skippedRecipes = append(c.skippedRecipes, skipped...)
	return used
}

// loadRecipeDir loads the user recipes of one directory and returns the ids
// added and the files skipped.
func (c *Catalog) loadRecipeDir(dir string, vaultCopy bool) (added []string, skipped []SkippedRecipe) {
	skip := func(id, path, reason string) {
		skipped = append(skipped, SkippedRecipe{ID: id, Path: path, Reason: reason})
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		skip("", dir, fmt.Sprintf("reading directory: %v", err))
		return nil, skipped
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fileName := strings.TrimSuffix(e.Name(), ".yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			skip(fileName, path, err.Error())
			continue
		}
		var item Item
		if err := yaml.Unmarshal(data, &item); err != nil {
			skip(fileName, path, fmt.Sprintf("invalid YAML: %v", err))
			continue
		}
		if item.ID == "" {
			item.ID = fileName
		}
		if existing, ok := c.recipeByFoldedID(item.ID); ok {
			switch {
			case vaultCopy:
				// The definition on this computer is the one in use.
			case existing.ID != item.ID:
				skip(item.ID, path, fmt.Sprintf("id %q differs only in letter case from the recipe %q; they would share one folder on the vault", item.ID, existing.ID))
			case existing.Path == "":
				skip(item.ID, path, fmt.Sprintf("id %q shadows a built-in recipe; pick a different id", item.ID))
			default:
				skip(item.ID, path, fmt.Sprintf("id %q is already defined in %s", item.ID, existing.Path))
			}
			continue
		}
		if reason := prepareUserRecipe(&item, dir, !vaultCopy); reason != "" {
			skip(item.ID, path, reason)
			continue
		}
		item.Path, item.VaultCopy = path, vaultCopy
		c.recipes[item.ID] = item
		added = append(added, item.ID)
	}
	return added, skipped
}

// recipeByFoldedID finds a recipe whose id equals id without regard to
// letter case. Vaults are usually exFAT, where such ids name the same folder.
func (c *Catalog) recipeByFoldedID(id string) (Item, bool) {
	if item, ok := c.recipes[id]; ok {
		return item, true
	}
	for existing, item := range c.recipes {
		if strings.EqualFold(existing, id) {
			return item, true
		}
	}
	return Item{}, false
}

// prepareUserRecipe validates a user recipe and, with resolve, makes its
// local paths absolute against baseDir. It returns why the recipe cannot be
// used, or "".
func prepareUserRecipe(item *Item, baseDir string, resolve bool) string {
	if !userRecipeIDRe.MatchString(item.ID) {
		return fmt.Sprintf("id %q may only hold letters, digits, '.', '_' and '-'", item.ID)
	}
	if len(item.Documents) == 0 {
		return "missing documents: a user recipe lists the documents of a collection"
	}
	if item.URL != "" || item.URLPattern != "" || len(item.Platforms) > 0 || len(item.Bundle) > 0 || item.Build != nil {
		return "a user recipe takes documents only, not url, url_pattern, platforms, bundle or build"
	}
	if item.Type == "" {
		item.Type = collectionType
	}
	if item.Type != collectionType {
		return fmt.Sprintf("type %q is not supported for a document collection; use type: %s", item.Type, collectionType)
	}
	if item.Strategy == "" {
		item.Strategy = "download"
	}

	names := map[string]bool{}
	for i := range item.Documents {
		doc := &item.Documents[i]
		entry := fmt.Sprintf("documents[%d]", i)
		switch {
		case doc.URL != "" && doc.Path != "":
			return entry + ": set url or path, not both"
		case doc.URL == "" && doc.Path == "":
			return entry + ": needs a url or a path"
		}
		if doc.SHA256 != "" && !sha256Re.MatchString(doc.SHA256) {
			return entry + ": sha256 must be 64 hex characters"
		}
		if doc.Filename != "" && !portableName(doc.Filename) {
			return fmt.Sprintf("%s: filename %q must be a plain file name without folders or any of <>:\"/\\|?*", entry, doc.Filename)
		}
		name := doc.Filename
		if doc.URL != "" {
			u, err := url.Parse(doc.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return fmt.Sprintf("%s: url %q must start with http:// or https://", entry, doc.URL)
			}
			if name, err = documentURLName(*doc); err != nil {
				return entry + ": " + err.Error()
			}
		} else {
			if resolve {
				resolved, err := resolveUserPath(doc.Path, baseDir)
				if err != nil {
					return fmt.Sprintf("%s: %v", entry, err)
				}
				doc.Path = resolved
			}
			if name == "" {
				// A folder's contents are only known when it is read; its own
				// name is still the first path segment they are saved under.
				name = filepath.Base(filepath.Clean(doc.Path))
			}
			if !portableName(name) {
				return fmt.Sprintf("%s: path %q ends in a name that cannot be stored on every drive", entry, doc.Path)
			}
		}
		key := strings.ToLower(name)
		if names[key] {
			return fmt.Sprintf("%s: another document is already saved as %s; set filename: on one of them", entry, name)
		}
		names[key] = true
	}
	return ""
}

// resolveUserPath expands a leading "~" and anchors a relative path at baseDir.
func resolveUserPath(p, baseDir string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locating home directory: %w", err)
		}
		p = filepath.Join(home, strings.TrimPrefix(p[1:], "/"))
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	return filepath.Clean(p), nil
}
