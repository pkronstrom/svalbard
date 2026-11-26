package chat

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

// newFakeDrive lays out a drive with two chat models and a llama-server that
// records its arguments and exits.
func newFakeDrive(t *testing.T, settings string) (driveRoot, record string) {
	t.Helper()
	driveRoot = t.TempDir()
	record = filepath.Join(t.TempDir(), "llama-server.args")
	platformName, err := platform.Detect()
	if err != nil {
		t.Skip(err)
	}
	binDir := filepath.Join(driveRoot, "bin", platformName)
	for _, dir := range []string{binDir, filepath.Join(driveRoot, "models"), filepath.Join(driveRoot, ".svalbard")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > '%s'\n", record)
	if err := os.WriteFile(filepath.Join(binDir, "llama-server"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"alpha-4B.gguf", "beta-12B.gguf"} {
		if err := os.WriteFile(filepath.Join(driveRoot, "models", model), []byte("gguf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if settings != "" {
		if err := os.WriteFile(filepath.Join(driveRoot, ".svalbard", "settings.json"), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return driveRoot, record
}

// runChat starts Chat with a picker that answers 8192 and a browser that
// opens nothing, and returns llama-server's arguments.
func runChat(t *testing.T, driveRoot, record, selected string) (args []string, output string, picked bool) {
	t.Helper()
	previous := pickContext
	pickContext = func(int) int { picked = true; return 8192 }
	t.Cleanup(func() { pickContext = previous })

	var stdout bytes.Buffer
	if err := Run(context.Background(), &stdout, driveRoot, selected, func(string) error { return nil }); err != nil {
		t.Fatalf("Run() error = %v\n%s", err, stdout.String())
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("llama-server was not started: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n"), stdout.String(), picked
}

func argAfter(args []string, flag string) string {
	if i := slices.Index(args, flag); i >= 0 && i+1 < len(args) {
		return args[i+1]
	}
	return ""
}

func TestChatAsksForTheContextAndTakesTheFirstModelByDefault(t *testing.T) {
	driveRoot, record := newFakeDrive(t, "")
	args, _, picked := runChat(t, driveRoot, record, "")
	if !picked {
		t.Error("the context picker was skipped although ai.context is ask")
	}
	if got := argAfter(args, "--ctx-size"); got != "8192" {
		t.Errorf("--ctx-size = %q, want the picked 8192", got)
	}
	if got, want := argAfter(args, "-m"), filepath.Join(driveRoot, "models", "alpha-4B.gguf"); got != want {
		t.Errorf("-m = %q, want %q", got, want)
	}
}

func TestChatHonoursContextAndDefaultModelSettings(t *testing.T) {
	driveRoot, record := newFakeDrive(t, `{"ai": {"context": "balanced", "default_model": "beta-12B"}}`)
	args, output, picked := runChat(t, driveRoot, record, "")
	if picked {
		t.Error("the context picker was shown although ai.context is balanced")
	}
	if got := argAfter(args, "--ctx-size"); got != "32768" {
		t.Errorf("--ctx-size = %q, want 32768", got)
	}
	if got, want := argAfter(args, "-m"), filepath.Join(driveRoot, "models", "beta-12B.gguf"); got != want {
		t.Errorf("-m = %q, want the default model %q", got, want)
	}
	if !strings.Contains(output, "beta-12B.gguf (context 32768)") {
		t.Errorf("output does not name the model and context:\n%s", output)
	}

	// A model named by the menu entry wins over the default model.
	args, _, _ = runChat(t, driveRoot, record, "alpha-4B.gguf")
	if got, want := argAfter(args, "-m"), filepath.Join(driveRoot, "models", "alpha-4B.gguf"); got != want {
		t.Errorf("-m = %q, want the named model %q", got, want)
	}
}

func TestChatFallsBackWhenTheDefaultModelIsGone(t *testing.T) {
	driveRoot, record := newFakeDrive(t, `{"ai": {"default_model": "removed-27B"}}`)
	args, output, picked := runChat(t, driveRoot, record, "")
	if got, want := argAfter(args, "-m"), filepath.Join(driveRoot, "models", "alpha-4B.gguf"); got != want {
		t.Errorf("-m = %q, want today's pick %q", got, want)
	}
	if !strings.Contains(output, "Default model removed-27B is no longer on the drive") {
		t.Errorf("output does not say the default model is gone:\n%s", output)
	}
	if !picked {
		t.Error("the context picker was skipped although ai.context is ask")
	}
}

func TestChatCarriesOnWithDefaultsWhenTheSettingsFileIsBroken(t *testing.T) {
	driveRoot, record := newFakeDrive(t, `{"ai": {"context": "max"`)
	args, output, picked := runChat(t, driveRoot, record, "")
	if !picked || argAfter(args, "--ctx-size") != "8192" {
		t.Errorf("a broken settings file changed the launch: picked=%v args=%v", picked, args)
	}
	if !strings.Contains(output, "Settings ignored, using the defaults") || !strings.Contains(output, "settings.json") {
		t.Errorf("output does not report the broken file:\n%s", output)
	}
}
