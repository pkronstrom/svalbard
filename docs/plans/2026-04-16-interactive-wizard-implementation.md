# Interactive Init Wizard — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the skeleton wizard (labels-only) with a fully interactive init wizard that detects volumes, shows presets, provides a 3-level pack picker with tri-state checkboxes, and runs the apply pipeline — porting the Python wizard.py + picker.py UX to Go/Bubble Tea.

**Architecture:** Nested Bubble Tea sub-models (approach B). Each wizard stage is a self-contained `tea.Model`. The wizard shell orchestrates transitions via done-messages and accumulates state between stages. Data flows in from `host-cli` via a `WizardConfig` struct (no circular imports). Each sub-model owns its own keyboard handling and view rendering.

**Tech Stack:** Go 1.25, Bubble Tea v1, Lip Gloss, shared `tui/` design system. No new external dependencies.

**Module topology:** `host-cli` imports `host-tui` imports `tui/`. The wizard lives in `host-tui` but receives pre-computed data from `host-cli`. The catalog lives in `host-cli`.

**Reference implementations:**
- Python wizard: `git show main~2:src/svalbard/wizard.py`
- Python picker: `git show main~2:src/svalbard/picker.py`

---

## Task 1: Extend Catalog Preset Struct

The Go `Preset` struct is missing fields that exist in YAML: `kind`, `display_group`, `description`, `target_size_gb`, `extends`. Almost all presets use `extends:` chains but the Go parser silently ignores them. This task adds the fields and resolves inheritance.

**Files:**
- Modify: `host-cli/internal/catalog/catalog.go`
- Modify: `host-cli/internal/catalog/catalog_test.go`

**Step 1: Write failing tests for new Preset fields**

Add to `catalog_test.go`:

```go
func TestPresetHasKindAndDisplayGroup(t *testing.T) {
	// Use embedded catalog which includes packs
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	p, err := cat.ResolvePreset("core")
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "pack" {
		t.Errorf("expected Kind 'pack', got %q", p.Kind)
	}
	if p.DisplayGroup != "Core" {
		t.Errorf("expected DisplayGroup 'Core', got %q", p.DisplayGroup)
	}
	if p.Description == "" {
		t.Error("expected non-empty Description")
	}
	if p.TargetSizeGB <= 0 {
		t.Error("expected positive TargetSizeGB")
	}
}

func TestPresetExtendsResolution(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// default-64 extends default-32, which extends default-2
	p, err := cat.ResolvePreset("default-64")
	if err != nil {
		t.Fatal(err)
	}
	// Should include sources from default-2 and default-32 plus its own
	if len(p.Sources) < 20 {
		t.Errorf("expected at least 20 resolved sources (with extends), got %d", len(p.Sources))
	}
	// Should include kiwix-serve from default-2
	found := false
	for _, src := range p.Sources {
		if src == "kiwix-serve" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'kiwix-serve' inherited from default-2")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd host-cli && go test ./internal/catalog/ -run 'TestPresetHasKind|TestPresetExtends' -v`
Expected: FAIL — Kind, DisplayGroup, Description, TargetSizeGB fields don't exist

**Step 3: Add fields to Preset struct and resolve extends**

In `catalog.go`, update the `Preset` struct:

```go
type Preset struct {
	Name         string   `yaml:"name"`
	Kind         string   `yaml:"kind,omitempty"`          // "pack" or "" (regular preset)
	Region       string   `yaml:"region"`
	DisplayGroup string   `yaml:"display_group,omitempty"`
	Description  string   `yaml:"description,omitempty"`
	TargetSizeGB float64  `yaml:"target_size_gb,omitempty"`
	Extends      []string `yaml:"extends,omitempty"`       // parent preset names
	Sources      []string `yaml:"sources"`
	Items        []Item   `yaml:"-"` // populated by ResolvePreset
}
```

Add extends resolution in `ResolvePreset`:

```go
func (c *Catalog) ResolvePreset(name string) (Preset, error) {
	return c.resolvePresetRecursive(name, make(map[string]bool))
}

func (c *Catalog) resolvePresetRecursive(name string, visited map[string]bool) (Preset, error) {
	if visited[name] {
		return Preset{}, fmt.Errorf("circular extends in preset %q", name)
	}
	visited[name] = true

	preset, ok := c.presets[name]
	if !ok {
		return Preset{}, fmt.Errorf("preset %q not found", name)
	}

	// Collect sources from parent presets first (order: parents then own)
	seen := make(map[string]bool)
	var allSources []string

	for _, parent := range preset.Extends {
		resolved, err := c.resolvePresetRecursive(parent, visited)
		if err != nil {
			return Preset{}, fmt.Errorf("preset %q extends %q: %w", name, parent, err)
		}
		for _, src := range resolved.Sources {
			if !seen[src] {
				seen[src] = true
				allSources = append(allSources, src)
			}
		}
	}

	// Add own sources
	for _, src := range preset.Sources {
		if !seen[src] {
			seen[src] = true
			allSources = append(allSources, src)
		}
	}
	preset.Sources = allSources

	// Resolve items
	items := make([]Item, 0, len(preset.Sources))
	for _, src := range preset.Sources {
		item, found := c.recipes[src]
		if !found {
			continue // tolerate missing recipes in extends chains
		}
		items = append(items, item)
	}
	preset.Items = items
	return preset, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd host-cli && go test ./internal/catalog/ -v`
Expected: ALL PASS

**Step 5: Add helper methods for wizard data preparation**

Add to `catalog.go`:

```go
// Packs returns all presets with Kind == "pack", sorted by DisplayGroup then Name.
func (c *Catalog) Packs() []Preset {
	var packs []Preset
	for _, p := range c.presets {
		if p.Kind == "pack" {
			packs = append(packs, p)
		}
	}
	sort.Slice(packs, func(i, j int) bool {
		if packs[i].DisplayGroup != packs[j].DisplayGroup {
			return packs[i].DisplayGroup < packs[j].DisplayGroup
		}
		return packs[i].Name < packs[j].Name
	})
	return packs
}

// PresetsForRegion returns non-pack presets for a region, sorted by TargetSizeGB.
func (c *Catalog) PresetsForRegion(region string) []Preset {
	var result []Preset
	for _, p := range c.presets {
		if p.Kind == "pack" || p.Region != region {
			continue
		}
		if strings.HasPrefix(p.Name, "test-") {
			continue
		}
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TargetSizeGB < result[j].TargetSizeGB
	})
	return result
}

// Regions returns distinct regions from non-pack presets.
func (c *Catalog) Regions() []string {
	seen := make(map[string]bool)
	for _, p := range c.presets {
		if p.Kind != "pack" && p.Region != "" && !strings.HasPrefix(p.Name, "test-") {
			seen[p.Region] = true
		}
	}
	regions := make([]string, 0, len(seen))
	for r := range seen {
		regions = append(regions, r)
	}
	sort.Strings(regions)
	return regions
}

// ContentSizeGB returns the total size of resolved items in a preset.
func (p *Preset) ContentSizeGB() float64 {
	total := 0.0
	for _, item := range p.Items {
		total += item.SizeGB
	}
	return total
}
```

**Step 6: Add tests for new helper methods**

```go
func TestCatalogPacks(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	packs := cat.Packs()
	if len(packs) == 0 {
		t.Fatal("expected at least one pack")
	}
	for _, p := range packs {
		if p.Kind != "pack" {
			t.Errorf("Packs() returned non-pack %q with kind %q", p.Name, p.Kind)
		}
	}
}

func TestCatalogPresetsForRegion(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	presets := cat.PresetsForRegion("default")
	if len(presets) == 0 {
		t.Fatal("expected default region presets")
	}
	for i := 1; i < len(presets); i++ {
		if presets[i].TargetSizeGB < presets[i-1].TargetSizeGB {
			t.Error("presets not sorted by size")
		}
	}
}

func TestCatalogRegions(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	regions := cat.Regions()
	if len(regions) < 2 {
		t.Fatalf("expected at least 2 regions, got %v", regions)
	}
}
```

**Step 7: Run all catalog tests**

Run: `cd host-cli && go test ./internal/catalog/ -v`
Expected: ALL PASS

**Step 8: Commit**

```bash
git add host-cli/internal/catalog/catalog.go host-cli/internal/catalog/catalog_test.go
git commit -m "feat(catalog): add preset extends resolution, kind, display_group, and helper methods"
```

