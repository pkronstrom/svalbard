# Drive wiki with agent write tools: design

Date: 2026-11-26. Status: approved by the owner, including the post-review security changes. Revised after a four-model review (Sol, Sonnet, Grok,
GLM); the changes are listed at the end.
Research: `docs/plans/research/2026-11-26-wiki-app.md` (LeafWiki v0.15.0 vs SilverBullet 3.0.1, read at
their release tags). Builds on the Open WebUI file tools and the stick settings store, both on local `main`
(`docs/plans/2026-11-26-open-webui-implementation.md`, `docs/plans/2026-11-26-stick-settings-implementation.md`).

## Goal

A wiki on the stick that the user edits in a browser and that every local agent (pi, Goose, opencode and
Open WebUI) can read, search, create, append to, edit, move and delete pages in. Pages are plain Markdown
files on the stick, readable without any app.

Done when, on a real stick:
- SilverBullet opens from the drive menu on localhost, with a seeded start page and no login;
- in Open WebUI and in pi, with a real local model, the request "summarise X from the vault into a wiki
  page" produces a page under `wiki/` that shows in SilverBullet's UI and its search without a restart;
- a page an agent deletes is in the trash folder, not gone;
- the wiki still opens after the stick is mounted at a different path.

## Decisions

| Decision | Choice | Why |
|---|---|---|
| App | SilverBullet 3.0.x, one recipe, in the default presets | A static binary (17–20 MB zip) that keeps its state out of the page folder and watches the folder, so pages written by agents show up by themselves |
| LeafWiki | Not in this build; a backlog task | It cannot share the page folder: a page `A.md` next to a folder `A/` aborts its whole tree, names outside `[A-Za-z0-9_-]` are skipped, it writes `index.md` into every folder, and it does not notice external changes |
| Page folder | `<drive>/wiki/` | The pages are the user's content; SilverBullet is a viewer |
| Agent scope | list, read, search, create, append, edit, move, delete | Owner's choice. Delete is a soft delete into a trash folder |
| How agents reach it | A `wiki` capability inside the one existing svalbard MCP server | The simplest path: stdio for pi, Goose and opencode, and `/mcp` for Open WebUI, are already wired |
| Open WebUI switch | No separate switch for writing | Owner's choice: in a chat, the wiki tools switch on and off together with vault search |
| Agent search | A keyword scan of the Markdown on disk, inside the MCP tool | A personal wiki is small, so a scan takes milliseconds, needs no index, and always sees fresh pages |
| Network | No accounts. SilverBullet listens on a Unix socket, behind the launcher's proxy on 127.0.0.1, which refuses foreign `Host` headers and cross-site writes | Without accounts, SilverBullet has no cross-origin protection of its own. LAN sharing needs accounts and is a follow-up |

## Components and data ownership

```text
 <drive>/wiki/        ◄── pages: plain .md files, the only source of truth
     ▲                         ▲
     │ watches (notify)        │ list / read / search / create / append / edit / move / delete
 SilverBullet server       wiki capability (drive-runtime/internal/mcp), in mcp.DriveCapabilities
 Unix socket, no accounts      ├─ stdio ──────────────► pi · Goose · opencode
     ▲                         └─ /mcp (Streamable) ──► Open WebUI
 launcher proxy 127.0.0.1:<port>  (refuses foreign Host and cross-site writes)  ◄── browser
 state: per-run host temp dir      trash: <drive>/.svalbard/state/wiki-trash/
        (users.json, spaces.json, search index, socket; deleted on exit)
```

- **The pages are authoritative, and they are the only wiki data on the stick.** SilverBullet's state
  (config, search index, session files, socket) lives in a fresh temporary folder on the computer, which
  the launcher writes at each start and deletes on exit. The search index rebuilds from the pages, which
  takes seconds for a personal wiki. SilverBullet writes its account and session files to the state
  folder, not the page folder (`server/src/auth/browser_sessions.rs:51`).
