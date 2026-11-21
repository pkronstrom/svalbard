# Search reliability checkpoint

Status: done 2026-10-07; implementation, final review and verification complete.

Scope: MASTERPLAN Phase 4, following the [design](2026-10-06-search-reliability-design.md) and [implementation](2026-10-06-search-reliability-implementation.md).

## Resulting user path

1. Successful apply saves the installed selection before indexing. Empty/legacy/default strategies build keyword search; `none` skips indexing; semantic/full builds keyword then semantic. Unknown strategies report an error. An indexing failure preserves completed acquisition and reports that indexing failed.
2. The wizard and dashboard show cancellable indexing progress. Cancellation waits for the worker; failure and cancellation never display completion. Errors remain visible in a small terminal. Subsequent apply resumes semantic batches.
3. Keyword reconciliation detects ZIM UUID, size and nanosecond mtime changes, extracts before replacing the old source atomically, and prunes removed sources and their vectors. Failed extraction preserves the previous searchable source. One OS-backed writer lock covers keyword and semantic work.
4. Semantic indexing records the exact model path/hash, prefixes, dimensions and permitted Matryoshka shaping. Matching interrupted builds retain completed vectors; a changed contract rebuilds. The completed contract is published only after full article coverage.
5. Native search and MCP share the recorded model and retained embedding backend. Hybrid reads validate the complete contract inside one database snapshot. Incomplete or incompatible semantic data visibly falls back to keyword; caller cancellation propagates. MCP keeps its first JSON results block and adds fallback advice separately. Full article bodies are loaded only for requested full-detail results after filtering/limit, within the same snapshot; normal hybrid results retain bounded snippets and existing ranking. Embedding requests retain a 30-second timeout and startup health requests respect their deadline.

## Verification evidence

- Fresh `GOCACHE=/private/tmp/svalbard-go-cache GOFLAGS="-count=1 -timeout=2m" make verify` passed: catalog synchronization and 53 tested packages across all five Go modules.
- Fresh host searchdb/embedder/commands/CLI and full host TUI race suites passed. Final coordinator `go test -race ./internal/search/... ./internal/mcp/... -count=1 -timeout=2m` passed.
- Final `make build` passed: host CLI and macOS/Linux arm64/x86_64 runtime targets. `git diff --check` passed.
- Final correctness/design review reopened all reported locations and found zero remaining issues. Consumer regressions prove a long-lived keyword-to-hybrid session, recorded native/MCP query prefixes, 768-to-256 shaping, retained backend reuse, cancellation, full-body snapshot consistency, JSON-first fallback serialization, unchanged ranking/snippets and bounded HTTP timeout/reaping.
- A disposable macOS arm64 vault acquired pinned llama-server b11146, Nomic Q8_0 and Kiwix tools 3.8.2 through normal recipes; WikiCiv and a small real fixture ZIM supplied 401 articles.
- Actual SIGINT during semantic indexing retained 77 vectors across 33 articles. Ordinary apply resumed to 1,149 normalized 256-dimensional vectors covering all 401 articles; all retained vector hashes were unchanged.
- Actual source refresh triggered semantic building/resume; source removal alone retained complete semantic state, and restore resumed successfully. Surviving vector hashes remained unchanged.
- Actual unchanged-source skip, same-name mtime refresh, corrupt replacement preservation and removal/restore passed.
- Packaged public `run search` repeated hybrid queries and MCP repeated hybrid full-detail searches passed with restricted system PATH. MCP browse exercised the bundled Kiwix binary. An invalid alphabetically earlier GGUF was ignored; temporarily hiding the recorded model produced native keyword results and a separate MCP fallback advisory.

All completion gates passed. Final packaged checks used the final rebuilt binaries after all source fixes.

## Scope and limits

The actual execution gate uses macOS arm64 on a local disposable directory. Other runtime targets are cross-compiled only; physical-stick, power-loss and other operating-system execution remain unverified. Host semantic indexing retains its existing model discovery policy; explicit selection among multiple installed embedding models remains part of the multi-model follow-up. Exact recorded-model selection is enforced for native/MCP readers. Existing ranking and keyword candidate policy are retained; recall/performance tuning and non-ZIM ingestion remain follow-ups. Phase 5 covers offline PlatformIO next.

All pre-existing uncommitted work is retained. No staging, commits or Git metadata edits were performed.
