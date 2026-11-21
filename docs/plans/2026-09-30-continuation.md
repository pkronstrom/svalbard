# Continuation checkpoint

Later concurrent work: [Claude session coordination snapshot](2026-09-30-claude-session-coordination.md) supersedes the model/MTP/provider and Python/live-inference limitations recorded below. That session is actively completing Phase 1b; consult its current records before editing overlapping files.

Status: Phase 1 catalog/runtime refresh complete and verified on 2026-09-30. Final fix review has zero current findings. Changes remain uncommitted. No disk deployment or tools-image publication was performed.

## Start here

Open work is in the task tracker (Aven project `svalbard`; `aven list --ready`). Background: [refresh results](2026-09-30-catalog-refresh-results.md), the [catalog audit](2026-09-30-catalog-update-audit.md), [Dodo ledger](2026-09-30-dodo-triage.md) and [runtime/backlog audit](2026-09-30-runtime-backlog-audit.md).

The user prioritized full user paths: initial deployment/resume, preset customization, operation of every pack from the stick, installed-recipe updates, and add/remove. Minimal configuration web UI remains queued for design. Phase 1b records remaining catalog curation and operability fixes; do not repeat its completed model/pin work.

## Completed this continuation

Tool/harness pins refreshed; Pi 0.99.1, OpenCode 1.18.33, Goose 1.52.0 and llama.cpp b11146 native binaries verified. GGUF RAM tiers now use Qwen3.5 4B/9B, Gemma 4 12B QAT and Qwen3.8 27B IQ4/Q6 alongside retained Gemma tiers/Qwen3.6 MoE. Refreshed model URLs are revision-pinned; Nomic v1.5 remains the embedding default. Optional matching MTP drafters install separately from selectable chat models.

Runtime flags, context defaults and Pi sampling/thinking/compaction settings implemented with behavioral coverage. Actual native Pi loading exposed missing cache-cost schema fields, now fixed and verified. Embedded catalog synchronized; temporary smoke helper removed. README links the refreshed lineup. See refresh results for evidence and exceptions.

Review corrections also verified actual Pi off/medium/high requests, context-switch compaction updates, missing-field initialization in partial custom settings, and field-level preservation of user overrides. The final [review record](2026-09-30-catalog-refresh-review.md) reports zero current findings. Concurrent custom-preset callers/tests were updated by the other work; the final whole-repository verification passes after those edits.

Full `make verify` passes (local HTTP fixtures require loopback binding permission). Host/drive builds pass. 145 acquisition URLs checked successfully. Native llama.cpp + pinned Nomic returns two 768-dimensional vectors. Large chat/MTP inference and actual harness tool-call conversations were not run. Python provisioning, standalone DuckDB WASM use, offline PIO builds and remaining pack paths are not completed by the pin refresh.

## Backlog state

Dodo cleanup is complete: all 39 original IDs preserved, 4 clearly completed tasks closed, 11 narrowed to remaining scope, all 39 linked through metadata to masterplan groups. Do not rerun cleanup or mark partial features complete.

The agreed acquisition-hash feature remains Phase 7: compute on recipe load, store `acquisition_hash` only after successful installation, compare in plan/status/apply, absent means unknown. No schema-migration framework or provenance graph. Existing vaults currently do not detect a new acquisition pin merely from the same recipe ID.

## Open questions

- Bootable media and optional apps/maps: wishlist, implementation queue or archive?
- Mobile/KiCad: guides or offline installers/APKs?
- What source does NODE PDF links mean, and what is AutoDoc's intended scope?
- Pi barebones third-party plugins remain a design discussion; native MCP already exists.

Do not reask the obsolete phase-order question: the later user-path priorities supersede it. Parakeet v3/Whisper Turbo are not bundled; ASR is separate scope.

## Workspace preservation

The workspace contains concurrent content, preset, builder, resolver and README edits. Preserve them; do not reset/stage/commit unrelated work. No clean baseline for a new `/work` run was established. Inspection and catalog synchronization included those changes; this does not make all their planned follow-ups complete.

Temporary evidence under `/private/tmp/svalbard-support/`, `/private/tmp/svalbard-release-audit.json`, `/private/tmp/svalbard-current-link-results.json`, `/private/tmp/svalbard-smoke/` and runtime/review handoffs may disappear. Durable results and source links are in the repository. Reverify upstream evidence if scratch files are absent. Local TurboLLM was read-only; its hardware-specific MTP results were not generalized.
