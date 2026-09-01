## Why

Builder retries currently fingerprint procedure configuration but not input content, and container tools receive broad vault/work mounts. Slow crawl or transformation outputs should be inspectable, reusable, and isolated so downstream template/package edits do not repeat upstream work.

## What Changes

- Add optional declared input/output paths to build steps and compiled procedures.
- Define each cacheable block as `(input directories, normalized params) -> output directory`.
- Cache block output by content hash under vault staging with a small manifest.
- Reuse cached output without re-running the block.
- Provide isolated input/output/work directories for new tool procedures.
- Keep existing undeclared procedures working uncached; no DAG or map combinator yet.

## Capabilities

### New Capabilities
- `content-addressed-builder-blocks`: Build procedures can reuse inspectable directory outputs based on inputs and parameters.

### Modified Capabilities

None.

## Impact

- Extends the builder foundation without changing existing linear recipe behavior.
- No new dependency or workflow framework.
