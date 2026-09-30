# Phase 1 catalog/runtime refresh review

## Ranked findings

1. **BLOCKER — Qwen3.8's `chat-template` configuration never sends Pi's selected thinking level.**  
   **Location:** `drive-runtime/internal/agent/agent.go:174–176`.  
   **Scenario:** Launch Pi with the generated Qwen3.8 model and choose `medium`, `high`, or `off`. A native Pi v0.99.1 request capture using the generated configuration and `--thinking medium` included the configured sampler fields but omitted both `reasoning_effort` and `chat_template_kwargs`. Pi's current OpenAI adapter handles `thinkingFormat: "chat-template"` by building only the explicitly configured `chatTemplateKwargs`; this model config supplies none, and that branch bypasses the generic `reasoning_effort` path. The llama server is started with a process-wide `--reasoning-effort medium`, so changing Pi's thinking level cannot change the request and `off`/`high` silently continue using the server's medium default. The unit test only inspects serialized JSON and therefore misses the resolved request behavior.  
   **Smallest fix:** Add the verified Qwen3.8 template variables under model `compat.chatTemplateKwargs`: `enable_thinking: {"$var":"thinking.enabled"}`, `preserve_thinking: true`, and `reasoning_effort: {"$var":"thinking.effort","omitWhenOff":true}`. Repeat the native request capture for `off`, `medium`, and `high`, asserting that Pi emits matching `chat_template_kwargs`.

2. **BLOCKER — generated Pi compaction settings become invalid when the user later selects a smaller context.**  
   **Location:** `drive-runtime/internal/agent/agent.go:195–209`.  
   **Scenario:** On a fresh drive, launch Pi at 131072 context. Svalbard writes `reserveTokens: 8192` and `keepRecentTokens: 20000`. On the next launch of the same drive, choose the 8192 context. Because `compaction` already exists, Svalbard preserves its own generated values as though they were user customization. Pi's documented trigger becomes `contextTokens > 8192 - 8192`, so every non-empty conversation is immediately eligible for compaction, while the retained-token target is itself larger than the context. Model selection and context choice are launch-time operations, so this is a normal lifecycle rather than a corrupt-config edge case.  
   **Smallest fix:** Distinguish Svalbard-owned defaults from user-owned settings (for example with a small sidecar recording the last generated pair), refresh only values that still equal the recorded generated values on each launch, and preserve values the user changed. Add a two-launch test covering Max→Fast as well as the existing custom-settings case.

3. **SUSPICION — the fresh 8K compaction budget leaves no room for the summary or system prompt.**  
   **Location:** `drive-runtime/internal/agent/agent.go:207–208`; asserted by `drive-runtime/internal/agent/agent_test.go:193–199`.  
   **Scenario:** At 8192 context the generated values are `reserveTokens: 4096` and `keepRecentTokens: 4096`. Pi triggers compaction above 4096 input tokens, then aims to retain approximately 4096 recent tokens before adding the generated summary and system prompt. That can rebuild a context already above the next trigger threshold and cause repeated compaction or overflow. Schema loading does not exercise this long-session behavior.  
   **Smallest fix:** Give the retained history explicit headroom below `contextWindow - reserveTokens` (for example 2048 at the 8K tier), and verify with a long-session Pi test that one compaction produces a context below the trigger threshold.

## Other reviewed areas

No additional actionable defect was found in the exact-file MTP target matching, b11146 flag spellings, `gguf-mtp` installation under `models/draft`, exclusion of drafters from chat actions, model-pack size totals, synchronized embedded catalog, or the pinned harness archive routing. The corrected Pi cost object now satisfies the v0.99.1 native schema.

Verification during review: `scripts/sync-catalog.sh --check`, `go test ./internal/llamaserve ./internal/agent -count=1`, `go test ./internal/toolkit -count=1`, and focused `git diff --check` all passed. Native request capture was configuration/protocol validation only; no large chat GGUF or MTP benchmark was performed.

## Fix review

The original findings are resolved in the final diff:

- Qwen3.8 now supplies explicit `chatTemplateKwargs`; the reported native Pi captures for `off`, `medium`, and `high` show the expected enable flag, preservation flag, effort omission/value, and sampler fields.
- Svalbard-owned compaction defaults now refresh across Max→Fast→Max while individually modified generated fields remain unchanged.
- The 8K retained-history default is reduced to 2048, leaving space below the 4096 compaction threshold.

One current issue remains:

1. **BLOCKER — a pre-existing partial `compaction` object leaves its missing field on Pi's unsafe built-in default.**  
   **Location:** `drive-runtime/internal/agent/agent.go:222–236`; the uncovered fixture is `drive-runtime/internal/agent/agent_test.go:215–240`.  
   **Scenario:** Start with the test's realistic custom settings shape, `{"compaction":{"reserveTokens":777}}`, and launch an 8192-context model. Because the `compaction` object exists and there is no Svalbard sidecar, the code neither changes it nor initializes the missing `keepRecentTokens`. Pi resolves compaction fields independently, so the omission falls back to its built-in 20000-token retention target, larger than the entire 8192-token context; compaction cannot free the intended space. The inverse partial shape has the same problem for a missing reserve value.  
   **Smallest fix:** When a pre-existing compaction map has no sidecar, preserve every present field but fill each absent `reserveTokens`/`keepRecentTokens` field from the context defaults, then write the sidecar so only still-matching generated values refresh later. Extend `TestPrepareClientLaunchConfigForPiPreservesSettings` to assert that its missing `keepRecentTokens` becomes 2048, plus cover the inverse partial object.

Bounded fix verification: `go test ./internal/agent -count=1` passed. No other current issue was found in the reviewed fix diff.

## Final fix review

The remaining partial-compaction blocker is resolved. `refreshPiCompaction` now tracks ownership per field: absent fields receive context-appropriate defaults and become Svalbard-owned; present custom fields remain unowned; changing an owned value removes ownership while the other generated field continues to follow context changes. The reserve-only, keep-only, modified-field, and Max→Fast→Max tests cover the reported scenarios.

Final bounded verification: `go test ./internal/agent -count=1` passed. **Current findings: 0.**
