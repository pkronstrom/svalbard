# svalbard local AI: model roster, embedders and backends (research 2026-11-21)

Read-only research: nothing in the repo was changed. "GB" means decimal GB as Hugging
Face shows it; "GiB" is the unit svalbard recipes store in `size_gb`. Benchmarks are
the vendors' own numbers unless marked otherwise. "UNVERIFIED" means no primary source
confirmed it. Finnish retrieval scores were computed from the official MTEB results repo.
The script and data are in the scratchpad: `fin.py` and `mteb/`.

## 1. Current roster (working branch `catalog-refresh-2026-09`, pins checked against `main`)

| Recipe id | Model / file | Quant | size_gb (GiB) | Pack / tier | Role | Notes |
|---|---|---|---|---|---|---|
| gemma-4-e2b-it | unsloth gemma-4-E2B-it-qat | UD-Q4_K_XL | 2.44 | `test-comprehensive` only | general, smoke test | Apache-2.0 |
| gemma-4-e4b-it | unsloth gemma-4-E4B-it-qat | UD-Q4_K_XL | 3.93 | ai/models-8gb-ram | general / tools | |
| qwen-4b | bartowski Qwen3.5-4B | Q4_K_M | 2.81 | ai/models-8gb-ram | coding | |
| gemma-4-12b-it | unsloth gemma-4-12B-it-qat | UD-Q4_K_XL | 6.26 | ai/models-16gb-ram | general / tools | |
| qwen-9b | unsloth Qwen3.5-9B | UD-Q4_K_XL | 5.56 | ai/models-16gb-ram | coding | |
| gemma-4-26b-a4b-it | unsloth gemma-4-26B-A4B-it-qat | UD-Q4_K_XL | 13.27 | ai/models-24gb-ram | general MoE (4B active) | |
| qwen-27b | unsloth Qwen3.8-27B @4ca7207 | UD-IQ4_XS | 13.27 | ai/models-24gb-ram | coding (dense) | ships a fixed jinja template |
| gemma-4-31b-it | unsloth gemma-4-31B-it-qat | UD-Q4_K_XL | 16.10 | ai/models-64gb-ram | general (dense) | |
| qwen-27b-hq | unsloth Qwen3.8-27B @4ca7207 | UD-Q6_K_XL | 23.56 | ai/models-64gb-ram | coding (dense) | |
| qwen-35b-a3b | unsloth Qwen3.6-35B-A3B | UD-Q4_K_M | 20.61 | ai/models-64gb-ram | fast MoE coding (3B active) | |
| qwen-27b-mtp | unsloth mtp-Qwen3.8-27B | Q4_0 | 1.28 | ai/mtp (opt-in) | MTP drafter | "no gain" was measured on M4 Max **at b11146** |
| qwen-0.8b | bartowski Qwen3.5-0.8B | Q4_K_M | 0.54 | none (orphan) | tiny | the only unpinned URL (`resolve/main`) |
| nomic-embed-text-v1.5 | nomic-ai GGUF | Q8_0 | 0.14 | tools-base (every tier) | search embedder | 256-dim Matryoshka; English-only in practice (see #1) |

**Runtime and harness pins.** llama-server is b11146 on this branch and **b11840 on
main** (62f2c30). On main, Pi 0.99.1 → 1.4.0, OpenCode 1.18.33 → 1.19.4 and Goose
1.52.0 → 1.56.0.

**Runtime limits today:**
- svalbard ships only llama.cpp **CPU** builds: macOS arm64/x64 and Ubuntu x64/arm64.
- Every model recipe says "text-only runtime", because `--mmproj` is not wired up.
- `drive-runtime/internal/llamaserve/flags.go` picks per-family flags by matching
  substrings of the filename. Each new model family needs a sampler/flags entry there.

**Which preset gets which pack:** default-256 gets the harnesses plus the 8 GB and 16 GB
packs, default-512 the 24 GB pack, default-1tb the 64 GB pack. nomic-embed is in every
tier through tools-base.

## 2. Recommended changes, ranked by value

| # | Change | Kind | Size |
|---|---|---|---|
| 1 | **EmbeddingGemma 2** as the search embedder. Finnish retrieval goes from 37 to 84 nDCG@10 | replace | M (L if titles go into the document prefix) |
| 2 | Turn on **vision (`--mmproj`)** for Qwen3.8-27B, Gemma 4 and Qwen3.6-35B-A3B | add | M |
| 3 | Re-measure speculative decoding on Metal at b11840 or later, and add the **DFlash2 drafter** for Qwen3.8-27B | add, and revisit the MTP decisions | S (M together with `draft_for` metadata) |
| 4 | Repin Qwen3.8-27B to Unsloth **Dynamic 3.1** quants | replace (repin) | S |
| 5 | A fast **MoE coding option for CPU-only x86 hosts** at 24–32 GB (new 32 GB pack) | add (preset) | S |
| 6 | Run the **svalbard MCP server from llama-server**, so the built-in web UI can search the vault | add (runtime) | S |
| 7 | Move llama.cpp to **v1.0.0 = b12359**, for features rather than routine | bump | S, unless the dylib rename breaks packaging |
| 8 | **Inkling 2 Mini**: evaluate in 2–4 weeks as the next 24/32 GB model | watch, then evaluate | S to try, M to adopt |
| 9 | **Qwen3-Reranker-0.6B** for search, opt-in | add | M |
| 10 | **Speech-to-text (ASR)**: try llama-server audio first, then sherpa-onnx + Parakeet v3 | add (optional) | M |
| 11 | Harnesses: re-test Pi 1.6.0, stay on OpenCode 1.x, add Crush | mixed | S each |
| 12 | Clean-up: drop `qwen-0.8b` | drop | S |
| 13 | Linux **Vulkan** build variant for Intel/AMD iGPU laptops | add | M |
| — | Considered and skipped (§3): Muse Glimmer 30B, Nemotron 3.5 Bolt E4B and 74B-A9B, Nemotron 3.5 Lightning, Mellum2.1, Qwen3.8-Flash-Next (there is no 128 GB tier yet), Qwen3-Embedding-0.6B, jina models (non-commercial licence), MLX, ik_llama.cpp, llamafile, Ollama | skip | — |

```text
          harness (pi / opencode / goose / crush)     Llama Chat (browser)
                       │  /v1/chat  model=<id>              │
                       ▼                                    ▼
 ┌────────────── llama-server (b11840+, router mode later) ────────────────┐
 │ --mmproj <sidecar> (#2)  --spec-type draft-dflash (#3)  --mcp-... (#6)  │
 └──────┬──────────────────────┬─────────────────────────┬────────────────┘
        ▼                      ▼                         ▼
  qwen-27b(-hq)           gemma-4-*                 2nd llama-server
  + mmproj 0.9 GB         + mmproj 0.2-1.2 GB       --embedding: EmbeddingGemma 2
  + DFlash2 1.1 GB        (E4B: vision + audio)     (#1) [+ --reranking (#9)]
                                                    → data/search.db
```

---

### #1 EmbeddingGemma 2 as the search embedder (REPLACE nomic-embed-text-v1.5)

**Why.** nomic-embed-text-v1.5 is English-only in practice, so the Finnish ZIMs get
almost nothing from vector rerank. Table: Finnish split of three MMTEB tasks, nDCG@10,
from github.com/embeddings-benchmark/results.

| Model | Licence | GGUF size (Q8_0) | MIRACL-fi | Belebele-fi | Wiki-fi | Mean |
|---|---|---|---|---|---|---|
| bge-m3 | MIT | 634.6 MB (community) | 77.8 | 93.3 | 91.8 | 87.6 |
| snowflake-arctic-embed-l-v2.0 | Apache-2.0 | 634.6 MB (community) | 76.6 | 91.9 | 92.4 | 87.0 |
| nomic-embed-text-v2-moe | Apache-2.0 | 512.2 MB (official) | 76.8 | 92.3 | 90.2 | 86.4 |
| **EmbeddingGemma 2** | **Apache-2.0** | **309.9 MB (ggml-org)** | 73.5 | 87.5 | 89.8 | **83.6** |
| Qwen3-Embedding-0.6B | Apache-2.0 | 639.2 MB (official) | 70.3 | 85.5 | 86.7 | 80.8 |
| granite-embedding-311m-multilingual-r2 | Apache-2.0, but its tokenizer falls under the Gemma Terms | 347.0 MB (community) | 68.5 | 84.0 | 87.0 | 79.9 |
| nomic-embed-text-v1.5 (current) | Apache-2.0 | 0.14 GiB | **15.9** | **44.8** | **51.5** | **37.4** |

The MIRACL-fi lead of bge-m3, arctic and nomic-v2 is partly training-set exposure: they
trained on MIRACL data. Belebele and Wiki are nearly saturated.

**Recommendation: EmbeddingGemma 2** (Google, released 2026-09/10).
- **File:** `ggml-org/embeddinggemma-2-GGUF` → `embeddinggemma-2-Q8_0.gguf`, **309.9 MB
  (0.29 GiB)**. A BF16 file (558 MB) and an Unsloth `UD-Q6_K_XL` (248.8 MB) also exist.
  The `mmproj` in the repo (555 MB) is only for image/audio embedding, not needed here.
- **Licence:** Apache-2.0, confirmed from the HF API card data. The earlier
  EmbeddingGemma-300m is under the Gemma Terms and is manually gated.
- **Vectors:** 768 dims, Matryoshka at 512/256/128. Keep **256 dims**, so vector storage
  stays the same as today. MMTEB (multilingual v2) mean: 61.36 at 768 dims, 60.41 at 256.
- **Input:** up to 8192 tokens (sliding window 1024), so today's 2048-token chunks fit.
- **Prefixes:** query `task: search result | query: `, document
  `title: {title or "none"} | text: `. The current recipe fields already cover this:
  `query_prefix`, `doc_prefix`, `dims: 256`, `matryoshka: true`, `max_input_tokens`.
- **Compute:** about 130M transformer parameters, close to nomic-v1.5's 137M, so
  re-indexing should take about as long as it does now.
- **llama.cpp:** PR #30054, merged 2026-10-06. That is in **b11840 (main) but not in
  b11146 (this branch)**.
- **Caveat:** the model card says "do not use float16". Check the Metal build for NaNs or
  drift: embed the same text on CPU and Metal and compare cosine similarity.

**If Finnish quality is worth about 3 points at 3× the indexing compute**, use
snowflake-arctic-embed-l-v2.0 instead: XLM-R-large, 1024 dims with Matryoshka down to
256, 8192 tokens, query prefix `query: `, Apache-2.0. Its GGUF is community-made
(`Casual-Autopsy/snowflake-arctic-embed-l-v2.0-gguf` q8_0, 634.6 MB); a self-conversion
would be cleaner. Decide by testing on one real Finnish ZIM, for example fi-wikipedia.

**Ruled out:**
- **Qwen3-Embedding-0.6B**, the previously planned opt-in: weaker on Finnish (80.8) at
  about 4× nomic's compute. Drop the plan.
- **jina-embeddings v3/v5 and all jina rerankers:** CC-BY-NC-4.0.
- **nomic-embed-text-v2-moe** and **multilingual-e5-large-instruct:** 512-token input
  limit, so everything would need re-chunking.
- **pplx-embed v1/v2:** custom architectures, not usable through llama-server's
  `/embedding`.
- **harrier-oss-v1-0.6b** (MIT, MMTEB 69.0, Finnish 84.1): only a community GGUF, and
  its pooling metadata is UNVERIFIED. Worth a later look.

**How to roll it out:**
- **Replace, don't add alongside.** At build time, host-cli `FindEmbeddingModel` takes
  the first `.gguf` in `models/embed/`, and `embeddinggemma-2-…` sorts before `nomic-…`.
  A drive holding both would silently switch embedders.
- Every `data/search.db` has to be rebuilt, because the index records the model, its
  hash, prefixes and dims.
- **Checked in this branch:** the 2026-09-30 backlog audit's concern is already fixed at
  query time. `drive-runtime/internal/search/semantic.go` `resolveRecordedModel` opens
  only the model named in the index's semantic contract and checks its sha256. Only the
  host indexer (`host-cli/internal/embedder/embedder.go` `FindEmbeddingModel`) still
  globs. After the swap, a stale index should fall back to keyword search (the audit
  describes that fallback) instead of comparing vectors from mismatched models. That
  fallback was not tested here.
- Pass the article title in the document prefix later. Start with `title: none`.

**Size:** M. It becomes L if titles have to be plumbed through chunk metadata. Risk:
indexing time for very large ZIMs on CPU-only hosts. Compute is about the same as today,
but measure it.

### #2 Turn on vision with `--mmproj` (ADD: runtime plus sidecar recipes)

All three model families on the stick are natively multimodal, and their pinned Unsloth
repos ship projector files. Image input is the biggest capability gap for an offline
vault: labels, manuals, schematics, maps, handwritten notes, OCR of photographed pages,
and plant identification. Plant identification needs a safety disclaimer and must never
be used to decide whether something is edible.

| Target recipe | Projector file (same repo as the model) | Size | Notes |
|---|---|---|---|
| qwen-27b / qwen-27b-hq | `unsloth/Qwen3.8-27B-GGUF` → `mmproj-F16.gguf` | 928 MB (0.86 GiB) | image and video input |
| qwen-35b-a3b | `unsloth/Qwen3.6-35B-A3B-GGUF` → `mmproj-F16.gguf` | 899 MB (0.84 GiB) | |
| gemma-4-e4b-it | `unsloth/gemma-4-E4B-it-qat-GGUF` → `mmproj-F16.gguf` | 990 MB (0.92 GiB) | **vision and audio** encoder |
| gemma-4-e2b-it | `unsloth/gemma-4-E2B-it-qat-GGUF` → `mmproj-F16.gguf` | ~1 GB (size UNVERIFIED) | vision and audio |
| gemma-4-12b-it | `unsloth/gemma-4-12B-it-qat-GGUF` → `mmproj-F16.gguf` | 175 MB (0.16 GiB) | cheapest win |
| gemma-4-26b-a4b-it | `unsloth/gemma-4-26B-A4B-it-qat-GGUF` → `mmproj-F16.gguf` | 1.19 GB (1.11 GiB) | |
| gemma-4-31b-it | `unsloth/gemma-4-31B-it-qat-GGUF` → `mmproj-F16.gguf` | 1.2 GB (1.12 GiB) | |

- **Licence:** the same as each model (Apache-2.0). Pin each projector to the same repo
  revision as its model file.
- **llama.cpp:** `--mmproj FILE` already works in b11840 for Qwen3.5/3.6/3.8 and Gemma 4,
  with GPU offload on by default. Later builds add Gemma 4 streaming audio input (#31965,
  ≈ b12237) and resizing without letterbox padding (#30922).
- **Breaking change in v1.0.0 (#32115):** `image_url`s that point at local or private
  addresses are refused. Harnesses must send base64 data URLs, or the server needs
  `--media-path`.
- **Work:**
  - A `gguf-mmproj` sidecar recipe type, like `gguf-mtp`, installed next to its model and
    picked up by `llamaserve`.
  - Mark image input in the generated harness configs: pi `models.json` with
    `input: ["text","image"]`, OpenCode `modalities`, and Goose.
  - Keep projectors optional per pack, since each adds 0.2–1.2 GiB per model. Either an
    `ai/vision` pack like `ai/mtp`, or default-on only for 12B (175 MB) and the 27B.
  - Drop "text-only runtime" from the recipe descriptions.
- **Size:** M. Risk: image handling differs per harness, and pi or OpenCode may need
  extra flags to send images to a custom provider.

### #3 Re-measure speculative decoding on Metal and add DFlash2 for Qwen3.8-27B (ADD / REVISIT a decision)

**Why the old verdicts are stale.** Both MTP verdicts (Gemma dropped; Qwen opt-in with no
gain) were measured on an M4 Max at **b11146**. A later llama.cpp fix targets exactly the
slowdown svalbard measured:
- **PR [#29869](https://github.com/ggml-org/llama.cpp/pull/29869)**, "metal : few-row MMA
  mat-mul and batched copies for speculative decoding", merged 2026-10-05 (in v0.6.0 =
  b11429; verified with `gh`). The PR says that on Apple GPUs without the tensor API,
  **M1 to M4**, the 2–16-row mat-muls that verify drafts ran on mat-vec kernels "whose
  time grows with each row". Drafting was therefore slower than plain decoding.
- **PR #30065** (2026-10-07) adds the IQ types, including IQ4_XS, the `qwen-27b` quant.
- **The PR's own test** (M3 Ultra, Qwen3.8-27B Q4_0): DFlash2 went from 30 to 110 tok/s
  on code and from 17 to 63 on prose. Without a drafter it runs at 32 tok/s.
- Both PRs are already in main's b11840.

**The drafter:** `z-lab/Qwen3.8-27B-DFlash2-GGUF` → `Qwen3.8-27B-DFlash2-Q4_K_M.gguf`.
- **Size and licence:** 1.14 GB (1.06 GiB), Apache-2.0. A Q8_0 (2.06 GB) also exists.
- **Measured gains so far:** about 5.1–5.4 tokens accepted per step at
  `--spec-draft-n-max 7`. A community RTX 3090 run went from 42 to 108 tok/s (2.6×) with
  Qwen3.8-27B UD-Q4_K_M.
- **Unknown:** there are no published Metal numbers on b11840 or later. **Measure.**

**llama.cpp:**
- DFlash (#22105) and DFlash2 (#27342, merged 2026-08-27, verified) are both in b11840.
- At b11840 the flags are `--spec-type draft-dflash --spec-draft-model <file>
  --spec-draft-n-max 7`. The explicit `draft-dflash2` name arrives ≈ b12377 (#32016), and
  older drafter files still load under `draft-dflash`.
- Use the z-lab file. Drafter GGUFs converted by forks use the arch name `dflash-draft`
  and fail to load in mainline (issue #25116).

**Plan:**
- One bench session on the M4 Max at b11840 (or b12359). Compare a baseline, MTP at n=1
  and n=2, and DFlash2 at n=4 and n=7, with short and 5k-token prompts, for `qwen-27b`
  and `qwen-27b-hq`.
- Re-run Gemma 4 12B/31B MTP in the same session. The drafters are
  `mtp-gemma-4-12B-it.gguf` (254 MB) and `mtp-gemma-4-31B-it.gguf` (280 MB). Community
  DFlash drafters also exist for gemma-4-31B and Qwen3.5-9B (Anbeeld); their quality is
  UNVERIFIED.
- Also try `--spec-type ngram-mod`. It needs no extra model and suits agent edit loops;
  its gain is UNVERIFIED.
- If DFlash2 wins, it replaces `qwen-27b-mtp` in `ai/mtp`. Its presence stays the
  opt-in switch, but the filename match in `packagedDraft` has to change. This is a good
  moment for the planned `draft_for:` recipe metadata.
- Update the memory file whichever way the numbers go.

**Tier/preset:** `ai/mtp` (opt-in), used with the 24 GB and 64 GB packs.
**Size:** S to measure; M if it lands together with `draft_for`. Risk: if Metal still
shows no gain, keep it opt-in for Linux/CUDA users.

### #4 Repin Qwen3.8-27B to Unsloth Dynamic 3.1 (REPLACE: repin)

- Unsloth re-uploaded most Qwen3.8-27B quants as "Dynamic v3.1" on 2026-11-10 to 11-12:
  commits `43f522d` and `acb953b`, latest `4af51b3`. Unsloth claims "5–10% more average
  accuracy than other frontier quants" on Terminal Bench, DeepSWE and LiveCodeBench. That
  is its own claim, and no KLD table was published.
- **`qwen-27b`:** `Qwen3.8-27B-UD-IQ4_XS.gguf` is now **14.4 GB (13.4 GiB)**, up from
  13.27 GiB, and still fits the 24 GB pack.
- **`qwen-27b-hq`:** `UD-Q6_K_XL` was **not** re-uploaded and is still the August file.
  New UD3.1 files in that range are `UD-Q6_K` 22.0 GB, `UD-Q6_K_M` 23.1 GB and
  `UD-Q6_K_L` 24.2 GB. Use `UD-Q6_K_M` (21.5 GiB, smaller than today's 23.56 GiB) if it
  measures the same or better.
- **Chat template:** the commit log has no template or system-message fix since August,
  so keep shipping `qwen3.8.jinja`. Pi 1.6.0 changes how it sends mid-conversation system
  messages (see #11); re-test the two together.
- **Licence:** Apache-2.0. Same `qwen3.8` flags. Tiers: 24 GB and 64 GB packs.
- **Size:** S. Risk: the new quant must pass the pi/OpenCode/Goose read→write tool task
  again. Re-measure decode speed too, since the IQ4_XS change may shift Metal kernel paths
  (see #3).

### #5 Fast MoE coding option for CPU-only x86 hosts at 24–32 GB (ADD: preset)

**Why.** On an x86 laptop without a usable GPU, decode speed is roughly memory bandwidth
divided by the bytes read per token, which is set by the active weights. Dual-channel
DDR5 (about 70–90 GB/s) gives:
- about 5–6 tok/s for the 13.3 GiB dense `qwen-27b`;
- about 20–30 tok/s for a MoE with about 3B active parameters.

NVIDIA's same-hardware table shows the gap even on a GPU: Qwen3.6-35B-A3B UD-Q4_K_M + MTP
runs at 96.6 tok/s, Gemma 4 26B-A4B + MTP at 100.6, and Qwen3.8-27B UD-Q4_K_M + DFlash at
28.3 ([Nemotron 3.5 Bolt page](https://research.nvidia.com/labs/nemotron/Nemotron-3.5-Bolt-24GB/)).

The 24 GB pack already has a fast general MoE (gemma-4-26b-a4b), but its only coding
model is dense. Two options:
- **(a) Recommended:** add the existing `qwen-35b-a3b` recipe to a new
  `ai/models-32gb-ram` pack: gemma-4-26b-a4b 13.27 + qwen-27b 13.4 + qwen-35b-a3b 20.61,
  about 47 GiB. Put the pack in default-512, in place of or next to the 24 GB pack.
- **(b)** A second, smaller recipe `qwen-35b-a3b-lite`: `unsloth/Qwen3.6-35B-A3B-GGUF` →
  `Qwen3.6-35B-A3B-UD-IQ4_XS.gguf`, **17.7 GB (16.5 GiB)**, for the 24 GB pack. That is
  tight on a 24 GB Mac (the default GPU working-set limit is about 16–18 GB) but fine on
  x86 with 24–32 GB.

**Licence:** Apache-2.0. The model was supported long before b11146, and its flags
(`qwen3.6`) already exist.
**Size:** S (option (a) is preset-only). Risk: the preset target sizes and rebalance
numbers in the catalog-update audit change.

### #6 Run the svalbard MCP server from llama-server (ADD: runtime)

- llama-server can launch stdio MCP servers itself through `--mcp-servers-config FILE`
  or `--mcp-servers-json` (#26062, already in b11840).
- svalbard's MCP server already speaks stdio. Connected this way, the built-in
  **Llama Chat** web UI gets vault search without any terminal harness, which helps
  non-technical users of the stick.
- Since v1.0.0 (#32113), tools on a non-loopback `--host` need `--api-key` or
  `--allow-unauthenticated-tools`. The LAN-bind ("serveall") path must handle this.
- Don't enable the built-in `--tools` / `--agent` file and shell tools on the stick.
- **Size:** S. Risk: the redesigned UI hides the MCP and Models panels by default
  (#32527), so check that users can find them.

### #7 llama.cpp to v1.0.0 = b12359 (BUMP, for the features)

The version-bump agent owns routine pins. These are the model-related reasons to go to
b12359 rather than stop at b11840.

**What it adds:**
- Faster prompt processing for Gemma 4 E2B/E4B (#31356) and for multiple sequences
  (#32342).
- MTP hidden state saved and restored in checkpoints (#30141).
- The template's reasoning-effort levels exposed through the API (#31271).
- Context checkpoints shared by reference instead of copied, so less RAM (#30137).
- `--models-memory-margin` for router mode.

**Release-scheme gotcha:** since 2026-08-21, numbered releases (v0.2.0…v1.0.0) carry **no
binaries**. Binaries live on `bNNNNN` tags marked pre-release, so "latest release"
resolves to an empty v1.0.0.
- Mapping: v0.6.0 = b11429, v0.7.0 = b11669, v0.8.0 = b11960, v0.9.0 = b12110,
  **v1.0.0 = b12359**.
- Latest nightly today: b12602.

**Behaviour changes:**
- Default port is 9931; svalbard passes `--port`, so no effect.
- `--jinja` and `--reasoning-preserve` are on by default; passing them is harmless.
- `--defrag-thold` was removed; svalbard doesn't use it.
- `--cache-ram 8192` keeps an 8 GiB host-RAM prompt cache by default. **Lower it on 8 and
  16 GB hosts.**
- `-np auto` is the default; svalbard already forces `--parallel 1`.
- Library names changed from `libggml-*.0.dylib` to `.1.dylib`.

**Size:** S, unless the dylib rename trips packaging or signing.

### #8 Inkling 2 Mini (EVALUATE: likely an add for 24/32 GB, possibly replacing qwen-35b-a3b)

**What it is:** Thinking Machines Lab, released **2026-11-20**.
([model card](https://huggingface.co/thinkingmachines/Inkling-2-Mini),
[HF blog](https://huggingface.co/blog/inkling2mini))
- 26.4B total, **3.4B active**: the middle 12 layers loop 3×, giving 5.4B effective.
  128 experts, 8 routed per token.
- Text and image input, 1M context, **Apache-2.0**.

**Why.** In the vendor's table it matches Qwen3.8-27B while running at MoE speed:

| Benchmark | Inkling 2 Mini | Qwen3.8-27B | Gemma 4 31B |
|---|---|---|---|
| AA Index v5.0 | 20.1 | 20.1 | 4.4 |
| SWE-bench Verified | 79.2 | 77.8 | 46.1 |
| Terminal-Bench 2.1 | 71.6 | 73.0 | 43.4 |
| MCP Atlas | 82.7 | 77.3 | 54.2 |
| LongBench v2 | 66.9 | 52.3 | 52.9 |
| BrowseComp | 79.0 | 61.6 | 37.0 |

**Files:** `ggml-org/Inkling-2-Mini-GGUF` → `Inkling-2-Mini-IQ4_NL.gguf` **17.6 GB
(16.4 GiB)**; `Inkling-2-Mini-Q8_0.gguf` 27.8 GB; `mmproj-Inkling-2-Mini-Q8_0.gguf`
346 MB. Only IQ4_NL and Q8_0 exist so far; Unsloth builds are announced.

**llama.cpp:** the architecture landed in **b12600 (2026-11-21, nightly only)**. It is
not in b11840 or v1.0.0.

**Caveats:**
- It is one day old, and runtime support for looped models is new; the card says "speed
  varies".
- The card notes weaker output outside English (mixed languages in Chinese tests).
  Finnish is untested.
- Setting reasoning effort to its minimum does not fully turn thinking off, and it can
  loop while debugging. Cap output tokens and agent turns.
- It needs a `llamaserve` sampler entry.

**Recommendation:** revisit in 2–4 weeks, once a numbered llama.cpp release includes it
and Unsloth quants exist. Then test it as the coding/agentic MoE for
`ai/models-32gb-ram`, and as a replacement for `qwen-35b-a3b` (from April 2026) in the
64 GB pack. Keep Gemma for the Finnish/general role; Gemma 4 31B is reported first on
Finnish in EuroEval, but that comes from a blog citing r/LocalLLaMA and is UNVERIFIED.

**Size:** S to try; M to adopt (new family flags and template checks with all three
harnesses).

### #9 Qwen3-Reranker-0.6B for search (ADD, optional)

- **File:** `ggml-org/Qwen3-Reranker-0.6B-Q8_0-GGUF` → `qwen3-reranker-0.6b-q8_0.gguf`,
  **639.2 MB (0.60 GiB)**. Apache-2.0, 32K tokens, MMTEB-R 66.36.
  - Use this exact conversion: most community GGUFs return garbage scores (llama.cpp
    issue #16407).
  - `Voodisss/Qwen3-Reranker-0.6B-GGUF-llama_cpp` Q4_K_M (0.37 GB) is reported to lose
    only 0.3%.
- **Alternative:** `gpustack/bge-reranker-v2-m3-GGUF` → `bge-reranker-v2-m3-Q8_0.gguf`,
  635.7 MB, Apache-2.0, MMTEB-R 58.36.
- **Excluded:** jina rerankers v2/v3/v3.5 (CC-BY-NC; v3 also needs a fork). No Finnish
  reranker benchmarks exist, so Finnish quality is UNVERIFIED.
- **llama.cpp:** `--rerank` (alias `--reranking`) or `--embedding --pooling rank`, with
  the `POST /reranking` endpoint. The server fixes for causal rerankers (#28876, #30864)
  are in b11840.
- **Placement:** rerank the top 20–30 results after RRF. That needs a second
  llama-server, or router mode.
- **Cost:** several thousand tokens per query through a 0.6B cross-encoder, which could
  take seconds on CPU (estimate). Measure it, and do it after #1, which closes the bigger
  Finnish gap.
- **Size:** M.

### #10 Speech-to-text (ASR) (ADD, optional)

Try these in order:
1. **No new binary: speech through the shipped llama-server.**
   - **Qwen3-ASR-0.6B:** `ggml-org/Qwen3-ASR-0.6B-GGUF`, Q8_0 804.7 MB plus `mmproj` Q8_0
     214.4 MB. Apache-2.0. Finnish is listed; its Finnish WER is UNVERIFIED.
   - **Gemma 4 E4B audio:** comes for free once #2 ships its projector.
   - **Endpoints:** `/v1/audio/transcriptions` (PR #21863, in b11840) and the
     `/v1/realtime` WebSocket.
   - **Rumble-2-Flash** (Reson8, 0.5B, Apache-2.0, a Qwen3-ASR fine-tune that lists
     Finnish): `AtomicChat/Rumble-2-Flash-GGUF`, Q8_0 346 MB plus mmproj 214 MB. Tested
     on b12521. Known issue: it invents trailing words on padded audio.
2. **If Finnish WER is poor: sherpa-onnx v2.0.2 with Parakeet TDT 0.6B v3.**
   - sherpa-onnx is Apache-2.0, with prebuilt osx-arm64/x64 and linux-x64/aarch64
     binaries.
   - The model is `sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8`, 487 MB, CC-BY-4.0, with
     **FLEURS-fi WER 13.21%**.
   - The same binary does Finnish TTS with the Piper voice `vits-piper-fi_FI-harri-medium-int8`
     (21 MB, CC0 dataset). Check the espeak-ng (GPL) licensing first.
   - NVIDIA NeMo-Speech.cpp v0.3.0 is an alternative with 7–10 MB binaries for all four
     targets, including Metal.
3. **For 32 GB+ only:** Nemotron 3.5 Transcribe 30B-A4B. It has the best Finnish result
   found (FLEURS-fi 6.27), is OpenMDW-1.1, about 22 GB, and needs llama.cpp ≥ b12072.

**Skipped:**
- **whisper.cpp:** still no macOS CLI binary. v1.9.5 has no assets; the rolling b5907
  has Linux only. `ggml-large-v3-turbo-q5_0.bin` would be 574 MB.
- **Kokoro TTS:** no Finnish.
- **LFM3-Audio:** its LFM licence is not OSI-approved.

**Size:** M, mostly for the UI or harness that consumes transcripts.

### #11 Harnesses

- **Pi 1.6.0 (2026-11-20):**
  - It now sends system messages, including mid-conversation ones, with the `system`
    role to non-OpenAI providers (#10741). Re-test against the fixed Qwen3.8 template:
    this is exactly the "System message must be at the beginning" path.
  - Its built-in llama.cpp extension now defaults to port 9931 and targets router mode.
    Keep the provider id `svalbard`.
  - Set `PI_OFFLINE=1` (or `--offline`) in the generated launchers to stop pi.dev catalog
    and update checks.
  - 1.5.1 added the compatibility knobs `requiresNonNullAssistantContent` and
    `promptCacheKeyMode`.
- **OpenCode:**
  - 1.x is maintenance-only (v1.19.6, 11-17), and 2.x (v2.3.2, 11-18) breaks 1.x plugins.
    Stay on 1.19.x for now and plan a 2.x migration separately.
  - 2.3.1 stops resending old reasoning to models that use `reasoning_content`.
  - Offline, a failed models.dev fetch is now skipped (#59475), but offline hangs are
    still reported (issue #18492).
- **Goose 1.60.2 (11-20):** set `GOOSE_TELEMETRY_ENABLED=false` in the launcher.
- **Add Crush** (charmbracelet, v0.102.0): a single Go binary that works with plain chat
  completions and has MCP and an offline switch. Its licence is **FSL-1.1-MIT**, which
  allows redistribution for non-competing use and becomes MIT after two years. Confirm
  that is acceptable for a stick.
- **Skip:**
  - Codex CLI: it only speaks the Responses API to custom providers, and reports against
    llama-server are poor.
  - Aider: no release since 2025-08.
- **Watch:** Qwen Code, Hermes Agent, Kilo CLI.
- **Size:** S each.

### #12 Clean-up (DROP)

- **Drop `qwen-0.8b`.** It is orphaned, and its URL is the only unpinned one in the
  roster (`resolve/main`).
- `gemma-4-e2b-it` (2.44 GiB, Apache-2.0, already used by `test-comprehensive`) covers
  the smoke-test role.
- If a model under 1 GB is still wanted for default-2/32, pin a revision.
- **Size:** S.

### #13 Linux Vulkan build variant (ADD: runtime)

- llama.cpp now publishes `llama-bN-bin-ubuntu-vulkan-x64.tar.gz` and an arm64
  equivalent.
- On Intel Arc/Xe and AMD iGPU laptops it speeds up prompt processing a lot, and the
  large prompts agent harnesses send dominate the wait. Decode speed stays
  bandwidth-bound.
- It needs a Vulkan loader on the host (`libvulkan1`, present on most desktop distros).
  The runtime must fall back to the CPU build when device init fails.
- Windows builds (`win-cpu`, `win-vulkan`) exist too, if svalbard ever adds Windows.
- **Size:** M (a second binary per platform plus selection and fallback logic). Risk:
  driver bugs on old iGPUs, so the fallback must be clean.

---

## 3. Considered and skipped

| Candidate | Facts | Verdict |
|---|---|---|
| **Muse Glimmer 30B** (Meta, 2026-08-10, dense, vision, Apache-2.0). File: `unsloth/Muse-Glimmer-30B-GGUF` UD-Q4_K_XL 15.9 GB; mmproj 1.4–3.85 GB | Beats Gemma 4 31B on agentic tasks (MCP Atlas 75.5 vs 54.2) but trails Qwen3.6-27B on coding. Its AA Index is far below Qwen3.8-27B (6.3 vs 20.1 in the Inkling table) | Skip. Qwen3.8-27B is better and Glimmer isn't worth 15 GB |
| **Nemotron 3.5 Bolt E4B 4GB** (NVIDIA, 2026-10-23, Gemma-4-E4B base, OpenMDW-1.1). File: `Nemotron-3.5-Bolt-E4B-Q4_K_M.gguf` 6.18 GB, plus a DFlash drafter | Beats Gemma 4 E4B: tau2 55.1 vs 38.5, IFBench 63.8 vs 42.8. Qwen3.5-4B is still ahead (tau2 75.1, BFCL 70.9). The file is bigger than the QAT E4B (3.93 GiB), text-only, and Finnish is not in its post-training languages | Skip / watch |
| **Nemotron 3.5 Bolt 74B-A9B 24GB** (2026-10-15) | Custom mixed IQ2_XXS + NVFP4 GGUF. The vendor's own table shows it slower than Qwen3.6-35B-A3B, and there is no accuracy table in the text | Skip |
| **Nemotron 3.5 Lightning 30B-A3B** (2026-08-11, OpenMDW) | SWE-bench Verified 51.6, below Qwen3.6-35B-A3B | Skip |
| **Mellum2.1-12B-A2.5B** (JetBrains, 2026-10-08, Apache-2.0) | SWE-bench Verified 47.0 vs Qwen3.5-9B's 50.0. English-only, and the official GGUF is "coming soon" | Skip. Revisit only for CPU-only 16 GB coding |
| **Qwen3.8-Flash-Next** (125B + 51B engram, 6B active, 2026-08-26) | Beats Qwen3.8-27B on every row listed (DeepSWE 58.7 vs 42.2, SWE-Pro 62.5 vs 61.7, SWE-Multilingual 81.0 vs 73.8). Smallest useful GGUFs: UD-IQ2_XS 75.8 GB, UD-Q3_K_XL 89.5 GB. **Licence is `qwen-community-1.0`, not Apache; redistribution terms UNVERIFIED.** Engram loading (`--ple`) needs b12233+, and MTP only works on an Unsloth branch | Skip until svalbard has a 128 GB pack and the licence is checked |
| **Olala mini 7A1B** (OVHcloud, 2026-11-17, Apache-2.0 announced) | No published weights, GGUF or benchmarks as of 11-18 | Watch |
| **Qwen3-Coder-Next 80B-A3B** (Feb 2026) | UD-IQ4_XS 38.4 GB; older than Qwen3.8-27B and qwen-35b-a3b | Skip |
| **Qwen3-Embedding-0.6B** | Finnish mean 80.8, below EmbeddingGemma 2's 83.6, at about 2× the file size and about 4× the compute | Drop the planned opt-in |
| **MLX / mlx-lm server** | Mac-only, needs Python and wheels, and stores each model a second time. In a public M5/M1 Max benchmark, llama.cpp was equal or 10–24% faster on Qwen3.6 27B and 35B-A3B | Skip |
| **ik_llama.cpp** | No official binaries, no Metal, and quant types only it can read | Skip (revisit for x64 CPU-only) |
| **llamafile 0.10.6** | About 2,000 builds behind mainline | Skip as the engine |
| **Ollama / LM Studio** | Ollama re-imports GGUFs into its own store; LM Studio is proprietary and installs into the home folder | Skip |

## 4. Sources

- **llama.cpp**
  - Releases: https://github.com/ggml-org/llama.cpp/releases
  - PR #29869 (Metal fix for speculative decoding): https://github.com/ggml-org/llama.cpp/pull/29869
  - PR #27342 (DFlash2): https://github.com/ggml-org/llama.cpp/pull/27342
  - PR #30054 (EmbeddingGemma 2): https://github.com/ggml-org/llama.cpp/pull/30054
  - PR #21863 (audio transcriptions): https://github.com/ggml-org/llama.cpp/pull/21863
  - Issue #25116 (DFlash drafter load failure): https://github.com/ggml-org/llama.cpp/issues/25116
  - Issue #16407 (reranker conversions): https://github.com/ggml-org/llama.cpp/issues/16407
  - Release scheme: https://github.com/ggml-org/ggml/discussions/1579
  - Server README on master and at b11840: scratchpad `server-README.md`, `server-README-b11840.md`
- **Chat and coding models**
  - Qwen3.8-27B GGUF (files and commits): https://huggingface.co/unsloth/Qwen3.8-27B-GGUF
  - DFlash2 drafter: https://huggingface.co/z-lab/Qwen3.8-27B-DFlash2-GGUF and https://huggingface.co/Anbeeld/Qwen3.8-27B-DFlash2-GGUF
  - Qwen3.6-35B-A3B: https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF
  - Gemma 4: https://huggingface.co/unsloth/gemma-4-E4B-it-qat-GGUF, https://huggingface.co/unsloth/gemma-4-12B-it-qat-GGUF, https://huggingface.co/unsloth/gemma-4-26B-A4B-it-qat-GGUF, https://huggingface.co/unsloth/gemma-4-31B-it-qat-GGUF
  - Inkling 2 Mini: https://huggingface.co/thinkingmachines/Inkling-2-Mini, https://huggingface.co/blog/inkling2mini, https://huggingface.co/ggml-org/Inkling-2-Mini-GGUF
  - Qwen3.8-Flash-Next: https://huggingface.co/Qwen/Qwen3.8-Flash-Next, https://huggingface.co/unsloth/Qwen3.8-Flash-Next-GGUF, https://unsloth.ai/docs/models/qwen3.8-next
  - Muse Glimmer: https://huggingface.co/blog/muse-glimmer, https://unsloth.ai/docs/models/muse-glimmer
  - Nemotron 3.5: https://research.nvidia.com/labs/nemotron/NVIDIA-Nemotron-3.5-Bolt-E4B-4GB/, https://huggingface.co/nvidia/NVIDIA-Nemotron-3.5-Bolt-E4B-4GB, https://research.nvidia.com/labs/nemotron/Nemotron-3.5-Bolt-24GB/
  - Mellum2.1: https://huggingface.co/JetBrains/Mellum2.1-12B-A2.5B-Thinking
  - Overviews: https://vdf.ai/blog/best-local-llm-for-coding/, https://pasqualepillitteri.it/en/news/14005/best-local-llm-2026
  - Release trackers: https://www.nocode.mba/ai-release-tracker/2026/10, https://www.nocode.mba/ai-release-tracker/2026/11
- **Embedders and rerankers**
  - EmbeddingGemma 2: https://huggingface.co/google/embeddinggemma-2, https://huggingface.co/ggml-org/embeddinggemma-2-GGUF
  - Snowflake arctic-embed-l-v2.0: https://huggingface.co/Snowflake/snowflake-arctic-embed-l-v2.0
  - Qwen3-Embedding-0.6B: https://huggingface.co/Qwen/Qwen3-Embedding-0.6B
  - Finnish MTEB results: https://github.com/embeddings-benchmark/results
  - Qwen3-Reranker-0.6B: https://huggingface.co/ggml-org/Qwen3-Reranker-0.6B-Q8_0-GGUF
  - bge-reranker-v2-m3: https://huggingface.co/gpustack/bge-reranker-v2-m3-GGUF
- **Speech**
  - Qwen3-ASR-0.6B: https://huggingface.co/ggml-org/Qwen3-ASR-0.6B-GGUF
  - Rumble-2-Flash: https://huggingface.co/Reson8/Rumble-2-Flash
  - Parakeet TDT 0.6B v3: https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3
  - sherpa-onnx: https://github.com/k2-fsa/sherpa-onnx/releases
  - NeMo-Speech.cpp: https://github.com/NVIDIA/NeMo-Speech.cpp/releases
  - whisper.cpp: https://github.com/ggml-org/whisper.cpp/releases
  - Nemotron 3.5 Transcribe: https://huggingface.co/nvidia/NVIDIA-Nemotron-3.5-Transcribe-30B-A4B
  - Piper Finnish voice: https://huggingface.co/rhasspy/piper-voices/tree/main/fi/fi_FI/harri
- **Harnesses**
  - Pi: https://github.com/earendil-works/pi/releases/tag/v1.6.0
  - OpenCode: https://github.com/anomalyco/opencode/releases/tag/v1.19.6
  - Goose: https://github.com/aaif-goose/goose/releases/tag/v1.60.0
  - Crush: https://raw.githubusercontent.com/charmbracelet/crush/main/README.md
- **Backends**
  - MLX server docs: https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/SERVER.md
  - Apple Silicon benchmark: https://github.com/stared/benching-local-llms-on-apple-silicon
  - ik_llama.cpp: https://github.com/ikawrakow/ik_llama.cpp
  - llamafile: https://github.com/mozilla-ai/llamafile/releases/tag/0.10.6
