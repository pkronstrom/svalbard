## Why

After search ownership is consolidated, Session and MCP still risk duplicating exact embedding and Kiwix launch flags, readiness polling, timeout cleanup, and binary resolution. Concrete shared launchers prevent protocol drift while retaining caller-specific ownership.

## What Changes

- Reassess duplication after `consolidate-search-session` lands.
- Extract only still-duplicated embedding-server launch behavior.
- Extract only still-duplicated Kiwix launch behavior.
- Preserve per-Session ownership and MCP caching/close semantics.

## Capabilities

### New Capabilities
- `search-server-lifecycle`: Concrete embedding and Kiwix startup, readiness, timeout, and cleanup behavior shared by search consumers.

### Modified Capabilities

None.

## Impact

- Depends on `consolidate-search-session`.
- Affects `drive-runtime/internal/search` and `drive-runtime/internal/mcp`.
- Does not create a generic process supervisor or absorb unrelated file servers.
