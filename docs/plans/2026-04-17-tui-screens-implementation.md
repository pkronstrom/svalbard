# TUI Screens Implementation

## Context

The TUI menu redesign (committed in `6b5d64d`) created a 6-item dashboard menu (Status, Browse, Plan, Import, Index, New Vault) and a 3-item welcome screen (New Vault, Open Vault, Browse). Only "New Vault" is wired — the rest show "not yet implemented." This plan implements all remaining screens.

## Architecture: Callback Bridge

**Problem**: host-tui cannot import host-cli (dependency is one-way: host-cli → host-tui). TUI screens need to read manifests, run apply, import files, and rebuild indexes — all CLI-side logic.

**Solution**: The CLI builds closures and passes them to the TUI via a `DashboardDeps` struct. This extends the existing pattern where `WizardConfig` carries static data from CLI to TUI.

```
host-cli/root.go
  └─ buildDashboardDeps()     ← builds closures from CLI packages
       └─ DashboardDeps       ← defined in host-tui
            └─ passed to RunInteractive()
                 └─ dashboard/browse/plan/etc. call closures via tea.Cmd
```

### DashboardDeps (defined in host-tui/dashboarddeps.go)

```go
type DashboardDeps struct {
    // Read-only queries (called synchronously or in tea.Cmd)
    LoadStatus      func() (VaultStatus, error)
    LoadPlan        func() (PlanSummary, error)
    LoadIndexStatus func() (IndexStatus, error)

    // State mutation
    SaveDesiredItems func(ids []string) error

    // Long-running operations (run in goroutines, report progress via callback)
    RunApply  func(ctx context.Context, onProgress func(ApplyEvent)) error
    RunImport func(ctx context.Context, source string) (ImportResult, error)
    RunIndex  func(ctx context.Context, indexType string, onProgress func(IndexEvent)) error

    // Static catalog data (for Browse)
    PackGroups []PackGroup
    Presets    []PresetOption
}
```

### Bridge Value Types

```go
type VaultStatus struct {
    VaultPath     string
    VaultName     string
    PresetName    string
    DesiredCount  int
    RealizedCount int
    PendingCount  int
    DiskUsedGB    float64
    DiskFreeGB    float64
    LastApplied   string
}

type PlanSummary struct {
    ToDownload  []PlanItem
    ToRemove    []PlanItem
    DownloadGB  float64
    RemoveGB    float64
    FreeAfterGB float64
}

type PlanItem struct {
    ID     string
    SizeGB float64
    Action string // "download", "remove"
}

type ApplyEvent struct {
    ID     string
    Status string // "queued", "active", "done", "failed"
    Error  string
}

type ImportResult struct {
    ID     string
    SizeGB float64
}

type IndexStatus struct {
    KeywordEnabled   bool
    KeywordSources   int64
    KeywordArticles  int64
    KeywordLastBuilt string
    SemanticEnabled  bool
    SemanticStatus   string
}

type IndexEvent struct {
    File   string
    Status string // "indexing", "skip", "done", "failed"
}
```

## Screens

### Browse (host-tui/internal/browse/)

Full-screen pack tree with checkboxes. Adapted from wizard's packpicker (`host-tui/internal/wizard/packpicker.go`).

**Copy from packpicker**: `pickerRow`, `rebuildRows`, `toggleAtCursor`, `expandCollapseAtCursor`, `ensureVisible`, View rendering logic. Modify for Browse-specific behavior.

**Model fields**: tree state (groups, checkedIDs, collapsed, rows, cursor, scroll), `deps *DashboardDeps`, `readOnly bool`, `initialIDs` (snapshot for dirty check), save prompt state.

