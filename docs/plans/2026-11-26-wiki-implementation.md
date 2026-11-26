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
                  │  refuses: foreign Host · every write not from its own page ·
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
  environment also sets `SB_MCP=0`, `SB_SHELL_BACKEND=off`, `SB_DISABLE_SERVICE_WORKER=1`, `NO_COLOR=1`
  and `HOME` to the state folder, and carries nothing of the host's but PATH, locale, time zone and
  TMPDIR. Without its service worker the tab does not go on answering after the wiki has stopped.
- **Proxy.** Beyond the three refusals of the specification:
  - a request with an `Upgrade` header is refused (3.0.1 has no WebSocket endpoint, and a handshake is a
    GET that browsers send across origins);
  - a path that is not in its plain form is refused, and `.proxy` is refused as a path segment at any
    depth after decoding, so no spelling reaches the fetch proxy;
  - the one event stream (`GET /.events`) passes unbuffered;
  - a write passes only when `Sec-Fetch-Site` is `same-origin` or `none`, or, without that header, when
    `Origin` is the proxy's own. A write with neither header is refused: a browser from before 2020 can
    send one from a form, and nothing of the drive's writes through the proxy;
  - forwarding headers from the client are not passed on, because SilverBullet derives its origin from
    them;
  - every answer carries `X-Frame-Options: SAMEORIGIN`, `Cross-Origin-Resource-Policy: same-origin` and
    a content policy that keeps requests, images, media, fonts, frames and scripts on the wiki's own
    address (`default-src 'self'`, `connect-src 'self' data: blob:`, `img-src 'self' data: blob:`,
    `object-src 'none'`, `base-uri 'self'`, `form-action 'self'`, `frame-ancestors 'self'`). Scripts keep
    `'unsafe-inline' 'unsafe-eval' blob:`: that was the first policy tried and SilverBullet's client
    raised no violation under it; a tighter script rule was not tried. A user's page that embeds an
    image from the internet no longer shows it.
- **Pages are code to SilverBullet, and the tools write none.** It evaluates `${...}` as Lua when a page
  is shown, with the browser's JavaScript in reach, and it loads `space-lua` blocks from pages under
  `Library/` for the whole wiki. So:
  - the write tools refuse any page under `Library/`, in any letter case;
  - text they write has each `${` outside a fenced block or code span saved as `\${`, and the result
    says so: `The text had "${" 2 times outside a code block. The wiki would run that as code, so it
    was saved as "\${", which the wiki shows as the same text.` Of five spellings tried on the real
    binary, `\${`, `$\{` and a code span are shown and not run; an HTML entity is shown as typed;
  - an edit that joins a `$` and a `{` already on the page is refused;
  - `space-lua` and `space-style` fences need no change: 3.0.1 runs them from `Library/` pages only;
  - an expression the user wrote stays, and a tool can edit around it. See "Open risk" below.
- **One write at a time** inside one MCP server: several appends sent in one turn all arrive.
- **Existing pages keep their names.** The naming rules apply to pages the tools create. A page the user
  named `Recipe: bread` is read, edited, moved and deleted under that name.
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
- Proxy: foreign `Host` (eight forms) refused; ten kinds of write that are not the page's own refused
  for five methods with the page folder compared afterwards; fourteen spellings of `/.proxy` refused for
  three methods; unclean paths, upgrades and forwarding headers; the frame, form and resource headers;
  the wiki's own reads and writes passing; an event arriving while its stream is open.
- After the review: writes under `Library/` in eleven forms; `${` in created, appended and edited text
  saved as `\${` with the note, left alone inside fences and code spans, and eight edge cases (an
  unclosed span, a closed fence, an escaped backslash, text already inert); an edit that assembles one
  from pieces refused; a user's own expression kept through an edit; 20 appends and 20 creates sent at
  once (also under the race detector); freely named pages read, edited, moved and deleted; a move that
  changes only letter case; the content policy naming no other address.
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

## Review

An independent review of the first two chunks read the diff against the SilverBullet source. Its eleven
findings and what was done:

