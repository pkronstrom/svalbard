package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRuntimeLogFileCreatesClientScopedLog(t *testing.T) {
	driveRoot := t.TempDir()

	file, path, err := openRuntimeLogFile(driveRoot, "opencode", "llama-server.log")
	if err != nil {
		t.Fatalf("openRuntimeLogFile() error = %v", err)
	}
	defer file.Close()

	want := filepath.Join(driveRoot, ".svalbard", "runtime", "opencode", "llama-server.log")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("runtime log dir missing: %v", err)
	}
}
