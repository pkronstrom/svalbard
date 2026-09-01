reviewed: true

## 1. Define generated catalog contract

- [x] 1.1 Inventory canonical and embedded catalog differences
- [x] 1.2 Define deterministic copy, exclusion, permission, and stale-file rules
- [x] 1.3 Preserve ignored local recipes and Python caches

## 2. Implement one editable source

- [x] 2.1 Add catalog synchronization and check modes
- [x] 2.2 Exercise synchronization against the current tracked snapshot
- [x] 2.3 Document the root catalog as canonical

## 3. Wire and verify workflow

- [x] 3.1 Add catalog parity checking to `make verify`
- [x] 3.2 Confirm direct clean-checkout module tests need no pre-generation step
- [x] 3.3 Run canonical repository verification
