## Why

The root `make test` command verifies only `host-cli`, while the repository has five Go modules and spec-flow duplicates the complete matrix. One canonical command should define repository readiness.

## What Changes

- Add `make verify` covering catalog parity and all five Go modules.
- Keep `make test` as an explicit alias.
- Make spec-flow call the canonical target.
- Document catalog synchronization and full verification.

## Capabilities

### New Capabilities
- `repository-verification`: One root command verifies every maintained Go module and generated catalog parity.

### Modified Capabilities

None.

## Impact

- Affects `Makefile`, `.spec-flow.yaml`, and contributor documentation.
- Does not change builds, runtime behavior, release publishing, or dependencies.
