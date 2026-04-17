package toolkit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

const svalbardDir = ".svalbard"
const actionsFile = "actions.json"
const runtimeBinaryName = "svalbard-drive"

var supportedPlatforms = []string{
	"macos-arm64",
	"macos-x86_64",
	"linux-arm64",
	"linux-x86_64",
}

var runtimeBuildTargets = map[string]struct {
	goos   string
	goarch string
}{
	"macos-arm64":  {goos: "darwin", goarch: "arm64"},
	"macos-x86_64": {goos: "darwin", goarch: "amd64"},
	"linux-arm64":  {goos: "linux", goarch: "arm64"},
	"linux-x86_64": {goos: "linux", goarch: "amd64"},
}

var runtimeBinarySources = loadRuntimeBinaries

func loadRuntimeBinaries() (map[string]string, error) {
	// Try embedded binaries first (release mode).
	if bins, err := extractEmbeddedBinaries(); err == nil {
		return bins, nil
	}
	// Fall back to compiling from source (dev mode).
	return buildDriveRuntimeBinaries()
}

// TypeDirs maps content types to their destination subdirectories.
var TypeDirs = map[string]string{
	"zim":            "zim",
	"pmtiles":        "maps",
	"pdf":            "books",
	"epub":           "books",
	"gguf":           "models",
	"gguf-embed":     "models/embed",
	"binary":         "bin",
	"app":            "apps",
	"dataset":        "data",
	"python-venv":    "runtime/python",
	"python-package": "runtime/python",
}

// runtimeConfig mirrors drive-runtime/internal/config.RuntimeConfig.
// These types are duplicated because host-cli and drive-runtime are separate
// Go modules. Keep in sync with drive-runtime/internal/config/config.go.
type runtimeConfig struct {
	Version int         `json:"version"`
	Preset  string      `json:"preset"`
	Groups  []menuGroup `json:"groups"`
}

type menuGroup struct {
	ID           string     `json:"id"`
	Label        string     `json:"label"`
	Description  string     `json:"description"`
	Order        int        `json:"order"`
	AutoActivate bool       `json:"auto_activate,omitempty"`
	Items        []menuItem `json:"items"`
}

type menuItem struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Subheader   string     `json:"subheader,omitempty"`
	Order       int        `json:"order"`
	Action      actionSpec `json:"action"`
}

type actionSpec struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

type builtinConfig struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args,omitempty"`
}

// builtinAction constructs an actionSpec for a builtin action.
func builtinAction(name string, args map[string]string) actionSpec {
	cfg := builtinConfig{Name: name, Args: args}
	data, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return actionSpec{
		Type:   "builtin",
		Config: data,
	}
}

