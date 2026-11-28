# llama.cpp, MTP and model picks: follow-up (2026-11-26)

Scope: the llama.cpp delta since the pinned `b11146`, MTP/drafter status per catalog family and candidate, a Qwen3.8-Flash-Next recipe pick, and the MTP default policy.
Repo context: `recipes/tools/llama-server.yaml` (b11146), `drive-runtime/internal/llamaserve/flags.go` (`packagedDraft`, only the Qwen3.8-27B drafter), `presets/packs/ai/*.yaml`.
Evidence labels: **[vendor]** = claim by the model or quant publisher; **[indep]** = measurement by a third party; **[PR]** = number in an upstream llama.cpp PR (by the contributor, usually on one machine); **[unverified]** = I could not confirm it.

## 1. llama.cpp: where it is now

**Versions.** In 2026-08 llama.cpp started cutting semver releases on top of the per-commit `bNNNNN` pre-release tags ([releases](https://github.com/ggml-org/llama.cpp/releases)).
- Latest semver: **v1.1.0**, 2026-11-26 09:16 UTC, the same commit as **b12774** (checked with the GitHub compare API). The v1.1.0 release carries no binaries (its only asset is `nightly-tag.txt`). The binaries are on the b-tags.
- Latest b-tag: **b12777** (2026-11-26 14:17 UTC). It ships `llama-b12777-bin-{macos-arm64,macos-x64,ubuntu-x64,ubuntu-arm64}.tar.gz`, so the recipe's URL pattern is unchanged.
- Our pin **b11146** is the same commit as **v0.5.0** (2026-09-23).
- Between them came v0.6.0 (10-05), v0.7.0 (10-15), v0.8.0 (10-28), v0.9.0 (11-04), v1.0.0 (11-13) and v1.1.0 (11-26) ([v1.0.0 notes](https://github.com/ggml-org/llama.cpp/releases/tag/v1.0.0), [blog](https://huggingface.co/blog/ggml-org/llama-cpp-v1)).

**Changes since b11146 that matter to us**
- **Metal speculative verify fix (the big one).** [#29869](https://github.com/ggml-org/llama.cpp/pull/29869), merged 10-05 and shipped in v0.6.0, added "few-row MMA" mat-mul kernels for 2–16 src1 rows. Before it, M1–M4 ran the verify batch through mat-vec kernels whose cost grows with every row, so on the PR's own M3 Ultra test DFlash2 decoding of Qwen3.8-27B was slower than plain decoding.
  - [PR] M3 Ultra, Qwen3.8-27B Q4_0 + DFlash2 Q8_0 drafter, n-max 7: code T=0 went 30.2→110.0 tok/s and prose T=1 went 13.9→48.8, against a 32 tok/s serial baseline.
  - ggerganov enabled the same path on M5 as well.
  - [indep, comment on the PR] M2 Ultra, "MTP finally does something": Qwen3.8 27B Q8 went 21→30 tok/s at ~20k context.
  - [#30065](https://github.com/ggml-org/llama.cpp/pull/30065), merged 10-07, extended the kernels to the IQ types, Q2_K/Q3_K, BF16 and MXFP4. Our `qwen-27b` is UD-IQ4_XS, so it was not covered until this PR.
  - **Still open:** [#30047](https://github.com/ggml-org/llama.cpp/pull/30047), the MUL_MAT_ID (MoE experts) variant. On Metal, MoE models (35B-A3B, 26B-A4B, Mellum, Inkling, Flash-Next) still verify their experts through the old path.
- **GDN replay instead of rollback** ([#31669](https://github.com/ggml-org/llama.cpp/pull/31669), v1.1.0). This matters for the hybrid Qwen3.5/3.6/3.8 models when speculating. It cuts recurrent-state memory by 73–91% and makes the batched verify faster ([PR], measured on CUDA).
- **MTP and speculative decoding**
  - Qwen4Exp (Flash-Next) MTP: [#29761](https://github.com/ggml-org/llama.cpp/pull/29761), first tag **b11330**.
  - Probabilistic draft sampling for draft-simple/MTP (v0.6.0). New `--spec-draft-n-ubatch` (v0.7.0).
  - MTP drafts accept mtmd (image) inputs (v0.7.0).
  - MTP hidden state is saved in context checkpoints (v0.9.0).
  - The NextN API was promoted into `llama.h` (v1.0.0).
  - Sources: [v0.6.0](https://github.com/ggml-org/llama.cpp/releases/tag/v0.6.0), [v0.7.0](https://github.com/ggml-org/llama.cpp/releases/tag/v0.7.0), [v0.9.0](https://github.com/ggml-org/llama.cpp/releases/tag/v0.9.0).
- **DFlash family.** v1.1.0 split DFlash into `draft-dflash`, `draft-dflash2`, `draft-dspark` and `draft-lilicorr` ([#32016](https://github.com/ggml-org/llama.cpp/pull/32016)). The docs say `draft-dflash` still auto-resolves DFlash2, DSpark and LiliCorr drafts from the model ([docs/speculative.md @ v1.1.0](https://github.com/ggml-org/llama.cpp/blob/v1.1.0/docs/speculative.md)). LiliCorr arrived in v0.7.0.
- **PLE (n-gram/engram) tables.**
  - `-lzm/--lazy-mode` reads rows of huge tensors from disk on demand. The default `auto` turns it on for tensors over 4 GiB ([arg.cpp @ v1.1.0](https://github.com/ggml-org/llama.cpp/blob/v1.1.0/common/arg.cpp)), and b11146 already has the flag.
  - `llama_prefetch_rows` uses madvise to prefetch PLE rows ([#29599](https://github.com/ggml-org/llama.cpp/pull/29599), v0.6.0).
  - `--ple`/`--shard` loads an external engram shard ([#31185](https://github.com/ggml-org/llama.cpp/pull/31185), b12233, v1.0.0).
- **New architectures**
  - GLM-5.3-Flash (v0.6.0)
  - Muse Spark Open, DeepSeek V4.1 Flash, LFM3 (v0.8.0)
  - Mistral Large 4 (v0.9.0)
  - Kimi-K3.1, Ripple (v1.0.0)
  - Inkling 2 / 2 Mini / 2 DSpark, Hy4 flash, RAON-1-mini, Domyn Edge 4B, Zaya1 8B, MiniMax-M3.1 ([v1.1.0](https://github.com/ggml-org/llama.cpp/releases/tag/v1.1.0))
- **mtmd, TTS and rerank**
  - mtmd/TTS/audio: Kokoro, Zonos3-Mini, tiny-voca, VoxCPM2, Fish S2, Kyutai Hibiki, plus `--tts-voice` and related flags (v1.1.0). `/v1/realtime` and `/tts` endpoints (v0.8.0). `/v1/audio/speech` (v1.0.0).
  - Rerank: `classifier_pooling` for rerankers, and RANK pooling batch splitting for Qwen3/Qwen3-VL rerankers (v0.6.0). EmbeddingGemma-2 (v0.7.0).

**Breaking changes for `llama-server`**
- **Default port is now 9931 instead of 8080** ([#30159](https://github.com/ggml-org/llama.cpp/pull/30159), v0.7.0). Svalbard always passes `--port` (`drive-runtime/internal/chat/chat.go:36`, `agent/agent.go:368`, `serveall/serveall.go:80`), so it is not affected. Model cards now show `localhost:9931`.
- `--defrag-thold` was removed (v1.0.0). Svalbard does not use it.
- I diffed the speculative flags in `common/arg.cpp` between b11146 and v1.1.0. **No flag was removed or renamed.**
  - Flags Svalbard uses are still present: `--spec-type`, `--spec-draft-model`/`-md`/`--model-draft`, `--spec-draft-n-max`.
  - Added: `--ple`, `--shard`, `--spec-draft-n-ubatch`, `--spec-draft-sampling`.
  - `--mtp` and `--dflash` are download-only flags (they also fetch the sidecar with `-hf`).
  - The `--spec-type` values grew from b11146's `draft-simple|draft-eagle3|draft-dflash|draft-dspark|draft-mtp|ngram-*` to add `draft-dflash2|draft-lilicorr`.
  - Passing a repeated `--spec-type` now logs a deprecation warning that asks for comma-separated values.

## 2. MTP and drafters per family

For the catalog files I read the GGUF headers with HTTP range requests (`nextn_predict_layers` and `blk.N.nextn.*` tensors).

| Model (catalog file) | Built-in head in our GGUF? | Separate drafter | llama.cpp flags | Speed evidence |
|---|---|---|---|---|
| Qwen3.5-0.8B / 4B (bartowski Q4_K_M) | **Yes**: `qwen35.nextn_predict_layers=1`, `blk.24.nextn.*` / `blk.32.nextn.*` | — | `--spec-type draft-mtp` | [indep, pre-fix b9330] Qwen3.5-9B on M1 Max: −11% to −28% ([#23752](https://github.com/ggml-org/llama.cpp/issues/23752)). No post-fix Metal data found |
| Qwen3.5-9B (unsloth `Qwen3.5-9B-GGUF`) | **No** (no nextn KV or tensors) | Swap to [unsloth/Qwen3.5-9B-MTP-GGUF](https://huggingface.co/unsloth/Qwen3.5-9B-MTP-GGUF) | `draft-mtp` | Same as above |
| Qwen3.6-35B-A3B (unsloth UD-Q4_K_M, 20.61 GB) | **No** | [unsloth/Qwen3.6-35B-A3B-MTP-GGUF](https://huggingface.co/unsloth/Qwen3.6-35B-A3B-MTP-GGUF): UD-Q4_K_M 22.66 GB, UD-IQ4_XS 18.21 GB | `draft-mtp` | [indep] dual RTX 3090: +10–20% ([comment on #28243](https://github.com/ggml-org/llama.cpp/pull/28243)). On Metal, MoE verify is not covered by the MMA fix (#30047 open) |
| Qwen3.8-27B (UD-IQ4_XS / UD-Q6_K_XL) | **Yes**: `blk.64.nextn.*`, n=1 (re-verified) | `MTP/mtp-Qwen3.8-27B-Q4_0.gguf` 1.37 GB (same layer). DFlash2 drafter [z-lab/Qwen3.8-27B-DFlash2-GGUF](https://huggingface.co/z-lab/Qwen3.8-27B-DFlash2-GGUF) (Q4_K_M 1.14 GB, Q8_0 2.06 GB) | `draft-mtp` (no `-md` needed); DFlash2: `-md … --spec-type draft-dflash --spec-draft-n-max 7` | See the Qwen3.8-27B Metal rows below the table |
| Gemma 4 E2B/E4B/12B/26B-A4B/31B (unsloth QAT) | **No** (separate "assistant" head) | `mtp-gemma-4-*.gguf` in each unsloth repo (12B: Q4_0 0.25 GB, Q8_0 0.47 GB; 31B: Q4_0 0.28 GB), e.g. [unsloth/gemma-4-12B-it-qat-GGUF](https://huggingface.co/unsloth/gemma-4-12B-it-qat-GGUF/tree/main/MTP). DFlash: [z-lab/gemma4-12B-it-DFlash](https://huggingface.co/z-lab/gemma4-12B-it-DFlash); EAGLE3/DSpark for 26B/31B from RedHatAI (docs/speculative.md) | `-md mtp-… --spec-type draft-mtp` | [indep] M1 Max 64 GB, 26B-A4B Q4_K_XL + Q8_0 MTP: 58.2→72.2 tok/s (1.24×) at n-max 2–3, build unknown ([ikyle.me](https://ikyle.me/blog/2026/how-to-setup-a-local-coding-agent-on-macos)). [indep] Gemma 4 26B on M1 Max slower ([#23752](https://github.com/ggml-org/llama.cpp/issues/23752)). Ours (b11146, M4 Max, 12B): 9–40% slower |
| **Qwen3.8-Flash-Next** | **No**: the main shards have no nextn tensors (read shard 2 of UD-Q2_K_XL and of ggml-org Q2_K). The 4B MTP layer is a sidecar | unsloth `MTP/mtp-Qwen3.8-Flash-Next-{Q4_K_M 2.79, Q8_0 4.14, BF16 7.77}` GB plus `shared-*` variants that borrow embed/output (Q4_K_M 1.91, Q8_0 2.79, BF16 5.23 GB). ggml-org `mtp-…-Q4_0` 2.20 GB, `-Q8_0` 4.14 GB, and DFlash2 `dflash-…-Q4_0` 0.35 GB | `-md mtp-….gguf --spec-type draft-mtp --spec-draft-n-max 2–3` (mainline ≥ b11330). `shared-*` heads need unsloth's fork, see §3 | See the Flash-Next rows below the table |
| Inkling 2 Mini (26.2–26.4B, 3.4B active, looped MoE; Apache-2.0; released 2026-11-20) | No MTP. It ships a **DSpark** drafter (`mtp.safetensors`) | [unsloth/Inkling-2-Mini-GGUF](https://huggingface.co/unsloth/Inkling-2-Mini-GGUF): `dspark-Inkling-2-Mini-Q8_0.gguf` 0.50 GB (mainline). Main UD-Q4_K_XL 17.63 GB, UD-IQ4_XS 15.73 GB, UD-Q3_K_XL 13.63 GB. [ggml-org/Inkling-2-Mini-GGUF](https://huggingface.co/ggml-org/Inkling-2-Mini-GGUF): IQ4_NL 17.63 GB | `-md …DSpark.gguf --spec-type draft-dspark --spec-draft-n-max 3 -fa on --jinja` (docs) | No published speed found. Model support from **b12600** ([#32694](https://github.com/ggml-org/llama.cpp/pull/32694)); DSpark from ~b12716–b12720 ([#32218](https://github.com/ggml-org/llama.cpp/pull/32218)). Card: "Local performance… speed varies" ([card](https://huggingface.co/thinkingmachines/Inkling-2-Mini)) |
| Muse Glimmer 30B (29.6B dense incl. 1.8B ViT, 52 layers, 131k context; Apache-2.0; Aug 2026) | No MTP. It ships a **DFlash** drafter | [meta-models/Muse-Glimmer-30B-GGUF](https://huggingface.co/meta-models/Muse-Glimmer-30B-GGUF): KQuant Q4_K_M 16.76 GB, Q4_K_XL 19.65 GB, `dflash-…-Q4_K_M` 1.63 GB, mmproj 1.40 GB. [ggml-org](https://huggingface.co/ggml-org/Muse-Glimmer-30B-GGUF): Q4_K_M 19.25 GB, dflash Q4_0 1.45 GB | `-md dflash-… -ngld 99` with `--spec-type draft-dflash` (needs ≥ b10353, so b11146 works) | [vendor] M4 Max 23.7→37.8 tok/s (1.5×), M5 Max 26.6→50.2 (1.8×). **The Apple numbers are ExecuTorch, not llama.cpp.** RTX 5090 on llama.cpp: 74.9→233.4 (3.1×) ([card](https://huggingface.co/meta-models/Muse-Glimmer-30B)) |
| Mellum2.1-12B-A2.5B Thinking (MoE 64 experts / 8 active, 131k context; Apache-2.0) | Built-in head only in the `mtp/…-MTP-*.gguf` combined files. Plain files have none | [JetBrains/Mellum2.1-12B-A2.5B-Thinking-GGUF](https://huggingface.co/JetBrains/Mellum2.1-12B-A2.5B-Thinking-GGUF): Q4_K_M 8.07 GB + `mtp-…-BF16.gguf` 1.77 GB, or combined `mtp/…-MTP-Q4_K_M.gguf` 8.33 GB | `--spec-type draft-mtp --spec-draft-n-max 1` (≥ **b11747**) | [vendor] 1.4–1.8×, engine unstated. [PR 30612 via [modelfit](https://modelfit.io/blog/jetbrains-mellum2-1-mtp-draft-head-mac/)] M2 Max: 1.05× at n-max 1, 0.81× at 2, 0.66× at 3 |

**Qwen3.8-27B on Metal, by build.** These three rows of evidence come from the Qwen3.8-27B table row.
- **Before the Metal fix:**
  - Ours: no gain at b11146.
  - [indep] M4 base 24 GB, b10450: parity, prose −22% ([sweep](https://github.com/sudoingX/qwen38-mtp/blob/master/sweeps/apple-silicon.md)).
  - [indep] M4 Mac mini 24 GB, 09-02 master: −24% prose, −4% code ([cooconsbit](https://tools.cooconsbit.com/en/articles/qwen38-mtp-vs-dflash2-mac-mini-24gb-en)).
  - [indep] M3 Ultra (LM Studio): +6% at n-max 2.
  - [indep] **M5 Max** b10434: 21.9→31.8 (+45%) at n-max 4, +1% at n-max 2 ([asiai](https://asiai.dev/qwen38-27b-apple-silicon/)).
- **After the fix:** [indep] M2 Ultra Q8: 21→30 ([#29869 comment](https://github.com/ggml-org/llama.cpp/pull/29869)). [PR] M3 Ultra DFlash2: 3.4×.
- **CUDA:** +33–145% ([sudoingX](https://github.com/sudoingX/qwen38-mtp)); RTX 3090 +69–82% (sweep).

**Qwen3.8-Flash-Next MTP measurements.** These rows of evidence come from the Flash-Next table row.
- [PR #29761, merged] DGX Spark IQ4_XS, n-max 3: 28.4→43.9 tok/s (1.55×), acceptance 0.64.
- [vendor, unsloth] B200 greedy: 1.67× (UD-Q4_K_XL), 1.34× (UD-IQ1_S). It is a net loss at concurrency 8 (0.81–0.87×) ([MTP README](https://huggingface.co/unsloth/Qwen3.8-Flash-Next-GGUF/blob/main/MTP/README.md)).
- [PR #27836, unmerged alternative] **M3 Max 128 GB** UD-IQ4_XS: 27.4→37.2 (+36%) at n-max 2, 38.8 (+42%) at n-max 3 ([#27836](https://github.com/ggml-org/llama.cpp/pull/27836)).
- [indep] dual RTX 3090 with partial offload: −10% ([#28243 comment](https://github.com/ggml-org/llama.cpp/pull/28243)).
- [indep] M3 Ultra llama.cpp baseline without MTP: 26.4 tok/s at UD-Q4_K_XL ([RockTalk card](https://huggingface.co/RockTalk/Qwen3.8-Flash-Next-MLX-4bit)).

## 3. Qwen3.8-Flash-Next: what it is and a recipe pick

**What it is** ([model card](https://huggingface.co/Qwen/Qwen3.8-Flash-Next), [unsloth card](https://huggingface.co/unsloth/Qwen3.8-Flash-Next-GGUF))
- **Release and status.** Qwen repo created 2026-08-24; released 2026-08-26 (per [search summary](https://www.alibabacloud.com/blog/qwen3-8-flash-next-a-new-architecture-towardsultimate-cost-efficiency_603501), date [unverified beyond the HF repo dates]). Billed as an "experimental preview of the architecture that will underpin Qwen4". The HF architecture is `qwen4_exp`; the GGUF arch is `qwen4exp`.
- **Licence: `qwen-community-1.0`** ([LICENSE](https://huggingface.co/Qwen/Qwen3.8-Flash-Next/raw/main/LICENSE)). It is MIT-like, with two conditions:
  1. Products above 100M MAU or US$20M monthly revenue must display the model name.
  2. "Model as a Service" or "AI Work Assistant" businesses need a separate licence for commercial use. Internal use is exempt.

  It is **not Apache-2.0**, so the recipe must say so. The Qwen repo is gated ("auto"); the unsloth and ggml-org mirrors are not.
- **Parameters.** 125B main with 6B active, plus a 51B n-gram embedding ("engram"/PLE, `per_layer_token_embd` 160 × 320M) and a 4B MTP layer. HF counts 180B in total.
- **Architecture.**
  - 48 layers laid out as 12 × (3 Gated DeltaNet + 1 Qwen Sparse Attention).
  - MoE with 512 experts, 10 routed + 1 shared.
  - Hidden size 2560; 2 KV heads on the QSA layers, so the KV cache is small.
  - Vision encoder: mmproj BF16 0.91 GB.
- **Context:** 262,144 native, extensible to 1M.

**llama.cpp support**
- Architecture since **b10660** ([#27742](https://github.com/ggml-org/llama.cpp/pull/27742), merged 08-27). Our b11146 can load it.
- Mainline MTP since **b11330** (#29761). External `--ple` since **b12233**.
- Correctness and speed fixes kept landing through v1.1.0, e.g. #32483, #32519, #32669 (`.scale` tensors, 11-21), #32884 (QSA query chunking).
- Metal hc-op support: [#29000](https://github.com/ggml-org/llama.cpp/pull/29000). A Metal fusion PR, #30680, is still open.
- **Minimum practical build: v1.1.0 / b12774+** [judgement; the fixes are listed in the v1.1.0 notes].
- **MTP head caveat.** unsloth's MTP README says "a stock ggml-org build cannot use these", meaning the `shared-*` heads and PR #28243, which closed unmerged. Mainline's #29761 is a different implementation that loads a self-contained `mtp-*.gguf` through `-md`. ggml-org ships `mtp-Qwen3.8-Flash-Next-Q4_0.gguf` (2.20 GB) for it. **Use the ggml-org Q4_0 sidecar or unsloth's non-shared `mtp-…-Q4_K_M.gguf` (2.79 GB) with mainline** [inferred: I did not confirm that unsloth's non-shared head loads on mainline].

**GGUF sizes (unsloth, repo SHA `f5d175de…`, GB = 10⁹ bytes)** ([tree](https://huggingface.co/unsloth/Qwen3.8-Flash-Next-GGUF/tree/main))
- Each quant is split. Shard 1 is metadata (0.01 GB). The **last shard is the 28.8 GB IQ4_NL engram table** for every quant from Q4_K_XL down ([engram README](https://huggingface.co/unsloth/Qwen3.8-Flash-Next-GGUF/blob/main/engram/README.md)).

| Quant | Backbone (resident) | Engram shard | Total |
|---|---|---|---|
| UD-IQ1_S | 37.6 | 28.8 | 66.4 |
| UD-IQ1_M | 39.2 | 28.8 | 68.0 |
| **UD-Q2_0** | **41.7** | 28.8 | **70.5** |
| UD-IQ2_XS | 47.0 | 28.8 | 75.8 |
| UD-Q2_K_XL | 50.8 | 28.8 | 79.6 |
| UD-IQ3_XXS | 54.8 | 28.8 | 83.6 |
| UD-Q3_K_XL | 60.7 | 28.8 | 89.5 |
| UD-IQ4_XS | 64.9 | 28.8 | 93.7 |
| UD-Q4_K_XL | 82.5 | 28.8 | 111 |

- Other publishers:
  - [ggml-org](https://huggingface.co/ggml-org/Qwen3.8-Flash-Next-GGUF) Q2_K 83.1 GB (PLE at Q4_0), IQ4_NL 102 GB.
  - [ISTA-DASLab GSQ-RCO](https://huggingface.co/ISTA-DASLab/Qwen3.8-Flash-Next-GSQ-RCO-GGUF) Q2_0 66.4 GB, IQ2_XS 68.0 GB (Apache-2.0 label on the quant repo).
  - No bartowski repo was reachable (API returned 401).
- unsloth's own guide gives the RAM needed by bit width: 1-bit 32 GB, **2-bit 42 GB**, 3-bit 50 GB, 4-bit 64 GB. Adding MTP raises these to 1-bit 76 GB and 2-bit 80 GB. The same page also says "at least 75 GB". It notes that the PLE table can be "offload[ed] to SSD and use mmap" ([guide](https://unsloth.ai/docs/models/qwen3.8-next)). My reading: the lower figures are the backbone with the engram lazily mmapped, and the MTP figures appear to count everything as resident.

**Tier fit and recommendation**
- **24 GB: does not fit.** Even the IQ1_S backbone is 37.6 GB.
- **64 GB: fits only at 2-bit, relying on lazy-mode engram.** Recommend **unsloth `UD-Q2_0`**, all 3 shards = **70.5 GB download, ~41.7 GB resident**, plus a small KV cache. At ~42 GB resident this matches unsloth's "2-bit = 42 GB" row.
  - It leaves room for macOS. The Metal working-set cap on a 64 GB Mac is roughly 48 GB by default [unverified; the 24 GB figure of 18000 MB is cited in the [sweep](https://github.com/sudoingX/qwen38-mtp/blob/master/sweeps/apple-silicon.md)].
  - UD-IQ2_XS (47 GB resident) is the stretch option.
  - UD-Q2_K_XL (50.8 GB) probably exceeds the default Metal cap.
- Add MTP only as an opt-in sidecar: ggml-org `mtp-Qwen3.8-Flash-Next-Q4_0.gguf`, 2.20 GB.
- Treat it as a **separate pack (e.g. `ai/models-64gb-flash` or a future 128 GB tier)**. It is not an addition to `ai/models-64gb-ram`, whose target is 61 GB.

**Must re-measure / seams to fix before shipping**
1. Decode speed when the 28.8 GB engram is lazily read **from the USB stick**: random row reads per token, warmed by the page cache. Nobody has published this.
2. Quality at 2-bit: unsloth's quality claims are vendor claims only.
3. Recipes have no multi-shard download support: no recipe uses split GGUFs. Model discovery would also see 3–4 `.gguf` files per model, so pass `-m` the `-00001-of-` shard.
4. `llamaserve.ExtraFlags` matches `strings.Contains(name, "qwen3.8")`. It would **wrongly apply the Qwen3.8-27B custom Jinja template, q8_0 KV and reasoning flags to Flash-Next**.
5. The license field must be `qwen-community-1.0`.

## 4. Default MTP policy

Evidence summary:
- **CUDA:** positive and consistent for dense or fully-resident models (Qwen3.8-27B +33–145%; Flash-Next 1.3–1.7× on B200 and DGX Spark at concurrency 1).
  - Negative when the model is partly offloaded to system RAM (#28243 comment).
  - Negative under concurrency (unsloth: 0.81–0.87× at 8 streams).
- **Metal before v0.6.0 (our b11146):** null to negative on M1–M4 everywhere except M5, which has the tensor API. That explains our 2026-09-30 numbers.
- **Metal after #29869/#30065 (v0.6.0/v0.7.0):** the verify cost now amortizes for dense matmuls.
  - Anecdotal dense evidence is positive: M2 Ultra 27B Q8 +43%.
  - DFlash2 on M3 Ultra is 2–3.4× ([PR]).
  - **MoE on Metal is still unfixed** (#30047 open). Mellum2.1 measured 1.00–1.05× on M2 Max at n-max 1.
  - There is still no independent M4 Max measurement after the fix.

Recommended policy (defensible now):
- **Keep MTP opt-in, but change the rule.** Turn it on when all of these hold:
  - the target GGUF has a built-in nextn head (or a sidecar is installed),
  - `--parallel 1`,
  - the model fits fully on the GPU or in unified memory.

  Default `--spec-draft-n-max 2`.
- **CUDA:** default on for dense models with a built-in head.
- **Metal, dense (Qwen3.8-27B, Gemma 4 12B/31B, Qwen3.5 0.8B/4B):** after the bump to v1.1.0, keep it off until it is re-measured on M4 Max. If the gain is ≥ 10% at n-max 2–3, flip the default to on.
- **Metal, MoE (35B-A3B, 26B-A4B, Flash-Next, Mellum, Inkling):** keep it off until #30047 lands. Flash-Next is the possible exception: the unmerged #27836 measured +36% on M3 Max.
- **Re-measure on M4 Max after bumping to b12774+:**
  - Qwen3.8-27B IQ4_XS and Q6_K_XL with the built-in head, n-max 1/2/3, short and 5k-token prompts. Repeat our 2026-09-30 protocol.
  - Same model with the z-lab DFlash2 Q4_K_M drafter (`--spec-type draft-dflash --spec-draft-n-max 7`). It is likely the bigger win on Metal.
  - Gemma 4 12B and 31B with the unsloth Q8_0 MTP head.
  - Qwen3.6-35B-A3B from the MTP repo, as the MoE control.
  - Flash-Next UD-Q2_0 with and without the ggml-org MTP Q4_0 head, run from the stick.
