package main

import (
	"fmt"
	"os"

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
