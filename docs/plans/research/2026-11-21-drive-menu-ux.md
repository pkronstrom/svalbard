# Drive menu UX: simulation on a realistic vault (research, 2026-11-21)

Research only; nothing in the repo was changed. Code references are to `7350384` (tip of
`catalog-refresh-2026-09`). Broken pack actions are already listed in the
[pack operations inventory](../2026-11-21-pack-operations-inventory.md). This document cites that list and doesn't
re-derive it.

## Method

- **Rich vault (placeholders).** A temporary Go helper resolved `default-64` plus `tools-base`, `cybersec`,
  `esp32-dev`, `ai/models-16gb-ram`, `ai/harnesses`, `fi-maps` and `survival-medical`, then applied
  `recipes/dep-defaults.yaml`. That gave 114 recipes: 50 zim, 29 binary, 13 toolchain, 8 pmtiles, 5 app,
  3 python-package, 2 gguf, and one each of gguf-embed, python-venv, pdf and gpkg. The helper wrote a small
  placeholder file at each path the generator expects, built the menu map the same way as
  `host-cli/internal/apply/apply.go:240-263`, and called the real `toolkit.Generate` with the runtime binaries
  from `make build-drive-runtime`. I built two variants: **multi** (host platforms macos-arm64 and linux-x86_64,
  148 realized entries) and **single** (macos-arm64 only, 112 entries).
- **Small real vault.** I ran `svalbard add permacomputing devdocs-c duckdb-wasm sqlite3` and then `apply`
  (119 MB). Kiwix had to be added separately, because `svalbard add` doesn't pull dep defaults; only the TUI
  wizard does (`host-cli/internal/cli/root.go:531`). Apply built the keyword index automatically. In this vault,
  search, opening a result, ZIM browsing, DuckDB, inspect and verify all really run.
- **Terminal runs.** `./run` ran in detached tmux at 80x24, 120x40 and 60x20, with live resizes to 100x30 and
  40x12. It also ran with `NO_COLOR=1`, with `TERM=dumb` (with and without the inherited `COLORTERM=truecolor`),
  and with `LANG=C LC_ALL=C`. Linux ran in Docker: Debian amd64 (emulated) and Alpine arm64. `open` and
  `xdg-open` were replaced by stubs that log the URL. In the rich vault, fake `kiwix-serve` and `llama-server`
  scripts let the chat flow get past binary resolution.
