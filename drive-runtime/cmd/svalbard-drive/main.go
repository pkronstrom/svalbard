package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/agent"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/apps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browse"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/chat"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/embedded"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/folder"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/maps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/menu"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/openwebui"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/serveall"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/share"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/shell"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/verify"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/wiki"
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
		streamable := false
		for i, arg := range os.Args {
			if arg == "--drive" && i+1 < len(os.Args) {
				drive = os.Args[i+1]
			}
			if arg == "--sse" {
				sse = true
			}
			if arg == "--http" {
				streamable = true
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
		if streamable {
			return runMCPHTTP(drive)
		}
		return runMCP(drive)
	}

	driveRoot, err := resolveDriveRoot()
	if err != nil {
		return err
	}
	// Settings subcommand: a fast path that needs neither actions.json nor the
	// menu, so scripts and other programs can read and change a value.
	if len(os.Args) > 1 && os.Args[1] == "settings" {
		return settings.Run(os.Stdout, os.Stderr, driveRoot, os.Args[2:])
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

	// SIGHUP is the terminal window being closed: the action stops as it
	// does on Ctrl+C, so its servers end and what they kept for the run
	// (the wiki's state folder and socket) is removed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
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
	case "open-webui":
		return openwebui.Run(ctx, os.Stdout, driveRoot, invocation.Args["model"], nil)
	case "wiki":
		return wiki.Run(ctx, os.Stdout, driveRoot, nil)
	case "documents":
		return folder.Documents(ctx, os.Stdout, driveRoot, invocation.Args["path"], nil)
	case "workspace":
		return folder.Workspace(ctx, os.Stdout, driveRoot, nil)
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runResolvedActionContext(ctx, resolved)
}

func runResolvedActionContext(ctx context.Context, resolved actions.ResolvedAction) error {
	err := runActionProcess(ctx, resolved.Cmd)
	switch resolved.Mode {
	case actions.ModeCaptureOutput:
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
		return err
	default:
		return fmt.Errorf("unknown action mode: %d", resolved.Mode)
	}
}

// runActionProcess runs cmd in its own process group, so cancellation can reap
// everything it started. An action that reads the terminal also gets it: a
// separate group is otherwise a background job, stopped on its first read.
func runActionProcess(ctx context.Context, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	holdsTerminal := false
	if stdin, ok := cmd.Stdin.(*os.File); ok && ownsTerminal(int(stdin.Fd())) {
		holdsTerminal = true
		cmd.SysProcAttr.Foreground = true
		cmd.SysProcAttr.Ctty = int(stdin.Fd())
		defer reclaimTerminal(int(stdin.Fd()))
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		if !holdsTerminal {
			done <- cmd.Wait()
			return
		}
		err := waitHoldingTerminal(cmd.Process.Pid)
		_ = cmd.Wait() // already reaped; this only releases the command's pipes
		// Ctrl+C went to the action alone, so nothing else ends what it left behind.
		sweepGroup(cmd.Process.Pid)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// The grace period is for the action itself; what outlives it is killed at once.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		reaped := false
		select {
		case <-done:
			reaped = true
		case <-time.After(2 * time.Second):
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return err
		}
		if !reaped {
			<-done
		}
		return ctx.Err()
	}
}

// ownsTerminal reports whether fd is a terminal whose foreground group is this
// process's group. A runtime started as a background job must not take it.
func ownsTerminal(fd int) bool {
	if !term.IsTerminal(uintptr(fd)) {
		return false
	}
	foreground, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	return err == nil && foreground == syscall.Getpgrp()
}

// waitHoldingTerminal waits for an action that owns the terminal. Ctrl+Z would
// leave it stopped with the terminal, out of reach of Ctrl+C and of the calling
// shell, so a suspended action is continued at once.
func waitHoldingTerminal(pid int) error {
	for {
		var status syscall.WaitStatus
		_, err := syscall.Wait4(pid, &status, syscall.WUNTRACED, nil)
		switch {
		case err == syscall.EINTR:
		case err != nil:
			return err
		case status.Stopped():
			if status.StopSignal() == syscall.SIGTSTP {
				_ = syscall.Kill(-pid, syscall.SIGCONT)
			}
		case status.Exited() && status.ExitStatus() == 0:
			return nil
		case status.Exited():
			return fmt.Errorf("exit status %d", status.ExitStatus())
		default:
			return fmt.Errorf("signal: %v", status.Signal())
		}
	}
}

// sweepGroup ends whatever an exited action left running in its process group:
// SIGTERM, then SIGKILL for whatever is still there two seconds later.
func sweepGroup(pid int) {
	if syscall.Kill(-pid, syscall.SIGTERM) != nil {
		return
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if syscall.Kill(-pid, 0) != nil {
			return
		}
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// reclaimTerminal makes this process's group the terminal's foreground group
// again after an action held it.
func reclaimTerminal(fd int) {
	// Changing the foreground group from a background group raises SIGTTOU, and
	// an ignore cannot be undone, so it stays ignored. The runtime runs one
	// action and exits; revisit before running a second in the same process.
	signal.Ignore(syscall.SIGTTOU)
	_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, syscall.Getpgrp())
}

func runMCPServe(driveRoot string) error {
	srv := mcp.NewDriveServer(driveRoot)
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

func runMCPHTTP(driveRoot string) error {
	srv := mcp.NewDriveServer(driveRoot)
	defer srv.Close()

	listener, port, err := netutil.Listen("127.0.0.1", 8090)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(os.Stdout, "MCP server (Streamable HTTP) listening on http://127.0.0.1:%d%s\n", port, mcp.StreamableHTTPPath)
	return srv.ServeStreamableHTTP(ctx, listener)
}

func runMCP(driveRoot string) error {
	srv := mcp.NewDriveServer(driveRoot)
	defer srv.Close()
	return srv.ServeStdio()
}

func resolveDriveRoot() (string, error) {
	if driveRoot := os.Getenv("DRIVE_ROOT"); driveRoot != "" {
		return driveRoot, nil
	}
	return os.Getwd()
}
