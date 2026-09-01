## Context

The foundation supplies typed build data, linear procedures, recipe dependency ordering, JSONL events, and base/browser ToolRunner targets. This change implements the catalog families whose domain behavior or dependency sets are substantial.

## Goals / Non-Goals

**Goals:** one Go handler per retained family, typed validation, resource disclosure, resumability, and verified outputs.

**Non-Goals:** generic scripts, recipe-selected images, host package installation, or MIY-specific crawling.

## Decisions

- `reference-static` uses Go XML/CSV/SQLite support and typed table declarations.
- `zim-compact` uses Go extraction/transformation, then pinned zimwriterfs/zimcheck tool calls.
- `zimit` uses tools-browser and preserves WARC state in vault staging.
- geodata families use explicit Go handlers and typed GDAL/tippecanoe/pmtiles calls in tools-base.
- each family fails validation before execution when required catalog fields are missing.

## Risks / Trade-offs

Upstream data contracts need representative fixtures. Browser and geodata integration tests are opt-in because they pull images and may access networks.