- **Page rule:** page `A/B` is `wiki/A/B.md`, and page `A` is `wiki/A.md`. This is SilverBullet's own rule.
- **Trash** is outside the page folder, so SilverBullet does not show it.

## Agent tools

One `wiki` capability added to `mcp.DriveCapabilities(driveRoot)`
(`drive-runtime/internal/mcp/server.go:104`). It is registered only when the SilverBullet recipe is installed.
It has no separate endpoint and no setting of its own.

| Tool | Behaviour | `Writes` |
|---|---|---|
| `wiki_list` | Page names (`A/B`, no `.md`) with size and modified time, to a chosen depth, capped | no |
| `wiki_read` | A page in parts, reusing `mcp/read.go`'s paging (`offset`, a `continues` line) and its near-miss "nearest pages" errors | no |
| `wiki_search` | Case-insensitive keyword scan of page names and text. Every term must match. Results are ranked by matches, each with short line excerpts, and capped. When nothing matches it returns a note listing what the wiki holds, copying the search tool's empty-result pattern, which stopped small models from looping | no |
| `wiki_create` | Creates a page; refuses if it exists | yes |
| `wiki_append` | Adds text to the end after a blank line; creates the page if it is missing | yes |
| `wiki_edit` | Replaces one exact piece of text that occurs only once, as `file_edit` does | yes |
| `wiki_move` | Renames a page; refuses to overwrite; links in other pages are not rewritten | yes |
| `wiki_delete` | Moves the page into the trash under a timestamped name and returns where it went | yes |

Rules:
- **Pages are addressed by name, not path.** The tool adds `.md`, and refuses `..`, absolute paths and name
  segments that start with a dot.
- **Two confined folders.** Pages go through `workspace.Open(<drive>/wiki)` and the trash through a second
  `workspace.Open(<drive>/.svalbard/state/wiki-trash)`. Both are on the stick, so a delete is a rename on
  one volume. The workspace's error text and dot-name rule are workspace-specific today; making them
  per-instance is a small change in that package.
- **Atomic writes and a size cap:** each write is a temporary file renamed into place (2 MiB cap), so a
  page is never half-written.
- **Concurrent edits: last write wins.** If the UI saves the same page at the same moment as an agent, one
  of the two edits can be lost. `wiki_edit`'s exact-match check catches most such cases. No locking.
- **Results give the next step.** Every write result returns the page name and its URL in the running wiki
  when one is up. Errors say what to send instead, as in the file tools.
- **Wiki pages are not a source of the drive's `search` tools.** Agents use `wiki_search`.

### What the change touches beyond the capability

These are required parts of the change. Without them, launchers that enable only named tools drop the wiki
tools, and models told the drive is read-only will not try to write:
- `mcp.DriveToolNames(driveRoot)` includes the wiki tools when they are registered. The oh-my-pi
  launcher builds its enabled-tool list from it.
- pi's generated `AGENTS.md`, the oh-my-pi prompt (`piDriveContext` / `ompLeanPrompt`), and Open WebUI's
  "Svalbard vault" server description each gain one line about the wiki.
- Tests that assert "exactly eight vault tools" are updated for an installed wiki and a missing one.

### Prompt cost

Measured on a `main` build: the current eight drive tools' definitions are 5,225 characters, about 1,300
tokens. The eight wiki tools, with descriptions as long as the file tools', add about 1,500 tokens, so
pi's first prompt grows from about 2,850 to about 4,350 tokens. A 16K context stays comfortable. The open
task for a reduced pi tool set at 8K context leaves the wiki tools out of that set; this design does not
add its own reduced set.

## Launching SilverBullet

- **Recipe:** `silverbullet` 3.0.1, the `silverbullet-server-<os>-<arch>.zip` asset for the four platforms,
  pinned with the release sha256. It is in the default presets. The binary inside the zip is `silverbullet`.
