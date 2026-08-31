reviewed: true

## 1. Define the protocol

- [x] 1.1 Inventory every built-in action argument contract and existing default
- [x] 1.2 Add round-trip and validation tables for every built-in action
- [x] 1.3 Define concrete `NativeInvocation` encoding and decoding in `internal/actions`

## 2. Migrate both sides

- [x] 2.1 Route action resolution through the centralized encoder
- [x] 2.2 Route hidden-subcommand parsing through the centralized decoder
- [x] 2.3 Change `main` dispatch to consume stable action IDs and named arguments
- [x] 2.4 Remove duplicate positional mapping and defaulting

## 3. Verify compatibility

- [x] 3.1 Run focused and full `drive-runtime` tests
- [x] 3.2 Smoke representative required, supplied optional, and omitted optional arguments
