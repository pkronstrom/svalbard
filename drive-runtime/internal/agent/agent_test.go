package agent_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/agent"
)

func TestResolveModelSkipsEmbeddingModelsByDefault(t *testing.T) {
	driveRoot := t.TempDir()
	mustWriteModel(t, driveRoot, "bge-small.gguf")
	mustWriteModel(t, driveRoot, "qwen.gguf")

	got, err := agent.ResolveModel(driveRoot, "")
	if err != nil {
		t.Fatalf("ResolveModel() error = %v", err)
	}
	if want := filepath.Join(driveRoot, "models", "qwen.gguf"); got != want {
		t.Fatalf("ResolveModel() = %q, want %q", got, want)
	}
}

func TestClientEnvironmentUsesLocalOpenAICompatibilityVars(t *testing.T) {
	env := agent.ClientEnvironment("http://127.0.0.1:8082/v1", "gemma")
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

func TestPrepareClientLaunchConfigForOpenCode(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "opencode", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen")
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	if len(cfg.Args) != 2 || cfg.Args[0] != "-m" || cfg.Args[1] != "llama.cpp/qwen" {
		t.Fatalf("Args = %v, want opencode model args", cfg.Args)
	}
	if got := cfg.Env["OPENCODE_CONFIG"]; got == "" {
		t.Fatal("OPENCODE_CONFIG missing")
	}
	if got := cfg.Env["HOME"]; got == "" || got != filepath.Join(driveRoot, ".svalbard", "runtime", "opencode", "home") {
		t.Fatalf("HOME = %q", got)
	}
	data, err := os.ReadFile(cfg.Env["OPENCODE_CONFIG"])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if want := `"model": "llama.cpp/qwen"`; !strings.Contains(string(data), want) {
		t.Fatalf("config missing %s: %s", want, string(data))
	}
	if want := `"baseURL": "http://127.0.0.1:8082/v1"`; !strings.Contains(string(data), want) {
		t.Fatalf("config missing %s: %s", want, string(data))
	}
}

func TestPrepareClientLaunchConfigForGoose(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "goose", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen")
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	if len(cfg.Args) != 0 {
		t.Fatalf("Args = %v, want none", cfg.Args)
	}
	for key, want := range map[string]string{
		"GOOSE_PROVIDER": "openai",
		"GOOSE_MODEL":    "qwen",
		"OPENAI_API_KEY": "local",
		"OPENAI_HOST":    "http://127.0.0.1:8082",
	} {
		if got := cfg.Env[key]; got != want {
			t.Fatalf("Env[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestPrepareClientLaunchConfigOpenCodeIncludesMCPServers(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "opencode", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "test-model")
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}

	configPath := cfg.Env["OPENCODE_CONFIG"]
	if configPath == "" {
		t.Fatal("OPENCODE_CONFIG not set in env")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	mcpServers, ok := parsed["mcpServers"]
	if !ok {
		t.Fatal("opencode.json missing mcpServers key")
	}

	svalbard, ok := mcpServers.(map[string]any)["svalbard"]
	if !ok {
		t.Fatal("mcpServers missing svalbard entry")
	}

	srv := svalbard.(map[string]any)
	if srv["command"] == "" {
		t.Error("svalbard MCP server command is empty")
	}

	args, ok := srv["args"].([]any)
	if !ok || len(args) < 3 {
		t.Fatal("svalbard MCP server args missing or too short")
	}
	if args[0] != "mcp" {
		t.Errorf("args[0] = %q, want %q", args[0], "mcp")
	}
	if args[1] != "--drive" {
		t.Errorf("args[1] = %q, want %q", args[1], "--drive")
	}
	if args[2] != driveRoot {
		t.Errorf("args[2] = %q, want %q", args[2], driveRoot)
	}
}

func TestPrepareClientLaunchConfigGooseIncludesMCPServers(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "goose", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "test-model")
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}

	mcpConfigPath := cfg.Env["GOOSE_MCP_SERVERS"]
	if mcpConfigPath == "" {
		t.Fatal("GOOSE_MCP_SERVERS not set in env")
	}

	data, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	svalbard, ok := parsed["svalbard"]
	if !ok {
		t.Fatal("mcp-servers.json missing svalbard entry")
	}

	srv := svalbard.(map[string]any)
	if srv["command"] == "" {
		t.Error("svalbard MCP server command is empty")
	}

	args, ok := srv["args"].([]any)
	if !ok || len(args) < 3 {
		t.Fatal("svalbard MCP server args missing or too short")
	}
	if args[0] != "mcp" {
		t.Errorf("args[0] = %q, want %q", args[0], "mcp")
	}
	if args[1] != "--drive" {
		t.Errorf("args[1] = %q, want %q", args[1], "--drive")
	}
	if args[2] != driveRoot {
		t.Errorf("args[2] = %q, want %q", args[2], driveRoot)
	}

	expectedDir := filepath.Join(driveRoot, ".svalbard", "runtime", "goose", "config")
	if filepath.Dir(mcpConfigPath) != expectedDir {
		t.Errorf("mcp config dir = %q, want %q", filepath.Dir(mcpConfigPath), expectedDir)
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
