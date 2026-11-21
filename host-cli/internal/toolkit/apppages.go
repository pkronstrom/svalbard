package toolkit

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// appPages holds svalbard-owned entry pages, one directory per app recipe ID,
// for apps whose upstream package ships no page of its own (duckdb-wasm).
//
//go:embed apppages
var appPages embed.FS

// writeAppPages copies apppages/<id>/ into each realized app directory. Pages
// are rewritten on every apply, so a page fix ships with the CLI, and they
// replace any file of the same name that came from the app's archive.
func writeAppPages(root string, entries []manifest.RealizedEntry) error {
	for _, entry := range entries {
		if entry.Type != "app" {
			continue
		}
		pages, err := fs.Sub(appPages, path.Join("apppages", entry.ID))
		if err != nil {
			continue
		}
		if _, err := fs.Stat(pages, "."); err != nil {
			continue // no svalbard page for this app
		}
		appDir := filepath.Join(root, entry.RelativePath)
		if info, err := os.Stat(appDir); err != nil || !info.IsDir() {
			continue // realized as a file or missing: nothing to put a page into
		}
		err = fs.WalkDir(pages, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			dest := filepath.Join(appDir, filepath.FromSlash(p))
			if d.IsDir() {
				return os.MkdirAll(dest, 0o755)
			}
			data, err := fs.ReadFile(pages, p)
			if err != nil {
				return err
			}
			return os.WriteFile(dest, data, 0o644)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
