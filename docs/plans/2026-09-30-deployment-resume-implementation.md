# Deployment resume implementation plan

Status: complete and verified 2026-09-30. See [checkpoint](2026-09-30-deployment-resume-checkpoint.md) for results and limits.

## Goal

Make initial vault deployment durable across cancellation, ordinary failures and process restarts. A completed recipe is checkpointed before the UI reports it complete; an incomplete transfer remains outside the final filename and resumes only when its bytes can be tied to the same remote representation.

Design: [Initial deployment and interruption recovery](2026-09-30-deployment-resume-design.md)

## Chunks

1. **Safe transfer persistence** — download and cache through sibling partial files, validate HTTP resume semantics, and reuse only verified completed artifacts.
2. **Incremental apply and durable manifest state** — checkpoint recipe completions as they arrive, save manifests atomically on every exit path, preserve init choices, and prove cancel/reload/resume end to end.
3. **Cancellation-aware CLI and TUI flow** — propagate contexts, wait for apply and saving before navigation or exit, and render complete/failed/pending outcomes accurately.

Chunks are sequential because Chunk 2 consumes the transfer contract from Chunk 1, and Chunk 3 consumes the apply/checkpoint contract from Chunk 2. Keep the shared dirty tree intact; do not reset, stash, stage or commit any files. This shared-workspace rule overrides the worker brief’s commit steps.

## Whole-feature acceptance

- A deterministic local HTTP fixture initializes a selected vault, completes one item while a second is blocked, cancels, reloads the manifest, and shows the same preset, region, platform and selected IDs with only the completed recipe realized.
- Reapplying that vault requests only pending recipes, resumes the safe partial at the exact byte offset, reuses already completed platform artifacts, and finishes with a non-empty `AppliedAt` only after the whole apply succeeds.
- Changed URLs/checksums/validators, ignored Range requests, malformed `206` responses, unsafe `416` responses, checksum failures and cancelled cache copies never expose incomplete final files or mark recipes realized.
- CLI cancellation returns only after checkpointing and final saving. TUI Ctrl+C/Esc shows stopping/saving, then exits or navigates only after the apply goroutine and manifest save finish. Incomplete results show completed, failed and pending counts, the actual error and a reopen-vault resume hint; indexing starts only after complete success.
- Focused package tests and race tests pass, followed by `make verify` and `make build`. When this plan is marked done, append the commands run and note that real public bulk downloads, physical-stick removal and power-loss durability were not exercised.

## Non-goals

- Acquisition hashes, online update discovery, retry/backoff infrastructure, a transfer database or journal, multi-writer locking and power-loss-proof filesystem guarantees.
- Preset composition, customization UX, indexing behavior, managed removal, builder design, pack actions or catalog curation beyond reading an existing recipe `sha256`.
- Runtime finalization remains rerunnable through the existing apply callback even for an empty recipe plan.
- A new resume screen or a second deployment path. Resume remains: reopen the vault, Dashboard → Plan → apply remaining items.

## Things not to redesign

- Keep the v2 manifest as desired/realized source of truth and keep planner/status deriving pending IDs from desired minus realized.
- Keep the current init wizard, dashboard, plan layout, progress rows, key bindings and CLI commands. Add only the agreed in-place stopping and incomplete-result states.
- Keep workers responsible for acquisition only. The apply coordinator orders results and commands own manifest persistence.

## Chunk 1 — Safe transfer persistence

### Goal

Give HTTP and local-cache acquisition one safe artifact contract: write away from the final name, preserve resumable bytes on cancellation, and promote only validated content.

### Architecture

`downloader.Download` owns transfer recovery. For destination `x`, data is written to `x.part`; `x.download.json` is a versioned sidecar containing the exact source URL, expected SHA-256, trusted strong ETag or valid HTTP-date Last-Modified validator, total size when known, computed SHA-256, and whether the artifact reached validation. The sidecar is transfer state only and must not enter the manifest or Phase 7 acquisition-hash model.

A final file is reusable when its supplied expected checksum matches, even without a sidecar, or when a completed sidecar matches URL/checksum/size and its recorded hash matches a context-aware rehash. Unknown legacy bytes under a final name are left untouched until a replacement is validated; they are never appended to. After close/sync and checksum validation, atomically save the completed sidecar before same-directory rename. Recovery rehashes matching completed bytes at either `x.part` (then promotes) or `x` (then reuses). This covers crashes on either side of promotion. Promotion uses same-directory rename, so cancellation or failure preserves any prior valid final.

