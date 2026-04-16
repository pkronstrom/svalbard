package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

const manifestFilename = "manifest.yaml"

// FindRoot locates the vault root directory.
//
// If explicit is non-empty it is treated as the vault path and returned as an
// absolute path. Otherwise the function walks from cwd upward looking for a
// directory that contains manifest.yaml. It returns the directory containing
// the manifest, or an error if the filesystem root is reached without finding
// one.
func FindRoot(explicit string, cwd string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}

	dir, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, manifestFilename)
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root.
			return "", fmt.Errorf("no %s found walking from %s to filesystem root", manifestFilename, cwd)
		}
		dir = parent
	}
}
