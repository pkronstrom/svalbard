package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/importer"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// ImportAndMaybeAdd imports a local file into the workspace library.
// If add is true it also appends the resulting id to the vault manifest's
// desired items list and records a realized entry for it, so plan/apply
// treat the import as reconciled instead of an unknown recipe to download.
func ImportAndMaybeAdd(workspace string, source string, outputName string, add bool, vaultRoot string) (string, error) {
	id, destPath, err := importer.ImportLocalFile(workspace, source, outputName)
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

		info, err := os.Stat(destPath)
		if err != nil {
			return "", fmt.Errorf("stat imported file: %w", err)
		}
		// Both call sites pass workspace == vaultRoot; Rel against workspace
		// keeps the path clean ("library/<file>") either way.
		rel, err := filepath.Rel(workspace, destPath)
		if err != nil {
			return "", err
		}
		upsertRealized(&m, manifest.RealizedEntry{
			ID:             id,
			Type:           strings.TrimPrefix(filepath.Ext(destPath), "."),
			Filename:       filepath.Base(destPath),
			RelativePath:   rel,
			SizeBytes:      info.Size(),
			SourceStrategy: "local",
		})

		if err := manifest.Save(mPath, m); err != nil {
			return "", err
		}
	}

	return id, nil
}

// upsertRealized replaces any realized entries sharing an id with the given
// entries, then appends them.
func upsertRealized(m *manifest.Manifest, entries ...manifest.RealizedEntry) {
	replaced := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		replaced[e.ID] = struct{}{}
	}
	kept := m.Realized.Entries[:0]
	for _, e := range m.Realized.Entries {
		if _, ok := replaced[e.ID]; !ok {
			kept = append(kept, e)
		}
	}
	m.Realized.Entries = append(kept, entries...)
}
