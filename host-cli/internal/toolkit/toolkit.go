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
	"binary":         "bin",
	"app":            "apps",
	"dataset":        "data",
	"python-venv":    "runtime/python",
	"python-package": "runtime/python",
}

// runtimeConfig matches the drive-runtime RuntimeConfig JSON structure.
type runtimeConfig struct {
	Version int         `json:"version"`
	Preset  string      `json:"preset"`
	Groups  []menuGroup `json:"groups"`
}

type menuGroup struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Order       int        `json:"order"`
	Items       []menuItem `json:"items"`
}

type menuItem struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
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

	// Descriptions maps entry IDs to human-readable descriptions from the
	// catalog. Used for menu item labels when available.
	Descriptions map[string]string
}

// Generate creates the .svalbard/actions.json runtime config under root.
// entries is the list of realized manifest entries; presetName is recorded in
// the config for the drive-runtime to identify which preset was applied.
func Generate(root string, entries []manifest.RealizedEntry, presetName string, opts ...GenerateOpts) error {
	if err := os.MkdirAll(filepath.Join(root, svalbardDir), 0o755); err != nil {
		return err
	}
	var descriptions map[string]string
	if len(opts) > 0 && opts[0].Descriptions != nil {
		descriptions = opts[0].Descriptions
	}
	if err := writeActionsConfig(root, entries, presetName, descriptions); err != nil {
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

func isEmbeddingModel(filename string) bool {
	base := strings.ToLower(filename)
	return strings.Contains(base, "embed") || strings.Contains(base, "bge-") ||
		strings.Contains(base, "e5-") || strings.Contains(base, "arctic-embed")
}

func writeActionsConfig(root string, entries []manifest.RealizedEntry, presetName string, descriptions map[string]string) error {
	descFor := func(id, fallback string) string {
		if d, ok := descriptions[id]; ok && d != "" {
			return d
		}
		return fallback
	}

	var libraryItems []menuItem
	var mapsItems []menuItem
	var chatItems []menuItem
	var appsItems []menuItem
	var dataItems []menuItem

	for i, e := range entries {
		order := (i + 1) * 100
		label := descFor(e.ID, humanize(e.ID))
		switch e.Type {
		case "zim":
			libraryItems = append(libraryItems, menuItem{
				ID:          "browse-" + e.ID,
				Label:       label,
				Description: "Browse the " + label + " archive.",
				Order:       order,
				Action:      builtinAction("browse", map[string]string{"zim": e.Filename}),
			})
		case "pmtiles":
			mapsItems = append(mapsItems, menuItem{
				ID:          "map-" + e.ID,
				Label:       label,
				Description: "View the " + label + " map.",
				Order:       order,
				Action:      builtinAction("maps", nil),
			})
		case "gguf":
			if isEmbeddingModel(e.Filename) {
				continue
			}
			chatItems = append(chatItems, menuItem{
				ID:          "chat-" + e.ID,
				Label:       label,
				Description: "Chat with " + label + " locally.",
				Order:       order,
				Action:      builtinAction("chat", map[string]string{"model": e.Filename}),
			})
		case "app":
			appsItems = append(appsItems, menuItem{
				ID:          "app-" + e.ID,
				Label:       label,
				Description: "Open " + label + ".",
				Order:       order,
				Action:      builtinAction("apps", map[string]string{"app": e.ID}),
			})
		case "sqlite":
			dataItems = append(dataItems, menuItem{
				ID:          "data-" + e.ID,
				Label:       label,
				Description: "Query " + label + ".",
				Order:       order,
				Action:      builtinAction("apps", map[string]string{"app": "sqliteviz"}),
			})
		}
	}

	var groups []menuGroup

	if len(libraryItems) > 0 {
		groups = append(groups, menuGroup{
			ID:          "library",
			Label:       "Library",
			Description: "Browse packaged offline archives and documents.",
			Order:       200,
			Items:       libraryItems,
		})
	}

	if len(mapsItems) > 0 {
		groups = append(groups, menuGroup{
			ID:          "maps",
			Label:       "Maps",
			Description: "View offline map layers.",
			Order:       300,
			Items:       mapsItems,
		})
	}

	if len(chatItems) > 0 {
		groups = append(groups, menuGroup{
			ID:          "chat",
			Label:       "AI Chat",
			Description: "Chat with local AI models.",
			Order:       400,
			Items:       chatItems,
		})
	}

	if len(appsItems) > 0 {
		groups = append(groups, menuGroup{
			ID:          "apps",
			Label:       "Apps",
			Description: "Interactive offline applications.",
			Order:       500,
			Items:       appsItems,
		})
	}

	if len(dataItems) > 0 {
		groups = append(groups, menuGroup{
			ID:          "data",
			Label:       "Data",
			Description: "Query offline datasets.",
			Order:       600,
			Items:       dataItems,
		})
	}

	// Serve group — shown when there are any browseable/serveable services.
	hasServices := len(libraryItems) > 0 || len(mapsItems) > 0 || len(chatItems) > 0
	if hasServices {
		groups = append(groups, menuGroup{
			ID:          "share",
			Label:       "Serve",
			Description: "Serve content on the local network.",
			Order:       800,
			Items: []menuItem{
				{
					ID:          "serve-all",
					Label:       "Serve Everything",
					Description: "Start all services at once.",
					Order:       100,
					Action:      builtinAction("serve-all", nil),
				},
				{
					ID:          "share-files",
					Label:       "Share on Local Network",
					Description: "Share drive files on the local network.",
					Order:       200,
					Action:      builtinAction("share", nil),
				},
			},
		})
	}

	// Tools group is always present.
	groups = append(groups, menuGroup{
		ID:          "tools",
		Label:       "Tools",
		Description: "System utilities and diagnostics.",
		Order:       900,
		Items: []menuItem{
			{
				ID:          "inspect-drive",
				Label:       "Inspect Drive",
				Description: "Inspect the drive contents and configuration.",
				Order:       100,
				Action:      builtinAction("inspect", nil),
			},
			{
				ID:          "verify-checksums",
				Label:       "Verify Checksums",
				Description: "Verify file integrity using checksums.",
				Order:       200,
				Action:      builtinAction("verify", nil),
			},
		},
	})

	// Sort groups by order.
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Order < groups[j].Order
	})

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

		cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/svalbard-drive")
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
