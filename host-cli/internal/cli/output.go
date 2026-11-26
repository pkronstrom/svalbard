package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/tui"
	"github.com/spf13/cobra"
)

// writeJSON writes v to w as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// vaultPlatforms returns the platforms of the vault the command addresses, or
// nil outside a vault, so size estimates assume one platform.
func vaultPlatforms(cmd *cobra.Command) []string {
	vaultFlag, _ := cmd.Flags().GetString("vault")
	root, err := ResolveVaultRoot(vaultFlag)
	if err != nil {
		return nil
	}
	m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		return nil
	}
	return m.Desired.Options.HostPlatforms
}

// applyLines reports apply progress as one line per change of an item's
// state or build step, for terminals and logs alike.
func applyLines(w io.Writer) apply.ProgressFunc {
	var mu sync.Mutex
	last := make(map[string]string)
	return func(ev apply.ProgressEvent) {
		if ev.ID == "" {
			return
		}
		var line string
		switch ev.Status {
		case tui.StatusActive:
			line = "downloading"
			if ev.Step != "" {
				line = ev.Step
			}
		case tui.StatusDone:
			line = "done"
		case tui.StatusFailed:
			line = "failed: " + ev.Error
		default:
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if last[ev.ID] == line {
			return
		}
		last[ev.ID] = line
		fmt.Fprintf(w, "%s: %s\n", ev.ID, line)
	}
}

// applyAndIndex reconciles the vault and then runs its configured indexing,
// the same two steps the wizard and the TUI plan screen run.
func applyAndIndex(ctx context.Context, vaultRoot string, cat *catalog.Catalog, out io.Writer) error {
	if err := commands.ApplyVault(ctx, vaultRoot, cat, applyLines(out)); err != nil {
		return err
	}
	return indexAfter(ctx, vaultRoot, "Apply", out)
}

// indexAfter runs the vault's configured indexing after a completed apply or
// update, named by what, and reports the whole operation done.
func indexAfter(ctx context.Context, vaultRoot, what string, out io.Writer) error {
	m, err := manifest.Load(filepath.Join(vaultRoot, "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("loading applied manifest: %w", err)
	}
	if err := commands.RunConfiguredIndex(ctx, vaultRoot, m.Desired.Options.IndexStrategy, out, nil, nil); err != nil {
		return fmt.Errorf("%s completed but indexing failed: %w", strings.ToLower(what), err)
	}
	fmt.Fprintln(out, what+" complete")
	return nil
}

// sizeOf estimates the size of the catalog recipes among ids on a vault that
// targets platforms.
func sizeOf(cat *catalog.Catalog, ids, platforms []string) string {
	var items []catalog.Item
	for _, id := range ids {
		if item, ok := cat.RecipeByID(id); ok {
			items = append(items, item)
		}
	}
	return tui.FormatSize(catalog.TotalSizeGB(items, platforms))
}

// applyCommand is the apply command line for the vault a command was
// pointed at, so its hint works from any directory.
func applyCommand(vaultFlag string) string {
	if vaultFlag == "" {
		return "svalbard apply"
	}
	return "svalbard apply --vault " + vaultFlag
}
