package runtimechat_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimechat"
)

func TestResolveModelSkipsEmbeddingModelsByDefault(t *testing.T) {
	driveRoot := t.TempDir()
	mustWriteModel(t, driveRoot, "nomic-embed-text.gguf")
	mustWriteModel(t, driveRoot, "gemma.gguf")

	got, err := runtimechat.ResolveModel(driveRoot, "")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if want := filepath.Join(driveRoot, "models", "gemma.gguf"); got != want {
		t.Fatalf("ResolveModel() = %q, want %q", got, want)
	}
}

func TestResolveModelUsesExplicitSelection(t *testing.T) {
	driveRoot := t.TempDir()
	mustWriteModel(t, driveRoot, "gemma.gguf")

	got, err := runtimechat.ResolveModel(driveRoot, filepath.Join(driveRoot, "models", "gemma.gguf"))
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if want := filepath.Join(driveRoot, "models", "gemma.gguf"); got != want {
		t.Fatalf("ResolveModel() = %q, want %q", got, want)
	}
}

func mustWriteModel(t *testing.T, driveRoot, name string) {
	t.Helper()
	path := filepath.Join(driveRoot, "models", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("model"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
