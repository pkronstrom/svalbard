package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"github.com/pkronstrom/svalbard/host-cli/internal/volumes"
	"github.com/pkronstrom/svalbard/tui"
)

// Version states of an installed item, comparing the acquisition hash
// recorded at install with its recipe's hash today.
const (
	VersionCurrent   = "current"   // installed from the recipe as it is now
	VersionUpdate    = "update"    // the recipe changed since install
	VersionUnknown   = "unknown"   // installed before hashes were recorded
	VersionWithdrawn = "withdrawn" // no longer in the catalog; the copy is kept
	VersionLocal     = "local"     // imported by the user
	VersionDocuments = "documents" // a collection; apply updates it by document
)

// ItemVersion returns the version state of the installed item whose realized
// entries are given. A multi-platform item has one entry per platform.
func ItemVersion(cat *catalog.Catalog, entries []manifest.RealizedEntry) string {
	if entries[0].SourceStrategy == "local" || strings.HasPrefix(entries[0].ID, "local:") {
		return VersionLocal
	}
	recipe, ok := cat.RecipeByID(entries[0].ID)
	if !ok {
		return VersionWithdrawn
	}
	if recipe.IsCollection() {
		return VersionDocuments
	}
	hash := recipe.AcquisitionHash()
	version := VersionCurrent
	for _, e := range entries {
		switch {
		case e.AcquisitionHash == "":
			return VersionUnknown
		case e.AcquisitionHash != hash:
			version = VersionUpdate
		}
	}
	return version
}

// realizedByID groups m's realized entries by item id.
func realizedByID(m manifest.Manifest) map[string][]manifest.RealizedEntry {
	byID := make(map[string][]manifest.RealizedEntry, len(m.Realized.Entries))
	for _, e := range m.Realized.Entries {
		byID[e.ID] = append(byID[e.ID], e)
	}
	return byID
}

// AvailableUpdates lists the desired, installed items whose recipe changed
// since they were installed, in desired order, with their catalog details.
func AvailableUpdates(cat *catalog.Catalog, m manifest.Manifest) []PlanItem {
	installed := realizedByID(m)
	updates := []PlanItem{}
	for _, id := range m.Desired.Items {
		entries := installed[id]
		if len(entries) == 0 || ItemVersion(cat, entries) != VersionUpdate {
			continue
		}
		recipe, _ := cat.RecipeByID(id)
		updates = append(updates, PlanItem{
			ID: id, Action: "update", Type: recipe.Type, Description: recipe.Description,
			SizeGB: recipe.SizeGBFor(m.Desired.Options.HostPlatforms),
		})
	}
	return updates
}

// UpdateVault acquires the installed items ids again from their current
// recipes and regenerates the vault's runtime files. Each item's installed
// copy is kept until its new one is in place and restored if that fails;
// with replace it is deleted first, for vaults without room for both.
func UpdateVault(ctx context.Context, root string, cat *catalog.Catalog, ids []string, replace bool, onProgress ...apply.ProgressFunc) error {
	slog.Info("update vault", "root", root, "ids", ids, "replace", replace)
	mPath := filepath.Join(root, "manifest.yaml")
	m, err := manifest.Load(mPath)
	if err != nil {
		return err
	}

	installed := realizedByID(m)
	var needGB float64
	for _, id := range ids {
		entries := installed[id]
		if len(entries) == 0 {
			return fmt.Errorf("%s is not installed; \"svalbard apply\" downloads pending items", id)
		}
		switch ItemVersion(cat, entries) {
		case VersionWithdrawn:
			return fmt.Errorf("%s is no longer in the catalog, so it cannot be updated; its installed copy is kept", id)
		case VersionLocal:
			return fmt.Errorf("%s is a local import; import the new file to replace it", id)
		case VersionDocuments:
			return fmt.Errorf("%s is a document collection; \"svalbard apply\" fetches its new and changed documents", id)
		}
		recipe, _ := cat.RecipeByID(id)
		if recipe.Type == "python-package" || recipe.Type == "python-venv" {
			return fmt.Errorf("%s is a Python tool; update it with \"svalbard remove %s\", apply, then add it back", id, id)
		}
		needGB += recipe.SizeGBFor(m.Desired.Options.HostPlatforms)
	}
	if freeGB := volumes.FreeSpaceGB(root); !replace && freeGB > 0 && needGB > freeGB {
		return fmt.Errorf("updating keeps each installed copy until its new one arrives, which needs about %s; %s is free. Update fewer items at once, or pass --replace to delete the installed copies first",
			tui.FormatSize(needGB), tui.FormatSize(freeGB))
	}

	err = runPlan(ctx, root, &m, planner.Plan{ToUpdate: ids, Replace: replace}, cat, onProgress...)
	if err != nil && !replace {
		return fmt.Errorf("%w\nthe installed copy of each item that failed was kept", err)
	}
	return err
}

