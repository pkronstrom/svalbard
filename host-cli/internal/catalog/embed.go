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

//go:embed embedded/recipes embedded/presets
var embeddedData embed.FS

//go:embed testdata/recipes/*.yaml testdata/presets/*.yaml
var testData embed.FS

func loadCatalogFromSubFS(data fs.FS, recipesDir, presetsDir string) (*Catalog, error) {
	recipesFS, err := fs.Sub(data, recipesDir)
	if err != nil {
		return nil, fmt.Errorf("sub-fs for %s: %w", recipesDir, err)
	}
	presetsFS, err := fs.Sub(data, presetsDir)
	if err != nil {
		return nil, fmt.Errorf("sub-fs for %s: %w", presetsDir, err)
	}
	return LoadFromFS(recipesFS, presetsFS)
}

// NewTestCatalog creates a Catalog loaded from the embedded test fixtures.
// It calls t.Fatal on any error.
func NewTestCatalog(t *testing.T) *Catalog {
	t.Helper()

	cat, err := loadCatalogFromSubFS(testData, "testdata/recipes", "testdata/presets")
	if err != nil {
		t.Fatalf("loading test catalog: %v", err)
	}
	return cat
}

// NewEmbeddedCatalog creates a Catalog from the embedded real catalog without
// requiring a *testing.T. This is the binary-friendly entry point and should
// be preferred by LoadCatalog.
func NewEmbeddedCatalog() (*Catalog, error) {
	return loadCatalogFromSubFS(embeddedData, "embedded/recipes", "embedded/presets")
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

// LoadCatalog prefers the embedded real catalog and falls back to the repo
// root during development if the embedded assets are unavailable.
func LoadCatalog() (*Catalog, error) {
	cat, err := NewEmbeddedCatalog()
	if err == nil {
		return cat, nil
	}
	cat, fallbackErr := NewDefaultCatalog()
	if fallbackErr == nil {
		return cat, nil
	}
	return nil, fmt.Errorf("embedded catalog: %w; default catalog: %v", err, fallbackErr)
}
