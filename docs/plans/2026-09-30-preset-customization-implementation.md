# Preset customization through the TUI — implementation plan

Status: done 2026-10-05; final review findings fixed, targeted fix review clear, verification passed. See [checkpoint](2026-10-05-preset-customization-checkpoint.md).

## Goal

Repair the init wizard so a user can start from a built-in or local preset, customize its explicit recipe selection, review the dependency-expanded result and estimated size, and save exactly that reviewed snapshot before apply.

Design: [2026-09-30-preset-customization-design.md](2026-09-30-preset-customization-design.md)

This plan specifies the recommended Choose Preset → Pack Picker → Review → confirm/save → Apply flow. The user accepted this flow and authorized implementation on 2026-10-05.

## Chunks

1. **Checked catalog resolution** — return known automatic dependencies and deterministic missing IDs without changing tolerant callers.
2. **Picker boundary and session output** — preserve explicit picker choices while carrying effective and invalid selections separately.
3. **Wizard state, review and confirmation** — preserve edits across navigation, show invalid entries and save only a valid reviewed snapshot.

## Whole-feature acceptance

- Built-in presets, local presets and Customize seed Pack Picker with the correct explicit IDs.
- Dependency recomputation retains shared dependencies, releases obsolete automatic dependencies and preserves explicitly selected dependencies.
- Review/back and Pack Picker/preset/reselect preserve explicit choices and dependency classification; choosing another preset resets from its resolved sources.
- Missing selected recipes and dependencies are visible in Pack Picker and Review and block both manifest initialization and apply.
- Review IDs, sorted IDs passed to `InitVault`, `desired.items` and apply IDs agree without duplicates. `desired.presets` retains provenance.
- Empty Customize is valid. Unknown free space and over-budget estimates stay informational.
- No write or download occurs before Review confirmation. Init failure keeps the selection on Review; resume continues from saved `desired.items` without expanding the preset again.
- `go test ./...` passes in `tui`, `host-tui` and `host-cli`; repository `make verify` passes.

## Non-goals

- Editing an initialized vault (Phase 8), recipe refresh/acquisition hashes (Phase 7), or browser configuration (Phase 9).
- New screens, search/filtering, preset export, or draft persistence before confirmation.
- Exact installed sizing, per-platform size policy, or capacity enforcement.
- Preset syntax, manifest schema, planner, apply, index or catalog inheritance redesign.

## Things not to redesign

- `tui.TreePicker` owns explicit selection, automatic dependencies, toggles and deduplicated size totals.
- Catalog interpretation stays in `host-cli`; `host-tui` does not import catalog packages.
- `desired.items` remains the expanded snapshot and `desired.presets` provenance; add no include/exclude fields or migration.
- Keep the Phase 2 boundary: Review confirmation initializes, then starts cancellable Apply.
- Preserve concurrent Phase 1b work and all pre-existing uncommitted changes. No reset, stash, staging, commits or Git metadata edits in this shared workspace; use the coordinator’s file snapshot for scoped review instead.

## Chunk 1 — Checked catalog resolution

### Goal

Add a deterministic resolver for the wizard while preserving `Catalog.ResolveDeps` and its tolerant log-and-skip behavior for existing callers.

### Architecture

Add this sibling API in the catalog:

```go
func (c *Catalog) ResolveDepsChecked(
    selectedIDs map[string]bool,
    defaults DepDefaults,
) (autoDeps map[string]bool, missingIDs []string)
```

It uses the existing recursive dependency and type-default rules, but reports unknown explicit and transitive IDs instead of silently dropping them.

### Files

- `host-cli/internal/catalog/deps.go`
- `host-cli/internal/catalog/deps_test.go`

### Depends on

None.

### Invariants

- Only true input values seed traversal; false map entries are ignored on this new path.
- Known transitive dependencies not explicitly selected appear in `autoDeps`.
- Unknown explicit IDs and dependencies appear once in sorted `missingIDs`.
- Cycles terminate; explicit known IDs are never relabeled automatic.
- Nil `deps` uses type defaults and explicit `deps: []` suppresses them.
- Existing `ResolveDeps` behavior and signature do not change.

### Follow this existing pattern

`host-cli/internal/catalog/deps.go` — reuse its cycle-safe walk and `depsForItem` rules; add a narrow sibling API, not another dependency framework.

### Implementation notes

- Traverse with visited and missing sets, then sort missing IDs once.
- Do not put missing IDs in `autoDeps`; they have no resolvable recipe and Chunk 3 blocks saving them.
- Keep `ResolvePreset.Sources` intact even when `Items` omitted an unknown ID.

### Edge cases

- Multiple consumers share a missing dependency; an explicit missing ID is also referenced as a dependency.
- A known cycle has an unknown branch.
- The selection map contains only false entries.