---

## Task 2: Volume Detection Utility

Port Python's volume detection to a Go utility in `host-cli`. Detects mounted volumes on macOS and Linux with size info and network classification.

**Files:**
- Create: `host-cli/internal/volumes/detect.go`
- Create: `host-cli/internal/volumes/detect_test.go`

**Step 1: Write the failing test**

```go
package volumes

import "testing"

func TestDetectVolumesReturnsSlice(t *testing.T) {
	vols := Detect()
	// Should at least return something on any OS (even if empty)
	if vols == nil {
		t.Error("Detect() returned nil, expected empty slice")
	}
}

func TestVolumeHasSizeInfo(t *testing.T) {
	vols := Detect()
	for _, v := range vols {
		if v.Path == "" {
			t.Error("volume has empty path")
		}
		if v.TotalGB < 0 {
			t.Errorf("volume %s has negative TotalGB", v.Path)
		}
	}
}

func TestHomeSvalbardOption(t *testing.T) {
	home := HomeSvalbardVolume()
	if home.Path == "" {
		t.Error("HomeSvalbardVolume() returned empty path")
	}
	if home.Name != "~/svalbard/" {
		t.Errorf("expected name '~/svalbard/', got %q", home.Name)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd host-cli && go test ./internal/volumes/ -v`
Expected: FAIL — package doesn't exist

**Step 3: Implement volume detection**

```go
package volumes

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// Volume represents a mounted storage device.
type Volume struct {
	Path    string
	Name    string
	TotalGB float64
	FreeGB  float64
	Network bool
}

var networkFSTypes = map[string]bool{
	"smbfs": true, "nfs": true, "nfs4": true,
	"afpfs": true, "cifs": true, "9p": true, "fuse.sshfs": true,
}

var skipNames = map[string]bool{
	"Macintosh HD": true, "com.apple.TimeMachine.localsnapshots": true,
	".timemachine": true, ".MobileBackups": true,
}

var skipMarkers = []string{"Backups.backupdb", ".timemachine"}

// Detect detects mounted volumes, sorted: local first (by size), then network.
func Detect() []Volume {
	mountTypes := parseMountTypes()
	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		entries, err := os.ReadDir("/Volumes")
		if err != nil {
			return []Volume{}
		}
		for _, e := range entries {
			path := filepath.Join("/Volumes", e.Name())
			if !shouldSkip(path, e.Name()) {
				candidates = append(candidates, path)
			}
		}
	default: // Linux
		user := os.Getenv("USER")
		if mediaUser := filepath.Join("/media", user); user != "" {
			if entries, err := os.ReadDir(mediaUser); err == nil {
				for _, e := range entries {
					candidates = append(candidates, filepath.Join(mediaUser, e.Name()))
				}
			}
		}
		if entries, err := os.ReadDir("/mnt"); err == nil {
			for _, e := range entries {
				candidates = append(candidates, filepath.Join("/mnt", e.Name()))
			}
		}
	}

	var volumes []Volume
	for _, path := range candidates {
		if shouldSkip(path, filepath.Base(path)) {
			continue
		}
		var stat unix.Statfs_t
		if err := unix.Statfs(path, &stat); err != nil {
			continue
		}
		totalGB := float64(stat.Blocks) * float64(stat.Bsize) / 1e9
		freeGB := float64(stat.Bavail) * float64(stat.Bsize) / 1e9
		fsType := mountTypes[path]
		volumes = append(volumes, Volume{
			Path:    filepath.Join(path, "svalbard"),
			Name:    filepath.Base(path),
			TotalGB: totalGB,
			FreeGB:  freeGB,
			Network: networkFSTypes[fsType],
		})
	}

	sort.Slice(volumes, func(i, j int) bool {
		if volumes[i].Network != volumes[j].Network {
			return !volumes[i].Network
		}
		return volumes[i].TotalGB < volumes[j].TotalGB
	})
	return volumes
}

// HomeSvalbardVolume returns a volume entry for ~/svalbard/.
func HomeSvalbardVolume() Volume {
	home, _ := os.UserHomeDir()
	svalbardPath := filepath.Join(home, "svalbard")
	v := Volume{
		Path: svalbardPath,
		Name: "~/svalbard/",
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(home, &stat); err == nil {
		v.FreeGB = float64(stat.Bavail) * float64(stat.Bsize) / 1e9
		v.TotalGB = float64(stat.Blocks) * float64(stat.Bsize) / 1e9
	}
	return v
}

// FreeSpaceGB returns free space at the given path (walks up to find existing dir).
func FreeSpaceGB(path string) float64 {
	check := path
	for {
		var stat unix.Statfs_t
		if err := unix.Statfs(check, &stat); err == nil {
			return float64(stat.Bavail) * float64(stat.Bsize) / 1e9
		}
		parent := filepath.Dir(check)
		if parent == check {
			return 0
		}
		check = parent
	}
}

func shouldSkip(path, name string) bool {
	if skipNames[name] {
		return true
	}
	for _, part := range strings.Split(path, string(os.PathSeparator)) {
		if skipNames[part] {
			return true
		}
	}
	for _, marker := range skipMarkers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	return false
}

func parseMountTypes() map[string]string {
	result := make(map[string]string)
	out, err := exec.Command("mount").Output()
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, " on ") {
			continue
		}
		rest := strings.SplitN(line, " on ", 2)[1]
		if strings.Contains(rest, " type ") {
			parts := strings.SplitN(rest, " type ", 2)
			mountPoint := strings.TrimSpace(parts[0])
			fsType := strings.Fields(parts[1])[0]
			result[mountPoint] = strings.TrimRight(fsType, ",")
		} else if idx := strings.LastIndex(rest, " ("); idx >= 0 {
			mountPoint := strings.TrimSpace(rest[:idx])
			inner := rest[idx+2:]
			fsType := strings.SplitN(inner, ",", 2)[0]
			result[mountPoint] = strings.TrimRight(fsType, ")")
		}
	}
	return result
}
```

**Step 4: Run tests**

Run: `cd host-cli && go test ./internal/volumes/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add host-cli/internal/volumes/
git commit -m "feat(volumes): add mounted volume detection for wizard"
```

---

## Task 3: Wizard Data Types and Sub-Model Interface

Define the types that flow between `host-cli` (data provider) and `host-tui` (wizard UI). These types are declared in `host-tui` so the wizard can use them without importing catalog.

**Files:**
- Create: `host-tui/internal/wizard/types.go`
- Modify: `host-tui/internal/wizard/model.go`

**Step 1: Create wizard types**

```go
package wizard

// Volume is a detected storage mount point.
type Volume struct {
	Path    string  // e.g. "/Volumes/KINGSTON/svalbard"
	Name    string  // e.g. "KINGSTON"
	TotalGB float64
	FreeGB  float64
	Network bool
}

// PresetOption is a preset the user can pick.
type PresetOption struct {
	Name         string
	Description  string
	ContentGB    float64 // total resolved content size
	TargetSizeGB float64
	Region       string
	SourceIDs    []string // resolved source IDs (with extends)
}

// PackGroup is a display group containing packs.
type PackGroup struct {
	Name  string // display_group value, e.g. "Maps & Geodata"
	Packs []Pack
}

// Pack is a named bundle of sources (kind: pack).
type Pack struct {
	Name        string
	Description string
	Sources     []PackSource
}

// PackSource is a single recipe inside a pack.
type PackSource struct {
	ID          string
	Description string
	SizeGB      float64
}

// WizardConfig is everything the wizard needs to run.
// Prepared by host-cli, consumed by host-tui.
type WizardConfig struct {
	Volumes     []Volume
	HomeVolume  Volume
	Presets     []PresetOption // for the selected region
	Regions     []string       // available regions
	PackGroups  []PackGroup    // all packs grouped
	PrefillPath string
	StartAtStep int
}

// WizardResult is returned when the wizard completes.
type WizardResult struct {
	VaultPath   string
	SelectedIDs []string
	PresetName  string // empty if custom
	Region      string
}

// DoneMsg is emitted by the wizard when it completes.
type DoneMsg struct {
	Result WizardResult
}
```

**Step 2: Commit**

```bash
git add host-tui/internal/wizard/types.go
git commit -m "feat(wizard): add data types for wizard config and result"
```

---

## Task 4: PathPicker Sub-Model

