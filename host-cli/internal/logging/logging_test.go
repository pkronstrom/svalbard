package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "test.log")

	cleanup, err := Init(Options{LogFile: logPath})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer cleanup()

	slog.Info("hello from test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty after slog.Info")
	}
	if !strings.Contains(string(data), "hello from test") {
		t.Fatalf("log file does not contain expected message, got: %s", string(data))
	}
}

func TestInitStderrMode(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "stderr.log")

	cleanup, err := Init(Options{LogFile: logPath, Stderr: true})
	if err != nil {
		t.Fatalf("Init with Stderr failed: %v", err)
	}
	defer cleanup()

	// Should not panic when logging in stderr mode.
	slog.Info("stderr mode test")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty after slog.Info in stderr mode")
	}
}

func TestInitDebugLevel(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "debug.log")

	cleanup, err := Init(Options{LogFile: logPath, Debug: true})
	if err != nil {
		t.Fatalf("Init with Debug failed: %v", err)
	}
	defer cleanup()

	slog.Debug("debug level message")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "debug level message") {
		t.Fatalf("debug message not found in log file, got: %s", string(data))
	}
}

func TestInitEmptyLogFile(t *testing.T) {
	_, err := Init(Options{})
	if err == nil {
		t.Fatal("expected error for empty LogFile, got nil")
	}
}

func TestDefaultLogPath(t *testing.T) {
	// With vault root
	got := DefaultLogPath("/my/vault")
	want := "/my/vault/.svalbard/svalbard.log"
	if got != want {
		t.Errorf("DefaultLogPath(/my/vault) = %q, want %q", got, want)
	}

	// Fallback with empty vault root
	got = DefaultLogPath("")
	want = filepath.Join(os.TempDir(), "svalbard.log")
	if got != want {
		t.Errorf("DefaultLogPath(\"\") = %q, want %q", got, want)
	}
}
