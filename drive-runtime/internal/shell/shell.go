package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	cmd, extraEnv, err := newInteractiveShellCommand(ctx, shellPath, tempDir)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"DRIVE_ROOT="+driveRoot,
		"PATH="+tempDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	return cmd.Run()
}

func newInteractiveShellCommand(ctx context.Context, shellPath, tempDir string) (*exec.Cmd, []string, error) {
	shellName := filepath.Base(shellPath)
	switch shellName {
	case "zsh":
		rcPath := filepath.Join(tempDir, ".zshrc")
		content := `
if [ -f "$HOME/.zshrc" ]; then
  source "$HOME/.zshrc"
fi
PROMPT="(sb) ${PROMPT}"
`
		if err := os.WriteFile(rcPath, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
			return nil, nil, err
		}
		return exec.CommandContext(ctx, shellPath, "-i"), []string{"ZDOTDIR=" + tempDir}, nil
	case "bash":
		rcPath := filepath.Join(tempDir, ".bashrc")
		content := `
if [ -f "$HOME/.bashrc" ]; then
  source "$HOME/.bashrc"
fi
PS1="(sb) ${PS1:-\s-\v\$ }"
`
		if err := os.WriteFile(rcPath, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
			return nil, nil, err
		}
		return exec.CommandContext(ctx, shellPath, "--rcfile", rcPath, "-i"), nil, nil
	case "fish":
		configDir := filepath.Join(tempDir, "fish")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, nil, err
		}
		configPath := filepath.Join(configDir, "config.fish")
		content := `
if test -f "$HOME/.config/fish/config.fish"
    source "$HOME/.config/fish/config.fish"
end
if functions -q fish_prompt
    functions -c fish_prompt __sb_original_fish_prompt
end
function fish_prompt
    printf "(sb) "
    if functions -q __sb_original_fish_prompt
        __sb_original_fish_prompt
    else
        printf "%s> " (prompt_pwd)
    end
end
`
		if err := os.WriteFile(configPath, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
			return nil, nil, err
		}
		return exec.CommandContext(ctx, shellPath, "-i"), []string{"XDG_CONFIG_HOME=" + tempDir}, nil
	default:
		return exec.CommandContext(ctx, shellPath, "-i"), []string{"PS1=(sb) ${PS1:-$ }"}, nil
	}
}
