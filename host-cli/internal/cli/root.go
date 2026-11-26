package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
	"github.com/pkronstrom/svalbard/host-cli/internal/volumes"
	hosttui "github.com/pkronstrom/svalbard/host-tui"
	"github.com/pkronstrom/svalbard/tui"
	"github.com/spf13/cobra"
)

// NewRootCommand returns the top-level svalbard CLI command with all
// hard-reset subcommands registered.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "svalbard",
		Short: "Provision and reconcile offline knowledge vaults",
		// main prints the error once; a failed command is not a usage mistake.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// No subcommand → launch interactive TUI dashboard
			vaultFlag, _ := cmd.Flags().GetString("vault")
			config, err := buildWizardConfig("", cmd.ErrOrStderr())
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
		Use:   "add <item|preset...>",
		Short: "Add items (or every item of a preset/pack) to the desired state",
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
			cat, err := loadCatalog(cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			ids, err := commands.ExpandItemRefs(cat, args)
			if err != nil {
				return err
			}
			// An item this vault already holds can be selected again even after
			// the catalog dropped it; that is how a removal is taken back.
			unknown := slices.DeleteFunc(commands.UnknownItems(cat, ids), func(id string) bool {
				return slices.ContainsFunc(m.Realized.Entries, func(e manifest.RealizedEntry) bool { return e.ID == id })
			})
			if len(unknown) > 0 {
				return fmt.Errorf("not in the catalog: %s; run \"svalbard catalog\" to list ids", strings.Join(unknown, ", "))
			}
			defaults, err := catalog.LoadDepDefaultsAuto()
			if err != nil {
				return fmt.Errorf("loading dep defaults: %w", err)
			}
			deps, missing := commands.RequiredDependencies(cat, defaults, ids)
			if len(missing) > 0 {
				return fmt.Errorf("missing catalog recipes: %s", strings.Join(missing, ", "))
			}
			before := len(m.Desired.Items)
			if err := commands.AddItems(&m, ids); err != nil {
				return err
			}
			requested := len(m.Desired.Items)
			if err := commands.AddItems(&m, deps); err != nil {
				return err
			}
			added := m.Desired.Items[before:]
			if len(added) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Already selected; nothing added")
				return nil
			}
			if err := manifest.Save(mPath, m); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Added %d item(s) to desired state (%s)\n", len(added), sizeOf(cat, added, m.Desired.Options.HostPlatforms))
			if required := m.Desired.Items[requested:]; len(required) > 0 {
				fmt.Fprintf(out, "Includes required: %s\n", strings.Join(required, ", "))
			}
			fmt.Fprintf(out, "Run %q to download\n", applyCommand(vaultFlag))
			for _, warning := range commands.DrafterWarnings(cat, m.Desired.Items) {
				fmt.Fprintln(cmd.ErrOrStderr(), warning)
			}
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <item|preset...>",
		Short: "Remove items (or every item of a preset/pack) from the desired state",
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
			cat, err := loadCatalog(cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			ids, err := commands.ExpandItemRefs(cat, args)
			if err != nil {
				return err
			}
			withdrawn := commands.BuildStatus(cat, m).Withdrawn
			before := len(m.Desired.Items)
			if err := commands.RemoveItems(&m, ids); err != nil {
				return err
			}
			removed := before - len(m.Desired.Items)
			if removed == 0 {
				return fmt.Errorf("none of %s is selected in this vault; run \"svalbard status\"", strings.Join(args, ", "))
			}
			if err := manifest.Save(mPath, m); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Removed %d item(s) from desired state\n", removed)
			if defaults, err := catalog.LoadDepDefaultsAuto(); err == nil {
				if required := commands.StillRequired(cat, defaults, m.Desired.Items, ids); len(required) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: selected items still need %s\n", strings.Join(required, ", "))
				}
			}
			for _, id := range ids {
				if slices.Contains(withdrawn, id) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s is no longer in the catalog; once apply deletes it, it cannot be downloaded again (\"svalbard add %s\" before applying keeps it)\n", id, id)
				}
			}
			fmt.Fprintf(out, "Run %q to update the vault\n", applyCommand(vaultFlag))
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
			cat, err := loadVaultCatalog(cmd.ErrOrStderr(), vaultRoot)
			if err != nil {
				return err
			}
			detail := commands.DescribePlan(cat, m)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return writeJSON(cmd.OutOrStdout(), detail)
			}
			if err := commands.WritePlanDetail(cmd.OutOrStdout(), detail); err != nil {
				return err
			}
			_, documents := commands.BuildPlan(m, cat)
			commands.WriteDocumentPlans(cmd.OutOrStdout(), documents)
			commands.WritePlanNext(cmd.OutOrStdout(), detail)
			return nil
		},
	}
	planCmd.Flags().Bool("json", false, "print the plan as JSON")

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
			cat, err := loadVaultCatalog(cmd.ErrOrStderr(), vaultRoot)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return writeJSON(cmd.OutOrStdout(), commands.BuildStatus(cat, m))
			}
			return commands.WriteStatus(cmd.OutOrStdout(), cat, m)
		},
	}
	statusCmd.Flags().Bool("json", false, "print the status as JSON")

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Execute reconciliation plan and update realized state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			cat, err := loadVaultCatalog(cmd.ErrOrStderr(), vaultRoot)
			if err != nil {
				return err
			}
			commands.WriteVaultDocumentNotes(cmd.ErrOrStderr(), vaultRoot, cat)
			if m, err := manifest.Load(filepath.Join(vaultRoot, "manifest.yaml")); err == nil {
				commands.WriteSkipped(cmd.ErrOrStderr(), cat, m)
			}
			return applyAndIndex(ctx, vaultRoot, cat, cmd.OutOrStdout())
		},
	}

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Build the search index from ZIM files and documents in the vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch kind, _ := cmd.Flags().GetString("type"); kind {
			case "keyword":
				return commands.IndexVaultContext(ctx, vaultRoot, false, out, nil)
			case "semantic":
				return commands.IndexSemantic(ctx, vaultRoot, false, out, nil)
			case "full":
				return commands.RunConfiguredIndex(ctx, vaultRoot, "full", out, nil, nil)
			default:
				return fmt.Errorf("unknown index type %q; use keyword, semantic or full", kind)
			}
		},
	}
	indexCmd.Flags().String("type", "keyword", "what to build: keyword, semantic (needs an embedding model on the vault) or full (both)")

	root.AddCommand(
		newInitCommand(),
		addCmd,
		removeCmd,
		planCmd,
		applyCmd,
		statusCmd,
		newImportCommand(),
		newZimCommand(),
		newPresetCommand(),
		newCatalogCommand(),
		newUpdateCommand(),
		newSkillCommand(),
		indexCmd,
	)

	return root
}

