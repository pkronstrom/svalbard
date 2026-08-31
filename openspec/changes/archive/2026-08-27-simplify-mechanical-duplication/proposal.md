## Why

Several small abstractions and branches have no production use and obscure the paths that actually execute. Removing them reduces maintenance cost without adding dependencies or changing supported workflows.

## What Changes

- Remove unused palette verb/freeform matching.
- Remove unused disabled-navigation behavior.
- Replace manual image copying with `image/draw`.
- Replace a one-entry builder dispatch map with a direct condition.

## Capabilities

### New Capabilities
- `mechanical-simplification`: Preserved observable behavior for palette matching, navigation, image conversion, and builder dispatch after dead flexibility is removed.

### Modified Capabilities

None.

## Impact

- Affects `tui`, `build-tools/pkg/imaging`, and `host-cli/internal/builder`.
- Removes unused exported struct fields in the internal reusable TUI module.
- No new dependencies or supported behaviors are introduced.