Interactive volume/path selection. Shows numbered list of detected volumes, home dir option, and custom path input. Sends `PathDoneMsg` when confirmed.

**Files:**
- Create: `host-tui/internal/wizard/pathpicker.go`
- Create: `host-tui/internal/wizard/pathpicker_test.go`

**Step 1: Write failing tests**

```go
package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPathPickerShowsVolumes(t *testing.T) {
	vols := []Volume{
		{Path: "/Volumes/USB/svalbard", Name: "USB", TotalGB: 64, FreeGB: 50},
		{Path: "/Volumes/NAS/svalbard", Name: "NAS", TotalGB: 1000, FreeGB: 500, Network: true},
	}
	home := Volume{Path: "/Users/test/svalbard", Name: "~/svalbard/", FreeGB: 100}
	m := newPathPicker(vols, home, "")
	m.width = 80
	m.height = 24
	out := stripAnsi(m.View())

	if !strings.Contains(out, "USB") {
		t.Error("should show USB volume")
	}
	if !strings.Contains(out, "NAS") {
		t.Error("should show NAS volume")
	}
	if !strings.Contains(out, "~/svalbard/") {
		t.Error("should show home option")
	}
	if !strings.Contains(out, "Custom path") {
		t.Error("should show custom path option")
	}
}

func TestPathPickerSelectsVolume(t *testing.T) {
	vols := []Volume{
		{Path: "/Volumes/USB/svalbard", Name: "USB", TotalGB: 64, FreeGB: 50},
	}
	home := Volume{Path: "/Users/test/svalbard", Name: "~/svalbard/", FreeGB: 100}
	m := newPathPicker(vols, home, "")
	m.width = 80
	m.height = 24

	// Select first volume and press enter
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(pathPickerModel)
	_ = pm

	if cmd == nil {
		t.Fatal("expected a command on Enter")
	}
	msg := cmd()
	done, ok := msg.(pathDoneMsg)
	if !ok {
		t.Fatalf("expected pathDoneMsg, got %T", msg)
	}
	if done.path != "/Volumes/USB/svalbard" {
		t.Errorf("expected USB path, got %q", done.path)
	}
}

func TestPathPickerNavigates(t *testing.T) {
	vols := []Volume{
		{Path: "/Volumes/A/svalbard", Name: "A", TotalGB: 32, FreeGB: 20},
		{Path: "/Volumes/B/svalbard", Name: "B", TotalGB: 64, FreeGB: 50},
	}
	home := Volume{Path: "/Users/test/svalbard", Name: "~/svalbard/", FreeGB: 100}
	m := newPathPicker(vols, home, "")
	m.width = 80
	m.height = 24

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(pathPickerModel)
	if pm.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", pm.cursor)
	}
}

func TestPathPickerCustomInput(t *testing.T) {
	m := newPathPicker(nil, Volume{Path: "/Users/test/svalbard"}, "")
	m.width = 80
	m.height = 24

	// Navigate to custom option (after home = index 1, custom = index 1 with no volumes)
	// With no volumes: index 0 = home, index 1 = custom
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	pm := updated.(pathPickerModel)

	// Press enter on custom to activate text input mode
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm = updated.(pathPickerModel)
	if !pm.customInput {
		t.Error("expected custom input mode")
	}
}

func TestPathPickerPrefill(t *testing.T) {
	m := newPathPicker(nil, Volume{Path: "/Users/test/svalbard"}, "/mnt/drive")
	m.width = 80
	m.height = 24

	// Prefilled path should show and be selectable directly
	out := stripAnsi(m.View())
	if !strings.Contains(out, "/mnt/drive") {
		t.Error("should show prefilled path")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd host-tui && go test ./internal/wizard/ -run TestPathPicker -v`
Expected: FAIL — pathPickerModel doesn't exist

**Step 3: Implement PathPicker**

Create `pathpicker.go`:

```go
package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

// pathDoneMsg is sent when the user confirms a vault path.
type pathDoneMsg struct {
	path   string
	freeGB float64
}

type pathOption struct {
	label   string
	detail  string // e.g. "50/64 GB"
	path    string
	freeGB  float64
	network bool
	custom  bool // the "Custom path..." entry
}

type pathPickerModel struct {
	options     []pathOption
	cursor      int
	customInput bool   // true when typing custom path
	inputBuffer string // custom path text
	width       int
	height      int
	theme       tui.Theme
	keys        tui.KeyMap
}

func newPathPicker(volumes []Volume, home Volume, prefill string) pathPickerModel {
	var opts []pathOption

	// Prefilled path first (if provided)
	if prefill != "" {
		opts = append(opts, pathOption{
			label:  prefill,
			detail: "provided path",
			path:   prefill,
		})
	}

	// Detected volumes
	for _, v := range volumes {
		style := ""
		if v.Network {
			style = " [network]"
		}
		opts = append(opts, pathOption{
			label:   fmt.Sprintf("%s%s", v.Path, style),
			detail:  fmt.Sprintf("%.0f/%.0f GB", v.FreeGB, v.TotalGB),
			path:    v.Path,
			freeGB:  v.FreeGB,
			network: v.Network,
		})
	}

	// Home directory
	detail := "home directory"
	if home.FreeGB > 0 {
		detail = fmt.Sprintf("%.0f GB free", home.FreeGB)
	}
	opts = append(opts, pathOption{
		label:  home.Name,
		detail: detail,
		path:   home.Path,
		freeGB: home.FreeGB,
	})

	// Custom path
	opts = append(opts, pathOption{
		label:  "Custom path...",
		custom: true,
	})

	return pathPickerModel{
		options: opts,
		theme:   tui.DefaultTheme(),
		keys:    tui.DefaultKeyMap(),
	}
}

func (m pathPickerModel) Init() tea.Cmd { return nil }

func (m pathPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.customInput {
		return m.updateCustomInput(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case m.keys.MoveDown.Matches(msg):
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
			}
		case m.keys.Enter.Matches(msg):
			opt := m.options[m.cursor]
			if opt.custom {
				m.customInput = true
				m.inputBuffer = ""
				return m, nil
			}
			return m, func() tea.Msg {
				return pathDoneMsg{path: opt.path, freeGB: opt.freeGB}
			}
		}
	}
	return m, nil
}

func (m pathPickerModel) updateCustomInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			path := strings.TrimSpace(m.inputBuffer)
			if path != "" {
				return m, func() tea.Msg {
					return pathDoneMsg{path: path}
				}
			}
		case tea.KeyEscape:
			m.customInput = false
			m.inputBuffer = ""
		case tea.KeyBackspace:
			if len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
		default:
			if len(msg.Runes) > 0 {
				m.inputBuffer += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m pathPickerModel) View() string {
	var b strings.Builder

	if m.customInput {
		b.WriteString("  Enter path: " + m.inputBuffer + "█\n\n")
		b.WriteString("  Press Enter to confirm, Esc to cancel")
		return b.String()
	}

	for i, opt := range m.options {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		num := fmt.Sprintf("%d", i+1)
		if opt.custom {
			num = "c"
		}
		line := fmt.Sprintf("%s%s) %s", prefix, num, opt.label)
		if opt.detail != "" {
			line += "  " + opt.detail
		}
		b.WriteString(line + "\n")
	}

	return b.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `cd host-tui && go test ./internal/wizard/ -run TestPathPicker -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add host-tui/internal/wizard/pathpicker.go host-tui/internal/wizard/pathpicker_test.go
git commit -m "feat(wizard): add PathPicker sub-model with volume selection"
```

---

## Task 5: PresetPicker Sub-Model

Shows region selection then presets that fit available space. Presets too large are dimmed. Largest fitting preset is recommended.

**Files:**
- Create: `host-tui/internal/wizard/presetpicker.go`
- Create: `host-tui/internal/wizard/presetpicker_test.go`

**Step 1: Write failing tests**

```go
package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPresetPickerShowsPresets(t *testing.T) {
	presets := []PresetOption{
		{Name: "default-2", Description: "Bugout kit", ContentGB: 1.5, TargetSizeGB: 2},
		{Name: "default-32", Description: "Broad reference", ContentGB: 25, TargetSizeGB: 32},
		{Name: "default-128", Description: "Full reference", ContentGB: 100, TargetSizeGB: 128},
	}
	m := newPresetPicker(presets, []string{"default", "finland"}, 50)
	m.width = 80
	m.height = 24
	out := stripAnsi(m.View())

	if !strings.Contains(out, "default-2") {
		t.Error("should show default-2")
	}
	if !strings.Contains(out, "default-32") {
		t.Error("should show default-32")
	}
}

