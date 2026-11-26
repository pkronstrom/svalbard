# Open WebUI opt-in: implementation record

Status: phase A done 2026-11-26 on macOS arm64. Not run on Linux, Intel Mac or a physical stick.

Scope: work items 1 to 5 of the [research](research/2026-11-26-open-webui.md). llama.cpp's own page stays the default Chat entry and is untouched. Open WebUI is an opt-in recipe, in no pack or preset.

The workspace file tools added afterwards are recorded in [File tools for the workspace](#file-tools-for-the-workspace) at the end.

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

## File tools for the workspace

Status: done 2026-11-26 on macOS arm64, on top of phase A. Not run on Linux, Intel Mac or a physical stick.

Scope: let a chat in Open WebUI save what it found. The model searches the vault, writes or edits a file such as an HTML page in one folder on the drive, and gives a link that opens it. The terminal harnesses are untouched: they have file tools of their own, and the stdio MCP server still lists the eight vault tools.

### Resulting user path

1. `./run open-webui` prints a second address, `Workspace:  http://localhost:8086/ (files the assistant saves, in <drive>/workspace)`. Nothing new opens in the browser.
2. A new chat has two tool servers switched on, **Svalbard vault** and **Svalbard files**. Each has its own switch in the chat's Tools menu.
3. "Search the vault for X and Y, read both and save a page as notes.html" produces `<drive>/workspace/notes.html` and a reply with a clickable link to it. "Change the heading in notes.html" edits the same file.
4. The folder is an ordinary folder at the root of the stick. Files put there by hand can be read and edited by the model, and the address above lists them.
5. **File tools** under Open WebUI on the Settings screen, or `./run settings open-webui.file_tools false`, turns all of this off for the next start: no second tool server, no workspace address, and the folder is not created.

```text
┌─────────────────────────┐ tools ┌──────────────────────────────────────┐
│ Open WebUI        :8084 │──────▶│ svalbard-drive __native-open-webui   │
│ (browser page, chat)    │       │  :8090/mcp        vault, 8 tools     │
└────────────┬────────────┘       │  :8090/files/mcp  file_list, _read,  │
             │ link in a reply    │                   _write, _edit      │
             ▼                    │  :8086            read-only pages    │
┌─────────────────────────┐ GET   │                   own origin, CSP    │
│ generated page    :8086 │◀──────│                                      │
└─────────────────────────┘       └──────────────────┬───────────────────┘
                                                     │ only this folder
                                             <drive>/workspace/
```

### Decisions

- **One folder, `<drive>/workspace/`**, created when the action starts. If it cannot be created (read-only stick, a file of that name) the action says "File tools are off" and continues with chat and vault search.
- **Four tools and no more**: `file_list`, `file_read`, `file_write`, `file_edit`. No delete, move or execute. Open WebUI prefixes them with the server id, so the model sees `workspace_file_write`.
- **Two checks on every path.** `Resolve` rejects absolute paths, `..`, backslashes, names starting with a dot, and anything whose deepest existing part resolves outside the folder through a symlink, with an error that says what to send instead. All file access then goes through an `os.Root` opened on the folder, which refuses an escape on its own if a path ever reaches it unchecked. A link that stays inside the folder is followed.
- **Dot names are refused, unlisted and unserved.** They are the atomic writer's temporary files and the host's litter (`.DS_Store`, `._name`).
- **Limits**: 2 MiB per written file; `file_read` returns at most 20,000 bytes per call (about 6,000 tokens) and loads no file over 8 MiB; `file_list` goes two levels down by default, five at most, 200 entries at most. Binary or non-UTF-8 files are refused for read and edit.
- **Writes are atomic**: a temporary file in the same folder, synced and renamed.
- **`file_edit` replaces one exact occurrence.** Absent text and text that occurs more than once are errors that leave the file alone; `replace_all` replaces every occurrence. When the text is absent but one line matches apart from indentation, the error quotes that line.
- **A tool server of its own, on the listener the action already held.** `mcp.ServeStreamableHTTP` now mounts several servers by path; the file tools are at `/files/mcp` and get their own entry in `TOOL_SERVER_CONNECTIONS` and in the default `toolIds`. `svalbard-drive mcp` in all three modes is unchanged.
- **Pages are served from a different origin than Open WebUI**, on a third listener bound to 127.0.0.1 (preferred port 8086), read-only. Each response carries `Content-Security-Policy: default-src 'none'; script-src 'unsafe-inline' 'self'; style-src 'unsafe-inline' 'self'; img-src data: blob: 'self'; font-src data: 'self'; media-src data: blob: 'self'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Cross-Origin-Resource-Policy: same-site` and `Cache-Control: no-store`. A request whose `Host` is not `localhost` or `127.0.0.1` on that port gets 403, and everything outside the folder is the same 404. Folders get a plain index unless they hold an `index.html`.
- **`'self'` is allowed** for scripts, styles, images, fonts and media so a page can use other workspace files. It still cannot fetch, post a form or load from another origin. A page can navigate away; no header prevents that.
- **`same-site`, not `same-origin`**, so Open WebUI (localhost on another port) can show an image the model saved, while a page on any other site cannot embed workspace files.
- **The on/off switch is the recipe setting `open-webui.file_tools`** (bool, default on), declared beside `open_browser` and read in one function, `fileToolsEnabled` in `drive-runtime/internal/openwebui`, the way `opensBrowser` reads its own. Off means the workspace is not opened, its listener is not started and Open WebUI is given the vault's tool server alone. A drive whose schema predates the setting keeps file tools on. The store itself is recorded in the [settings implementation](2026-11-26-stick-settings-implementation.md).
- **`workspace.Open` takes the folder**, not the drive root, so another tool set can confine itself to a folder of its own; the drive's workspace is `filepath.Join(driveRoot, workspace.DirName)`. The package's errors name no tool: they say what is wrong and carry a kind (`fs.ErrNotExist`, `syscall.ENOTDIR`, `workspace.ErrIsFolder`), and the files capability adds which of its tools to call next. Its path errors still say "workspace" and give `notes/page.html` as the example.
- **No system prompt and no model metadata were added.** Both models used the tools from their descriptions alone. Three things were changed after watching real runs, all in the tools:
  - `file_read` now says "give only the path" and marks the line range as "only for a very long file". With the first wording Qwen3.5 4B read a 314-line page in 22 calls of ten to fifty lines.
  - The link is spelled `http://localhost:<port>/...`, not `127.0.0.1`. The model copied `127.0.0.1:8086` into its reply as `127.0.0.806` in one chat.
  - `file_write` and `file_edit` return a ready Markdown link (`"link":"[page.html](http://localhost:8086/page.html)"`) and say to put it in the reply as returned. Given a bare URL the model put it in backticks in one chat, which Open WebUI does not make clickable.
- **Tool annotations follow the action**: `file_write` and `file_edit` are no longer announced as read-only (`ActionDef.Writes`).

### Verified, and how

Unit tests (`make verify`, 58 tested packages after the merge with the settings store, and `make build`):

- Workspace: accepted and cleaned paths; every escape (absolute, home, `..` in any position, backslash, NUL, dot names, a symlink to an outside folder or file, a dangling symlink, a symlinked parent with missing folders below it); the same escapes sent straight to the file operations without the check; atomic replacement with no leftover; the size cap at and over the limit; a folder or a file in the way; folders, oversized files, a named pipe and a missing file refused for reading without naming the host path.
- Static server: the header values and each CSP directive, no `connect-src` and no remote source; folder index, redirect to the slash, `index.html` preferred, hidden files absent; 404 for `..`, a doubled slash, an absolute host path, both symlink kinds, a hidden file and the drive's own manifest; 405 for POST, PUT and DELETE; 403 for a foreign `Host`; the port released on cancellation.
- Tools: list (empty workspace, depth, the 200-entry cap, hidden files and outward links left out), read (whole, by line range, continued part by part until the file is rebuilt byte for byte, one over-long line, binary and non-UTF-8 refused), write (parents created, replacement, the size cap, the link with spaces, brackets and parentheses), edit (one occurrence, absent, ambiguous, `replace_all`, the indentation hint, deletion, an edit that would exceed the cap), and all four tools against every escape with the outside directory and the drive's manifest checked afterwards.
- Servers: the drive server has exactly the eight vault tools and creates no workspace; the file tools carry write hints; two servers on one listener each list their own tools, a write over HTTP lands in the folder, and a refused path comes back as a tool error; a write request with a form content type or a foreign `Host`, which is what a page on another site could send, is refused, while the same request from a local client writes.
- Action: both tool servers and both default tool ids with file tools on, one of each with them off; a start against stand-in servers serves both MCP paths, writes through the file tools into `<drive>/workspace`, serves that file from a localhost origin other than Open WebUI's with the CSP, prints the workspace address, opens only Open WebUI, and closes every port on cancellation; a drive whose `workspace` is a file starts without file tools and says so; the setting read with no schema, an older schema, the default, on, off, an invalid value and the other switch off; with it off, a start gives Open WebUI one tool server and one default tool id, answers 404 at `/files/mcp`, creates no folder and prints no workspace line, and no listener is opened until it is switched on.
- Catalog: the real `open-webui` recipe declares `open_browser` and `file_tools`, both bool and on by default.

Real run on this Mac (64 GB, internal SSD), runtime built from the branch and applied to a copy of the phase A scratch vault (`wikem`, NHS medicines, keyword index), 32K context, driven through the page itself in an isolated browser profile with `open` replaced by a no-op:

- A new chat showed "Tools 2", both switches on. With **Svalbard files** switched off the model still searched and read the vault, made no request to `/files/mcp`, wrote no file and said it had no tool to save one.
- **Qwen3.5 4B, final build, three chats** (burns and paracetamol, ibuprofen against paracetamol, hypothermia and frostbite). Each write turn was two searches, two article reads and one `file_write`: 5 tool calls in 4 model requests, 35 to 57 s, 1,700 to 4,100 tokens generated, and a reply with a clickable link. The pages were 3.9 to 11 KB of styled HTML naming both source articles. Each edit turn was one whole-file read and one or two `file_edit` calls: 3 or 4 model requests, 18 to 28 s. In one of the three the note that was asked for "at the very top" was added as an HTML comment, so the tool call succeeded and the page did not visibly change there.
- **Qwen3.5 4B, earlier builds, five chats**: all five write turns produced the page (24 to 78 s). The link was right and clickable in three, mistyped in one and in backticks in one, which led to the two link changes above. All five edits changed the file as asked, except that one replaced the footer line where it was asked to add one, in 4, 25, 4, 3 and 10 model requests; the 25 led to the `file_read` wording, and the 10 (two "not found" errors from wrong indentation, each recovered) led to the indentation hint.
- **Granite 4.2 3B, final build, three chats**: three of three pages written and linked, 6 or 7 tool calls in 7 or 8 model requests, 77 to 145 s, pages of 0.9 to 1.6 KB. Three of three edits applied correctly: one with `file_edit` in 238 s (9,000 tokens generated, nearly all reasoning), two by rewriting the whole file with `file_write` in 49 and 65 s. Its content was weaker: one search with `limit=1` read a less relevant article, and doses were paraphrased loosely.
- Context peaked at 17,100 tokens for Qwen (the 25-request edit; 13,000 on the final build) and 19,100 for Granite, with no truncation. The first request of a chat is about 2,250 tokens with both tool servers, against 1,500 with the vault alone.
- The link from the first chat, fetched with curl, returned the file byte for byte with the headers above. In the browser the page rendered with its styles; from inside it a fetch to its own origin, to Open WebUI and to the MCP server all failed, an image from Open WebUI's origin was blocked and a `data:` image loaded. From Open WebUI's page a workspace image loaded through `localhost` and was blocked through `127.0.0.1`, and a fetch of a workspace page could not be read.
- Against the running server: `/../manifest.yaml`, `/..%2fmanifest.yaml`, `/%2e%2e/manifest.yaml`, `//etc/passwd`, `/.svalbard/recipes.json`, the Open WebUI secret and a symlink to `/etc/hosts` all returned 404, POST returned 405, and a foreign `Host` 403.
- The real binary's `mcp --drive` over stdio listed the eight vault tools and created no workspace folder.
- With a second Open WebUI instance from another vault running on this machine, the action moved to free ports (Open WebUI 8085, MCP 8091, workspace 8086) and a write through `/files/mcp` returned a link that served the file.
- The setting, on the scratch vault after `svalbard apply` wrote it into `settings.schema.json`: `./run settings` listed `open-webui.file_tools on`. After `./run settings open-webui.file_tools false` (with the workspace folder moved aside) a start printed no workspace line, the runtime held the MCP port only, Open WebUI listed **Svalbard vault** alone with one default tool id, and no folder was created. The Settings screen showed `File tools * off`; Enter on that row turned it on and removed the key from `settings.json`, and the next start printed the workspace address, held the second port, created the folder and listed both tool servers.
- Ctrl+C, ten stops: exit status 0 on the nine where it was recorded; after the first and the last no process and no listener was left, and every restart got the same ports back. The no-op `open` recorded one call per start, for Open WebUI's address only.

### Not verified

- Linux and Intel Mac: the code is the same and cross-compiles, never run. Linux cannot be run on this machine.
- A physical stick: exFAT (which has no symlinks and no permission bits), a rename over an existing file there, and pulling the stick during a write.
- Contexts under 32K. The 8K choice probably cannot hold two articles and a page.
- Whether the pages say what the articles say. They were read, not checked line by line; the 4B model called WikEM "Wikipedia" and wrote "Original article" links that lead nowhere, because search results carry no address a page could link to.
- Larger models, and anything but HTML and short notes.
- Wall times are indicative: another llama-server ran on the same machine during part of the runs.
- One start test (`TestRunStartsEverythingAndStopsItOnCancel`) failed once in thirteen runs under the race detector and did not reproduce. The action picks free ports for its two child servers before they bind, and other Open WebUI instances were starting on this machine at the time; the listeners added here are held from the start.

### Follow-ups

| Item | Size | Risk that could change the size |
|---|---|---|
| Delete and move | S | M if deletion needs a trash folder or a confirmation in the page |
| A workspace on a host folder | S | M if one stick must remember a different folder per host |
| Open the result automatically | S | M if the signal has to come from Open WebUI and not from the tool call |
| The same tools in llama.cpp's built-in page | M | L if that page cannot be given a tool server at start |
| Read PDF and Office files | M | L if it needs an extractor shipped per platform |
| Versions or undo | M | L if it needs its own page to browse them |
| Search inside workspace files, to cut reads of long files | S | M if it should share the vault's index |
| Addresses for vault articles, so a page can link to its sources | M | L if kiwix must run on a fixed port for the life of the stick |
| Hold the child servers' ports instead of probing them | S | M if llama-server cannot take an inherited socket |
| Check the 8K and 16K context choices with file tools on | S | M if tool definitions must be trimmed for them |
