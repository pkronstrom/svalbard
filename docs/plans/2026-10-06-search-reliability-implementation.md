# Search reliability and usable indexes from the stick — implementation plan

Status: done 2026-10-07; final review and verification complete. See [checkpoint](2026-10-06-search-reliability-checkpoint.md).

## Goal

Make the existing on-drive ZIM keyword and hybrid search reliable: provisioning builds the selected index, source changes reconcile safely, interrupted work resumes truthfully, and native/MCP semantic queries use the exact completed embedding contract.

Design: [Search and indexing reliability](2026-10-06-search-reliability-design.md)

## Chunks

1. **Atomic keyword reconciliation** — add source identity, cancellable extraction, transactional replacement, and stale-source pruning.
2. **Resumable semantic contract** — publish compatible embedding metadata atomically, validate vectors, and resume incomplete batches.
3. **Provisioning and progress lifecycle** — honor manifest strategy after durable apply and make both existing progress screens report cancellation/failure truthfully.
4. **Shared runtime/MCP compatibility and packaged journey** — use one semantic backend contract, enforce dimensions, rebuild runtime assets, and exercise the real provisioned vault where available.

Chunks normally run sequentially because each later chunk consumes earlier contracts. After Chunk 3 host runner and actual cancellation/resume pass, its remaining TUI-only fixes and Chunk 4 runtime-only implementation may run independently in disjoint files; final whole-feature review covers their integration. One worker owns each chunk through local verification; a fresh plan review precedes execution and fresh correctness/design review follows all chunks.

Final Chunk 4 acceptance tests can finish in parallel with disjoint ownership: consumer fixtures own native/MCP tests except `semantic_timeout_test.go` and `search/server` tests; the timeout worker owns only those latter tests. Coordinator owns the settled production fixes and final integration verification.

## Whole-feature acceptance

- A real small ZIM covers first build, keyword query/read, unchanged skip, same-name replacement, deletion, extraction failure, cancellation, and retry without losing the last successful source.
- Empty and legacy `index_strategy` values build keyword search after successful apply; `none` disables indexing; semantic/full builds keyword then semantic; unknown nonempty values fail clearly.
- Interrupted semantic work retains completed batches but remains keyword-only until every indexed article is covered by one exact, completed contract.
- Native search and MCP select the recorded GGUF, apply the recorded query prefix and permitted dimension shaping, and visibly fall back to keyword for missing, incomplete, malformed, or incompatible semantic state.
- Existing wizard and index-management navigation remains intact; Esc waits for cancellation, Ctrl-C cannot strand a producer, and failed/cancelled work never displays success.
- `make build-drive-runtime`, `make build`, uncached `make verify`, affected race suites, and `git diff --check` pass. One current-platform actual pinned-artifact journey is mandatory; unsupported platforms remain explicit limitations. Phase 4 stays unchecked if the native gate fails.

## Non-goals

- Non-ZIM ingestion/RAG, a new search service/database, ANN retrieval, ranking or candidate-policy redesign.
- Recipe acquisition hashes, installed-item editing, PlatformIO, broader pack actions, preset redesign, or silent model downloads/switching.
- Physical-stick, power-loss, or unsupported-platform claims without direct evidence.

## Workspace and verification policy

All pre-existing Phase 2/3 changes remain untouched. No staging, commits, Git metadata changes or worktrees. Review source deltas against /private/tmp/svalbard-phase4-baseline using /private/tmp/svalbard-phase4-diff.py. Workers are not alone in this shared tree and must not revert other edits. Use GOCACHE=/private/tmp/svalbard-go-cache; local HTTP/subprocess fixtures may need approved sandbox escalation. Keep docs paths stable rather than archive moves. Coordinator owns docs/state and final native smoke.

## Things not to redesign

- Keep `data/search.db`, FTS5, current hybrid ranking/candidate filtering, and the existing native/MCP search/read result shapes.
- Keep `desired.items` authoritative and acquisition checkpoints durable independently of later indexing failure.
- Keep the wizard path and the Full index / Keyword only management choices; add states and wiring inside those screens rather than new screens.

## Chunk 1 — Atomic keyword reconciliation

