# Catalog refresh design and implementation plan

**Goal:** Refresh tool and AI client release pins, local model choices, RAM-tier preset defaults, and the runtime settings needed to serve those models.

**Approved scope:** The user requested all pinned tool versions and the model lineup/defaults after the release audit, and explicitly included Pi and the other harnesses. Preserve the existing catalog, pack, and runtime boundaries. This is catalog maintenance with narrowly scoped runtime compatibility changes; map builders and other unfinished features remain separate.

**Follow-up:** Recipe acquisition tracking and disk update reconciliation are queued as Phase 7 in [MASTERPLAN.md](../../MASTERPLAN.md), with a [separate design proposal](2026-09-30-recipe-acquisition-tracking-design.md). Refreshing pins alone does not currently mark already realized recipe IDs stale.

## Design

- Use current stable upstream releases with verified asset names for each supported platform. Pin floating AI clients and browser libraries where upstream provides versioned artifacts. Keep toolchain/framework packages compatible with their PlatformIO consumer rather than mixing unrelated newest packages.
- Preserve existing recipe IDs when upgrading their generation. Prefer Qwen3.8-27B as the higher-RAM dense coding model, retain smaller models for modest hosts, and evaluate Gemma's mid-size option. RAM packs describe one model loaded at a time, with OS and context-cache headroom. Do not equate drive capacity with available host memory.
- Keep Pi, OpenCode, and Goose in the shared harness pack. Verify their current distribution and local endpoint/MCP configuration interfaces before changing pins.
- Include the PlatformIO/ESP32 toolchain recipes in the compatibility audit. The [runtime audit](2026-09-30-runtime-backlog-audit.md) identifies missing offline provisioning and archive routing; do not describe the full PIO epic as complete merely because component pins changed.
- Match model-family sampling settings to upstream recommendations. Do not enable speculative decoding just because an unrelated filename contains `mtp`, or promise vision support without its required projector.
- Evaluate multilingual embedding options without silently changing existing index compatibility. Document any intentionally retained embedding default.
- Keep canonical recipes/presets and their generated embedded copies synchronized. Document release evidence, retained compatibility pins, and hardware limitations.

## Implementation

1. Inventory recipes under `recipes/tools/`, `recipes/apps/`, relevant supporting datasets, CDN assets, and embedded toolchain pins. Query primary release/package metadata and verify actual artifacts; record exceptions.
2. Update the canonical tool and harness recipes, related version/checksum/size metadata, and references needed by their consumers.
3. Refresh `recipes/models/`, `presets/packs/ai/`, affected default/regional presets, and AI usage documentation using verified GGUF files and realistic memory budgets.
4. Add failing tests in `drive-runtime/internal/llamaserve/flags_test.go` for the newly supported model settings. Run `go test ./internal/llamaserve` in `drive-runtime`, implement the minimal compatibility changes, and rerun those tests.
5. Update existing catalog/preset assertions as needed and add behavioral coverage only where changed behavior warrants it. Regenerate using `scripts/sync-catalog.sh`.
6. Run `make verify`, build the host CLI, review the diff, and verify changed release URLs and asset availability. Smoke-test native runtime/harness artifacts where feasible. Record any cross-platform or large-model checks that cannot be exercised locally.

## Completion evidence

See [refresh results](2026-09-30-catalog-refresh-results.md) for the final lineup, settings, primary evidence and deliberate exceptions. Full `make verify`, `make build`, catalog synchronization and diff checks pass. Native pinned binaries launch, Pi/OpenCode load generated model configuration, and llama.cpp produces actual Nomic embeddings. Large chat/MTP inference, offline Python/PIO provisioning and other pack journeys remain explicitly unverified. Changes remain uncommitted; no vault deployment or tools-image publication occurred.

## Chunk 3 — Runtime compatibility and Pi defaults (resumed)

Ownership: `drive-runtime/internal/llamaserve/flags.go`, `context.go`, `flags_test.go`, and `drive-runtime/internal/agent/agent.go` plus relevant agent tests. You are not alone in the codebase. Preserve other edits and do not touch catalog, README, resolver, or generated files. Do not stage or commit: this resumed working tree contains unrelated work and the user has not authorized combining it into commits.

Implement the model flags covered by the existing RED tests: Qwen3.8 native sampler (temp 1, top-p .95, top-k 20, min-p 0, presence penalty 0, repeat penalty 1), q8_0 KV cache/flash attention and medium reasoning preservation. Share sampler values through a small model-family helper so Pi requests do not override them with incompatible defaults. Preserve existing Gemma/Qwen3.5/3.6 values. 24/32 GB automatic context is 32768 rather than 65536.

MTP is optional. Targets select only their matching installed file in `models/draft/`: Qwen3.8-27B (UD-IQ4_XS, UD-Q4_K_M and UD-Q6_K_XL) → mtp-Qwen3.8-27B-Q4_0.gguf, n-max 2; Gemma 4 12B QAT UD-Q4_K_XL → mtp-gemma-4-12B-it.gguf, n-max 4. Enable only when the matching drafter is a regular file, with explicit draft path and parallel 1. No speculation from an arbitrary filename containing MTP. Earlier legacy-Qwen tests claim a built-in head based solely on a renamed target file: correct those expectations to require the actual packaged companion instead of preserving the false assumption. Do not attach the Gemma 12B drafter to other sizes. Matching should be narrow and case insensitive; draft paths must work on paths with spaces. Confirm flag spelling with the saved b11146 arg source.

Pi v0.99.1: keep the drive-local models.json/mcp.json and isolated config/home. Set PI_OFFLINE=1. Serialize JSON structurally. Qwen3.8 uses reasoning=true, chat-template thinking format, supportsReasoningEffort=true, medium thinking by default, and snake-case samplingParams per official Pi schema. Gemma retains its tested named-tool-result/assistant-after-result compat. Bound maxTokens to the chosen context (at most 8192 and a sensible fraction for 8K). Initialize context-appropriate compaction settings only when absent; preserve custom user settings. Use native MCP; no third-party plugins added. Keep OpenCode/Goose compatibility unless a changed shared helper warrants a bound on advertised output budget.

Evidence: `/private/tmp/svalbard-support/doc-models`, `doc-settings`, `doc-mcp`, `doc-cli`, `doc-pi-model-config`, `doc-pi-openai`, `llama-arg`, `doc-gemma12-mtp-readme`, and read-only `/Users/pkronstrom/Projects/own/turbollm/docs/qwen38-pi-validation.md` and harness/provider files. Use primary docs, not unsupported assumptions.

Acceptance: add behavioral RED tests for Pi JSON parsing, Qwen sampler/thinking, low-context bounds, offline env, preservation of existing settings, and quoted/space-containing paths; then implement and run `go test ./internal/llamaserve ./internal/agent` in drive-runtime. Go-format owned files, reread own diff, and write a handoff with changes, test evidence, and any remaining runtime compatibility limitation to `/private/tmp/svalbard-runtime-handoff.md`.

### Review corrections

Native Pi request capture proved that `thinkingFormat: chat-template` requires explicit `chatTemplateKwargs`; a schema-valid empty mapping silently ignored the selected thinking level. Add dynamic enable/effort mappings and preserved thinking, then capture off/medium/high requests against a local fixture.

Keep recent history at one quarter of context (capped at 20000), leaving summary headroom below the compaction trigger. Generated compaction defaults must follow later context choices: record only the last generated fields in a small drive-local sidecar and refresh values still equal to those defaults, while preserving user-modified values. Cover Max→Fast→Max launches and customization. This corrects the earlier initialize-only rule without introducing recipe versioning or a migration framework.