Resume requires matching URL and expected checksum plus either a strong validator or a supplied checksum that will validate the complete result. Send `Accept-Encoding: identity`, `Range`, and `If-Range` when a validator exists. A `206` must have a parseable `Content-Range` whose start equals the partial size, whose end/body length/total are internally consistent, and whose representation identity matches when validator-based recovery is used. A `200` safely truncates `x.part` and becomes a fresh transfer.

For `416`, accept the partial only when `Content-Range: bytes */N` matches its size and representation identity or the full expected checksum validates. Otherwise discard unsafe recovery state and make one fresh non-Range request; never loop. Malformed ranges are rejected without appending. Hashing and streaming check `ctx` between chunks.

Local cache reuse keeps its current offline size/trust policy, but copies through a distinct sibling `x.cache.part` and validates before promotion, leaving HTTP `x.part` and its sidecar intact on cancellation or rejection, stops on cancellation, and validates a supplied expected checksum before promotion. A checksum-mismatched cache candidate is discarded as unsuitable and falls back to HTTP download; it does not fail the whole acquisition or replace the final file.

### Files

- `host-cli/internal/downloader/downloader.go`
- `host-cli/internal/downloader/downloader_test.go`
- `host-cli/internal/downloader/cache.go`
- `host-cli/internal/downloader/cache_test.go`

### Depends on

None.

### Invariants

- Incomplete bytes never occupy the final filename, and a failed replacement never destroys a prior valid final.
- A source URL/checksum/validator change invalidates partial recovery; no `206` bytes are appended before all range checks pass.
- Cancellation leaves only recovery state that a later call will revalidate.
- The single fresh retry is limited to recovery from an unprovable `416`; there is no general automatic retry policy.

### Follow this existing pattern

`host-cli/internal/downloader/downloader.go` — retain its context-bound request, chunked progress callback and `Result` boundary while replacing its unsafe in-place append behavior.

### Implementation notes

- Add a context-aware internal hash helper and keep `ComputeSHA256(path)` as the existing public convenience wrapper.
- Save sidecar updates atomically in the destination directory. Treat corrupt or unknown-version sidecars as unusable recovery state and start fresh.
- Preserve `ReuseFromCache` compatibility and add `ReuseFromCacheWithSHA256` for checksum-aware callers.
- Keep `Result.Path`, `SHA256` and `Cached`; `Cached` may represent verified final reuse or local-cache reuse as it does today.

### Edge cases and acceptance

- Tests cover fresh transfer, strong-ETag and Last-Modified resume, checksum-only resume, changed identity, server-ignored Range, malformed/wrong-offset/wrong-length `206`, safe and unsafe `416`, checksum mismatch, response read cancellation, cancellable hashing, completed-artifact reuse after promotion, and `Accept-Encoding: identity`. Test recovery immediately before and after final rename.
- Cache tests cover cancellation mid-copy, checksum mismatch falling back to HTTP, offline reuse under the existing policy, atomic promotion and preservation of an old final on failure, and a rejected cache candidate followed by HTTP resume at the original partial offset.
- Run `cd host-cli && go test -race ./internal/downloader`.

### Do not

- Add background retries, parallel range downloads, content-addressed storage or recipe/update identity to this package.

## Chunk 2 — Incremental apply and durable manifest state

### Goal

Persist every fully completed recipe before announcing success, retain desired selections on interruption, and make init unable to overwrite an existing vault.

### Architecture

Add the optional `SHA256 string` field with YAML key `sha256` to `catalog.Item` and pass it unchanged through `fetchAndRecord` to both cache reuse and HTTP download. It is the expected file checksum, not an acquisition hash.

Preserve the existing `apply.Run` signature as a wrapper and add `RunWithCheckpoint` accepting `CheckpointFunc func(manifest.Manifest) error`; the wrapper delegates with a nil checkpoint. Workers still return complete recipe results and never touch the manifest. The coordinator consumes results as they arrive, stages a manifest snapshot with the recipe's full entry set, sorts realized entries deterministically by ID then relative path, calls the checkpoint with that snapshot, and replaces the live manifest only on success before emitting that recipe's done event. Failed checkpoints leave the live manifest at its last successfully saved state; the final save must not realize rejected snapshots. Stage removals and AppliedAt the same way. A successful builder can return entries for its own ID and selected child package IDs (the Python venv builder already does this); emit done for each newly realized planned ID covered by the successful checkpoint, not just the job ID. Before reporting full success, verify that every requested download ID is realized; return an explicit incomplete error for skipped/unrealized IDs without changing builder selection or add/remove semantics. A multi-platform recipe checkpoints only after every selected platform succeeds; already promoted platform files remain reusable after interruption.

