package tui

// Status constants used by apply, index, and progress screens.
// Producers (host-cli/apply, host-cli/root) and consumers (plan, wizard,
// index TUI models) must use these instead of bare string literals.
const (
	StatusActive   = "active"
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusQueued   = "queued"
	StatusIndexing = "indexing"
	StatusSkip     = "skip"
)