### Goal

Make an ordinary keyword refresh detect changes and removals while preserving the last successful searchable source across failure or cancellation.

### Architecture

`commands.IndexVaultContext` owns the cancellable scan/reconcile loop. Keep existing `IndexVault` as a compatibility wrapper with background context so Chunk 1 leaves all callers building. `zimext` exposes cancellable extraction plus the archive UUID needed for identity. `searchdb.DB` owns all source mutations: replacement and pruning explicitly delete vectors before articles so both fresh and legacy foreign-key shapes work.

### Files

- `host-cli/internal/searchdb/db.go`, `host-cli/internal/searchdb/db_test.go`
- `host-cli/internal/zimext/extractor.go`, `host-cli/internal/zimext/extractor_test.go`
- `host-cli/internal/commands/index.go`, `host-cli/internal/commands/index_test.go`
- `host-cli/internal/commands/index_lock.go`, `host-cli/internal/commands/index_lock_test.go`
- `host-cli/internal/commands/index_semantic.go`, `host-cli/internal/commands/index_semantic_test.go` (Chunk 1 fix: public lock wrapper only; full contract stays Chunk 2)

### Depends on

None.

### Invariants

- Source identity is ZIM UUID + file size + nanosecond mtime; nullable legacy identity is dirty once, not current.
- Extraction completes before a replacement transaction starts. Re-read the identity immediately before commit; a changed file fails retryably.
- Replacement commits source row, identity, article deletion/insertion, and FTS trigger effects together. It explicitly deletes affected embeddings first and never relies on cascade.
- Successful reconciliation prunes every indexed filename absent from `zim/`, including an empty directory. A missing/unreadable directory is an error and preserves the database.
- Adding or replacing articles invalidates semantic `complete` state until those articles receive compatible vectors. Removal-only pruning may remain complete when every surviving article is still covered.
- Cancellation propagates into DB BeginTx/QueryContext/ExecContext/PrepareContext and is checked before commit and after pruning, before advancing indexed_at. Cancellation during replacement/pruning rolls back; add a deterministic in-transaction cancellation fixture.
- Public keyword AND semantic entrypoints acquire the same lock in Chunk 1, not only the lock primitive; test actual entrypoint contention with no loser mutation.

### Follow this existing pattern

`host-cli/internal/searchdb/db.go` — keep transaction ownership and error wrapping beside the existing transactional insert helpers; do not put SQL orchestration in the command.

### Implementation notes

- Introduce the semantic state metadata helper needed to invalidate complete→building with source replacement in this chunk; Chunk 2 expands it into the full contract.
- Add nullable identity columns with additive `pragma_table_info` migrations; `search.db` stays rebuildable and no destructive migration is required.
- Delete production UpsertSource/InsertArticles/DeleteSourceArticles now that no production callers remain. Seed tests through safe ReplaceSource or explicit legacy-only test SQL.
- Replace filename-only APIs with source records carrying identity. Provide transactional `ReplaceSource` and `PruneSources` operations; keep article insertion private to their transaction path where practical.
- Add cancellable extraction returning the opened archive UUID with title/articles, retaining existing `ExtractArticles` compatibility wrapper. Use `Archive.UUID()` rather than a full-file hash.
- Later orchestration calls the cancellable sibling with its existing context. `force` means rebuild present sources, but still preserves old data until replacement commit.

- Add an OS-backed exclusive per-vault index writer lock using existing golang.org/x/sys/unix; no new dependency. Public keyword and later semantic entrypoints acquire it; internal unlocked helpers allow one combined job to hold it over both stages. Contention fails promptly; cancellation/error/exit releases the lock. Put lock data in the vault internal directory, not Git.

**Edge cases:** Writer contention; Same filename with changed UUID, size, or mtime; mtime-only change; file mutation during extraction; corrupt replacement; new source after a complete semantic build; removed last archive; legacy row with null identity; migrated embeddings table without cascade.

### Acceptance

