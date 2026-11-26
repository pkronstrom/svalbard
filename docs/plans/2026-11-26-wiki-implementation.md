# Drive wiki: implementation record

Status: done 2026-11-26 on macOS arm64, from an exFAT disk image and with a real model. Not run on Linux,
Intel Mac or a physical stick.

Specification: [wiki design](2026-11-26-wiki-design.md). Research: [wiki app](research/2026-11-26-wiki-app.md).
This file was the plan (four chunks: tools, launcher, real runs, documents) and is now the record of what
was built and how it was checked.

## Resulting user path

1. A vault built from any default preset has the `silverbullet` recipe. The drive's menu gains **Wiki**
   under Apps, and `./run wiki` does the same.
2. It prints `Wiki:  http://localhost:8088` and opens the browser there: SilverBullet with a start page and
   no login. Ctrl+C, or closing the terminal, stops it and leaves nothing on the computer.
3. The pages are Markdown files in `<drive>/wiki/`. pi, Goose, opencode and Open WebUI get eight
   `wiki_*` tools from the drive's MCP server. A page a tool writes is in the open browser tab and in
   SilverBullet's search within a second.
4. A page a tool deletes is in `<drive>/.svalbard/state/wiki-trash/` under its name and the time.
5. Serve Everything starts the wiki too and prints `Wiki:   http://localhost:8088 (local only)`.

```text
 browser ──► 127.0.0.1:8088  proxy in svalbard-drive __native-wiki
                  │  refuses: foreign Host · cross-site and same-site writes ·
                  │           Upgrade · any /.proxy segment · unclean paths
                  ▼
             /tmp/sbw-XXXX/s  Unix socket ──► silverbullet <state>
                                                  │ watches        state: $TMPDIR/svalbard-wiki-XXXX
                                                  ▼                (users, spaces, server .json, index)
 pi · Goose · opencode ── stdio ──┐          <drive>/wiki/*.md
 Open WebUI ── /mcp ──────────────┴─► wiki_* tools ──► <drive>/.svalbard/state/wiki-trash/
```

## What was built

- **Tools** (`drive-runtime/internal/mcp/wiki.go`): `wiki_list`, `wiki_read`, `wiki_search`, `wiki_create`,
  `wiki_append`, `wiki_edit`, `wiki_move`, `wiki_delete`, registered by `mcp.DriveCapabilities` only when
  `.svalbard/recipes.json` has `silverbullet`. `DriveToolNames`, pi's `AGENTS.md`, the oh-my-pi prompt and
  Open WebUI's "Svalbard vault" description follow. The two folders are opened on first use, so listing
  tools creates nothing.
- **Workspace** (`drive-runtime/internal/workspace`): `OpenWith` gives a folder its own name and example
  in errors; `Rename` and `MoveTo` move a file without ever replacing one. `Open` is unchanged.
- **Launcher** (`drive-runtime/internal/wiki`): seeds `index.md` into an empty page folder, writes
  SilverBullet's state into a new folder in the host's temporary directory, starts it on a Unix socket in
  a 0700 folder under `/tmp`, serves the guarded proxy on 127.0.0.1, waits for `/.ping`, and on exit stops
  the child and removes both folders. `wiki` native action; Serve all service.
