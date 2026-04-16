package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func Run(ctx context.Context, stdout io.Writer, driveRoot string) error {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}

	tempDir, err := os.MkdirTemp("", "svalbard-shell-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	sbPath := filepath.Join(tempDir, "sb")
	content := fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\nexport DRIVE_ROOT=%q\nexec %q \"$@\"\n", driveRoot, filepath.Join(driveRoot, "run"))
	if err := os.WriteFile(sbPath, []byte(content), 0o755); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Activated sb shell for %s\n", driveRoot)
	fmt.Fprintln(stdout, "Use `sb`, `sb search`, `sb chat`, `sb opencode`, or `sb goose`. Exit the shell to leave.")

	cmd := exec.CommandContext(ctx, shellPath, "-i")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"DRIVE_ROOT="+driveRoot,
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	return cmd.Run()
}
