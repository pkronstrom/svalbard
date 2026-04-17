package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	hosttui "github.com/pkronstrom/svalbard/host-tui"
	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/tui"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/pkronstrom/svalbard/host-cli/internal/volumes"
	"github.com/spf13/cobra"
)

// NewRootCommand returns the top-level svalbard CLI command with all
// hard-reset subcommands registered.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "svalbard",
		Short: "Provision and reconcile offline knowledge vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			// No subcommand → launch interactive TUI dashboard
			vaultFlag, _ := cmd.Flags().GetString("vault")
			config, err := buildWizardConfig("")
			if err != nil {
				// Degrade gracefully — launch TUI without wizard data
				return hosttui.RunInteractive(vaultFlag, nil, nil)
			}
			deps := buildDashboardDeps(vaultFlag, &config)
			return hosttui.RunInteractive(vaultFlag, &config, deps)
		},
	}

	root.PersistentFlags().String("vault", "", "path to vault root (default: auto-detect from cwd)")

	addCmd := &cobra.Command{
		Use:   "add <item...>",
		Short: "Add items to the desired state",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			mPath := filepath.Join(vaultRoot, "manifest.yaml")
			m, err := manifest.Load(mPath)
			if err != nil {
				return err
			}
			if err := commands.AddItems(&m, args); err != nil {
				return err
			}
			if err := manifest.Save(mPath, m); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %d item(s) to desired state\n", len(args))
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <item...>",
		Short: "Remove items from the desired state",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			mPath := filepath.Join(vaultRoot, "manifest.yaml")
			m, err := manifest.Load(mPath)
			if err != nil {
				return err
			}
			if err := commands.RemoveItems(&m, args); err != nil {
				return err
			}
			if err := manifest.Save(mPath, m); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d item(s) from desired state\n", len(args))
			return nil
		},
	}

	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Show reconciliation plan between desired and realized state",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			mPath := filepath.Join(vaultRoot, "manifest.yaml")
			m, err := manifest.Load(mPath)
			if err != nil {
				return err
			}
			p := planner.Build(m)
			return commands.WritePlan(cmd.OutOrStdout(), p)
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show vault state — desired, realized, and pending items",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			mPath := filepath.Join(vaultRoot, "manifest.yaml")
			m, err := manifest.Load(mPath)
			if err != nil {
				return err
			}
			return commands.WriteStatus(cmd.OutOrStdout(), m)
		},
	}

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Execute reconciliation plan and update realized state",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			cat, err := catalog.LoadCatalog()
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}
			if err := commands.ApplyVault(cmd.Context(), vaultRoot, cat); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Apply complete")
			return nil
		},
	}

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Build full-text search index from ZIM files in the vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			return commands.IndexVault(vaultRoot, false, cmd.OutOrStdout(), nil)
		},
	}

	root.AddCommand(
		newInitCommand(),
		addCmd,
		removeCmd,
		planCmd,
		applyCmd,
		statusCmd,
		newImportCommand(),
		newPresetCommand(),
		indexCmd,
	)

	return root
}

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Open the guided vault setup wizard",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefill := ""
			if len(args) > 0 {
				abs, err := filepath.Abs(args[0])
				if err != nil {
					return fmt.Errorf("resolving path: %w", err)
				}
				prefill = abs
			}

			config, err := buildWizardConfig(prefill)
			if err != nil {
				return fmt.Errorf("preparing wizard: %w", err)
			}
			deps := buildDashboardDeps("", &config)
			config.InitVault = deps.InitVault
			if deps.RunApply != nil && deps.RebuildForVault != nil {
				rebuildForVault := deps.RebuildForVault
				config.RunApply = func(vaultPath string, onProgress func(hosttui.WizardApplyEvent)) error {
					newDeps := rebuildForVault(vaultPath)
					return newDeps.RunApply(context.Background(), func(ev hosttui.ApplyEvent) {
						onProgress(hosttui.WizardApplyEvent{ID: ev.ID, Status: ev.Status, Error: ev.Error})
					})
				}
			}
			return hosttui.RunInitWizard(config)
		},
	}

	return cmd
}

func newImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <input>",
		Short: "Import a local file into the vault library",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			addFlag, _ := cmd.Flags().GetBool("add")
			nameFlag, _ := cmd.Flags().GetString("name")

			source, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolving source path: %w", err)
			}

			id, err := commands.ImportAndMaybeAdd(vaultRoot, source, nameFlag, addFlag, vaultRoot)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported %s\n", id)
			return nil
		},
	}

	cmd.Flags().Bool("add", false, "also add the imported item to the desired state")
	cmd.Flags().String("name", "", "override the output name (used to derive the local: id)")

	return cmd
}

func newPresetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Manage presets",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := catalog.LoadCatalog()
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}
			return commands.WritePresetList(cmd.OutOrStdout(), cat)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "copy <source> <target>",
		Short: "Copy a preset to a local file for customization",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := catalog.LoadCatalog()
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}
			if err := commands.CopyPreset(cat, args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Copied preset %q to %s\n", args[0], args[1])
			return nil
		},
	})

	return cmd
}

// buildWizardConfig loads the catalog and detected volumes to prepare
// a WizardConfig for the interactive init wizard.
func buildWizardConfig(prefillPath string) (hosttui.WizardConfig, error) {
	cat, err := catalog.LoadCatalog()
	if err != nil {
		return hosttui.WizardConfig{}, fmt.Errorf("loading catalog: %w", err)
	}

	// Detect volumes
	vols := volumes.Detect()
	var wizVols []hosttui.Volume
	for _, v := range vols {
		wizVols = append(wizVols, hosttui.Volume{
			Path:    v.Path,
			Name:    v.Name,
			TotalGB: v.TotalGB,
			FreeGB:  v.FreeGB,
			Network: v.Network,
		})
	}

	home := volumes.HomeSvalbardVolume()
	homeVol := hosttui.Volume{
		Path:    home.Path,
		Name:    home.Name,
		TotalGB: home.TotalGB,
		FreeGB:  home.FreeGB,
	}

	// Build preset options for all regions
	var presetOpts []hosttui.PresetOption
	for _, region := range cat.Regions() {
		for _, p := range cat.PresetsForRegion(region) {
			resolved, err := cat.ResolvePreset(p.Name)
			if err != nil {
				continue
			}
			presetOpts = append(presetOpts, hosttui.PresetOption{
				Name:         p.Name,
				Description:  p.Description,
				ContentGB:    resolved.ContentSizeGB(),
				TargetSizeGB: p.TargetSizeGB,
				Region:       p.Region,
				SourceIDs:    resolved.Sources,
			})
		}
	}

	// Build pack groups
	groupMap := make(map[string]*hosttui.PackGroup)
	for _, p := range cat.Packs() {
		resolved, err := cat.ResolvePreset(p.Name)
		if err != nil {
			continue
		}
		pg, ok := groupMap[p.DisplayGroup]
		if !ok {
			pg = &hosttui.PackGroup{Name: p.DisplayGroup}
			groupMap[p.DisplayGroup] = pg
		}
		pack := hosttui.Pack{
			Name:        p.Name,
			Description: p.Description,
		}
		for _, item := range resolved.Items {
			pack.Sources = append(pack.Sources, hosttui.PackSource{
				ID:          item.ID,
				Type:        item.Type,
				Strategy:    item.Strategy,
				Description: item.Description,
				SizeGB:      item.SizeGB,
			})
		}
		pg.Packs = append(pg.Packs, pack)
	}

	// Collect IDs that are in packs.
	inPack := make(map[string]bool)
	for _, pg := range groupMap {
		for _, pack := range pg.Packs {
			for _, src := range pack.Sources {
				inPack[src.ID] = true
			}
		}
	}

	// Add individual recipes that aren't in any pack.
	var loose []hosttui.PackSource
	for _, item := range cat.AllRecipes() {
		if inPack[item.ID] {
			continue
		}
		loose = append(loose, hosttui.PackSource{
			ID:          item.ID,
			Type:        item.Type,
			Strategy:    item.Strategy,
			Description: item.Description,
			SizeGB:      item.SizeGB,
		})
	}
	if len(loose) > 0 {
		pg, ok := groupMap["Other"]
		if !ok {
			pg = &hosttui.PackGroup{Name: "Other"}
			groupMap["Other"] = pg
		}
		pg.Packs = append(pg.Packs, hosttui.Pack{
			Name:        "individual-items",
			Description: "Recipes not included in any pack",
			Sources:     loose,
		})
	}

	var packGroups []hosttui.PackGroup
	for _, pg := range groupMap {
		packGroups = append(packGroups, *pg)
	}
	sort.Slice(packGroups, func(i, j int) bool {
		return packGroups[i].Name < packGroups[j].Name
	})

	return hosttui.WizardConfig{
		Volumes:     wizVols,
		HomeVolume:  homeVol,
		Presets:     presetOpts,
		Regions:     cat.Regions(),
		PackGroups:  packGroups,
		PrefillPath: prefillPath,
	}, nil
}

