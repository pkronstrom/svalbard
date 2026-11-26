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
	"sync"
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

// SupportedPlatforms lists the platforms a vault can target, in menu order.
func SupportedPlatforms() []string {
	return append([]string(nil), supportedPlatforms...)
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
	"gguf-mtp":       "models/draft",
	"binary":         "bin",
	"app":            "apps",
	"dataset":        "data",
	"sqlite":         "data",
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
	Note        string     `json:"note,omitempty"`
	Aliases     []string   `json:"aliases,omitempty"`
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

	// PlatformIORecipes is the requested PlatformIO archive closure. It is
	// supplied by apply, which has the catalog and desired selection together.
	PlatformIORecipes map[string]catalog.Item

	// PlatformIOPlatforms are normalized requested host targets. Only these
	// targets are required in the inventory closure.
	PlatformIOPlatforms []string

	// Recipes holds catalog metadata for realized entries, written to
	// .svalbard/recipes.json for the MCP server; the settings they declare go
	// to .svalbard/settings.schema.json.
	Recipes map[string]catalog.Item
}

// Generate creates the .svalbard/actions.json runtime config under root,
// along with the recipe metadata, PlatformIO inventory and root scripts.
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
	var recipes map[string]catalog.Item
	var platformIORecipes map[string]catalog.Item
	var platformIOPlatforms []string
	if len(opts) > 0 {
		recipes = opts[0].Recipes
		platformIORecipes = opts[0].PlatformIORecipes
		platformIOPlatforms = opts[0].PlatformIOPlatforms
	}
	if err := writeActionsConfig(root, entries, presetName, menus, recipes); err != nil {
		return err
	}
	if err := writePlatformIOInventory(root, entries, platformIORecipes, platformIOPlatforms); err != nil {
		return err
	}
	if err := writePlatformIOExample(root, entries); err != nil {
		return err
	}
	if err := writeAppPages(root, entries); err != nil {
		return err
	}
	if err := writeRecipesMetadata(root, entries, recipes); err != nil {
		return err
	}
	if err := writeSettingsSchema(root, entries, recipes); err != nil {
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
	if err := writeChecksums(root, entries); err != nil {
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
	// Last: a group the registry does not know gets order 999.
	"settings": {Label: "Settings", Description: "Choices saved on this drive.", Order: 1000, AutoActivate: true},
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
	HasOpenWebUI bool
	HasWiki      bool
}

// wikiRecipeID is the recipe that installs the wiki's server. Keep in sync
// with drive-runtime/internal/wiki.
const wikiRecipeID = "silverbullet"

type builtinCapability struct {
	ID          string
	Group       string
	Subheader   string
	Label       string
	Note        string
	Description string
	Order       int
	ActionID    string
	Args        map[string]string
	Aliases     []string
	Condition   func(driveState) bool
}

var builtinCapabilities = []builtinCapability{
	// Search
	{ID: "search-all", Group: "search", Label: "Search all content",
		Description: "Query the on-drive search index.", Order: 100, ActionID: "search",
		Aliases:   []string{"search"},
		Condition: func(s driveState) bool { return s.HasLibrary }},
	// Local AI. The label is fixed here, not taken from the recipe: the Open
	// WebUI License forbids renaming it.
	{ID: "open-webui", Group: "local-ai", Subheader: "AI Clients", Label: "Open WebUI",
		Description: "Chat with the local model and the vault's tools in the browser.", Order: 260, ActionID: "open-webui",
		Aliases:   []string{"open-webui"},
		Condition: func(s driveState) bool { return s.HasOpenWebUI }},
	// The wiki is named for what it is to the user, not for the server
	// behind it, and its settings appear under the same name.
	{ID: wikiRecipeID, Group: "apps", Subheader: "Apps", Label: "Wiki",
		Description: "Your own notes on this drive, in the browser. The AI clients can read and write them.", Order: 50, ActionID: "wiki",
		Aliases:   []string{"wiki"},
		Condition: func(s driveState) bool { return s.HasWiki }},
	// The folder Open WebUI's file tools save to, for looking at what is in
	// it without starting the model.
	{ID: "workspace", Group: "apps", Subheader: "Apps", Label: "Workspace", Note: "pages the assistant saved",
		Description: "Open the pages and files the assistant saved from Open WebUI, in the browser.", Order: 60, ActionID: "workspace",
		Aliases:   []string{"workspace"},
		Condition: func(s driveState) bool { return s.HasOpenWebUI }},
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
		Aliases:   []string{"embedded"},
		Condition: func(s driveState) bool { return s.HasToolchain }},
	// Drive
	{ID: "inspect-drive", Group: "tools", Subheader: "Drive", Label: "Inspect Drive",
		Description: "Show a terminal summary of the drive contents.", Order: 500, ActionID: "inspect"},
	{ID: "verify-checksums", Group: "tools", Subheader: "Drive", Label: "Verify Checksums",
		Description: "Verify file integrity using checksums.", Order: 510, ActionID: "verify",
		Condition: func(s driveState) bool { return s.HasChecksums }},
	// Settings. A screen of the menu itself; `./run settings` is the command.
	{ID: "settings", Group: "settings", Label: "Settings",
		Description: "Change the choices saved on this drive.", Order: 100, ActionID: "settings"},
}

