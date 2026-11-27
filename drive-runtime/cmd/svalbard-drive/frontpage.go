package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
	return frontpage.Run(ctx, os.Stdout, driveRoot, cfg, open)
}
