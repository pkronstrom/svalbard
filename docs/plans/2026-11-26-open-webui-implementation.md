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

What the tools contributed to the failures (existing behaviour at the time; the first three are fixed in the next section):

- `search` returns nothing unless every word matches, with no hint to shorten the query.
- `search_read` rejects a path without its trailing slash ("article not found"); all four of Qwen's rejected reads in one run were this.
- `search_read` returned about 3,000 characters of an article with no links, the index's partial body, although `kiwix-serve` had started. Why the full page was not used was not investigated.
- `search_read` without a path returns the main page, which Granite took for the article it wanted.
- Every search result ends with "Hybrid unavailable, fell back to keyword", which Granite quoted as a finding.

## Search and read tool fixes

Done the same day on branch `search-tools`, after the comparison above. The tool names and required parameters are unchanged; `search_read` gained an optional `offset`. The index, its schema and the hybrid search path are untouched.

What changed, in the order a model meets it:

- **Search no longer comes back empty when one word is wrong.** Articles with every word still come first. When they do not fill the limit, articles with most of the words (more than half) follow, most words first, each carrying a `matched` list. The note after the results says when this happened and names any word that appears nowhere on the drive ("No article on the drive mentions: Toyota, Corolla, tyre"). A two-word query gets no partial matches: one word of two is a different search.
- **Ranking favours the article about the subject.** A word in the title counts four times one in the body, and the whole word outranks a longer word that starts with it ("lyme" before "lymecycline"). Stop words and surrounding punctuation are dropped from the query; a plural also finds the singular.
- **The drive's own search screen uses the same search**, and shows the same notes in its status line. Hybrid search keeps its previous candidates and ranking.
- **Read returns the whole article, 6,000 characters at a time.** A longer article ends with a `continues` line naming the offset for the next part. At most 20 links come with the first part, resolved to paths that can be read.
- **Read accepts the paths models actually send**: with or without leading or trailing slash, URL-encoded, with spaces for underscores, as the page's web address, in the wrong case, with `.zim` on the source or a unique part of its name, and without leading directories when only one article ends that way. When nothing resolves, the error names up to five existing paths.
- **Descriptions match.** The two tool descriptions and the drive context written for pi describe the partial matches and the `continues` line, in fewer characters than before.
- **Smaller answers.** Search results lost the repeated `read_hint`, the default limit is 8, `detail: full` returns the first 1,500 characters of each article, and JSON is no longer HTML-escaped. A keyword-only drive no longer adds "Hybrid unavailable, fell back to keyword" to every result.

**Why reads were cut at about 3,000 characters.** The index stores paths with a leading slash (`/Lyme_disease`), and the read built `/content/<archive>//Lyme_disease`. kiwix-serve answers 404 to the double slash, so every read that used a search result's path fell back to the index, which keeps the first 4,000 characters and no links. Only a read without a path reached kiwix-serve. A second defect sat beside it: page links are relative (`../warfarin/`) and were returned as written, so a link from any page below the archive root could not be read.

Response sizes on the test vault, in characters:

| Call | Before | After |
|---|---|---|
| `search` "paracetamol adult dose", defaults | 3,432 (10 results) | 2,201 (8 results) |
| `search` with the eight-word query Granite used | 42 (no results) | 2,238 (8 partial matches) |
| `search` "paracetamol adult dose", `detail: full` | 21,090 | 8,374 |
| `search_read` NHS "Paracetamol for adults" (7,276 characters) | 4,134, cut, no links | 6,795: the first 6,000 and a `continues` line |
| `search_read` WikEM "Lyme disease" (5,980 characters) | 3,153, cut, no links | 7,307: all of it and 20 links |
| `search_read` NHS main page | 23,576 | 7,853 |
| All eight tool definitions | 4,772 | 4,728 |

The three questions again, three runs each, same request path, same vault, 32K context. Baseline columns repeat the table above. Means per run; correct answers out of 3.