- **Native action:** a `wiki` action registered in `nativeActionSpecs`, with a menu entry "Wiki" added by
  an explicit toolkit rule for the recipe ID, as Open WebUI's entry is (`host-cli/internal/toolkit/toolkit.go`).
  `type: binary` recipes get no menu entry otherwise. Following the Open WebUI launcher
  (`drive-runtime/internal/openwebui`), the action:
  1. creates `wiki/` and seeds `index.md` when the folder is empty. The seed explains the wiki, that agents
     can write to it, and where the trash is.
  2. **creates a fresh state folder in the host's temp directory and writes SilverBullet's config into
     it**, rather than running `silverbullet setup`, which requires an admin password, turns on git
     revisions inside the page folder and applies a starter kit (`server/src/multi/setup.rs:31-39, 138`).
     The config is:
     - `users.json` with no accounts;
     - `spaces.json` with one space at `/`, anonymous access `write`, revisions disabled, and the page
       folder given as the absolute path of `wiki/` on the current mount.

     Both files come from a template that is tested against the pinned version. Because the state is
     rebuilt on every start, a stick mounted at another path, or a format change, cannot leave stale
     state. Keeping the state off the stick also avoids running tantivy's memory-mapped index and lock
     files on exFAT. SilverBullet ignores `SB_SEARCH_DIR` in this mode (`bin/silverbullet/src/multi.rs:57`),
     so the index could not be moved off the stick on its own.
  3. starts `silverbullet <state folder>` with `SB_UNIX_SOCKET` pointing at a socket, so SilverBullet has
     **no TCP listener** a browser could reach directly. A socket path is limited to about 104 bytes on
     macOS, and macOS's `$TMPDIR` is long, so the socket goes in a short folder created for it under
     `/tmp`, readable only by the user. The environment also sets `SB_UPDATE_CHECK=0`, and
     `SB_FS_WATCH` from the `fs_watch` setting (`auto` by default, `poll` as the fallback for sticks where
     the watcher misses changes).
  4. **serves a small reverse proxy on `127.0.0.1:<free port>`** that forwards to the socket and refuses:
     - any request whose `Host` is not `localhost:<port>` or `127.0.0.1:<port>`, which blocks DNS
       rebinding;
     - any non-GET/HEAD request whose `Sec-Fetch-Site` is `cross-site` or `same-site`, or, when that header
       is absent, whose `Origin` is not the proxy's own. This blocks writes from other web pages.
       SilverBullet itself skips its cross-origin check when there is no session cookie
       (`server/src/router.rs:115`), so with no accounts nothing else stops a page from posting to
       `/.fs/…` (`router.rs:288`).
     - every request under `/.proxy/`. SilverBullet's own fetch proxy (`router.rs:340`) returns remote
       content with its original `Content-Type` (`handlers/proxy.rs`), so attacker HTML would run on the
       wiki's origin and pass both checks above. An offline wiki never needs it.
  5. waits until the proxy answers, opens the browser if `open_browser` is on, and on Ctrl-C or menu exit
     stops the child and deletes the state folder.
- **Serve all:** starts the wiki too, always on 127.0.0.1, even when Serve all binds the LAN, because the
  server has no login. Its status line says "local only".
- **Settings**, declared in the recipe's `settings:` block and read with `settings.OpenForAction`:
  `open_browser` (bool, default true) and `fs_watch` (`auto` or `poll`).

## Security

- **The SilverBullet server is local only** (127.0.0.1, no accounts). **The pages are not private on the
  LAN:** Serve all's Files server already serves the whole drive read-only on its bind address
  (`drive-runtime/internal/serveall/serveall.go:101`), so `wiki/` and the trash can be read there like
  everything else on the drive. This design does not change that.
- **Every MCP listener binds 127.0.0.1** (`cmd/svalbard-drive/main.go:301-327`), so the write tools are
  not reachable from the LAN.
- **Agents act on untrusted text.** A model reading injected vault content could edit or delete pages.
  Deletes are recoverable from the trash. Edits, appends and moves are not versioned in this build. This is
  an accepted risk; SilverBullet revisions (git) are a possible follow-up.
