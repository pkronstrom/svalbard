package wizard

import (
	"context"
	"github.com/pkronstrom/svalbard/tui"
)

// Volume is a detected storage mount point.
type Volume struct {
	Path    string // e.g. "/Volumes/KINGSTON/svalbard"
	Name    string // e.g. "KINGSTON"
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
	Custom       bool     // user-defined preset (from the user preset dir)
}

// Type aliases — pack types now live in tui/ as the shared tree picker data model.
type (
	PackGroup  = tui.PackGroup
	Pack       = tui.Pack
	PackSource = tui.PackSource
)

// DepResolver returns known auto-dep IDs and missing catalog IDs for a given
// explicit selection.
type DepResolver func(selectedIDs map[string]bool) (autoDeps map[string]bool, missingIDs []string)

// ApplyEvent reports progress of a single item during apply.
type ApplyEvent struct {
	ID         string
	Status     string // tui.Status* constants
	Step       string // current build step
	Message    string // a message about the vault as a whole when ID is no item
	Downloaded int64
	Total      int64
	Error      string
}

// ApplyFunc runs vault apply with progress reporting.
type ApplyFunc func(ctx context.Context, vaultPath string, onProgress func(ApplyEvent)) error

// InitFunc initializes a new vault.
type InitFunc func(path string, items []string, presetName, region string, platforms []string) error

// WizardConfig is everything the wizard needs to run.
// Prepared by host-cli, consumed by host-tui.
type WizardConfig struct {
	Volumes     []Volume
	HomeVolume  Volume
	Presets     []PresetOption // for the selected region
	Regions     []string       // available regions
	PackGroups  []PackGroup    // all packs grouped
	PrefillPath string
	// FreeSpace measures the free space in GB at a path the user typed, or
	// at its nearest existing parent; nil leaves a typed path unmeasured.
	FreeSpace   func(path string) float64
	StartAtStep int
	InitVault   InitFunc                    // called after review confirm
	ResolveDeps DepResolver                 // nil = no dep resolution
	RunApply    ApplyFunc                   // called to download content
	RunIndex    IndexFunc                   // called to build search index after apply
	ShouldIndex func(vaultPath string) bool // nil means automatic indexing is enabled
	// ForPlatforms re-lists Presets and PackGroups with sizes estimated for
	// the chosen target platforms; nil keeps the sizes above.
	ForPlatforms func(platforms []string) ([]PresetOption, []PackGroup)
	// StorageNote is a warning about the target disk for the selection, such
	// as what its large clusters cost, shown on the review screen; nil or ""
	// shows none.
	StorageNote func(path string, items []string, platforms []string) string
}

// WizardResult is returned when the wizard completes.
type WizardResult struct {
	VaultPath     string
	SelectedIDs   []string
	PresetName    string // empty if custom
	Region        string
	HostPlatforms []string // e.g. ["macos-arm64", "linux-arm64"]
}

// DoneMsg is emitted by the wizard when it completes.
type DoneMsg struct {
	Result WizardResult
}
