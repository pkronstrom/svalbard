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

var runtimeBinarySources = buildDriveRuntimeBinaries

// TypeDirs maps content types to their destination subdirectories.
var TypeDirs = map[string]string{
	"zim":     "zim",
	"pmtiles": "maps",
	"pdf":     "books",
	"epub":    "books",
	"gguf":    "models",
	"binary":  "bin",
	"app":     "apps",
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

// Generate creates the .svalbard/actions.json runtime config under root.
// entries is the list of realized manifest entries; presetName is recorded in
// the config for the drive-runtime to identify which preset was applied.
func Generate(root string, entries []manifest.RealizedEntry, presetName string) error {
	if err := os.MkdirAll(filepath.Join(root, svalbardDir), 0o755); err != nil {
		return err
	}
	if err := writeActionsConfig(root, entries, presetName); err != nil {
		return err
	}
	if err := installRuntimeBinaries(root); err != nil {
		return err
	}
	if err := writeRootScripts(root); err != nil {
		return err
	}
	return nil
}

func writeActionsConfig(root string, entries []manifest.RealizedEntry, presetName string) error {
	var libraryItems []menuItem
	var mapsItems []menuItem

	for i, e := range entries {
		order := (i + 1) * 100
		switch e.Type {
		case "zim":
			libraryItems = append(libraryItems, menuItem{
				ID:          "browse-" + e.ID,
				Label:       humanize(e.ID),
				Description: "Browse the " + humanize(e.ID) + " archive.",
				Order:       order,
				Action:      builtinAction("browse", map[string]string{"zim": e.Filename}),
			})
		case "pmtiles":
			mapsItems = append(mapsItems, menuItem{
				ID:          "map-" + e.ID,
				Label:       humanize(e.ID),
				Description: "View the " + humanize(e.ID) + " map.",
				Order:       order,
				Action:      builtinAction("maps", nil),
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

func writeRootScripts(root string) error {
	if err := os.WriteFile(filepath.Join(root, "run"), []byte(runScriptContents()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "activate"), []byte(activateScriptContents()), 0o755); err != nil {
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

func activateScriptContents() string {
	return `#!/usr/bin/env bash
set -euo pipefail

DRIVE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    exec "$DRIVE_ROOT/run" activate "$@"
fi

sb() {
    "$DRIVE_ROOT/run" "$@"
}

deactivate() {
    unset -f sb deactivate
}

export DRIVE_ROOT
echo "Activated sb shell for $DRIVE_ROOT"
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