- **Other web pages in the user's browser cannot write to the wiki.** The launcher's proxy refuses foreign
  `Host` headers, which covers DNS rebinding, and refuses cross-site writes. SilverBullet listens only on a
  Unix socket, so nothing can go around the proxy. Any local process can still write pages, as it can
  write any file on the stick.

## Failure behaviour

- **Launcher:** a missing binary, a taken port, or a server that does not answer within the timeout gives
  a clear message and a clean exit, with no child left running.
- **The launcher is killed without cleaning up:** the temporary state folder is left in the host's temp
  directory, which the OS clears; the next start uses a fresh folder.
- **The watcher misses changes on an exFAT stick:** set `fs_watch` to `poll`. Watching is the one part of
  SilverBullet that still touches the stick.
- **Writes:** a read-only or full stick, or the size cap, returns a tool error worded for the model.
- **Trash:** a timestamp suffix keeps pages deleted twice apart.
- **SilverBullet not installed:** no wiki tools are registered, and the MCP server is otherwise unchanged.

## Testing

- **Go unit tests in a temporary folder:**
  - for the capability: name checks, create refusing an existing page, append creating a missing page,
    edit requiring a unique match, move refusing to overwrite, delete going to trash, search ranking, caps
    and the empty-result note, and registration only when the recipe is installed (including
    `DriveToolNames` and the tool-count tests)
  - for the launcher: the generated `users.json` and `spaces.json` (absolute page folder, write access,
    revisions disabled), and readiness, timeout and state-folder cleanup with a fake binary
  - for the proxy: a foreign `Host` is refused; a foreign-`Origin` `text/plain` POST and a
    `Sec-Fetch-Site: cross-site` PUT are refused without creating or changing a page; same-origin
    requests pass; any `/.proxy/` path is refused
- `make verify`, covering the catalog sync for the recipe.
- **Real-binary checks with SilverBullet 3.0.1 on macOS arm64, from an exFAT disk image:**
  - an agent-written page shows in the UI and in search;
  - the wiki opens after the image is mounted at a second path;
  - SilverBullet has no TCP listener (`lsof`), and a cross-site POST through the proxy changes nothing.
- **Real-model checks:** one Open WebUI run and one pi run, each producing a wiki page from vault content.
- **Owner's rules for test runs:** no default browser, an explicit local model endpoint with isolated HOME
  and XDG, deletes only by literal absolute path, and a warning before large downloads.

## Out of scope (follow-ups)

- LeafWiki as an opt-in standalone wiki with its own folder (backlog task).
- Indexing wiki pages into `data/search.db`, keyword and semantic, on the host through the PDF-sources
  document pass, which is being kept open to a second kind of file.
- Semantic search over wiki pages on the stick.
- LAN sharing with accounts.
- Rewriting links when a page moves, and versioning edits.
- Checking the watcher on Linux and on an Intel Mac.
- SilverBullet's own semantic search model and its built-in `/.mcp` endpoint.

## Changes after review

- LeafWiki dropped, which removes the shared folder, the resync loop, the PID file and the signal
  handling. The page folder is `wiki/`, not `wiki/root/`.
- SilverBullet's state is written by the launcher, not by `silverbullet setup`, into a per-run host temp
  folder. This removes exFAT from the index's path and makes a change of mount path harmless.
- SilverBullet runs on a Unix socket behind the launcher's proxy, which refuses foreign `Host` headers and
  cross-site writes. SilverBullet itself has no cross-origin protection when there are no accounts. This
  came from the second-round review. The third round added blocking `/.proxy/`, which would otherwise let
  remote HTML run on the wiki's origin.
- The trash is a second confined folder.
- The "local only" claim is corrected: Serve all already shares the whole drive read-only.
- Required updates to `DriveToolNames`, prompts, descriptions and tests are named. Prompt cost is measured.
- The "agents act on untrusted text" risk is recorded.
