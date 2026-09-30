# Catalog refresh results — 2026-09-30

The portable tool/harness pins and GGUF defaults have been refreshed. Canonical recipes and the generated host catalog are synchronized. Changes are uncommitted; no existing vault has been updated and no tools image has been published.

Final [correctness/fix review](2026-09-30-catalog-refresh-review.md): zero current findings after correcting Pi schema, thinking request mappings and compaction lifecycle/defaults.

## Local model choices

These packs offer alternatives: load one chat model at a time. File sizes are decimal GB, not total runtime memory; OS, context cache and optional speculation require additional headroom. They are starting points rather than benchmarked guarantees on every host.

| Host RAM | General model | Coding model | Pack |
| --- | --- | --- | --- |
| 8 GB | Gemma 4 E2B QAT | Qwen3.5 4B UD-Q4_K_XL | `ai/models-8gb-ram` |
| 16 GB | Gemma 4 12B QAT UD-Q4_K_XL | Qwen3.5 9B UD-Q4_K_XL | `ai/models-16gb-ram` |
| 24 GB | Gemma 4 26B A4B QAT | Qwen3.8 27B UD-IQ4_XS (14.253 GB) | `ai/models-24gb-ram` |
| 64 GB | Gemma 4 31B QAT | Qwen3.8 27B UD-Q6_K_XL (25.299 GB), plus Qwen3.6 35B A3B alternative | `ai/models-64gb-ram` |

Refreshed model artifacts use exact Hugging Face commit revisions instead of floating `main` URLs. Unsloth dynamic GGUF quants provide a portable choice across llama.cpp hosts. The 24 GB Qwen tier uses IQ4_XS to leave more headroom; the quality tier uses Q6. No MLX runtime was added. The packaged runtime is text-only: model projectors and vision workflows are outside this phase.

Optional `ai/mtp` installs separate Qwen3.8 27B and Gemma 4 12B drafters under `models/draft/`, excluded from chat-model selection. Runtime activation requires the matching regular file and supported target name. Qwen uses two draft tokens; Gemma uses four. MTP consumes extra memory and speed depends on hardware/workload. No large chat-model or MTP benchmark was run here.

Qwen3.8 defaults are temperature 1, top-p .95, top-k 20, min-p 0, presence penalty 0 and repeat penalty 1, with medium thinking, flash attention and q8 KV cache. Pi shares the sampler rather than overriding it with another default. Automatic context on 24/32 GB hosts is reduced to 32768. Existing explicit context choices remain available.

Nomic embed-text v1.5 Q8 remains the embedding default, now revision-pinned. Changing it would require index compatibility/reindex work. Nomic v2's 512-token input does not satisfy the current 750-token preflight. No multilingual embedding default or speech-recognition runtime was silently introduced.

## Harnesses and toolchain

Current pins include llama.cpp b11146, Pi 0.99.1, OpenCode 1.18.33 and Goose 1.52.0. Pi runs with isolated drive-local settings and `PI_OFFLINE=1`, native MCP configuration, bounded output/compaction defaults, and medium Qwen3.8 thinking. Explicit chat-template request mappings allow changing the thinking level. Generated compaction fields follow later context choices using a small last-defaults sidecar; user overrides are preserved. Its updated model schema includes input/output/cache cost fields. No third-party plugins were selected; the barebones plugin list remains a design discussion.

Tool recipes and tools-image pins were checked against upstream release/package metadata and available assets. PlatformIO components retain compatible framework/OpenOCD packages rather than combining unrelated latest versions. Updated Dockerfile pins require an image build/publication before downstream consumers receive them.

## Verification

