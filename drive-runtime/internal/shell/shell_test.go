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