func TestPresetPickerHighlightsRecommended(t *testing.T) {
	presets := []PresetOption{
		{Name: "default-2", ContentGB: 1.5, TargetSizeGB: 2},
		{Name: "default-32", ContentGB: 25, TargetSizeGB: 32},
		{Name: "default-128", ContentGB: 100, TargetSizeGB: 128},
	}
	m := newPresetPicker(presets, []string{"default"}, 50)
	// default-32 fits (25 <= 50), default-128 doesn't (100 > 50)
	// Recommended = largest fitting = default-32 = index 1
	if m.cursor != 1 {
		t.Errorf("expected cursor at recommended preset index 1, got %d", m.cursor)
	}
}

func TestPresetPickerSelectsSendsMsg(t *testing.T) {
	presets := []PresetOption{
		{Name: "default-2", ContentGB: 1.5, SourceIDs: []string{"kiwix-serve", "sqlite3"}},
	}
	m := newPresetPicker(presets, []string{"default"}, 50)
	m.width = 80
	m.height = 24

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on Enter")
	}
	msg := cmd()
	done, ok := msg.(presetDoneMsg)
	if !ok {
		t.Fatalf("expected presetDoneMsg, got %T", msg)
	}
	if done.preset.Name != "default-2" {
		t.Errorf("expected default-2, got %q", done.preset.Name)
	}
}

func TestPresetPickerSkipOption(t *testing.T) {
	presets := []PresetOption{
		{Name: "default-2", ContentGB: 1.5},
	}
	m := newPresetPicker(presets, []string{"default"}, 50)
	m.width = 80
	m.height = 24

	// Navigate to "skip" option (last item)
	for i := 0; i < len(presets)+1; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(presetPickerModel)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on Enter for skip")
	}
	msg := cmd()
	done, ok := msg.(presetDoneMsg)
	if !ok {
		t.Fatalf("expected presetDoneMsg, got %T", msg)
	}
	if done.preset.Name != "" {
		t.Error("skip option should return empty preset name")
	}
}
```

**Step 2: Run tests**

Run: `cd host-tui && go test ./internal/wizard/ -run TestPresetPicker -v`
Expected: FAIL

**Step 3: Implement PresetPicker**

Create `presetpicker.go` — numbered list of presets with size, description, fits/over indicator. "Customize" option at the bottom sends empty preset (goes to pack picker with nothing pre-checked). Region selector if multiple regions.

```go
package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

type presetDoneMsg struct {
	preset PresetOption
}

type presetPickerModel struct {
	presets     []PresetOption
	regions     []string
	freeGB      float64
	cursor      int
	hasCustom   bool // "Customize" option appended
	width       int
	height      int
	theme       tui.Theme
	keys        tui.KeyMap
}

func newPresetPicker(presets []PresetOption, regions []string, freeGB float64) presetPickerModel {
	// Find recommended = largest fitting preset
	recommended := 0
	for i, p := range presets {
		if p.ContentGB <= freeGB {
			recommended = i
		}
	}

	return presetPickerModel{
		presets:   presets,
		regions:   regions,
		freeGB:    freeGB,
		cursor:    recommended,
		hasCustom: true,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

func (m presetPickerModel) itemCount() int {
	n := len(m.presets)
	if m.hasCustom {
		n++ // "Customize" option
	}
	return n
}

func (m presetPickerModel) Init() tea.Cmd { return nil }

func (m presetPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case m.keys.MoveDown.Matches(msg):
			if m.cursor < m.itemCount()-1 {
				m.cursor++
			}
		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
			}
		case m.keys.Enter.Matches(msg):
			if m.cursor < len(m.presets) {
				return m, func() tea.Msg {
					return presetDoneMsg{preset: m.presets[m.cursor]}
				}
			}
			// Customize option
			return m, func() tea.Msg {
				return presetDoneMsg{preset: PresetOption{}}
			}
		}
	}
	return m, nil
}

func (m presetPickerModel) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("  %.0f GB free\n\n", m.freeGB))

	for i, p := range m.presets {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		fits := p.ContentGB <= m.freeGB
		sizeStr := formatSizeGB(p.ContentGB)
		line := fmt.Sprintf("%s%d) %-18s %8s  %s", prefix, i+1, p.Name, sizeStr, p.Description)
		if !fits {
			over := p.ContentGB - m.freeGB
			line += fmt.Sprintf(" (needs ~%.0f GB more)", over)
		}
		b.WriteString(line + "\n")
	}

	if m.hasCustom {
		prefix := "  "
		idx := len(m.presets)
		if m.cursor == idx {
			prefix = "> "
		}
		b.WriteString(fmt.Sprintf("%sc) Customize — browse all content\n", prefix))
	}

	return b.String()
}

func formatSizeGB(gb float64) string {
	if gb >= 1 {
		return fmt.Sprintf("~%.0f GB", gb)
	}
	return fmt.Sprintf("~%.0f MB", gb*1024)
}
```

**Step 4: Run tests**

Run: `cd host-tui && go test ./internal/wizard/ -run TestPresetPicker -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add host-tui/internal/wizard/presetpicker.go host-tui/internal/wizard/presetpicker_test.go
git commit -m "feat(wizard): add PresetPicker sub-model with size-aware selection"
```

---

## Task 6: PackPicker Sub-Model (the complex one)

Port the Python picker.py to Go/Bubble Tea. 3-level collapsible tree (Group → Pack → Item) with Space toggle, tri-state checkboxes, running size total, fits/over indicator.

**Files:**
- Create: `host-tui/internal/wizard/packpicker.go`
- Create: `host-tui/internal/wizard/packpicker_test.go`

**Step 1: Write failing tests**

```go
package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func samplePackGroups() []PackGroup {
	return []PackGroup{
		{
			Name: "Core",
			Packs: []Pack{
				{
					Name:        "core",
					Description: "Universal foundation",
					Sources: []PackSource{
						{ID: "wikiciv", Description: "Wikipedia Civilization", SizeGB: 0.1},
						{ID: "permacomputing", Description: "Permacomputing Wiki", SizeGB: 0.05},
					},
				},
			},
		},
		{
			Name: "Maps & Geodata",
			Packs: []Pack{
				{
					Name:        "fi-maps",
					Description: "Finnish maps and geodata",
					Sources: []PackSource{
						{ID: "osm-finland", Description: "OpenStreetMap Finland", SizeGB: 3.0},
						{ID: "natural-earth", Description: "Natural Earth vectors", SizeGB: 0.3},
					},
				},
			},
		},
	}
}

func TestPackPickerShowsGroupsAndPacks(t *testing.T) {
	m := newPackPicker(samplePackGroups(), nil, 64)
	m.width = 80
	m.height = 24
	out := stripAnsi(m.View())

	if !strings.Contains(out, "Core") {
		t.Error("should show Core group")
	}
	if !strings.Contains(out, "Maps & Geodata") {
		t.Error("should show Maps group")
	}
	if !strings.Contains(out, "core") {
		t.Error("should show core pack")
	}
	if !strings.Contains(out, "fi-maps") {
		t.Error("should show fi-maps pack")
	}
}

func TestPackPickerTogglePack(t *testing.T) {
	m := newPackPicker(samplePackGroups(), nil, 64)
	m.width = 80
	m.height = 24

	// Navigate to first pack (cursor starts at 0 = first group, down = first pack)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(packPickerModel)

	// Toggle with space
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(packPickerModel)

	// Both sources in "core" pack should be checked
	if !m.checkedIDs["wikiciv"] || !m.checkedIDs["permacomputing"] {
		t.Error("toggling pack should check all its sources")
	}

	// Toggle again — should uncheck all
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(packPickerModel)

	if m.checkedIDs["wikiciv"] || m.checkedIDs["permacomputing"] {
		t.Error("toggling pack again should uncheck all its sources")
	}
}

func TestPackPickerExpandCollapse(t *testing.T) {
	m := newPackPicker(samplePackGroups(), nil, 64)
	m.width = 80
	m.height = 24

	// Navigate to first pack
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(packPickerModel)

	// Packs start collapsed. Expand with Enter.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(packPickerModel)

	out := stripAnsi(m.View())
	// After expanding, individual sources should be visible
	if !strings.Contains(out, "Wikipedia Civilization") && !strings.Contains(out, "wikiciv") {
		t.Error("expanding pack should show individual sources")
	}
}

