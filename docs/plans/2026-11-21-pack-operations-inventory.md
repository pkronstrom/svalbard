# Pack operations inventory

Status: discovery complete 2026-11-21. Read-only; nothing was fixed, applied or downloaded.

Scope: MASTERPLAN Phase 6, following the [user paths](2026-09-30-vault-user-paths.md#pack-operations) rule that user paths, not recipe files or menu entries, define completion. Covers all 24 packs in `presets/packs/**` (the embedded copy is identical; `favorites.local.yaml` is gitignored and personal).

Method: read the pack YAML, each source recipe, `recipes/dep-defaults.yaml`, the apply acquisition switch (`host-cli/internal/apply/apply.go` `executeJob`), native builders (`host-cli/internal/builder/builder.go` `Dispatch`), menu generation (`host-cli/internal/toolkit/toolkit.go` `TypeDirs`, `typeDefaults`, `builtinCapabilities`, `writeActionsConfig`), the map viewer (`host-cli/internal/mapview/generate.go`) and the drive runtime handlers (`drive-runtime/internal/{actions,apps,maps,browse,chat,agent,shell,binary,serveall,share}`). Earlier live evidence comes from the Phase 4 and 5 checkpoints and the [catalog refresh results](2026-09-30-catalog-refresh-results.md). Another session was editing `host-cli/internal/builder/*` during this audit (notably `pythonvenv.go`), so builder findings describe the working tree at about 10:00.

## Status legend

- **verified**: this pack's own user path ran live and has recorded evidence (path given).
- **plausible**: the code path exists and reads correctly, but nobody has run it for this pack. Pure-ZIM packs use the download → Kiwix → search path that Phase 4 verified with WikiCiv. Only their own `url_pattern` resolution is unexercised (the refresh HEAD check covered 145 tool/app/model URLs, not ZIMs).
- **broken**: at least one selected item fails to install, or installs with no working way to use it. The note says what is missing.

Counts: verified 2, plausible 16, broken 6.

## How each content type reaches the user

| Type | Acquired by | Lands at | Menu action (runtime handler) | Needs |
|---|---|---|---|---|
| zim (download) | `url_pattern` → `resolver.Resolve` → download | `zim/` | Library → `browse` (kiwix-serve); `search-all`, `serve-all`, `share-files` | kiwix-serve (dep default) |
| zim (build) | steps `exec: wget`/`warc2zim`, `zim-compact`, `content-archive` | `zim/` | as above | wget/warc2zim on host PATH, else Docker `svalbard-tools:0.4.0[-browser]` |
| gguf | download | `models/` | Local AI → `chat` + `chat-default` | llama-server (dep default) |
| gguf-embed / gguf-mtp | download | `models/embed/`, `models/draft/` | none; used by semantic search and llama-server drafting | llama-server |
| binary in `local-ai` group | platform download | flat `bin/` (archive) | `agent` (client + llama-server) | resolved and extracted at runtime by `binary.Resolve` |
| binary (other) | platform download | flat `bin/` (archive, not extracted) | Tools item with **empty action** → "unknown action:" (`toolkit.go:238`, `actions.go:76-80`) | no launcher; not on any PATH |
| python-package | `svalbard-python` python-venv builder via Docker `uv` | `runtime/python/<plat>/`, wrappers in `bin/<plat>/` | none | only reachable by `source ./activate` |
| toolchain | PlatformIO archive download | `tools/platformio/...` | `embedded-shell` | verified in Phase 5 |
| app (url) | download, **no extraction** | `apps/<file>.zip` | Apps → `apps` (serves `apps/<id>/` in a browser) | the `apps/<id>/` directory, which is never created |
| app (app-bundle / steps) | download + extract | `apps/<id>/` | Apps → `apps` (browser directory or index.html) | an index.html for web apps; native apps have no launcher |
| sqlite (`reference-static`) | Go-native XML → SQLite | `data/` | Tools/Data → `apps` with app=sqliteviz | sqliteviz (tools-base); MCP structured query |
| pmtiles | native `vector-static`/`vector-service` (Docker ogr2ogr + tippecanoe); `osm-extract`/`raster-tms`/`mml-topo` **no handler** | `maps/` | Maps → `maps` (opens `apps/map/index.html`) | maplibre-vendor assets at the vault-root `vendor/` (see fi-maps) |
| dataset | download, no extraction | `data/` | none | — |
| pdf | download | `books/` | none; not search-indexed | share/serve-all file listing only |
| gpkg | download | **vault root** (no `TypeDirs` entry) | none | no consumer |

Always present: `mcp-serve`, `activate-shell` (adds only an `sb` wrapper to PATH, `shell.go:44`), `inspect-drive`, and `verify-checksums` when checksums exist. Legacy `recipes/actions/*.sh` are embedded in the host catalog but no Go code writes or calls them. `pio-setup.sh` is a shim around `run embedded`. Every operation is reachable from the Go runtime, and nothing depends on those scripts.

## Pack table

| Pack | Install (types/strategy) | Menu / capabilities | Relies on | First useful operation | Status |
|---|---|---|---|---|---|
| ai/harnesses | 4 binary (platform archives) | Local AI → OpenCode, Goose, pi (`agent`); llama-server gets a dead Tools item | a models pack for any GGUF | `./run` → pi | plausible: pinned binaries launched, and Pi/OpenCode loaded their generated configs; Goose was only version-checked (refresh results:37-39) |
| ai/models-8gb-ram | 2 gguf download | Chat, per-model chat | llama-server (auto dep); harnesses for agents | `./run chat` | plausible: large-model chat inference is not recorded in any checkpoint |
| ai/models-16gb-ram | 2 gguf | same | same | same | plausible |
| ai/models-24gb-ram | 2 gguf | same | same | same | plausible: the operator's notes say Qwen3.8 IQ4_XS completed harness tool tasks on 2026-09-30, but no repo doc records it |
| ai/models-64gb-ram | 3 gguf | same | same | same | plausible |
| ai/mtp | 1 gguf-mtp → `models/draft/` | none (drafter used automatically) | qwen-27b / qwen-27b-hq | chat with Qwen 27B | plausible: routing reviewed (refresh review:22); no gain on Metal |
| communications | 6 zim download | Library, Search, Serve, Share | kiwix-serve | browse / search | plausible |
| computing | 22 zim download | same | same | same | plausible |
| core | 2 zim download | same | same | same | **verified**: WikiCiv acquired, browsed and searched ([search checkpoint](2026-10-06-search-reliability-checkpoint.md)) |
| cybersec-ctf | 2 app (app-bundle; steps with `exec 7z`), 4 python-package | Apps → Ghidra, Hashcat (browser directory listing) | JDK 21+ on host (undisclosed in the UI); 7z on host PATH for the hashcat build; Docker for uv | `source ./activate; pwn version` | **broken**: native apps have no launcher, and the Python tools have no menu entry |
| cybersec-reference | 6 zim build (wget + warc2zim), 2 zim download | Library, Search | warc2zim: host or `-browser` image (not present locally; publication unrecorded) | browse HackTricks | plausible |
| cybersec-wordlists | 3 dataset download | none | — | use `data/rockyou.txt` by path | **broken**: the SecLists and nuclei-templates branch zips are never extracted and use floating `master`/`main` URLs with no checksum |
| cybersec | 16 binary, 1 python-package | 16 dead Tools items (empty action) | an extraction step and PATH entry that don't exist | `source ./activate; sqlmap` (Python only) | **broken**: no binary is runnable; nmap-static has no macOS build, so the item fails on macOS-only vaults |
| esp32-dev | 13 toolchain, 2 python-package, python-venv, uv | Embedded Dev Shell | self-contained | `./run embedded`, `pio run` | **verified**: [PIO checkpoint](2026-11-21-offline-platformio-checkpoint.md) (macOS arm64; no upload or monitor test) |
| engineering | 12 zim download | Library, Search | kiwix-serve | browse | plausible |
| favorites (local) | 3 zim download, 7 zim build (wget/warc2zim) | Library, Search | same as cybersec-reference | browse | plausible: personal and gitignored |
| fi-maps | 8 pmtiles build, 2 app-bundle, 1 gpkg, 1 binary, 1 zim | Maps (`maps`), Apps → Maplibre Vendor, Mml Map Sheets; dufs gets a dead Tools item | Docker ogr2ogr/tippecanoe; root-level `vendor/` | `./run maps` | **broken**: see notes |
| fi-reference | 22 zim download | Library, Search | kiwix-serve | browse | plausible |
| fi-survival | 23 zim, 1 pdf, 1 sqlite (`reference-static`) | Library, Search; Tools/Data → Fimea opens sqliteviz | sqliteviz from tools-base (not in this pack) | browse WikiMed | plausible: PDF is file-only; Fimea opens a blank sqliteviz without the data loaded |
| homesteading | 22 zim download, 1 `content-archive` (14 GB, network) | Library, Search | Docker zimwriterfs/zimcheck | browse | plausible |
| sciences | 11 zim download | Library, Search | kiwix-serve | browse | plausible |
| survival-medical | 6 zim download, 1 `zim-compact` | Library, Search | Docker zim-compact | browse | **broken**: wikipedia-en-medicine-compact cannot download (see notes) |
| tools-base | 9 binary, 3 app, gguf-embed, python-venv | Apps → Cyberchef, Duckdb Wasm, SQLiteViz; 9 dead Tools items (incl. kiwix-serve) | — | semantic search via nomic | **broken**: CyberChef and DuckDB have no working page, and the CLI tools can't run |

## Per-pack notes

**fi-maps** (`finland-32` extends it, so every Finland preset inherits these problems):
- Four map builders are missing. `osm-finland` and `osm-uusimaa` use `osm-extract`, `mml-maastokartta` uses `raster-tms`, and `mml-topo-uusimaa` uses `mml-topo`. `builder.Dispatch` handles none of these families, so `executeJob` fails each item with "unsupported build family" (`apply.go:453`) after the user has already chosen them. The planner and TUI do not disclose this before apply. `buildItem` (`apply.go:668`) is dead code: nothing calls it, and the `recipes/builders/<family>.py` scripts it expects don't exist. The [catalog audit](2026-09-30-catalog-update-audit.md) §5 records the pruned Protomaps dailies and a 22.8 GB MML source that needs a bbox API.
- Vendor assets end up in the wrong place. maplibre-vendor is an app-bundle, so its files land in `apps/maplibre-vendor/vendor/*`. `mapview.ensureVendorAssets` and the viewer's `../../vendor/` paths expect `<vault>/vendor/*`. Once any pmtiles item is realized, apply returns "generating map viewer: stat …" (`apply.go:279`). Tests pass only because they create the vault-root `vendor/` themselves (`apply_test.go:82`).
- The generated viewer shows a blank map even with assets in place. It declares every layer as a `vector` source with `layers: []` (`generate.go:58-63`). It has no basemap style and no raster support (needed for `mml-maastokartta`), and it ignores `viewer.style` (that field only reaches `recipes.json`).
- natural-earth (`type: gpkg`, also in `default-32`) has no `TypeDirs` entry, so the `.gpkg.zip` lands at the vault root and nothing uses it. go-pmtiles and dufs come in as pmtiles dependencies, but no runtime handler uses them (maps and share use Go `http.FileServer`).
- mml-map-sheets is plausible: the `apps` action serves its PNG directory listing. The vector-static and vector-service layers (pohjavesialueet, luonnonsuojelualueet, virtavesien-lohikalakannat, lipas) have builders, but none has been run live, and they need Docker.

**Binaries (tools-base, cybersec, also llama-server and dufs everywhere):**
- `fetchAndRecordAt` stores platform archives in flat `bin/` without extracting them. `typeDefaults["binary"]` has no `ActionID`, so each binary gets a menu item that fails with "unknown action:".
- The root `activate` script adds `bin/<plat>` to PATH, but only the Python wrappers live there. The `activate-shell` menu action adds only `sb`. As a result `age`, `7z`, `fzf`, `toybox`, `sqlite3`, `uv`, `nmap`, `ffuf` and the rest can't be run without unpacking them by hand.
- `binary.Resolve` handles the runtime-owned tools (kiwix-serve, llama-server, the agent clients). On first use it extracts **every** archive in `bin/` into `bin/` and returns the first file whose name matches.
- Unverified risk: on a multi-platform vault, archives for different platforms can overwrite each other or resolve to the wrong platform's binary. Resolution also writes to the vault, which fails on read-only media. All live evidence so far comes from single-platform macOS arm64 vaults.

**Apps:**
- cyberchef (`url:` zip) is never extracted, so `apps` reports "app not found: …/apps/cyberchef" (`apps.go:19`).
- duckdb-wasm downloads only `shell.js` and wasm/worker files. There is no HTML page to launch it, so the menu opens a bare file listing.
- ghidra and hashcat are native programs routed to the browser `apps` action. The generator never emits `exec` actions (runtime audit). Ghidra also needs a host JDK 21+.
- The hashcat `exec: 7z` step resolves `bin/<plat>/7z` and then the host PATH, then falls back to the Docker base image, which has no 7z. The vault's own 7z sits as an archive in flat `bin/`, so the build depends on a host 7z.
- sqliteviz is plausible: whether `dist.zip` puts `index.html` at the bundle root is unverified.

**Builders and Docker:**
- `runToolCommand` always uses Docker (`toolrunner.go:28`). That covers python-venv (uv), geodata, zim-compact and content-archive.
- `svalbard-tools:0.4.0` exists locally. `0.4.0-browser`, which warc2zim needs, does not. The refreshed Dockerfile pins are not published (refresh results:30). A fresh host without Docker cannot build any python-package, map layer or scraped ZIM.
- `zim-compact` passes `source_url` (which contains `{date}`) straight to `stepDownload` (`pipeline.go:108`). Only `downloadItem` calls `resolver.Resolve`, so the download requests a literal `…_{date}.zim` and fails.

**Python packages** (pwntools, angr, volatility3, binwalk, sqlmap): the builder writes their wrappers to `bin/<plat>/`, which only a sourced `./activate` puts on PATH. No menu item points to them. The python-venv path is verified only for the PIO closure.

**PDF and fimea:**
- The WHO PDF sits in `books/` with no action and no search coverage.
- fimea: the menu opens sqliteviz without loading `data/fimea.sqlite`, so the user has to pick the file by hand. MCP structured SQLite query is the better path but has only unit-test evidence.

## Unsupported or unverified paths (explicit)

- Unsupported: map families `osm-extract`, `raster-tms` and `mml-topo`. `foraging-habitats-fi` stays out of the pack (it has no URL; its Python builder is not wired). `zstd` is excluded from tools-base for the same reason.
- No launcher: every non-agent `binary`, all `python-package` entry points (sourced activate only), duckdb-wasm, ghidra, hashcat, datasets, PDFs and gpkg.
- Unverified live: model chat inference and Goose conversations; every Docker builder; warc2zim scrapes; multi-platform vaults; Linux and x86_64 runtime execution; physical media; ESP32 upload and monitor.
- No pack depends on a shell script in `recipes/actions/`.

## Proposed repair plan

1. *(Partly done 2026-11-21: the viewer now loads MapLibre from `apps/maplibre-vendor/vendor/`, where the recipe installs it, so apply no longer fails at "generating map viewer". Tests now use the real layout. The per-layer styles, the unsupported-family disclosure and the dead `buildItem` remain.)* **The maps journey works for supported layers, and unsupported ones are disclosed first (M; becomes an L if osm-extract is included).** Put maplibre-vendor assets where the viewer expects them, or point the viewer at the bundle. Generate per-layer styles from `viewer` metadata (vector fill, line and circle; raster sources), and stop apply from failing on the viewer. Have the planner flag families with no handler so they are marked unsupported before acquisition, then drop the dead `buildItem`. Implementing `osm-extract` (`pmtiles extract` from a pinned Protomaps build, plus style, glyphs and sprites) is a separate L.
   Done when: a disposable vault with one vector-static layer and mml-map-sheets applies cleanly, and `./run maps` shows that layer in a browser. Selecting fi-maps lists the 4 pending items as unsupported before any download.
   Files: `host-cli/internal/mapview/generate.go`, `host-cli/internal/apply/apply.go`, `recipes/tools/maplibre-vendor.yaml` (+ embedded copy), the planner/TUI preview, `presets/packs/fi-maps.yaml`.
2. **Bundled CLI tools run from the drive (M).** At apply time, extract platform archives into `bin/<plat>/`, or make `binary.Resolve` choose only the current platform's archive. Put `bin/<plat>` on PATH in `activate-shell`. Give `binary` menu items a real action, such as opening the activate shell with a usage hint, or stop emitting them.
   Done when: on macOS arm64, `./run` → Activate shell → `age --version`, `ffuf -V`, `7z`, `pwn version` and `sqlmap --version` all work with the network off. A vault with both macOS and Linux platforms resolves the correct llama-server. No "unknown action" items remain.
   Files: `host-cli/internal/apply/apply.go` (`fetchAndRecordAt`), `host-cli/internal/toolkit/toolkit.go`, `drive-runtime/internal/shell/shell.go`, `drive-runtime/internal/binary/runtimebinary.go`.
3. **Apps open correctly or are disclosed (M).** Extract `app` URL archives (CyberChef). Add an HTML entry page for duckdb-wasm or remove it. Emit `exec` menu actions for native apps (ghidra with a JDK check, hashcat through the activated PATH) or remove them from the Apps menu with a note. Build hashcat with the Go extractor or the vault's 7z instead of a host 7z. Make the sqlite item open the selected database.
   Done when: each Apps entry opens a working page or program, or is labelled as needing a host dependency.
   Files: `host-cli/internal/apply/apply.go`, `host-cli/internal/toolkit/toolkit.go`, `host-cli/internal/catalog` (menu schema), `recipes/tools/{cyberchef,duckdb-wasm,sqliteviz}.yaml`, `recipes/apps/{ghidra,hashcat}.yaml`.
4. *(Partly done 2026-11-21: SecLists 2026.2 and nuclei-templates v10.5.2 are pinned to tag archives with sha256 and explicit filenames. The nuclei hash matches upstream's published checksum, and a real apply verified it. Extraction, `{date}` in zim-compact and `gpkg` remain.)* **Archive and URL correctness (S; an M if dataset extraction needs per-type layout rules).** Resolve `{date}` in builder `source_url` (zim-compact). Extract dataset zips or label them as raw archives. Give `gpkg` a type directory and a consumer, or drop natural-earth from `default-32` and fi-maps. Pin SecLists and nuclei-templates to tagged releases with SHA-256.
   Done when: survival-medical and cybersec-wordlists apply, and their files are usable at the documented paths.
   Files: `host-cli/internal/builder/zimcompact.go` (coordinate with the builder session), `host-cli/internal/resolver`, `host-cli/internal/toolkit/toolkit.go` (`TypeDirs`), `recipes/datasets/*.yaml`, `recipes/maps/natural-earth.yaml`, the preset files.

## Limits

This inventory comes from reading code; it is not a live test. Findings marked broken come from code paths that are deterministic given the recipe (an unhandled family, a missing extraction, a mismatched path, an empty action ID). Each should get a failing test before its fix. No `apply`, download, Docker build or go test was run. All pre-existing uncommitted work is retained; this document is the only file created.
