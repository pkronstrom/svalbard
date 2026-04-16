package vault

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNoVault is returned when no vault can be found.
var ErrNoVault = errors.New("no vault found (no manifest.yaml in directory tree)")

// Resolve walks up from startDir looking for a directory containing manifest.yaml.
// Returns the absolute path to the vault root directory.
func Resolve(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		manifest := filepath.Join(dir, "manifest.yaml")
		if _, err := os.Stat(manifest); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding manifest.yaml.
			return "", ErrNoVault
		}
		dir = parent
	}
}
