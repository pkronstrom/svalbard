package agent_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/agent"
)

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

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "opencode", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 32768)
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

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "goose", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 32768)
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

func TestPrepareClientLaunchConfigForPi(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 65536)
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	if len(cfg.Args) != 2 || cfg.Args[0] != "--model" || cfg.Args[1] != "llama.cpp/qwen" {
		t.Fatalf("Args = %v, want pi model args", cfg.Args)
	}
	piDir := cfg.Env["PI_CODING_AGENT_DIR"]
	if piDir == "" {
		t.Fatal("PI_CODING_AGENT_DIR not set")
	}

	models, err := os.ReadFile(filepath.Join(piDir, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	for _, want := range []string{`"baseUrl": "http://127.0.0.1:8082/v1"`, `"id": "qwen"`, `"contextWindow": 65536`} {
		if !strings.Contains(string(models), want) {
			t.Errorf("models.json missing %s: %s", want, string(models))
		}
	}
	if !json.Valid(models) {
		t.Errorf("qwen models.json is not valid JSON: %s", string(models))
	}
	// Qwen round-trips tool calls without the Gemma chat-template compat flags.
	if strings.Contains(string(models), "requiresToolResultName") {
		t.Errorf("qwen models.json should not carry Gemma compat flags: %s", string(models))
	}

	// A Gemma model must carry the chat-template compat flags pi needs for
	// tool-call round-tripping.
	gemmaCfg, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "gemma-4-E2B-it-qat", 32768)
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig(gemma) error = %v", err)
	}
	gemmaModels, err := os.ReadFile(filepath.Join(gemmaCfg.Env["PI_CODING_AGENT_DIR"], "models.json"))
	if err != nil {
		t.Fatalf("read gemma models.json: %v", err)
	}
	if !json.Valid(gemmaModels) {
		t.Errorf("gemma models.json is not valid JSON: %s", string(gemmaModels))
	}
	for _, want := range []string{`"requiresToolResultName": true`, `"requiresAssistantAfterToolResult": true`} {
		if !strings.Contains(string(gemmaModels), want) {
			t.Errorf("gemma models.json missing %s: %s", want, string(gemmaModels))
		}
	}

	mcp, err := os.ReadFile(filepath.Join(piDir, "mcp.json"))
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(mcp, &parsed); err != nil {
		t.Fatalf("mcp.json not valid JSON: %v", err)
	}
	servers, ok := parsed["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcp.json missing mcpServers")
	}
	svalbard, ok := servers["svalbard"].(map[string]any)
	if !ok {
		t.Fatal("mcp.json missing svalbard server")
	}
	args, ok := svalbard["args"].([]any)
	if !ok || len(args) != 3 || args[0] != "mcp" || args[1] != "--drive" || args[2] != driveRoot {
		t.Errorf("svalbard args = %v, want [mcp --drive %s]", svalbard["args"], driveRoot)
	}
}

func TestPrepareClientLaunchConfigOpenCodeIncludesMCPServers(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "opencode", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "test-model", 32768)
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

	mcpServers, ok := parsed["mcp"]
	if !ok {
		t.Fatal("opencode.json missing mcp key")
	}

	svalbard, ok := mcpServers.(map[string]any)["svalbard"]
	if !ok {
		t.Fatal("mcp missing svalbard entry")
	}

	srv := svalbard.(map[string]any)
	if srv["type"] != "local" {
		t.Errorf("type = %q, want %q", srv["type"], "local")
	}
	if srv["enabled"] != true {
		t.Errorf("enabled = %v, want true", srv["enabled"])
	}

	command, ok := srv["command"].([]any)
	if !ok || len(command) < 4 {
		t.Fatal("svalbard MCP server command missing or too short")
	}
	if command[0] == "" {
		t.Error("svalbard MCP server executable is empty")
	}
	if command[1] != "mcp" {
		t.Errorf("command[1] = %q, want %q", command[1], "mcp")
	}
	if command[2] != "--drive" {
		t.Errorf("command[2] = %q, want %q", command[2], "--drive")
	}
	if command[3] != driveRoot {
		t.Errorf("command[3] = %q, want %q", command[3], driveRoot)
	}
}

func TestPrepareClientLaunchConfigGooseIncludesMCPServers(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "goose", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "test-model", 32768)
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}

	if got := cfg.Env["GOOSE_MCP_SERVERS"]; got != "" {
		t.Fatalf("GOOSE_MCP_SERVERS = %q, want empty", got)
	}

	configPath := filepath.Join(driveRoot, ".svalbard", "runtime", "goose", "config", "goose", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	content := string(data)
	for _, want := range []string{
		"extensions:",
		"  svalbard:",
		"    enabled: true",
		"    type: stdio",
		"    name: svalbard",
		`    cmd: "` + exe + `"`,
		"    args:",
		`      - "` + driveRoot + `"`,
		"    timeout: 300",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config.yaml missing %q:\n%s", want, content)
		}
	}
}