- Search DB tests prove replacement/pruning are atomic and work against the legacy no-cascade embeddings schema.
- Cancellation fixtures use a real cancellable context and SQLite test callbacks/barriers, not a synthetic Err call counter. Final indexed_at uses a context-aware write and is covered at its write boundary. Run affected race tests.
- Contention test proves a second writer cannot mutate state while the first holds the lock.
- Real `small.zim` tests prove create/query, unchanged skip, same-name refresh, removal, failure preservation, identity recheck, cancellation/retry, and complete-semantic invalidation on add/change but not covered removal-only pruning.
- `go test ./internal/searchdb ./internal/zimext ./internal/commands` passes in `host-cli`.

**Do not:** Reuse acquisition checksums, hash whole archives, change search result schemas, or prune after a failed scan.

## Chunk 2 — Resumable semantic contract

### Goal

Make semantic indexing resumable and prevent partial or incompatible vectors from being advertised as usable.

### Architecture

`index_semantic.go` computes the contract and drives ingestion; `searchdb.DB` atomically activates a new building contract with vector invalidation, then publishes completion only after coverage checks. The embedder validates server response cardinality before the command maps responses to chunks.

### Files

- `host-cli/internal/searchdb/db.go`, `host-cli/internal/searchdb/db_test.go`
- `host-cli/internal/embedder/embedder.go`, `host-cli/internal/embedder/embedder_test.go`, `host-cli/internal/embedder/embedder_context_test.go`
- `host-cli/internal/commands/index_semantic.go`, `host-cli/internal/commands/index_semantic_test.go`
- `drive-runtime/internal/search/search.go`, `drive-runtime/internal/search/search_test.go`
- `drive-runtime/internal/search/session.go`, `drive-runtime/internal/search/session_test.go`, `drive-runtime/internal/search/session_contract_test.go`
- `drive-runtime/internal/search/semantic.go`, `drive-runtime/internal/search/semantic_test.go` (shared contract loader and snapshot gate moved forward from Chunk 4)
- `drive-runtime/internal/search/engine/engine.go`, `drive-runtime/internal/search/engine/engine_test.go` (DB/transaction query interface moved forward from Chunk 4)
- `drive-runtime/internal/mcp/search.go`, `drive-runtime/internal/mcp/search_internal_test.go`

### Depends on

Chunk 1.

### Invariants

- The contract contains exact relative GGUF filename, SHA-256 of its current bytes (not a blindly reused manifest hash), document/query prefixes, effective dimensions, Matryoshka truncation permission and a small compatibility format marker. Runtime readers later validate the hash once per opened contract. State is absent/building/complete.
- Public semantic entrypoint uses the Chunk 1 writer lock; combined runner uses its internal unlocked sibling.
- Preflight and backend health succeed before a working contract is invalidated. A new/incompatible contract and vector clearing commit atomically with state `building` before its first vector batch.
- A matching `building` contract resumes without clearing completed batches. State becomes `complete` only when every indexed source has no article missing embeddings.
- Batch persistence writes all chunks for each included article transactionally. Coverage counts distinct article IDs, never raw chunk rows; a partial vector response writes nothing.
- Model/prefix/dimension change, forced rebuild, or keyword source replacement cannot mix contracts. Cancellation leaves resumable `building` state and keyword search remains available.

### Follow this existing pattern

`host-cli/internal/commands/index_semantic.go` — retain its source-by-source, batch-transaction pipeline and context propagation; repair contract setup and completion around it rather than replacing ingestion.

### Implementation notes

- Move the planned shared contract loader and read-snapshot generation gate into this chunk: load a complete contract before embedding, recheck that same generation/state in one read transaction afterward, and perform every hybrid keyword/vector/detail read through that transaction. Native and MCP call one shared seam. Existing backend caches must not embed with metadata from a different generation; conservatively fall back until Chunk 4 supplies contract-aware backend refresh. Update affected legacy fixtures to assert keyword fallback; exact model/hash/shaping/backend lifecycle follows in Chunk 4.
- Add typed search DB helpers for loading, beginning, and completing the semantic contract; do not scatter raw meta-key reads/writes through callers.
- Always probe native dimensions before invalidation, validate configured positive dimensions against native output and require Matryoshka permission for truncation. Resolve effective dimensions before contract comparison. Preserve existing preflight rules, truncation/normalization, context-limit shrink retry, and sidecar semantics.
- Validate one response vector per requested text, consistent native dimensions, and configured truncation bounds before indexing response slices or persisting blobs.
- Complete publication verifies coverage and writes all still-required legacy model/prefix/dimension metadata, timestamp and state together in one cancellable transaction. Matching completion is idempotent; a no-work building retry must still publish complete.
- Hash actual GGUF bytes using bounded streaming with context checks, without model-sized allocation.
- Ordinary retry uses resume semantics. Reserve force for explicit rebuild while still waiting for a healthy backend before invalidation.