### Acceptance

- Test missing explicit/direct/transitive IDs, deduplication, false entries, cycles, stable ordering and unchanged tolerant resolution.
- `go test ./...` passes in `host-cli`.

### Do not

- Make catalog loading globally strict or change user-preset validation, inheritance/removal or dependency YAML.

## Chunk 2 — Picker boundary and session output

### Goal

Expose checked resolution through the host/TUI boundary and make Pack Picker return explicit choices, effective choices and missing IDs without conflating them.

### Architecture

Change the host-TUI boundary to:

```go
type DepResolver func(map[string]bool) (autoDeps map[string]bool, missingIDs []string)
```

`buildWizardConfig` adapts `cat.ResolveDepsChecked`. `packPickerModel` initializes `TreePicker` from explicit IDs, resolves from `TreePicker.UserSelection()`, applies known dependencies with `SetAutoDependencies`, and stores sorted missing IDs. Both `packDoneMsg` and `packCancelMsg` carry `explicitIDs`, effective `selectedIDs`, and `missingIDs` as independent copies; the orchestrator records them before any stage transition.

### Files

- `host-tui/internal/wizard/types.go`
- `host-tui/internal/wizard/packpicker.go`
- `host-tui/internal/wizard/packpicker_test.go`
- `host-cli/internal/cli/root.go`
- `tui/treepicker.go`
- `tui/treepicker_test.go`

### Depends on

Chunk 1.

### Invariants

- Resolver input is always explicit selection, never the dependency-expanded set.
- Automatic items cannot be toggled off while required; an explicit dependency can become automatic when the user untoggles it but a consumer remains.
- Every toggle reconciles dependencies once. Navigation and resize do not alter selection.
- Completion values are copied, deduplicated and stable; config/preset maps and slices are never mutated.
- Capture the copied done/cancel message synchronously during key handling before returning its asynchronous Bubble Tea command; later key events cannot change the earlier snapshot.
- Unavailable explicit rows remain validation errors even with a nil resolver. Merge their currently selected IDs with any resolver-reported missing IDs into one sorted deduplicated slice; nil disables dependency expansion, not known unavailable-item validation.
- All known loose recipes remain in `Other / individual-items`.

### Follow this existing pattern

`host-tui/internal/wizard/packpicker.go` — retain its thin adapter around `TreePicker`; use existing `UserSelection`, `SetAutoDependencies`, `IsAutoDep` and `CheckedIDSlice` APIs.

### Implementation notes

- Keep `selectedIDs` as the message's effective set for compatibility with the orchestrator; add explicit and missing fields beside it.
- Render `Missing catalog recipes: <sorted IDs>` below the existing size summary using the danger theme.
- Copy the supplied groups and append a small unavailable-items pack containing initially selected explicit IDs absent from the known source index. These rows use normal toggles, so a stale preset source can be removed. Do not fabricate rows for missing automatic dependencies or put unknown dependencies into the saved effective set.
- Add an optional `Unavailable` display flag to `tui.PackSource`; its default preserves every existing caller. Item rows/details show `unknown size` for these entries and omit them from numeric estimates; while selected, the summary identifies its numeric total as a known-size subtotal. Do not introduce a second selection API, catalog traversal in the TUI or new key bindings.

### Edge cases

- A and B require D: removing A retains D; removing B releases it.
- D is explicit, then required, then its consumer is removed: D remains explicit.
- A missing preset source has a removable unavailable row; a missing dependency has neither row nor size, but remains visible in the validation line until its consumer is removed.

### Acceptance

- Test distinct explicit/effective completion maps, shared-dependency release, explicit retention, missing-ID rendering and item/pack/group reconciliation.
- Test removing a stale explicit preset source using ordinary toggles, and that missing automatic dependencies cannot be silently omitted. Verify unknown-size rendering and unchanged size output for callers without unavailable entries.
- `go test ./...` passes in `tui`, `host-tui` and `host-cli`.

### Do not

- Add a new screen, capacity block, exact-size claim, or duplicate selection implementation.

## Chunk 3 — Wizard state, review and confirmation

### Goal

Preserve explicit and effective state across the existing flow, render missing entries in Review, and prevent invalid state from reaching init/apply.

### Architecture

`wizard.Model` owns `explicitIDs`, `effectiveIDs`, sorted `missingIDs` and a preset-selection-initialized flag. Review/back reconstructs Pack Picker from explicit IDs and recomputes dependencies.

Preset identity is `(Name, Region)`. Reselecting the same identity preserves explicit edits; selecting another replaces them from `PresetOption.SourceIDs`. The initialized flag makes empty `(Name, Region)` Customize distinguishable from no prior selection.

Before handling either picker done or cancel, copy the live explicit/effective/missing snapshot into the wizard. Cancel still returns to Choose Preset using the existing key; same-preset reselection reconstructs from that captured explicit state.