// runPlan executes plan against the vault at root, saving the manifest after
// each completed item and once more at the end.
func runPlan(ctx context.Context, root string, m *manifest.Manifest, plan planner.Plan, cat *catalog.Catalog, onProgress ...apply.ProgressFunc) error {
	mPath := filepath.Join(root, "manifest.yaml")
	applyErr := apply.RunWithCheckpoint(ctx, root, m, plan, cat, func(snapshot manifest.Manifest) error {
		return manifest.Save(mPath, snapshot)
	}, onProgress...)
	saveErr := manifest.Save(mPath, *m)
	if applyErr != nil || saveErr != nil {
		return errors.Join(applyErr, saveErr)
	}
	return nil
}

// installedAsRecipe reports whether installed entries can be what the
// recipe installs today, as far as can be told without downloading. The
// recipe's sha256 pins the downloaded file, so it is compared only with an
// entry that is that file; a GeoPackage is installed unzipped from its pinned
// zip, so its entry's checksum is the GeoPackage's and is never compared with
// the pin, but it must be under data/ (a zip at the vault's root is the old
// layout, which only an update moves).
func installedAsRecipe(recipe catalog.Item, entries []manifest.RealizedEntry) bool {
	downloaded := recipe.Filename
	if downloaded == "" {
		downloaded = path.Base(strings.SplitN(recipe.URL, "?", 2)[0])
	}
	if recipe.Type == "gpkg" && strings.HasSuffix(strings.ToLower(downloaded), ".zip") {
		for _, e := range entries {
			if !strings.HasPrefix(filepath.ToSlash(e.RelativePath), "data/") || !strings.HasSuffix(strings.ToLower(e.Filename), ".gpkg") {
				return false
			}
		}
		return true
	}
	if pinned := recipe.SHA256; pinned != "" && len(entries) == 1 && entries[0].ChecksumSHA256 != "" && entries[0].Filename == downloaded {
		return strings.EqualFold(pinned, entries[0].ChecksumSHA256)
	}
	return true
}

// MarkCurrent records today's recipe as the version of installed items whose
// version is unknown, without downloading anything: the ids given, or every
// such item when ids is empty. An item whose recipe pins a checksum that the
// installed file does not have is returned in differing and left unknown.
func MarkCurrent(root string, cat *catalog.Catalog, ids []string) (marked, differing []string, err error) {
	mPath := filepath.Join(root, "manifest.yaml")
	m, err := manifest.Load(mPath)
	if err != nil {
		return nil, nil, err
	}
	installed := realizedByID(m)
	if len(ids) == 0 {
		for _, id := range m.Desired.Items {
			if entries := installed[id]; len(entries) > 0 && ItemVersion(cat, entries) == VersionUnknown {
				ids = append(ids, id)
			}
		}
	}
	for _, id := range ids {
		entries := installed[id]
		if len(entries) == 0 {
			return nil, nil, fmt.Errorf("%s is not installed", id)
		}
		if version := ItemVersion(cat, entries); version != VersionUnknown {
			return nil, nil, fmt.Errorf("%s is %s; only an item of unknown version can be marked current", id, versionLabels[version])
		}
		recipe, _ := cat.RecipeByID(id)
		if !installedAsRecipe(recipe, entries) {
			differing = append(differing, id)
			continue
		}
		hash := recipe.AcquisitionHash()
		for i := range m.Realized.Entries {
			if m.Realized.Entries[i].ID == id {
				m.Realized.Entries[i].AcquisitionHash = hash
			}
		}
		marked = append(marked, id)
	}
	if len(marked) == 0 {
		return marked, differing, nil
	}
	return marked, differing, manifest.Save(mPath, m)
}