func mapSemanticIndexEvent(p commands.SemanticProgress, fullIndex bool) hosttui.IndexEvent {
	ev := hosttui.IndexEvent{
		Status: p.Status,
		Detail: p.Detail,
	}
	if p.File == "" {
		return ev
	}

	if fullIndex {
		ev.File = "embed: " + p.File
		if p.Detail != "" {
			ev.Detail = p.File + "  " + p.Detail
		} else {
			ev.Detail = p.File
		}
		return ev
	}

	ev.File = p.File
	if ev.Detail == "" {
		ev.Detail = p.File
	}
	return ev
}

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Create a vault: the guided wizard, or directly with --preset",
		Long: "Without flags, opens the guided setup wizard.\n\n" +
			"With --preset or --add, creates the vault at <path> without the wizard: the\n" +
			"preset's items, plus --add, minus --remove, plus the dependencies they need.",
		Example: "  svalbard init /Volumes/STICK --preset default-64 --apply\n" +
			"  svalbard init ./vault --preset default-32 --add ai/harnesses --remove ifixit --platform all",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			presetFlag, _ := cmd.Flags().GetString("preset")
			addFlag, _ := cmd.Flags().GetStringSlice("add")
			if presetFlag != "" || len(addFlag) > 0 {
				return initWithoutWizard(cmd, args, presetFlag, addFlag)
			}
			for _, name := range []string{"remove", "platform", "apply"} {
				if cmd.Flags().Changed(name) {
					return fmt.Errorf("--%s needs --preset or --add", name)
				}
			}

			prefill := ""
			if len(args) > 0 {
				abs, err := filepath.Abs(args[0])
				if err != nil {
					return fmt.Errorf("resolving path: %w", err)
				}
				prefill = abs
			}

			config, err := buildWizardConfig(prefill, cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("preparing wizard: %w", err)
			}
			deps := buildDashboardDeps("", &config)
			config.InitVault = deps.InitVault
			if deps.RunApply != nil && deps.RebuildForVault != nil {
				rebuildForVault := deps.RebuildForVault
				config.RunApply = func(ctx context.Context, vaultPath string, onProgress func(hosttui.WizardApplyEvent)) error {
					newDeps := rebuildForVault(vaultPath)
					return newDeps.RunApply(ctx, func(ev hosttui.ApplyEvent) {
						onProgress(hosttui.WizardApplyEvent{ID: ev.ID, Status: ev.Status, Step: ev.Step, Downloaded: ev.Downloaded, Total: ev.Total, Error: ev.Error})
					})
				}
			}
			return hosttui.RunInitWizard(config)
		},
	}

	cmd.Flags().String("preset", "", "preset to start from (see \"svalbard preset list\")")
	cmd.Flags().StringSlice("add", nil, "recipe ids, packs or presets to add")
	cmd.Flags().StringSlice("remove", nil, "recipe ids, packs or presets to leave out")
	cmd.Flags().StringSlice("platform", nil, "platforms the vault runs on, or \"all\" (default: this computer's)")
	cmd.Flags().Bool("apply", false, "download the content and build the search index right away")

	return cmd
}

