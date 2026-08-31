## Why

`TreePicker` exposes three mutable selection maps, forcing Browse and Wizard callers to reproduce invariants and allowing stale automatic dependencies. Encapsulation gives the picker one authoritative selection state while leaving dependency resolution in the domain layer.

## What Changes

- Make explicit, effective, and automatic selection maps private.
- Add minimal replacement, read-copy, and automatic-dependency mutation methods.
- Migrate Browse preset cycling and Wizard dependency recalculation.
- Add invariant and caller behavior coverage.

## Capabilities

### New Capabilities
- `treepicker-selection`: Explicit and automatic tree selection semantics with protected internal state.

### Modified Capabilities

None.

## Impact

- Affects `tui/treepicker.go`, host Browse, and the pack-selection Wizard.
- Removes direct field mutation from external packages.
- Rendering, cursor movement, collapse state, and dependency-resolution rules remain unchanged.
