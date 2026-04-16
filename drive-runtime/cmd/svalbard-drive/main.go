package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/agent"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/apps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browse"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/chat"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/embedded"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/maps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/menu"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/serveall"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/share"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/shell"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/verify"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "svalbard-drive: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// MCP subcommand: intercept before config.Load() so it works without actions.json.
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		drive := ""
		for i, arg := range os.Args {
			if arg == "--drive" && i+1 < len(os.Args) {
				drive = os.Args[i+1]
			}
		}
		if drive == "" {
			drive = os.Getenv("DRIVE_ROOT")
		}
		if drive == "" {
			return fmt.Errorf("--drive path required")
		}
		return runMCP(drive)
	}

	driveRoot, err := resolveDriveRoot()
	if err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		workDir = driveRoot
	}

	cfg, err := config.Load(filepath.Join(driveRoot, ".svalbard", "actions.json"))
	if err != nil {
		return err
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case actions.NativeInspectSubcommand:
			return inspect.Run(os.Stdout, driveRoot)
		case actions.NativeVerifySubcommand:
			return verify.Run(os.Stdout, driveRoot)
		case actions.NativeShareSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return share.Run(ctx, os.Stdout, driveRoot)
		case actions.NativeBrowseSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			selected := ""
			if len(os.Args) > 2 {
				selected = os.Args[2]
			}
			return browse.Run(ctx, os.Stdout, driveRoot, selected, nil)
		case actions.NativeAppsSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if len(os.Args) < 3 {
				return fmt.Errorf("app name required")
			}
			return apps.Run(ctx, os.Stdout, driveRoot, os.Args[2], nil)
		case actions.NativeMapsSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return maps.Run(ctx, os.Stdout, driveRoot, nil)
		case actions.NativeChatSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			selected := ""
			if len(os.Args) > 2 {
				selected = os.Args[2]
			}
			return chat.Run(ctx, os.Stdout, driveRoot, selected, nil)
		case actions.NativeAgentSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if len(os.Args) < 3 {
				return fmt.Errorf("client name required")
			}
			selectedModel := ""
			if len(os.Args) > 3 {
				selectedModel = os.Args[3]
			}
			return agent.Run(ctx, os.Stdout, driveRoot, os.Args[2], selectedModel)
		case actions.NativeServeAllSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			bind := "127.0.0.1"
			if len(os.Args) > 2 {
				bind = os.Args[2]
			}
			return serveall.Run(ctx, os.Stdout, driveRoot, bind)
		case actions.NativeSearchSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			query := ""
			if len(os.Args) > 2 {
				query = os.Args[2]
			}
			return search.Run(ctx, os.Stdin, os.Stdout, driveRoot, query, nil)
		case actions.NativeEmbeddedSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return embedded.Run(ctx, os.Stdout, driveRoot)
		case actions.NativeActivateSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return shell.Run(ctx, os.Stdout, driveRoot)
		}
		if item, ok := cfg.FindItemByAlias(os.Args[1]); ok {
			runner := actions.NewRunnerWithWorkDir(driveRoot, workDir)
			resolved, err := runner.Resolve(item.Action)
			if err != nil {
				return err
			}
			return runResolvedAction(resolved)
		}
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}

	p := tea.NewProgram(menu.NewModel(cfg, driveRoot, workDir), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func runResolvedAction(resolved actions.ResolvedAction) error {
	switch resolved.Mode {
	case actions.ModeCaptureOutput:
		err := resolved.Cmd.Run()
		if resolved.Cmd.Stdout != nil {
			if buf, ok := resolved.Cmd.Stdout.(interface{ String() string }); ok {
				fmt.Fprint(os.Stdout, buf.String())
			}
		}
		if resolved.Cmd.Stderr != nil {
			if buf, ok := resolved.Cmd.Stderr.(interface{ String() string }); ok {
				fmt.Fprint(os.Stderr, buf.String())
			}
		}
		return err
	case actions.ModeExecProcess:
		return resolved.Cmd.Run()
	default:
		return fmt.Errorf("unknown action mode: %d", resolved.Mode)
	}
}

func runMCP(driveRoot string) error {
	meta, _ := mcp.LoadMetadata(driveRoot)
	srv := mcp.NewServer(
		mcp.NewSearchCapability(driveRoot, meta),
		mcp.NewVaultCapability(driveRoot, meta),
	)
	defer srv.Close()
	return srv.ServeStdio()
}

func resolveDriveRoot() (string, error) {
	if driveRoot := os.Getenv("DRIVE_ROOT"); driveRoot != "" {
		return driveRoot, nil
	}
	return os.Getwd()
}