func TestPackPickerTriState(t *testing.T) {
	m := newPackPicker(samplePackGroups(), nil, 64)
	m.width = 80
	m.height = 24

	// Navigate to first pack and expand it
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(packPickerModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(packPickerModel)

	// Navigate to first source and toggle it
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(packPickerModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(packPickerModel)

	// Now "core" pack has 1/2 checked = partial state
	out := m.View()
	if !strings.Contains(out, "◐") {
		t.Error("pack with partial selection should show ◐")
	}
}

func TestPackPickerPreChecked(t *testing.T) {
	checked := map[string]bool{"wikiciv": true, "osm-finland": true}
	m := newPackPicker(samplePackGroups(), checked, 64)

	if !m.checkedIDs["wikiciv"] {
		t.Error("wikiciv should be pre-checked")
	}
	if !m.checkedIDs["osm-finland"] {
		t.Error("osm-finland should be pre-checked")
	}
}

func TestPackPickerSizeTotal(t *testing.T) {
	checked := map[string]bool{"osm-finland": true, "natural-earth": true}
	m := newPackPicker(samplePackGroups(), checked, 64)
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())
	// Total should be 3.3 GB (3.0 + 0.3)
	if !strings.Contains(out, "3.3") {
		t.Errorf("should show total ~3.3 GB, got:\n%s", out)
	}
	if !strings.Contains(out, "fits") {
		t.Error("3.3 GB should fit in 64 GB")
	}
}

func TestPackPickerOverBudget(t *testing.T) {
	checked := map[string]bool{"osm-finland": true}
	m := newPackPicker(samplePackGroups(), checked, 2) // only 2 GB free
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())
	if !strings.Contains(out, "over") {
		t.Error("3.0 GB in 2 GB should show 'over'")
	}
}

func TestPackPickerApply(t *testing.T) {
	checked := map[string]bool{"wikiciv": true}
	m := newPackPicker(samplePackGroups(), checked, 64)
	m.width = 80
	m.height = 24

	// Press 'a' to apply
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected command on 'a'")
	}
	msg := cmd()
	done, ok := msg.(packDoneMsg)
	if !ok {
		t.Fatalf("expected packDoneMsg, got %T", msg)
	}
	if !done.selectedIDs["wikiciv"] {
		t.Error("should include wikiciv in selection")
	}
}

func TestPackPickerCancel(t *testing.T) {
	m := newPackPicker(samplePackGroups(), nil, 64)
	m.width = 80
	m.height = 24

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected command on 'q'")
	}
	msg := cmd()
	if _, ok := msg.(packCancelMsg); !ok {
		t.Fatalf("expected packCancelMsg, got %T", msg)
	}
}
```

**Step 2: Run tests**

Run: `cd host-tui && go test ./internal/wizard/ -run TestPackPicker -v`
Expected: FAIL

**Step 3: Implement PackPicker**

Create `packpicker.go` (~250 lines). This is a direct port of Python's `picker.py`:

```go
package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

type packDoneMsg struct {
	selectedIDs map[string]bool
}

type packCancelMsg struct{}

const (
	rowGroup = iota
	rowPack
	rowItem
)

type pickerRow struct {
	kind       int
	groupName  string
	pack       *Pack
	source     *PackSource
	groupPacks []Pack // for group rows
}

type packPickerModel struct {
	groups          []PackGroup
	checkedIDs      map[string]bool
	collapsedGroups map[string]bool
	collapsedPacks  map[string]bool
	rows            []pickerRow
	cursor          int
	scrollOffset    int
	freeGB          float64
	width           int
	height          int
	theme           tui.Theme
	keys            tui.KeyMap
}

func newPackPicker(groups []PackGroup, checked map[string]bool, freeGB float64) packPickerModel {
	checkedIDs := make(map[string]bool)
	for id, v := range checked {
		if v {
			checkedIDs[id] = true
		}
	}
	collapsedGroups := make(map[string]bool)
	collapsedPacks := make(map[string]bool)
	for _, g := range groups {
		for _, p := range g.Packs {
			collapsedPacks[p.Name] = true // packs start collapsed
		}
	}

	m := packPickerModel{
		groups:          groups,
		checkedIDs:      checkedIDs,
		collapsedGroups: collapsedGroups,
		collapsedPacks:  collapsedPacks,
		freeGB:          freeGB,
		theme:           tui.DefaultTheme(),
		keys:            tui.DefaultKeyMap(),
	}
	m.rebuildRows()
	// Move cursor to first selectable row
	if len(m.rows) > 0 {
		m.cursor = m.nextSelectable(-1, 1)
	}
	return m
}

func (m *packPickerModel) rebuildRows() {
	m.rows = nil
	for _, g := range m.groups {
		m.rows = append(m.rows, pickerRow{
			kind:       rowGroup,
			groupName:  g.Name,
			groupPacks: g.Packs,
		})
		if m.collapsedGroups[g.Name] {
			continue
		}
		for i := range g.Packs {
			pack := &g.Packs[i]
			m.rows = append(m.rows, pickerRow{
				kind: rowPack,
				pack: pack,
			})
			if m.collapsedPacks[pack.Name] {
				continue
			}
			for j := range pack.Sources {
				src := &pack.Sources[j]
				m.rows = append(m.rows, pickerRow{
					kind:   rowItem,
					source: src,
					pack:   pack,
				})
			}
		}
	}
}

func (m *packPickerModel) nextSelectable(from, dir int) int {
	n := len(m.rows)
	if n == 0 {
		return 0
	}
	idx := from
	for i := 0; i < n; i++ {
		idx = (idx + dir + n) % n
		return idx // all rows are selectable
	}
	return 0
}

func (m packPickerModel) totalCheckedGB() float64 {
	seen := make(map[string]bool)
	total := 0.0
	for _, g := range m.groups {
		for _, p := range g.Packs {
			for _, src := range p.Sources {
				if m.checkedIDs[src.ID] && !seen[src.ID] {
					seen[src.ID] = true
					total += src.SizeGB
				}
			}
		}
	}
	return total
}

func (m packPickerModel) packCheckState(pack *Pack) (checked, total int) {
	for _, src := range pack.Sources {
		total++
		if m.checkedIDs[src.ID] {
			checked++
		}
	}
	return
}

func (m packPickerModel) Init() tea.Cmd { return nil }

func (m packPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case m.keys.MoveDown.Matches(msg):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			m.updateScroll()

		case m.keys.MoveUp.Matches(msg):
			if m.cursor > 0 {
				m.cursor--
			}
			m.updateScroll()

		case m.keys.Enter.Matches(msg):
			if m.cursor >= 0 && m.cursor < len(m.rows) {
				row := m.rows[m.cursor]
				switch row.kind {
				case rowGroup:
					m.collapsedGroups[row.groupName] = !m.collapsedGroups[row.groupName]
					m.rebuildRows()
					if m.cursor >= len(m.rows) {
						m.cursor = len(m.rows) - 1
					}
				case rowPack:
					m.collapsedPacks[row.pack.Name] = !m.collapsedPacks[row.pack.Name]
					m.rebuildRows()
				}
			}

		case m.keys.Toggle.Matches(msg):
			if m.cursor >= 0 && m.cursor < len(m.rows) {
				row := m.rows[m.cursor]
				switch row.kind {
				case rowGroup:
					// Toggle all packs in group
					m.toggleGroup(row)
				case rowPack:
					m.togglePack(row.pack)
				case rowItem:
					if m.checkedIDs[row.source.ID] {
						delete(m.checkedIDs, row.source.ID)
					} else {
						m.checkedIDs[row.source.ID] = true
					}
				}
			}

		case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'a':
			selected := make(map[string]bool)
			for id := range m.checkedIDs {
				selected[id] = true
			}
			return m, func() tea.Msg { return packDoneMsg{selectedIDs: selected} }

		case m.keys.Quit.Matches(msg):
			return m, func() tea.Msg { return packCancelMsg{} }
		}
	}
	return m, nil
}

