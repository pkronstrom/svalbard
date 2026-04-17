package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/pkronstrom/svalbard/host-cli/internal/cli"
	"github.com/pkronstrom/svalbard/host-cli/internal/logging"
)

func main() {
	logPath := logging.DefaultLogPath("")
	debug := os.Getenv("SVALBARD_DEBUG") != ""
	// Don't log to stderr by default — it corrupts the TUI alt-screen.
	cleanup, err := logging.Init(logging.Options{
		LogFile: logPath,
		Debug:   debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not init logging: %v\n", err)
	} else {
		defer cleanup()
		slog.Debug("logger initialized", "path", logPath)
	}

	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
