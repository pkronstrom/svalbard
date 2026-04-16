package serveall_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/serveall"
)

func TestPlanServicesIncludesAvailableContentAndTooling(t *testing.T) {
	driveRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(driveRoot, "zim", "wiki.zim"))
	mustWriteFile(t, filepath.Join(driveRoot, "models", "gemma.gguf"))
	mustWriteFile(t, filepath.Join(driveRoot, "apps", "map", "index.html"))

	plan := serveall.PlanServices(driveRoot, map[string]string{
		"kiwix-serve":  "/tmp/bin/kiwix-serve",
		"llama-server": "/tmp/bin/llama-server",
	})

	if len(plan) != 4 {
		t.Fatalf("len(plan) = %d, want 4", len(plan))
	}
	if plan[0].Name != "kiwix" || plan[1].Name != "llm" || plan[2].Name != "files" || plan[3].Name != "map" {
		t.Fatalf("plan = %#v, want kiwix/llm/files/map order", plan)
	}
}

func TestPlanServicesOmitsUnavailableServices(t *testing.T) {
	driveRoot := t.TempDir()

	plan := serveall.PlanServices(driveRoot, map[string]string{})
	if len(plan) != 1 || plan[0].Name != "files" {
		t.Fatalf("plan = %#v, want only files service", plan)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