- `GOCACHE=/private/tmp/svalbard-go-cache make verify` passed across all five Go modules. HTTP fixtures required permission to bind localhost; the earlier restricted run failed for that environmental reason.
- `make build` passed: all four drive-runtime targets (macOS/Linux, arm64/x86_64) were cross-compiled and embedded into `bin/svalbard`. Temporary native host/drive builds also passed. Cross-compilation does not prove execution on the other platforms.
- 145 tool/app/model acquisition URLs from the Phase 1 refresh returned successfully in HTTP HEAD/range checks. This is availability evidence, not an installation test of every pack or an audit of subsequent concurrent additions.
- Native macOS arm64 llama.cpp, Pi, OpenCode and Goose launched with the expected versions; archive hashes matched upstream GitHub release digests.
- llama.cpp b11146 with the pinned Nomic GGUF passed a live health/embedding check: two inputs, two 768-dimensional vectors. The GGUF hash matched upstream LFS metadata.
- Native Pi accepted the generated local Qwen3.8 provider (8192 context, 4096 output, thinking enabled) after fixing missing cache-cost fields. OpenCode's native model listing accepted its generated provider. Goose was version/help checked; an actual chat/MCP conversation was not exercised.
- Real Pi HTTP requests captured against a local response fixture verify off/medium/high thinking controls, preserved thinking and all six sampler fields. This checks the actual provider adapter, not model inference. Review also found and fixed generated compaction settings surviving a switch from Max to Fast context; tests cover Max→Fast→Max and custom-field preservation.
- Runtime tests cover model sampler, exact drafter matching, missing/unrelated draft files, paths with spaces, output/context bounds, settings preservation and separate draft installation without chat actions.
- Partial custom compaction objects now receive only their missing context-safe fields. Field-level default ownership prevents later context changes from reclaiming user overrides; reserve-only and retention-only cases are covered.

Large chat GGUF inference, live MTP/tool-call conversations, non-macOS execution, offline PIO builds and device flashing remain unverified. Search unit/integration tests and actual embedding inference pass; the whole provision-index-search user path is still Phase 4.

## Deliberate exceptions and next work

- Binwalk's invalid Python version was replaced with the existing upstream legacy Python source tag v2.3.4. Rust v3 lacks the portable release assets expected by this recipe. Python provisioning still needs repair: the builder calls unsupported `uv pip download`, and source-package wheel handling needs validation. Updated Python pins do not imply usable installations.
- zstd is removed from the default tools pack until its deferred source acquisition is implemented.
- Some portable binary providers lag upstream application releases (for example static Nmap); do not describe the entire catalog as uniformly latest.
- ZIM tools 3.6 is retained for the tools image's libmagic compatibility; a version bump needs a compatible image change.
- DuckDB WASM assets are corrected, but an interactive standalone application path still needs verification in Phase 6.
- Offline PIO acquisition/layout/dependency/cache work remains Phase 5. Pin refresh does not complete that epic.
- Existing manifests do not detect changed acquisition pins yet. The simple acquisition-hash feature remains Phase 7; refreshing this catalog does not update an already realized recipe ID on a disk.

Next: Phase 1b's remaining catalog-audit items, then initial deployment and interruption/resume (Phase 2). Preserve ongoing content/preset/builder work rather than repeating already completed audit fixes.

## Primary evidence

- [llama.cpp b11146 release](https://github.com/ggml-org/llama.cpp/releases/tag/b11146)
- [Pi model configuration](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/models.md) and [settings](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/settings.md)
- [Unsloth Qwen3.8 GGUF repository at the pinned revision](https://huggingface.co/unsloth/Qwen3.8-27B-GGUF/tree/4ca720788d1e01f1bff70c033e0d0028fd02e502)
- [Unsloth Gemma 4 12B QAT GGUF at the pinned revision](https://huggingface.co/unsloth/gemma-4-12B-it-qat-GGUF/tree/980b060c40a8539ac159e0501a3e0f66a6365af3)

Temporary evidence files: `/private/tmp/svalbard-release-audit.json`, `/private/tmp/svalbard-current-link-results.json`, `/private/tmp/svalbard-smoke/downloads.json`, `embedding-result.json`, `pi-config-result.json` and `harness-config-result.json` under that smoke directory. The earlier harness result preserves the rejected Pi schema; the later Pi-specific result records the corrected successful load. Local TurboLLM's historical GGUF/Pi validation informed settings; its machine-specific speed measurements were not generalized.