- **Placeholder artifacts** are kept apart from UX issues; see [the end of this document](#placeholder-artifacts-not-ux-issues).

## 1. How recipes become menu entries

```text
recipe YAML: type, description, menu{}      presets/packs + dep-defaults
        │                                            │
        ▼                                            ▼
host-cli apply ── realized entries, sorted by ID (apply.go:367-374)
        │         menus map: label ← menu.label ← description (apply.go:240-263)
        ▼
toolkit.writeActionsConfig: typeDefaults · builtinCapabilities · groupRegistry
        │                   (toolkit.go:218-477)
        ▼
.svalbard/actions.json  groups[{id,label,order,items[{label,subheader,
        │                aliases,order,action{type:"builtin",name,args}}]}]
        ▼
./run (bash) → .svalbard/runtime/<os-arch>/svalbard-drive
        ├─ no args  → menu (Bubble Tea, alt screen) ──── Enter ──┐
        └─ <alias>  → cfg.FindItemByAlias (main.go:88-94) ───────┤
                                                                 ▼
               actions.Runner re-execs itself as `__native-<id> <args>`
               → browse · apps · maps · chat · agent · share · serve-all ·
                 mcp-serve · activate-shell · embedded · inspect · verify
```

### Fields and how each is resolved

| Item field | Resolution order | Where |
|---|---|---|
| group | `menu.group` → type default | `toolkit.go:351,359-362` |
| subheader | `menu.subheader` → type default | `toolkit.go:352,363-365` |
| label | `menu.label` → **recipe `description`** → `humanize(id)` | `apply.go:256-262`, `toolkit.go:353` |
| description | `menu.description` → template per action ("Browse the <label> archive.", "<label>.") | `toolkit.go:383-397` |
| order | `menu.order` → `(index in ID-sorted realized entries + 1) × 100` | `toolkit.go:356`, `apply.go:367-374` |
| aliases | `menu.aliases`, or the built-in list | `toolkit.go:375`, `toolkit.go:270,286` |
| action, args | **type only**, never the recipe | `toolkit.go:235-242,378-405` |

`MenuSpec` (`host-cli/internal/catalog/catalog.go:96-104`) has `group`, `subheader`, `label`, `description`,
`aliases` and `order`. It has no field for the action. In practice only 6 of 218 recipes have a `menu:` block
(duckdb-wasm, sqliteviz, wikipedia-en-nopic, goose, opencode, pi), and only duckdb-wasm sets `aliases`. Every
other label is the catalog description, which was written for the pack picker. The longest is 167 characters
(wikipedia-en-medicine-compact). Recipes also carry `display_group` (65 practical, 60 tools, 49 reference,
11 education, …) and `depth`, but `catalog.Item` (`catalog.go:13-52`) has neither field, so both are dropped at
parse time and never reach the menu.

### Action kinds

| Recipe type | Default group / subheader | Builtin action, args | What it does |
|---|---|---|---|
| zim | library / Archives | `browse`, zim=filename | Starts kiwix-serve on 127.0.0.1, waits until healthy, opens the browser, blocks until Ctrl+C (`browse/browse.go:49-103`) |
| pmtiles | maps / Maps | `maps`, **no args** | Serves the drive and opens `apps/map/` (`maps/maps.go`). Every layer entry does the same thing |
| gguf | local-ai / Chat Models | `chat`, model=filename | Context picker, then llama-server; opens the browser right after `Start` (`chat/chat.go:34-50`) |
| app | apps / Apps | `apps`, app=id | Serves the drive and opens `apps/<id>/` (`apps/apps.go`) |
| sqlite | tools / Data | `apps`, app=sqliteviz | Always opens sqliteviz, not the database |
| binary in group `local-ai` | from the recipe | `agent`, client=id | Special case at `toolkit.go:378-381` |
| binary, any other group | tools / Tools | **empty name** | "unknown action:" (inventory) |
| pdf, epub, dataset, gguf-embed, gguf-mtp, python-*, toolchain, gpkg | none | no entry | Skipped at `toolkit.go:345-348`. toolchain only enables `embedded-shell` |

The generator adds these built-ins (`toolkit.go:266-294`): `search-all` (when the drive has ZIMs; alias
`search`), `serve-all` and `share-files` (ZIM, map or model present), `mcp-serve`, `activate-shell` and
`inspect-drive` (always), `embedded-shell` (toolchain; alias `embedded`) and `verify-checksums`. It also adds
`chat-default` whenever a gguf exists (`toolkit.go:418-428`).

The runtime has two modes. `inspect` and `verify` run captured: their output appears in an in-menu output view
(`actions.go:212-219`). Every other action is handed the terminal through `tea.ExecProcess`
(`model.go:499-501`) until it exits or the user presses Ctrl+C. Search runs inside the menu.

The runtime also has a complete second action type, `exec` (`config.go:44-51`, `actions.go:189-206,254-304`).
It takes an executable resolved from drive-bin or PATH, args with `{drive_root}` and `{platform}` placeholders,
env, cwd, and a capture, interactive or service mode. The generator never emits it, and a recipe can't ask for it.

### Groups and ordering

- `groupRegistry` (`toolkit.go:218-225`) fixes the label, description, order and auto-activate flag for
  search (100, auto-activate), library (200), maps (300), local-ai (400), apps (450) and tools (500). A new group
  ID that a recipe invents gets `humanize(id)`, no description and order 999 (`toolkit.go:450-453`).
- Within a group, items sort only by `order`. Subheaders are not sort keys: the view prints a header each time
  the subheader *changes* (`view.go:255-261`). Implicit orders are alphabetical-by-ID index × 100, so they
  interleave with explicit orders and built-in orders (300-700). The result is fragmented headers (screen S4).
- The right-hand detail pane for groups is hard-coded by group ID (`menu/context.go:20-33`). Library shows
  "Archives", maps "Layers", apps and tools "Tools". Search and local-ai show only the description.

**A recipe author controls:** group, subheader, label, description, aliases and order.
**Hard-coded:** the action and its args, which types appear at all, group labels, descriptions, order and
auto-activate, the built-in capabilities and their conditions, `chat-default`, the detail-pane fields and the key
bindings. The config types are also duplicated across the two Go modules (`toolkit.go:68-99` and
`drive-runtime/internal/config/config.go:9-51`).

## 2. Captured screens

All screens come from `tmux capture-pane -p`, with trailing blank rows trimmed unless the blank rows matter.
TERM is xterm-256color unless stated.

**S1 — first screen.** 80x24, rich multi-platform vault.
```text
Svalbard default-64

> ⌕ Search            Search
  Library             Search across indexed archives and documents.
  Maps
  Local AI
  Apps
  Tools

j/k: move | Enter: open | Esc: back | q: quit
```
The identity field shows the first preset name, not the vault, and is blank for vaults built with `svalbard add`.
The groups have no counts. Ctrl+K (palette) is not advertised. Rows 11-24 are empty.

**S2 — Library just after Enter.** 80x24, rich vault, 50 archives. The cursor is on item 1 ("100 Rabbits"),
which is off-screen. The title and subheader are cut off too.
```text
    Homebrewing Stack Exchange — brewing and fermentation
    Motor Vehicle Maintenance & Repair Stack Exchange
    Medical Sciences Stack Exchange
    Outdoors & Survival Stack Exchange
    Sustainability Stack Exchange Q&A
    USDA National Nutrient Database — food composition and nutrition data
    WikEM — concise emergency medicine wiki for clinical decision-making
    English Wikibooks textbooks
    Wikiciv — guide to recreating modern civilization technologies from raw mate
    WikiMed medical encyclopedia
    Wikipedia medical articles — diseases, drugs, anatomy, procedures with resiz
    English Wikipedia Top 50k — community-curated most important articles with f
    WikiVet — veterinary encyclopedia for livestock and pets
    Food preparation and preservation techniques
    Knot-tying reference — essential for shelter, rescue, and outdoor survival
    Practical medicine reference — supplements WikiMed with field-oriented conte
    Post-disaster response guides — shelter, water, sanitation, infrastructure r
    Water sourcing, purification, and safety reference

Selected
Browse the 100 Rabbits — off-grid computing, sailing, and resilient technology g

j/k: move | Enter: open | Esc: back | q: quit
```

**S3 — Local AI.** 80x24, rich **multi-platform** vault. Each agent appears once per platform, and "Chat Models"
is split in two.
```text
Svalbard / Local AI
Chat with local models and launch AI clients.

Chat Models
  > Chat
AI Clients
    OpenCode
    OpenCode
    Goose
    Goose
    pi
    pi
Chat Models
    Gemma 4 12B IT (QAT UD-Q4_K_XL) — general/tool-use model for 16 GB hosts; te
    Qwen3.5 9B (Unsloth UD-Q4_K_XL) — coding/tool-use model for 16 GB hosts; tex

Selected
Start chat with the default local model.

j/k: move | Enter: open | Esc: back | q: quit
```

**S4 — Tools.** 120x40, single-platform vault, 33 items. The top rows (title, 7-Zip with the cursor, Apps) are cut
off. "Tools" appears as a header four times and "Sharing" twice.
```text
    age — simple, modern file encryption tool (keygen + encrypt/decrypt)
Sharing
    Share on Local Network
    Serve MCP
Drive
    Inspect Drive
    Verify Checksums
Tools
    Lightweight web proxy with browser UI (Burp Suite alternative)
Development
    Activate sb Shell
    Embedded Dev Shell
Tools
    TCP/UDP tunnel over HTTP secured via SSH
    Dufs — HTTP file server with upload, resume, directory UI
    Fast recursive content discovery tool
    Fast web fuzzer (Fuzz Faster U Fool)
    fzf — fuzzy finder for interactive menu selection
    go-pmtiles — PMTiles CLI for serving and extracting map tiles
    Directory, DNS, and vhost brute-forcing tool
    Multi-purpose HTTP toolkit — probing, tech detection, fingerprinting
    Web crawling and spidering framework
    Kiwix tools archive for all supported desktop platforms
    ligolo-ng agent — deploy on target for network pivoting
    ligolo-ng proxy — advanced network pivoting via TUN interfaces
    Linux privilege escalation enumeration script (compiled binary)
    llama.cpp server binary for serving GGUF models (v0.5.0 / b11146 — portable CPU/Metal builds with native MTP support
    Fast port scanner (ProjectDiscovery)
    Template-based vulnerability scanner (pair with nuclei-templates dataset)
    Fast port scanner — scans all 65535 ports in seconds, feeds nmap
    SQLite CLI — required for cross-ZIM search
    Subdomain enumeration tool
    Fast Python package manager for building drive venvs
    Windows privilege escalation enumeration tool (x64 exe, deploy on targets)

Selected
7-Zip — universal archive tool (zip, 7z, tar, gz, xz, rar, iso).

j/k: move | Enter: open | Esc: back | q: quit
```

**S5 — Enter on a CLI tool.** 80x24, single vault. The selected row is not visible.
```text
    go-pmtiles — PMTiles CLI for serving and extracting map tiles
    Directory, DNS, and vhost brute-forcing tool
    ...
    Fast Python package manager for building drive venvs
    Windows privilege escalation enumeration tool (x64 exe, deploy on targets)

Selected
7-Zip — universal archive tool (zip, 7z, tar, gz, xz, rar, iso).

j/k: move | Enter: open | Esc: back | q: quit

Action failed: unknown action:
```

**S6 — Enter on CyberChef.** 80x24, rich vault.
```text
Svalbard / Apps
Interactive offline applications.

Apps
  > CyberChef web-based data analysis tool
    MapLibre GL JS + PMTiles JS + Protomaps fonts and sprites
    MML overview and regional map sheets as PNG images — viewable on any phone

Selected
Open CyberChef web-based data analysis tool.

j/k: move | Enter: open | Esc: back | q: quit

Action failed: exit status 1
```
The real reason only appears in the shell scrollback after quitting the menu:
`svalbard-drive: app not found: …/rich/apps/cyberchef`. A library (MapLibre) is listed as an app.

**S7 — top level after a failed action.** 80x24, rich vault, Search pressed. The status line is appended below
the full-height layout, which pushes the title row off-screen. The status stays until the next action.
```text
> ⌕ Search            Search
  Library             Search across indexed archives and documents.
  Maps
  Local AI
  Apps
  Tools

j/k: move | Enter: open | Esc: back | q: quit
(rows 9-22 blank)

Action failed: search index not found. run 'svalbard index' to build it
```

**S8 — Ctrl+K palette.** 80x24, rich vault, about 135 entries. The "Command Palette" title and the input line are
off-screen. The last of the 24 rows is the footer.
```text
  ligolo-ng agent — deploy on target for network pivoting
  ligolo-ng proxy — advanced network pivoting via TUN interfaces
  ligolo-ng proxy — advanced network pivoting via TUN interfaces
  Linux privilege escalation enumeration script (compiled binary)
  ...
  Fast Python package manager for building drive venvs
  Windows privilege escalation enumeration tool (x64 exe, deploy on targets)

Enter: select | Esc: close | ↑/↓: move
```
After typing `duck`, the palette shows `> DuckDB`. Enter opens the Tools group with **7-Zip** selected, not DuckDB.

**S9 — Search with 20 results.** 80x24, small real vault. The "⌕ Search [Keyword]" title and the status line are
off-screen, so the user never sees "Opened in browser" or "Open failed". Resizing to 40 rows reveals the
status line "Opened in browser".
```text
> pointer

Results (20 shown, max 20)
01  Pointer declaration
02  Predefined null pointer constant
...
12  strpbrk
  ↓ 8 more

Selected Result
  Source: devdocs_en_c_2026-10.zim
  Path:   /language/pointer
  Snippet: »Pointer« declaration »Pointer« declaration »Pointer« is a type of an

Enter search/open • Tab mode • j/k move • Esc clear/back
```

**S10 — typing `k i w i j a v a` one key at a time into the search box.** 80x24, small real vault.
```text
⌕ Search [Keyword]
2 sources • 1538 articles
> iwiava
```

**S11 — the same search under `NO_COLOR=1` after pressing j j.** 80x24, small vault. Result 03 is selected, but
nothing on the row shows it.
```text
> pointer

Results (20 shown, max 20)
01  Pointer declaration
02  Predefined null pointer constant
03  Member access operators
04  cast operator
```

**S12 — Local AI in a narrow terminal.** 60x20, single vault. Labels are cut at the terminal edge with no ellipsis.
```text
Svalbard / Local AI
Chat with local models and launch AI clients.

Chat Models
  > Chat
AI Clients
    OpenCode
    Goose
    pi
Chat Models
    Gemma 4 12B IT (QAT UD-Q4_K_XL) — general/tool-use model
    Qwen3.5 9B (Unsloth UD-Q4_K_XL) — coding/tool-use model

Selected
Start chat with the default local model.

j/k: move | Enter: open | Esc: back | q: quit
```

**S13 — browsing a real ZIM while it runs.** 80x24, small real vault. Kiwix says the archive is reachable "in the
local network", but it is bound to 127.0.0.1. Nothing says that Ctrl+C returns to the menu.
```text
The Kiwix server is running and can be accessed in the local network at:
  - http://127.0.0.1:8080
Kiwix: http://localhost:8080
```

**S14 — Local AI → Chat.** 80x24, rich vault with the fake llama-server. The context picker is drawn under stale
output from earlier actions.
```text
svalbard-drive: unknown command: goose
duckdb-wasm: http://localhost:8083/apps/duckdb-wasm/
^Csvalbard-drive: context canceled
svalbard-drive: reading platformio inventory: open /Users/…/rich/.svalbard/pla
tformio.json: no such file or directory
...
svalbard-drive: llama-server not found
Context window:
▸ Auto — 128K  (recommended)
  Fast — 8K  (snappier, less memory)
  Balanced — 32K
  Max — 128K  (may be slow / high memory)

↑/↓ select · enter confirm · esc keeps Auto
```
After Enter, it prints `Starting llama-server … (context 131072)...` and `LLM: http://localhost:8082`, and the
browser stub logged the URL before the server was listening.

**S15 — the macOS-only small vault, run on Linux.** 80x24, Debian amd64 container, `-v vault:/v:ro`.
Library → Enter.
```text
Svalbard / Library
Browse offline archives and documents.

Archives
  > DevDocs C language reference
    Permacomputing wiki — sustainable and resilient computing practices

Selected
Browse the DevDocs C language reference archive.

j/k: move | Enter: open | Esc: back | q: quit

Action failed: exit status 1
```
Search on the same mount: `Search failed: engine: keyword search: sqlite3: unable to open database file`.

## 3. UX issues, ranked by user impact

Placeholder-only failures are excluded; see the end of this document. "Inventory" means the issue is already
recorded in the pack inventory.

1. **Lists, the palette, the output view and search don't scroll, so the cursor goes off-screen.**
   - Nothing limits the menu to the terminal height: no viewport, no offset, no `MaxHeight`
     (`view.go:235-290`, `tui/palette_model.go:100-136`, `view.go:292-309`). Bubble Tea drops the top rows of a
     view that is too tall.
   - On default-64 the Library holds 50 archives. At 80x24 the first 32 never appear, and the cursor starts on a
     hidden row (S2).
   - Tools hides Inspect, Verify, Share and Shell even at 120x40 (S4). The palette hides its own input line (S8).
     Inspect output loses its header and the "Enter or Esc: back" hint.
   - Search budgets 12 rows of chrome (`model.go:437-444`) but uses more, so its title and status scroll away (S9).
2. **The search box swallows `j` and `k`** (S10). In `updateSearch`, `"up","k"` and `"down","j"` return before
   the text input sees the key (`model.go:289-314`). As a result, queries such as "knee", "joint", "kidney",
   "javascript" or "knots" can't be typed key by key; pasted text still works. Search is the first entry and opens
   straight away. Under `NO_COLOR` the selected result has no marker at all (S11; `view.go:158-170` styles it by
   background only). On read-only media search fails outright (S15, below).
3. **Failures are opaque, and the error status sticks.**
   - Child processes write their stderr to the normal screen. The alt screen then returns and covers it, so the
     menu only says "Action failed: exit status 1" (S6, S15). The real messages ("app not found", "kiwix-serve
     not found", "opencode not found") appear only after quitting.
   - The status line is never cleared on navigation (`model.go:137-154`). It is appended below a full-height
     layout (`view.go:37-45`, `tui/shell.go:102-104`), which pushes the title off-screen (S7).
   - Stopping an action with Ctrl+C reports three different things: "Action failed: exit status 1" for browse
     and chat, "signal: interrupt" for MCP, and "Action finished" for apps and serve-all.
   - Exiting the sb shell after any failed command reports "Action failed".
   - The search error tells a drive user to run `svalbard index`, a host command (`search/session.go:69`).
4. **Most of the Tools group is dead, duplicated or internal.**
   - 24 of the 33 Tools entries in the single-platform vault have an empty action (inventory).
   - **New:** on a multi-platform vault every platform binary gets its own menu entry, because the generator emits
     one item per realized entry with no deduplication by ID (`toolkit.go:328-416`). That doubles OpenCode, Goose,
     pi and every CLI tool (S3); Tools grows to 58 entries, 49 of them dead.
   - Runtime dependencies are listed as user entries: kiwix-serve, llama-server, uv, go-pmtiles, and maplibre-vendor
     under Apps (S6).
   - Items for platforms the host can't run (nmap-static on macOS) and items whose files are missing are listed like
     any other. `VisibleItems` doesn't check anything (`model.go:628-634`).
5. **Labels and order read as noise.**
   - Labels are catalog descriptions, up to 167 characters. At 80 columns 18 of 101 labels are cut mid-word with no
     ellipsis (S2, S12).
   - Descriptions become templates around those labels ("Browse the 100 Rabbits — off-grid … archive.") or repeat
     the label word for word (S5).
   - Order follows recipe IDs, not labels. "English Wikibooks", "Food preparation" and "Homebrewing Stack Exchange"
     look randomly placed.
   - Subheader runs fragment, for example tools: Tools, Apps, Sharing, Tools, Sharing, Drive, Tools, Development,
     Tools (S4).
   - Library is one flat "Archives" list, because the recipes' `display_group` and `tags` are dropped.
6. **The command-line path barely exists.**
   - `./run --help`, `-h` and `help` all print "unknown command" and exit 1.
   - Only three aliases exist: `search`, `duckdb` and `embedded` (`config.go:135-150` matches aliases only, never
     item IDs). `./run chat|maps|inspect|verify|share|opencode|pi|browse` are all unknown, yet the inventory cites
     `./run chat` and `./run maps`, and the activate-shell banner advertises `sb chat`, `sb opencode` and
     `sb goose` (`shell/shell.go:33`). All three fail inside that shell.
   - Arguments after an alias are dropped: `./run search pointer` opens the prompt and ignores "pointer"
     (`main.go:88-94`).
   - **Interactive aliases hang.** `./run search` prints its prompt, and its child then sits in state `T`
     (stopped by SIGTTIN), because `runActionProcess` puts the child in its own process group (`main.go:172`).
     Only Ctrl+C gets out. `./run embedded` takes the same path, yet the PIO checkpoint recorded it working (run
     under `env -i`/sandbox-exec). Re-test it on a real ESP32 vault.
7. **Keys and help.**
   - At the top level, Esc quits without asking, although the footer calls it "Esc: back". Inside a group `q` means
     back, while the footer says "q: quit" (`model.go:226-247`).
   - There is no `?` help, no `/` filter, no Home/End/PgUp/PgDn and no number keys, even though `tui.KeyMap`
     defines Filter, Palette and Help (`tui/keys.go:52-56`) and `NavList` supports number prefixes.
   - Ctrl+K is not in any footer. The palette only accepts arrows, and its results don't say which group they
     belong to.
   - Selecting an item in the palette opens its group at item 0 (`model.go:116-136`; S8).
   - There is no mouse support (`main.go:99` sets only `WithAltScreen`).
8. **Long-running actions give no guidance.**
   - Browse, apps, maps, chat and serve-all never say "Ctrl+C returns to the menu" (S13); only MCP prints a stop
     hint.
   - Chat opens the browser before llama-server listens (`chat/chat.go:45-49`). Browse waits for a health check
     (`browse/browse.go:79-88`).
   - The context picker draws over stale output (S14). On hosts with 40 GB or more, Auto and Max are both 128K.
   - "Chat" doesn't say which model it uses: the alphabetically first file, here Qwen, while Gemma is listed first.
   - Verify hashes the whole drive in capture mode with no status, so on a full 64 GB stick the menu looks frozen
     (`model.go:482-498`).
9. **Colour and terminal assumptions.**
   - The theme uses fixed 256-colour values tuned for dark backgrounds (`tui/style.go:33-52`). On a white
     background the top-level items (colour 252) reach a contrast of 1.5:1 and the selected item (230) 1.0:1, so
     only the `>` caret shows on light terminals such as the macOS Terminal default profile. The title (124)
     reaches 2.8:1 on black.
   - `NO_COLOR` works.
   - `TERM=dumb` still gets 256-colour output when `COLORTERM=truecolor` is inherited, and always gets the alt
     screen and cursor control (raw output: `ESC[?1049h ESC[2J …`). With stdout redirected nothing renders,
     because width stays 0 (`tui/shell.go:41-43`).
   - `⌕`, `—`, `█` and `▸` are printed even under `LANG=C`, and there is no ASCII fallback.
   - Resizing reflows correctly. At 40x12 the header and footer are clipped.
10. **Per-item granularity is missing where it matters.**
    - All map entries open the same viewer, because pmtiles has no args (`toolkit.go:237`).
    - The sqlite entry opens sqliteviz without the database (inventory).
    - Python tools (sqlmap and others), PDFs, gguf-embed and datasets are installed but invisible.
    - The group view uses one column even at 120 columns, and no pane shows item details such as size, source,
      licence, date or platforms, although `.svalbard/recipes.json` has most of them.
11. **Phones on the LAN get very little.**
    - "Serve Everything" from the menu binds 127.0.0.1 (`actions.go:67` default). Only the hidden
      `__native-serve-all 0.0.0.0` reaches the LAN.
    - "Share on Local Network" binds 0.0.0.0 but serves a raw directory index of the whole drive (`share/share.go:14,50-52`).
      That includes `manifest.yaml` and `.svalbard/actions.json` (both HTTP 200). There is no landing page,
      ZIMs are plain downloads, and kiwix, chat and maps are not exposed.

## 4. Gaps against the four goals

**Granular (per-item actions and details).**
- Every entry has exactly one action; there are no secondary actions such as open, serve on LAN, show path or
  verify this file.
- There is no item detail view; only a description line.
- Map layers can't be selected one by one, and you can't search a single ZIM.
- The categories the catalog already has (`display_group`, `tags`) are dropped at parse time.

**Powerful (cross-content search, multi-step actions, scripting).**
- Keyword and hybrid search across ZIMs works, and opening a result in Kiwix works (verified). The j/k bug,
  the read-only failure and ZIM-only coverage (PDFs and books are not indexed; see the inventory) limit it.
- The menu itself can't be searched except through the hidden palette.
- There are no chained actions, for example "answer from the vault with the local model"; MCP exists only as a
  separate server.
- Scripting is limited to three aliases: no `--help`, no listing, no argument passing, no JSON output, and
  interactive aliases hang. The real command line is the undocumented `__native-*` subcommands.

**Expandable (new recipe kinds without code changes).**
- A recipe can only relabel and reposition its entry. The action comes from the type, so adding a kind touches
  `typeDefaults`, `nativeActionSpecs`, the `main.go` switch and a new handler, in two modules with duplicated
  config types.
- The runtime already has an `exec` action type with placeholders and modes that could carry recipe-defined
  launchers, but `MenuSpec` can't express it and the generator never emits it.
- A new group gets order 999 with no description.

**Cross-platform (any terminal, Windows, phones over LAN).**
- On terminals, see issues 1 and 9: no scrolling, dark-background colours, no ASCII or dumb-terminal fallback,
  no mouse.
- Phones: see issue 11. A LAN-ready page set would build on `serve-all`, which already starts Kiwix, LLM and
  file servers on a chosen bind address.
- Windows: none. `supportedPlatforms` lists four Unix targets (`toolkit.go:23-28`). There is no `run.cmd` or
  `run.ps1`, and `GOOS=windows go build ./cmd/svalbard-drive` fails on `syscall.Setpgid` and `syscall.Kill`
  (`main.go:172-189`, `embedded/embedded.go:331-348`) and on `unix.Flock` (`embedded.go:420-434`).

### What `./run` does on each platform

`run` (`toolkit.go:530-552`) is bash. It maps `uname -s` and `uname -m` to a platform and execs
`.svalbard/runtime/<platform>/svalbard-drive`. All four runtimes are always copied, about 67 MB, whatever
`host_platforms` says (`toolkit.go:479-495`). Content binaries (kiwix, llama-server, agents) exist only for the
selected platforms.

| Host | Result | Evidence |
|---|---|---|
| macOS arm64 | Menu runs | This Mac |
| macOS x86_64 | Runs the macos-x86_64 binary | Read; binary built, not run |
| Linux x86_64 with bash | Menu runs. Library fails with "exit status 1" when the vault lacks Linux kiwix, and the menu doesn't say why (S15) | Debian amd64 container (emulated) |
| Linux arm64 without bash (Alpine, BusyBox, OpenWrt) | `env: can't execute 'bash'`, exit 127. `sh run` fails with `bad substitution`, then `//.svalbard/runtime/…: not found`. The static binary itself runs (`__native-inspect` worked) | Alpine arm64 container |
| Read-only mount (write-protected stick, ISO, NTFS on macOS) | Search fails with `unable to open database file`, and its header reads "0 sources, 0 articles". The same vault mounted read-write works. `binary.Resolve` also extracts into the vault on first use (inventory) | Debian, `:ro` vs `:rw` |
| armv7l, i686, riscv64 | `Unsupported architecture: <m>`, exit 1 | Read |
| FreeBSD, MSYS, Cygwin, Git Bash | `Unsupported OS: <s>`, exit 1 | Read |
| Windows cmd or PowerShell | No launcher at all | Read, plus the build failure above |
| Linux with a FAT32 stick automounted by udisks (`showexec`) | `run` probably isn't executable; `bash run` would still fail at `exec` | Unverified risk |

No platform gets a fallback such as "open `index.html` in a browser", and there is nothing for a host without a
binary.

## Placeholder artifacts (not UX issues)

These failures came from fake files or from shortcuts in the simulation, not from the menu:

- In the rich vault, "kiwix-serve not found", "llama-server not found" and "opencode not found": the archives
  were placeholders.
- "search index not found" in the rich vault: no index was built. A real apply builds one automatically.
- Embedded Dev Shell failed on a missing `.svalbard/platformio.json`: the helper passed no PlatformIO recipes.
- Maps printed only `Files: …` because `apps/map/index.html` was missing. `syncMapViewer` runs in apply, not in
  `toolkit.Generate`.
- App-bundle apps landed as `apps/<id>.app` files and toolchain archives at the vault root; a real apply puts
  them in directories.
- Four fi-maps layers (unsupported build families) and wikipedia-en-medicine-compact (`{date}` bug) would not be
  realized by a real apply (inventory), so a real Maps group would have 4 entries, not 8.
- Inspect showed 0 KB sizes, and verify passed 147 of 147 against placeholder checksums.

## Limits

- macOS x86_64 and real Linux hardware were not run. Linux ran only in Docker, with amd64 emulated.
- Mouse behaviour and light-background contrast were judged from code and computed contrast ratios, not checked
  in a light-theme terminal.
- No full-size vault was hashed, so the verify freeze time is not measured.
- During early runs, before the `open` stub was in place, the DuckDB action opened two localhost tabs in the
  host browser. Nothing else left the scratch vaults, which were deleted afterwards.
