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
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
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
		sse := false
		for i, arg := range os.Args {
			if arg == "--drive" && i+1 < len(os.Args) {
				drive = os.Args[i+1]
			}
			if arg == "--sse" {
				sse = true
			}
		}
		if drive == "" {
			drive = os.Getenv("DRIVE_ROOT")
		}
		if drive == "" {
			return fmt.Errorf("--drive path required")
		}
		if sse {
			return runMCPServe(drive)
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
		invocation, known, err := actions.DecodeNativeInvocation(os.Args[1:])
		if err != nil {
			return err
		}
		if known {
			return runNativeInvocation(invocation, driveRoot)
		}
		if item, ok := cfg.FindItemByAlias(os.Args[1]); ok {
			runner := actions.NewRunner(driveRoot, workDir)
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

func runNativeInvocation(invocation actions.NativeInvocation, driveRoot string) error {
	switch invocation.ActionID {
	case "inspect":
		return inspect.Run(os.Stdout, driveRoot)
	case "verify":
		return verify.Run(os.Stdout, driveRoot)
	case "mcp-serve":
		return runMCPServe(driveRoot)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch invocation.ActionID {
	case "share":
		return share.Run(ctx, os.Stdout, driveRoot)
	case "browse":
		return browse.Run(ctx, os.Stdout, driveRoot, invocation.Args["zim"], nil)
	case "apps":
		return apps.Run(ctx, os.Stdout, driveRoot, invocation.Args["app"], nil)
	case "maps":
		return maps.Run(ctx, os.Stdout, driveRoot, nil)
	case "chat":
		return chat.Run(ctx, os.Stdout, driveRoot, invocation.Args["model"], nil)
	case "agent":
		return agent.Run(ctx, os.Stdout, driveRoot, invocation.Args["client"], invocation.Args["model"])
	case "serve-all":
		return serveall.Run(ctx, os.Stdout, driveRoot, invocation.Args["bind"])
	case "search":
		return search.Run(ctx, os.Stdin, os.Stdout, driveRoot, invocation.Args["query"], nil)
	case "embedded-shell":
		return embedded.Run(ctx, os.Stdout, driveRoot)
	case "activate-shell":
		return shell.Run(ctx, os.Stdout, driveRoot)
	default:
		return fmt.Errorf("unknown native action: %s", invocation.ActionID)
	}
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

func runMCPServe(driveRoot string) error {
	meta, _ := mcp.LoadMetadata(driveRoot)
	srv := mcp.NewServer(
		mcp.NewSearchCapability(driveRoot, meta),
		mcp.NewVaultCapability(driveRoot, meta),
		mcp.NewQueryCapability(driveRoot, meta),
	)
	defer srv.Close()

	port, err := netutil.FindAvailablePort("127.0.0.1", 8090)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "MCP server (SSE) listening on http://%s/sse\n", addr)
	fmt.Fprintln(os.Stdout, "────────────────────────────────")
	fmt.Fprintln(os.Stdout, "Connect from any MCP client with:")
	fmt.Fprintf(os.Stdout, "  URL: http://%s/sse\n", addr)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Press Ctrl+C to stop.")
	fmt.Fprintln(os.Stdout)

	return srv.ServeSSE(addr)
}

func runMCP(driveRoot string) error {
	meta, _ := mcp.LoadMetadata(driveRoot)
	srv := mcp.NewServer(
		mcp.NewSearchCapability(driveRoot, meta),
		mcp.NewVaultCapability(driveRoot, meta),
		mcp.NewQueryCapability(driveRoot, meta),
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