// initWithoutWizard creates a vault from flags, with the same result the
// wizard produces for the same choices.
func initWithoutWizard(cmd *cobra.Command, args []string, presetName string, add []string) error {
	if len(args) == 0 {
		return fmt.Errorf("init --preset needs the vault path: svalbard init <path> --preset <name>")
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
		return fmt.Errorf("parent directory does not exist: %s", filepath.Dir(path))
	}

	platforms, _ := cmd.Flags().GetStringSlice("platform")
	supported := toolkit.SupportedPlatforms()
	switch {
	case len(platforms) == 0:
		platforms = []string{apply.HostPlatform()}
	case len(platforms) == 1 && platforms[0] == "all":
		platforms = supported
	default:
		for _, p := range platforms {
			if !slices.Contains(supported, p) {
				return fmt.Errorf("unknown platform %q; use %s or all", p, strings.Join(supported, ", "))
			}
		}
	}

	cat, err := loadCatalog(cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	defaults, err := catalog.LoadDepDefaultsAuto()
	if err != nil {
		return fmt.Errorf("loading dep defaults: %w", err)
	}
	remove, _ := cmd.Flags().GetStringSlice("remove")
	ids, region, err := commands.ResolveInitSelection(cat, defaults, presetName, add, remove)
	if err != nil {
		return err
	}
	if err := commands.InitVaultWithOptions(path, ids, presetName, region, platforms); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Created vault at %s: %d items, %s, for %s\n", path, len(ids),
		sizeOf(cat, ids, platforms), strings.Join(platforms, ", "))
	if runApply, _ := cmd.Flags().GetBool("apply"); !runApply {
		fmt.Fprintf(out, "Run \"svalbard apply --vault %s\" to download\n", path)
		return nil
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return applyAndIndex(ctx, path, cat, out)
}

// newUpdateCommand reacquires installed items from their current recipes.
func newUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [item...]",
		Short: "Update installed items whose recipe changed since they were installed",
		Long: "Without arguments, lists the installed items that have an update available.\n" +
			"With item ids, or --all for every item with an update, acquires them again.\n\n" +
			"Each installed copy is kept until its new one is in place and restored if the\n" +
			"download or build fails. Items the catalog no longer has cannot be updated and\n" +
			"stay on the vault. \"svalbard apply\" never updates installed items.\n\n" +
			"An item installed before versions were recorded reads as unknown. If you trust\n" +
			"that it matches today's recipe, --mark-current records that without downloading.",
		Example: "  svalbard update\n  svalbard update --all\n  svalbard update wikipedia-en-100 ifixit\n  svalbard update wikipedia-en-maxi --replace",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			m, err := manifest.Load(filepath.Join(vaultRoot, "manifest.yaml"))
			if err != nil {
				return err
			}
			cat, err := loadVaultCatalog(cmd.ErrOrStderr(), vaultRoot)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			all, _ := cmd.Flags().GetBool("all")
			replace, _ := cmd.Flags().GetBool("replace")
			available := commands.AvailableUpdates(cat, m)

			if mark, _ := cmd.Flags().GetBool("mark-current"); mark {
				if all || replace {
					return fmt.Errorf("--mark-current downloads nothing, so it takes neither --all nor --replace; without item ids it marks every item of unknown version")
				}
				marked, differing, err := commands.MarkCurrent(vaultRoot, cat, args)
				if err != nil {
					return err
				}
				if len(marked) > 0 {
					fmt.Fprintf(out, "Marked %d item(s) as current without downloading: %s\n", len(marked), strings.Join(marked, ", "))
				} else if len(differing) == 0 {
					fmt.Fprintln(out, "No items of unknown version")
				}
				if len(differing) > 0 {
					fmt.Fprintf(out, "Left as unknown, the installed file is not the one the recipe pins: %s. \"svalbard update %s\" downloads it.\n", strings.Join(differing, ", "), strings.Join(differing, " "))
				}
				return nil
			}

			if len(args) == 0 && !all {
				if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
					return writeJSON(out, available)
				}
				for _, item := range available {
					fmt.Fprintf(out, "  %s  %s  %s\n", item.ID, tui.FormatSize(item.SizeGB), item.Description)
				}
				if len(available) > 0 {
					fmt.Fprintf(out, "\n%d update(s) available. Run \"svalbard update --all\", or name the items to update.\n", len(available))
				} else {
					fmt.Fprintln(out, "No updates available")
				}
				report := commands.BuildStatus(cat, m)
				if len(report.Unknown) > 0 {
					fmt.Fprintf(out, "%d item(s) of unknown version; \"svalbard update --mark-current\" records them as current without downloading.\n", len(report.Unknown))
				}
				if len(report.Withdrawn) > 0 {
					fmt.Fprintf(out, "No longer in the catalog, kept on this vault: %s\n", strings.Join(report.Withdrawn, ", "))
				}
				return nil
			}
			ids := args
			if all {
				if len(args) > 0 {
					return fmt.Errorf("give item ids or --all, not both")
				}
				for _, item := range available {
					ids = append(ids, item.ID)
				}
				if len(ids) == 0 {
					fmt.Fprintln(out, "No updates available")
					return nil
				}
			}
			if err := commands.UpdateVault(ctx, vaultRoot, cat, ids, replace, applyLines(out)); err != nil {
				return err
			}
			return indexAfter(ctx, vaultRoot, "Update", out)
		},
	}
	cmd.Flags().Bool("all", false, "update every item that has an update available")
	cmd.Flags().Bool("replace", false, "delete each installed copy before downloading its update (for vaults without room for both)")
	cmd.Flags().Bool("mark-current", false, "record items of unknown version (the ids given, or all of them) as current without downloading")
	cmd.Flags().Bool("json", false, "print the available updates as JSON")
	return cmd
}