Replace batch collection with a streamed result channel. Job dispatch must select on `ctx.Done()` before scheduling or starting more work; the coordinator drains already-started workers. Missing recipes and recipes without an acquisition strategy join the returned apply error. A failed level prevents later dependency levels, while successful peers already running may finish and checkpoint. `AppliedAt` retains its previous value on any incomplete run and is updated/checkpointed only after every planned operation and runtime-asset generation succeeds.

`commands.ApplyVault` supplies the checkpoint as `manifest.Save(manifestPath, snapshot)`, then makes a final save attempt on success, ordinary error and cancellation. Return `errors.Join` (or equivalent preserving both causes) when apply and save both fail. `manifest.Save` marshals first, creates a same-directory temporary file, writes with the existing 0644 public manifest permissions, syncs, closes and renames it over the manifest, cleaning up the temp on every failure. Both init entry points reject an existing `manifest.yaml` with a clear reopen/resume error before changing it.

### Files

- `host-cli/internal/catalog/catalog.go`
- `host-cli/internal/catalog/catalog_test.go`
- `host-cli/internal/apply/apply.go`
- `host-cli/internal/apply/apply_test.go`
- `host-cli/internal/commands/apply.go`
- `host-cli/internal/commands/apply_test.go` (new)
- `host-cli/internal/commands/init.go`
- `host-cli/internal/commands/init_test.go`
- `host-cli/internal/manifest/manifest.go`
- `host-cli/internal/manifest/manifest_test.go`

### Depends on

Chunk 1.

### Invariants

- A done progress event means the corresponding realized entries are already durably checkpointed.
- Successful peers are saved even when another peer stalls, fails or is cancelled; incomplete recipes never enter realized state.
- Desired items, preset, region and host platforms are never reconstructed during apply and survive every save unchanged.
- Save failure is user-visible and cannot be mistaken for a completed recipe or fully successful apply.

### Follow this existing pattern

`host-cli/internal/apply/apply.go` — preserve its worker/coordinator split and dependency levels; move result consumption and persistence into the coordinator rather than introducing another orchestration layer.

### Implementation notes

- Canonicalize entries at the checkpoint boundary so completion timing cannot reorder the manifest.
- Checkpoint removal state before its done event too; keep existing remove-first behavior and runtime asset generation.
- Avoid holding a mutex across filesystem persistence: only the coordinator mutates the manifest.

### Edge cases and acceptance

- Apply tests cover unknown/no-strategy errors, streamed fast completion while a peer blocks, cancellation before later jobs start, checkpoint failure, multi-platform all-or-nothing realization, checksum propagation, builder child-ID completion events, skipped/unrealized planned IDs returning incomplete errors, and `AppliedAt` only on full success.
- Manifest/init tests prove atomic replacement leaves a loadable old-or-new manifest without stray temp files, and both init APIs preserve an existing manifest byte-for-byte.
- A command-level local-server test performs init → one completion → cancel → reload/status/plan → range resume → success, asserting only remaining work is requested and both apply/save errors are retained. A failed checkpoint followed by final saving must not realize the rejected recipe.
- Run `cd host-cli && go test -race ./internal/downloader ./internal/apply ./internal/commands ./internal/manifest ./internal/catalog`.

### Do not

- Add a journal, database, lock protocol or generalized transaction abstraction; do not change planner/status semantics.

## Chunk 3 — Cancellation-aware CLI and TUI flow

### Goal

Carry cancellation from CLI/TUI entry points through apply, keep consuming progress until saving finishes, and present truthful success or resume states.

### Architecture

Change wizard `ApplyFunc` to accept `context.Context`, matching the plan/dashboard apply boundary. Create the child context and cancel function before scheduling the Bubble Tea start command so cancel-before-start is deterministic. Progress producers send with a context-aware select; completion is delivered on a separate buffered result channel after the event channel closes, so no goroutine depends on an abandoned UI receiver and terminal global errors cannot disappear. The terminal result carries a synchronized snapshot of the latest per-item outcomes recorded before attempted progress delivery; reconcile rows from it after draining. Thus cancelled progress sends cannot hide peer completions that were checkpointed after cancellation.

