## Why

Recipes and presets are manually maintained in two nearly identical trees: 245 source files and 244 embedded files. A deterministic synchronization guard removes manual dual-edit drift without changing catalog contents or runtime behavior.

## What Changes

- Keep root `recipes/` and `presets/` as the only human-edited catalog source.
- Add a deterministic command that refreshes tracked embedded files while preserving ignored local data.
- Add catalog parity checking to canonical repository verification.
- Document synchronization and verification.

## Capabilities

### New Capabilities
- `catalog-source`: Catalog data has one editable source and a reproducible tracked embedded snapshot.

### Modified Capabilities

None.

## Impact

- Affects synchronization tooling, `Makefile`, and contributor documentation.
- Catalog IDs, files, parsing, builder availability, runtime fallback, and distribution behavior remain unchanged.
