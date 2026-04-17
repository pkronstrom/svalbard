# TUI Dedup + Wizard Indexing Stage

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate duplicate progress/status rendering across plan, wizard, and index screens by extending the existing `tui.ProgressView`. Then add an indexing stage to the wizard as the final step after apply.

**Architecture:** The unused `tui.ProgressView` becomes the single source of truth for rendering step lists with status icons, download progress, and error display. All three apply/progress views delegate to it. The wizard gains a `stageIndex` after `stageApply` that runs keyword indexing on the newly populated vault.

**Tech Stack:** Existing Bubble Tea + tui/ package. No new dependencies.

---

### Task 1: Extend `tui.ProgressView` to handle download progress

**Files:**
- Modify: `tui/progress.go`

The existing `ProgressView` uses `StepStatus` (int enum) and `[done]`-style icons. Upgrade it to:

1. Add `Downloaded`, `Total int64`, and `Step string` fields to `ProgressStep`
2. Replace `[done]`/`[FAIL]` with `✓`/`✗`/`↓` symbols (matching current inline rendering)
3. Show byte progress for active items: `↓  item-id  2.1/5.3 GB  39%`
4. Show step name for build items: `↓  item-id  wget`
5. Truncate inline errors to ~60 chars
6. Add `RenderSummary(theme)` that returns `"3/11 done  2 active  1 failed"` line
7. Add scrolling support (MaxVisible, ScrollOffset, Cursor) so plan/wizard don't reimplement it
8. Use string status constants (`tui.StatusDone` etc.) instead of the int enum, to match what apply produces

---

### Task 2: Rewrite plan's `viewApply()` to use `tui.ProgressView`

**Files:**
- Modify: `host-tui/internal/plan/model.go`

Replace the hand-rolled `viewApply()` (icon switch, byte formatting, scroll, summary) with:
- Store a `tui.ProgressView` on the model (or build it in View from `applyItems`)
- Map `applyStep` → `tui.ProgressStep` in the view function
- Use `ProgressView.Render()` for the body
- Use `ProgressView.RenderSummary()` for the summary line
- Delete the inline `applyStep` type if ProgressStep covers all fields

---

### Task 3: Rewrite wizard's `applymodel.go` to use `tui.ProgressView`

**Files:**
- Modify: `host-tui/internal/wizard/applymodel.go`

Same approach as Task 2. The wizard apply view is nearly identical to plan's — replace with ProgressView. Delete `wizFormatBytes` (already done in simplify pass).

---

### Task 4: Rewrite index's rebuild view to use `tui.ProgressView`

**Files:**
- Modify: `host-tui/internal/index/model.go`

The index rebuild view at `viewRebuilding()` has its own status icon switch. Replace with ProgressView. Index has extra statuses (`StatusIndexing`, `StatusSkip`) — ProgressView should handle these via the string-based status approach.

---

### Task 5: Add `stageIndex` to the wizard

**Files:**
- Modify: `host-tui/internal/wizard/model.go` (add stage, transitions)
- Create: `host-tui/internal/wizard/indexmodel.go` (new sub-model)
- Modify: `host-tui/internal/wizard/types.go` (add RunIndex to WizardConfig)
- Modify: `host-tui/wizardtypes.go` (re-export if needed)
- Modify: `host-tui/launch.go` (wire RunIndex callback)
- Modify: `host-cli/internal/cli/root.go` (wire RunIndex from deps)

Add `stageIndex` after `stageApply`:

```go
var wizardSteps = []struct{ id, label string }{
    {"path", "Vault Path"},
    {"platforms", "Platforms"},
    {"preset", "Choose Preset"},
    {"packs", "Pack Picker"},
    {"review", "Review"},
    {"apply", "Apply"},
    {"index", "Build Index"},
}
```

The `indexmodel` sub-model:
- Runs keyword (FTS5) indexing automatically after apply completes
- Shows progress using `tui.ProgressView`
- Emits `wizardIndexDoneMsg` when complete
- On done → wizard emits `DoneMsg` with result

Wire the `RunIndex` callback from `DashboardDeps` through `WizardConfig` so the wizard can trigger indexing without importing host-cli.

---

### Task 6: Tests and verify

- Run `go test ./...` across tui, host-tui, host-cli
- Verify: plan apply view renders identically
- Verify: wizard apply view renders identically
- Verify: index rebuild view renders identically
- Verify: wizard completes with index stage
- Commit
