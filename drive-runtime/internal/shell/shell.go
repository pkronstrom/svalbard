package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	cmd.Env = MergeEnvironment(os.Environ(), map[string]string{
		"DRIVE_ROOT": driveRoot,
		"PATH":       tempDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	cmd.Env = MergeEnvironment(cmd.Env, envMap(extraEnv))
	return cmd.Run()
}

func newInteractiveShellCommand(ctx context.Context, shellPath, tempDir string) (*exec.Cmd, []string, error) {
	return NewInteractiveShellCommandAfterRC(ctx, shellPath, tempDir, "")
}

// NewInteractiveShellCommandAfterRC preserves the user's normal interactive
// configuration, then appends afterRC. Callers use it when startup files must
// not be allowed to undo an environment supplied by the drive.
func NewInteractiveShellCommandAfterRC(ctx context.Context, shellPath, tempDir, afterRC string) (*exec.Cmd, []string, error) {
	shellName := filepath.Base(shellPath)
	switch shellName {
	case "zsh":
		rcPath := filepath.Join(tempDir, ".zshrc")
		content := `
if [ -f "$HOME/.zshrc" ]; then
  source "$HOME/.zshrc"
fi
PROMPT="(sb) ${PROMPT}"

` + afterRC
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

` + afterRC
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

` + afterRC
		if err := os.WriteFile(configPath, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
			return nil, nil, err
		}
		return exec.CommandContext(ctx, shellPath, "-i"), []string{"XDG_CONFIG_HOME=" + tempDir}, nil
	default:
		return exec.CommandContext(ctx, shellPath, "-i"), []string{"PS1=(sb) ${PS1:-$ }"}, nil
	}
}

// NewInteractiveShellCommandWithEnvironment applies drive-owned values after
// user startup files using syntax for the selected shell.
func NewInteractiveShellCommandWithEnvironment(ctx context.Context, shellPath, tempDir string, values map[string]string, pathPrefix string) (*exec.Cmd, []string, error) {
	shellName := filepath.Base(shellPath)
	switch shellName {
	case "fish":
		return NewInteractiveShellCommandAfterRC(ctx, shellPath, tempDir, fishEnvironment(values, pathPrefix))
	case "bash", "zsh":
		return NewInteractiveShellCommandAfterRC(ctx, shellPath, tempDir, posixEnvironment(values, pathPrefix))
	default:
		return nil, nil, CheckEmbeddedShell(shellPath)
	}
}

// CheckEmbeddedShell reports whether drive values can be applied after the
// shell's startup files, so callers can reject a shell before slow setup.
func CheckEmbeddedShell(shellPath string) error {
	switch shellName := filepath.Base(shellPath); shellName {
	case "fish", "bash", "zsh":
		return nil
	default:
		return fmt.Errorf("embedded shell requires bash, zsh, or fish; got %q", shellName)
	}
}

// MergeEnvironment replaces entries by key, avoiding duplicate KEY=value
// pairs. exec accepts duplicates, but shells do not consistently agree which
// value should win.
func MergeEnvironment(base []string, changes map[string]string) []string {
	result := make([]string, 0, len(base)+len(changes))
	seen := make(map[string]bool, len(base)+len(changes))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if _, replaced := changes[key]; !ok || seen[key] || replaced {
			continue
		}
		seen[key] = true
		result = append(result, entry)
	}
	for key, value := range changes {
		result = append(result, key+"="+value)
	}
	return result
}

func envMap(entries []string) map[string]string {
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func posixEnvironment(values map[string]string, pathPrefix string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var content strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&content, "export %s=%s\n", key, shellSingleQuote(values[key]))
	}
	fmt.Fprintf(&content, "export PATH=%s:\"$PATH\"\n", shellSingleQuote(pathPrefix))
	return content.String()
}

func fishEnvironment(values map[string]string, pathPrefix string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var content strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&content, "set -gx %s %s\n", key, shellSingleQuote(values[key]))
	}
	fmt.Fprintf(&content, "set -gx PATH %s $PATH\n", shellSingleQuote(pathPrefix))
	return content.String()
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
