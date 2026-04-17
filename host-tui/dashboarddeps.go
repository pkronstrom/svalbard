package hosttui

import "context"

// DashboardDeps holds callback functions that dashboard screens use to
// interact with CLI business logic. Built by host-cli, passed to RunInteractive.
// This bridges the module boundary (host-tui cannot import host-cli).
type DashboardDeps struct {
	// Read-only queries
	LoadStatus       func() (VaultStatus, error)
	LoadDesiredItems func() ([]string, error)
	LoadPlan         func() (PlanSummary, error)
	LoadIndexStatus  func() (IndexStatus, error)

	// State mutation
	SaveDesiredItems func(ids []string) error
	InitVault        func(path string, items []string, presetName, region string, platforms []string) error

	// Long-running operations (run in goroutines, report progress via callback)
	RunApply  func(ctx context.Context, onProgress func(ApplyEvent)) error
	RunImport func(ctx context.Context, source string) (ImportResult, error)
	RunIndex  func(ctx context.Context, indexType string, onProgress func(IndexEvent)) error

	// Static catalog data (for Browse)
	PackGroups []PackGroup
	Presets    []PresetOption

	// RebuildForVault returns a new DashboardDeps targeting a different vault path.
	// Used when the user opens a different vault via the Open Vault screen.
	RebuildForVault func(vaultPath string) *DashboardDeps
}

// VaultStatus summarizes the current vault state for the Status right-pane preview.
type VaultStatus struct {
	VaultPath     string
	VaultName     string
	PresetName    string
	DesiredCount  int
	RealizedCount int
	PendingCount  int
	DiskUsedGB    float64
	DiskFreeGB    float64
	LastApplied   string // RFC3339 timestamp or empty
}

// PlanSummary describes pending reconciliation changes for the Plan screen.
type PlanSummary struct {
	ToDownload  []PlanItem
	ToRemove    []PlanItem
	DownloadGB  float64
	RemoveGB    float64
	FreeAfterGB float64
}

// PlanItem is a single entry in a reconciliation plan.
type PlanItem struct {
	ID          string
	Type        string
	SizeGB      float64
	Description string
	Action      string // "download" or "remove"
}

// ApplyEvent reports progress of a single item during apply.
type ApplyEvent struct {
	ID     string
	Status string // tui.StatusQueued, tui.StatusActive, tui.StatusDone, tui.StatusFailed
	Error  string
}

// ImportResult is returned after a successful import.
type ImportResult struct {
	ID     string
	SizeGB float64
}

// IndexStatus describes the state of search indexes.
type IndexStatus struct {
	KeywordEnabled   bool
	KeywordSources   int64
	KeywordArticles  int64
	KeywordLastBuilt string
	SemanticEnabled  bool
	SemanticStatus   string // e.g. "model not installed"
}

// IndexEvent reports progress during index rebuild.
type IndexEvent struct {
	File   string
	Status string // tui.StatusIndexing, tui.StatusSkip, tui.StatusDone, tui.StatusFailed
}