While apply is active, Ctrl+C sets `Stopping… saving progress`, cancels once and waits for the terminal result before returning `tea.Quit`. Plan Esc/back uses the same state but emits `BackMsg` after completion/save. The wizard orchestrator must forward active-apply Ctrl+C to the apply model instead of quitting at the parent. A finished incomplete wizard apply goes to the dashboard on Enter so the user can inspect/resume; it never advances to indexing.

Both apply views compute completed, failed and pending from the original plan. Success requires no returned error, no failed rows and zero pending rows. Otherwise render all three counts, the terminal/global error even when it has no item ID, and `Reopen this vault and apply the remaining plan to resume.` Preserve per-item errors and the existing progress layout.

Scope `signal.NotifyContext` for interrupt/termination to the noninteractive CLI apply command, deriving from its Cobra command context. Do not install a global signal context that competes with Bubble Tea’s signal lifecycle or exits the TUI before saving. Remove wizard `context.Background()` adapters in CLI root and TUI launch; forward the context supplied by the wizard apply model through `DashboardDeps.RunApply` to `commands.ApplyVault`.

### Files

- `host-tui/internal/wizard/types.go`
- `host-tui/internal/wizard/applymodel.go`
- `host-tui/internal/wizard/applymodel_test.go` (new)
- `host-tui/internal/wizard/model.go`
- `host-tui/internal/wizard/model_test.go`
- `host-tui/internal/plan/model.go`
- `host-tui/internal/plan/model_test.go`
- `host-tui/launch.go`
- `host-tui/launch_test.go` (new adapter tests)
- `host-cli/internal/cli/root.go`
- `host-cli/internal/cli/root_test.go`

### Depends on

Chunk 2. For the Chunk 3 completion pass, the worker owns production .go files and the coordinator owns the listed *_test.go acceptance fixtures; coordinate across that boundary and preserve each other’s edits.

### Invariants

- Navigation and process exit occur only after `commands.ApplyVault` has returned from its final save attempt.
- Cancelling before the start command runs still reaches apply as an already-cancelled context.
- No closed/abandoned progress consumer can strand an apply goroutine.
- Incomplete apply never renders success or starts indexing.

### Follow this existing pattern

`host-tui/internal/plan/model.go` — keep its Bubble Tea start/tick model, per-item progress projection and parent `BackMsg`; extend its state machine with stopping intent and terminal result handling, then mirror that contract in the wizard apply model.

### Implementation notes

- Store exit intent (`back` or `quit`) separately from terminal outcome; repeated cancel keys are idempotent.
- Keep the actual returned apply/save error as model state instead of relying on a synthetic item event.
- Preserve Enter-after-success behavior. One required repair to the empty-plan state: when RunApply is available, Enter starts even an empty apply to finalize/repair runtime assets; existing b opens Browse, and nil-callback empty plans keep their current browse behavior. Render applying before the empty-plan view check so zero-row finalization displays its progress/error. Do not claim runtime health from zero pending recipe IDs; label the empty view "No recipe downloads or removals pending" with apply/finalize and browse hints.

### Edge cases and acceptance

- Model tests cover cancel before start, cancel while blocked, repeated cancel, back/quit emitted only after completion, empty-ID/global errors, pending counts, failed Enter returning to dashboard, and indexing skipped after incomplete apply. Empty-plan Enter invokes RunApply and surfaces finalization success/error; b still browses. An already-started peer checkpointed after cancellation must still count as completed in the terminal outcome.
- Adapter tests verify the exact UI-created context reaches `commands.ApplyVault`; CLI command tests verify a cancelled command context returns cancellation after checkpointing.
- Run `cd host-tui && go test -race ./...`, `cd host-cli && go test -race ./internal/cli ./cmd/svalbard`, then the whole-feature verification commands.

### Do not

- Add asynchronous background continuation, a new screen, automatic restart, new key bindings or changes to indexing itself.

## Final review

Review correctness and design in one pass against the scoped pre-edit diff, covering interactions across all chunks. Read both review briefs provided in context. Owned source/test files are listed in .work/scoped-files.json; do not review unrelated Claude work.

Completed. All three final review findings were reproduced with regression fixtures and fixed. Focused host race tests, the full TUI race suite, `make verify` and `make build` passed after the fixes. No unresolved findings remain in this scope; physical-stick and power-loss testing were not performed.
