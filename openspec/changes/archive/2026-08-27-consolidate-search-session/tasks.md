reviewed: true

## 1. Characterize existing behavior

- [x] 1.1 Add terminal-loop tests for initial query, empty input, quit, and mode aliases
- [x] 1.2 Add Session-backed tests for fallback status and numbered result opening
- [x] 1.3 Add normal-exit and error-exit cleanup coverage

## 2. Consolidate backend ownership

- [x] 2.1 Construct and close one Session from `Run`
- [x] 2.2 Read counts and default mode from `Session.Info`
- [x] 2.3 Route terminal searches and result opening through Session methods
- [x] 2.4 Remove database, engine, capability, embedding, Kiwix, and cleanup ownership from `Run`

## 3. Verify entry points

- [x] 3.1 Run focused and full `drive-runtime` tests
- [x] 3.2 Smoke menu and native search, result opening, fallback, exit, and child-process cleanup
