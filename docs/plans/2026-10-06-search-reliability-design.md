# Search and indexing reliability

Status: design settled 2026-10-06 within the authorized Phase 4 repair scope; recommended reversible provisioning default used after optional preference question.

Implementation verified 2026-10-07 → [checkpoint](2026-10-06-search-reliability-checkpoint.md).

## Goal
Make the existing ZIM keyword/hybrid search usable from the vault, with truthful indexing state, safe incremental refresh and interruption recovery, and compatible embeddings shared by native search and MCP.

## Decisions
- Keep the existing host-owned `data/search.db` and drive runtime readers. No second search service/database or new dependency.
- Preserve current keyword/hybrid navigation and index management screens; repair lifecycle and completion/error reporting in place.
- Preserve the shared uncommitted Phase 2/3 changes. No staging, commits, Git metadata changes or worktree creation. Scope baselines use copied source snapshots.
- Provisioning defaults to automatic keyword indexing; semantic indexing remains an explicit optional operation. The preference was asked asynchronously; proceeding with the recommendation after a reasonable response window, subject to user steering. Honor explicit index_strategy settings including disabled indexing.

## Invariants
- A source is marked current only after successful atomic article replacement; failed/cancelled refresh must remain retryable.
- Removed archives do not keep appearing as search results after a successful refresh.
- Query and document vectors must share model identity, prefixes and compatible dimensions. Incompatible or unavailable embeddings fall back to keyword search.
- Cancellation propagates to extraction, indexing and embedding operations; quitting does not strand a producer on an abandoned progress channel.
- Saved desired.items remains authoritative; acquisition success remains durable even if subsequent indexing fails.
- No unsupported claims about physical drives, power loss, untested platforms or actual pinned binaries.

## Relevant existing architecture
- `host-cli/internal/commands/index.go` scans ZIMs; currently skips by filename, has no context, and deletes articles before replacement extraction.
- `host-cli/internal/searchdb/db.go` owns FTS5, articles, chunks/vectors and metadata.
- `host-cli/internal/commands/index_semantic.go` owns embedding ingestion; records contract metadata only after whole-job success.
- `host-cli/internal/commands/apply.go` owns reconciliation/checkpoint saving; currently does not consume desired.options.index_strategy.
- `host-cli/internal/cli/root.go` wires host TUI and CLI index operations; wizard receives keyword-only callback.
- `host-tui/internal/wizard/indexmodel.go` and `host-tui/internal/index/model.go` own indexing progress lifecycle.
- `drive-runtime/internal/search/engine` shares FTS/vector search with native sessions and MCP.
- `drive-runtime/internal/search/server` owns embedding/Kiwix subprocess startup. Runtime and MCP currently select the first model despite recorded identity.

## Non-goals
- Non-ZIM PDF/EPUB ingestion/document RAG, new retrieval backend or ANN database.
- Recipe acquisition tracking, installed-item editing, PlatformIO, broader pack actions or web UI.
- Silent downloads/model installation, automatically switching installed models, or redesigning presets.
- Semantic recall/ranking redesign without a bounded corpus/performance decision; candidate filtering limitation must remain disclosed unless separately agreed.

## UI
Keep the existing flow: preset → picker → review → apply → indexing progress → dashboard.
Keep existing index management: Full index / Keyword only → progress → return.
Recovery changes present the actual failed/cancelled outcome rather than success, following the progress states below; no new screen.

## Acceptance
- Real ZIM fixtures cover create → keyword query/read, unchanged skip, same-name change, removal and failure/retry.
- Cancellation fixtures cover both command and UI producer lifecycle.
- Embedding fixtures cover recorded-model selection, dimensions/truncation, prefix/model change, partial interruption and keyword fallback in native/MCP paths.
- Rebuild packaged runtime assets and pass an actual current-platform pinned llama/model/native/MCP journey before marking Phase 4 done. Other supported targets remain explicitly unexecuted.
- Final uncached repository verification, affected race suites, build and diff whitespace checks.

## Open questions
- None; user can steer the optional provisioning preference while planning.

## Provisioning and recovery shape
- Shared apply remains acquisition-only. CLI runs selected indexing afterward; dashboard emits a success transition into existing indexing progress; wizard retains its dedicated indexing step. Use one host selected-index runner and avoid double indexing.
- Empty legacy strategy means keyword; `full-text` means keyword; `none` disables. Semantic/full requests run keyword first, then semantic; unavailable semantic must be reported without discarding keyword results. Unknown nonempty strategies fail clearly, not silently default.
- Progress stays on existing screens. Esc cancels indexing and waits for producer completion before continuing; Ctrl-C cancels/quits without blocked progress sends. Failed/cancelled indexing never renders a successful completion message. Successful artifact installation remains saved.

