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
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/menu"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeagent"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeapps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebrowse"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimechat"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeembedded"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeinspect"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimemaps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimesearch"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeserveall"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeshare"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeverify"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "svalbard-drive: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	driveRoot, err := resolveDriveRoot()
	if err != nil {
		return err
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case actions.NativeInspectSubcommand:
			return runtimeinspect.Run(os.Stdout, driveRoot)
		case actions.NativeVerifySubcommand:
			return runtimeverify.Run(os.Stdout, driveRoot)
		case actions.NativeShareSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runtimeshare.Run(ctx, os.Stdout, driveRoot)
		case actions.NativeBrowseSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			selected := ""
			if len(os.Args) > 2 {
				selected = os.Args[2]
			}
			return runtimebrowse.Run(ctx, os.Stdout, driveRoot, selected, nil)
		case actions.NativeAppsSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if len(os.Args) < 3 {
				return fmt.Errorf("app name required")
			}
			return runtimeapps.Run(ctx, os.Stdout, driveRoot, os.Args[2], nil)
		case actions.NativeMapsSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runtimemaps.Run(ctx, os.Stdout, driveRoot, nil)
		case actions.NativeChatSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			selected := ""
			if len(os.Args) > 2 {
				selected = os.Args[2]
			}
			return runtimechat.Run(ctx, os.Stdout, driveRoot, selected, nil)
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
			return runtimeagent.Run(ctx, os.Stdout, driveRoot, os.Args[2], selectedModel)
		case actions.NativeServeAllSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			bind := "127.0.0.1"
			if len(os.Args) > 2 {
				bind = os.Args[2]
			}
			return runtimeserveall.Run(ctx, os.Stdout, driveRoot, bind)
		case actions.NativeSearchSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			query := ""
			if len(os.Args) > 2 {
				query = os.Args[2]
			}
			return runtimesearch.Run(ctx, os.Stdin, os.Stdout, driveRoot, query, nil)
		case actions.NativeEmbeddedSubcommand:
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runtimeembedded.Run(ctx, os.Stdout, driveRoot)
		}
	}

	cfg, err := config.Load(filepath.Join(driveRoot, ".svalbard", "actions.json"))
	if err != nil {
		return err
	}

	p := tea.NewProgram(menu.NewModel(cfg, driveRoot), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func resolveDriveRoot() (string, error) {
	if driveRoot := os.Getenv("DRIVE_ROOT"); driveRoot != "" {
		return driveRoot, nil
	}
	return os.Getwd()
}