**Edge cases:** Legacy vectors with no complete contract; same basename with replaced model bytes; changed sidecar prefix/dims; startup failure with an old complete index; cancellation after some committed batches; multiple chunks per article; short/empty/mixed-dimension responses.

### Acceptance

- Command-level lifecycle fixtures exercise startup failure, invalid dimension preservation, cancellation after committed batches and ordinary retry including the last-batch completion gap; DB cancellation fixtures prove atomic metadata publication.
- Tests prove startup failure preserves the previous complete contract; incompatible restart atomically clears vectors and publishes `building`; matching retry resumes; completion requires all articles.
- Embedder fixtures return short and mixed-dimension responses as diagnostics without panic or partial writes.
- `go test ./internal/searchdb ./internal/embedder ./internal/commands` and affected host race tests pass.

**Do not:** Use partial semantic results, mark a source complete from chunk-row count, change chunking/ranking, or download/select another model silently.

## Chunk 3 — Provisioning and progress lifecycle

### Goal

Run the manifest-selected indexing operation after durable acquisition and make the existing wizard and management screens wait for a truthful terminal outcome.

### Architecture

Host commands expose one strategy parser/runner used by CLI apply, dashboard apply, and wizard indexing. `ApplyVault` remains acquisition/checkpoint ownership. Dashboard/CLI orchestration invokes indexing afterward; the wizard invokes acquisition-only and then the same configured-index runner in its dedicated step, preventing double indexing.

### Files

- `host-cli/internal/embedder/embedder.go`, `host-cli/internal/embedder/embedder_context_test.go` (actual cancellation smoke found concurrent double Wait; one waiter owns exit/reaping, Stop waits on its completion)
- `host-cli/internal/commands/apply.go`, `host-cli/internal/commands/apply_test.go`
- `host-cli/internal/commands/index.go` (extract existing unlocked helper for combined lock; no behavior redesign)
- `host-cli/internal/commands/index_configured.go`, `host-cli/internal/commands/index_configured_test.go`
- `host-cli/internal/cli/root.go`, `host-cli/internal/cli/root_test.go`
- `host-tui/dashboarddeps.go`, `host-tui/launch.go`, `host-tui/launch_test.go`
- `host-tui/internal/plan/model.go`, `host-tui/internal/plan/model_test.go`
- `host-tui/internal/wizard/indexmodel.go`, `host-tui/internal/wizard/indexmodel_test.go`, `host-tui/internal/wizard/model.go`, `host-tui/internal/wizard/model_test.go`, `host-tui/internal/wizard/view.go`, `host-tui/internal/wizard/types.go` (existing ShellLayout body budget/status visibility)
- `host-tui/internal/index/model.go`, `host-tui/internal/index/model_test.go`

### Depends on

Chunk 2.

### Invariants

- Empty and `full-text` strategy run keyword; `none` skips; semantic/full run keyword then resumable semantic. Unknown nonempty values return an error. Manual management choices remain `keyword` and `full`.
- Manifest/acquisition completion is saved before indexing begins and stays saved if indexing fails, is unavailable, or is cancelled.
- Full indexing reports semantic failure after preserving successful keyword output; no path turns it into overall success.
- Automatic apply with zero indexable sources completes after successful empty-set reconciliation. An explicit semantic-only operation may still diagnose that there is nothing to embed.
- Esc requests cancellation and remains on progress until the producer closes. Ctrl-C cancels/quits, and every progress send selects on context so an abandoned consumer cannot strand a goroutine.
- Terminal state is exactly finished, cancelled, or failed with detail; only finished renders the success summary.

### Follow this existing pattern

