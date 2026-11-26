# Open WebUI on a Svalbard stick

Research date: 2026-11-26. Claims marked **UNVERIFIED** were not confirmed from
Open WebUI's repo, docs, PyPI metadata, another project's own repo, or a measurement
made for this report.

Work items 1 to 5 were built the same day; see the
[implementation record](../2026-11-26-open-webui-implementation.md). Text marked
**Verified by the run** was corrected from that run on macOS arm64. The rest of
the report is unchanged research.

## 1. Recommendation

**Ship Open WebUI as an opt-in recipe in its "slim" shape. Do not make it a pack
default yet. Keep llama.cpp's own chat page as the default Chat entry on every stick.**

Open WebUI can do what the owner wants. Its slim dependency set installs from wheels
on all four platforms through the existing Python mechanism, it runs with no sign-in,
all of its state can live under `<stick>/.svalbard/runtime/open-webui/`, and chat,
image input, image generation and editing, spoken replies and the vault's MCP tools
can all be preconfigured by environment variables so the user sets up nothing.

It should stay opt-in for now for six reasons:

1. **Size.** Slim measures 584 MB installed per platform (macOS arm64). A two-platform
   stick pays about 1.2 GB. llama.cpp's page costs nothing extra.
2. **Slim removes things a "just ask" user expects.** PDF and Office attachments return
   an error, dictation needs an audio conversion step that the stick does not have, and
   there is no document search inside Open WebUI (slim only supports a PostgreSQL
   vector store). Section 4 proposes a fix for the first of these. The full install fixes these but cannot be installed on Intel Macs at
   all and is a 3.5 GB download per Linux platform.
3. **Intel Mac is fragile.** The slim set resolves there only because `uv` falls back to
   an older `argon2-cffi-bindings`. Open WebUI's own hash-pinned slim lock file fails on
   Intel Mac today.
4. **Upkeep.** Six releases in the 23 days from 0.13.0 to 0.13.5. Image settings moved
   from environment variables to "connections" during this period; the variables this
   proposal relies on for images are still in the source but no longer in the docs.
5. **Tool calling with stick-sized models.** Open WebUI's docs say models under about
   30B parameters "often produce malformed JSON or fail multi-step tool chains", and
   every request carries all enabled built-in tool definitions unless they are trimmed.
6. **llama.cpp v1.1.0 (released today) closed most of the gap.** Its page now has voice
   messages and dictation, agent traces, and `llama-server` can spawn a stdio MCP
   server itself (`--mcp-servers-config`), so Svalbard's MCP server works with it as
   it is.

What only Open WebUI gives the stick: image generation and editing inside the chat,
chat history stored on the stick instead of in the host's browser, and per-model
defaults for tools and features.

Two follow-up questions from the owner are answered in their own sections. Section 4:
of everything else Open WebUI can talk to locally, only a document extraction
endpoint, the bundled code interpreter and Notes are worth serving, and none adds a
binary. Section 5: model switching from Open WebUI's picker works with `llama-server`
in router mode, provided the runtime writes a preset file and limits it to one loaded
model.

**Evidence that would change this**

- Promote to a pack default if a real-stick test shows all of: Intel Mac slim starts
  and chats; cold start is acceptable from USB; Gemma 4 12B and Qwen3.8 27B complete
  a vault search and an image request through tools with the built-in tools trimmed;
  dictation works or is deliberately left out.
- Drop it if Intel Mac slim fails at run time, or cold start from USB is over about
  a minute, or llama.cpp's page gains image generation.