// buildDashboardDeps constructs callback closures that bridge host-cli
// business logic into the TUI screens without the TUI importing host-cli.
func buildDashboardDeps(vaultFlag string, wizConfig *hosttui.WizardConfig) *hosttui.DashboardDeps {
	deps := &hosttui.DashboardDeps{}

	if wizConfig != nil {
		deps.PackGroups = wizConfig.PackGroups
		deps.Presets = wizConfig.Presets
	}

	deps.RebuildForVault = func(vaultPath string) *hosttui.DashboardDeps {
		return buildDashboardDeps(vaultPath, wizConfig)
	}

	deps.LoadStatus = func() (hosttui.VaultStatus, error) {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return hosttui.VaultStatus{}, err
		}
		mPath := filepath.Join(root, "manifest.yaml")
		m, err := manifest.Load(mPath)
		if err != nil {
			return hosttui.VaultStatus{}, err
		}

		realizedByID := make(map[string]bool, len(m.Realized.Entries))
		for _, e := range m.Realized.Entries {
			realizedByID[e.ID] = true
		}
		realized := 0
		for _, id := range m.Desired.Items {
			if realizedByID[id] {
				realized++
			}
		}

		presetName := ""
		if len(m.Desired.Presets) > 0 {
			presetName = m.Desired.Presets[0]
		}

		return hosttui.VaultStatus{
			VaultPath:     root,
			VaultName:     m.Vault.Name,
			PresetName:    presetName,
			DesiredCount:  len(m.Desired.Items),
			RealizedCount: realized,
			PendingCount:  len(m.Desired.Items) - realized,
			LastApplied:   m.Realized.AppliedAt,
		}, nil
	}

	deps.LoadDesiredItems = func() ([]string, error) {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return nil, err
		}
		mPath := filepath.Join(root, "manifest.yaml")
		m, err := manifest.Load(mPath)
		if err != nil {
			return nil, err
		}
		return m.Desired.Items, nil
	}

	deps.LoadPlan = func() (hosttui.PlanSummary, error) {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return hosttui.PlanSummary{}, err
		}
		mPath := filepath.Join(root, "manifest.yaml")
		m, err := manifest.Load(mPath)
		if err != nil {
			return hosttui.PlanSummary{}, err
		}
		p := planner.Build(m)

		cat, err := catalog.LoadCatalog()
		if err != nil {
			return hosttui.PlanSummary{}, err
		}

		var summary hosttui.PlanSummary
		for _, id := range p.ToDownload {
			item := hosttui.PlanItem{ID: id, Action: "download"}
			if recipe, ok := cat.RecipeByID(id); ok {
				item.Type = recipe.Type
				item.SizeGB = recipe.SizeGB
				item.Description = recipe.Description
			}
			summary.ToDownload = append(summary.ToDownload, item)
			summary.DownloadGB += item.SizeGB
		}
		for _, id := range p.ToRemove {
			item := hosttui.PlanItem{ID: id, Action: "remove"}
			if recipe, ok := cat.RecipeByID(id); ok {
				item.Type = recipe.Type
				item.SizeGB = recipe.SizeGB
				item.Description = recipe.Description
			}
			summary.ToRemove = append(summary.ToRemove, item)
			summary.RemoveGB += item.SizeGB
		}
		return summary, nil
	}

	deps.SaveDesiredItems = func(ids []string) error {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return err
		}
		mPath := filepath.Join(root, "manifest.yaml")
		m, err := manifest.Load(mPath)
		if err != nil {
			return err
		}
		m.Desired.Items = ids
		return manifest.Save(mPath, m)
	}

	deps.InitVault = func(path string, items []string, presetName, region string, platforms []string) error {
		return commands.InitVaultWithOptions(path, items, presetName, region, platforms)
	}

	deps.RunApply = func(ctx context.Context, onProgress func(hosttui.ApplyEvent)) error {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return err
		}
		cat, err := catalog.LoadCatalog()
		if err != nil {
			return err
		}
		progress := apply.ProgressFunc(func(ev apply.ProgressEvent) {
			onProgress(hosttui.ApplyEvent{
				ID:         ev.ID,
				Status:     ev.Status,
				Step:       ev.Step,
				Downloaded: ev.Downloaded,
				Total:      ev.Total,
				Error:      ev.Error,
			})
		})
		return commands.ApplyVault(ctx, root, cat, progress)
	}

	deps.RunImport = func(_ context.Context, source string) (hosttui.ImportResult, error) {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return hosttui.ImportResult{}, err
		}
		abs, err := filepath.Abs(source)
		if err != nil {
			return hosttui.ImportResult{}, err
		}
		id, err := commands.ImportAndMaybeAdd(root, abs, "", true, root)
		if err != nil {
			return hosttui.ImportResult{}, err
		}
		return hosttui.ImportResult{ID: id}, nil
	}

	deps.LoadIndexStatus = func() (hosttui.IndexStatus, error) {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return hosttui.IndexStatus{}, err
		}
		dbPath := filepath.Join(root, "data", "search.db")
		status := hosttui.IndexStatus{}
		if _, statErr := os.Stat(dbPath); statErr == nil {
			status.KeywordEnabled = true
			// Try to get stats from the DB
			db, dbErr := searchdb.Open(dbPath)
			if dbErr == nil {
				defer db.Close()
				sc, ac, _ := db.Stats()
				status.KeywordSources = sc
				status.KeywordArticles = ac
				if ts, err := db.GetMeta("indexed_at"); err == nil {
					status.KeywordLastBuilt = ts
				}
			}
		}
		// Check for embedding model presence in models/embed/
		embedDir := filepath.Join(root, "models", "embed")
		if entries, err := os.ReadDir(embedDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".gguf") {
					status.SemanticEnabled = true
					break
				}
			}
		}
		if !status.SemanticEnabled {
			status.SemanticStatus = "model not installed"
		}
		return status, nil
	}

	deps.RunIndex = func(ctx context.Context, indexType string, onProgress func(hosttui.IndexEvent)) error {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return err
		}

		statusMap := map[string]string{
			"extracting": tui.StatusIndexing,
			"embedding":  tui.StatusIndexing,
			"skip":       tui.StatusSkip,
			"done":       tui.StatusDone,
			"failed":     tui.StatusFailed,
			"starting":   tui.StatusIndexing,
			"queued":     tui.StatusQueued,
		}
		mapStatus := func(s string) string {
			if v, ok := statusMap[s]; ok {
				return v
			}
			return tui.StatusIndexing
		}

		keywordCb := func(p commands.IndexProgress) {
			onProgress(hosttui.IndexEvent{
				File:   p.File,
				Status: mapStatus(p.Status),
				Detail: p.Detail,
			})
		}
		semanticCb := func(p commands.SemanticProgress) {
			file := p.File
			if file == "" {
				file = p.Detail
			}
			onProgress(hosttui.IndexEvent{
				File:   file,
				Status: mapStatus(p.Status),
				Detail: p.Detail,
			})
		}

		if indexType == "full" {
			if err := commands.IndexVault(root, true, io.Discard, keywordCb); err != nil {
				return err
			}
			// Semantic is best-effort — keyword index still succeeds if model is missing.
			if err := commands.IndexSemantic(ctx, root, true, io.Discard, semanticCb); err != nil {
				onProgress(hosttui.IndexEvent{
					File: "semantic", Status: tui.StatusFailed, Detail: err.Error(),
				})
			}
			return nil
		}

		if indexType == "semantic" {
			return commands.IndexSemantic(ctx, root, true, io.Discard, semanticCb)
		}

		return commands.IndexVault(root, true, io.Discard, keywordCb)
	}

	return deps
}

