package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewInteractiveShellCommandForFish(t *testing.T) {
	tempDir := t.TempDir()

	cmd, extraEnv, err := newInteractiveShellCommand(context.Background(), "/usr/bin/fish", tempDir)
	if err != nil {
		t.Fatalf("newInteractiveShellCommand() error = %v", err)
	}

	if got, want := cmd.Args, []string{"/usr/bin/fish", "-i"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("cmd.Args = %v, want %v", got, want)
	}
	if len(extraEnv) != 1 || extraEnv[0] != "XDG_CONFIG_HOME="+tempDir {
		t.Fatalf("extraEnv = %v, want XDG_CONFIG_HOME override", extraEnv)
	}

	configPath := filepath.Join(tempDir, "fish", "config.fish")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `source "$HOME/.config/fish/config.fish"`) {
		t.Fatalf("config missing source of user fish config: %q", text)
	}
	if !strings.Contains(text, `printf "(sb) "`) {
		t.Fatalf("config missing sb prompt prefix: %q", text)
	}
}

func TestInteractiveShellAppendsEnvironmentAfterUserRC(t *testing.T) {
	tempDir := t.TempDir()
	cmd, _, err := NewInteractiveShellCommandAfterRC(context.Background(), "/bin/bash", tempDir, "export PATH=/bundled:$PATH\nexport PLATFORMIO_CORE_DIR=/cache\n")
	if err != nil {
		t.Fatalf("NewInteractiveShellCommandAfterRC() error = %v", err)
	}
	if got, want := cmd.Args, []string{"/bin/bash", "--rcfile", filepath.Join(tempDir, ".bashrc"), "-i"}; len(got) != len(want) || got[1] != want[1] {
		t.Fatalf("cmd.Args = %v, want %v", got, want)
	}
	data, err := os.ReadFile(filepath.Join(tempDir, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Index(text, `source "$HOME/.bashrc"`) > strings.Index(text, "PLATFORMIO_CORE_DIR=/cache") {
		t.Fatalf("after-RC environment was applied before user rc: %q", text)
	}
}

func TestMergeEnvironmentReplacesKeysWithoutDuplicates(t *testing.T) {
	got := MergeEnvironment([]string{"PATH=/old", "HOME=/home", "PATH=/stale"}, map[string]string{"PATH": "/new", "PIP_NO_INDEX": "1"})
	joined := strings.Join(got, "\n")
	if strings.Count(joined, "PATH=") != 1 || !strings.Contains(joined, "PATH=/new") || !strings.Contains(joined, "PIP_NO_INDEX=1") {
		t.Fatalf("MergeEnvironment() = %v", got)
	}
}

func TestEmbeddedEnvironmentIsQuotedAfterBashUserRC(t *testing.T) {
	tempDir := t.TempDir()
	home := t.TempDir()
	binDir := filepath.Join(tempDir, "bin$not-expanded'quoted")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pio := filepath.Join(binDir, "pio")
	if err := os.WriteFile(pio, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("export PATH=/user-overrode\nexport DRIVE_ROOT=/user-overrode\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"DRIVE_ROOT": filepath.Join(tempDir, "vault$literal'quoted"), "PIP_NO_INDEX": "1"}
	cmd, _, err := NewInteractiveShellCommandWithEnvironment(context.Background(), "/bin/bash", tempDir, values, binDir)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Args = append(cmd.Args, "-c", "printf '%s\\n' \"$DRIVE_ROOT\"; command -v pio")
	cmd.Env = MergeEnvironment(os.Environ(), map[string]string{"HOME": home, "PATH": os.Getenv("PATH")})
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash environment command failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), values["DRIVE_ROOT"]) || !strings.Contains(string(output), pio) {
		t.Fatalf("post-RC environment output = %q", output)
	}
}

func TestEmbeddedFishEnvironmentUsesFishSyntax(t *testing.T) {
	tempDir := t.TempDir()
	_, _, err := NewInteractiveShellCommandWithEnvironment(context.Background(), "/usr/bin/fish", tempDir, map[string]string{"DRIVE_ROOT": "/vault$literal'quoted"}, "/bin$literal'quoted")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tempDir, "fish", "config.fish"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "set -gx DRIVE_ROOT") || !strings.Contains(text, "set -gx PATH") || strings.Contains(text, "export DRIVE_ROOT") {
		t.Fatalf("fish after-RC environment = %q", text)
	}
}
