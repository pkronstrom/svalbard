## Why

Several `strategy: build` recipes currently declare fields or families that are discarded or routed to missing checkout-relative scripts. Svalbard needs one real Go-controlled build path that preserves the standalone-drive contract while isolating non-Go provisioning dependencies from the host.

## What Changes

- Compile YAML build steps into typed Go procedures and execute them linearly in v1.
- Preserve a DAG-ready procedure seam without shipping a scheduler before a real recipe needs one.
- Add typed build assets, tables, tool requirements, network use, and transfer/staging estimates.
- Add recipe dependency ordering and resumable vault staging.
- Emit one JSON Lines build-event contract to CLI and TUI.
- Replace checkout-relative script fallback with app-bundle/Python foundation handlers and pre-execution rejection for unsupported families.
- Route named tool capabilities through pinned GHCR base/browser targets from one Dockerfile.
- Keep the finished drive standalone; runtime payload never depends on Docker.

## Capabilities

### New Capabilities
- `builder-pipeline`: Build recipes execute through typed Go procedures with isolated tools, resource disclosure, progress, resumability, and verified artifacts.

### Modified Capabilities

None.

## Impact

- Affects host catalog parsing, planning/apply order, builder implementations, progress reporting, the tools Dockerfile/workflow, and build recipes.
- Download-only recipes remain Docker-free.
- No scripting runtime, arbitrary plugin protocol, host package installation, or end-machine container dependency is introduced.
