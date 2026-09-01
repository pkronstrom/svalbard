## Why

Several production APIs and wrappers now exist only for tests or have been superseded by narrower implementations. Deleting them reduces misleading surface area without changing supported behavior.

## What Changes

- Remove four superseded host search-DB methods and redundant tests.
- Remove the obsolete MCP Kiwix resolution wrapper.
- Remove MCP production introspection and description APIs used only by tests.
- Remove no-op menu filtering, allocating visible-entry construction, the one-entry dashboard separator map, and variadic inspect filtering.
- Replace hand-rolled test helpers with existing standard/shared functions.

## Capabilities

### New Capabilities
- `runtime-surface-hygiene`: Runtime packages expose only behavior used by production callers.

### Modified Capabilities

None.

## Impact

- Affects host search indexing, MCP, menu, dashboard, inspect, and tests.
- Expected reduction: roughly 180–240 lines with no dependency changes.
- No user-facing command, protocol, or output contract changes.
