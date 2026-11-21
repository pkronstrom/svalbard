# Runtime and Dodo backlog audit

Date: 2026-09-30. Read-only audit of current implementation; findings below are not fixed. Open findings (MCP recipes.json, semantic recall, non-ZIM ingestion, ESP32 device check) moved to the task tracker 2026-11-21.

## Dodo

The pre-triage Svalbard Dodo export contains 39 pending tasks created March 29–April 17 (correcting the initial count of 38). No completed tasks were present before cleanup. Pending labels were stale in several cases: Go/Cobra migration, native drive services, local sharing, SQLite ZIM indexing, and semantic retrieval have implementations. The combined semantic-search/RAG task is only partially satisfied: retrieval exists, arbitrary-document ingestion does not. The [triage ledger](2026-09-30-dodo-triage.md) records subsequent closures and remaining scope; it supersedes the original pending labels.

Other open themes include mobile recommendations/provisioning, offline routing, bootable media/Kali/Ventoy, documentation and regional content curation, diagram readability, non-ZIM indexing/document-upload RAG, local multi-model routing, broader MCP action exposure, online update checks, and optional interactive apps. Mobile provisioning docs and a macOS Brewfile already exist, so those tasks need scope reconciliation too. KiCad is a provisioning recommendation, not a bundled offline application.

## Drive runtime and recipe integration

`host-cli/internal/toolkit/toolkit.go` generates root `run` and `activate` wrappers, four platform runtime binaries, `.svalbard/actions.json`, and checksum listings. The wrappers launch the Go `svalbard-drive` runtime. Apply regenerates these assets from realized entries and recipe menu/env fields.

Recipes expose installed content through predefined type-to-action mappings (ZIM browse, PMTiles maps, GGUF chat, app serving, local-ai binaries as agent clients). Menu metadata changes labels/groups/order/aliases; env fields configure the activation script. Supported installed types enable built-in capabilities such as search/sharing. Adding a new capability still requires a Go handler and generator mapping; recipe YAML is not an arbitrary runtime plugin interface. Runtime exec actions exist, but the parsed recipe menu schema does not currently expose an arbitrary action definition.

MCP implements vault inventory, structured SQLite queries, and search/read capabilities; it does not expose every native drive action. Its metadata reader accepts `.svalbard/recipes.json`, but the current Go generator does not write that file, so recipe descriptions/tags are not fully propagated through that path. Fixed 2026-11-21: apply now writes `.svalbard/recipes.json` (object keyed by recipe ID: type, description, tags, viewer) for realized entries and removes a stale copy (`host-cli/internal/toolkit/recipes_metadata.go`).

## Search

Update 2026-10-07: Phase 4 repaired the first three limitations below and verified actual pinned llama/Nomic/Kiwix packaged journeys, cancellation/resume and source reconciliation. See the [completed checkpoint](2026-10-06-search-reliability-checkpoint.md). The original findings and verification below are the 2026-09-30 audit; recall/performance and non-ZIM ingestion remain follow-ups.

FTS5 indexing of actual ZIM fixtures, database queries, semantic chunk/vector helpers, hybrid search with fixture vectors, keyword fallback, MCP integration, action resolution, toolkit generation, and embedded archive helpers passed the targeted package tests on this date.

The host builds `data/search.db`; the runtime reads it. Keyword ingestion scans ZIMs only. Semantic indexing uses an installed embedding GGUF, chunk embeddings, and model/prefix/dimension metadata. Hybrid retrieval combines keyword candidates and vector scoring with reciprocal-rank fusion and diversity selection; it falls back to keyword on embedding startup/request failures.

Material limitations found by inspection:

- Indexing is separate from apply; `IndexStrategy` is stored but not consumed by apply. A generated Search menu is not evidence the disk has an index.
- Incremental keyword indexing skips by filename alone. Same-name changed archives need forced reindexing; removed sources are not automatically pruned.
- Runtime search and MCP pick the first embedding GGUF rather than resolving the model recorded in the index. Runtime query vectors also lack explicit dimension compatibility checks/truncation. Multiple models or a changed embedding model can therefore yield invalid semantic comparisons despite passing existing tests.
- Hybrid vector candidates are restricted to the top keyword candidates when keyword hits exist; full-vector scanning happens only when there are no keyword candidates. This can limit semantic recall.
- PDFs/EPUBs/geodata are not ingested into the shared FTS/vector index.

No live smoke test with the newly pinned llama.cpp binary and an actual embedding model was performed in this audit. Passing these tests does not establish compatibility of that new binary/model combination.

## PlatformIO

Catalog edits update Xtensa compiler to 15.2.0+20251204, ESP-IDF framework package to 4.60100.0, and esptool package to 2.41100.260830. ULP and OpenOCD pins were retained following the release inventory. Embedded catalog copies have not yet been synchronized while refresh is in progress.

The existing offline PIO epic is incomplete beyond pin changes:

- `toolchain` is missing from `toolkit.TypeDirs`; direct acquisition therefore writes these archives at the drive root, while the embedded shell expects `tools/platformio/packages/` and platform subdirectories.
- The pack contains five component recipes but no PlatformIO Core distribution, espressif32 platform package/board definitions/build scripts, or complete auxiliary/Python dependency closure. The guide asks for host-installed PIO and claims fully offline operation more broadly than the code establishes.
- The current upstream espressif32 metadata inspected during the release audit requests OpenOCD ~2.1100.0, while the catalog retains 2.1200.20230419; pin compatibility needs deliberate validation.
- Extraction cache markers contain only `ok` and are keyed by package name, so updated archives can reuse stale extracted packages.

A clean-cache, network-disabled ESP32 blink build is required before declaring the offline toolchain usable. Hardware flashing/monitoring needs a separate device check. No such build or device test was run here.

## Verification commands

Passed in `host-cli`: `go test ./internal/searchdb/... ./internal/embedder/... ./internal/commands/... ./internal/toolkit/...`.

Passed in `drive-runtime`: `go test ./internal/search/... ./internal/mcp/... ./internal/actions/... ./internal/config/... ./internal/embedded/...`.

This was targeted verification, not `make verify`; the catalog refresh still has pending runtime implementation and generated-catalog synchronization.
