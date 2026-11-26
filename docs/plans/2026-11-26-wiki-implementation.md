# Drive wiki: implementation plan

Date: 2026-11-26. Specification: [wiki design](2026-11-26-wiki-design.md). Research:
[wiki app](research/2026-11-26-wiki-app.md). This plan orders the work and names the checks; it reopens
no decision of the specification.

Sizes: S one module, M several modules or a small design choice, L needs a design first.

```text
 chunk 1                      chunk 2                          chunk 3            chunk 4
┌──────────────────────┐    ┌──────────────────────────────┐  ┌───────────────┐  ┌────────────┐
│ wiki capability      │    │ recipe + presets             │  │ real binary   │  │ docs and   │
│ 8 wiki_* tools       │───▶│ launcher: state, socket,     │─▶│ on exFAT,     │─▶│ checkpoint │
│ registration, names, │    │ proxy, settings              │  │ real model    │  │            │
│ prompt texts         │    │ action, menu, Serve all      │  │ runs          │  │            │
└──────────────────────┘    └──────────────────────────────┘  └───────────────┘  └────────────┘
```

## Found before planning

Read from the SilverBullet 3.0.1 source and tried once against the release binary:

- `users.json` holding `{}` and a `spaces.json` with one space boot the server without setup, with no
  Dashboard login. The state folder also needs `server.json` with `runtimeApi: false`: without it one
  `POST /.runtime/lua` makes SilverBullet start the host's Chrome headless.
- The client uses no WebSocket. It holds one server-sent event stream (`GET /.events`).
- `GET /.ping` answers 200 with the page folder in `x-space-path`. The launcher uses it for readiness, and
  the tools use it to find a running wiki.
- A page name that ends like a file extension (`Version 1.2`) is a file to SilverBullet, not a page, and
  it refuses `@ # | < > $`, `[[` and `]]` in names. The tools refuse the same names.
- A drive's `type: binary` entries already get a generated Tools entry each, one per platform, with no
  action behind it. The wiki's recipe is left out of that rule and gets one fixed entry.

## Chunk 1: the wiki capability

Size: M. An L only if confining the trash needs more than a second `workspace.Open`.

Goal: the eight `wiki_*` tools on the drive's MCP server, registered only when the `silverbullet` recipe
is installed, and named in every place that lists the drive's tools.

Files:
- `drive-runtime/internal/workspace/`: per-instance wording and hidden-name rule (`OpenWith`), `Rename`
  that refuses to overwrite, and `MoveTo` another workspace. `Open` keeps today's behaviour.
- `drive-runtime/internal/wiki/` (new): the recipe ID, folder names, the seed page, and `RunningURL`,
  which asks the preferred ports for `/.ping` and accepts the one that serves this drive's folder.
- `drive-runtime/internal/mcp/wiki.go` (new): the capability. `server.go`: registration in
  `DriveCapabilities`, so `DriveToolNames` follows.
- `drive-runtime/internal/agent/agent.go`: pi's `AGENTS.md` text and the oh-my-pi prompt gain a wiki line
  when the tools are registered. `drive-runtime/internal/openwebui/openwebui.go`: the "Svalbard vault"
  description.

Tests: name checks; create refusing an existing page; append creating one; edit needing a unique match;
move refusing to overwrite; delete going to the trash, twice; search ranking, caps and the empty-result
note; read paging and nearest pages; every escape against both folders; registration with and without the
recipe in `DriveToolNames`, the tool-count tests, the two prompts and the Open WebUI description.

User path: the runtime's `mcp --drive` over stdio on a scratch drive lists 8 tools without the recipe and
16 with it, and a `wiki_create` call leaves `wiki/<name>.md` on the drive.

## Chunk 2: recipe, launcher, menu, Serve all

Size: M. An L if SilverBullet needs a request the proxy rules cannot tell from a cross-site write.

Goal: `./run wiki` and the menu entry "Wiki" open SilverBullet on localhost with a seeded start page and
no login; Serve all starts it local only.

Files:
- `recipes/tools/silverbullet.yaml` with the four pinned zips, the licence and the `settings:` block;
  `presets/packs/tools-base.yaml` and `presets/default-2.yaml`; mirrored by `scripts/sync-catalog.sh`.
- `drive-runtime/internal/wiki/`: the state template (`users.json`, `spaces.json`, `server.json`), the
  socket folder under `/tmp`, the guarded reverse proxy, readiness, cleanup, `Run` and `Start`.
- `drive-runtime/internal/binary/`: a resolver that looks on the drive only, so a host `silverbullet` of
  another version is never started.
- `drive-runtime/internal/actions/actions.go` and `cmd/svalbard-drive/main.go`: the `wiki` native action.
- `host-cli/internal/toolkit/toolkit.go`: the fixed "Wiki" entry and its `wiki` alias.
- `drive-runtime/internal/serveall/serveall.go`: the wiki service, always on 127.0.0.1.

Proxy choices the specification leaves open:
- A request with an `Upgrade` header is refused. SilverBullet 3.0.1 has no WebSocket endpoint, and a
  WebSocket handshake is a GET that browsers send across origins.
- Streaming passes through unbuffered, for `/.events`.
- A write with neither `Sec-Fetch-Site` nor `Origin` is let through. Every browser sends `Origin` on a
  cross-origin write, so such a request comes from a local program, which could write the files directly.
- `/.proxy` is refused as a path segment anywhere, after decoding, and a path that is not in its clean
  form is refused, so no spelling reaches SilverBullet's fetch proxy.

Tests: the three template files; readiness, timeout and cleanup of the state and socket folders with a
fake binary; each proxy refusal with the page folder checked afterwards, and same-origin requests and the
event stream passing; the menu entry present once for a four-platform install and absent without the
recipe; the settings read with and without a schema; Serve all's plan and its "local only" line; the
catalog loading the recipe with both settings.

User path: on a scratch drive made by `svalbard apply`, `./run wiki` prints the address, the page opens
with the seed, `./run settings` lists the two wiki settings, and Ctrl+C leaves no process, socket or
state folder.

## Chunk 3: real binary and real models

Size: M. An L if the watcher misses changes on exFAT and `poll` has to become the default.

Goal: the specification's "done when" list, on a vault applied to an exFAT disk image on macOS arm64.

Checks: a page written through the tools shows in the UI and in SilverBullet's search with no restart;
a deleted page is in the trash; the wiki opens after the image is mounted at a second path; `lsof` shows
no TCP listener for SilverBullet; a cross-site POST through the proxy changes nothing; one pi run with
Qwen3.5 4B that writes a wiki page from vault content, with the prompt tokens of its first request
measured; one Open WebUI run if disk and time allow.

No test code is expected from this chunk, only fixes for what it finds.

## Chunk 4: documentation and checkpoint

Size: S.

Goal: the README section for the wiki, the agent skill text if it lists drive tools, and this file
turned into the implementation record: what was verified and how, what was not, deviations, follow-ups
with sizes.