- A cheaper route to "ask for a picture in chat" exists and is worth one test before
  committing: add an image tool to Svalbard's MCP server that calls `sd-server`.
  llama.cpp v1.1.0 passes image data from MCP tool results to its page (release notes,
  PR #32064). Whether the page then shows the picture inline is **UNVERIFIED**.

## 2. Facts

| Fact | Value | Source |
|---|---|---|
| Latest version | 0.13.5, uploaded 2026-11-25 | PyPI |
| Release cadence | 25 releases from 0.9.2 (2026-04-24) to 0.13.5; 0.13.0 to 0.13.5 in 23 days; a new minor version every 3 to 11 weeks (0.10.0 on 06-29, 0.11.0 on 07-27, 0.12.0 on 10-10, 0.13.0 on 11-03) | PyPI |
| Licence | "Open WebUI License": BSD-3-Clause text plus a branding clause. Not OSI-approved (their own FAQ). Code up to v0.6.5 stays BSD-3 | wheel `LICENSE`, docs |
| Package | `open-webui` on PyPI, entry point `open-webui serve --host --port` | PyPI, wheel |
| Python | `>=3.11,<3.13`. Svalbard's `svalbard-python` is 3.12, which fits | PyPI, `recipes/tools/svalbard-python.yaml` |
| Wheel | 110.9 MB download, 266 MB unpacked, of which 255 MB is the browser front end (87 MB source maps, about 60 MB Pyodide) | measured |
| Default install, macOS arm64 | 243 packages, 595 MB download, **1.9 GB installed** (torch 569 MB, open_webui 270 MB, cv2 124 MB, pyarrow 109 MB) | measured |
| Default install, Linux x86_64 | 262 packages, **3,544 MB download** (PyTorch with CUDA libraries). Installed size not measured | `uv` dry run + PyPI sizes |
| Default install, Linux arm64 | 262 packages, 3,678 MB download. Installed size not measured | `uv` dry run + PyPI sizes |
| Default install, macOS x86_64 | **Does not install.** `onnxruntime==1.26.0`, `brotlicffi==1.2.1.0` and `torch 2.15.0` have no Intel Mac wheels | `uv` dry run, PyPI |
| Slim install | 176 packages plus Open WebUI itself on all four platforms from wheels only. Download 92 to 109 MB plus the 110.9 MB wheel. **582 MB installed on macOS arm64** by the real builder; other platforms not installed, expected similar (**UNVERIFIED**) | `uv` dry run, measured |
| Slim, pruned | About 497 MB per platform without source maps; about 434 MB also without Pyodide, which would remove the offline code interpreter (arithmetic; not run) | computed |
| Slim lock file | Upstream `requirements-slim.lock` installs on three platforms and **fails on macOS x86_64** (`argon2-cffi-bindings==26.1.0` has no Intel wheel) | `uv` dry run |
| Offline | `OFFLINE_MODE=true` turns off the version check and Hugging Face downloads. Slim never downloads a model. Two features still reach out: the token text splitter (tiktoken) and the code interpreter (`cdn.jsdelivr.net`) | docs, source |
| Sign-in | `WEBUI_AUTH=False` creates `admin@localhost` and signs in automatically. Only works on a fresh database with no users | docs, `routers/auths.py` |
| State location | `DATA_DIR` holds the database, uploads and cache. Three more writes need redirecting: `STATIC_DIR`, the secret key file, Python bytecode (see section 3) | docs, `env.py`, `__init__.py` |
| Idle RAM | About 300 MiB for slim, upstream's own test of 0.12.3 in Docker on Linux arm64. **Verified by the run:** about 520 MB resident on macOS arm64 after a few chats | docs, measured |
| Startup time | Not published. **Verified by the run:** from an internal SSD, 16 to 19 s to a healthy server on the first start (database migrations, bytecode written), 4 s on the second. From a USB stick: not measured | measured |
| Single binary / desktop | No single binary. `open-webui/desktop` v0.1.4 ships desktop installers (dmg, setup.exe, zip; AGPL-3.0, about 130 MB); `open-webui/tether` is a companion app. Whether either runs without installing or writing to the host home is **UNVERIFIED**, and an installer does not fit the stick | GitHub API |

### The branding clause

Quoted from the `LICENSE` in the 0.13.5 wheel:

> 4. Notwithstanding any other provision of this License, and as a material
> condition of the rights granted herein, licensees are strictly prohibited
> from altering, removing, obscuring, or replacing any "Open WebUI"
> branding, including but not limited to the name, logo, or any visual,
> textual, or symbolic identifiers that distinguish the software and its
> interfaces, in any deployment or distribution, except in the following
> circumstances: (i) deployments or distributions where the total number
> of end users (defined as individual natural persons with direct access
> to the application) does not exceed fifty (50) within any rolling
> thirty (30) day period; (ii) the licensee has obtained specific prior
> written permission from the copyright holder; or (iii) where the
> licensee has obtained a duly executed enterprise license expressly
> permitting such modification.

What it means for a stick given to other people (a reading, not legal advice):

- Redistribution is allowed. Their docs say "You can still use, modify, and
  redistribute it for any purpose, just don't remove or alter our branding".
- Ship it unmodified, keep the name and logo, and call the menu entry "Open WebUI".
  Do not rename it to something like "Svalbard Chat". Setting `WEBUI_NAME` is allowed
  by the software and appends "(Open WebUI)", but leaving it unset is simplest.
- Clauses 1 to 3 are plain BSD-3: put the licence text on the stick (the wheel carries
  `LICENSE`, `LICENSE_HISTORY` and `LICENSE_NOTICE`) and do not imply endorsement.
- The 50-user exemption does not matter because nothing is rebranded.
- Deleting source maps does not touch branding and is what upstream's own slim image
  does. Whether deleting other bundled files is wise is a testing question, not a
  licence one.

### What the default install pulls in, and what slim leaves out

| Dependency | Default | Slim | Note |
|---|---|---|---|
| PyTorch (via sentence-transformers, accelerate, transformers) | yes | no | 569 MB on macOS arm64; CUDA build on Linux from PyPI |
| ChromaDB | yes | no | needs onnxruntime; slim's only vector store is PostgreSQL pgvector |
| onnxruntime, rapidocr, opencv | yes | no | local OCR and embeddings |
| faster-whisper, av, pydub, soundfile | yes | no | local speech-to-text and audio conversion; pydub needs an `ffmpeg` binary on the host |
| pandas, pyarrow, pypdf, python-docx, python-pptx, openpyxl | yes | no | PDF and Office extraction |
| Playwright | only in the `[all]` extra | no | not installed by default |
| boto3, azure-*, psycopg, redis, ldap3, opentelemetry | yes | yes | imported at module level, so they cannot be dropped without patching |

Slim is switched on at run time with `USE_SLIM_DOCKER=true`. The name says Docker but
the check is a plain environment read in `open_webui/env.py`, and the package list is
shipped inside the wheel as `requirements-slim.txt`. Upstream supports slim only as a
Docker image; running it from a `pip`/`uv` install is the same code but is not a
documented path. **Verified by the run:** it starts cleanly from a
`uv pip install --target` tree on macOS arm64.

A middle shape (default packages minus PyTorch, keeping ChromaDB and the document
loaders) is not offered upstream. It would need a hand-kept package list and still
breaks on Intel Mac at `onnxruntime`. Not recommended.

### Riding on Svalbard's Python mechanism

`host-cli/internal/builder/pythonvenv.go` runs
`uv pip install --no-build --target <dir> --python-platform <p> <packages...>` per
platform and writes a wrapper script into `bin/<platform>/`.

- **As it is, with `packages: [open-webui==0.13.5]`:** macOS arm64 gets the 1.9 GB
  default install. Intel Mac is skipped with a warning (no wheels). Both Linux
  platforms get PyTorch with CUDA, a 3.5 GB download each. Not usable.
- **What has to change (small):** the builder needs a way to install the package with
  `--no-deps` and then install the dependency list that ships inside it. One new
  recipe field covers it (see the recipe below; built as `requirements_from`).
  Optional second field: a list of paths to delete after install (not built).
- **Already right:** Python 3.12, the wrapper sets `PYTHONPATH` and
  `PYTHONNOUSERSITE=1`, and `bin/open-webui` is produced by `--target` installs
  (confirmed in the test install).
- **Not handled:** the 269 MB `open_webui` tree is identical on every platform but is
  installed once per platform.

## 3. Wiring proposal

One menu entry, "Open WebUI", and one `./run` alias start everything, wait for
`GET /health`, and open the browser. Ports are the first free port from the base shown,
as the existing actions do.

```text
┌─────────┐ HTTP ┌──────────────────────────────┐ files ┌──────────────────┐
│ browser │─────▶│ open-webui serve       :8084 │──────▶│ <stick>/         │
└─────────┘      │ slim, no sign-in, env config │       │ .svalbard/       │
                 └───────────────┬──────────────┘       │ runtime/         │
                                 │ HTTP, 127.0.0.1 only │ open-webui/      │
                                 │                      └──────────────────┘
       ┌──────────────┬──────────┴──┬────────────┬──────────────┐
       ▼              ▼             ▼            ▼              ▼
┌─────────────┐ ┌───────────┐ ┌───────────┐ ┌──────────┐ ┌─────────────┐
│ llama-server│ │ sd-server │ │ llama-    │ │ speech   │ │ svalbard-   │
│ :8082       │ │ :8086     │ │ server    │ │ to text  │ │ drive       │
│ chat,       │ │ generate  │ │ :8087     │ │ :8088    │ │ mcp-http    │
│ image input │ │ and edit  │ │ text to   │ │ needs a  │ │ :8090 /mcp  │
│ (--mmproj)  │ │ images    │ │ speech    │ │ WAV step │ │ vault tools │
└─────────────┘ └───────────┘ └───────────┘ └──────────┘ └─────────────┘
```

Only the first box in the bottom row is required. The others start when their recipe
is installed. The MCP box can be a goroutine inside the same `svalbard-drive` process
that launches the rest, so it is not an extra binary.

### How each feature is wired

**Chat.** OpenAI connection to `http://127.0.0.1:8082/v1` with provider `llama.cpp`.
The model list comes from `/v1/models`. Streaming works over the standard API.
Open WebUI does not learn the context length from an OpenAI-compatible server;
`llama-server` enforces its own `--ctx-size`, and Open WebUI's optional context
compaction is off by default. Reasoning from Qwen3.8 arrives in `reasoning_content`,
which Open WebUI shows as a collapsible "Thinking" block; with provider `llama.cpp` it
sends the reasoning back in the same field inside a tool loop
(`utils/middleware.py`). Turning thinking off needs a custom parameter
`chat_template_kwargs` per the docs. All preconfigurable by environment.

**Image input.** No Open WebUI setting beyond the model's `vision` capability, which
defaults to on. The work is on the Svalbard side: load the projector with `--mmproj`
(not wired yet). Preconfigurable.

**Image generation and editing.** Use the "OpenAI compatible" connection type against
`sd-server`'s `/v1/images/generations` and `/v1/images/edits`. Do not use the
AUTOMATIC1111 type: Open WebUI's docs say it "handles generation only". In 0.13 image
back ends are "connections" in the admin panel; the older variables
(`IMAGE_GENERATION_ENGINE`, `IMAGES_OPENAI_API_BASE_URL`, …) are still read and turned
into a public connection at start-up by `migrate_legacy_image_config()`. With
persistence off this runs on every start, so a changed port is picked up. The model
makes pictures by calling the built-in `generate_image` / `edit_image` tools, so this
depends on tool calling working. Preconfigurable, on undocumented variables.
`sd-server` also has its own web page on the same port, which works without any of
this.

**Text-to-speech.** `AUDIO_TTS_ENGINE=openai` against a second `llama-server` loaded
with a TTS model (`/v1/audio/speech`, added in llama.cpp v1.0.0). Slim passes the
returned WAV through untouched. Two traps from llama.cpp's README: OpenAI voice names
such as `alloy` are rejected, so `AUDIO_TTS_VOICE` must be a voice the model lists, and
only `wav` or `pcm` are produced. Preconfigurable.

**Speech-to-text. Blocked as specified.** The Open WebUI front end records with
`MediaRecorder`, preferring `audio/webm; codecs=opus` (seen in the bundled
JavaScript). Slim cannot convert audio (`BYPASS_PYDUB_PREPROCESSING` is forced on), and
the full install converts with `pydub`, which needs `ffmpeg` on the host.
`llama-server` accepts "formats supported by `miniaudio` (mp3, wav, flac)". Something
must decode WebM/Opus to WAV in between. Whether sherpa-onnx's server accepts WebM is
**UNVERIFIED**. `AUDIO_STT_ENGINE=web` (browser speech recognition) needs the network
in Chrome (**UNVERIFIED**) and is not an offline answer.

**RAG embeddings.** `RAG_EMBEDDING_ENGINE=openai` with `RAG_OPENAI_API_BASE_URL`
pointing at `llama-server`'s `/v1/embeddings` works for the embedding call. But slim
has no vector store without PostgreSQL ("Slim requires PostgreSQL/pgvector for vector
storage"), so document search inside Open WebUI is not available in the slim shape.
Proposal: set `BYPASS_EMBEDDING_AND_RETRIEVAL=true` so attached text files go into the
context whole, and let vault search go through Svalbard's MCP tools. In slim only
plain-text formats (txt, md, csv, html, xml) can be attached; PDFs and Office files
return 503.

**Agent tools.** Open WebUI has native MCP support since v0.6.31, **Streamable HTTP
only**. Its docs point stdio and SSE servers at the `mcpo` proxy (another Python package) or the Tether desktop app.
Svalbard does not need `mcpo`: the Go MCP server already uses `mark3labs/mcp-go`
v1.6.0, which has `NewStreamableHTTPServer`; today the runtime offers stdio (`mcp`) and
SSE (`mcp-serve`), and SSE does not work with Open WebUI. Add a Streamable HTTP mode on
`/mcp`. The connection is preconfigured with `TOOL_SERVER_CONNECTIONS` (`"type":"mcp"`)
and attached to every model through `DEFAULT_MODEL_METADATA.toolIds`.

Svalbard's MCP tools today (from `drive-runtime/internal/mcp/`): `search`,
`search_read`, `vault_sources`, `vault_databases`, `vault_maps`, `vault_stats`,
`query_describe`, `query_sql`. All are read-only.

Native function calling is Open WebUI's default and only supported mode. It uses the
standard `tools` field, which `llama-server` supports with `--jinja` (enabled by default in v1.1.0). Their docs list
Qwen 3.6 27B and Gemma 4 12B among local models that "work well", and warn that models
under about 30B are unreliable. Results on the stick's models are **UNVERIFIED**.

Built-in features that need the network or cost context: web search is off by default
(`ENABLE_WEB_SEARCH`); the code interpreter is on by default and may fetch Python
packages from a CDN, so keep it off until the offline test passes (section 4); memory, notes, chat history, calendar,
automations and other built-in tool categories are on by default and add tool
definitions to every request, so turn them off per category.

### Background model calls

Each of these is a separate request that queues behind the user's turn on a
one-slot server.

| Call | Default | Variable to turn off |
|---|---|---|
| Chat title | on | `ENABLE_TITLE_GENERATION=false` |
| Tags | on | `ENABLE_TAGS_GENERATION=false` |
| Follow-up suggestions | on | `ENABLE_FOLLOW_UP_GENERATION=false` |
| Autocomplete | off | `ENABLE_AUTOCOMPLETE_GENERATION=false` |
| Retrieval query rewriting | on | `ENABLE_RETRIEVAL_QUERY_GENERATION=false` |
| Web search query rewriting | on, legacy mode only | `ENABLE_SEARCH_QUERY_GENERATION=false` |
| Image prompt rewriting | on, legacy mode only | `ENABLE_IMAGE_PROMPT_GENERATION=false` |
| Memory background review | off | `ENABLE_MEMORY_BACKGROUND_REVIEW=false` |
| Context compaction | off | `ENABLE_CONTEXT_COMPACTION=false` |

With titles off, a chat is named after its first message. Titles are the one call
worth reconsidering later, with `TASK_MODEL_PARAMS` capping the output.

### Environment the runtime would set

`R` is `<stick>/.svalbard/runtime/open-webui`. Ports are examples.

**Verified by the run:** the process, chat, tools and features-off blocks below are
what the runtime sets (`drive-runtime/internal/openwebui`), and the JSON shapes
worked as written. Corrections to the first draft of this list:

- Open WebUI gets a clean environment, not the host's plus these values. It reads
  several hundred variables, and `OPENAI_API_BASE_URLS` (plural) from the host
  would take priority over `OPENAI_API_BASE_URL`. With no base URL at all it
  defaults to `https://api.openai.com/v1`.
- `CORS_ALLOW_ORIGIN` is set to the page's own origin. The default `*`, together
  with no sign-in, lets any page open in the host's browser read the chats.
- `TMPDIR=R/tmp` was added so uploads in flight stay on the stick.
- `SCARF_NO_ANALYTICS`, `DO_NOT_TRACK` and `ANONYMIZED_TELEMETRY` were dropped:
  nothing in the slim tree reads them.
- `--alias <model file name>` is passed to `llama-server`. Without it the model id
  in the picker, and in `DEFAULT_MODELS`, is the model's full path.
- Vision is off in `DEFAULT_MODEL_METADATA` until a projector is loaded, and the
  image capability and `defaultFeatureIds` wait for `sd-server`.
- Built-in tool categories: only `time` and `files` stay on. `user_input` is off
  too. With the eight MCP tools attached, the first turn's prompt was 1,504 tokens.
- Tools named in `toolIds` are preselected for a new chat in the browser with no
  click, and a tool server id of `svalbard` prefixes every tool
  (`svalbard_search`).

The image, speech, dictation and document-extraction blocks further down were not
run and their shapes stay **UNVERIFIED**.

Process and paths (read on every start):

```text
USE_SLIM_DOCKER=true
DATA_DIR=R/data
STATIC_DIR=R/static              # default is inside the package tree and is
                                 # rewritten on every start
WEBUI_SECRET_KEY=<random, generated once, kept in R/secret>
                                 # else .webui_secret_key lands in the cwd
HOME=R/home  XDG_CONFIG_HOME=R/config
XDG_CACHE_HOME=R/cache  XDG_DATA_HOME=R/share  TMPDIR=R/tmp
PYTHONPYCACHEPREFIX=R/pycache    # keeps bytecode out of the installed tree
PYTHONNOUSERSITE=1               # the generated wrapper already sets this
OFFLINE_MODE=true                # also sets HF_HUB_OFFLINE=1, no version check
WEBUI_AUTH=False
ENABLE_PERSISTENT_CONFIG=False   # environment wins on every start
CORS_ALLOW_ORIGIN=http://localhost:8084;http://127.0.0.1:8084
inherited from the host: PATH, LANG, LC_ALL, LC_CTYPE, TZ; nothing else
cwd = R;  command: open-webui serve --host 127.0.0.1 --port 8084
```

`HOST` and `PORT` are ignored by `open-webui serve`; it takes flags. Its default host
is `0.0.0.0`, so `--host 127.0.0.1` is required.

`ENABLE_PERSISTENT_CONFIG=False` is the key choice. Settings then come from the
environment on every start, so ports and the model name can change between runs, and
changes made in the admin panel last only for the session. Chats, uploads and each
user's personal settings are ordinary database rows and do persist.

Chat:

```text
ENABLE_OLLAMA_API=False
ENABLE_OPENAI_API=True
OPENAI_API_BASE_URL=http://127.0.0.1:8082/v1
OPENAI_API_KEY=local
OPENAI_API_CONFIGS={"0":{"enable":true,"provider":"llama.cpp",
                         "connection_type":"local","auth_type":"none"}}
DEFAULT_MODELS=<the --alias given to llama-server>
AIOHTTP_CLIENT_TIMEOUT_MODEL_LIST=30
```

Do not set sampling in `DEFAULT_MODEL_PARAMS`. Open WebUI's OpenAI mapping has no
`top_k`, and with several model families one global value is wrong for some. The
server's flags or preset file carry sampling (section 5).

Tools and model defaults:

```text
TOOL_SERVER_CONNECTIONS=[{"type":"mcp","url":"http://127.0.0.1:8090/mcp",
  "path":"","auth_type":"none","key":"","config":{"enable":true},
  "info":{"id":"svalbard","name":"Svalbard vault",
          "description":"Search and read the content on this drive"}}]
DEFAULT_MODEL_METADATA={
  "capabilities":{"file_upload":true,"vision":false,"image_generation":false,
                  "web_search":false,"code_interpreter":false,"memory":false},
  "toolIds":["server:mcp:svalbard"],
  "builtinTools":{"time":true,"files":true,"image_generation":false,
    "user_input":false,"memory":false,"chats":false,"users":false,"notes":false,
    "knowledge":false,"channels":false,"tasks":false,"automations":false,
    "calendar":false,"skills":false,"subagents":false,"notifications":false,
    "tools":false,"web_search":false,"code_interpreter":false,
    "video_generation":false,"visualizations":false}}
```

Once `sd-server` is wired, `image_generation` goes back to `true` in both maps and
`"defaultFeatureIds":["image_generation"]` is added (**UNVERIFIED**).

Features off:

```text
ENABLE_WEB_SEARCH=False          ENABLE_CODE_INTERPRETER=False
ENABLE_CODE_EXECUTION=False      ENABLE_MEMORIES=False
ENABLE_CALENDAR=False            ENABLE_AUTOMATIONS=False
ENABLE_CHANNELS=False
ENABLE_COMMUNITY_SHARING=False   ENABLE_MESSAGE_RATING=False
ENABLE_USER_STATUS=False         BYPASS_EMBEDDING_AND_RETRIEVAL=True
ENABLE_TITLE_GENERATION=False    ENABLE_TAGS_GENERATION=False
ENABLE_FOLLOW_UP_GENERATION=False
ENABLE_AUTOCOMPLETE_GENERATION=False
ENABLE_RETRIEVAL_QUERY_GENERATION=False
ENABLE_SEARCH_QUERY_GENERATION=False
ENABLE_IMAGE_PROMPT_GENERATION=False
```

Notes stay on (the default). The code interpreter stays off until the offline test in
section 7 passes. Document extraction, once the endpoint exists:

```text
CONTENT_EXTRACTION_ENGINE=external
EXTERNAL_DOCUMENT_LOADER_URL=http://127.0.0.1:8090
EXTERNAL_DOCUMENT_LOADER_API_KEY=local
```

Images, only when `sd-server` is installed (legacy variables, see above):

```text
ENABLE_IMAGE_GENERATION=true     IMAGE_GENERATION_ENGINE=openai
IMAGES_OPENAI_API_BASE_URL=http://127.0.0.1:8086/v1
IMAGES_OPENAI_API_KEY=local      IMAGE_GENERATION_MODEL=<id from sd-server>
IMAGE_SIZE=1024x1024
ENABLE_IMAGE_EDIT=true           IMAGE_EDIT_ENGINE=openai
IMAGES_EDIT_OPENAI_API_BASE_URL=http://127.0.0.1:8086/v1
IMAGES_EDIT_OPENAI_API_KEY=local IMAGE_EDIT_MODEL=<same id>
```

Spoken replies, only when a TTS model is installed:

```text
AUDIO_TTS_ENGINE=openai
AUDIO_TTS_OPENAI_API_BASE_URL=http://127.0.0.1:8087/v1
AUDIO_TTS_OPENAI_API_KEY=local
AUDIO_TTS_MODEL=<tts model id>   AUDIO_TTS_VOICE=<a voice from GET /props>
```

Dictation, only once the WAV step exists:

```text
AUDIO_STT_ENGINE=openai
AUDIO_STT_OPENAI_API_BASE_URL=http://127.0.0.1:8088/v1
AUDIO_STT_OPENAI_API_KEY=local   AUDIO_STT_MODEL=<stt model id>
```

Without it, leave `AUDIO_STT_ENGINE` unset; in slim the microphone then returns an
error rather than downloading anything.

### Recipes

`recipes/apps/open-webui.yaml`, as built (one new builder field):

```yaml
id: open-webui
type: python-package
display_group: tools
tags: [computing, agentic]
depth: reference-only
size_gb: 0.58                     # per platform, measured on macos-arm64
venv: svalbard-python
deps: [svalbard-python, llama-server]
packages:
- open-webui==0.13.5
requirements_from: requirements-slim.txt  # NEW: install packages with --no-deps,
                                          # then this file from the installed tree
entry_points:
- open-webui
description: Open WebUI browser front end for the drive's local models and vault tools (slim install)
license:
  id: LicenseRef-Open-WebUI
  attribution: Open WebUI Inc. (Timothy Jaeryang Baek); Open WebUI License, BSD-3-Clause terms plus a branding clause
  url: https://docs.openwebui.com/license
```

`requirements_from` keeps the dependency pins in step with the pinned version without
a second URL or hash. It installs with dependency resolution, not from upstream's hash
lock, because the hash lock does not install on Intel Mac. The first draft called the
field `deps_from`; `deps` already means recipe dependencies in this schema. The draft's
`prune` field and `menu` block were not built: the menu entry is a fixed built-in so
its label cannot drift from "Open WebUI".

Other recipes this proposal leans on, none of which exist yet: `sd-server` (binary,
four platforms) with a FLUX.2 klein 4B model recipe; a TTS model recipe for
llama.cpp v1.1.0; projector files for the vision models. They are planned separately
and are needed for the same features on llama.cpp's own page.

Runtime (built as `drive-runtime/internal/openwebui/`): a package following
`internal/chat/chat.go` and `PrepareClientLaunchConfig` in `internal/agent/agent.go`:
resolve the model, start `llama-server`, serve MCP over Streamable HTTP in-process,
start the optional servers, start `open-webui` with the environment above, poll
`/health`, open the browser, and stop every child on exit.

## 4. Other local back ends Open WebUI can use

**Serve three things, none of which adds a binary to the stick: a document extraction
endpoint inside `svalbard-drive`, the Pyodide code interpreter that is already in the
wheel, and Notes. Everything else is "later" or "skip".**

Open WebUI's admin settings expose many more back ends than chat, images, speech and
embeddings. Most are cloud services. The table lists every kind that can run locally.
"On the stick" means a matching piece exists today (Kiwix archives behind
`kiwix-serve`, the keyword and semantic index in SQLite, PMTiles maps, and the MCP
server over all of them).

| Back end | What the user gets offline | On the stick? | Verdict |
|---|---|---|---|
| Document extraction, `external` engine | PDFs and Office files can be attached in slim | no | **Serve it** |
| Code interpreter, Pyodide in the browser | Calculations, CSV and spreadsheet analysis, charts | in the wheel | **Serve it** if the offline test passes |
| Notes | A notebook kept on the stick; a note can be attached whole to a chat | no | **Serve it** (leave on) |
| Web search, `external` engine | A "Web Search" toggle that searches the vault, with source links | yes, index and `kiwix-serve` | Later |
| Skills | Plain-text instructions the model loads on demand | no | Later |
| Voice calls, Realtime mode | Hands-free spoken conversation | no | Later |
| Document extraction, Apache Tika | Same as `external`, more formats | no | Later, as fallback |
| Reranker | Better ordering in Open WebUI's own document search | own ranking | Skip |
| Vector stores and Knowledge bases | Semantic search over the user's own uploads | own index | Skip |
| Memory | Facts remembered across chats | no | Skip |
| Web search: SearXNG, YaCy, OpenSERP | Nothing without the internet | — | Skip |
| Code execution, Jupyter | Full Python on the host | no | Skip |
| Open Terminal, Machines, Tether | A shell for the model | pi, opencode, goose | Skip |
| Pipelines server, Functions | Custom Python filters and model wrappers | no | Skip |
| Docling, MinerU, PaddleOCR-VL, Marker | Layout-aware extraction and OCR | no | Skip |
| Video generation | Text to video | no | Skip |
| Channels, Apps, Automations, Calendar | Team rooms, scheduled prompts | no | Skip |
| Ollama connection | A second model server | `llama-server` | Skip |

Open WebUI has no setting for map tiles or ZIM archives. Maps stay in Svalbard's own
map page, and Kiwix content reaches the model through the MCP tools.

### The three to serve

**Document extraction endpoint.** This closes slim's biggest gap: attaching a PDF
returns 503. The `external` engine is a small HTTP contract: Open WebUI sends
`PUT <url>/process` with the raw file bytes and expects
`{"page_content": "...", "metadata": {...}}` or a list of them. The check for it comes
before the slim check in `retrieval/loaders/main.py`, so it works in slim. It can be a
handler inside `svalbard-drive` using a pure-Go PDF text library, so it adds no binary,
no size and no licence question beyond the library's. Settings:
`CONTENT_EXTRACTION_ENGINE=external`, `EXTERNAL_DOCUMENT_LOADER_URL`, and
`EXTERNAL_DOCUMENT_LOADER_API_KEY`, which must be non-empty or Open WebUI silently
skips the service. Text quality from a pure-Go library on real PDFs is **UNVERIFIED**,
and scanned PDFs would need OCR, which this does not give.

**Pyodide code interpreter.** The PyPI wheel carries the Pyodide runtime and its
packages (numpy, pandas, matplotlib, scipy, scikit-learn, sympy, pillow, openpyxl,
python-docx and others; 60 MB, 67 files). Code runs in the browser sandbox, not on the
host. Nothing has to be served. Two cautions: upstream says the slim *image* fetches
these packages from `cdn.jsdelivr.net`, and whether the front end uses the bundled copy
when slim mode is on is **UNVERIFIED**; and the model reaches it by calling a tool, so
it depends on tool calling. Keep it off (`ENABLE_CODE_INTERPRETER=False`) until the
offline test passes, and do not prune the `pyodide` folder.

**Notes.** A built-in feature with no back end. Notes are rows in the database on the
stick. Leave `ENABLE_NOTES` at its default (on) and keep the notes tool category off
so it adds no tool definitions.

### Why the rest is later or skip

- **Vault search as "web search" (later).** The `external` web search contract is
  `POST {"query", "count"}` returning `[{"link", "title", "snippet"}]`, which maps
  directly onto Svalbard's search index with links to `kiwix-serve` pages. A Go
  endpoint would be small. It needs `ENABLE_WEB_SEARCH=true`,
  `WEB_SEARCH_ENGINE=external`, `EXTERNAL_WEB_SEARCH_URL`,
  `BYPASS_WEB_SEARCH_EMBEDDING_AND_RETRIEVAL=true` (slim has no vector store) and
  `ENABLE_LOCAL_WEB_FETCH=true` so pages on 127.0.0.1 can be fetched. It duplicates
  the MCP `search` and `search_read` tools, and the toggle is labelled "Web Search",
  which is misleading on an offline stick. Do it only if MCP results turn out to show
  sources poorly.
- **Skills (later).** A "how to use this stick" skill could help small models pick the
  vault tools. Skills live in the database, so seeding one needs an API call after
  start. Try a default system prompt first: `DEFAULT_INTERFACE_SETTINGS` "also reach[es]
  personal settings written outside the Interface page, such as `system` and `params`"
  (env reference). The exact JSON is **UNVERIFIED**.
- **Realtime voice calls (later).** Open WebUI opens a WebSocket to
  `<base URL>/realtime?model=…` and the browser captures 24 kHz audio with an
  AudioWorklet instead of recording WebM, so this path may avoid the WebM problem.
  llama.cpp has `/v1/realtime` and v1.1.0 adds full-duplex voice models. Whether the
  two fit together is **UNVERIFIED**. Standard voice calls use the
  ordinary speech-to-text path and are blocked with dictation.
- **Apache Tika (later, as fallback).** `tika-server-standard` 3.4.0 is a 75 MB jar,
  Apache-2.0, and runs anywhere Java does. It needs a Java runtime: Temurin 21 JRE is
  42 to 52 MB compressed per platform. No Java runtime recipe was found under
  `recipes/tools/`. OCR needs Tesseract on top. Use it only if the Go extractor is not
  good enough.
- **Reranker (skip).** `RAG_EXTERNAL_RERANKER_URL` could point at `llama-server`
  started with `--reranking` and a reranker model. It only improves Open WebUI's own
  document search, which slim does not have. Whether llama-server's response matches
  the `results[].index` / `relevance_score` shape Open WebUI expects is **UNVERIFIED**.
- **Vector stores, Knowledge, Memory (skip).** In slim all three need PostgreSQL with
  pgvector, which is a database server, not a file on the stick. The stick's own index
  already covers vault content. A user's own files go into the chat whole.
- **SearXNG, YaCy, OpenSERP (skip).** SearXNG (AGPL-3.0, Python) and OpenSERP query
  internet search engines, so they return nothing offline. YaCy (Java) can crawl its
  own index but would duplicate Svalbard's.
- **Jupyter (skip).** `jupyter_server` (BSD-3-Clause) could ride on `svalbard-python`,
  but it runs model-written code on a borrowed computer with no sandbox.
- **Open Terminal, Machines, Tether (skip).** Open WebUI's "local machine" needs
  internet to install itself and is marked experimental. The stick already ships
  terminal agents.
- **Pipelines and Functions (skip).** Pipelines is a separate Python server (MIT).
  Functions run inside Open WebUI and are stored in the database. No need was found
  that the MCP server does not already meet.
- **Docling and the other OCR engines (skip).** `docling-serve` (MIT) depends on
  PyTorch; the rest are cloud or GPU services.
- **Video generation (skip).** Needs an OpenAI-style video API or ComfyUI.
  `sd-server` only has its own native video route.
- **Channels, Apps, Automations, Calendar (skip).** Built for teams, or they add
  background model calls on a one-slot server.

## 5. Switching models from the picker

**Confirmed from source and docs: the picker lists every entry that `/v1/models`
returns and sends the chosen id in each request, so with `llama-server` in router mode
picking another model makes it load. The full path was not run.**

- `get_all_models()` in `routers/openai.py` fetches `<url>/models` for each connection
  and keeps every item in `data`. The only filter applies to `api.openai.com`.
- With the connection's provider set to `llama.cpp`, it reads each item's
  `status.value`: `loaded` and `sleeping` show the green "Loaded" marker, `loading`
  and `unloaded` do not.
- The chat request carries the picked id in `model` (the connection prefix, if any, is
  stripped first).
- llama-server's README at the pinned build b12359: "The router uses the `"model"`
  field in the JSON body" and "By default, the model will be loaded automatically if
  it's not loaded." The router flags `--models-dir`, `--models-preset`,
  `--models-max`, `--models-memory-margin` and `--models-autoload` are all documented
  there.

### What goes wrong in practice, and the setting for each

1. **The list shows models that cannot chat.** `--models-dir <stick>/models` would list
   the embedding model, speech models and the MTP draft file too, and it expects a
   model with a projector to sit in its own subfolder with a file named `mmproj*`.
   Svalbard keeps models flat. Use `--models-preset` only, with one section per chat
   model, written by the runtime at launch.
2. **Models from the host can appear.** The router also lists llama.cpp's download
   cache, which `LLAMA_CACHE` controls and which defaults to the host's home. Set
   `LLAMA_CACHE` to an empty folder under the stick's runtime directory. Whether a
   preset alone hides the cache is **UNVERIFIED**.
3. **Two large models load at once.** `--models-max` defaults to 4. Set
   `--models-max 1`. `--models-memory-margin` is not a substitute: the README says
   "Host (CPU) memory is assumed to be unlimited" and "On a build without GPU devices
   the memory limit has no effect", and projector files are not counted. It helps on
   Macs with Metal and does nothing on CPU-only Linux.
4. **The first message after a switch waits for the load.** A 14 GB file read from USB
   at 100 to 200 MB/s takes roughly 1 to 2.5 minutes (arithmetic, not measured).
   Open WebUI's backend does not time out: `AIOHTTP_CLIENT_TIMEOUT` is unset by
   default, which means no limit, and llama-server's own `--timeout` is 3600 seconds.
   Leave `AIOHTTP_CLIENT_STREAM_IDLE_TIMEOUT` unset. What the user sees during the
   wait, and whether the router holds the request open or answers with an error while
   loading, are **UNVERIFIED**. Give the default model `load-on-startup = true` so the
   first chat does not pay this.
5. **Model list caching.** Not a problem at defaults. `MODELS_CACHE_TTL` is 1 second
   and `ENABLE_BASE_MODELS_CACHE` is off, so the list and the "Loaded" markers are
   refetched. Do not turn the base cache on: it has no expiry. The list fetch times
   out after 10 seconds (`AIOHTTP_CLIENT_TIMEOUT_MODEL_LIST`); raise it to 30 in case
   the router answers slowly while a child process is reading the stick.
6. **Sampling set in two places.** Open WebUI sends a parameter only when it is set
   (globally, per model or per chat), and a value in the request overrides the
   server's preset. Its OpenAI mapping covers `temperature`, `top_p`, `min_p`,
   `max_tokens`, penalties, `reasoning_effort`, `seed` and `stop`
   (`utils/payload.py`). `top_k` is not in it. The stick's families need different
   values (Gemma 4: temperature 1.0, top-k 64; Qwen3.5: 0.6 and 20), so one global
   `DEFAULT_MODEL_PARAMS` would be wrong for some of them. Leave sampling out of
   Open WebUI and put it in the preset file. Per-model settings in Open WebUI are
   database rows, cannot be set by environment, and would drift from the preset.
7. **Per-model server flags.** Today `llamaserve.ExtraFlags` derives flags from the
   file name: sampler values, the q8_0 cache for Qwen3.6 and 3.8, reasoning flags, the
   Qwen3.8 chat template, the MTP draft model. In router mode these go into each preset
   section. Command-line arguments given to the router override preset values, so
   start the router with shared flags only. The multi-line chat template has to be a
   file (`chat-template-file`).
8. **Anything that calls a second model causes reload thrash.** With one model
   allowed, leave `TASK_MODEL` and `TASK_MODEL_EXTERNAL` unset (background work then
   uses the chat's model, and the proposal turns that work off anyway). A chat with
   several models selected would reload on every turn. Keep embedding and speech
   models on their own `llama-server` processes, not in the router, so they never
   evict the chat model.
9. **The admin model panel.** Provider `llama.cpp` gives the no-sign-in user, who is
   an admin, a panel with Download, Load, Unload and Delete. Stick models are safe:
   the README says "only model stored in cache can be deleted. You cannot delete
   models in a preset." A download started while the host is online would land in
   `LLAMA_CACHE` on the stick. Acceptable; the "Loaded" marker and Eject button are
   worth it.
10. **Readable ids.** The preset section name is the id shown in the picker and must
    match `DEFAULT_MODELS`. Avoid ids that start with `gpt-5` or `o` plus a digit:
    Open WebUI then sends the system prompt with the `developer` role.

Router start and preset the runtime would write (keys are llama-server arguments
without the leading dashes; exact spelling of the flag-style keys is **UNVERIFIED**):

```text
LLAMA_CACHE=R/llama-cache
llama-server --models-preset R/models.ini --models-max 1 \
             --host 127.0.0.1 --port 8082
```

```ini
version = 1

[*]
parallel = 1
c = 32768                      ; llamaserve.ContextForHost()

[gemma-4-12b]
model = <stick>/models/gemma-4-12B-it-qat-UD-Q4_K_XL.gguf
mmproj = <stick>/models/<projector file>
temp = 1.0
top-k = 64
top-p = 0.95
min-p = 0.0
load-on-startup = true

[qwen3.8-27b]
model = <stick>/models/<file>
temp = 1.0
top-k = 20
top-p = 0.95
flash-attn = on
cache-type-k = q8_0
cache-type-v = q8_0
reasoning-effort = medium
chat-template-file = R/qwen3.8.jinja
```

On the Open WebUI side this needs only `AIOHTTP_CLIENT_TIMEOUT_MODEL_LIST=30` and no
`DEFAULT_MODEL_PARAMS` for sampling. `--sleep-idle-seconds` also works in router mode
if idle memory should be released; the next message then pays the load again.

## 6. Work items

| # | Item | Size | Risk that could change the size |
|---|---|---|---|
| 1 | Builder: `deps_from` (and optional `prune`) for `python-package` recipes | S | M if the stamp and rebuild logic must also track the dependency file's content |
| 2 | Recipe `open-webui` with per-platform size and licence entry | S | M if Intel Mac needs its own pins once a direct dependency drops Intel wheels |
| 3 | MCP server: Streamable HTTP mode on `/mcp` | S | M if Open WebUI's client needs session or protocol-revision handling mcp-go does not provide |
| 4 | Runtime: "Open WebUI" action that starts and stops all processes, sets the environment, waits for `/health`, opens the browser | M | L if slim does not start cleanly from a `uv --target` tree, or if cold start from USB needs a prepared copy on faster storage |
| 5 | Chat defaults: environment for connection, model defaults, trimmed built-in tools, background calls off | S | M if the JSON shapes need a run-time API call instead of environment variables |
| 6 | Image generation and editing through `sd-server` | S on top of the `sd-server` recipe | M if the legacy image variables are removed upstream and connections must be written through the config API; L if both models do not fit in memory together and need load/unload orchestration |
| 7 | Spoken replies through a TTS `llama-server` | S on top of the TTS model recipe | M if a third model in memory forces on-demand start |
| 8 | Dictation: WebM/Opus to WAV step in front of the STT server | L | Stays L unless the chosen STT server accepts WebM, then S. Pure-Go Opus decoding is the open design question |
| 9 | Share the platform-independent `open_webui` tree across platforms | M | L if the wrapper and manifest model assume one tree per platform |
| 10 | Upgrade routine: re-run the four-platform resolve and the stick tests for each pinned bump | S per bump | M when a release changes config keys, as 0.13 did for images |
| 11 | Alternative: `generate_image` tool on Svalbard's MCP server, for llama.cpp's page | S | M if the page does not show tool-result images inline |
| 12 | Router mode for the Open WebUI action: write the preset file from `llamaserve` flags, start with `--models-max 1`, set `LLAMA_CACHE` | M | L if router children do not honour a flag the single-model path relies on (MTP draft, Qwen3.8 template), or if the request fails instead of waiting while a model loads |
| 13 | Document extraction endpoint (`PUT /process`) in `svalbard-drive` | M | L if text quality forces Tika plus a Java runtime per platform |
| 14 | Confirm the bundled Pyodide runs offline in slim mode, then turn the code interpreter on | S | M if the front end insists on the CDN and the package path has to be overridden |
| 15 | Later: vault search as Open WebUI's `external` web search provider | S | M if page fetches from `kiwix-serve` need cleaning before they are useful |

Items 1 to 5 make a usable opt-in entry with chat and vault tools, and are built.
Image input also needs the projector wiring from "Other recipes" above. Items
12 and 13 are the next most valuable. The whole proposal is an XL only if items 8 and
9 are included.

## 7. Test on a real stick before shipping

1. Slim starts from the stick on all four platforms, Intel Mac first, with no network.
2. Cold start and second start, in seconds, from a USB 3 stick on exFAT; idle RAM.
3. Nothing is written outside `<stick>/.svalbard/runtime/open-webui/` (watch the host
   home and the installed package tree). Stop and restart; pull the stick mid-run and
   check the SQLite database still opens.
4. No outbound connections with a network present (packet capture): start, chat,
   open settings, generate a picture.
5. No sign-in screen on first start and on every later start.
6. Chat streams; Qwen3.8 thinking shows as a block; sampling values reaching
   `llama-server` match the model's recipe.
7. Attach a photo to a vision model. Attach a `.txt` file. Attach a PDF and see what the
   user is told.
8. Vault search by tool call with each shipped model size, with built-in tools
   trimmed; count the prompt tokens of an empty first turn.
9. Picture from a prompt, then an edit of it, through `sd-server`; memory use with the
   chat model loaded on a 16 GB host.
10. Spoken reply plays in Safari, Chrome and Firefox (WAV passthrough).
11. Microphone: what Open WebUI actually uploads from each browser, and what the STT
    server does with it.
12. With titles, tags and follow-ups off, one user turn produces exactly one request
    to `llama-server`.
13. Upgrade over an existing data directory from one pinned version to the next.
14. Router mode: the picker lists only chat models and none from the host; pick a
    second model and send a message; record the wait, what the page shows meanwhile,
    and that only one model is in memory afterwards.
15. Router mode: sampling values and context size reaching each model match its
    recipe; a Qwen3.8 chat with tools still works through a router child.
16. Admin model panel: Delete cannot remove a stick model; Download fails cleanly
    with no network.
17. Code interpreter with no network: run code that imports pandas and matplotlib.
18. Attach a text PDF and a scanned PDF through the extraction endpoint.

## 8. Alternatives

**llama.cpp's own page, v1.1.0 (released 2026-11-26).** Already on the stick, MIT, all
four platforms, no extra size. Release notes list a redesigned UI ("Llama Chat"),
voice messages and dictation, agent replies shown as an "Agent Trace", reasoning
effort in the model picker, and MCP management in settings. `llama-server` can spawn
stdio MCP servers itself with `--mcp-servers-config` / `--mcp-servers-json`
(Cursor-style JSON), which fits `svalbard-drive mcp --drive <root>` unchanged, and
defaults can be preset with `--ui-config-file`. Image, PDF and audio attachments work
with models that support them (Open WebUI's comparison page, dated 2026-11-14). It has
no image generation, and whether it reads replies aloud is **UNVERIFIED**. Chats are
kept in the host browser's storage, tied to the page's port, not on the stick
(**UNVERIFIED** for v1.1.0). This is the default to build on.

**`sd-server`'s own page.** stable-diffusion.cpp's server ships an embedded web UI for
generation and model management on the same port as its API. It needs no chat front
end at all and is the zero-cost way to offer pictures.

**KoboldCpp v1.125.1.** One file with its own chat UI, image generation, speech and
MCP support (MCP per a third-party report, **UNVERIFIED**). AGPL-3.0. Release assets
cover Linux x86_64 and macOS arm64 only, so two of four platforms are missing, and it
is its own model server, so it would replace `llama-server` rather than sit in front
of it.

**LocalAI v5.0.1.** MIT, single Go binary of about 200 MB with a web UI, images and
speech. No Intel Mac binary. It downloads back ends on demand, so an offline stick
would need them fetched at build time (**UNVERIFIED** how). Also its own model server.

**Jan v0.8.8, LibreChat, Open WebUI Desktop.** Jan and Open WebUI Desktop are desktop
apps with installers that keep state in the host profile. LibreChat needs Node and
MongoDB. None fits "runs from the stick without installing".

**Llampart, Off Grid AI Desktop, others seen in search.** Only third-party
descriptions were found. **UNVERIFIED**; not evaluated.

## 9. Sources

Open WebUI:
- PyPI metadata: https://pypi.org/pypi/open-webui/json
- Licence: https://github.com/open-webui/open-webui/blob/v0.13.5/LICENSE and https://docs.openwebui.com/license
- Licence history: https://github.com/open-webui/open-webui/blob/v0.13.5/LICENSE_HISTORY
- Slim package list and lock: https://github.com/open-webui/open-webui/blob/v0.13.5/backend/requirements-slim.txt , https://github.com/open-webui/open-webui/blob/v0.13.5/backend/requirements-slim.lock
- Dockerfile (slim branch, telemetry variables): https://github.com/open-webui/open-webui/blob/v0.13.5/Dockerfile
- Environment reference: https://docs.openwebui.com/reference/env-configuration
- Quick start, slim limits, Python versions: https://docs.openwebui.com/getting-started/quick-start
- Offline guide: https://docs.openwebui.com/tutorials/maintenance/offline-mode
- MCP: https://docs.openwebui.com/features/extensibility/mcp
- Tools and calling modes: https://docs.openwebui.com/features/extensibility/plugin/tools
- Task models: https://docs.openwebui.com/features/administration/task-models
- Image generation: https://docs.openwebui.com/features/chat-conversations/image-generation-and-editing and https://docs.openwebui.com/features/chat-conversations/image-generation-and-editing/usage
- Reasoning models: https://docs.openwebui.com/features/chat-conversations/chat-features/reasoning-models
- Self-hosted speech-to-text: https://docs.openwebui.com/features/chat-conversations/audio/speech-to-text/self-hosted-stt
- OpenAI-compatible connections, provider setting, model management: https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai-compatible
- External document extraction: https://docs.openwebui.com/features/chat-conversations/rag/document-extraction/external
- External web search: https://docs.openwebui.com/features/chat-conversations/web-search/providers/external
- Python code execution: https://docs.openwebui.com/features/chat-conversations/chat-features/code-execution/python
- Voice mode and Realtime calls: https://docs.openwebui.com/features/chat-conversations/chat-features/voice-mode
- Notes, Memory, Skills, local machines: https://docs.openwebui.com/features/notes , https://docs.openwebui.com/features/chat-conversations/memory , https://docs.openwebui.com/features/workspace/skills , https://docs.openwebui.com/features/open-terminal/setup/local-machine
- llama.cpp connection and comparison: https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-llama-cpp , https://docs.openwebui.com/alternatives/llama-cpp
- Desktop and Tether: https://github.com/open-webui/desktop , https://github.com/open-webui/tether , https://docs.openwebui.com/ecosystem/tether
- mcpo: https://github.com/open-webui/mcpo
- Source read from the 0.13.5 wheel: `open_webui/__init__.py`, `env.py`, `config.py`, `main.py`, `models/config.py`, `routers/audio/__init__.py`, `routers/retrieval.py`, `retrieval/vector/factory.py`, `utils/images/connections.py`, `utils/models.py`, `utils/tools.py`, `utils/payload.py`, `utils/mcp/client.py`, `routers/openai.py`, `retrieval/loaders/main.py`

llama.cpp and stable-diffusion.cpp:
- https://github.com/ggml-org/llama.cpp/releases/tag/v1.1.0
- https://github.com/ggml-org/llama.cpp/releases/tag/v1.0.0
- https://github.com/ggml-org/llama.cpp/blob/v1.1.0/tools/server/README.md
- Router mode at the pinned build: https://github.com/ggml-org/llama.cpp/blob/b12359/tools/server/README.md
- https://github.com/leejet/stable-diffusion.cpp/blob/master/examples/server/README.md
- https://github.com/leejet/stable-diffusion.cpp/blob/master/examples/server/api.md

Alternatives (GitHub API, latest release and licence):
- https://github.com/LostRuins/koboldcpp
- https://github.com/mudler/LocalAI
- https://github.com/janhq/jan
- https://aiweekly.co/alerts/koboldcpp-v1116-adds-mcp-support-and-three-new-image-models (third party)

Other local services (licence and size lookups):
- Apache Tika: https://github.com/apache/tika , https://repo1.maven.org/maven2/org/apache/tika/tika-server-standard/3.4.0/
- Java runtime sizes: https://api.adoptium.net/v3/assets/latest/21/hotspot?image_type=jre
- https://github.com/searxng/searxng , https://github.com/yacy/yacy_search_server , https://github.com/docling-project/docling-serve , https://github.com/open-webui/pipelines , https://github.com/open-webui/open-terminal , https://github.com/jupyter-server/jupyter_server

Svalbard (read only):
- `host-cli/internal/builder/pythonvenv.go`, `recipes/tools/svalbard-python.yaml`, `recipes/tools/angr.yaml`, `recipes/apps/pi.yaml`, `recipes/tools/llama-server.yaml`
- `drive-runtime/internal/chat/chat.go`, `internal/serveall/serveall.go`, `internal/agent/agent.go`, `internal/llamaserve/flags.go`, `internal/mcp/*.go`, `cmd/svalbard-drive/main.go`

Measurements made for this report (macOS arm64 host, `uv` 0.13.5, CPython 3.12.11):
- `uv pip install --dry-run --no-build --target … --python-platform <p> --python-version 3.12` for the default set, the slim list and the slim lock, on all four platforms.
- Real `--target` installs of the default set and the slim set for macOS arm64, sized with `du`, then deleted.
- Download sizes summed from PyPI file metadata for each resolved package.
