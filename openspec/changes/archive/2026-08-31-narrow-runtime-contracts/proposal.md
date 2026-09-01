## Why

`binary.Resolve` exposes a platform-detector callback even though every production caller passes the same default detector. Removing that test seam makes the internal runtime API smaller without changing resolution behavior.

## What Changes

- Detect the current platform inside `binary.Resolve`.
- Migrate all production callers to the two-argument API.
- Build test fixture paths from the detected current platform.

## Capabilities

### New Capabilities
- `runtime-contracts`: Packaged binary resolution has one production contract and preserves its existing search order.

### Modified Capabilities

None.

## Impact

- Affects internal drive-runtime binary resolution and its callers.
- No external API, supported platform, search order, or dependency changes.
