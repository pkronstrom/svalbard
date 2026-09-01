reviewed: true

## 1. Define directory edges

- [x] 1.1 Add declared inputs/outputs to BuildStep and Procedure
- [x] 1.2 Add deterministic tree manifest and digest tests

## 2. Implement cache

- [x] 2.1 Compute procedure key from params and input digests
- [x] 2.2 Publish verified output atomically with manifest
- [x] 2.3 Restore cache hits without final staging symlinks
- [x] 2.4 Preserve failed block workspace for inspection

## 3. Narrow tool workspace

- [x] 3.1 Materialize declared inputs under read-only input mounts
- [x] 3.2 Provide empty output and persistent work directories
- [x] 3.3 Keep undeclared legacy procedures uncached and compatible

## 4. Verify

- [x] 4.1 Prove hit, miss, input change, invalid manifest, and cache deletion behavior
- [x] 4.2 Run canonical verification and strict OpenSpec validation