| Finding | Done |
|---|---|
| Pages the tools write can be code in the user's browser | `Library/` refused, `${` saved inert, a content policy on every answer; the rest is the open risk below |
| The browser keeps a service worker and a copy of the notes | Service worker off. The page index in IndexedDB stays: open risk |
| Any site can frame the wiki | Frame, form and resource headers added by the proxy |
| Parallel tool calls lose writes (20 appends kept 2) | One write at a time; 20 of 20 arrive |
| A write with neither origin header passes | Refused; unknown `Sec-Fetch-Site` values too |
| Pages with names the rules refuse cannot be opened | The rules apply to new pages only |
| `wiki_move` cannot change only the letter case | It can |
| A closed terminal skips cleanup | Fixed before the review arrived |
| The port probe follows redirects | It does not; the other points are accepted and listed below |
| A test passed without the code it named | It now checks that the host's binary never ran |
| `binary.Resolve` fell through to PATH on an unsupported platform | Restored; the file tools' rename error text is as it was; `wiki.Installed` removed |

After these changes the real binary was run again, in an isolated browser profile:

- The page loads without a service worker, a tool append reaches the open tab, typed text is saved, the
  search answers, and SilverBullet's own client raised no policy violation. A write without origin
  headers gets 403. From a script on the wiki's page, a request to another address, to another
  localhost port and an image from another site were each blocked by the policy.
- A page with `${js.window.localStorage.setItem("lua_ran", "yes")}` written by hand set the marker when
  viewed. The same text sent through `wiki_create` was saved as `\${...}`, shown as text, and set
  nothing.
- **pi again, on the final build, from the exFAT image:** `wiki/Ibuprofen for adults.md` (3,578 bytes)
  after one search, reads and one `wiki_create`; first request 4,268 tokens; the reply ended with the
  page's link; the page showed in the UI and its search.

Evaluated and not shipped:

- **Lua off** (`?disableSpaceLua=1`, the same switch the proxy could set in `/.config`): the marker page
  did not run and expressions show nothing. Viewing, the page list and navigation work. The Search
  button answers "Error: Command Space: Search not found", because SilverBullet's standard library is
  Lua; its other commands go the same way. Templates and queries were not tried: they are expressions.
- **`ephemeralStorage`** (set by a proxy that rewrote `/.config`, not committed; the variant is kept
  as `proxy.go.ephemeral-experiment` in the scratch folder): in a fresh browser profile the page showed
  in about 5 s, as without it, the search dialog worked, and the profile held no IndexedDB database,
  no cache and zero bytes of storage. Without it the same wiki leaves `sb_data_*` databases behind.
  Cost seen on a four-page wiki: none. Expected cost on a large one: the page index is rebuilt in the
  browser at each load. It is about 20 lines in the proxy.
- **A sweep of stale state folders at start** was left out: two drives' wikis run at once, and telling
  a stale folder from the other wiki's live one needs a liveness check.

## Open risk, for the owner

The specification's accepted risk is that a model acting on injected vault text can edit or delete
pages. SilverBullet adds a second one: **a page is a program**. What is closed and what is not:

- Closed for the tools: scripts under `Library/`, and `${...}` expressions, the two ways the 3.0.1
  documentation and source give for a page to run code. The content policy then keeps code that ran
  anyway from sending the notes to another address by a request, an image or a form.
- Not closed: this is a list of known ways, not a proof. Another construct that runs when a page is
  shown would pass. Code that runs has the wiki's address: it can rewrite or erase pages through
  SilverBullet's own file API, past the trash, and it can navigate the tab away. It does not reach the
  host's files: shell, the runtime API and the fetch proxy are off.
- Not closed: a page the user pastes or writes with `${...}` runs, as SilverBullet intends.
- Not closed: the browser profile keeps SilverBullet's page index in IndexedDB after the stick is gone.
- The owner decided both choices (see "Live pages and a browser that keeps nothing"): Lua stays on
  and agents may write live pages, with a setting to switch that off; and the browser keeps nothing.
- Versioning the page folder would make any change undoable; it is a follow-up.
- Accepted from the review without change: a local process that answers `/.ping` on ports 8088 to 8107
  with this drive's folder gets its address returned as the page link; a port that accepts and never
  answers costs a write 300 ms; a wiki pushed to a port outside that range is not found, and writes
  then return the file path; the printed address says `localhost` while the proxy listens on
  127.0.0.1, so a server on `[::1]` at the same port would get the browser.

## Live pages and a browser that keeps nothing