func (m *packPickerModel) togglePack(pack *Pack) {
	packIDs := make(map[string]bool)
	for _, src := range pack.Sources {
		packIDs[src.ID] = true
	}
	// If all checked, uncheck all. Otherwise check all.
	allChecked := true
	for id := range packIDs {
		if !m.checkedIDs[id] {
			allChecked = false
			break
		}
	}
	if allChecked {
		for id := range packIDs {
			delete(m.checkedIDs, id)
		}
	} else {
		for id := range packIDs {
			m.checkedIDs[id] = true
		}
	}
}

func (m *packPickerModel) toggleGroup(row pickerRow) {
	// Check if all sources in all packs of this group are checked
	allChecked := true
	for _, pack := range row.groupPacks {
		for _, src := range pack.Sources {
			if !m.checkedIDs[src.ID] {
				allChecked = false
				break
			}
		}
		if !allChecked {
			break
		}
	}
	for _, pack := range row.groupPacks {
		for _, src := range pack.Sources {
			if allChecked {
				delete(m.checkedIDs, src.ID)
			} else {
				m.checkedIDs[src.ID] = true
			}
		}
	}
}

func (m *packPickerModel) updateScroll() {
	maxVisible := m.maxVisible()
	if m.cursor < m.scrollOffset+2 {
		m.scrollOffset = max(0, m.cursor-2)
	} else if m.cursor >= m.scrollOffset+maxVisible-2 {
		m.scrollOffset = min(max(0, len(m.rows)-maxVisible), m.cursor-maxVisible+3)
	}
}

func (m packPickerModel) maxVisible() int {
	h := m.height - 8 // reserve for header + footer
	if h < 8 {
		return 8
	}
	return h
}

func (m packPickerModel) View() string {
	var b strings.Builder
	totalGB := m.totalCheckedGB()
	fits := m.freeGB <= 0 || totalGB <= m.freeGB

	maxVisible := m.maxVisible()
	end := m.scrollOffset + maxVisible
	if end > len(m.rows) {
		end = len(m.rows)
	}
	visibleRows := m.rows[m.scrollOffset:end]

	for offset, row := range visibleRows {
		absIdx := m.scrollOffset + offset
		prefix := "  "
		if absIdx == m.cursor {
			prefix = "> "
		}

		switch row.kind {
		case rowGroup:
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, row.groupName))

		case rowPack:
			checked, total := m.packCheckState(row.pack)
			mark := "☐"
			if checked == total && total > 0 {
				mark = "☑"
			} else if checked > 0 {
				mark = "◐"
			}
			checkedSize := 0.0
			for _, src := range row.pack.Sources {
				if m.checkedIDs[src.ID] {
					checkedSize += src.SizeGB
				}
			}
			b.WriteString(fmt.Sprintf("    %s%s %s  %s\n",
				prefix, mark, row.pack.Name, formatSizeGB(checkedSize)))

		case rowItem:
			mark := "☐"
			if m.checkedIDs[row.source.ID] {
				mark = "☑"
			}
			label := row.source.Description
			if label == "" {
				label = row.source.ID
			}
			b.WriteString(fmt.Sprintf("        %s%s %s  %s\n",
				prefix, mark, label, formatSizeGB(row.source.SizeGB)))
		}
	}

	// Footer
	b.WriteString("\n")
	if fits {
		b.WriteString(fmt.Sprintf("  Total: %.1f / %.0f GB  fits\n", totalGB, m.freeGB))
	} else {
		over := totalGB - m.freeGB
		b.WriteString(fmt.Sprintf("  Total: %.1f / %.0f GB  %.1f GB over\n", totalGB, m.freeGB, over))
	}
	b.WriteString("  j/k navigate  SPACE toggle  ENTER expand/collapse  a apply  q cancel\n")

	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

**Step 4: Run tests**

Run: `cd host-tui && go test ./internal/wizard/ -run TestPackPicker -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add host-tui/internal/wizard/packpicker.go host-tui/internal/wizard/packpicker_test.go
git commit -m "feat(wizard): add PackPicker sub-model with tri-state tree"
```

---

## Task 7: Review Sub-Model

Shows a summary table of selected items with type, size, and description. Confirm proceeds, Esc goes back.

**Files:**
- Create: `host-tui/internal/wizard/review.go`
- Create: `host-tui/internal/wizard/review_test.go`

**Step 1: Write failing tests**

```go
package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestReviewShowsSelectedItems(t *testing.T) {
	items := []ReviewItem{
		{ID: "kiwix-serve", Type: "binary", SizeGB: 0.003, Description: "Kiwix web server"},
		{ID: "osm-finland", Type: "pmtiles", SizeGB: 3.0, Description: "OpenStreetMap Finland"},
	}
	m := newReviewModel("/mnt/vault", items, 64)
	m.width = 80
	m.height = 24
	out := stripAnsi(m.View())

	if !strings.Contains(out, "kiwix-serve") {
		t.Error("should show kiwix-serve")
	}
	if !strings.Contains(out, "osm-finland") {
		t.Error("should show osm-finland")
	}
	if !strings.Contains(out, "/mnt/vault") {
		t.Error("should show vault path")
	}
}

func TestReviewShowsTotal(t *testing.T) {
	items := []ReviewItem{
		{ID: "a", SizeGB: 1.5},
		{ID: "b", SizeGB: 2.5},
	}
	m := newReviewModel("/mnt/vault", items, 64)
	m.width = 80
	m.height = 24
	out := stripAnsi(m.View())

	if !strings.Contains(out, "4.0") {
		t.Error("should show total 4.0 GB")
	}
}

func TestReviewConfirm(t *testing.T) {
	items := []ReviewItem{{ID: "a", SizeGB: 1.0}}
	m := newReviewModel("/mnt/vault", items, 64)
	m.width = 80
	m.height = 24

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on Enter")
	}
	msg := cmd()
	if _, ok := msg.(reviewConfirmMsg); !ok {
		t.Fatalf("expected reviewConfirmMsg, got %T", msg)
	}
}

func TestReviewCancel(t *testing.T) {
	items := []ReviewItem{{ID: "a", SizeGB: 1.0}}
	m := newReviewModel("/mnt/vault", items, 64)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command on Esc")
	}
	msg := cmd()
	if _, ok := msg.(reviewBackMsg); !ok {
		t.Fatalf("expected reviewBackMsg, got %T", msg)
	}
}
```

**Step 2: Run, verify fail**

Run: `cd host-tui && go test ./internal/wizard/ -run TestReview -v`

**Step 3: Implement Review**

```go
package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

type reviewConfirmMsg struct{}
type reviewBackMsg struct{}

// ReviewItem is a resolved item for the review screen.
type ReviewItem struct {
	ID          string
	Type        string
	SizeGB      float64
	Description string
}

type reviewModel struct {
	vaultPath    string
	items        []ReviewItem
	freeGB       float64
	scrollOffset int
	width        int
	height       int
	theme        tui.Theme
	keys         tui.KeyMap
}

func newReviewModel(vaultPath string, items []ReviewItem, freeGB float64) reviewModel {
	return reviewModel{
		vaultPath: vaultPath,
		items:     items,
		freeGB:    freeGB,
		theme:     tui.DefaultTheme(),
		keys:      tui.DefaultKeyMap(),
	}
}

func (m reviewModel) Init() tea.Cmd { return nil }

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch {
		case m.keys.Enter.Matches(msg):
			return m, func() tea.Msg { return reviewConfirmMsg{} }
		case m.keys.Back.Matches(msg):
			return m, func() tea.Msg { return reviewBackMsg{} }
		case m.keys.MoveDown.Matches(msg):
			maxVisible := m.height - 10
			if m.scrollOffset < len(m.items)-maxVisible {
				m.scrollOffset++
			}
		case m.keys.MoveUp.Matches(msg):
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		}
	}
	return m, nil
}

func (m reviewModel) View() string {
	var b strings.Builder
	totalGB := 0.0
	for _, item := range m.items {
		totalGB += item.SizeGB
	}

	b.WriteString(fmt.Sprintf("  Target: %s\n", m.vaultPath))
	b.WriteString(fmt.Sprintf("  Items:  %d sources, %.1f GB / %.0f GB free\n\n", len(m.items), totalGB, m.freeGB))

	// Group by type
	byType := make(map[string][]ReviewItem)
	for _, item := range m.items {
		byType[item.Type] = append(byType[item.Type], item)
	}

	maxVisible := m.height - 10
	if maxVisible < 5 {
		maxVisible = 5
	}
	lineIdx := 0
	for _, typeName := range sortedKeys(byType) {
		items := byType[typeName]
		for _, item := range items {
			if lineIdx >= m.scrollOffset && lineIdx < m.scrollOffset+maxVisible {
				sizeStr := formatSizeGB(item.SizeGB)
				desc := item.Description
				if desc == "" {
					desc = item.ID
				}
				b.WriteString(fmt.Sprintf("  %-20s %-8s %8s  %s\n", item.ID, strings.ToUpper(item.Type), sizeStr, desc))
			}
			lineIdx++
		}
	}

	b.WriteString("\n  Enter to confirm  |  Esc to go back\n")
	return b.String()
}

func sortedKeys(m map[string][]ReviewItem) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// sort inline
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
```