// newCatalogCommand lists the recipes a vault can hold, the CLI counterpart
// of the TUI browse screen.
func newCatalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog [words...]",
		Short: "List the items a vault can hold, optionally filtered by words",
		Long: "Lists recipe ids with type, size, the packs that include them and a description.\n" +
			"Every word given must appear in one of those fields.",
		Example: "  svalbard catalog\n  svalbard catalog wikipedia en\n  svalbard catalog --json map",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := loadCatalog(cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			entries := commands.CatalogEntries(cat, vaultPlatforms(cmd), strings.Join(args, " "))
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return writeJSON(cmd.OutOrStdout(), entries)
			}
			if len(entries) == 0 {
				return fmt.Errorf("no catalog item matches %q", strings.Join(args, " "))
			}
			return commands.WriteCatalog(cmd.OutOrStdout(), entries)
		},
	}
	cmd.Flags().Bool("json", false, "print the items as JSON")
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

func newZimCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zim <url>",
		Short: "Crawl a website into a ZIM on the vault (runs the zimit container, needs Docker)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultFlag, _ := cmd.Flags().GetString("vault")
			vaultRoot, err := ResolveVaultRoot(vaultFlag)
			if err != nil {
				return err
			}
			nameFlag, _ := cmd.Flags().GetString("name")
			videosFlag, _ := cmd.Flags().GetBool("videos")
			qualityFlag, _ := cmd.Flags().GetString("quality")

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Crawling %s — this can take a while\n", args[0])
			opts := commands.ZimOptions{Name: nameFlag, Videos: videosFlag, Quality: qualityFlag}
			id, err := commands.BuildZim(cmd.Context(), vaultRoot, args[0], opts, func(step string) {
				fmt.Fprintf(out, "  %s\n", step)
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Built %s.zim and added %q to the vault\n", id, id)
			return nil
		},
	}

	cmd.Flags().String("name", "", "item id / output name (default: derived from the URL host)")
	cmd.Flags().Bool("videos", false, "download embedded YouTube videos and rewrite them to play offline")
	cmd.Flags().String("quality", "480p", "max video height when --videos is set")

	return cmd
}

func newPresetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Manage presets",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available presets and packs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := loadCatalog(cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return writeJSON(cmd.OutOrStdout(), commands.PresetSummaries(cat, vaultPlatforms(cmd)))
			}
			return commands.WritePresetList(cmd.OutOrStdout(), cat)
		},
	}
	listCmd.Flags().Bool("json", false, "print presets with kind, region, size and description as JSON")
	cmd.AddCommand(listCmd)

	showCmd := &cobra.Command{
		Use:   "show <name>",
		Short: "List the items a preset or pack contains",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := loadCatalog(cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			entries, err := commands.PresetEntries(cat, vaultPlatforms(cmd), args[0])
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return writeJSON(cmd.OutOrStdout(), entries)
			}
			return commands.WriteCatalog(cmd.OutOrStdout(), entries)
		},
	}
	showCmd.Flags().Bool("json", false, "print the items as JSON")
	cmd.AddCommand(showCmd)

	copyCmd := &cobra.Command{
		Use:   "copy <source> <target>",
		Short: "Start a user preset that extends <source>",
		Long: "Writes a preset file that extends <source>. A bare <target> name is written to\n" +
			"the user preset dir (~/.config/svalbard/presets/<target>.yaml); a path is used as is.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := loadCatalog(cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			target, err := commands.PresetTargetPath(args[1])
			if err != nil {
				return err
			}
			flatten, _ := cmd.Flags().GetBool("flatten")
			if err := commands.CopyPreset(cat, args[0], target, flatten); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Copied preset %q to %s\n", args[0], target)
			return nil
		},
	}
	copyCmd.Flags().Bool("flatten", false, "write the fully resolved source list instead of an extends stub")
	cmd.AddCommand(copyCmd)

	return cmd
}

// loadCatalog loads the catalog and prints one warning to w for each user
// recipe and user preset it skipped.
func loadCatalog(w io.Writer) (*catalog.Catalog, error) {
	cat, err := catalog.LoadCatalog()
	if err != nil {
		return nil, fmt.Errorf("loading catalog: %w", err)
	}
	for _, skipped := range cat.SkippedUserRecipes() {
		fmt.Fprintf(w, "warning: %s\n", skipped)
	}
	for _, skipped := range cat.SkippedUserPresets() {
		fmt.Fprintf(w, "warning: %s\n", skipped)
	}
	return cat, nil
}

// loadVaultCatalog is loadCatalog for a command that works on a vault. A
// user recipe the vault holds but this computer does not define is loaded
// from the copy the vault keeps of it, and w is told so.
func loadVaultCatalog(w io.Writer, vaultRoot string) (*catalog.Catalog, error) {
	cat, err := loadCatalog(w)
	if err != nil {
		return nil, err
	}
	for _, id := range cat.LoadVaultRecipes(catalog.VaultRecipeDir(vaultRoot)) {
		fmt.Fprintf(w, "note: %s is not defined on this computer; using the vault's copy of its recipe\n", id)
	}
	return cat, nil
}

