package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/contextpicker"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

type LaunchConfig struct {
	Args []string
	Env  map[string]string
}

func ClientEnvironment(baseURL, modelName string) map[string]string {
	return map[string]string{
		"OPENAI_API_KEY":       "local",
		"OPENAI_BASE_URL":      baseURL,
		"OPENAI_API_BASE":      baseURL,
		"OPENAI_MODEL":         modelName,
		"OPENAI_DEFAULT_MODEL": modelName,
	}
}

func PrepareClientLaunchConfig(driveRoot, clientName, hostRoot, baseURL, modelName string, ctxSize int) (LaunchConfig, error) {
	cfg := LaunchConfig{Env: map[string]string{}}
	runtimeRoot := filepath.Join(driveRoot, ".svalbard", "runtime", clientName)
	configRoot := filepath.Join(runtimeRoot, "config")
	cacheRoot := filepath.Join(runtimeRoot, "cache")
	dataRoot := filepath.Join(runtimeRoot, "data")
	homeRoot := filepath.Join(runtimeRoot, "home")

	switch clientName {
	case "opencode":
		for _, dir := range []string{configRoot, cacheRoot, dataRoot, homeRoot} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return LaunchConfig{}, err
			}
		}
		mcpBinary, err := os.Executable()
		if err != nil {
			return LaunchConfig{}, fmt.Errorf("resolve mcp binary: %w", err)
		}
		configPath := filepath.Join(configRoot, "opencode.json")
		content := fmt.Sprintf(`{
  "$schema": "https://opencode.ai/config.json",
  "enabled_providers": ["llama.cpp"],
  "model": "llama.cpp/%[1]s",
  "small_model": "llama.cpp/%[1]s",
  "provider": {
    "llama.cpp": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "llama-server (local)",
      "options": {
        "baseURL": "%[2]s",
        "apiKey": "local"
      },
      "models": {
        "%[1]s": {
          "name": "%[1]s",
          "limit": { "context": %[5]d, "output": 8192 }
        }
      }
    }
  },
  "mcp": {
    "svalbard": {
      "type": "local",
      "command": ["%[3]s", "mcp", "--drive", "%[4]s"],
      "enabled": true
    }
  }
}
`, modelName, baseURL, mcpBinary, driveRoot, ctxSize)
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			return LaunchConfig{}, err
		}
		cfg.Args = []string{"-m", "llama.cpp/" + modelName}
		cfg.Env = map[string]string{
			"HOME":            homeRoot,
			"XDG_CONFIG_HOME": configRoot,
			"XDG_CACHE_HOME":  cacheRoot,
			"XDG_DATA_HOME":   dataRoot,
			"OPENCODE_CONFIG": configPath,
		}
	case "goose":
		for _, dir := range []string{configRoot, cacheRoot, dataRoot, homeRoot} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return LaunchConfig{}, err
			}
		}
		mcpBinary, err := os.Executable()
		if err != nil {
			return LaunchConfig{}, fmt.Errorf("resolve mcp binary: %w", err)
		}
		gooseConfigPath := filepath.Join(configRoot, "goose", "config.yaml")
		if err := ensureGooseStdioExtension(gooseConfigPath, mcpBinary, driveRoot); err != nil {
			return LaunchConfig{}, fmt.Errorf("write goose extension config: %w", err)
		}
		cfg.Env = map[string]string{
			"HOME":                homeRoot,
			"XDG_CONFIG_HOME":     configRoot,
			"XDG_CACHE_HOME":      cacheRoot,
			"XDG_DATA_HOME":       dataRoot,
			"GOOSE_PROVIDER":      "openai",
			"GOOSE_MODEL":         modelName,
			"GOOSE_CONTEXT_LIMIT": fmt.Sprintf("%d", ctxSize),
			"OPENAI_API_KEY":      "local",
			"OPENAI_HOST":         hostRoot,
			// GOOSE_TELEMETRY_OFF is the environment switch; the config key
			// records the choice so Goose has no consent question to ask.
			"GOOSE_TELEMETRY_OFF":     "1",
			"GOOSE_TELEMETRY_ENABLED": "false",
		}
	case "pi":
		for _, dir := range []string{configRoot, cacheRoot, dataRoot, homeRoot} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return LaunchConfig{}, err
			}
		}
		mcpBinary, err := os.Executable()
		if err != nil {
			return LaunchConfig{}, fmt.Errorf("resolve mcp binary: %w", err)
		}
		// Gemma's chat template requires every tool result to carry a name and
		// to be followed by an assistant turn; pi needs these per-model compat
		// flags to round-trip tool calls (mirrors a verified turbollm pi config).
		// Other families (Qwen) round-trip without them.
		isGemma := strings.Contains(strings.ToLower(modelName), "gemma")
		if err := writePiModels(filepath.Join(configRoot, "models.json"), baseURL, modelName, ctxSize, isGemma); err != nil {
			return LaunchConfig{}, err
		}
		// pi supports MCP natively (no plugin) via mcp.json — wire the svalbard
		// stdio server, at parity with OpenCode/Goose. pi's default exposure
		// hides a server's tools behind one "codemode" script tool; "direct"
		// declares them like built-ins, which small local models need.
		if err := writeJSON(filepath.Join(configRoot, "mcp.json"), svalbardMCPServers(mcpBinary, driveRoot, map[string]any{"exposure": "direct"})); err != nil {
			return LaunchConfig{}, err
		}
		if err := writeGeneratedFile(filepath.Join(configRoot, "AGENTS.md"), fmt.Sprintf(piDriveContext, driveRoot)); err != nil {
			return LaunchConfig{}, err
		}
		if err := ensurePiSettings(filepath.Join(configRoot, "settings.json"), modelName, ctxSize); err != nil {
			return LaunchConfig{}, err
		}
		// No skill discovery: pi also walks the working folder's parents, so a
		// host with ~/.agents/skills adds its whole skill list to every prompt
		// (about 5,000 tokens for 80 skills, more than the Fast tier holds).
		cfg.Args = []string{"--model", piProvider + "/" + modelName, "--no-skills"}
		cfg.Env = map[string]string{
			"HOME":                homeRoot,
			"XDG_CONFIG_HOME":     configRoot,
			"XDG_CACHE_HOME":      cacheRoot,
			"XDG_DATA_HOME":       dataRoot,
			"PI_CODING_AGENT_DIR": configRoot,
			"PI_OFFLINE":          "1",
		}
	case "omp":
		// oh-my-pi unpacks a native addon on first run; keep it on the drive.
		nativesRoot := filepath.Join(runtimeRoot, "natives")
		for _, dir := range []string{configRoot, cacheRoot, dataRoot, homeRoot, nativesRoot} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return LaunchConfig{}, err
			}
		}
		mcpBinary, err := os.Executable()
		if err != nil {
			return LaunchConfig{}, fmt.Errorf("resolve mcp binary: %w", err)
		}
		// omp reads the same mcp.json as pi, but its own models.yml: the compat
		// schema differs, so pi's models.json does not carry over.
		if err := os.WriteFile(filepath.Join(configRoot, "models.yml"), []byte(ompModelsYAML(baseURL, modelName, ctxSize)), 0o644); err != nil {
			return LaunchConfig{}, err
		}
		if err := writeJSON(filepath.Join(configRoot, "mcp.json"), svalbardMCPServers(mcpBinary, driveRoot, nil)); err != nil {
			return LaunchConfig{}, err
		}
		// The user's own config.yml is left alone apart from marking setup as
		// done; the offline profile is a per-run overlay passed with --config.
		userConfig := filepath.Join(configRoot, "config.yml")
		if _, err := os.Stat(userConfig); os.IsNotExist(err) {
			if err := os.WriteFile(userConfig, []byte("setupVersion: 2\n"), 0o644); err != nil {
				return LaunchConfig{}, err
			}
		}
		overlayPath := filepath.Join(configRoot, "svalbard.yml")
		if err := os.WriteFile(overlayPath, []byte(ompOverlayYAML(modelName, ctxSize)), 0o644); err != nil {
			return LaunchConfig{}, err
		}
		promptPath := filepath.Join(configRoot, "lean-prompt.md")
		if err := os.WriteFile(promptPath, []byte(ompLeanPrompt), 0o644); err != nil {
			return LaunchConfig{}, err
		}
		// The lean profile: six file and shell tools plus the drive's MCP
		// tools. The default tool set and prompt cost about 7,700 tokens. omp
		// enables only the tools named here, so the MCP tools are listed too,
		// under the names omp gives them (mcp__<server>_<tool>).
		tools := []string{"read", "bash", "edit", "write", "grep", "glob"}
		for _, name := range mcp.DriveToolNames(driveRoot) {
			tools = append(tools, "mcp__svalbard_"+name)
		}
		cfg.Args = []string{
			"--model", piProvider + "/" + modelName,
			"--config", overlayPath,
			"--offline", "--tools=" + strings.Join(tools, ","),
			"--no-lsp", "--no-skills", "--no-rules", "--no-extensions",
			"--system-prompt", promptPath,
		}
		if strings.Contains(strings.ToLower(modelName), "qwen3.8") {
			cfg.Args = append(cfg.Args, "--thinking=medium")
		}
		cfg.Env = map[string]string{
			"HOME":                homeRoot,
			"XDG_CONFIG_HOME":     configRoot,
			"XDG_CACHE_HOME":      cacheRoot,
			"XDG_DATA_HOME":       dataRoot,
			"PI_CODING_AGENT_DIR": configRoot,
			"PI_NATIVES_DIR":      nativesRoot,
			"PI_NO_TITLE":         "1",
			"OMP_SKIP_SETUP":      "1",
		}
	default:
		cfg.Env = map[string]string{}
	}
	return cfg, nil
}

