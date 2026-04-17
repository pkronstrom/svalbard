package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/host-tui/internal/dashboard"
	"github.com/pkronstrom/svalbard/host-tui/internal/vault"
	"github.com/pkronstrom/svalbard/host-tui/internal/welcome"
	"github.com/pkronstrom/svalbard/host-tui/internal/wizard"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Set up file-only logging (TUI owns stderr).
	logPath := filepath.Join(os.TempDir(), "svalbard.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			level := slog.LevelInfo
			if os.Getenv("SVALBARD_DEBUG") != "" {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})))
			defer f.Close()
		}
	}

	// Check for init subcommand
	if len(os.Args) > 1 && os.Args[1] == "init" {
		prefillPath := ""
		if len(os.Args) > 2 {
			prefillPath = os.Args[2]
		}
		return runWizard(prefillPath)
	}

	// Try to resolve vault from cwd
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Resolve(cwd)
	if err != nil {
		return runWelcome()
	}

	return runDashboard(vaultPath)
}

func runDashboard(vaultPath string) error {
	p := tea.NewProgram(dashboard.New(vaultPath), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runWelcome() error {
	p := tea.NewProgram(welcome.New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runWizard(prefillPath string) error {
	// Standalone TUI binary — no catalog available, so wizard gets empty config.
	// For full wizard experience, use the host-cli `svalbard` binary instead.
	config := wizard.WizardConfig{
		PrefillPath: prefillPath,
	}
	p := tea.NewProgram(wizard.New(config), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
