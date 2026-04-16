package catalog

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

//go:embed testdata/recipes/*.yaml testdata/presets/*.yaml
var testData embed.FS

// NewTestCatalog creates a Catalog loaded from the embedded test fixtures.
// It calls t.Fatal on any error.
func NewTestCatalog(t *testing.T) *Catalog {
	t.Helper()

	recipesFS, err := fs.Sub(testData, "testdata/recipes")
	if err != nil {
		t.Fatalf("sub-fs for recipes: %v", err)
	}
	presetsFS, err := fs.Sub(testData, "testdata/presets")
	if err != nil {
		t.Fatalf("sub-fs for presets: %v", err)
	}

	cat, err := LoadFromFS(recipesFS, presetsFS)
	if err != nil {
		t.Fatalf("loading test catalog: %v", err)
	}
	return cat
}

// NewEmbeddedCatalog creates a Catalog from the embedded test fixtures without
// requiring a *testing.T. The embedded data contains only test fixtures and
// should be used as a fallback when the real recipes/presets directories are
// not available on disk (e.g. in compiled binaries without a catalog flag).
func NewEmbeddedCatalog() (*Catalog, error) {
	recipesFS, err := fs.Sub(testData, "testdata/recipes")
	if err != nil {
		return nil, fmt.Errorf("sub-fs for recipes: %w", err)
	}
	presetsFS, err := fs.Sub(testData, "testdata/presets")
	if err != nil {
		return nil, fmt.Errorf("sub-fs for presets: %w", err)
	}
	return LoadFromFS(recipesFS, presetsFS)
}

// NewDefaultCatalog loads recipes and presets from the repository root.
// This only works during development (go run/go test) — compiled binaries
// should use NewEmbeddedCatalog or provide catalog paths via flags.
// It locates the repo root relative to this source file using runtime.Caller,
// which will fail in compiled binaries.
func NewDefaultCatalog() (*Catalog, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("cannot determine source file path")
	}

	// thisFile is .../host-cli/internal/catalog/embed.go
	// repo root is three directories up.
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")

	recipesFS := os.DirFS(filepath.Join(repoRoot, "recipes"))
	presetsFS := os.DirFS(filepath.Join(repoRoot, "presets"))

	return LoadFromFS(recipesFS, presetsFS)
}

// NewCatalogFromPaths loads recipes and presets from explicit directory paths.
// Use this when the caller knows where the catalog data lives on disk (e.g.
// via a --catalog CLI flag).
func NewCatalogFromPaths(recipesDir, presetsDir string) (*Catalog, error) {
	recipesFS := os.DirFS(recipesDir)
	presetsFS := os.DirFS(presetsDir)
	return LoadFromFS(recipesFS, presetsFS)
}

// LoadCatalog tries NewDefaultCatalog first (repo-root recipes/presets); if
// that fails it falls back to the embedded test fixtures. This is the
// recommended entry point for CLI commands.
func LoadCatalog() (*Catalog, error) {
	cat, err := NewDefaultCatalog()
	if err == nil {
		return cat, nil
	}
	return NewEmbeddedCatalog()
}
