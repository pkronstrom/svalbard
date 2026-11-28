package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/frontpage"
)

func runFrontpage(cfg config.RuntimeConfig, driveRoot string, args []string) error {
	open := browser.Open
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "--no-open":
		open = nil
	case len(args) == 1 && (args[0] == "--help" || args[0] == "-h"):
		fmt.Fprintln(os.Stdout, "Usage: ./run web [--no-open]")
		return nil
	default:
		return fmt.Errorf("usage: ./run web [--no-open]")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	return frontpage.Foreground(ctx, os.Stdout, driveRoot, cfg, open)
}

func runDesktop(args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(os.Stdout, "Usage: ./run launch [--no-open | status | stop]")
		return nil
	}
	// The installed run wrapper sets DRIVE_ROOT. A direct executable invocation
	// resolves its own runtime directory, ignoring any unrelated working vault.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	root := driveRootFor(os.Getenv("DRIVE_ROOT"), "", exe)
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("use this vault's run launcher or set DRIVE_ROOT to its absolute path")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(root, ".svalbard", "actions.json"))
	if err != nil {
		return fmt.Errorf("load the vault configuration: %w. Apply the vault with the Svalbard host", err)
	}
	if len(args) == 1 && args[0] == "--serve" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return frontpage.ServeDesktop(ctx, root, cfg)
	}
	return frontpage.Launch(root, args, os.Stdout, browser.OpenFile)
}
