package agent

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

// newSettingsDrive lays out a drive with two chat models and the given saved
// settings ("" for no file).
func newSettingsDrive(t *testing.T, settings string) string {
	t.Helper()
	driveRoot := t.TempDir()
	for _, dir := range []string{"models", ".svalbard"} {
		if err := os.MkdirAll(filepath.Join(driveRoot, dir), 0o755); err != nil {
			t.Fatal(err)
		}
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
	return driveRoot
}

// stubPicker replaces the terminal picker with one that answers size and
// reports whether it was asked.
func stubPicker(t *testing.T, size int) *bool {
	t.Helper()
	asked := false
	previous := pickContext
	pickContext = func(int) int { asked = true; return size }
	t.Cleanup(func() { pickContext = previous })
	return &asked
}

func TestLaunchAsksForTheContextAndTakesTheFirstModelByDefault(t *testing.T) {
	driveRoot := newSettingsDrive(t, "")
	asked := stubPicker(t, 8192)

	model, ctxSize, err := resolveLaunch(&bytes.Buffer{}, driveRoot, "pi", "")
	if err != nil {
		t.Fatal(err)
	}
	if !*asked || ctxSize != 8192 {
		t.Errorf("context = %d (asked: %v), want the picker's 8192", ctxSize, *asked)
	}
	if want := filepath.Join(driveRoot, "models", "alpha-4B.gguf"); model != want {
		t.Errorf("model = %q, want %q", model, want)
	}
}

func TestLaunchHonoursContextAndDefaultModelSettings(t *testing.T) {
	driveRoot := newSettingsDrive(t, `{"ai": {"context": "balanced", "default_model": "beta-12B"}}`)
	asked := stubPicker(t, 8192)

	var stdout bytes.Buffer
	model, ctxSize, err := resolveLaunch(&stdout, driveRoot, "pi", "")
	if err != nil {
		t.Fatal(err)
	}
	if *asked || ctxSize != 32768 {
		t.Errorf("context = %d (asked: %v), want 32768 without the picker", ctxSize, *asked)
	}
	if want := filepath.Join(driveRoot, "models", "beta-12B.gguf"); model != want {
		t.Errorf("model = %q, want the default model %q", model, want)
	}

	// A model named by the caller wins over the default model.
	model, _, err = resolveLaunch(&stdout, driveRoot, "pi", "alpha-4B.gguf")
	if want := filepath.Join(driveRoot, "models", "alpha-4B.gguf"); err != nil || model != want {
		t.Errorf("named model = %q, %v; want %q", model, err, want)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected output: %q", stdout.String())
	}
}

func TestLaunchFallsBackWhenTheDefaultModelIsGone(t *testing.T) {
	driveRoot := newSettingsDrive(t, `{"ai": {"default_model": "removed-27B"}}`)
	stubPicker(t, 8192)

	var stdout bytes.Buffer
	model, _, err := resolveLaunch(&stdout, driveRoot, "pi", "")
	if want := filepath.Join(driveRoot, "models", "alpha-4B.gguf"); err != nil || model != want {
		t.Errorf("model = %q, %v; want today's pick %q", model, err, want)
	}
	if !strings.Contains(stdout.String(), "Default model removed-27B is no longer on the drive") {
		t.Errorf("output does not say the default model is gone: %q", stdout.String())
	}
}

func TestOmpIsStillRefusedWhenTheSettingFixesASmallContext(t *testing.T) {
	driveRoot := newSettingsDrive(t, `{"ai": {"context": "fast"}}`)
	asked := stubPicker(t, 131072)

	_, _, err := resolveLaunch(&bytes.Buffer{}, driveRoot, "omp", "")
	if err == nil {
		t.Fatal("omp accepted at 8K")
	}
	// The refusal names the setting, since nothing was picked at this launch.
	for _, want := range []string{"oh-my-pi needs a 32K context or larger (chosen: 8K)", "ai.context = fast", "Settings"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if *asked {
		t.Error("the context picker was shown although ai.context is fast")
	}
	// pi fits the Fast tier, and omp is fine once the setting is large enough.
	if _, _, err := resolveLaunch(&bytes.Buffer{}, driveRoot, "pi", ""); err != nil {
		t.Errorf("pi refused at 8K: %v", err)
	}
	if err := os.WriteFile(filepath.Join(driveRoot, ".svalbard", "settings.json"), []byte(`{"ai": {"context": "balanced"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ctxSize, err := resolveLaunch(&bytes.Buffer{}, driveRoot, "omp", ""); err != nil || ctxSize != 32768 {
		t.Errorf("omp at balanced: %d, %v", ctxSize, err)
	}

	// With the setting at ask, the refusal is the picker's own message.
	if err := os.Remove(filepath.Join(driveRoot, ".svalbard", "settings.json")); err != nil {
		t.Fatal(err)
	}
	stubPicker(t, 8192)
	_, _, err = resolveLaunch(&bytes.Buffer{}, driveRoot, "omp", "")
	if err == nil || strings.Contains(err.Error(), "ai.context") || !strings.Contains(err.Error(), "pick Balanced or Max") {
		t.Errorf("error = %v, want the picker's refusal without the setting", err)
	}
}

func TestRunRefusesOmpBeforeStartingAnything(t *testing.T) {
	driveRoot := newSettingsDrive(t, `{"ai": {"context": "fast"}}`)
	platformName, err := platform.Detect()
	if err != nil {
		t.Skip(err)
	}
	binDir := filepath.Join(driveRoot, "bin", platformName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Launchers that would leave a marker if Run started them.
	marker := filepath.Join(t.TempDir(), "started")
	for _, name := range []string{"omp", "llama-server"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stubPicker(t, 131072)

	err = Run(context.Background(), &bytes.Buffer{}, driveRoot, "omp", "")
	if err == nil || !strings.Contains(err.Error(), "ai.context = fast") {
		t.Fatalf("Run() error = %v, want the refusal naming the setting", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Error("a process was started for a refused launch")
	}
}
