## Context

`binary.Resolve` accepts an injected detector, defaults nil to `platform.Detect`, and receives `platform.Detect` from every production caller. Only tests vary the callback.

## Goals / Non-Goals

**Goals:** remove the redundant callback and preserve binary discovery and extraction behavior.

**Non-Goals:** changing supported platforms, path precedence, archive extraction, or introducing a platform registry.

## Decisions

- `binary.Resolve(name, driveRoot)` calls `platform.Detect` internally.
- Tests create fixture directories for the actual detected platform.
- Every caller migrates in the same cutover; no compatibility shim remains.

## Risks / Trade-offs

- Tests no longer simulate unsupported platforms through this API; platform mapping remains covered by the platform package.