**Step 4: Run tests**

Run: `cd host-tui && go test ./internal/wizard/ -run TestReview -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add host-tui/internal/wizard/review.go host-tui/internal/wizard/review_test.go
git commit -m "feat(wizard): add Review sub-model with item summary"
```

---

## Task 8: Refactor Wizard Shell — Orchestrate Sub-Models

Replace the current skeleton wizard model with a real orchestrator that creates sub-models per stage, routes messages, and accumulates state.

**Files:**
- Modify: `host-tui/internal/wizard/model.go` (major rewrite)
- Modify: `host-tui/internal/wizard/view.go` (major rewrite)
- Modify: `host-tui/internal/wizard/model_test.go` (update tests)

**Step 1: Rewrite model.go**

```go
package wizard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkronstrom/svalbard/tui"
)

var wizardSteps = []struct{ id, label string }{
	{"path", "Vault Path"},
	{"preset", "Choose Preset"},
	{"packs", "Pack Picker"},
	{"review", "Review"},
}

// BackMsg is sent when the user navigates back from the first wizard step.
type BackMsg struct{}

// stage tracks which sub-model is active.
type stage int

const (
	stagePath stage = iota
	stagePreset
	stagePacks
	stageReview
)

// Model is the Bubble Tea model for the init wizard.
type Model struct {
	config      WizardConfig
	stage       stage
	width       int
	height      int
	theme       tui.Theme
	keys        tui.KeyMap

	// Sub-models (created lazily)
	pathPicker   pathPickerModel
	presetPicker presetPickerModel
	packPicker   packPickerModel
	review       reviewModel

	// Accumulated state
	vaultPath   string
	freeGB      float64
	checkedIDs  map[string]bool
	presetName  string
}

// New creates a new wizard Model with the given config.
func New(config WizardConfig) Model {
	m := Model{
		config:     config,
		stage:      stagePath,
		theme:      tui.DefaultTheme(),
		keys:       tui.DefaultKeyMap(),
		checkedIDs: make(map[string]bool),
	}

	if config.StartAtStep > 0 && config.StartAtStep <= int(stageReview) {
		m.stage = stage(config.StartAtStep)
	}

	// Initialize path picker
	m.pathPicker = newPathPicker(config.Volumes, config.HomeVolume, config.PrefillPath)

	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to active sub-model
		switch m.stage {
		case stagePath:
			updated, cmd := m.pathPicker.Update(msg)
			m.pathPicker = updated.(pathPickerModel)
			return m, cmd
		case stagePreset:
			updated, cmd := m.presetPicker.Update(msg)
			m.presetPicker = updated.(presetPickerModel)
			return m, cmd
		case stagePacks:
			updated, cmd := m.packPicker.Update(msg)
			m.packPicker = updated.(packPickerModel)
			return m, cmd
		case stageReview:
			updated, cmd := m.review.Update(msg)
			m.review = updated.(reviewModel)
			return m, cmd
		}
		return m, nil

	// Sub-model done messages
	case pathDoneMsg:
		m.vaultPath = msg.path
		m.freeGB = msg.freeGB
		m.stage = stagePreset
		m.presetPicker = newPresetPicker(m.config.Presets, m.config.Regions, m.freeGB)
		return m, nil

	case presetDoneMsg:
		m.presetName = msg.preset.Name
		m.checkedIDs = make(map[string]bool)
		for _, id := range msg.preset.SourceIDs {
			m.checkedIDs[id] = true
		}
		m.stage = stagePacks
		m.packPicker = newPackPicker(m.config.PackGroups, m.checkedIDs, m.freeGB)
		return m, nil

	case packDoneMsg:
		m.checkedIDs = msg.selectedIDs
		m.stage = stageReview
		m.review = newReviewModel(m.vaultPath, m.buildReviewItems(), m.freeGB)
		return m, nil

	case packCancelMsg:
		// Go back to preset picker
		m.stage = stagePreset
		m.presetPicker = newPresetPicker(m.config.Presets, m.config.Regions, m.freeGB)
		return m, nil

	case reviewConfirmMsg:
		return m, func() tea.Msg {
			return DoneMsg{Result: WizardResult{
				VaultPath:   m.vaultPath,
				SelectedIDs: m.selectedIDList(),
				PresetName:  m.presetName,
			}}
		}

	case reviewBackMsg:
		m.stage = stagePacks
		m.packPicker = newPackPicker(m.config.PackGroups, m.checkedIDs, m.freeGB)
		return m, nil

	case tea.KeyMsg:
		// Handle force quit and back at top level
		if m.keys.ForceQuit.Matches(msg) {
			return m, tea.Quit
		}
		// Back from path picker goes to welcome
		if m.stage == stagePath && m.keys.Back.Matches(msg) && !m.pathPicker.customInput {
			return m, func() tea.Msg { return BackMsg{} }
		}
	}

	// Forward to active sub-model
	switch m.stage {
	case stagePath:
		updated, cmd := m.pathPicker.Update(msg)
		m.pathPicker = updated.(pathPickerModel)
		return m, cmd
	case stagePreset:
		updated, cmd := m.presetPicker.Update(msg)
		m.presetPicker = updated.(presetPickerModel)
		return m, cmd
	case stagePacks:
		updated, cmd := m.packPicker.Update(msg)
		m.packPicker = updated.(packPickerModel)
		return m, cmd
	case stageReview:
		updated, cmd := m.review.Update(msg)
		m.review = updated.(reviewModel)
		return m, cmd
	}

	return m, nil
}

func (m Model) buildReviewItems() []ReviewItem {
	// Build from pack groups — find source details for each checked ID
	var items []ReviewItem
	seen := make(map[string]bool)
	for _, g := range m.config.PackGroups {
		for _, p := range g.Packs {
			for _, src := range p.Sources {
				if m.checkedIDs[src.ID] && !seen[src.ID] {
					seen[src.ID] = true
					items = append(items, ReviewItem{
						ID:          src.ID,
						SizeGB:      src.SizeGB,
						Description: src.Description,
					})
				}
			}
		}
	}
	return items
}

func (m Model) selectedIDList() []string {
	ids := make([]string, 0, len(m.checkedIDs))
	for id := range m.checkedIDs {
		ids = append(ids, id)
	}
	return ids
}
```

**Step 2: Rewrite view.go**

```go
package wizard

import "github.com/pkronstrom/svalbard/tui"

func (m Model) View() string {
	// Build step navigation list (left pane)
	items := make([]tui.NavItem, len(wizardSteps))
	for i, step := range wizardSteps {
		items[i] = tui.NavItem{
			ID:    step.id,
			Label: step.label,
		}
	}

	nav := tui.NavList{
		Items:    items,
		Selected: int(m.stage),
		Theme:    m.theme,
	}

	// Right pane = active sub-model's view
	var right string
	switch m.stage {
	case stagePath:
		right = m.pathPicker.View()
	case stagePreset:
		right = m.presetPicker.View()
	case stagePacks:
		right = m.packPicker.View()
	case stageReview:
		right = m.review.View()
	}

	footer := tui.FooterHints(m.keys.Enter, m.keys.Back)

	shell := tui.ShellLayout{
		Theme:   m.theme,
		AppName: "Svalbard Init",
		Left:    nav.Render(),
		Right:   right,
		Footer:  footer,
		Width:   m.width,
		Height:  m.height,
	}

	return shell.Render()
}
```

**Step 3: Update tests**

Rewrite `model_test.go` to test the new orchestrator:

```go
package wizard

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func stripAnsi(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func testConfig() WizardConfig {
	return WizardConfig{
		Volumes: []Volume{
			{Path: "/Volumes/USB/svalbard", Name: "USB", TotalGB: 64, FreeGB: 50},
		},
		HomeVolume: Volume{Path: "/Users/test/svalbard", Name: "~/svalbard/", FreeGB: 100},
		Presets: []PresetOption{
			{Name: "default-2", Description: "Bugout kit", ContentGB: 1.5, TargetSizeGB: 2, SourceIDs: []string{"kiwix-serve"}},
		},
		Regions:    []string{"default"},
		PackGroups: samplePackGroups(),
	}
}

func TestWizardShowsAllSteps(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 24

	out := stripAnsi(m.View())
	for _, step := range wizardSteps {
		if !strings.Contains(out, step.label) {
			t.Errorf("View() should contain step label %q", step.label)
		}
	}
}

func TestWizardStartsAtPathStep(t *testing.T) {
	m := New(testConfig())
	if m.stage != stagePath {
		t.Errorf("expected stagePath, got %d", m.stage)
	}
}

func TestWizardPathToPresetTransition(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 24

	// Select first volume (enter on cursor=0)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected path done command")
	}
	// Execute the command to get the message
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.stage != stagePreset {
		t.Errorf("expected stagePreset after path selection, got %d", m.stage)
	}
	if m.vaultPath == "" {
		t.Error("vault path should be set")
	}
}

func TestWizardEscAtPathGoesBack(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 24

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected BackMsg command")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Errorf("expected BackMsg, got %T", msg)
	}
}

func TestWizardFullFlow(t *testing.T) {
	m := New(testConfig())
	m.width = 80
	m.height = 40

	// Step 1: Select path
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.stage != stagePreset {
		t.Fatalf("expected stagePreset, got %d", m.stage)
	}

	// Step 2: Select preset
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg = cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if m.stage != stagePacks {
		t.Fatalf("expected stagePacks, got %d", m.stage)
	}

	// Step 3: Apply pack selection (press 'a')
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msg = cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if m.stage != stageReview {
		t.Fatalf("expected stageReview, got %d", m.stage)
	}

	// Step 4: Confirm review
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg = cmd()
	done, ok := msg.(DoneMsg)
	if !ok {
		t.Fatalf("expected DoneMsg, got %T", msg)
	}
	if done.Result.VaultPath == "" {
		t.Error("DoneMsg should have vault path")
	}
}
```

**Step 4: Run all wizard tests**

Run: `cd host-tui && go test ./internal/wizard/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add host-tui/internal/wizard/
git commit -m "feat(wizard): refactor shell to orchestrate interactive sub-models"
```

---

## Task 9: Update launch.go and Wire Host-CLI Data

Update `launch.go` to accept `WizardConfig` and update `host-cli` to prepare config from catalog + volumes.

**Files:**
- Modify: `host-tui/launch.go`
- Modify: `host-cli/internal/cli/root.go` (or wherever TUI is launched)

**Step 1: Update launch.go**

Change `RunInitWizard` to accept `WizardConfig`:

```go
// RunInitWizard launches the init wizard TUI with full config.
func RunInitWizard(config wizard.WizardConfig) error {
	return runApp(&appModel{screen: screenWizard, wizard: wizard.New(config)})
}
```

Update `appModel.Update` to handle `wizard.DoneMsg`:

```go
case wizard.DoneMsg:
	// Wizard completed — exit TUI and return result
	// The caller (host-cli) handles init + apply
	return m, tea.Quit
```

**Step 2: Update host-cli TUI launch to build WizardConfig**

In `host-cli/internal/cli/root.go` (or the file that calls `hosttui.RunInitWizard`), add config preparation:

```go
func buildWizardConfig(cat *catalog.Catalog, prefillPath string) wizard.WizardConfig {
	vols := volumes.Detect()
	home := volumes.HomeSvalbardVolume()

	// Build preset options for default region
	var presetOpts []wizard.PresetOption
	for _, p := range cat.PresetsForRegion("default") {
		resolved, err := cat.ResolvePreset(p.Name)
		if err != nil {
			continue
		}
		presetOpts = append(presetOpts, wizard.PresetOption{
			Name:         p.Name,
			Description:  p.Description,
			ContentGB:    resolved.ContentSizeGB(),
			TargetSizeGB: p.TargetSizeGB,
			Region:       p.Region,
			SourceIDs:    resolved.Sources,
		})
	}

	// Build pack groups
	var packGroups []wizard.PackGroup
	groupMap := make(map[string]*wizard.PackGroup)
	for _, p := range cat.Packs() {
		resolved, err := cat.ResolvePreset(p.Name)
		if err != nil {
			continue
		}
		pg, ok := groupMap[p.DisplayGroup]
		if !ok {
			pg = &wizard.PackGroup{Name: p.DisplayGroup}
			groupMap[p.DisplayGroup] = pg
		}
		pack := wizard.Pack{
			Name:        p.Name,
			Description: p.Description,
		}
		for _, item := range resolved.Items {
			pack.Sources = append(pack.Sources, wizard.PackSource{
				ID:          item.ID,
				Description: item.Description,
				SizeGB:      item.SizeGB,
			})
		}
		pg.Packs = append(pg.Packs, pack)
	}
	for _, pg := range groupMap {
		packGroups = append(packGroups, *pg)
	}
	// Sort groups by name
	sort.Slice(packGroups, func(i, j int) bool {
		return packGroups[i].Name < packGroups[j].Name
	})

	// Convert volumes
	var wizVols []wizard.Volume
	for _, v := range vols {
		wizVols = append(wizVols, wizard.Volume{
			Path: v.Path, Name: v.Name,
			TotalGB: v.TotalGB, FreeGB: v.FreeGB,
			Network: v.Network,
		})
	}

	return wizard.WizardConfig{
		Volumes:     wizVols,
		HomeVolume:  wizard.Volume{Path: home.Path, Name: home.Name, FreeGB: home.FreeGB},
		Presets:     presetOpts,
		Regions:     cat.Regions(),
		PackGroups:  packGroups,
		PrefillPath: prefillPath,
	}
}
```

**Step 3: Run full test suite**

Run: `cd host-tui && go test ./... && cd ../host-cli && go test ./...`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add host-tui/launch.go host-cli/internal/cli/root.go
git commit -m "feat: wire wizard config from catalog to TUI"
```

---

## Task 10: Integration Test and Polish

Manual integration test + fix any rough edges.

**Step 1: Build and run**

```bash
cd host-cli && go install ./cmd/svalbard/
svalbard  # should launch TUI → welcome → wizard
```

**Step 2: Test the full wizard flow**

1. Launch `svalbard` (no args) → should show welcome screen
2. Select "Init Vault" → should show path picker with detected volumes
3. Pick a path → should show preset picker with sizes
4. Pick a preset → should show pack picker with pre-checked items
5. Toggle some packs, observe tri-state checkboxes and size total
6. Press 'a' → should show review with all selected items
7. Press Enter → TUI should exit

**Step 3: Fix any issues found during manual testing**

**Step 4: Run full test suite one more time**

```bash
cd host-tui && go test ./... -v
cd ../host-cli && go test ./... -v
```

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix(wizard): polish integration issues"
```

---

## Dependency Graph

```
Task 1 (Catalog extension)  ──────────────────┐
                                                ├── Task 9 (Wire-up)
Task 2 (Volume detection)   ──────────────────┤
                                                │
Task 3 (Wizard data types)  ──┬── Task 4 (Path)──┤
                              ├── Task 5 (Preset)─┤
                              ├── Task 6 (Pack) ──┼── Task 8 (Shell) ── Task 9
                              └── Task 7 (Review)─┘
```

**Parallelizable:**
- Tasks 1, 2, 3 can run in parallel (independent)
- Tasks 4, 5, 6, 7 can run in parallel (independent sub-models, all depend on Task 3)
- Task 8 depends on 4-7
- Task 9 depends on 1, 2, 8
- Task 10 depends on 9

## Notes for Implementer

- The `host-tui` module does NOT import `host-cli`. Data flows via `WizardConfig` struct.
- `golang.org/x/sys/unix` is needed for `Statfs` in volume detection — add to `host-cli/go.mod`.
- The existing wizard test `TestWizardAdvancesOnEnter` tests the OLD behavior. Task 8 rewrites all tests.
- Pack picker uses `readchar` in Python but Bubble Tea handles keyboard natively — no extra dependency.
- The `min`/`max` builtins exist in Go 1.21+ so the helper functions in packpicker.go can be removed if using Go 1.25.
