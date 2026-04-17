package commands

import (
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
)

// ApplyVault loads the manifest, builds a reconciliation plan, executes it,
// and saves the updated manifest back to disk.
func ApplyVault(root string, cat *catalog.Catalog, onProgress ...apply.ProgressFunc) error {
	mPath := filepath.Join(root, "manifest.yaml")

	m, err := manifest.Load(mPath)
	if err != nil {
		return err
	}

	plan := planner.Build(m)

	if err := apply.Run(root, &m, plan, cat, onProgress...); err != nil {
		return err
	}

	return manifest.Save(mPath, m)
}