- **Recipe** `recipes/apps/silverbullet.yaml`: the four 3.0.1 server zips with sha256 computed from the
  downloaded assets (equal to the release's digests), MIT, settings `open_browser` and `fs_watch`. In
  `tools-base` and `default-2`, so in all 16 default and finland presets.
- **Menu** (`host-cli/internal/toolkit`): one fixed "Wiki" entry with the `wiki` alias; the settings group
  is named "Wiki".

## Decisions made while building

Where the specification left a choice, or the release behaved differently from what was read:

- **A third state file.** `server.json` with `runtimeApi: false` and `mcp: false`. Without it, one
  `POST /.runtime/lua` made SilverBullet start the host's Google Chrome headless. The child's
  environment also sets `SB_MCP=0`, `SB_SHELL_BACKEND=off`, `NO_COLOR=1` and `HOME` to the state folder,
  and carries nothing of the host's but PATH, locale, time zone and TMPDIR.
- **Proxy.** Beyond the three refusals of the specification:
  - a request with an `Upgrade` header is refused (3.0.1 has no WebSocket endpoint, and a handshake is a
    GET that browsers send across origins);
  - a path that is not in its plain form is refused, and `.proxy` is refused as a path segment at any
    depth after decoding, so no spelling reaches the fetch proxy;
  - the one event stream (`GET /.events`) passes unbuffered;
  - a write with neither `Sec-Fetch-Site` nor `Origin` passes: every browser sends `Origin` with a
    cross-origin write, so such a request is a local program, which could write the files directly;
  - forwarding headers from the client are not passed on, because SilverBullet derives its origin from
    them.
- **Finding a running wiki.** The launcher and the MCP server are separate processes. No file joins
  them: the tools ask ports 8088 to 8107 for `/.ping` and take the one whose `X-Svalbard-Wiki` header,
  set by the proxy, names this drive's page folder. Two drives' wikis at once each get their own links.
- **Results.** While the wiki is open a write returns the page and a ready Markdown link; otherwise the
  page, its file on the drive and how to open the wiki.
- **Page names** follow SilverBullet's own rules, so no page is written that it cannot show: a name that
  ends like a file type (`Version 1.2`) and the characters `@ # | < > $ " * : ?` are refused with the
  name to send instead; a trailing `.md` is dropped. A name that differs only in letter case from an
  existing page or folder is that page, as it is on exFAT.
- **Reading a near miss.** `wiki_read` reads the one page a name means when it differs in separators or
  lacks its folder, and says so. The writing tools never guess; they name the nearest pages.
- **Seeding.** The first tool call on a drive with no `wiki/` also creates the folder and the start
  page, by the launcher's rule (only into an empty folder).
- **The dot-name rule stayed shared** in the workspace package. Both wiki folders need the rule the
  workspace has, and SilverBullet hides the same names; only the wording became per-folder.
- **Menu and presets.** "Wiki" is the first entry of the Apps group. The recipe is in the `tools-base`
  pack and inline in `default-2`.
- **Never a host SilverBullet.** `binary.ResolveOnDrive` does not fall back to PATH.

## Verified, and how

Unit tests (`make verify`: 60 tested packages, up from 59 with the new `wiki` package; `make build`):

- Tools: accepted and refused page names with the suggested name; create refusing an existing page;
  append creating one and joining after a blank line; edit needing one exact match, with the indentation
  hint; move refusing to overwrite and naming pages that still link; delete into the trash three times
  in one second; search needing every word, name weighting, excerpts, caps, and the empty-result notes;
  read in parts until the page is rebuilt, near misses and nearest pages; list depth, folders and the
  200-entry cap; every escape (`..`, absolute, home, backslash, three kinds of symlink, a symlinked
  trash folder) against all tools with the outside files checked afterwards.
- Registration: 8 tools without the recipe, 16 with it, in the server and in `DriveToolNames`; write
  hints; no folder created by listing; pi's `AGENTS.md` and the oh-my-pi tool list and prompt with and
  without the wiki; Open WebUI's description, also through a start against stand-in servers.
- Launcher, against a stand-in SilverBullet: the three state files; state in the host's temporary
  directory with mode 0600 and not on the drive; a socket path under 40 bytes in a 0700 folder; the
  environment, with host `SB_*` variables kept out; readiness, a server that exits (its last output in
  the error), one that never answers, and Ctrl+C while waiting, each leaving no child, state or socket;
  a second start after the drive moved to another path; two wikis at once; both settings.
- Proxy: foreign `Host` (eight forms) refused; eight kinds of cross-site and same-site write refused for
  five methods with the page folder compared afterwards; fourteen spellings of `/.proxy` refused for
  three methods; unclean paths, upgrades and forwarding headers; the wiki's own reads and writes passing;
  an event arriving while its stream is open.
- Menu, settings schema, catalog: one entry for a four-platform install; the group label; the recipe's
  pins and settings; every default preset has the wiki once.

Real SilverBullet 3.0.1, vault applied by `svalbard apply` to an exFAT disk image (FSKit driver), no
default browser opened (`open` replaced by a script that records its argument; pages in an isolated
browser profile):

- `./run wiki`: the start page showed with no login. `lsof` showed no TCP or UDP socket for
  SilverBullet, only the Unix socket; the runtime held `127.0.0.1:8088`.
- A page created with `wiki_create` over stdio was in SilverBullet's search 0.6 s later with
  `fs_watch = auto`, and opened from the link the tool returned. An append showed in the open tab
  without a reload. The UI's own search dialog found a second tool-written page. Text typed in the
  browser reached the file and `wiki_search`. **`auto` is the right default on exFAT here**; `poll` was
  not needed.
- `wiki_delete` moved the page into the trash on the same volume; the UI no longer listed it.
- From another origin in the same browser (`http://127.0.0.1:9477`, and `http://localhost:9477` for
  same-site): a `text/plain` POST, a POST over `index.md`, a form post, PUT and DELETE with their
  preflights, and a GET of `/.proxy/` all got 403; a cross-origin read failed in the browser. The hashes
  of every page and trashed page were the same afterwards.
- The image was detached and mounted at a second path. The wiki started with the new folder in its
  state, the browser that had the first mount's page open showed the pages, and a tool write worked.
- Ctrl+C and a closed terminal both left no process, no state folder and no socket folder.

Real model, Qwen3.5 4B at 32K, one run each unless said:

- **pi, from the exFAT image.** "Summarise what the vault says about first aid for burns into a wiki
  page called Burns first aid." Six model requests, 28 s of model time: a search, the WikEM Burns
  article read in three parts, one `wiki_create`, and a reply with the page's address.
  `wiki/Burns first aid.md` is 4,831 bytes and showed in the running wiki's UI and search.
- **Open WebUI, from a vault on the internal disk** (not the image; the model file was linked in by
  hand), two chats. Each: one search, three reads, one `wiki_create`, a 5.8 KB page
  (`Paracetamol for adults`, `Ibuprofen for adults`) found by the wiki's search. The paracetamol page
  has the NHS figures (1 or 2 tablets, 4 hours, 8 in 24 hours). The Tools menu showed "Svalbard vault"
  with the wiki in its description.
- **Prompt cost.** pi's first request was 4,265 tokens with the wiki tools and 2,987 without (same
  prompt, same vault): **+1,278 tokens**. Open WebUI's first request was 3,535 tokens with both tool
  servers. The eight wiki definitions are 4,376 characters, the eight vault ones 4,408.

