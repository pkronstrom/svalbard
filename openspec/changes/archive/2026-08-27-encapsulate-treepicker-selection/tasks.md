reviewed: true

## 1. Protect selection invariants

- [x] 1.1 Add tests for replacement, stale dependencies, explicit dependency preservation, copy isolation, and toggling
- [x] 1.2 Make effective, explicit, and automatic selection maps private
- [x] 1.3 Add replacement, copied-read, and automatic-dependency mutation methods
- [x] 1.4 Update internal toggling to maintain all selection sets

## 2. Migrate callers

- [x] 2.1 Replace Browse preset map assignment with the picker replacement method
- [x] 2.2 Replace Wizard map reconciliation with copied selection and automatic-dependency update
- [x] 2.3 Remove every external direct mutation of picker selection maps

## 3. Verify behavior

- [x] 3.1 Run full `tui` and `host-tui` tests
- [x] 3.2 Smoke Browse preset cycling and Wizard dependency selection