// buildWizardConfig loads the catalog and detected volumes to prepare
// a WizardConfig for the interactive init wizard. Skipped user presets are
// reported to warn before the TUI takes over the terminal.
func buildWizardConfig(prefillPath string, warn io.Writer) (hosttui.WizardConfig, error) {
	cat, err := loadCatalog(warn)
	if err != nil {
		return hosttui.WizardConfig{}, err
	}

	depDefaults, err := catalog.LoadDepDefaultsAuto()
	if err != nil {
		return hosttui.WizardConfig{}, fmt.Errorf("loading dep defaults: %w", err)
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

	presetOpts, packGroups := wizardCatalog(cat, nil)

	return hosttui.WizardConfig{
		Volumes:     wizVols,
		HomeVolume:  homeVol,
		Presets:     presetOpts,
		Regions:     cat.Regions(),
		PackGroups:  packGroups,
		PrefillPath: prefillPath,
		FreeSpace:   volumes.FreeSpaceGB,
		ResolveDeps: func(selectedIDs map[string]bool) (map[string]bool, []string) {
			return cat.ResolveDepsChecked(selectedIDs, depDefaults)
		},
		ForPlatforms: func(platforms []string) ([]hosttui.PresetOption, []hosttui.PackGroup) {
			return wizardCatalog(cat, platforms)
		},
	}, nil
}

// wizardCatalog lists the presets and pack groups the wizard and browse
// screens offer, with sizes estimated for a vault that targets platforms.
func wizardCatalog(cat *catalog.Catalog, platforms []string) ([]hosttui.PresetOption, []hosttui.PackGroup) {
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
				ContentGB:    resolved.ContentSizeGB(platforms),
				TargetSizeGB: p.TargetSizeGB,
				Region:       p.Region,
				SourceIDs:    resolved.Sources,
				Custom:       p.IsUser(),
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
				SizeGB:      item.SizeGBFor(platforms),
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
			SizeGB:      item.SizeGBFor(platforms),
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

	return presetOpts, packGroups
}

// buildDashboardDeps constructs callback closures that bridge host-cli
// business logic into the TUI screens without the TUI importing host-cli.
func buildDashboardDeps(vaultFlag string, wizConfig *hosttui.WizardConfig) *hosttui.DashboardDeps {
	deps := &hosttui.DashboardDeps{}

	if wizConfig != nil {
		deps.PackGroups = wizConfig.PackGroups
		deps.Presets = wizConfig.Presets
		deps.ResolveDeps = wizConfig.ResolveDeps
		if wizConfig.ForPlatforms != nil {
			// Browse estimates sizes for the open vault's platforms.
			if root, err := ResolveVaultRoot(vaultFlag); err == nil {
				if m, err := manifest.Load(filepath.Join(root, "manifest.yaml")); err == nil {
					deps.Presets, deps.PackGroups = wizConfig.ForPlatforms(m.Desired.Options.HostPlatforms)
				}
			}
		}
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

		status := hosttui.VaultStatus{
			VaultPath:     root,
			VaultName:     m.Vault.Name,
			PresetName:    presetName,
			DesiredCount:  len(m.Desired.Items),
			RealizedCount: realized,
			PendingCount:  len(m.Desired.Items) - realized,
			LastApplied:   m.Realized.AppliedAt,
			DiskFreeGB:    volumes.FreeSpaceGB(root),
		}
		for _, e := range m.Realized.Entries {
			status.DiskUsedGB += float64(e.SizeBytes) / (1 << 30)
		}
		if cat, err := loadVaultCatalog(io.Discard, root); err == nil {
			report := commands.BuildStatus(cat, m)
			status.UpdateCount = len(report.Updates)
			status.WithdrawnCount = len(report.Withdrawn)
			// Items with nothing to install on this vault's platforms
			// do not keep it from reading as synced.
			status.DesiredCount -= len(report.OtherPlatform) + len(report.Unavailable)
			status.PendingCount = report.Pending
		}
		return status, nil
	}

	deps.LoadUpdates = func() (hosttui.PlanSummary, error) {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return hosttui.PlanSummary{}, err
		}
		m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
		if err != nil {
			return hosttui.PlanSummary{}, err
		}
		cat, err := loadVaultCatalog(io.Discard, root)
		if err != nil {
			return hosttui.PlanSummary{}, err
		}
		var summary hosttui.PlanSummary
		for _, it := range commands.AvailableUpdates(cat, m) {
			summary.ToDownload = append(summary.ToDownload, hosttui.PlanItem{
				ID: it.ID, Action: it.Action, Type: it.Type, SizeGB: it.SizeGB, Description: it.Description,
			})
			summary.DownloadGB += it.SizeGB
		}
		summary.FreeGB = volumes.FreeSpaceGB(root)
		return summary, nil
	}

	deps.RunUpdate = func(ctx context.Context, ids []string, replace bool, onProgress func(hosttui.ApplyEvent)) error {
		root, err := ResolveVaultRoot(vaultFlag)
		if err != nil {
			return err
		}
		cat, err := loadVaultCatalog(io.Discard, root)
		if err != nil {
			return err
		}
		return commands.UpdateVault(ctx, root, cat, ids, replace, func(ev apply.ProgressEvent) {
			onProgress(hosttui.ApplyEvent{
				ID: ev.ID, Procedure: ev.Procedure, State: ev.State,
				Status: ev.Status, Step: ev.Step, Message: ev.Message,
				Downloaded: ev.Downloaded, Total: ev.Total, Error: ev.Error,
			})
		})
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
		cat, err := loadVaultCatalog(io.Discard, root)
		if err != nil {
			return hosttui.PlanSummary{}, err
		}
		detail := commands.DescribePlan(cat, m)
		planItems := func(items []commands.PlanItem) []hosttui.PlanItem {
			var mapped []hosttui.PlanItem
			for _, it := range items {
				mapped = append(mapped, hosttui.PlanItem{
					ID: it.ID, Action: it.Action, Type: it.Type, SizeGB: it.SizeGB,
					Description: it.Description, Network: it.Network,
					BuildDownloadGB: it.BuildDownloadGB, BuildWorkGB: it.BuildWorkGB,
					Requires: it.Requires,
				})
			}
			return mapped
		}
		summary := hosttui.PlanSummary{
			ToDownload: planItems(detail.Download), ToRemove: planItems(detail.Remove),
			DownloadGB: detail.DownloadGB, RemoveGB: detail.RemoveGB,
			BuildDownloadGB: detail.BuildDownloadGB, BuildWorkGB: detail.BuildWorkGB,
			ToolsRequired: detail.ToolsRequired,
		}
		summary.FreeGB = volumes.FreeSpaceGB(root)
		summary.FreeAfterGB = summary.FreeGB - detail.DownloadGB + detail.RemoveGB
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
		cat, err := loadVaultCatalog(io.Discard, root)
		if err != nil {
			return err
		}
		progress := apply.ProgressFunc(func(ev apply.ProgressEvent) {
			onProgress(hosttui.ApplyEvent{
				ID: ev.ID, Procedure: ev.Procedure, State: ev.State,
				Status: ev.Status, Step: ev.Step, Message: ev.Message,
				Downloaded: ev.Downloaded, Total: ev.Total, Error: ev.Error,
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
				if model, err := db.GetMeta("embedding_model"); err == nil {
					status.EmbeddingModel = model
				}
				if dimsStr, err := db.GetMeta("embedding_dims"); err == nil && dimsStr != "" {
					if d, convErr := strconv.Atoi(dimsStr); convErr == nil {
						status.EmbeddingDims = d
					}
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
			ev := mapSemanticIndexEvent(p, false)
			ev.Status = mapStatus(p.Status)
			onProgress(ev)
		}

		if indexType == "full" {
			fullSemanticCb := func(p commands.SemanticProgress) {
				ev := mapSemanticIndexEvent(p, true)
				ev.Status = mapStatus(p.Status)
				onProgress(ev)
			}
			return commands.RunConfiguredIndex(ctx, root, "full", io.Discard, keywordCb, fullSemanticCb)
		}

		if indexType == "semantic" {
			return commands.IndexSemantic(ctx, root, false, io.Discard, semanticCb)
		}

		return commands.IndexVaultContext(ctx, root, false, io.Discard, keywordCb)
	}

	deps.RunConfiguredIndex = func(ctx context.Context, vaultPath string, onProgress func(hosttui.IndexEvent)) error {
		m, err := manifest.Load(filepath.Join(vaultPath, "manifest.yaml"))
		if err != nil {
			return err
		}
		statusMap := map[string]string{"extracting": tui.StatusIndexing, "embedding": tui.StatusIndexing, "skip": tui.StatusSkip, "done": tui.StatusDone, "failed": tui.StatusFailed, "starting": tui.StatusIndexing, "queued": tui.StatusQueued}
		mapStatus := func(status string) string {
			if mapped, ok := statusMap[status]; ok {
				return mapped
			}
			return tui.StatusIndexing
		}
		return commands.RunConfiguredIndex(ctx, vaultPath, m.Desired.Options.IndexStrategy, io.Discard,
			func(p commands.IndexProgress) {
				onProgress(hosttui.IndexEvent{File: p.File, Status: mapStatus(p.Status), Detail: p.Detail})
			},
			func(p commands.SemanticProgress) {
				ev := mapSemanticIndexEvent(p, true)
				ev.Status = mapStatus(p.Status)
				onProgress(ev)
			},
		)
	}

	deps.ShouldIndex = func(vaultPath string) bool {
		m, err := manifest.Load(filepath.Join(vaultPath, "manifest.yaml"))
		// Let the configured runner report load errors in the progress screen.
		return err != nil || !strings.EqualFold(strings.TrimSpace(m.Desired.Options.IndexStrategy), "none")
	}
	return deps
}
