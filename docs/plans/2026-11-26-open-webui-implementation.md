# Open WebUI opt-in: implementation record

Status: phase A done 2026-11-26 on macOS arm64. Not run on Linux, Intel Mac or a physical stick.

Scope: work items 1 to 5 of the [research](research/2026-11-26-open-webui.md). llama.cpp's own page stays the default Chat entry and is untouched. Open WebUI is an opt-in recipe, in no pack or preset.

## Resulting user path

1. Add `open-webui` to a vault that has a chat model and apply. The recipe pulls `svalbard-python` and `llama-server` as dependencies in the wizard; the plain `svalbard add` command adds only what it is given, so `uv`, `svalbard-python` and `llama-server` must be named there (existing behaviour, see follow-ups).
2. On the drive, the Local AI group gains one entry, **Open WebUI**, and `./run open-webui` does the same. It asks for the context size as Chat does, starts three things, waits until the model and Open WebUI answer, opens the browser and prints the URL.
3. The page opens signed in, with the model selected and the vault's tools attached. Chats are kept on the drive between runs.
4. Ctrl+C stops all three servers.

```text
./run open-webui
      │
      ▼
┌───────────────────────────────┐ starts  ┌───────────────────────────────┐
│ svalbard-drive                │────────▶│ llama-server            :8082 │
│ __native-open-webui           │         │ one model, flags as Chat      │
│                               │         └───────────────▲───────────────┘
│ ┌───────────────────────────┐ │                         │ /v1
│ │ MCP server, in process    │ │ starts  ┌───────────────┴───────────────┐
│ │ Streamable HTTP :8090/mcp │◀┼─────────│ open-webui serve        :8084 │
│ └───────────────────────────┘ │ ───────▶│ slim, no sign-in, env config  │
└───────────────────────────────┘  tools  └───────────────┬───────────────┘
                                                          │ state
                              <drive>/.svalbard/runtime/open-webui/
```

## Decisions

