package runtimeagent_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimeagent"
)

func TestResolveModelSkipsEmbeddingModelsByDefault(t *testing.T) {
	driveRoot := t.TempDir()
	mustWriteModel(t, driveRoot, "bge-small.gguf")
	mustWriteModel(t, driveRoot, "qwen.gguf")

	got, err := runtimeagent.ResolveModel(driveRoot, "")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if want := filepath.Join(driveRoot, "models", "qwen.gguf"); got != want {
		t.Fatalf("ResolveModel() = %q, want %q", got, want)
	}
}

func TestClientEnvironmentUsesLocalOpenAICompatibilityVars(t *testing.T) {
	env := runtimeagent.ClientEnvironment("http://127.0.0.1:8082/v1", "gemma")
	for key, want := range map[string]string{
		"OPENAI_API_KEY":       "local",
		"OPENAI_BASE_URL":      "http://127.0.0.1:8082/v1",
		"OPENAI_API_BASE":      "http://127.0.0.1:8082/v1",
		"OPENAI_MODEL":         "gemma",
		"OPENAI_DEFAULT_MODEL": "gemma",
	} {
		if got := env[key]; got != want {
			t.Fatalf("env[%q] = %q, want %q", key, got, want)
		}
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
