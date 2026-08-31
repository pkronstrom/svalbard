## Context

`TreePicker` exposes mutable maps for effective, explicit, and automatic selections. Browse replaces maps directly for presets, while Wizard manually merges and removes automatic dependencies.

## Goals / Non-Goals

**Goals:**
- Make selection invariants internal to `TreePicker`.
- Provide only the mutation operations current callers require.
- Preserve explicit choices when automatic dependencies change.

**Non-Goals:**
- Moving dependency resolution into `tui`.
- Changing rendering, cursor movement, layout, or collapse state.
- Creating a generic picker wrapper.

## Decisions

- Rename the three maps to private fields.
- Add `ReplaceUserSelection`, `UserSelection`, and `SetAutoDependencies` (or equivalent local names).
- `UserSelection` returns a copy.
- Replacing explicit selection resets effective selection and clears stale automatic dependencies.
- Setting dependencies removes stale automatic-only entries, preserves explicit entries, and adds current dependencies.
- `ToggleAtCursor` updates explicit and effective sets atomically.

## Risks / Trade-offs

- Incorrect merge order can remove a manually selected dependency when it ceases to be automatic.
- Returning a mutable internal map would defeat encapsulation; copy semantics require focused tests.
- Callers must migrate in the same cutover because exported fields are removed.
