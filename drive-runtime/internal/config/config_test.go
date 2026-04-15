package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestLoadRuntimeConfig(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.json")
	content := `{
  "version": 1,
  "preset": "default-32",
  "actions": [
    {
      "section": "browse",
      "label": "Browse encyclopedias",
      "action": "browse",
      "args": {}
    }
  ]
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Version, 1; got != want {
		t.Fatalf("Version = %d, want %d", got, want)
	}
	if got, want := cfg.Actions[0].Action, "browse"; got != want {
		t.Fatalf("Actions[0].Action = %q, want %q", got, want)
	}
}