| Model | Question | Calls before | after | Rejected before | after | Prompt tok before | after | Correct before | after |
|---|---|---|---|---|---|---|---|---|---|
| Qwen3.5 4B | 1 single lookup | 7.3 | 2.3 | 4 | 0 | 49,972 | 9,040 | 2 | 3 |
| Qwen3.5 4B | 2 two sources | 6.7 | 5.7 | 0 | 0 | 19,445 | 29,960 | 2 | 3 |
| Qwen3.5 4B | 3 not on the drive | 10.7 | 15.3 | 1 | 0 | 39,378 | 79,491 | 3 | 3 |
| Granite 4.2 3B | 1 single lookup | 3.0 | 3.3 | 0 | 0 | 11,680 | 15,098 | 1 | 3 |
| Granite 4.2 3B | 2 two sources | 4.0 | 3.0 | 0 | 0 | 17,680 | 19,046 | 0 | 0 |
| Granite 4.2 3B | 3 not on the drive | 3.7 | 3.3 | 0 | 0 | 12,134 | 12,752 | 3 | 3 |

The runs after the change, one by one. Peak context is the largest prompt of the run, which is what has to fit the model's context.

| Model | Q | Run | Calls | Rejected | Read | Answer | Requests | Prompt tok | Output tok | Peak context | Wall s |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Qwen3.5 4B | 1 | 1 | VSR | 0 | the right article | correct | 4 | 10,822 | 722 | 5,096 | 13 |
| Qwen3.5 4B | 1 | 2 | SR | 0 | the right article | correct | 3 | 8,607 | 647 | 4,896 | 9 |
| Qwen3.5 4B | 1 | 3 | SR | 0 | the right article | correct | 3 | 7,691 | 684 | 4,460 | 9 |
| Qwen3.5 4B | 2 | 1 | SSRRR | 0 | both, and one more | correct | 4 | 22,494 | 1,655 | 10,643 | 25 |
| Qwen3.5 4B | 2 | 2 | VSSRRR | 0 | both, and one more | correct | 5 | 22,177 | 1,305 | 9,504 | 21 |
| Qwen3.5 4B | 2 | 3 | SRSRSR | 0 | both, and one more | correct | 7 | 45,208 | 1,186 | 11,046 | 23 |
| Qwen3.5 4B | 3 | 1 | VSSSSRDSSST | 0 | main page only | correct: not on the drive | 12 | 35,379 | 1,685 | 4,879 | 23 |
| Qwen3.5 4B | 3 | 2 | VSSSSDSSSSSS | 0 | nothing | correct: not on the drive | 13 | 39,107 | 1,488 | 6,678 | 23 |
| Qwen3.5 4B | 3 | 3 | V and 22 searches | 0 | nothing | correct: not on the drive | 24 | 163,988 | 2,364 | 13,831 | 44 |
| Granite 4.2 3B | 1 | 1 | VSR | 0 | the right article | correct | 4 | 13,860 | 2,975 | 7,182 | 38 |
| Granite 4.2 3B | 1 | 2 | VSR | 0 | the right article | correct | 4 | 14,036 | 3,502 | 7,693 | 44 |
| Granite 4.2 3B | 1 | 3 | VDSR | 0 | the right article | correct | 5 | 17,399 | 3,402 | 7,607 | 44 |
| Granite 4.2 3B | 2 | 1 | SRS | 0 | the Doxycycline article | wrong: (a) the treatment dose, (b) from a snippet | 4 | 25,394 | 7,614 | 13,215 | 108 |
| Granite 4.2 3B | 2 | 2 | VSS | 0 | openings, by `detail: full` | wrong: same | 4 | 17,047 | 5,859 | 10,065 | 82 |
| Granite 4.2 3B | 2 | 3 | VSS | 0 | nothing | wrong: (a) "100 mg", (b) from a snippet | 4 | 14,697 | 4,601 | 6,878 | 62 |
| Granite 4.2 3B | 3 | 1 | VSSD | 0 | nothing to read | correct: not on the drive | 5 | 16,202 | 3,601 | 5,279 | 46 |
| Granite 4.2 3B | 3 | 2 | VSSD | 0 | nothing to read | correct: not on the drive | 5 | 15,763 | 3,572 | 5,246 | 45 |
| Granite 4.2 3B | 3 | 3 | VS | 0 | nothing to read | correct: not on the drive | 3 | 6,292 | 1,669 | 3,267 | 21 |

