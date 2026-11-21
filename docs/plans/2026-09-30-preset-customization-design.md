# Preset customization through the TUI

Status: implemented and verified 2026-10-05; see [checkpoint](2026-10-05-preset-customization-checkpoint.md). Existing flow accepted by the user’s “do continue”.

# Goal
Choose a built-in or local preset, customize its recipe selection in the existing init wizard, inspect dependencies and estimated space, then save exactly the reviewed choices before acquisition. Reopening an interrupted vault must use that saved selection.

# Decisions
- Scope is MASTERPLAN Phase 3 initial customization. Editing an existing vault belongs to Phase 8; recipe refresh/merge semantics belong to Phase 7; browser configuration belongs to Phase 9.
- Reuse the Go Bubble Tea wizard, shared TreePicker, catalog resolver and existing manifest init/apply path. The older April Python picker plan is historical context, not an implementation starting point.
- `desired.items` remains the authoritative saved selection snapshot, including required dependencies. `desired.presets` records provenance. Do not expand the preset again on reopen or overwrite selections when master/local presets change.
- Do not introduce a manifest migration, persistent include/exclude fields, preset editor/export screen or new configuration syntax. Explicit versus automatic selections only need to remain distinct while editing this wizard session.
- Preserve Phase 2 cancellation/checkpoint guarantees and all shared uncommitted work. Planning changes documents only; execution must use scoped files and must not reset, stash, stage or commit unrelated work.

# Agreed behavior
- Keep Choose Preset → Pack Picker → Review → confirm/save → Apply. Keep current navigation and toggles; do not add another screen.
- Required dependencies remain automatically selected and marked with the current auto-included explanation. An item still needed by another selected recipe cannot be removed independently; removing its last consumer releases it unless explicitly selected.
- Review/back preserves explicit selections rather than turning automatic dependencies into permanent user selections. Picker/back/reselecting the same preset preserves edits; choosing a different preset starts from that preset's sources.
- Existing size displays remain estimates based on catalog values, unique recipe IDs and current free-space information. Do not claim exact installed size or resolve the concurrent Python per-platform size decision in this phase. Do not add a new capacity hard-block policy without a user decision.
- Invalid/missing selected recipes or required dependencies must be visible and prevent confirmation rather than silently disappearing. Empty selection stays valid if the runtime can be initialized with no recipes, as supported by Phase 2.
- Missing explicitly selected preset sources appear as removable unavailable rows in the existing picker, with unknown size. Missing automatic dependencies remain validation errors, repairable by removing their known consumer. Picker/back captures live edits before changing stages.

# Invariants
- The base preset and user preset files are never modified by vault customization.
- Review IDs, saved desired IDs and IDs handed to apply agree, deterministically and without duplicates.
- No required selected ID can be hidden from the tree/review or omitted from size accounting solely because it lacks pack membership.
- Shared dependencies survive while any consumer remains; obsolete automatic dependencies disappear; explicit dependencies remain.
- Back navigation and resizing do not lose edits or mutate preset/config slices and maps.
- No manifest write or download occurs before review confirmation; init failure remains on Review and does not start Apply.
- Reopen/resume never restores a removed preset item or installs a newly added master preset item automatically.

# Non-goals
New customization UI, fuzzy search/filtering, reusable preset saving, draft persistence before confirmation, remote version discovery, recipe hashes, removal from an already realized vault, search/index changes, new dependency framework, exact multi-platform sizing, bulk downloads.

# Relevant existing architecture
- `host-tui/internal/wizard/model.go`: presetDone currently resets selection; packDone currently stores the effective selection. Review/back reconstructs the picker from that effective set, which promotes automatic deps to explicit ones.
- `host-tui/internal/wizard/packpicker.go`: shared TreePicker already separates explicit and automatic IDs but its done message returns only effective IDs.
- `tui/treepicker.go`: UserSelection, SetAutoDependencies, IsAutoDep, CheckedIDSlice and deduplicated size accounting already exist. Preserve this ownership and API rather than duplicating selection logic.
- `host-cli/internal/cli/root.go`: prepares wizard catalog/groups, including loose recipes, and dependency resolution callback. Catalog-specific validation belongs on this host side, not inside the TUI.
- `host-cli/internal/catalog/deps.go`: current resolver logs and skips missing dependency IDs; its traversal accepts map keys irrespective of false values. Any stricter path must be scoped to customization and preserve current callers deliberately.
- `host-cli/internal/catalog/catalog.go` and `userpresets.go`: built-in/local preset inheritance and removals already exist. ResolvePreset Sources can include unknown IDs while Items skips them; do not silently hide this difference in the wizard adapter.
- `host-cli/internal/commands/init.go`, `manifest/manifest.go`, `planner/plan.go`: explicit IDs persist already and planner compares those IDs, not preset definitions.
- `docs/plans/2026-09-30-claude-session-coordination.md`: local custom preset loading/copying is existing work, not part of a second implementation here.

# UI — existing flow to retain

Preset picker: retain list, details and Enter select. A preset always opens Pack Picker, already seeded from that preset. Customize starts an empty selection.

```
Pack Picker
  [x] Selected preset item
  [ ] Other catalog item
  [x] Required tool (auto-included)
  Total: estimated catalog size / available space
  Current navigation / toggle / continue / back keys
```

Keep existing tree layout, dependency detail text and size display. Validation/errors take priority before scrollable content so the final shell viewport displays them even at short heights; reserve their wrapped height before choosing visible content rows. Do not silently convert picker continue into immediate downloading; it opens Review.

```
Review
  <visible validation/init error, when present>
  Target: <vault path>
  Items: <unique effective count>, <estimated size> / <free space>
  <scrollable complete list, including dependencies>
  enter confirm   esc back
```

Confirm validates, saves desired state, then starts the existing cancellable Apply screen. On validation/init failure stay here with selections intact; back permits repair. Existing small-terminal scrolling and unknown-free-space behavior remain.

# Open questions
- None. The user accepted continuing with the reviewed existing-flow implementation.
