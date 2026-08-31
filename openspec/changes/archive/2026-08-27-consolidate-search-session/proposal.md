## Why

Terminal search and menu search currently duplicate database, engine, embedding-process, Kiwix-process, fallback, and cleanup logic. Making `Session` the sole backend removes divergent behavior and makes resource ownership explicit.

## What Changes

- Refactor terminal `Run` into an input/output adapter over `Session`.
- Preserve terminal mode commands, result numbering, fallback status, and opening behavior.
- Add behavioral coverage for terminal commands, fallback, result opening, and cleanup.
- Keep native and menu entry points unchanged.

## Capabilities

### New Capabilities
- `drive-search-session`: Shared search behavior and lifecycle used by terminal and menu entry points.

### Modified Capabilities

None.

## Impact

- Affects `drive-runtime/internal/search` and its terminal/menu callers.
- Removes duplicated backend and child-process ownership from `Run`.
- No action IDs, menu routing, dependencies, or user-facing commands change.
