package commands

import (
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/importer"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// ImportAndMaybeAdd imports a local file into the workspace library.
// If add is true it also appends the resulting id to the vault manifest's
// desired items list.
func ImportAndMaybeAdd(workspace string, source string, outputName string, add bool, vaultRoot string) (string, error) {
	id, err := importer.ImportLocalFile(workspace, source, outputName)
	if err != nil {
		return "", err
	}

	if add {
		mPath := filepath.Join(vaultRoot, "manifest.yaml")
		m, err := manifest.Load(mPath)
		if err != nil {
			return "", err
		}
		if err := AddItems(&m, []string{id}); err != nil {
			return "", err
		}
		if err := manifest.Save(mPath, m); err != nil {
			return "", err
		}
	}

	return id, nil
}
