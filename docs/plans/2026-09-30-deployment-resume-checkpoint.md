# Deployment and interruption recovery checkpoint

Status: complete, including final correctness/design review and verification on 2026-09-30.

Scope: MASTERPLAN Phase 2. Preset customization, search/pack operations, catalog updates and the configuration web UI remain in their later phases.

## Resulting user path

1. Init saves selected items, preset, region and host platforms before acquisition. Both init APIs refuse to replace an existing manifest.
2. Apply downloads into sibling `.part` files with small `.download.json` recovery metadata; completed content is validated before promotion. Cache copies use a separate temporary file, so a rejected cache does not destroy HTTP progress.
3. The coordinator checkpoints each complete recipe before announcing it complete. A multi-platform recipe becomes realized only after every supported selected target completes; its already-finished files are reusable on retry.
4. CLI interrupts and TUI Ctrl+C/Esc cancel acquisition, drain terminal results and finish saving before exit/back. UI terminal snapshots retain checkpointed peer completions even if cancellation dropped an event.
5. Reopen the same vault → Dashboard → Plan → Enter to apply pending work. An empty recipe plan also permits Enter to finalize/repair runtime generation; existing `b` opens Browse.
6. Incomplete views show completed, failed and pending counts, the error and resume guidance. Incomplete wizard applies skip indexing.

This is transfer recovery, not recipe acquisition hashes/versioned updates. Sidecars use an internal format version; the existing v2 vault manifest remains unchanged.

## Verification

- `make verify` — passed, 53 tested packages across all five Go modules, plus catalog synchronization check.
- `make build` — passed; host CLI and drive runtimes for macOS arm64/x86_64 and Linux arm64/x86_64 built.
- Downloader/apply/commands/manifest/catalog focused race suite — passed.
- Full host TUI race suite and CLI race suite — passed, including final review regression fixes.
- `git diff --check` — passed.

Deterministic fixtures prove actual init → one completion while a peer stalls → saved manifest before cancellation → cancel → reload/status/plan → exact-offset Range resume → full success, retaining all desired selections and avoiding a completed-item re-request.

Additional fixtures cover malformed/chunked ranges, unsafe 416, changed/untrusted validators, checksum/cache rejection, hashing/copy cancellation, recovery on either side of promotion, preservation after rename failure, active-peer cancellation on checkpoint failure, joined apply/save errors, rejected-checkpoint final saving, multi-platform artifact reuse and empty-plan runtime repair. UI fixtures exercise concurrent callbacks, delayed final saving, cancel-before-start, parent forwarding and skipped indexing. A real OS interrupt reaches the actual Cobra apply handler against a blocked local HTTP fixture and leaves partial bytes plus recovery identity and a saved manifest.

Final review found three issues, all reproduced and fixed: cancellation during safe-416 hashing now preserves recovery files; cache promotion saves completed metadata and supports offline recovery before/after rename; empty-plan labels advertise the actual apply/finalize and Browse actions. Full verification and the four-target release build were rerun after these fixes.

## Limits

No public bulk downloads, physical-stick removal or power-loss tests were performed. Cross-platform runtime builds passed; the recovery fixtures ran on the current macOS host. This work does not add multi-writer locking or guarantee safe physical removal while writing. FTS/hybrid/pack user-path verification remains in later masterplan phases.

The shared worktree changes are intentionally uncommitted. Existing Claude work was preserved.
