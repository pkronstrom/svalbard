package agent

import (
	"os"
	"path/filepath"
	"strings"
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

func TestUpsertGooseExtensionYAMLPreservesExistingExtensions(t *testing.T) {
	existing := `extensions:
  developer:
    enabled: true
    type: platform
name: test
`

	updated := upsertGooseExtensionYAML(existing, gooseExtensionBlock("/bin/svalbard-drive", "/tmp/drive"))

	if !strings.Contains(updated, "  developer:") {
		t.Fatalf("updated config lost existing extension:\n%s", updated)
	}
	if !strings.Contains(updated, "  svalbard:") {
		t.Fatalf("updated config missing svalbard extension:\n%s", updated)
	}
	if !strings.Contains(updated, "name: test") {
		t.Fatalf("updated config lost top-level keys:\n%s", updated)
	}
}
