package cli

import (
	"fmt"
	"path/filepath"
	"sort"

	hosttui "github.com/pkronstrom/svalbard/host-tui"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
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
			config, err := buildWizardConfig("")
			if err != nil {
				// Degrade gracefully — launch TUI without wizard data
				return hosttui.RunInteractive(nil)
			}
			return hosttui.RunInteractive(&config)
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
			if err := commands.ApplyVault(vaultRoot, cat); err != nil {
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
			return commands.IndexVault(vaultRoot, cmd.OutOrStdout())
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
	var preset string

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new vault from a preset",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) > 0 {
				target = args[0]
			}
			target, err := filepath.Abs(target)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			cat, err := catalog.LoadCatalog()
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}

			return commands.InitVault(target, preset, cat)
		},
	}

	cmd.Flags().StringVar(&preset, "preset", "default-32", "preset to seed the vault with")

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
				Description: item.Description,
				SizeGB:      item.SizeGB,
			})
		}
		pg.Packs = append(pg.Packs, pack)
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
