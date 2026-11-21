# Initial deployment and interruption recovery

Status: implemented and verified 2026-09-30; see [checkpoint](2026-09-30-deployment-resume-checkpoint.md). Existing UI/navigation preserved while repairing the authorized deployment/cancel/restart path.

## Goal

Create a vault from a selected/customized preset, save those choices, download its contents, stop midway and reopen the same vault to finish safely. Completed items survive failures and process interruption. Incomplete files never look like installed content. The existing manifest remains the source of desired/realized state.

## Decisions

- Keep the existing init wizard, dashboard, plan and CLI apply/status commands. Resume by opening the existing vault and applying its remaining plan, not by running init over it.
- Init rejects an existing manifest rather than resetting selected items, options or realized state.
- HTTP downloader owns partial-file and remote-representation identity. It writes a sibling `.part` and small download metadata sidecar; only a completed, validated transfer becomes the final file. This metadata is transfer recovery, not the Phase 7 recipe hash.
- Resume requires the same source URL and expected checksum, plus a trusted representation validator (strong ETag, or an RFC-valid strong Last-Modified date) or a pinned checksum that will validate the whole result. Without safe identity, restart the transfer rather than append unproven bytes. Unknown old partials under final names are not trusted; valid checksum-verified final files remain reusable.
- Validate HTTP 206 range offset/bounds/length before combining bytes. HTTP 200 replaces the partial safely. A 416 alone never proves completion: require matching size and identity/checksum, otherwise use one bounded fresh restart or return a retryable error. Request identity encoding so Range counts stored bytes. No automatic unbounded retry/backoff machinery.
- Persist verified download identity/hash so a crash between final-file promotion and manifest checkpoint can reuse a completed artifact. Validate reused bytes before reporting success; avoid downloading the whole completed artifact again.
- Existing recipe `sha256` values must reach the downloader/cache verification path when present. Add only that optional catalog field and checksum plumbing; no acquisition-hash feature or new checksum policy. A checksum-incompatible local cache candidate falls back to download without replacing a valid prior final file.
- The apply coordinator owns result ordering and invokes a persistence callback as each complete recipe finishes. Commands own manifest persistence. Checkpoint before reporting an item complete. Workers never mutate the manifest directly.
- Save the manifest by same-directory temporary file, sync/close and rename. Save completed results on ordinary errors/cancellation too. Surface both apply and save errors. AppliedAt means a fully successful apply, not an incomplete run.
- Keep Ctrl+C's existing exit meaning: cancel active work, show stopping/saving, drain completion and then exit. Existing plan Esc/back cancels and returns only after completion is saved. An optional async preference was requested; if the user steers differently, amend the UI chunk before dependent edits.
- Empty recipe plans retain Enter to apply/finalize when the apply callback is available, with existing b for Browse. Runtime generation can fail after all recipe IDs are checkpointed, so zero pending recipes alone does not prove the vault runtime is ready. No new persisted state or key bindings.
- Wizard apply errors must appear even when the error has no item ID. Count pending work; do not show success or automatically start indexing after incomplete apply. Enter from a failed completed attempt can open the dashboard to inspect/resume.

## Invariants

- Selected IDs, preset, region and platform choices survive stop/restart unchanged.
- Only fully completed recipes enter realized state; completed platform artifacts within an interrupted multi-platform recipe remain reusable, but the recipe does not become realized prematurely.
- Successful completions are checkpointed as they arrive even while another item in the same batch is stalled.
- Context cancellation stops HTTP transfers, local cache copies and further scheduled acquisitions. Hashing/checking large files is cancellable too.
- Producer progress sends cannot leave an abandoned TUI goroutine blocked. UI exit/navigation waits for the apply worker to finish saving.
- CLI `apply` observes interrupt/termination signals through a cancellable command context. Do not replace TUI cleanup with immediate Bubble Tea program cancellation; Ctrl+C in the terminal is handled by the active model and waits for saved completion.
- Cancellation/errors leave valid prior final files intact and retain safely resumable partials. Wrong checksums or invalid ranges never produce realized content.
- One writer applies a vault at a time. This phase does not introduce a distributed lock, database, journal or concurrent mutation protocol.