## Found by the real runs and fixed

- **macOS AppleDouble files on exFAT.** Every file written to the image got a `._name` companion. The
  runtime's binary resolver tried to unpack `._tool.tar.gz` and then found no tool at all (the wiki,
  kiwix-serve, llama-server and pi were all "not installed"); kiwix-serve was handed `._name.zim`; and
  `svalbard apply` ended with "indexing failed". Fixed in the resolver, in one list of the drive's
  archives used by search, read, the search screen and Serve all, and in the host's ZIM scan.
- **A closed terminal** ended an action at once, leaving the wiki's state and socket folders. Native
  actions now treat SIGHUP as Ctrl+C.
- **The link in a reply.** Given the file path and the link, the model told the user the path. A write
  now returns the link alone while the wiki is open. In the one chat after the change the model passed
  the address on, but as an image (`![...](...)`).

## Not verified

- Linux and Intel Mac: the code cross-compiles, never run. SilverBullet's watcher there is untested.
- A physical stick: cold start, pulling it while the wiki runs.
- Open WebUI from exFAT. Its Python tree was not tried on a volume without symlinks.
- Goose, opencode and oh-my-pi with a real model. They get the tools by the same server; oh-my-pi's tool
  list and prompt are unit-tested.
- Whether the pages say what the articles say. One error was seen: the burns page gives a wrong Rule of
  Nines line.
- Contexts under 32K, and more than one run per client: this shows the path works, not how often.
- `fs_watch = poll`: the setting reaches SilverBullet (unit test); it was not needed and not run.
- The first start unpacks the zip beside it on the drive (53 to 61 MB, once per platform). On a slow
  stick that takes time before the wiki answers; it was not timed.

## Size

- Download per platform 18.0 to 21.1 MB (recipe `size_gb: 0.020`); all four 78.8 MB.
- Every default and finland preset grows by 0.020 GB with one platform and 0.080 GB with four.
  `default-2` goes from 1.127 to 1.147 GB of its 2 GB.
- On the drive after first use: the unpacked server, 53 MB on macOS arm64.

## Follow-ups

Sizes: S one module, M several modules or a small design choice, L needs a design first.

| Item | Size | Risk that could change the size |
|---|---|---|
| Run on Linux and Intel Mac, and from a physical stick | S | M if the watcher misses changes there and `poll` must be chosen per platform |
| Make small models pass the page link on as a link | S | M if it needs per-client wording, as Open WebUI and pi render links differently |
| Generated Tools menu entries for `type: binary` recipes have no action (one dead entry per platform) | S | M if tools should get a real action or a shell entry instead of being hidden |
| `apply` rewrites the runtime binary in place; on macOS the next `./run` is killed while an action still runs the old one | S | M if other installed binaries need the same replace-by-rename |
| Restore from the trash, and empty it | S | M if it needs a page in the wiki or a menu screen |
| Index wiki pages into the drive's search (keyword, then semantic) | M | L if the document pass cannot take a second kind of file |
| Rewrite links when a page moves; versions for edits | M | L if versions mean git on exFAT |
| LAN sharing of the wiki with accounts | L | — |
| LeafWiki as an opt-in second wiki | M | L if its page folder cannot be kept apart from `wiki/` |
| Upgrade routine for each SilverBullet pin bump: state template and proxy rules against the new release | S per bump | M when a release changes its state files or adds a transport |