Added 2026-11-26 on branch `wiki-live`, after the owner answered the open risk below: agents may use
SilverBullet's features, and the browser keeps nothing between visits.

What was built:

- **Setting `silverbullet.live_pages`** (bool, default on, label "Agents may write live pages"). On: the
  tools write `${...}`, `space-lua` blocks and pages under `Library/` as given. Off: the refusals of the
  review, now naming the setting. It is read at each write, so a change needs no restart.
- **Help page `Help/Live pages`**, seeded with the start page and restored by the launcher when
  missing; `wiki_read` answers for it from the built-in text when the wiki has no file for it. Its
  query examples are live on the page, not in code blocks.
- **Pointers, not a syntax guide.** `wiki_create`'s description gains one clause (99 characters) and
  pi's `AGENTS.md` one sentence, both only while the setting is on.
- **The proxy sets `ephemeralStorage`** in the answer to `/.config`, asks SilverBullet for that answer
  uncompressed, and answers 502 if it cannot read it as the settings.

Real SilverBullet 3.0.1, isolated browser profiles:

- **Every example on the help page works**, from the exFAT image, under the content policy and with
  the browser keeping nothing: the pages of a folder, the five changed last, a count (8), today's
  date, the open tasks, and a command defined on a `Library/` page, which was listed and ran.
- **Nothing loosened** in the content policy for this: a live query raised no violation.
- **After a visit** the profile held no IndexedDB database, no cache, no service worker, no cookie and
  zero bytes of storage. It held four localStorage entries, 1,070 characters in all: font metrics, the
  settings answer (with the page folder's path), theme and fonts. None had note text.
- **Load and re-index with 310 small pages** (internal disk): page text 0.30 s after navigation; a
  query over all pages showed 0 first and the full 310 at 1.39 s. So each load re-indexes for about a
  second, during which a live query shows a short result. Search answers from the server at once.

Real model, Qwen3.5 4B through pi, from the exFAT image, the same request three times ("Make a wiki
page called Projects overview that shows a live list of all pages under the Projects folder, using a
query so the list updates by itself."):

| Run | What pointed to the help page | Calls | Result |
|---|---|---|---|
| 1 | The clause in `wiki_create`; examples in code blocks | list, create, read, read help, edit, read | Wrote `{{#query ...}}` first, read the help page afterwards, then copied a query with its code block around it and broke `template.new`'s brackets. Shown, not run |
| 2 | The same clause, reworded; examples live | list, create | Never read the help page. Wrote `!list(Projects/*)` in a code block |
| 3 | The clause and one sentence in pi's `AGENTS.md` | read help, create | A valid query, copied exactly. In the wiki it listed Compost, Garden plan, Overview and Shed, and a page added afterwards joined the list by itself |

- Three runs are three samples. The description clause alone did not make the 4B model read the page;
  the sentence in the context file did, once. Open WebUI, Goose and opencode have no such context
  file, so there the clause is all a model gets. Transcripts of the three runs are in the scratch
  folder (`pi-live-query*.session.jsonl`).
- In run 3 the model named the page `Projects/Overview`, not `Projects overview` as asked.
- **Prompt cost.** pi's first request: 4,268 tokens before, 4,298 with the clause, 4,352 with the
  clause and the sentence: **+84 tokens**. The help page is 1,924 bytes and is read only when needed.

Not verified for this part: Open WebUI, Goose, opencode and oh-my-pi writing a live page; a larger
model; a wiki of thousands of pages (the re-index time grows with it); `template.new`, which was taken
off the help page after the 4B model broke its brackets.

Undo, sized against the existing follow-up for versions: a copy of a page into a third confined
folder (`.svalbard/state/wiki-history/`) before a tool changes or moves it is an S, with the trash as
its pattern. It becomes an M with a tool or menu entry to restore from it and a rule for how much to
keep. It does not need git, so it is smaller than the "versions for edits" follow-up (M, L with git on
exFAT) and covers the case the accepted risk leaves open only for changes made through the tools:
code that runs in the browser writes through SilverBullet, past any snapshot the tools take.

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
| A snapshot of a page before a tool changes it, as the undo for agent writes | S | M with a way to restore and a rule for how much to keep |
| Make the other clients find the help page before writing a live page (they have no context file) | S | M if each client needs its own wording or a system prompt |
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