`host-tui/internal/wizard/applymodel.go` — mirror its context-aware producer, terminal snapshot, and cancellation handshake for both index progress models.

### Implementation notes

- Add a command-level configured-index operation holding one writer lock over the entire keyword/semantic job. Keep shared RunApply and commands.ApplyVault acquisition-only. CLI sequences acquisition then configured indexing. Wizard retains its existing indexing step. Dashboard plan emits a distinct successful-apply message; launch routes it to the existing index progress screen, automatically starting configured indexing (none skips). Preserve failure/cancellation behavior; never transition on unsuccessful apply. Index management uses the same repaired progress lifecycle. Do not hide index events among planned recipe IDs.
- Host embedder process lifecycle must have exactly one waiter. Stop kills then waits on that completion; startup polling honors cancellation without reading a concurrently written stderr buffer. Add a real subprocess stop/cancel regression; the actual apply SIGINT smoke must exit promptly and preserve partial batches.
- Pass signal/TUI contexts through keyword and semantic commands. The standalone `svalbard index` also uses a signal context and ordinary incremental refresh.
- Map command progress to existing TUI rows and one global terminal message. Keep labels/navigation from the design; replace “skip indexing” with cancel semantics.
- Add optional ShouldIndex(vaultPath) read-only query to host TUI dependencies and wizard config so explicit none bypasses the progress screen. Nil defaults true; manifest load errors remain visible through the configured runner. This expresses the existing disabled strategy, not a new user option.
- Keep the default manifest field empty for backward compatibility: interpretation supplies automatic keyword indexing.

**Edge cases:** No ZIMs; `none`; unknown strategy; keyword succeeds then semantic model is absent; cancel before channel startup, during extraction, and during embedding; error racing buffered progress; reopen/resume after indexing failure.

### Acceptance

- Command/CLI tests prove strategy routing, ordering, no double run, durable apply state, and clear post-apply index errors.
- TUI tests drive actual keys and channel closure for finished/cancelled/failed outcomes and verify producer completion without leaks. Assert failures/cancellation remain visible in final ShellLayout/wizard output at 80×8, not only submodel View output; reserve actual wrapped status height before progress rows.
- `go test -race ./...` passes in `host-tui`; affected `host-cli` command/CLI tests pass.

**Do not:** Move indexing into downloader/catalog code, add a screen, overwrite desired items, or roll back acquired artifacts because indexing failed.

## Chunk 4 — Shared runtime/MCP compatibility and packaged journey

### Goal

Make native and MCP hybrid search consume the same completed contract and verify the repaired path through packaged runtime artifacts.

### Architecture

Add one search-package semantic backend used by `Session` and MCP. It loads the completed DB contract, resolves the exact recorded GGUF, owns restartable llama-server lifecycle, embeds with request context, shapes the query vector, and returns a reason for keyword fallback. `engine` rejects incompatible stored/query dimensions before scoring.

### Files

- `drive-runtime/internal/search/search.go`, `drive-runtime/internal/search/session.go`, `drive-runtime/internal/search/session_test.go`, `drive-runtime/internal/search/session_contract_test.go`
- `drive-runtime/internal/search/semantic.go`, `drive-runtime/internal/search/semantic_test.go`
- `drive-runtime/internal/search/search_test.go`, `drive-runtime/internal/search/run_test.go`
- `drive-runtime/internal/search/server/server.go`, `drive-runtime/internal/search/server/server_test.go`
- `drive-runtime/internal/search/engine/engine.go`, `drive-runtime/internal/search/engine/engine_test.go`
- `drive-runtime/internal/mcp/search.go`, `drive-runtime/internal/mcp/search_test.go`, `drive-runtime/internal/mcp/search_internal_test.go`, `drive-runtime/internal/mcp/integration_test.go`
- `drive-runtime/internal/mcp/server.go`, `drive-runtime/internal/mcp/server_test.go`
- `host-cli/internal/toolkit/toolkit.go`, `host-cli/internal/toolkit/toolkit_test.go` (actual packaged launcher lacked search alias; emit explicit search alias for its existing menu action)

### Depends on

Chunk 3.

### Invariants

