## Context

Go embedding cannot reference files outside the package directory, which led to a manually copied catalog tree. Physical generated duplication is required for clean-checkout builds, but manual dual editing is not.

## Goals / Non-Goals

**Goals:** one editable root catalog, deterministic tracked snapshot synchronization, and drift detection.

**Non-Goals:** deleting catalog content, pruning embedded assets, changing runtime fallback, moving canonical files, or requiring generation before ordinary `go test`.

## Decisions

- Root `recipes/` and `presets/` remain canonical.
- `scripts/sync-catalog.sh` copies tracked and non-ignored canonical files with preserved modes.
- The explicit test-only web-video builder remains excluded as before.
- Synchronization removes tracked/generated destination files but preserves ignored local recipes and caches.
- `--check` compares the managed snapshot while ignoring local recipe files and Python caches.
- `make verify` runs the parity check before module tests.

## Risks / Trade-offs

- Generated files remain tracked because Go embed and clean-checkout builds require them.
- The guard eliminates drift in managed files without claiming ignored local data is canonical.