Existing progress states (repair in place):

```text
Building search index
  archive.zim    indexing / done / skipped / failed
  [completion: finished / cancelled / failed, with detail]
  Enter: continue to dashboard
  Esc: cancel indexing (while running)
```

- Archive refresh compares cheap source identity (ZIM UUID plus file size and nanosecond mtime) and rechecks identity before commit. This is cache invalidation, not full byte-integrity verification or the later recipe acquisition hash. Legacy sources with no identity are refreshed once. Store identity with the source replacement transaction, never before extraction succeeds.
- Keep the FTS/vector candidate policy unchanged in this reliability phase; disclose its semantic recall limitation. Full-corpus vector retrieval/performance changes belong to a separate follow-up rather than introducing an unbounded scan for large vaults.

## Embedding contract and interrupted builds
- Start and health-check the embedding backend before replacing a working semantic contract. A startup/preflight failure preserves previously completed embeddings.
- On successful startup, publish the new contract and clear incompatible vectors atomically before the first new batch. Resume matching partial builds without force-clearing completed batches. Completion is published only after all indexed articles are covered.
- Native search and MCP use keyword-only mode while the semantic state is building/incomplete, including older indexes missing a reliable compatibility contract. Retain batches for resume; do not silently mix models or claim complete semantic coverage.
- Contract covers selected exact model filename/file identity, document/query prefixes, effective vector dimension and permitted Matryoshka truncation. Runtime query shaping follows the document vector rule: compatible truncation and normalization, or clear fallback.
- Validate response counts and vector dimensions before persistence/scoring; malformed/short responses return diagnostics rather than panic or silently score unequal vectors.
- Discovery also found legacy embeddings foreign keys without cascade. Source replacement and pruning explicitly delete affected embeddings inside the same transaction as articles/source changes; no mandatory destructive migration.

## Discovery evidence
[Read-only audit](../../.work/handoffs/search-discovery.md) confirms the repair seams. Baseline host searchdb/commands/embedder and runtime search/engine/server/MCP suites pass with local fixture listeners enabled. Existing tests passing do not cover the stale-source/model/lifecycle gaps. The external Claude design dispatch was rejected by automatic approval review; native adversarial review is used instead.

## Design review resolutions
Native adversarial review raised five concrete issues; the implementation plan must honor these clarifications:
- Semantic generation is `absent/building/complete`, keyed by one compatibility contract. Source replacement marks existing semantics building in the same transaction when vectors are removed/new articles added. Removal-only pruning may retain complete for surviving articles. Runtime enables hybrid only for complete.
- Model identity is the SHA-256 of the actual GGUF bytes, checked on indexing and once when a runtime reader opens its contract. Contract fields include exact relative filename, prefixes, effective dimensions, permitted truncation and a small compatibility format marker. This is embedding compatibility, separate from recipe acquisition hashes and cheap archive cache invalidation. Do not trust an old manifest checksum instead of current bytes.
- Keep shared `RunApply` and `commands.ApplyVault` acquisition/checkpoint primitives index-free. CLI sequences apply then selected indexing. Wizard retains apply → indexing. Dashboard plan's successful apply emits a distinct completion transition to the existing indexing progress screen, using the saved strategy. Avoid duplicate indexing and avoid hiding index events among recipe progress IDs. Disabled strategy goes straight to dashboard.
- Serialize writers per vault with a recoverable OS-backed exclusive lock shared by keyword, semantic and combined indexing entrypoints. Conflicting jobs fail clearly; runtime readers remain unlocked. A combined operation holds one lock over both stages; avoid nested locking. Kernel release on exit removes stale-process concerns.
- One actual current-platform provision → FTS → semantic → packaged native hybrid → MCP search/read journey is a completion gate. Other platforms/hardware can remain disclosed limits. Pinned artifacts have now been normally provisioned under `/private/tmp/svalbard-phase4-live-vault`, so the native gate is feasible.

## Plan review resolutions
- Readers remain unlocked but every hybrid score uses one SQLite read snapshot whose complete contract ID matches the one used to embed the query. Refresh the contract before embedding; recheck inside the scoring transaction. Generation change means discard/fallback, never mixed scoring.
- Introduce completeness state with source invalidation in Chunk 1 and gate native/MCP partial readers with Chunk 2. Exact model/lifecycle compatibility follows in Chunk 4.
- Preserve MCP JSON array as the first content block. Append an advisory text block for fallback status using the existing ActionResult.Text field; do not alter result item fields or add a JSON envelope.
- Backend lifetime is independent of request lifetime. Startup polling and HTTP honor request cancellation; exactly one waiter reaps child exit so later queries can restart. Completed request cancellation must not kill a retained healthy backend.
