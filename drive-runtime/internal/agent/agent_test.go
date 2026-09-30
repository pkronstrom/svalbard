package agent_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	if len(cfg.Args) != 2 || cfg.Args[0] != "--model" || cfg.Args[1] != "svalbard/qwen" {
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

func TestPrepareClientLaunchConfigForPiQwen38UsesNativeDefaults(t *testing.T) {
	driveRoot := filepath.Join(t.TempDir(), "drive with spaces")
	modelName := "Qwen3.8-27B-UD-Q4_K_M"
	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", modelName, 8192)
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	if got := cfg.Env["PI_OFFLINE"]; got != "1" {
		t.Fatalf("PI_OFFLINE = %q, want 1", got)
	}

	models := readJSON(t, filepath.Join(cfg.Env["PI_CODING_AGENT_DIR"], "models.json"))
	provider := models["providers"].(map[string]any)["svalbard"].(map[string]any)
	model := provider["models"].([]any)[0].(map[string]any)
	if got := model["maxTokens"]; got != float64(4096) {
		t.Errorf("maxTokens = %v, want 4096 for an 8K context", got)
	}
	if got := model["reasoning"]; got != true {
		t.Errorf("reasoning = %v, want true", got)
	}
	if got := model["cost"]; !reflect.DeepEqual(got, map[string]any{"input": 0.0, "output": 0.0, "cacheRead": 0.0, "cacheWrite": 0.0}) {
		t.Errorf("cost = %#v, want zero rates for input, output, cache read, and cache write", got)
	}
	compat := model["compat"].(map[string]any)
	for key, want := range map[string]any{
		"thinkingFormat": "chat-template", "supportsReasoningEffort": true,
		"chatTemplateKwargs": map[string]any{
			"enable_thinking":   map[string]any{"$var": "thinking.enabled"},
			"preserve_thinking": true,
			"reasoning_effort":  map[string]any{"$var": "thinking.effort", "omitWhenOff": true},
		},
	} {
		if got := compat[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("compat[%q] = %v, want %v", key, got, want)
		}
	}
	wantSampling := map[string]any{
		"temperature": 1.0, "top_p": 0.95, "top_k": float64(20), "min_p": 0.0,
		"presence_penalty": 0.0, "repeat_penalty": 1.0,
	}
	if got := model["samplingParams"]; !reflect.DeepEqual(got, wantSampling) {
		t.Errorf("samplingParams = %#v, want %#v", got, wantSampling)
	}

	settings := readJSON(t, filepath.Join(cfg.Env["PI_CODING_AGENT_DIR"], "settings.json"))
	if got := settings["modelThinkingLevels"].(map[string]any)["svalbard/"+modelName]; got != "medium" {
		t.Errorf("Qwen thinking level = %v, want medium", got)
	}
	compaction := settings["compaction"].(map[string]any)
	if got := compaction["reserveTokens"]; got != float64(4096) {
		t.Errorf("compaction reserveTokens = %v, want 4096", got)
	}
	if got := compaction["keepRecentTokens"]; got != float64(2048) {
		t.Errorf("compaction keepRecentTokens = %v, want 2048", got)
	}

	mcp := readJSON(t, filepath.Join(cfg.Env["PI_CODING_AGENT_DIR"], "mcp.json"))
	args := mcp["mcpServers"].(map[string]any)["svalbard"].(map[string]any)["args"].([]any)
	if got := args[2]; got != driveRoot {
		t.Errorf("MCP drive arg = %q, want %q", got, driveRoot)
	}
}

func TestPrepareClientLaunchConfigForPiPreservesSettings(t *testing.T) {
	driveRoot := t.TempDir()
	configRoot := filepath.Join(driveRoot, ".svalbard", "runtime", "pi", "config")
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configRoot, "settings.json")
	const existing = `{"compaction":{"reserveTokens":777},"modelThinkingLevels":{"svalbard/Qwen3.8":"high"},"custom":true}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "Qwen3.8", 8192); err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	settings := readJSON(t, settingsPath)
	if got := settings["compaction"].(map[string]any)["reserveTokens"]; got != float64(777) {
		t.Errorf("custom reserveTokens = %v, want 777", got)
	}
	if got := settings["compaction"].(map[string]any)["keepRecentTokens"]; got != float64(2048) {
		t.Errorf("generated keepRecentTokens = %v, want 2048", got)
	}
	if got := settings["modelThinkingLevels"].(map[string]any)["svalbard/Qwen3.8"]; got != "high" {
		t.Errorf("custom thinking level = %v, want high", got)
	}
	if got := settings["custom"]; got != true {
		t.Errorf("custom setting = %v, want true", got)
	}
}

func TestPrepareClientLaunchConfigForPiRefreshesGeneratedCompaction(t *testing.T) {
	driveRoot := t.TempDir()
	for _, tc := range []struct {
		name                  string
		wantReserve, wantKeep int
	}{
		{"max", 8192, 20000},
		{"fast", 4096, 2048},
		{"max again", 8192, 20000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := 131072
			if tc.name == "fast" {
				ctx = 8192
			}
			cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "Qwen3.8", ctx)
			if err != nil {
				t.Fatal(err)
			}
			compaction := readJSON(t, filepath.Join(cfg.Env["PI_CODING_AGENT_DIR"], "settings.json"))["compaction"].(map[string]any)
			if got := compaction["reserveTokens"]; got != float64(tc.wantReserve) {
				t.Errorf("reserveTokens = %v, want %d", got, tc.wantReserve)
			}
			if got := compaction["keepRecentTokens"]; got != float64(tc.wantKeep) {
				t.Errorf("keepRecentTokens = %v, want %d", got, tc.wantKeep)
			}
		})
	}
}

func TestPrepareClientLaunchConfigForPiPreservesModifiedGeneratedCompactionField(t *testing.T) {
	driveRoot := t.TempDir()
	if _, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "Qwen3.8", 131072); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(driveRoot, ".svalbard", "runtime", "pi", "config", "settings.json")
	settings := readJSON(t, settingsPath)
	settings["compaction"].(map[string]any)["reserveTokens"] = 333.0
	writeTestJSON(t, settingsPath, settings)

	if _, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "Qwen3.8", 8192); err != nil {
		t.Fatal(err)
	}
	compaction := readJSON(t, settingsPath)["compaction"].(map[string]any)
	if got := compaction["reserveTokens"]; got != 333.0 {
		t.Errorf("user reserveTokens = %v, want 333", got)
	}
	if got := compaction["keepRecentTokens"]; got != 2048.0 {
		t.Errorf("generated keepRecentTokens = %v, want 2048", got)
	}
	if _, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "Qwen3.8", 131072); err != nil {
		t.Fatal(err)
	}
	compaction = readJSON(t, settingsPath)["compaction"].(map[string]any)
	if got := compaction["reserveTokens"]; got != 333.0 {
		t.Errorf("user reserveTokens after context change = %v, want 333", got)
	}
	if got := compaction["keepRecentTokens"]; got != 20000.0 {
		t.Errorf("generated keepRecentTokens after context change = %v, want 20000", got)
	}
}

func TestPrepareClientLaunchConfigForPiFillsMissingCustomCompactionReserve(t *testing.T) {
	driveRoot := t.TempDir()
	configRoot := filepath.Join(driveRoot, ".svalbard", "runtime", "pi", "config")
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configRoot, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"compaction":{"keepRecentTokens":111}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "Qwen3.8", 8192); err != nil {
		t.Fatal(err)
	}
	compaction := readJSON(t, settingsPath)["compaction"].(map[string]any)
	if got := compaction["reserveTokens"]; got != 4096.0 {
		t.Errorf("generated reserveTokens = %v, want 4096", got)
	}
	if got := compaction["keepRecentTokens"]; got != 111.0 {
		t.Errorf("custom keepRecentTokens = %v, want 111", got)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return value
}

func writeTestJSON(t *testing.T, path string, value map[string]any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
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
