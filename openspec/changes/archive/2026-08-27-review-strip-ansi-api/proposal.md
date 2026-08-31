## Why

`StripAnsi` has no production caller in this repository but is exported from the reusable `tui` module. Removing it without checking downstream consumers could create an avoidable compatibility break.

## What Changes

- Determine whether the `tui` module has known downstream consumers or a public compatibility commitment.
- Keep `StripAnsi` exported when external use is possible.
- Only unexport or remove it when evidence establishes that the change is safe.

## Capabilities

### New Capabilities
- `ansi-output-normalization`: Compatibility requirements for ANSI-stripping behavior in the reusable TUI module.

### Modified Capabilities

None.

## Impact

- Affects only `tui/shell.go` and its tests if removal is proven safe.
- This optional change does not block the main simplification sequence.
