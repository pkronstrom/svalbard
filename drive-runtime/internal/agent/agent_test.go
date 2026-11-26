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
		// Telemetry stays off, and Goose has no consent question to ask.
		"GOOSE_TELEMETRY_OFF":     "1",
		"GOOSE_TELEMETRY_ENABLED": "false",
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
	// The host's own skills stay out of the prompt (--no-skills).
	if want := []string{"--model", "svalbard/qwen", "--no-skills"}; !reflect.DeepEqual(cfg.Args, want) {
		t.Fatalf("Args = %v, want %v", cfg.Args, want)
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
	// The server is told the model's window: below the Balanced tier it
	// offers less.
	if want := []any{"mcp", "--drive", driveRoot, "--context", "32768"}; !ok || !reflect.DeepEqual(args, want) {
		t.Errorf("svalbard args = %v, want %v", svalbard["args"], want)
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
	// pi keeps 4,096 tokens of whatever window it is told free of every
	// request. Told the real 8,192, it has 4,096 minus its prompt left for
	// the answer, which with the wiki's tools is nothing.
	if got := model["contextWindow"]; got != float64(8192+4096-512) {
		t.Errorf("contextWindow = %v, want the 8K window plus pi's margin less a sixteenth", got)
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
	// Compaction starts when an eighth of the real window is left: counted
	// from the window pi was told, that is 3,584 + 1,024.
	if got := compaction["reserveTokens"]; got != float64(4608) {
		t.Errorf("compaction reserveTokens = %v, want 4608", got)
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
		{"fast", 4608, 2048},
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
	if got := compaction["reserveTokens"]; got != 4608.0 {
		t.Errorf("generated reserveTokens = %v, want 4608", got)
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

func TestPrepareClientLaunchConfigForOmp(t *testing.T) {
	driveRoot := t.TempDir()

	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "omp", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "Qwen3.8-27B-UD-IQ4_XS", 32768)
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	ompDir := cfg.Env["PI_CODING_AGENT_DIR"]
	if want := filepath.Join(driveRoot, ".svalbard", "runtime", "omp", "config"); ompDir != want {
		t.Fatalf("PI_CODING_AGENT_DIR = %q, want %q", ompDir, want)
	}
	// Everything omp writes stays on the drive, including its native addon.
	for key, want := range map[string]string{
		"HOME":           filepath.Join(driveRoot, ".svalbard", "runtime", "omp", "home"),
		"PI_NATIVES_DIR": filepath.Join(driveRoot, ".svalbard", "runtime", "omp", "natives"),
		"PI_NO_TITLE":    "1",
		"OMP_SKIP_SETUP": "1",
	} {
		if got := cfg.Env[key]; got != want {
			t.Errorf("Env[%q] = %q, want %q", key, got, want)
		}
	}
	args := strings.Join(cfg.Args, " ")
	for _, want := range []string{
		"--model svalbard/Qwen3.8-27B-UD-IQ4_XS",
		"--config " + filepath.Join(ompDir, "svalbard.yml"),
		"--offline", "--tools=read,bash,edit,write,grep,glob,mcp__svalbard_search,",
		// The drive's MCP tools are only enabled when named.
		"mcp__svalbard_search_read", "mcp__svalbard_vault_sources", "mcp__svalbard_query_sql",
		"--no-lsp", "--no-skills", "--no-rules", "--no-extensions",
		"--system-prompt " + filepath.Join(ompDir, "lean-prompt.md"),
		"--thinking=medium",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("Args missing %q: %s", want, args)
		}
	}
	// --no-tools would also drop the drive's MCP server.
	if strings.Contains(args, "--no-tools") {
		t.Errorf("Args must keep MCP tools: %s", args)
	}

	models, err := os.ReadFile(filepath.Join(ompDir, "models.yml"))
	if err != nil {
		t.Fatalf("read models.yml: %v", err)
	}
	for _, want := range []string{
		`baseUrl: "http://127.0.0.1:8082/v1"`, "auth: none", `id: "Qwen3.8-27B-UD-IQ4_XS"`,
		"contextWindow: 32768", "maxTokens: 8192", "reasoning: true",
		"thinkingFormat: qwen-chat-template", "qwenTemplateReasoningEffort: true",
	} {
		if !strings.Contains(string(models), want) {
			t.Errorf("models.yml missing %q:\n%s", want, models)
		}
	}
	// pi's template-kwargs compat is not valid in omp.
	if strings.Contains(string(models), "chatTemplateKwargs") {
		t.Errorf("models.yml carries pi-only compat:\n%s", models)
	}

	overlay, err := os.ReadFile(filepath.Join(ompDir, "svalbard.yml"))
	if err != nil {
		t.Fatalf("read svalbard.yml: %v", err)
	}
	for _, want := range []string{
		"checkUpdate: false", "setupWizard: false", "otlpExportEnabled: false",
		`autoUpdate: "off"`, `cacheWarming: "off"`, `backend: "off"`,
		"reserveTokens: 8192", "keepRecentTokens: 8192",
		`default: "svalbard/Qwen3.8-27B-UD-IQ4_XS"`,
	} {
		if !strings.Contains(string(overlay), want) {
			t.Errorf("svalbard.yml missing %q:\n%s", want, overlay)
		}
	}

	var mcp map[string]any
	data, err := os.ReadFile(filepath.Join(ompDir, "mcp.json"))
	if err != nil || json.Unmarshal(data, &mcp) != nil {
		t.Fatalf("mcp.json unreadable: %v", err)
	}
	if _, ok := mcp["mcpServers"].(map[string]any)["svalbard"]; !ok {
		t.Errorf("mcp.json missing the svalbard server: %s", data)
	}
	if prompt, err := os.ReadFile(filepath.Join(ompDir, "lean-prompt.md")); err != nil || len(prompt) == 0 {
		t.Errorf("lean prompt not written: %v", err)
	}
}

func TestPrepareClientLaunchConfigForOmpKeepsUserConfig(t *testing.T) {
	driveRoot := t.TempDir()
	configDir := filepath.Join(driveRoot, ".svalbard", "runtime", "omp", "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userConfig := filepath.Join(configDir, "config.yml")
	if err := os.WriteFile(userConfig, []byte("setupVersion: 2\ntheme: light\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := agent.PrepareClientLaunchConfig(driveRoot, "omp", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "gemma-4-12B-it-qat", 32768); err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	got, err := os.ReadFile(userConfig)
	if err != nil || string(got) != "setupVersion: 2\ntheme: light\n" {
		t.Errorf("user config.yml changed: %q (%v)", got, err)
	}
	models, _ := os.ReadFile(filepath.Join(configDir, "models.yml"))
	for _, want := range []string{"requiresToolResultName: true", "requiresAssistantAfterToolResult: true", "reasoning: false"} {
		if !strings.Contains(string(models), want) {
			t.Errorf("gemma models.yml missing %q:\n%s", want, models)
		}
	}
}

func TestPiSeesTheDriveToolsDirectly(t *testing.T) {
	driveRoot := t.TempDir()
	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 32768)
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
	}
	piDir := cfg.Env["PI_CODING_AGENT_DIR"]

	// pi's default exposure would hide the eight drive tools behind "codemode".
	var mcp struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	data, err := os.ReadFile(filepath.Join(piDir, "mcp.json"))
	if err != nil || json.Unmarshal(data, &mcp) != nil {
		t.Fatalf("mcp.json unreadable: %v", err)
	}
	if got := mcp.MCPServers["svalbard"]["exposure"]; got != "direct" {
		t.Errorf("svalbard exposure = %v, want direct", got)
	}

	contextPath := filepath.Join(piDir, "AGENTS.md")
	context, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("drive context file not written: %v", err)
	}
	for _, want := range []string{driveRoot, "no internet", "mcp__svalbard__search_read"} {
		if !strings.Contains(string(context), want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}

func TestPiDriveContextFollowsTheDriveButKeepsUserEdits(t *testing.T) {
	driveRoot := t.TempDir()
	prepare := func(root string) {
		t.Helper()
		if _, err := agent.PrepareClientLaunchConfig(root, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 32768); err != nil {
			t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
		}
	}
	prepare(driveRoot)
	contextPath := filepath.Join(driveRoot, ".svalbard", "runtime", "pi", "config", "AGENTS.md")

	// The same drive mounted somewhere else: the generated file follows it.
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(driveRoot, moved); err != nil {
		t.Fatal(err)
	}
	prepare(moved)
	contextPath = filepath.Join(moved, ".svalbard", "runtime", "pi", "config", "AGENTS.md")
	if data, _ := os.ReadFile(contextPath); !strings.Contains(string(data), moved) || strings.Contains(string(data), driveRoot) {
		t.Errorf("generated AGENTS.md did not follow the mount point:\n%s", data)
	}

	// Once the user edits it, it is theirs.
	if err := os.WriteFile(contextPath, []byte("my own notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepare(moved)
	if data, _ := os.ReadFile(contextPath); string(data) != "my own notes\n" {
		t.Errorf("user-edited AGENTS.md was overwritten:\n%s", data)
	}
}

// installWiki marks the wiki's recipe as installed on the drive, which is
// what makes the drive's MCP server offer the wiki tools.
func installWiki(t *testing.T, driveRoot string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(driveRoot, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	recipes := `{"silverbullet": {"id": "silverbullet", "type": "binary", "description": "SilverBullet wiki", "tags": []}}`
	if err := os.WriteFile(filepath.Join(driveRoot, ".svalbard", "recipes.json"), []byte(recipes), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPiIsToldAboutTheWikiOnlyWhenItIsInstalled(t *testing.T) {
	contextFor := func(driveRoot string) string {
		t.Helper()
		cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "pi", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 32768)
		if err != nil {
			t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
		}
		data, err := os.ReadFile(filepath.Join(cfg.Env["PI_CODING_AGENT_DIR"], "AGENTS.md"))
		if err != nil {
			t.Fatalf("drive context file not written: %v", err)
		}
		return string(data)
	}

	without := contextFor(t.TempDir())
	if strings.Contains(without, "wiki") {
		t.Errorf("AGENTS.md mentions a wiki the drive does not have:\n%s", without)
	}

	driveRoot := t.TempDir()
	installWiki(t, driveRoot)
	with := contextFor(driveRoot)
	// A model told only that the drive is a read-only library does not try
	// to save anything.
	for _, want := range []string{"The drive also has a wiki", "which you can write", "mcp__svalbard__wiki_create", "mcp__svalbard__wiki_search", "mcp__svalbard__wiki_read"} {
		if !strings.Contains(with, want) {
			t.Errorf("AGENTS.md missing %q:\n%s", want, with)
		}
	}
	// With live pages on, the default, the model is sent to the help page
	// before it writes a page that computes something.
	if !strings.Contains(with, "read the page `Help/Live pages`") {
		t.Errorf("AGENTS.md does not point to the help page:\n%s", with)
	}
	plain := t.TempDir()
	installWiki(t, plain)
	schema := `{"version": 1, "groups": [{"owner": "silverbullet", "label": "Wiki", "settings": [
  {"name": "live_pages", "type": "bool", "default": true, "label": "Agents may write live pages", "description": "Let the AI clients write queries, expressions and scripts into wiki pages."}
]}]}`
	for file, content := range map[string]string{"settings.schema.json": schema, "settings.json": `{"silverbullet": {"live_pages": false}}`} {
		if err := os.WriteFile(filepath.Join(plain, ".svalbard", file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if off := contextFor(plain); strings.Contains(off, "Help/Live pages") || !strings.Contains(off, "The drive also has a wiki") {
		t.Errorf("AGENTS.md with live pages off:\n%s", off)
	}
	// The wiki's paragraph is the only difference, and it sits before the rules.
	paragraph, rules := strings.Index(with, "The drive also has a wiki"), strings.Index(with, "Rules:")
	if paragraph < 0 || rules < paragraph {
		t.Errorf("the wiki paragraph is not before the rules:\n%s", with)
	}
	if strings.Contains(with, "%!") || strings.Contains(without, "%!") {
		t.Errorf("AGENTS.md has a formatting slip:\n%s", with)
	}

	// The file follows the drive: a wiki added later reaches the next launch.
	later := t.TempDir()
	if before := contextFor(later); strings.Contains(before, "wiki") {
		t.Fatalf("AGENTS.md mentions the wiki too early:\n%s", before)
	}
	installWiki(t, later)
	if after := contextFor(later); !strings.Contains(after, "mcp__svalbard__wiki_create") {
		t.Errorf("AGENTS.md did not gain the wiki after it was installed:\n%s", after)
	}
}

func TestOmpGetsTheWikiToolsAndPromptLineOnlyWhenInstalled(t *testing.T) {
	launch := func(driveRoot string) (args, prompt string) {
		t.Helper()
		cfg, err := agent.PrepareClientLaunchConfig(driveRoot, "omp", "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 32768)
		if err != nil {
			t.Fatalf("PrepareClientLaunchConfig() error = %v", err)
		}
		data, err := os.ReadFile(filepath.Join(cfg.Env["PI_CODING_AGENT_DIR"], "lean-prompt.md"))
		if err != nil {
			t.Fatalf("lean prompt not written: %v", err)
		}
		return strings.Join(cfg.Args, " "), string(data)
	}
	wikiTools := []string{"wiki_list", "wiki_read", "wiki_search", "wiki_create", "wiki_append", "wiki_edit", "wiki_move", "wiki_delete"}

	args, prompt := launch(t.TempDir())
	if strings.Contains(args, "wiki") || strings.Contains(prompt, "wiki") {
		t.Errorf("omp was offered a wiki the drive does not have:\n%s\n%s", args, prompt)
	}

	driveRoot := t.TempDir()
	installWiki(t, driveRoot)
	args, prompt = launch(driveRoot)
	// omp enables only the tools it is given by name.
	for _, tool := range wikiTools {
		if !strings.Contains(args, "mcp__svalbard_"+tool) {
			t.Errorf("Args missing mcp__svalbard_%s: %s", tool, args)
		}
	}
	if !strings.Contains(args, "mcp__svalbard_search,") || !strings.Contains(args, "mcp__svalbard_vault_sources") {
		t.Errorf("Args lost the vault tools: %s", args)
	}
	if !strings.Contains(prompt, "wiki tools read and write the user's own notes") {
		t.Errorf("lean prompt missing the wiki line:\n%s", prompt)
	}
}
