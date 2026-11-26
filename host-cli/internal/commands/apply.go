package commands

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// ApplyVault loads the manifest, builds a reconciliation plan, executes it,
// and saves the updated manifest back to disk.
func ApplyVault(ctx context.Context, root string, cat *catalog.Catalog, onProgress ...apply.ProgressFunc) error {
	slog.Info("apply vault", "root", root)
	mPath := filepath.Join(root, "manifest.yaml")

	m, err := manifest.Load(mPath)
	if err != nil {
		return err
	}

	plan, _ := BuildPlan(m, cat)
	slog.Debug("manifest loaded", "desired", len(m.Desired.Items), "realized", len(m.Realized.Entries))

	applyErr := apply.RunWithCheckpoint(ctx, root, &m, plan, cat, func(snapshot manifest.Manifest) error {
		return manifest.Save(mPath, snapshot)
	}, onProgress...)
	saveErr := manifest.Save(mPath, m)
	if applyErr != nil || saveErr != nil {
		return errors.Join(applyErr, saveErr)
	}
	return nil
}
