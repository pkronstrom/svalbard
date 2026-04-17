package main

import (
	"fmt"
	"os"

	"github.com/pkronstrom/svalbard/host-cli/internal/cli"
	"github.com/pkronstrom/svalbard/host-cli/internal/logging"
)

func main() {
	logPath := logging.DefaultLogPath("")
	debug := os.Getenv("SVALBARD_DEBUG") != ""
	cleanup, err := logging.Init(logging.Options{
		LogFile: logPath,
		Stderr:  true,
		Debug:   debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not init logging: %v\n", err)
	} else {
		defer cleanup()
	}

	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
