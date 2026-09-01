## Context

The foundation supplies typed build data, linear procedures, recipe dependency ordering, JSONL events, and base/browser ToolRunner targets. This change implements the catalog families whose domain behavior or dependency sets are substantial.

## Goals / Non-Goals

**Goals:** concrete handlers for sufficiently specified families, typed validation, resumable staging, and verified outputs.

**Non-Goals:** OSM/raster/MML source decisions, generic scripts, host package installation, MIY crawling, or real network-heavy image tests in the default suite.

## Decisions

- Fimea `reference-static` uses Go XML streaming/data modeling and SQLite/FTS output.
- `zim-compact` uses Go orchestration, pinned zim-compact/zimwriterfs/zimcheck calls, and metadata handoff.
- existing zimit pipeline steps route automatically to the pinned browser target without recipe-selected images.
- vector-static and vector-service handlers own acquisition and call typed ogr2ogr/tippecanoe tools-base operations.
- unresolved OSM, raster-TMS, and MML-topo families move to `implement-map-build-families`.
- each handler validates required fields before effects.

## Risks / Trade-offs

Representative fixtures and fake tool calls cover behavior. Real image/network builds remain opt-in follow-up verification.
