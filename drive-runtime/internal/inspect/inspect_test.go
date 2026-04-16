package inspect_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
)

func TestRunPrintsDriveSummaryAndKnownFileSections(t *testing.T) {
	driveRoot := t.TempDir()
	manifest := "preset: default-32\nregion: default\ncreated: 2026-04-15T10:00:00Z\n"
	if err := os.WriteFile(filepath.Join(driveRoot, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}

	mustWriteFile(t, filepath.Join(driveRoot, "zim", "wikipedia-en-nopic.zim"), []byte("zim"))
	mustWriteFile(t, filepath.Join(driveRoot, "models", "gemma.gguf"), []byte("model"))
	mustWriteFile(t, filepath.Join(driveRoot, "data", "library.sqlite"), []byte("db"))
	mustWriteFile(t, filepath.Join(driveRoot, "maps", "world.pmtiles"), []byte("map"))

	var out bytes.Buffer
	if err := inspect.Run(&out, driveRoot); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"Drive contents",
		"Preset:  default-32",
		"Region:  default",
		"zim/",
		"models/",
		"ZIM files",
		"Models",
		"Databases",
		"Map tiles",
		"wikipedia-en-nopic.zim",
		"gemma.gguf",
		"library.sqlite",
		"world.pmtiles",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