// piProvider names the local endpoint in Pi's models.json. Pi reserves
// "llama.cpp" for its built-in router integration and rejects a custom
// provider with that ID ("Provider is not configured: llama.cpp").
const piProvider = "svalbard"

func writePiModels(path, baseURL, modelName string, ctxSize int, isGemma bool) error {
	model := map[string]any{
		"id": modelName, "contextWindow": ctxSize, "maxTokens": piMaxTokens(ctxSize),
		"cost": map[string]float64{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
	}
	if sampling := llamaserve.SamplingParams(modelName); len(sampling) > 0 {
		model["samplingParams"] = sampling
	}
	if strings.Contains(strings.ToLower(modelName), "qwen3.8") {
		model["reasoning"] = true
		model["compat"] = map[string]any{
			"thinkingFormat": "chat-template", "supportsReasoningEffort": true,
			"chatTemplateKwargs": map[string]any{
				"enable_thinking":   map[string]string{"$var": "thinking.enabled"},
				"preserve_thinking": true,
				"reasoning_effort":  map[string]any{"$var": "thinking.effort", "omitWhenOff": true},
			},
		}
	} else if isGemma {
		model["compat"] = map[string]any{"thinkingFormat": "qwen-chat-template", "requiresToolResultName": true, "requiresAssistantAfterToolResult": true}
	}
	return writeJSON(path, map[string]any{"providers": map[string]any{
		piProvider: map[string]any{
			"baseUrl": baseURL, "api": "openai-completions", "apiKey": "local",
			"compat": map[string]any{"supportsDeveloperRole": false, "supportsReasoningEffort": false}, "models": []any{model},
		},
	}})
}

// svalbardMCPServers is the mcp.json both pi and oh-my-pi read; extra holds
// client-specific keys for the svalbard server entry.
func svalbardMCPServers(mcpBinary, driveRoot string, extra map[string]any) map[string]any {
	server := map[string]any{"command": mcpBinary, "args": []string{"mcp", "--drive", driveRoot}}
	for key, value := range extra {
		server[key] = value
	}
	return map[string]any{"mcpServers": map[string]any{"svalbard": server}}
}

// piDriveContext is pi's global context file: what a small model has to be
// told to use the drive instead of the network. pi names MCP tools
// mcp__<server>__<tool>.
const piDriveContext = `# Svalbard offline drive

You run from a Svalbard USB drive at ` + "`%s`" + `. There is no internet. Do not try curl, wget, git clone, pip install, npm install or any other download. They fail.

The drive has an offline library: reference archives, SQLite databases and maps. For a factual question, look it up with the ` + "`mcp__svalbard__`" + ` tools instead of guessing:

1. ` + "`mcp__svalbard__vault_sources`" + ` lists what is on the drive. Call it first.
2. ` + "`mcp__svalbard__search`" + ` finds articles by keyword.
3. ` + "`mcp__svalbard__search_read`" + ` reads one article. Pass the exact ` + "`source`" + ` and ` + "`path`" + ` from a search result.

Rules:

- Name the source you used.
- If the drive has no answer, say so. Do not invent one.
- Do not repeat a tool call with the same arguments. If it failed twice, stop and tell the user.
- ` + "`./run`" + ` on the drive is an interactive menu for the user. Do not run it.
`

// writeGeneratedFile keeps a Svalbard-written file current without touching
// the user's edits: the file is replaced only while it still matches what
// Svalbard last wrote, recorded in a ".svalbard-sha256" file beside it.
func writeGeneratedFile(path, content string) error {
	markerPath := path + ".svalbard-sha256"
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		marker, _ := os.ReadFile(markerPath)
		if sha256Hex(existing) != strings.TrimSpace(string(marker)) {
			return nil
		}
		if string(existing) == content {
			return nil
		}
	case !os.IsNotExist(err):
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte(sha256Hex([]byte(content))+"\n"), 0o644)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ompMinContext is the smallest context oh-my-pi is offered at. Its lean
// prompt and tool schemas take about 2,500 tokens, which leaves too little of
// the 8K tier for the work itself.
const ompMinContext = llamaserve.CtxBalanced

func checkClientContext(clientName string, ctxSize int) error {
	if clientName == "omp" && ctxSize < ompMinContext {
		return fmt.Errorf("oh-my-pi needs a %dK context or larger (chosen: %dK): pick Balanced or Max, or use pi, which fits the Fast tier",
			ompMinContext/1024, ctxSize/1024)
	}
	return nil
}

const ompLeanPrompt = `You are a coding assistant running offline from a Svalbard drive against a local model.

Inspect the relevant files before changing them. Use the provided tools rather than guessing. Keep edits focused on what was asked. Run the smallest relevant check after a change, then state which files changed and what the check showed.

There is no network. The svalbard tools search and read the reference material stored on this drive.
`

// ompModelsYAML is oh-my-pi's provider file for the drive's llama-server. It
// has no sampling fields: llama-server already applies the per-family sampler.
func ompModelsYAML(baseURL, modelName string, ctxSize int) string {
	lower := strings.ToLower(modelName)
	isQwen38 := strings.Contains(lower, "qwen3.8")
	var b strings.Builder
	fmt.Fprintf(&b, "providers:\n  %s:\n    baseUrl: %q\n    api: openai-completions\n    auth: none\n    models:\n", piProvider, baseURL)
	fmt.Fprintf(&b, "      - id: %q\n        name: %q\n        reasoning: %t\n        input: [text]\n", modelName, modelName, isQwen38)
	b.WriteString("        cost: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0}\n")
	fmt.Fprintf(&b, "        contextWindow: %d\n        maxTokens: %d\n", ctxSize, piMaxTokens(ctxSize))
	switch {
	case isQwen38:
		b.WriteString("        thinking:\n          mode: effort\n          efforts: [low, medium, xhigh]\n")
		b.WriteString("        compat:\n          supportsDeveloperRole: false\n          thinkingFormat: qwen-chat-template\n")
		b.WriteString("          qwenTemplateReasoningEffort: true\n          reasoningContentField: reasoning_content\n")
	case strings.Contains(lower, "gemma"):
		// Same tool-result flags as pi needs for Gemma's chat template.
		b.WriteString("        compat:\n          supportsDeveloperRole: false\n          requiresToolResultName: true\n          requiresAssistantAfterToolResult: true\n")
	default:
		b.WriteString("        compat:\n          supportsDeveloperRole: false\n")
	}
	return b.String()
}

// ompOverlayYAML turns off what cannot work from the drive: services that need
// the network, and background model calls that would queue behind the user's
// turn on the single llama-server slot. Compaction mirrors ensurePiSettings.
func ompOverlayYAML(modelName string, ctxSize int) string {
	return fmt.Sprintf(`# Written by Svalbard at every launch and passed with --config.
# Change your own settings in config.yml; this file is overwritten.
modelRoles:
  default: %q
startup:
  setupWizard: false
  checkUpdate: false
  changelogMode: hidden
update:
  auto: false
marketplace:
  autoUpdate: "off"
dev:
  autoqa: false
telemetry:
  otlpExportEnabled: false
web_search:
  enabled: false
fetch:
  enabled: false
browser:
  enabled: false
exa:
  enabled: false
recap:
  enabled: false
composer:
  predictions: false
providers:
  cacheWarming: "off"
memory:
  backend: "off"
compaction:
  reserveTokens: %d
  keepRecentTokens: %d
`, piProvider+"/"+modelName, piMaxTokens(ctxSize), min(ctxSize/4, 20000))
}

func piMaxTokens(ctxSize int) int {
	if ctxSize/2 < 8192 {
		return ctxSize / 2
	}
	return 8192
}

func ensurePiSettings(path, modelName string, ctxSize int) error {
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse Pi settings: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	settingsMissing := os.IsNotExist(err)

	defaults := map[string]int{"reserveTokens": piMaxTokens(ctxSize), "keepRecentTokens": min(ctxSize/4, 20000)}
	sidecarPath := filepath.Join(filepath.Dir(path), ".svalbard-compaction.json")
	owned, hasOwned, err := readPiCompactionOwnership(sidecarPath)
	if err != nil {
		return err
	}

	changed := settingsMissing
	writeSidecar := false
	compaction, ok := settings["compaction"].(map[string]any)
	if !ok {
		if _, exists := settings["compaction"]; !exists {
			compaction = map[string]any{}
			settings["compaction"] = compaction
			changed = true
		} else {
			compaction = nil
		}
	}
	if compaction != nil {
		var compactionChanged, ownershipChanged bool
		owned, compactionChanged, ownershipChanged = refreshPiCompaction(compaction, owned, hasOwned, defaults)
		changed = changed || compactionChanged
		writeSidecar = ownershipChanged
	}
	if strings.Contains(strings.ToLower(modelName), "qwen3.8") {
		modelLevels, ok := settings["modelThinkingLevels"].(map[string]any)
		if !ok {
			if _, exists := settings["modelThinkingLevels"]; !exists {
				modelLevels = map[string]any{}
				settings["modelThinkingLevels"] = modelLevels
				changed = true
			}
		}
		if modelLevels != nil {
			key := piProvider + "/" + modelName
			if _, ok := modelLevels[key]; !ok {
				modelLevels[key] = "medium"
				changed = true
			}
		}
	}
	if changed {
		if err := writeJSON(path, settings); err != nil {
			return err
		}
	}
	if writeSidecar {
		return writeJSON(sidecarPath, owned)
	}
	return nil
}

func readPiCompactionOwnership(path string) (map[string]int, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var owned map[string]int
	if err := json.Unmarshal(data, &owned); err != nil {
		return nil, false, fmt.Errorf("parse Pi compaction ownership: %w", err)
	}
	return owned, true, nil
}

func refreshPiCompaction(compaction map[string]any, owned map[string]int, hasOwned bool, defaults map[string]int) (map[string]int, bool, bool) {
	if !hasOwned {
		owned = map[string]int{}
	}
	changed := false
	ownershipChanged := false
	for key, value := range defaults {
		current, exists := compaction[key]
		lastGenerated, ownedField := owned[key]
		switch {
		case !exists:
			compaction[key] = value
			owned[key] = value
			changed = true
			ownershipChanged = true
		case ownedField && piCompactionValueMatches(current, lastGenerated):
			if !piCompactionValueMatches(current, value) {
				compaction[key] = value
				changed = true
			}
			if lastGenerated != value {
				owned[key] = value
				ownershipChanged = true
			}
		case ownedField:
			delete(owned, key)
			ownershipChanged = true
		}
	}
	return owned, changed, ownershipChanged
}

func piCompactionValueMatches(value any, want int) bool {
	switch value := value.(type) {
	case float64:
		return value == float64(want)
	case int:
		return value == want
	default:
		return false
	}
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, clientName, selectedModel string) error {
	clientBin, err := binary.Resolve(clientName, driveRoot)
	if err != nil {
		return fmt.Errorf("%s not found", clientName)
	}
	llamaBin, err := binary.Resolve("llama-server", driveRoot)
	if err != nil {
		return fmt.Errorf("llama-server not found")
	}
	model, err := llamaserve.ResolveModel(driveRoot, selectedModel)
	if err != nil {
		return err
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", 8082)
	if err != nil {
		return err
	}

	modelName := strings.TrimSuffix(filepath.Base(model), filepath.Ext(model))
	hostRoot := fmt.Sprintf("http://127.0.0.1:%d", port)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", port)
	logFile, logPath, err := openRuntimeLogFile(driveRoot, clientName, "llama-server.log")
	if err != nil {
		return err
	}
	defer logFile.Close()

	ctxSize := contextpicker.Pick(llamaserve.ContextForHost())
	if err := checkClientContext(clientName, ctxSize); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Starting llama-server with %s (context %d)\n", modelName, ctxSize)
	fmt.Fprintf(stdout, "llama-server log: %s\n", logPath)
	llamaArgs := []string{"-m", model, "--jinja", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port)}
	llamaArgs = append(llamaArgs, llamaserve.ExtraFlags(model, ctxSize)...)
	llamaCmd := exec.CommandContext(ctx, llamaBin, llamaArgs...)
	llamaCmd.Stdout = logFile
	llamaCmd.Stderr = logFile
	if err := llamaCmd.Start(); err != nil {
		return err
	}
	defer func() {
		if llamaCmd.Process != nil {
			_ = llamaCmd.Process.Kill()
		}
	}()
	if err := waitForHTTPReady(hostRoot + "/health"); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Launching %s against %s\n", clientName, modelName)
	launchCfg, err := PrepareClientLaunchConfig(driveRoot, clientName, hostRoot, baseURL, modelName, ctxSize)
	if err != nil {
		return err
	}
	clientCmd := exec.CommandContext(ctx, clientBin, launchCfg.Args...)
	clientCmd.Stdin = os.Stdin
	clientCmd.Stdout = os.Stdout
	clientCmd.Stderr = os.Stderr
	clientCmd.Env = append(os.Environ(), platform.EnvList(ClientEnvironment(baseURL, modelName))...)
	clientCmd.Env = append(clientCmd.Env, platform.EnvList(launchCfg.Env)...)
	clientCmd.Dir = driveRoot
	if workDir, err := os.Getwd(); err == nil && workDir != "" {
		clientCmd.Dir = workDir
	}
	return clientCmd.Run()
}

func ensureGooseStdioExtension(configPath, mcpBinary, driveRoot string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	content, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	updated := upsertGooseExtensionYAML(string(content), gooseExtensionBlock(mcpBinary, driveRoot))
	return os.WriteFile(configPath, []byte(updated), 0o644)
}

func gooseExtensionBlock(mcpBinary, driveRoot string) string {
	return fmt.Sprintf(`  svalbard:
    enabled: true
    type: stdio
    name: svalbard
    cmd: %q
    args:
      - "mcp"
      - "--drive"
      - %q
    timeout: 300
`, mcpBinary, driveRoot)
}

func upsertGooseExtensionYAML(existing, block string) string {
	block = strings.TrimRight(block, "\n")
	if strings.TrimSpace(existing) == "" {
		return "extensions:\n" + block + "\n"
	}

	lines := strings.Split(existing, "\n")
	extensionsIdx := -1
	for i, line := range lines {
		if line == "extensions:" {
			extensionsIdx = i
			break
		}
	}
	if extensionsIdx == -1 {
		existing = strings.TrimRight(existing, "\n")
		return existing + "\nextensions:\n" + block + "\n"
	}

	sectionEnd := len(lines)
	for i := extensionsIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			sectionEnd = i
			break
		}
	}

	blockLines := strings.Split(block, "\n")
	svalbardIdx := -1
	for i := extensionsIdx + 1; i < sectionEnd; i++ {
		if lines[i] == "  svalbard:" {
			svalbardIdx = i
			break
		}
	}
	if svalbardIdx == -1 {
		merged := append([]string{}, lines[:sectionEnd]...)
		merged = append(merged, blockLines...)
		merged = append(merged, lines[sectionEnd:]...)
		return strings.TrimRight(strings.Join(merged, "\n"), "\n") + "\n"
	}

	svalbardEnd := sectionEnd
	for i := svalbardIdx + 1; i < sectionEnd; i++ {
		if isGooseExtensionKey(lines[i]) {
			svalbardEnd = i
			break
		}
	}

	merged := append([]string{}, lines[:svalbardIdx]...)
	merged = append(merged, blockLines...)
	merged = append(merged, lines[svalbardEnd:]...)
	return strings.TrimRight(strings.Join(merged, "\n"), "\n") + "\n"
}

func isGooseExtensionKey(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(line, "  ") &&
		!strings.HasPrefix(line, "    ") &&
		trimmed != "" &&
		!strings.HasPrefix(trimmed, "#")
}

func waitForHTTPReady(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("llama-server did not become healthy in time")
}

func openRuntimeLogFile(driveRoot, clientName, filename string) (*os.File, string, error) {
	runtimeRoot := filepath.Join(driveRoot, ".svalbard", "runtime", clientName)
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(runtimeRoot, filename)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, "", err
	}
	return file, path, nil
}
