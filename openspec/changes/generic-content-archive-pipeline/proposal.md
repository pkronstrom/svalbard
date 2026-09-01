## Why

The 3,700-line MIY Python builder contains a reusable content-archive pipeline buried under site-specific code: extract source links, verify, fetch, transform, render, collect, and package. Porting it line-for-line would preserve accidental complexity rather than deepen the builder system.

## What Changes

- Implement generic Go blocks for source parsing, URL verification, fetch, metadata extraction, asset collection, HTML transformation, rendering, collection, and ZIM packaging.
- Use directory edges and JSONL manifests between blocks.
- Express ordinary site behavior as typed data rules.
- Keep small named Go enrichers only for irreducible APIs/auth/site semantics.
- Use a linear pipeline plus a cached `map` block for per-project fan-out.
- Make MIY the first configuration of the generic archive pipeline.

## Capabilities

### New Capabilities
- `generic-content-archive-pipeline`: Catalog content collections can transform many linked pages into a cached, resumable offline archive through reusable Go blocks.

### Modified Capabilities

None.

## Impact

Depends on content-addressed builder blocks and heavy ZIM/browser seams. Replaces the planned line-by-line MIY port; the Python builder remains until parity.