Read of it, with the same caution about nine runs per model:

- **Correct answers went from 7 to 9 of 9 for Qwen3.5 4B and from 4 to 6 of 9 for Granite 4.2 3B.** Both models now open the right article for the single lookup every time; the title weighting puts it first.
- **Rejected calls went from 5 to 0.** No run needed a retry for a path.
- **The single lookup costs Qwen a fifth of the prompt tokens it did** (9,040 against 49,972), mostly because the 18-request run with four rejected reads no longer happens.
- **The two-source question costs more prompt tokens and is now answered right.** Whole articles are read where 3,000-character extracts were, and the answers quote criteria that sat below the old cut.
- **Granite still fails the two-source question, differently.** Its long query now returns results, so it no longer declares the fact missing. It reads the first hit, the Doxycycline article, and reports the treatment dose from it; and it still answers part (b) from a snippet.
- **Qwen's over-searching on the unanswerable question is not fixed, and may be worse**: 12, 13 and 24 requests against 12, 11 and 12. The tool now says "No article on the drive mentions: Toyota, Corolla, tyre", the model's reasoning shows it reads that, and it goes on to try "tire", "wheel", "car" and "vehicle". Two things were tried for three runs each and changed nothing measurable: a sentence in the note saying the drive does not cover a subject it never mentions (17, 8, 15 requests), and giving `vault_sources` its descriptions by hand (24, 10, 6). Neither is in the code.
- **Every run stayed below a 16K context.** The largest single prompt was 13,831 tokens, in the 24-search run; the answered questions peaked near 11,000. That run would not have fitted much more searching.

Limits and choices:

- Hybrid search was not exercised: the test vault has a keyword index only. Its code path keeps the old all-words candidates and ranking by design.
- The any-word fallback scores every article that has any of the words. On this 5,852-article index that took 9 ms for 3,707 matching articles. On a Wikipedia-sized index it is unmeasured and could take seconds for common words.
- The near-miss path lookup reads every path of the source, because the index has no index on path. It runs only after kiwix-serve has said 404.
- The 4,000-character cap on indexed bodies stays. Reads no longer depend on it. Search still sees only the opening of long articles: 365 of the 5,852 articles here (6%) are cut. The index is 65 MB for 15 MB of article text, so lifting the cap costs about four bytes of index per extra character; whether that is worth it needs measuring on a large source.
- Stop words are English only.
- One tested behaviour changed on purpose: an index that was never built for hybrid search no longer reports a hybrid fallback.

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
| `vault_sources` shows no descriptions for dated ZIMs: it looks recipes up by file name, `recipes.json` is keyed by recipe id. Qwen took WikEM for Wikipedia | S | M if `recipes.json` must carry file names from the host side |
| Stop a small model searching on after the drive has said it has nothing: a default system prompt for Open WebUI, as pi already gets | S | M if it has to be tuned per model family |
| Measure the any-word search fallback on a Wikipedia-sized index, and bound it if slow | S | M if common words have to be recognised from the index vocabulary |
| kiwix-serve start-up accepts any server that answers on its port, so two starts at once can read from the wrong process | S | M if every caller of `StartKiwix` needs a retry on another port |
| Decide on the 4,000-character cap of indexed bodies and on an index on article path | M | L if a large source makes the index too big for the stick |
| Repeat the tool-use comparison with more runs and with the larger shipped models | S | M if a default system prompt is added and has to be tuned per family |
| Upgrade routine for each Open WebUI pin bump | S per bump | M when a release changes config keys |