Review indexes known metadata from Pack Groups, iterates sorted effective IDs, and appends missing dependencies not already present. Unknown entries use `ReviewItem.Unavailable`, render `unknown size`, and are excluded from the numeric known-size sum; label that sum as a known-size subtotal while unavailable entries remain. The existing danger area lists missing IDs. The orchestrator intercepts confirmation while missing IDs remain, before `InitVault` or `RunApply`.

### Files

- `host-tui/internal/wizard/model.go`
- `host-tui/internal/wizard/model_test.go`
- `host-tui/internal/wizard/review.go`
- `host-tui/internal/wizard/review_test.go`
- `host-tui/internal/wizard/packpicker.go` (remove temporary constructor fallback)
- `host-tui/internal/wizard/packpicker_test.go` (update constructor fixtures)
- `host-tui/internal/wizard/view.go` (private validation rendering and actual pane-width calculation, if needed)
- `host-cli/internal/cli/root_test.go`

Ownership: the coordinator owns the host adapter/persistence fixture in `root_test.go`; the phase worker owns the seven wizard files. These checks can be developed alongside Chunk 2 without editing its files. The coordinator reports the fixture result in the Chunk 3 handoff.

### Depends on

Chunk 2.

### Invariants

- Automatic dependencies never become explicit through navigation.
- Review items and effective IDs are deduplicated; valid IDs are sorted before init and in `WizardResult`.
- Missing IDs stay visible across resize/back and communicate unknown size rather than zero-size certainty.
- Invalid confirmation performs no side effect. Init errors remain retryable on Review with state intact.
- Empty selection remains valid.

### Follow this existing pattern

`host-tui/internal/wizard/model.go` — keep transitions and side effects in the orchestrator while submodels emit value messages.

### Implementation notes

- Replace ambiguous model `checkedIDs` with explicit/effective state; retain no third representation.
- Remove Chunk 2's temporary `packPickerConfig.checkedIDs` compatibility fallback once all wizard constructors use `explicitIDs`; do not leave two ways to initialize the same selection.
- Build Review from effective plus missing IDs. Clear validation after repair; keep init errors only until the next attempt/back transition as current behavior dictates.
- Pass one sorted effective slice to `InitVault` and the final result. `RunApply` receives context/path and reads the saved manifest; do not add an item-list argument or re-expand the preset at any later boundary.

### Edge cases

- Review/back preserves classification; Pack Picker cancel and same-preset or Customize reselection preserve edits; another preset resets them.
- Duplicate pack membership affects neither review count nor size.
- Unknown free space omits comparison. Init failure does not start Apply.

### Acceptance

- Test navigation preservation/reset, deterministic init IDs, empty selection, blocked invalid confirmation, unavailable review rows, small-terminal visibility, init failure retention and no apply before successful init.
- Validation and init errors take priority over scrollable content in the final shell viewport. Render them before the tree/list and reserve their wrapped height when computing visible rows; do not reserve a minimum of three content rows at the expense of the error. Test `Model.View` at 80×8 for picker missing dependencies and Review missing/init errors, not just the unbounded submodel View. Keep existing navigation and the same validation text.
- Explicitly test edit → picker cancel → same-preset/Customize reselection, retaining explicit/automatic classification. Test removing an unavailable preset source, returning through Review, and successfully confirming the remaining valid selection.
- Add a host adapter/persistence fixture using a temporary local preset and real init callback: remove a preset item, add a different item with a dependency, save the effective selection, reload the manifest and build the pending plan. Change the preset definition afterward and prove reopening still uses only the saved selection. Pair this with the wizard test that captures exactly the same confirmed IDs; do not expose internal UI state just to join the tests.
- Existing `tui/treepicker_test.go` stays green.
- `go test ./...` passes in all three Go modules; `make verify` passes.

### Do not

- Save before confirmation, mutate preset files, re-expand presets on reopen/resume, or change manifest/planner/apply/index behavior.

## Final review and verification

Review correctness and design together against the coordinator's scoped pre-execution snapshot at `/private/tmp/svalbard-phase3-baseline`, not a whole-worktree Git diff containing pre-existing Phase 2 work. Scope is `.work/preset-scoped-files.json`. Verify the interactions between checked resolution, synchronous picker snapshots, wizard back/reselect state, unavailable-item repair and confirm-before-save. Preserve existing apply/cancellation behavior.

The coordinator runs `go test -race ./... -count=1` in `tui` and `host-tui`, focused catalog/CLI race tests in `host-cli`, whole-repository `make verify` with uncached tests, and the host build. No drive-runtime code changes require new target runtime behavior tests. Record concrete user-path evidence and limits in the checkpoint before closing Phase 3.
