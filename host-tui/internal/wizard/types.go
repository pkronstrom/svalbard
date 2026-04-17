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
	Type        string // e.g. "zim", "binary", "pmtiles"
	Description string
	SizeGB      float64
}

// ApplyFunc runs vault apply with progress reporting.
type ApplyFunc func(vaultPath string, onProgress func(id, status string)) error

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