- **Recipe field `requirements_from`**, not `deps_from`. `deps` already means recipe dependencies. When set, the python-package builder installs the packages with `--no-deps`, then installs the named requirements file from the installed tree. The field is part of the rebuild stamp, and a path outside the tree is rejected. No `prune` field: source maps stay (follow-up).
- **Dependency resolution, not upstream's hash lock**, because the lock does not install on Intel Mac.
- **The menu entry is a fixed built-in**, shown when a realized `open-webui` entry exists. The label is in code, not in the recipe, so it cannot drift from "Open WebUI" (the licence's branding clause). Python packages have one realized entry per platform; the built-in gives one menu entry regardless.
- **Open WebUI gets a clean environment**: PATH, locale and time zone from the host, everything else set by the runtime. It reads several hundred variables; `OPENAI_API_BASE_URLS` from the host would override the local endpoint, and with no endpoint at all it defaults to `api.openai.com`.
- **`CORS_ALLOW_ORIGIN` is the page's own origin.** With sign-in off, the default `*` would let any page in the host's browser read the chats.
- **`llama-server` gets `--alias <file name>` and `--jinja`** on top of Chat's arguments. The alias is the id the model picker shows (otherwise the full path); `--jinja` is what tool calling needs, as the agent launcher already passes.
- **No deadline while waiting for the model.** A large model on a slow stick takes minutes. The wait ends when both servers answer, one exits, or the user stops.
- **Server output goes to log files** in the state directory, not to the terminal, and their paths are printed. A server that exits is reported with its log path.
- **Vision is off** in the model defaults until the runtime loads a projector, so the page does not offer image upload that would fail.
- **Built-in tools trimmed to `time` and `files`.** Unlisted categories default to on and each adds tool definitions to every request.
- **`svalbard-drive mcp --drive <root> --http`** exposes the same Streamable HTTP mode on its own, beside `--sse`.

## Verified, and how

Unit tests (`make verify`, 55 tested packages, and `make build`):

- Builder: package installed wheels-only with `--no-deps`, then the shipped file with dependencies; stamp content; no `uv` call on rebuild; path escape rejected.
- Runtime: environment values and JSON shapes; secret key created once with mode 0600; the action started against stand-in servers hands Open WebUI the llama-server and MCP addresses, does not pass a host `OPENAI_API_BASE_URLS` through, opens the browser only after health, and on cancellation stops both servers and the MCP listener; a server that fails to start ends the action with its log path and stops the other.
- MCP: initialize, list and call a tool over Streamable HTTP; shutdown on cancellation.
- Menu: the entry is absent without the recipe, present once for a two-platform install, labelled "Open WebUI", with the `open-webui` alias and action.

Real run on this Mac (64 GB, internal SSD), CLI and runtime built from the branch, scratch vault with `llama-server`, `open-webui`, `qwen-4b` and `qwen-0.8b`:

- `svalbard apply` built the tree with the real Docker builder, run with `DOCKER_DEFAULT_PLATFORM=linux/amd64` because the published tools image has no arm64 manifest (see "Found on the way"): 177 packages, 582 MB, no PyTorch, ChromaDB or onnxruntime, and the wheel's `LICENSE`, `LICENSE_HISTORY` and `LICENSE_NOTICE` under `open_webui-0.13.5.dist-info/licenses/`.
- `./run __native-open-webui Qwen_Qwen3.5-4B-Q4_K_M.gguf` in tmux: context picker shown and used (32K), ready 19 s after the choice on the first start, URL printed, browser opened.
- `GET /health` returned `{"status":true}`; sign-in with empty credentials returned `admin@localhost`; `/api/models` listed `Qwen_Qwen3.5-4B-Q4_K_M` with the default metadata; the tool server listed the drive's 8 tools; a chat turn through `/api/chat/completions` answered.
- In a clean browser profile the page opened on the chat screen with no sign-in and the model selected. One message ("Call the vault stats tool…") produced an MCP call to `svalbard_vault_stats`, its JSON result, and an answer that quoted it, with no tool selected by hand. That turn was two requests to `llama-server` and no title or tag request; the first prompt was 1,504 tokens.
- All 227 requests the page made went to its own origin. The Open WebUI log held no outbound attempt, no warning and no error; the only external URL in it is the project link in the start banner. The process held one listening socket on 127.0.0.1.
- Nothing appeared under the real home: no `~/.open-webui`, `~/.cache/open-webui`, `~/.cache/llama.cpp`, `~/.webui_secret_key`, and no new file of ours under `~/.config` or `~/Library/Application Support`. Nothing in the installed tree changed during the run. State was 58 MB, of which 57 MB bytecode.
- Ctrl+C: exit status 0 in under a second, no process left, all three ports closed.
- Second start through the alias `./run open-webui`: ready in 4 s, no sign-in, both earlier chats listed, and the model had changed to the vault's default without a stale setting (persistent config off).
- `uv pip install --dry-run` of the same two steps resolved from wheels for all four platforms: 176 packages plus Open WebUI. Intel Mac falls back to `argon2-cffi-bindings` 25.1.0.

## Not verified

- Linux x86_64, Linux arm64 and macOS x86_64: resolved, never installed or started. Linux cannot be run on this machine.
- A physical USB stick: cold start time, exFAT, pulling the stick mid-run.
- A packet capture. Offline behaviour rests on the log, the process's sockets and the page's request list.
- Larger models. Multi-step tool chains with two small models are in the comparison below.
- File attachments, Notes, and upgrading over an existing data directory.

## Found on the way

- The published tools image `ghcr.io/pkronstrom/svalbard-tools:0.5.0` has a linux/amd64 manifest only. On an Apple Silicon host every Docker-built recipe fails with "no matching manifest for linux/arm64/v8". The real run used `DOCKER_DEFAULT_PLATFORM=linux/amd64` (the build took 57 s under emulation). Not fixed here.
- `svalbard add <id>` does not add a recipe's dependencies; the wizard does.
- MCP tools that find nothing (`vault_sources`, `vault_databases`, `vault_maps` on an empty vault) return an empty string, and a small model calls them again.

## Tool use: Granite 4.2 3B and Qwen3.5 4B

A comparison of how two small models call the drive's MCP tools and combine what comes back, through Open WebUI. No code change comes from it. **Nine runs per model is a small sample**: it shows what kind of failure each model has, not a rate.

Setup:

- Scratch vault with `wikem` (WikEM, 49 MB) and `nhs-medicines` (NHS medicines A to Z, 15 MB), `index_strategy: keyword`, a 65 MB search index. The action was started once per model with a 32K context.
- `granite-4.2-3b-Q4_K_M.gguf` (2.24 GB) was placed in the vault's `models/` by hand; there is no recipe. The pinned `llama-server` (b12359) loads it, and its chat template reports tool-call support and produced native tool calls in every run.
- Sampling: Qwen used the flags `llamaserve` sets for the family (temperature 0.6, top-k 20, top-p 0.95). Granite has no entry there, so its card's `temperature=1.0`, `top_p=0.95` went in each request and were confirmed on the server's slot; top-k 40 and min-p 0.05 were `llama-server` defaults.
- No browser tab was opened for these runs. The runtime has no switch to skip opening the browser, so a no-op `open` was put first on PATH for the action; the earlier acceptance runs each opened a tab in the host's default browser.
- Each run replays the request Open WebUI's own page sends to `POST /api/chat/completions` for a new chat, with the tool server attached, and reads the saved chat when the reply is done. The plain OpenAI-style call without a chat returns tool calls to the caller instead of running them. Open WebUI's defaults were used: no system prompt, thinking on.
- Questions, each asked three times in a new chat:
  1. One article answers: one dose, the gap between doses and the 24-hour maximum for 500 mg paracetamol tablets. On the drive: 1 or 2 tablets, at least 4 hours, 8 tablets (NHS "Paracetamol for adults").
  2. Two sources combined: (a) the single prophylactic doxycycline dose after a tick bite in the emergency medicine wiki, (b) what the NHS content says about alcohol with doxycycline. On the drive: 200 mg once (WikEM "Lyme disease"); best avoided, alcohol can affect it (NHS "Antibiotics, interactions").
  3. Not on the drive: tyre pressure for a 2019 Toyota Corolla. The right answer is to say so.

Tool calls in order: V `vault_sources`, S `search`, R `search_read`, D `vault_databases`, T `vault_stats`, M `vault_maps`. Every call from both models had parsable arguments that fit the schema; "rejected" counts calls the tool answered with an error. Prompt tokens are summed over the run's requests, most of them served from the prompt cache.

| Model | Q | Run | Calls | Rejected | Read after search | Answer | Requests | Prompt tok | Output tok | Wall s |
|---|---|---|---|---|---|---|---|---|---|---|
| Qwen3.5 4B | 1 | 1 | VSSR | 0 | the right article | correct | 5 | 14,068 | 938 | 14 |
| Qwen3.5 4B | 1 | 2 | VSSSRSSRRRRRSSR | 4 | neighbouring articles | partly: gap given as "4 to 6 hours" | 18 | 127,996 | 2,593 | 84 |
| Qwen3.5 4B | 1 | 3 | SRR | 0 | neighbouring articles | correct values | 3 | 7,853 | 972 | 22 |
| Qwen3.5 4B | 2 | 1 | VSSRSR | 0 | one of the two | wrong: (a) gave the treatment course | 5 | 13,876 | 1,348 | 32 |
| Qwen3.5 4B | 2 | 2 | VSSRRSR | 0 | both | correct | 8 | 29,004 | 1,767 | 40 |
| Qwen3.5 4B | 2 | 3 | VSSSRRR | 0 | both | correct | 5 | 15,456 | 1,344 | 30 |
| Qwen3.5 4B | 3 | 1 | VSSSRDSRSRS | 1 | main pages only | correct: not on the drive | 12 | 34,871 | 1,705 | 34 |
| Qwen3.5 4B | 3 | 2 | VSSSSRTVDM | 0 | main page only | correct: not on the drive | 11 | 28,077 | 1,282 | 23 |
| Qwen3.5 4B | 3 | 3 | VSSSSSRSRSS | 0 | main pages only | correct: not on the drive | 12 | 55,186 | 1,682 | 41 |
| Granite 4.2 3B | 1 | 1 | VSR | 0 | a neighbouring article | partly: gap given as "4 to 6 hours" | 4 | 12,493 | 5,146 | 66 |
| Granite 4.2 3B | 1 | 2 | VSR | 0 | the WikEM article | wrong: "1.3 tablets" | 4 | 11,118 | 6,144 | 79 |
| Granite 4.2 3B | 1 | 3 | VSR | 0 | the right article | correct | 4 | 11,428 | 2,199 | 28 |
| Granite 4.2 3B | 2 | 1 | VSRS | 0 | main page only | wrong: (a) "not on the drive", (b) from a snippet | 5 | 19,141 | 6,457 | 84 |
| Granite 4.2 3B | 2 | 2 | VSRS | 0 | main page only | wrong: same | 5 | 16,493 | 4,611 | 58 |
| Granite 4.2 3B | 2 | 3 | VSRS | 0 | main page only | wrong: same | 5 | 17,405 | 4,537 | 56 |
| Granite 4.2 3B | 3 | 1 | VSSD | 0 | nothing to read | correct: not on the drive | 5 | 14,629 | 3,129 | 34 |
| Granite 4.2 3B | 3 | 2 | VSD | 0 | nothing to read | correct: not on the drive | 4 | 8,920 | 1,985 | 24 |
| Granite 4.2 3B | 3 | 3 | VDSS | 0 | nothing to read | correct: not on the drive | 5 | 12,852 | 2,609 | 29 |

What the runs show:

- **Correct answers: Qwen3.5 4B 7 of 9, Granite 4.2 3B 4 of 9.** Both said "not on the drive" all three times. The difference is in the questions that have an answer.
- **Qwen searches again; Granite stops after one search.** Qwen averaged 8 tool calls a run and Granite under 4. On question 2 Granite sent one long query ("doxycycline prophylactic dose Lyme disease adult tick bite"), got no result, read the archive's main page and concluded the fact was missing, then answered part (b) from a search snippet without reading the article. Qwen used two- and three-word queries and read the articles.
- **Qwen's persistence costs requests.** It took 11 or 12 requests to conclude that tyre pressures are not on a medical drive, and one single-lookup run took 18 requests and 128K prompt tokens after four reads were rejected.
- **Granite reads the first plausible hit.** Twice on question 1 it read a neighbouring article (co-codamol, WikEM's acetaminophen page) and answered from that.
- **Granite is slower per run despite fewer requests**: about 4,100 output tokens a run against 1,500, most of it thinking, and 51 s against 35 s on average.
- Neither model produced a malformed call. Small-model tool calling through Open WebUI's native mode held up; choosing what to read is where they fail.

What the tools contributed to the failures (existing behaviour, not changed here):

- `search` returns nothing unless every word matches, with no hint to shorten the query.
- `search_read` rejects a path without its trailing slash ("article not found"); all four of Qwen's rejected reads in one run were this.
- `search_read` returned about 3,000 characters of an article with no links, the index's partial body, although `kiwix-serve` had started. Why the full page was not used was not investigated.
- `search_read` without a path returns the main page, which Granite took for the article it wanted.
- Every search result ends with "Hybrid unavailable, fell back to keyword", which Granite quoted as a finding.

## Follow-ups

| Item | Size | Risk that could change the size |
|---|---|---|
| Publish an arm64 tools image, or pass `--platform` when only amd64 exists | S | M if the image build itself is not multi-arch ready |
| Start and chat on Linux and Intel Mac, and from a real stick | S | M if Intel Mac needs its own pins, L if USB cold start needs a copy on faster storage |
| Router mode: pick any installed chat model from Open WebUI | M | L if router children ignore a flag the single-model path relies on |
| Projector files and `--mmproj`, then vision back on | S | M if a projector must be matched to each model by recipe |
| Image generation and editing through `sd-server` | S on top of the `sd-server` recipe | M if the legacy image variables are removed upstream |
| Spoken replies | S on top of a TTS model recipe | M if a third model in memory forces on-demand start |
| Dictation | L | S if the speech server accepts WebM |
| Document extraction endpoint so PDFs can be attached | M | L if text quality forces Tika and a Java runtime |
| Share the platform-independent `open_webui` tree across platforms | M | L if the wrapper and manifest assume one tree per platform |
| Prune source maps (87 MB per platform) | S | M if the builder needs a general prune field with its own stamp rules |
| `svalbard add` resolves dependencies like the wizard | S | M if removal must also drop automatic dependencies |
| Empty MCP results say "none" instead of nothing | S | — |
| A way to start the browser actions without opening the browser (a `--no-open` style switch), for tests and headless hosts | S | M if it should cover every action that opens a browser and be reachable from the menu |
| MCP search and read for small models: hint on zero hits, accept a path without its trailing slash, full article bodies, drop the hybrid notice | M | L if full bodies need paging to fit small contexts |
| Repeat the tool-use comparison with more runs and with the larger shipped models | S | M if a default system prompt is added and has to be tuned per family |
| Upgrade routine for each Open WebUI pin bump | S per bump | M when a release changes config keys |