- `building`, missing, legacy, or malformed contracts are keyword-only. A complete contract resolves only its exact filename/file identity; glob-first selection is not a fallback.
- Query text uses the recorded query prefix. Native vectors may be truncated only when the contract permits and provides enough dimensions, then are L2-normalized exactly like documents.
- Stored blobs must be valid float32 and exactly the contract dimension; dot product requires equal lengths. Any mismatch returns an error that native and MCP convert to visible keyword fallback.
- Per query, refresh the completed contract before embedding. After the query vector is produced, open a read transaction, load the contract ID/state in that snapshot and require the same complete contract used for embedding. All hybrid keyword/vector/detail reads use that transaction; mismatch discards the vector and falls back visibly (one bounded reload is optional, no loop). The engine accepts its existing DB or a transaction query interface.
- Embedding startup/health and HTTP requests honor caller context. Failed/exited startup is retryable in-process and cleanup reaps children; MCP does not cache a transient failure forever. Retained processes have backend-owned lifetime; startup polling/request HTTP honor request cancellation separately. Exactly one waiter reaps exit and signals restart, with no double Wait. A completed first MCP request does not kill a healthy retained backend.
- Native and MCP use the same contract loader, model resolver, vector shaper, and backend lifecycle. MCP keeps current result/read schemas and current keyword fallback behavior.

### Follow this existing pattern

`drive-runtime/internal/search/session.go` — preserve lazy start, mutex protection and fallback reporting; repair its invalid ProcessState-based exit detection via one explicit waiter in the shared backend. Do not copy the existing lifetime bug.

### Implementation notes

- Replace `FindEmbeddingModel` with exact recorded-model resolution for semantic use. Keep archive/binary resolution otherwise unchanged.
- Make query embedding context-aware and return dimension/response errors. Keep the existing FTS candidate set, reciprocal-rank fusion, diversity, and result ordering.
- MCP passes its request context through the shared backend. Preserve the existing JSON result array as the first text content block; when needed, emit the ActionResult.Text advisory as a second text content block in the generic MCP handler. No JSON envelope/result-item changes. Native status remains existing SearchResponse.Status.
- MCP passes its request context through the shared backend. Keep Kiwix/read behavior in scope only where required by the packaged search/read journey.
- Rebuild all four embedded runtime targets. Use the provisioned `/private/tmp/svalbard-phase4-live-vault`, which contains the pinned llama-server, Nomic model/sidecar, Kiwix, a real ZIM, and a working baseline packaged native keyword query. Run semantic indexing, native hybrid query, MCP search/read, same-name/remove reconciliation, model-selection, and fallback checks. Record unsupported platforms as limits rather than weakening assertions.

**Edge cases:** Two GGUFs; recorded model removed/replaced; incomplete contract; query vector shorter/longer than target; malformed/mixed stored blobs; server exits between searches; first MCP request finishes/cancels without poisoning retained backend; contract changes between embedding/scoring in one long-lived native/MCP instance; cancelled MCP request; semantic error with valid keyword hits.

### Acceptance

- Native and MCP fixtures prove exact-model selection, shared prefix/shaping, 256-dimension normalized queries, retry after transient startup failure, cancellation, malformed-vector rejection, and keyword fallback reason.
- Final review repairs: propagate caller context through native capability hashing before binary resolution and through keyword SQL; retain bounded snippet retrieval for normal hybrid results. MCP full-body capture is opt-in, filters/limits metadata results before loading bodies, and reads those bodies inside the same verified transaction. Preserve the existing ranking and source-filter candidate policy.
- The live provisioned vault proves keyword query/read and source reconciliation plus llama-server + Nomic semantic/hybrid behavior and packaged Kiwix MCP read on macOS arm64; the handoff identifies other platforms as unexecuted.
- `go test -race ./internal/search/... ./internal/mcp/...` passes in `drive-runtime`; `make build-drive-runtime`, `make build`, uncached `make verify`, and `git diff --check` pass from the repository root.

**Do not:** Broaden vector candidate scanning, alter ranking, redesign binary/archive resolution, claim cross-platform execution from cross-compilation, or commit generated/runtime artifacts outside established build behavior.