## Existing architecture and concrete gaps

- `host-cli/internal/downloader/downloader.go` writes directly to final filenames, appends any 206 without range validation, treats all 416 responses as complete, and deletes checksum-mismatched partials before trying resume.
- `downloader/cache.go` copies under final names and does not observe cancellation during copying. Legacy offline cache trust remains the existing policy, but copies must be cancellable and finalized safely; downloader expected checksums must be honored when supplied.
- `apply/apply.go` collects a whole batch before accepting successes. Unknown/no-strategy recipes emit failure events but can return overall success. `commands/apply.go` saves only after total success.
- `manifest.Save` currently truncates the only manifest. Init unconditionally overwrites it.
- `host-tui/internal/wizard` apply uses a callback without context, discards global errors, and forwards any completion to indexing. Both `host-cli/internal/cli/root.go` and `host-tui/launch.go` wire wizard apply with background contexts.
- Existing plan apply has a context, but exits/navigates immediately after cancel and uses unguarded channel sends. CLI main has no signal-derived command context.
- Planner/status already derive remaining IDs from desired minus realized: reuse them. File-based custom presets were implemented by the concurrent Claude session; preserve them.

## UI

Preserve current layout and bindings. Add an in-place stopping state, not a new screen:

```text
Downloading & applying
  ✓ completed-item
  ↓ current-item       84 MB / 200 MB
  · remaining-item

Stopping… saving progress
```

On incomplete completion, show completed/failed/pending counts plus the actual error and a short resume hint. Success appears only when no work failed/remains. Ctrl+C exits after cleanup; existing back navigation returns after cleanup. Reopen the same vault → dashboard → Plan → apply remaining items. No new wizard or web UI.

## Non-goals

- Acquisition hashes, automatic upstream version discovery, changes to preset composition/TUI customization, new content/model choices, Python/PIO builder design, indexing lifecycle, managed-removal redesign or new pack actions.
- Power-loss-proof storage guarantees, safe physical removal while writing, multiple simultaneous apply processes, automatic retries of every failure or actual bulk public downloads during tests.
- A general transfer framework or persistent failed-task database. Failed/cancelled items remain pending in the existing manifest.

## Acceptance

Use deterministic local HTTP fixtures to initialize a selected vault, complete one item while another is blocked, cancel, reload manifest/status/plan, resume only remaining bytes and finish. Cover source changes, server ignores Range, malformed 206, unsafe 416, expected checksum, cached-copy cancellation, successful artifact reuse, same-directory manifest replacement, no init overwrite, global errors and UI cancel-before-start/exit-after-save. Run relevant race tests, full `make verify` and `make build`; record hardware/physical-stick limitations.

Discovery baseline: existing downloader/apply/commands/manifest and wizard/plan suites pass. Disposable reproductions in `/private/tmp/svalbard-phase2-spike/` fail as expected: `TestUnproven416MustNotSucceed` accepts unknown bytes as complete; `TestCancelledPartialMustNotUseFinalName` finds the interrupted output under its final `.zim` name. No production code was changed for these probes.

HTTP protocol basis: [RFC 9110 Range and validators](https://www.rfc-editor.org/rfc/rfc9110.html#section-14), [If-Range](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.5).

## Workspace rules

This is the already authorized shared dirty tree. Do not reset, stash, stage or commit any changes, edit Git metadata, or invent a clean base. Preserve Claude session e58429bd's work. Scoped pre-edit copies are under `/private/tmp/svalbard-phase2-baseline/`; reviews compare owned files against those copies. `.work/` holds this run's local state without a fabricated base_ref.
