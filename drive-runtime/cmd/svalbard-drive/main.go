package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/menu"
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

	cfg, err := config.Load(filepath.Join(driveRoot, ".svalbard", "runtime.json"))
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
