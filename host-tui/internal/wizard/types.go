package wizard

import "github.com/pkronstrom/svalbard/tui"

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

// Type aliases — pack types now live in tui/ as the shared tree picker data model.
type (
	PackGroup  = tui.PackGroup
	Pack       = tui.Pack
	PackSource = tui.PackSource
)

// ApplyEvent reports progress of a single item during apply.
type ApplyEvent struct {
	ID         string
	Status     string // tui.Status* constants
	Step       string // current build step
	Downloaded int64
	Total      int64
	Error      string
}

// ApplyFunc runs vault apply with progress reporting.
type ApplyFunc func(vaultPath string, onProgress func(ApplyEvent)) error

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
	StartAtStep int
	InitVault   InitFunc  // called after review confirm
	RunApply    ApplyFunc // called to download content
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
