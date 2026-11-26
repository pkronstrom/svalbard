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

func TestOmpIsRefusedBelowTheBalancedContext(t *testing.T) {
	err := checkClientContext("omp", 8192)
	if err == nil {
		t.Fatal("omp accepted at 8K, where its prompt leaves no room for work")
	}
	for _, want := range []string{"32K", "8K", "Balanced", "pi"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if err := checkClientContext("omp", 32768); err != nil {
		t.Errorf("omp refused at 32K: %v", err)
	}
	if err := checkClientContext("pi", 8192); err != nil {
		t.Errorf("pi refused at 8K: %v", err)
	}
}