// humanize converts an ID like "wikipedia-en-nopic" to "Wikipedia En Nopic".
func humanize(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		if len(w) > 0 {
			runes := []rune(w)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

// GenerateOpts holds optional parameters for toolkit generation.
type GenerateOpts struct {
	// EnvVars maps environment variable names to drive-relative paths.
	// Collected from recipe env fields and written into the activate script.
	EnvVars map[string]string

	// Menus maps entry IDs to recipe menu specs from the catalog.
	Menus map[string]catalog.MenuSpec
}

// Generate creates the .svalbard/actions.json runtime config under root.
// entries is the list of realized manifest entries; presetName is recorded in
// the config for the drive-runtime to identify which preset was applied.
func Generate(root string, entries []manifest.RealizedEntry, presetName string, opts ...GenerateOpts) error {
	if err := os.MkdirAll(filepath.Join(root, svalbardDir), 0o755); err != nil {
		return err
	}
	var menus map[string]catalog.MenuSpec
	if len(opts) > 0 && opts[0].Menus != nil {
		menus = opts[0].Menus
	}
	if err := writeActionsConfig(root, entries, presetName, menus); err != nil {
		return err
	}
	if err := installRuntimeBinaries(root); err != nil {
		return err
	}
	var envVars map[string]string
	if len(opts) > 0 && opts[0].EnvVars != nil {
		envVars = opts[0].EnvVars
	}
	if err := writeRootScripts(root, envVars); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Menu spec: group registry, type defaults, built-in capabilities
// ---------------------------------------------------------------------------

type groupInfo struct {
	Label        string
	Description  string
	Order        int
	AutoActivate bool
}

var groupRegistry = map[string]groupInfo{
	"search":   {Label: "Search", Description: "Search across indexed archives and documents.", Order: 100, AutoActivate: true},
	"library":  {Label: "Library", Description: "Browse offline archives and documents.", Order: 200},
	"maps":     {Label: "Maps", Description: "View offline map layers.", Order: 300},
	"local-ai": {Label: "Local AI", Description: "Chat with local models and launch AI clients.", Order: 400},
	"apps":     {Label: "Apps", Description: "Interactive offline applications.", Order: 450},
	"tools":    {Label: "Tools", Description: "Inspect the drive and launch bundled utilities.", Order: 500},
}

type typeDefault struct {
	Group     string
	Subheader string
	ActionID  string
	// ArgsFunc builds action args from an entry. Nil means no args.
	ArgsFunc func(e manifest.RealizedEntry) map[string]string
}

var typeDefaults = map[string]typeDefault{
	"zim":     {Group: "library", Subheader: "Archives", ActionID: "browse", ArgsFunc: func(e manifest.RealizedEntry) map[string]string { return map[string]string{"zim": e.Filename} }},
	"pmtiles": {Group: "maps", Subheader: "Maps", ActionID: "maps"},
	"gguf":    {Group: "local-ai", Subheader: "Chat Models", ActionID: "chat", ArgsFunc: func(e manifest.RealizedEntry) map[string]string { return map[string]string{"model": e.Filename} }},
	"app":     {Group: "apps", Subheader: "Apps", ActionID: "apps", ArgsFunc: func(e manifest.RealizedEntry) map[string]string { return map[string]string{"app": e.ID} }},
	"sqlite":  {Group: "tools", Subheader: "Data", ActionID: "apps", ArgsFunc: func(e manifest.RealizedEntry) map[string]string { return map[string]string{"app": "sqliteviz"} }},
	"binary":  {Group: "tools", Subheader: "Tools"},
}

// driveState captures what content is on the drive for capability conditions.
type driveState struct {
	HasLibrary   bool
	HasMaps      bool
	HasChat      bool
	HasToolchain bool
	HasChecksums bool
}

type builtinCapability struct {
	ID          string
	Group       string
	Subheader   string
	Label       string
	Description string
	Order       int
	ActionID    string
	Args        map[string]string
	Condition   func(driveState) bool
}

var builtinCapabilities = []builtinCapability{
	// Search
	{ID: "search-all", Group: "search", Label: "Search all content",
		Description: "Query the on-drive search index.", Order: 100, ActionID: "search",
		Condition: func(s driveState) bool { return s.HasLibrary }},
	// Sharing
	{ID: "serve-all", Group: "tools", Subheader: "Sharing", Label: "Serve Everything",
		Description: "Start all services at once.", Order: 300, ActionID: "serve-all",
		Condition: func(s driveState) bool { return s.HasLibrary || s.HasMaps || s.HasChat }},
	{ID: "share-files", Group: "tools", Subheader: "Sharing", Label: "Share on Local Network",
		Description: "Share drive files on the local network.", Order: 310, ActionID: "share",
		Condition: func(s driveState) bool { return s.HasLibrary || s.HasMaps || s.HasChat }},
	{ID: "mcp-serve", Group: "tools", Subheader: "Sharing", Label: "Serve MCP",
		Description: "Start the MCP server so AI tools can access the vault.", Order: 320, ActionID: "mcp-serve"},
	// Development
	{ID: "activate-shell", Group: "tools", Subheader: "Development", Label: "Activate sb Shell",
		Description: "Open a temporary shell with drive commands available.", Order: 690, ActionID: "activate-shell"},
	{ID: "embedded-shell", Group: "tools", Subheader: "Development", Label: "Embedded Dev Shell",
		Description: "Drop into the bundled embedded development shell.", Order: 700, ActionID: "embedded-shell",
		Condition: func(s driveState) bool { return s.HasToolchain }},
	// Drive
	{ID: "inspect-drive", Group: "tools", Subheader: "Drive", Label: "Inspect Drive",
		Description: "Show a terminal summary of the drive contents.", Order: 500, ActionID: "inspect"},
	{ID: "verify-checksums", Group: "tools", Subheader: "Drive", Label: "Verify Checksums",
		Description: "Verify file integrity using checksums.", Order: 510, ActionID: "verify",
		Condition: func(s driveState) bool { return s.HasChecksums }},
}

// ---------------------------------------------------------------------------
// writeActionsConfig — spec-driven menu generation
// ---------------------------------------------------------------------------

func writeActionsConfig(root string, entries []manifest.RealizedEntry, presetName string, menus map[string]catalog.MenuSpec) error {
	// Collect items per group.
	grouped := make(map[string][]menuItem)
	var state driveState
	hasChatModels := false

	for i, e := range entries {
		// Track drive state before any continue — all entries contribute.
		if e.ChecksumSHA256 != "" {
			state.HasChecksums = true
		}
		switch e.Type {
		case "zim":
			state.HasLibrary = true
		case "pmtiles":
			state.HasMaps = true
		case "gguf":
			state.HasChat = true
			hasChatModels = true
		case "toolchain":
			state.HasToolchain = true
		}

		td, ok := typeDefaults[e.Type]
		if !ok {
			continue
		}

		// Resolve menu metadata: recipe spec → type defaults → humanize.
		group := td.Group
		subheader := td.Subheader
		label := humanize(e.ID)
		desc := ""
		order := (i + 1) * 100
		actionID := td.ActionID

		if m, ok := menus[e.ID]; ok {
			if m.Group != "" {
				group = m.Group
			}
			if m.Subheader != "" {
				subheader = m.Subheader
			}
			if m.Label != "" {
				label = m.Label
			}
			if m.Description != "" {
				desc = m.Description
			}
			if m.Order != 0 {
				order = m.Order
			}
		}

		// For binary entries in local-ai group, use agent action.
		if e.Type == "binary" && group == "local-ai" {
			actionID = "agent"
		}

		// Build default description if not provided by recipe.
		if desc == "" {
			switch actionID {
			case "browse":
				desc = "Browse the " + label + " archive."
			case "chat":
				desc = "Chat with " + label + " locally."
			case "apps":
				desc = "Open " + label + "."
			case "agent":
				desc = "Launch " + label + " against local models."
			default:
				desc = label + "."
			}
		}

		// Build action args.
		var args map[string]string
		if actionID == "agent" {
			args = map[string]string{"client": e.ID}
		} else if td.ArgsFunc != nil {
			args = td.ArgsFunc(e)
		}

		grouped[group] = append(grouped[group], menuItem{
			ID:          e.ID,
			Label:       label,
			Description: desc,
			Subheader:   subheader,
			Order:       order,
			Action:      builtinAction(actionID, args),
		})
	}

	// Add default "Chat" item (no model arg, auto-selects) when chat models exist.
	if hasChatModels {
		grouped["local-ai"] = append([]menuItem{{
			ID:          "chat-default",
			Label:       "Chat",
			Description: "Start chat with the default local model.",
			Subheader:   "Chat Models",
			Order:       50,
			Action:      builtinAction("chat", nil),
		}}, grouped["local-ai"]...)
	}

	// Add built-in capabilities.
	for _, cap := range builtinCapabilities {
		if cap.Condition != nil && !cap.Condition(state) {
			continue
		}
		grouped[cap.Group] = append(grouped[cap.Group], menuItem{
			ID:          cap.ID,
			Label:       cap.Label,
			Description: cap.Description,
			Subheader:   cap.Subheader,
			Order:       cap.Order,
			Action:      builtinAction(cap.ActionID, cap.Args),
		})
	}

	// Assemble groups, sort items within each, then sort groups.
	var groups []menuGroup
	for id, items := range grouped {
		sort.Slice(items, func(i, j int) bool { return items[i].Order < items[j].Order })
		info, ok := groupRegistry[id]
		if !ok {
			info = groupInfo{Label: humanize(id), Description: "", Order: 999}
		}
		groups = append(groups, menuGroup{
			ID:           id,
			Label:        info.Label,
			Description:  info.Description,
			Order:        info.Order,
			AutoActivate: info.AutoActivate,
			Items:        items,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Order < groups[j].Order })

	cfg := runtimeConfig{
		Version: 2,
		Preset:  presetName,
		Groups:  groups,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(root, svalbardDir, actionsFile), data, 0o644)
}

func installRuntimeBinaries(root string) error {
	sources, err := runtimeBinarySources()
	if err != nil {
		return err
	}
	for _, platform := range supportedPlatforms {
		source, ok := sources[platform]
		if !ok {
			return fmt.Errorf("missing runtime binary for %s", platform)
		}
		dest := filepath.Join(root, svalbardDir, "runtime", platform, runtimeBinaryName)
		if err := copyFile(source, dest, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func writeRootScripts(root string, envVars map[string]string) error {
	if err := os.WriteFile(filepath.Join(root, "run"), []byte(runScriptContents()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "activate"), []byte(activateScriptContents(envVars)), 0o755); err != nil {
		return err
	}
	return nil
}

func copyFile(sourcePath, destPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return err
	}
	return dest.Chmod(mode)
}

func runScriptContents() string {
	return `#!/usr/bin/env bash
set -euo pipefail

DRIVE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$(uname -s)" in
    Darwin*) os="macos" ;;
    Linux*)  os="linux" ;;
    *)       echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64)        arch="x86_64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)             echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

platform="${os}-${arch}"
export DRIVE_ROOT
exec "$DRIVE_ROOT/.svalbard/runtime/$platform/svalbard-drive" "$@"
`
}

// isValidEnvName checks that a name contains only [A-Z0-9_] characters.
func isValidEnvName(name string) bool {
	for _, r := range name {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return len(name) > 0
}

// isValidEnvValue checks that a value contains no shell metacharacters.
func isValidEnvValue(value string) bool {
	for _, r := range value {
		if r == '$' || r == '`' || r == '"' || r == '\'' || r == '\\' || r == ';' || r == '|' || r == '&' || r == '\n' {
			return false
		}
	}
	return true
}

func activateScriptContents(envVars map[string]string) string {
	var envLines string
	if len(envVars) > 0 {
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(envVars))
		for k := range envVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !isValidEnvName(k) || !isValidEnvValue(envVars[k]) {
				continue // skip unsafe env vars
			}
			envLines += fmt.Sprintf("export %s=\"$DRIVE_ROOT/%s\"\n", k, envVars[k])
		}
	}

	return `#!/usr/bin/env bash
set -euo pipefail

DRIVE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    exec "$DRIVE_ROOT/run" activate "$@"
fi

# Detect platform
case "$(uname -s)-$(uname -m)" in
    Darwin-arm64)  _SB_PLATFORM=macos-arm64 ;;
    Darwin-x86_64) _SB_PLATFORM=macos-x86_64 ;;
    Linux-aarch64) _SB_PLATFORM=linux-arm64 ;;
    Linux-x86_64)  _SB_PLATFORM=linux-x86_64 ;;
    *) echo "Unsupported platform: $(uname -s)-$(uname -m)" >&2; return 1 ;;
esac

# Add drive binaries to PATH (native + Python wrappers)
export PATH="$DRIVE_ROOT/bin/$_SB_PLATFORM:$PATH"

# Data directory
export SVALBARD_DATA="$DRIVE_ROOT/data"

# Recipe env vars
` + envLines + `
sb() {
    "$DRIVE_ROOT/run" "$@"
}

deactivate() {
    export PATH="${PATH//$DRIVE_ROOT\/bin\/$_SB_PLATFORM:/}"
    unset DRIVE_ROOT SVALBARD_DATA _SB_PLATFORM
    unset -f sb deactivate
}

export DRIVE_ROOT
echo "Activated svalbard shell for $DRIVE_ROOT ($_SB_PLATFORM)"
echo "Use 'sb' to run drive commands. Run 'deactivate' to leave."
`
}

func buildDriveRuntimeBinaries() (map[string]string, error) {
	driveRuntimeDir, err := resolveDriveRuntimeDir()
	if err != nil {
		return nil, err
	}

	outputRoot, err := os.MkdirTemp("", "svalbard-drive-runtime-")
	if err != nil {
		return nil, err
	}

	binaries := make(map[string]string, len(supportedPlatforms))
	for _, platform := range supportedPlatforms {
		target := runtimeBuildTargets[platform]
		outputPath := filepath.Join(outputRoot, platform, runtimeBinaryName)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return nil, err
		}

		cmd := exec.Command("go", "build", "-trimpath", "-o", outputPath, "./cmd/svalbard-drive")
		cmd.Dir = driveRuntimeDir
		cmd.Env = append(os.Environ(),
			"GOOS="+target.goos,
			"GOARCH="+target.goarch,
			"CGO_ENABLED=0",
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("build runtime binary for %s: %w: %s", platform, err, strings.TrimSpace(string(output)))
		}
		binaries[platform] = outputPath
	}

	return binaries, nil
}

func resolveDriveRuntimeDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve drive-runtime dir: runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "drive-runtime"), nil
}
