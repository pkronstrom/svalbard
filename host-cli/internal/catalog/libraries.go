package catalog

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// LibraryType is the recipe type of browser JS libraries. Its bundle is
// installed as one archive, lib/js/<id>.zip, holding a trimmed node_modules
// tree that `svalbard-drive html pack` reads in place; each library's
// quickstart and reference file are written beside it, loose, under
// lib/js/<id>/docs/.
const LibraryType = "js-lib"

// Portability levels of a library, as measured by loading a packed page: it
// opens by double-click, it needs the drive's server, or it needs its
// directory of files. Empty means not measured.
const (
	PortabilitySingleFile = "single-file"
	PortabilityServed     = "served"
	PortabilityDirectory  = "directory"
)

// Library is one browser library of a js-lib recipe: a package a page
// imports. Its own bundle source is the one whose dest is
// node_modules/<Package>.
type Library struct {
	Package     string `yaml:"package"`
	Version     string `yaml:"version"`
	License     string `yaml:"license"`
	Description string `yaml:"description"`
	// Entry is how a page uses it: an ESM specifier (chart.js/auto) or the
	// global a classic build sets (window.mermaid).
	Entry string `yaml:"entry"`
	// Portability is the measured level, one of the Portability constants,
	// or empty until measured.
	Portability string `yaml:"portability,omitempty"`
	// Quickstart is the library's file under lib/js/<recipe>/docs/; it is
	// always LibraryDocName(Package).
	Quickstart string `yaml:"quickstart"`
	// Reference says where the library's reference file comes from: the
	// project's own Markdown docs at the pinned version and their licence,
	// or "README and types" where the project has none.
	Reference string `yaml:"reference,omitempty"`
}

// ArchiveSize is what a js-lib recipe's archive holds, as the pin generator
// built it: the number of files and the archive's size in bytes.
type ArchiveSize struct {
	Files int   `yaml:"files"`
	Bytes int64 `yaml:"bytes"`
}

// ExtractedSize is what one bundle source contributes once trimmed.
type ExtractedSize struct {
	Files int   `yaml:"files"`
	Bytes int64 `yaml:"bytes"`
}

// LibraryDocName is the file name of a library's quickstart: the package name
// with "/" as "__", so a scoped package stays one path segment
// (@jscad/modeling → @jscad__modeling.md).
func LibraryDocName(pkg string) string {
	return strings.ReplaceAll(pkg, "/", "__") + ".md"
}

// LibraryReferenceName is the file name of a library's reference file, which
// the drive groups with its quickstart.
func LibraryReferenceName(pkg string) string {
	return strings.TrimSuffix(LibraryDocName(pkg), ".md") + ".reference.md"
}

// TopLevelPackage returns the package a bundle source installs at the top of
// a js-lib recipe's tree (dest node_modules/<name>), or "" for a nested one.
func TopLevelPackage(source BundleSource) string {
	rest, ok := strings.CutPrefix(path.Clean(source.Dest), "node_modules/")
	if !ok || strings.Contains(rest, "/node_modules/") {
		return ""
	}
	if strings.HasPrefix(rest, "@") {
		if strings.Count(rest, "/") == 1 {
			return rest
		}
		return ""
	}
	if strings.Contains(rest, "/") {
		return ""
	}
	return rest
}

// validateLibraryRecipe checks one js-lib recipe on its own.
func validateLibraryRecipe(item Item) error {
	topLevel := map[string]bool{}
	for _, source := range item.Bundle {
		if pkg := TopLevelPackage(source); pkg != "" {
			topLevel[pkg] = true
		} else if !strings.HasPrefix(path.Clean(source.Dest), "node_modules/") {
			return fmt.Errorf("%s: bundle dest %q is not in node_modules/", item.ID, source.Dest)
		}
	}
	seen := map[string]bool{}
	for _, lib := range item.Libraries {
		switch lib.Portability {
		case "", PortabilitySingleFile, PortabilityServed, PortabilityDirectory:
		default:
			return fmt.Errorf("%s: library %s has portability %q; use %s, %s or %s", item.ID, lib.Package, lib.Portability, PortabilitySingleFile, PortabilityServed, PortabilityDirectory)
		}
		if seen[lib.Package] {
			return fmt.Errorf("%s: library %s is listed twice", item.ID, lib.Package)
		}
		seen[lib.Package] = true
		if !topLevel[lib.Package] {
			return fmt.Errorf("%s: library %s has no bundle source at node_modules/%s", item.ID, lib.Package, lib.Package)
		}
		if lib.Quickstart != "" && lib.Quickstart != LibraryDocName(lib.Package) {
			return fmt.Errorf("%s: library %s has quickstart %q, want %q", item.ID, lib.Package, lib.Quickstart, LibraryDocName(lib.Package))
		}
	}
	if len(item.Libraries) > 0 && item.Type != LibraryType {
		return fmt.Errorf("%s: libraries belong to recipes of type %s, not %s", item.ID, LibraryType, item.Type)
	}
	return nil
}

// ValidateLibraries checks the js-lib recipes among items: every library's
// portability is a known level and has its bundle source, and no package is
// a library of two recipes, since a page's import of it must resolve to one.
func ValidateLibraries(items []Item) error {
	owner := map[string]string{}
	sorted := append([]Item(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, item := range sorted {
		if item.Type != LibraryType && len(item.Libraries) == 0 {
			continue
		}
		if err := validateLibraryRecipe(item); err != nil {
			return err
		}
		for _, lib := range item.Libraries {
			if other, ok := owner[lib.Package]; ok {
				return fmt.Errorf("library %s is in two recipes, %s and %s: a page's import of it must resolve to one", lib.Package, other, item.ID)
			}
			owner[lib.Package] = item.ID
		}
	}
	return nil
}
