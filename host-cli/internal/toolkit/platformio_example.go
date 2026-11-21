package toolkit

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

//go:embed platformio-example
var platformIOExample embed.FS

// Example templates are regenerated with the toolkit. User working copies
// belong elsewhere and are never touched by this function.
func writePlatformIOExample(root string, entries []manifest.RealizedEntry) error {
	installed := false
	for _, entry := range entries {
		if entry.ID == "platformio-espressif32" {
			installed = true
			break
		}
	}
	if !installed {
		return nil
	}
	return fs.WalkDir(platformIOExample, "platformio-example", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("platformio-example", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(root, "tools", "platformio", "examples", "blink", rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := platformIOExample.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}