**Key behavior**:
- Space: toggle item/group
- Enter: expand/collapse
- p: open preset picker (updates checkboxes to preset's items)
- Esc/q: if dirty → save prompt (y=save, n=discard, esc=stay), else → back
- Init loads current desired items from `deps.LoadStatus()` as initial checkboxes

**Messages emitted**: `BackMsg`, `SavedMsg`

### Plan (host-tui/internal/plan/)

Full-screen diff view of pending changes. Apply is a sub-state within Plan (not a separate screen).

**Plan view**:
```
  + wikipedia_en.zim          8.2 GB   download
  - gutenberg_es.zim          1.4 GB   remove
  
  Download: 8.2 GB  Remove: 1.4 GB  Free after: 72.9 GB
```

**Apply sub-state** (within Plan model):
```
  ✓  gutenberg_es.zim          removed
  ·  wikipedia_en.zim          downloading
     osm_nordic.pmtiles        queued
```

**Key behavior**:
- Enter: start Apply (switch to apply sub-state, run `deps.RunApply`)
- b: emit BrowseMsg (jump to Browse)
- Esc: emit BackMsg (return to dashboard)
- During apply: no input except Ctrl+C

**Async pattern**: Channel + Cmd chaining (see below).

**Messages emitted**: `BackMsg`, `BrowseMsg`

### Import (host-tui/internal/importscreen/)

Text input for path/URL. One-shot import, stays on screen for more.

```
  Path or URL: /home/user/guide.pdf

  ✓ Imported local:guide (12.4 MB)

  Path or URL: _
```

**Key behavior**:
- Enter: fire `deps.RunImport` in tea.Cmd
- Esc: back to dashboard
- Rune input / backspace: standard text editing

**Messages emitted**: `BackMsg`

### Index (host-tui/internal/index/)

Two-item master-detail within the screen. Uses tui.NavList for the 2-item list, tui.DetailPane for right-side info.

```
  1  Keyword search     fast exact matching              enabled
  2  Semantic search    meaning-based, heavier           disabled
```

**Key behavior**:
- Enter: rebuild selected index (or enable semantic)
- Esc: back to dashboard
- During rebuild: shows progress (same channel+tick pattern as Apply)

**Messages emitted**: `BackMsg`

### Open Vault (host-tui/internal/openvault/)

Text input for vault path. Validates manifest.yaml exists.

```
  Path: /mnt/usb/vault_

  (error: no vault found at this path)
```

**Key behavior**:
- Enter: validate path, emit DoneMsg if valid
- Esc: emit BackMsg

**Messages emitted**: `DoneMsg{Path string}`, `BackMsg`

### Status (right-pane only — no new package)

Live data in dashboard's right pane when Status is selected. Loaded from `deps.LoadStatus()` on dashboard init.

```
  Vault:     /mnt/usb/vault
  Preset:    nordic-64
  
  Desired:   15 items
  Realized:  12 items
  Pending:    3 items
  
  Disk:      45.2 GB used / 82.8 GB free
  Last apply: 2 hours ago
```

## Async Operations Pattern

Bubble Tea is single-threaded. Long-running operations use channel + Cmd chaining:

```go
// 1. Start: run in goroutine, return channel as message
func startApply(deps) tea.Cmd {
    return func() tea.Msg {
        ch := make(chan ApplyEvent, 16)
        go func() {
            defer close(ch)
            deps.RunApply(ctx, func(ev ApplyEvent) { ch <- ev })
        }()
        return applyStartedMsg{ch: ch}
    }
}

// 2. Tick: block on channel, return next event
func waitForEvent(ch <-chan ApplyEvent) tea.Cmd {
    return func() tea.Msg {
        ev, ok := <-ch
        return applyTickMsg{event: ev, done: !ok}
    }
}

// 3. Model: on each tick, update display, fire next waitForEvent
```

## Screen Transitions (appModel in launch.go)

New screen constants: `screenBrowse`, `screenPlan`, `screenImport`, `screenIndex`, `screenOpenVault`.

| Source Message | Transition |
|---|---|
| `dashboard.SelectMsg{ID: "browse"}` | → screenBrowse (create browse.New(deps)) |
| `dashboard.SelectMsg{ID: "plan"}` | → screenPlan (create plan.New(deps)) |
| `dashboard.SelectMsg{ID: "import"}` | → screenImport |
| `dashboard.SelectMsg{ID: "index"}` | → screenIndex |
| `welcome.SelectMsg{ID: "open-vault"}` | → screenOpenVault |
| `welcome.SelectMsg{ID: "browse"}` | → screenBrowse (readOnly, nil deps) |
| `browse.BackMsg` / `browse.SavedMsg` | → screenDashboard |
| `plan.BackMsg` | → screenDashboard |
| `plan.BrowseMsg` | → screenBrowse |
| `importscreen.BackMsg` | → screenDashboard |
| `index.BackMsg` | → screenDashboard |
| `openvault.DoneMsg{Path}` | → screenDashboard (rebuild with new vault) |
| `openvault.BackMsg` | → screenWelcome |

## Files to Create

| File | Purpose |
|---|---|
| `host-tui/dashboarddeps.go` | DashboardDeps struct + all bridge value types |
| `host-tui/internal/browse/model.go` | Browse screen (adapted packpicker) |
| `host-tui/internal/plan/model.go` | Plan + Apply sub-state |
| `host-tui/internal/importscreen/model.go` | Import screen |
| `host-tui/internal/index/model.go` | Index screen |
| `host-tui/internal/openvault/model.go` | Open Vault screen |
| Test files for each |

## Files to Modify

| File | Changes |
|---|---|
| `host-tui/launch.go` | New screens, routing, DashboardDeps in appModel, update RunInteractive signature |
| `host-tui/internal/dashboard/model.go` | Remove SelectMsg self-handling (let appModel route), accept deps |
| `host-tui/internal/dashboard/context.go` | Use VaultStatus for live right-pane data |
| `host-tui/internal/welcome/model.go` | Emit SelectMsg for all items (not just new-vault) |
| `host-cli/internal/cli/root.go` | Add buildDashboardDeps(), pass to RunInteractive |
| `host-cli/internal/apply/apply.go` | Add optional progress callback parameter |

## Implementation Order

1. **Foundation**: dashboarddeps.go, update RunInteractive signature, buildDashboardDeps in root.go
2. **Open Vault**: Simplest screen, no deps. Pure text input + path validation.
3. **Live Status**: Wire LoadStatus into dashboard right pane
4. **Browse**: Copy+adapt packpicker, preset picker, save prompt, dirty tracking
5. **Plan + Apply**: Plan view, apply sub-state with progress channel
6. **Import**: Text input, one-shot async
7. **Index**: Status display, rebuild with progress
8. **Wire all transitions** in launch.go

## Verification

1. `cd tui && go test ./...`
2. `cd host-tui && go test ./...`
3. `cd host-cli && go build ./...`
4. `cd drive-runtime && go build ./...`
5. Manual: `svalbard` in vault dir → navigate each screen
