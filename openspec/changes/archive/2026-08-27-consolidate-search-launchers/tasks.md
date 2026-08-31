reviewed: true

## 1. Reassess after Session consolidation

- [x] 1.1 Compare post-refactor Session and MCP embedding launch paths
- [x] 1.2 Compare post-refactor Session and MCP Kiwix launch paths
- [x] 1.3 Record which exact flags, resolution, readiness, and cleanup behavior remains duplicated

## 2. Consolidate concrete launch protocols

- [x] 2.1 Add deterministic readiness, failure, timeout, cleanup, reuse, and fallback tests
- [x] 2.2 Extract a concrete embedding launcher only if meaningful duplication remains
- [x] 2.3 Extract a concrete Kiwix launcher only if meaningful duplication remains
- [x] 2.4 Preserve Session ownership and MCP caching/close semantics

## 3. Verify lifecycle behavior

- [x] 3.1 Run focused and full `drive-runtime` tests
- [x] 3.2 Smoke terminal and MCP search with success, reuse, and embedding-unavailable fallback