// ---------------------------------------------------------------------------
// writeActionsConfig — spec-driven menu generation
// ---------------------------------------------------------------------------

// writeChecksums writes .svalbard/checksums.sha256 from manifest entries
// that have a SHA256 checksum recorded. Each line has the format:
//
//	<hex-sha256>  <relative-path>
func writeChecksums(root string, entries []manifest.RealizedEntry) error {
	var lines []string
	for _, e := range entries {
		if e.ChecksumSHA256 == "" || e.RelativePath == "" {
			continue
		}
		lines = append(lines, e.ChecksumSHA256+"  "+filepath.ToSlash(e.RelativePath))
	}
	checksumPath := filepath.Join(root, svalbardDir, "checksums.sha256")
	if len(lines) == 0 {
		// Remove stale file if no entries have checksums.
		os.Remove(checksumPath)
		return nil
	}
	sort.Strings(lines)
	return os.WriteFile(checksumPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func writeActionsConfig(root string, entries []manifest.RealizedEntry, presetName string, menus map[string]catalog.MenuSpec, recipes map[string]catalog.Item) error {
	// Collect items per group.
	grouped := make(map[string][]menuItem)
	var state driveState
	hasChatModels := false
	names := newRunNames()
	installed := make(map[string]bool, len(entries))
	for _, e := range entries {
		installed[e.ID] = true
	}
	// A recipe with a file per platform gets one row, not one per file.
	listed := make(map[string]bool)

	for i, e := range entries {
		// Track drive state before any continue — all entries contribute.
		if e.ChecksumSHA256 != "" {
			state.HasChecksums = true
		}
		switch e.Type {
		case "zim", "pdf":
			state.HasLibrary = true
		case "pmtiles":
			state.HasMaps = true
		case "gguf":
			state.HasChat = true
			hasChatModels = true
		case "toolchain":
			state.HasToolchain = true
		case "python-package":
			if e.ID == "open-webui" {
				state.HasOpenWebUI = true
			}
		case "binary":
			if e.ID == wikiRecipeID {
				// One fixed entry, added below, instead of one generated
				// entry per platform.
				state.HasWiki = true
				continue
			}
		}

		td, ok := typeDefaults[e.Type]
		if !ok {
			continue
		}
		// A database opens in SQLiteViz, which is a recipe of its own.
		if e.Type == "sqlite" && !installed["sqliteviz"] {
			continue
		}

		// Resolve menu metadata: recipe spec → type defaults → humanize.
		group := td.Group
		subheader := td.Subheader
		label := humanize(e.ID)
		desc := ""
		var aliases []string
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
			aliases = append([]string(nil), m.Aliases...)
		}

		// For binary entries in local-ai group, use agent action.
		if e.Type == "binary" && group == "local-ai" {
			actionID = "agent"
		}
		// Nothing to run means no row: a command-line tool or a server that
		// another entry starts.
		if actionID == "" {
			continue
		}
		if e.Type == "binary" {
			if listed[e.ID] {
				continue
			}
			listed[e.ID] = true
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
			Aliases:     names.claim(aliases...),
			Order:       order,
			Action:      builtinAction(actionID, args),
		})
	}
	if documents := documentItems(entries, recipes, names); len(documents) > 0 {
		grouped["library"] = append(grouped["library"], documents...)
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
			Note:        cap.Note,
			Order:       cap.Order,
			Aliases:     cap.Aliases,
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

// runtimeScratch holds the temporary directories the runtime binaries were
// extracted or built into. Each is tens of megabytes, so it is removed as soon
// as its binaries are on the drive instead of being left for the system.
var runtimeScratch sync.Map

func newRuntimeScratch(pattern string) (string, error) {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", err
	}
	runtimeScratch.Store(dir, struct{}{})
	return dir, nil
}

func discardRuntimeScratch(dir string) {
	if _, ours := runtimeScratch.LoadAndDelete(dir); ours {
		_ = os.RemoveAll(dir)
	}
}

func installRuntimeBinaries(root string) error {
	sources, err := runtimeBinarySources()
	if err != nil {
		return err
	}
	// Sources lie at <scratch>/<platform>/<binary>; a directory this package
	// did not create (a test's stub) is left alone.
	defer func() {
		for _, source := range sources {
			discardRuntimeScratch(filepath.Dir(filepath.Dir(source)))
		}
	}()
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

	outputRoot, err := newRuntimeScratch("svalbard-drive-runtime-")
	if err != nil {
		return nil, err
	}
	built := false
	defer func() {
		if !built {
			discardRuntimeScratch(outputRoot)
		}
	}()

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

	built = true
	return binaries, nil
}

func resolveDriveRuntimeDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve drive-runtime dir: runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "drive-runtime"), nil
}
