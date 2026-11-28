# Svalbard picks: where each one goes

Placement for the picks made on 2026-11-26 from the catalog research page, sorted by how hard each one is to add. Three batches of easy items are in the working tree (not committed); see "Status" below.

## Status

### Done (batches 1–2): in `recipes/` and `presets/`, `make verify` passes

Kiwix and PDF items that work end to end with no code changes. A throwaway vault with ten of them installed and its search found them.

| Added | Recipes | Presets / packs |
|---|---|---|
| Last Codex, Be Ready UK, foss.cooking, Public Domain Recipes | `last-codex`, `beready-uk`, `foss-cooking`, `public-domain-recipes` | default-32; engineering, homesteading, survival-medical |
| Army automotive primers | `army-tm-9-8000`, `army-tm-9-2700` (archive.org, US public domain) | default-32; engineering, homesteading |
| suomi.fi + SPEK 72 tuntia guides | `suomi-fi-varautuminen`, `spek-72-tuntia` | finland-2, finland-32; fi-survival |
| Phone-sized encyclopedias | `wikipedia-en-top-mini`, `wikipedia-fi-top-mini` | default-2, finland-2; core, fi-reference |
| Simple English Wikipedia, Wikivoyage EN | `wikipedia-en-simple` (nopic, 1.0 GB), `wikivoyage-en` | default-64; core |
| PhET English, Zimi Almanac | `phet-en`, `zimi-almanac` | default-64; sciences |
| DevDocs bundle | 21 new `devdocs-*` (bash … markdown); DevDocs man pages skipped because `man-pages` (mankier) covers it | default-64 together with the 6 existing `devdocs-*`; computing |
| Finnish Wikiversity, Wikiquote | `wikiversity-fi`, `wikiquote-fi` | finland-64; fi-reference |
| MDN | `mdn` | default-128; computing |
| TED growing ×5 | `ted-gardening`, `ted-plants`, `ted-agriculture`, `ted-food`, `ted-weather` | default-256; homesteading |
| Learn AI Anywhere | `learn-ai-anywhere` | default-256; computing |
| Fixes | `ready-gov` → `ready.gov_en_all_{date}`; `usda-nutrition` relabelled as the home-canning guide (id kept); Gutenberg lcc-r/s/t and fi → `/zim/books/` | — |

Tier totals after these: default-2 1.4 GB, finland-2 1.6 GB, default-32 18.2 GB, finland-32 24.7 GB, default-64 40.1 GB, finland-64 50.1 GB, default-128 97.8 GB, finland-128 107.4 GB. The Gutenberg move made `lcc-r`/`lcc-s` smaller, which offsets most of what was added.

### Done (batch 3): apps, datasets, map layers, one builder fix

| Added | Recipes | Presets / packs | Checked |
|---|---|---|---|
| Browser making apps | `drawio` (v32.7.7 `draw.war`), `minipaint` (4.14.3), `jscad-web` (@jscad/web 2.6.13) | default-64; engineering | Installed and opened in a headless browser: JSCAD renders and exports STL; miniPaint loads; draw.io loads with no external requests |
| Cybersec datasets | `exploitdb` (GitLab tag 2026-11-25, 70 MB), `attack-enterprise` (ATT&CK v20.1 STIX) | cybersec-reference (pack only) | Installed |
| Water layers | `syke-uomaverkosto` (75 MB), `syke-tulvariskialueet`, `hsy-vesipostit` | fi-maps (so finland-32) | Built to PMTiles. **Not visible yet**: the stick's map viewer draws no vector layers (see below) |
| Qwen3.8 pin | `qwen-27b` re-pinned to unsloth's UD3.1 IQ4_XS (commit 4af51b35). The GGUF header shows the same chat template and the built-in MTP layer. `qwen-27b-hq` and `qwen-27b-mtp` stay on the old commit: their bytes are identical, and re-pinning would re-download 26 GB | — | Header compared |

Two supporting changes:
- **Builder fix:** `stepExtract` (`host-cli/internal/builder/pipeline.go`) now detects zip/gzip by content when the file name has no known extension. Every `app-bundle` with a `source_url` failed before, including the existing `sqliteviz` in tools-base, so every default and Finland preset from 32 up had a failing item. There's a new regression test in `scenario_test.go`.
- **App pages:** `host-cli/internal/toolkit/apppages/jscad-web/index.html` serves the editor as the directory index, because JSCAD resolves its examples against the page directory. `host-cli/internal/toolkit/apppages/drawio/js/PreConfig.js` starts draw.io in offline/stealth mode; without it, the editor loads Google's API client on start.

Found and filed or noted in Aven:
- **Map viewer draws nothing.** `apps/map/index.html` registers the PMTiles sources with `layers: []`, so no vector overlay renders, old or new. That is the planned "maps journey" task, now with a note.
- **Existing tasks I duplicated.** Traficom depth data and SYKE lake depth already had tasks. I cross-linked and noted them instead of leaving duplicates.
- **CVE List V5 needs another download strategy.** Its GitHub keeps only the 30 newest daily releases (about 1.5 days), so any pinned URL dies within days.
- **NIOSH Pocket Guide and ERG 2024** are personal recipes in the gitignored `local/recipes/` with Wayback URLs, not in the public catalog. That needed two changes: the CLI now loads personal recipes (`~/.config/svalbard/recipes/` and, in dev builds, `local/recipes/`), and the downloader sends a `svalbard` User-Agent, because the Wayback Machine answers Go's default one with HTTP 498. Both PDFs installed in a test vault.

### Sorted: what's left

```text
EASY — data exists, a recipe type exists, no code
  medical PDF canon, army welding TMs .. per-file URL hunting (user-fetched / PD only)
  World Factbook (frozen 2020 ZIM) ..... optional; CIA ended it 2026-02-04
  SYKE Ranta10 shorelines, lake depth .. same builder; 112-341 MB sources each

NEEDS PLANNING — one small design choice or one small code change
  CLI tools (ripgrep, jq, pandoc, …) ... bundled tools aren't runnable yet (Phase 3)
  llama.cpp → v1.1.0 + MTP defaults .... M; then Inkling 2 Mini, mmproj, Hy-MT3
  NC/ND licence flag ................... S, schema choice
  URL check + "upgrade available" ...... S + M
  folder import, tiers inside packs .... M each
  AI skills recipes (type: skills) ..... M
  address search (Ryhti FTS5) .......... M
  BRouter data, world low-zoom, hillshade  M (map builder checks)
  Meshtastic web, mesh firmware ........ M (static app / firmware layout)
  CVE List V5 .......................... S-M (no stable pinned artifact)
  three.js editor ...................... S (needs 3 strips of a ~300 MB repo tarball)

DIFFICULT — new subsystem first
  generic table builder (L) ..► FDC, openFDA, Fineli, place names, sci tables
  AppImage/dmg convention (L) ► KeePassXC, radio + SDR apps, Avogadro,
                                Stellarium, Cutter, CAD apps, OpenSCAD
  Java runtime (L) ...........► Photon, BRouter engine, Ghidra 12.2, jadx
  raster/WMTS harvest + WFS dump (L) ► Traficom charts, orthophoto, flood zones
  EmbeddingGemma 2 swap (L), GLM-OCR (L), Qwen3.8-Flash-Next (split GGUF)
```

## How to read the placement tables

Each pick gets a recipe file, one or more packs, and the **lowest preset tier** that should include it. Default presets from 32 up inherit upward, so an item placed in `default-32` is in every larger default and Finland preset too. Each Finland preset extends the default of the same size plus the next smaller Finland preset. The 2 GB presets are separate roots: `default-32` doesn't extend `default-2`, and `finland-32` doesn't extend `finland-2`. `fi-maps` is pulled in whole by `finland-32`.

```text
default-2     default-32 ─► default-64 ─► default-128 ─► default-256 ─► default-512 ─► 1tb ─► 2tb
    │          (+tools-base)                            (+ai 8/16 GB)   (+ai 24 GB)  (+ai 64 GB)
    ▼               ▼             ▼             ▼              ▼              ▼          ▼      ▼
finland-2     finland-32 ─► finland-64 ─► finland-128 ─► finland-256 ─► finland-512 ─► 1tb ─► 2tb
              (+fi-maps)
```

"Pack only" means the recipe exists and is selectable in the wizard through its pack, but no preset ships it by default.

### Current size of each tier (sum of recipe `size_gb`)

| Tier | default | finland | Room left before the picks |
|---|---|---|---|
| 2 | 1.2 GB | 1.2 GB | ~0.7 GB |
| 32 | 17.7 GB | 24.2 GB | ~6 GB on finland-32 |
| 64 | 39.6 GB | 49.8 GB | ~11 GB on finland-64 |
| 128 | 96.8 GB | 106.5 GB | ~15 GB on finland-128 |
| 256 | 167.5 GB | 178.7 GB | ~64 GB |
| 512 | 362.1 GB | 376.5 GB | ~110 GB |
| 1tb | 694.4 GB | 708.8 GB | ~260 GB |
| 2tb | 1017.7 GB | 1032.1 GB | ~900 GB |

"Room" keeps a 5% margin below the nominal drive size. finland-32, finland-64 and finland-128 are the tight ones, so the placement keeps their additions small and moves larger items up a tier.

## Wants: content, datasets and maps

| Pick | Recipe file(s) | Pack(s) | First tier | Notes |
|---|---|---|---|---|
| The Last Codex | `content/last-codex.yaml` | core, engineering, homesteading | default-32 | 264 MB, CC BY-NC: needs the NC flag. Sits next to `wikiciv`. |
| suomi.fi + SPEK 72 tuntia guides | `content/fi-prep-guides.yaml` (type pdf, several files) | fi-survival | **finland-2** | ~4 MB; the most useful Finnish item for its size. No stated licence, so mark it user-fetched. |
| Be Ready UK | `content/beready-uk.yaml` | survival-medical | default-32 | 158 MB. |
| HSY public water posts | `maps/hsy-water-posts.yaml` | fi-maps, fi-survival | finland-32 | 5 KB. |
| NIOSH Pocket Guide + ERG 2024 | `content/niosh-pocket-guide.yaml`, `content/erg-2024.yaml` (pdf) | survival-medical | **default-2** | 15 MB together; check both URLs by hand. |
| wikipedia_en_top_mini | `content/wikipedia-en-top-mini.yaml` | core | **default-2** | 283 MB. default-2 currently has no general encyclopedia. |
| wikipedia_fi_top_mini | `content/wikipedia-fi-top-mini.yaml` | fi-reference | **finland-2** | ~112 MB. |
| wikiversity_fi, wikiquote_fi | `content/wikiversity-fi.yaml`, `content/wikiquote-fi.yaml` | fi-reference | finland-64 | 39 MB together. |
| Army automotive primers | `content/army-tm-9-8000.yaml`, `content/army-tm-9-2700.yaml` (pdf) | engineering, homesteading | default-32 | 48 MB, public domain. Tag `vehicles` so they shelve with imported manuals. |
| Medical PDF canon | `content/medical-pdf-canon.yaml` (pdf bundle) | survival-medical | default-32 | ~1 GB, user-fetched. Highest-value medical add; it takes most of finland-32's headroom. |
| foss.cooking + Public Domain Recipes | `content/foss-cooking.yaml`, `content/public-domain-recipes.yaml` | homesteading | default-32 | 45 MB. |
| Simple English Wikipedia + Wikivoyage EN | `content/wikipedia-en-simple.yaml` (nopic), `content/wikivoyage-en.yaml` (nopic) | core | default-64 | ~0.8 GB. Held back from default-32 to keep finland-32 under budget; it already has `wikipedia-en-top-nopic`. |
| PhET English | `content/phet-en.yaml` | sciences | default-64 | 110 MB. `phet-fi` is already in finland-32. |
| DevDocs bundle (~20) | `content/devdocs-<name>.yaml` ×20, generated from one list | computing | default-64 | < 60 MB. The existing `devdocs-*` recipes are pack only today; move them to default-64 at the same time (see note A). |
| Zimi Almanac mini | `content/zimi-almanac.yaml` | sciences | default-64 | 11 MB. |
| Army welding and equipment TMs | `content/army-welding-tms.yaml` (pdf bundle, PD items only) | engineering | default-64 | Small. Cherry-pick public-domain TMs; the archive.org pack also holds commercial manuals. |
| USDA FoodData Central | `datasets/usda-fdc.yaml` (table builder) | homesteading | default-64 | ~0.3–0.5 GB as SQLite. Blocked on the generic table builder. |
| Fineli | `datasets/fineli.yaml` (table builder) | fi-reference, fi-survival | finland-32 | Small. URL unconfirmed (403 during research); verify by hand. |
| MML place names | `datasets/mml-placenames.yaml` (table builder) | fi-maps | finland-32 | 53 MB raw. Search from the map viewer is a separate L. |
| SYKE water layers | `maps/syke-ranta10.yaml`, `maps/syke-lake-depth.yaml`, `maps/syke-river-network.yaml`, `maps/syke-flood-risk.yaml` | fi-maps | finland-32, lake depth finland-128 | Raw 112–341 MB each. If the built PMTiles stay under ~300 MB total, all go in finland-32; otherwise lake depth moves up. |
| BRouter rd5 for Finland | `maps/brouter-segments-fi.yaml` | fi-maps | finland-64 | 130–180 MB. Data only (for BRouter/OsmAnd on a phone) until the Java runtime exists. |
| World low-zoom basemap | `maps/world-lowzoom.yaml` | maps (core) | default-64 | ~1 GB, z0–8. |
| MDN web docs | `content/mdn.yaml` | computing | default-128 | 560 MB. |
| openFDA drug labels | `datasets/openfda-labels.yaml` (table builder) | survival-medical | default-128 | ~1.5 GB as SQLite. Blocked on the builder; yearly refresh. |
| TED growing (5 topics) | `content/ted-gardening.yaml` and 4 more | homesteading | default-256 | Up to ~5 GB for all five, next to `ted-farming`. |
| HYG, ChEBI, CODATA, periodic table | `datasets/<name>.yaml` ×4 (table builder) | sciences | default-256 | < 50 MB. Blocked on the builder. |
| Learn AI Anywhere | `content/learn-ai-anywhere.yaml` | ai/harnesses | default-256 | 96 MB. CC BY-NC. Rides with the first local AI tier. |
| Sheldon Brown | `content/sheldon-brown.local.yaml` | favorites (local) | none | No licence, so it is a local build like FarOutRide and never ships in a preset. |
| Owner's manuals hint | no recipe: wizard hint + docs | — | — | Points users at `svalbard import` (and folder import once it exists). |

## Wants: tools and apps

Bundled CLI tools aren't on the drive shell's PATH today. They can only be reached through runtime actions, and making them runnable is the planned "bundled tools runnable; recipes declare actions" phase. Adding these recipes before that phase would only add dead menu rows, so they wait for it. Existing backlog tasks under the "Everyday CLI tools" epic already cover jq, pandoc, ripgrep, restic, rclone, rtl_433, syncthing and micro. pdfcpu and restic also need the runtime to unpack `.tar.xz` and bare `.bz2` (filed).

| Pick | Recipe file(s) | Pack(s) | First tier | Notes |
|---|---|---|---|---|
| ripgrep, jq, pdfcpu, micro, Numbat, restic, zim-tools | `tools/<id>.yaml` | tools-base | default-32 | < 40 MB together. |
| pandoc, Syncthing, LocalSend CLI | `tools/<id>.yaml` | tools-base | default-32 | ~60 MB. |
| Neovim | `tools/neovim.yaml` | computing | default-64 | micro is the default editor; Neovim for people who know it. |
| TiddlyWiki | `tools/tiddlywiki.yaml` (single HTML) | tools-base | default-32 | 2.5 MB. Served like CyberChef. |
| SilverBullet | `tools/silverbullet.yaml` | tools-base | default-64 | ~20 MB, a notes server. |
| KeePassXC | `apps/keepassxc.yaml` | tools-base | default-32 | Blocked on the AppImage/dmg convention. |
| Meshtastic web client | `tools/meshtastic-web.yaml` (static web app) | communications | default-64 | Small. |
| rtl_433, Pat (Winlink), Direwolf | `tools/<id>.yaml` | communications | default-128 | Direwolf has no prebuilt binaries: it needs a source-build handler. |
| Meshtastic + MeshCore firmware | `datasets/meshtastic-firmware.yaml`, `datasets/meshcore-firmware.yaml` | communications, new `embedded/lora-mesh` | default-128 | Small. |
| JS8Call, FreeDV, Reticulum MeshChat, gqrx | `apps/<id>.yaml` | communications | default-256 | Blocked on the AppImage/dmg convention. |
| Avogadro 2, Stellarium | `apps/<id>.yaml` | sciences | default-256 | Blocked on the convention. Stellarium has no Linux arm64 build. |
| yara-x, gitleaks | `tools/<id>.yaml` | cybersec | pack only | Cybersec packs are not in any preset today; keep it that way. |
| CVE List V5, Exploit-DB, ATT&CK STIX | `datasets/<id>.yaml` | cybersec-reference | pack only | Exploit-DB from GitLab (the GitHub mirror is archived). |
| Cutter | `apps/cutter.yaml` | cybersec-ctf | pack only | See the reverse-engineering section below. |

## Wants: local AI

| Pick | Recipe file(s) | Pack(s) | First tier | Notes |
|---|---|---|---|---|
| EmbeddingGemma 2 | `models/embeddinggemma-2.yaml` | tools-base (replaces `nomic-embed-text-v1.5`) | default-32 | 0.31 GB. Changes the vector space, so every semantic index must be rebuilt; swap only after a Finnish retrieval check. Note the embedder picks the first `.gguf` in `models/embed/`, so the swap must remove nomic, not add beside it. |
| Inkling 2 Mini | `models/inkling-2-mini.yaml` | ai/models-24gb-ram | default-512 | 15.7 GB. Already tracked as an evaluation task; needs the llama.cpp bump. |
| GLM-OCR | — | — | — | Filed as a backlog task under the search epic. |
| Qwen3.8-Flash-Next | `models/qwen-flash-next.yaml` (+ optional `qwen-flash-next-mtp.yaml`) | new `ai/flash-next` | default-1tb | 70.5 GB, UD-Q2_0. Needs split-GGUF downloads and a template-match fix first. See note G. |


## Fixes and infrastructure

| Pick | Size | What it unblocks | Notes |
|---|---|---|---|
| Relabel `usda-nutrition` | S | — | Description and display name only; keep the id. |
| Fix `ready-gov` pattern | S | — | `ready.gov_en_all_{date}`. |
| Gutenberg to `/zim/books/` | S | — | URL and size edit for lcc-r/s/t and fi. |
| Qwen3.8-27B pin drift | S | — | Re-pin `qwen-27b-mtp` to the commit the targets use. |
| llama-server to latest + MTP defaults | M (L if MTP defaults change the launcher) | Inkling 2 Mini, GLM-OCR, vision, Qwen3.8-Flash-Next | See the MTP section below. |
| NC/ND licence flag | S | Last Codex, Learn AI Anywhere, TED | Do it before adding the first NC item. |
| URL check + "upgrade available" | M | — | See note B. |
| Generic table builder | L | FDC, openFDA, Fineli, place names, science tables | See note C, with an example. |
| AppImage/dmg launch convention | L, then S per app | KeePassXC, radio apps, Avogadro, Stellarium, Cutter, CAD apps | |
| Java runtime | L | Photon, BRouter engine, Ghidra without a host JDK | Ghidra already asks the host for JDK 21; a shipped JRE fixes that too. |
| pdf.js viewer | M | Every PDF pick above | See note D. |
| Folder import with shelf/tag | M | Owner's manuals, small-engine manuals | |
| Tiers inside packs | M | — | Makes "pack only" vs "in preset" a property of the pack entry instead of a hand-kept list. |

Suggested order: the four S fixes, then the NC flag, then the llama bump, then the table builder and pdf.js in parallel, then the AppImage convention and the Java runtime.

```text
 NC flag (S) ─────────────► Last Codex, Learn AI Anywhere, TED
 table builder (L) ───────► FDC, openFDA, Fineli, place names, science tables
 pdf.js viewer (M) ───────► NIOSH/ERG, medical canon, army TMs, imported manuals
 llama bump (M) ──────────► Inkling 2 Mini, Flash-Next, GLM-OCR, vision projectors
 AppImage/dmg (L) ────────► KeePassXC, radio GUIs, Avogadro, Stellarium, Cutter, CAD
 Java runtime (L) ────────► Photon, BRouter engine, Ghidra on hosts without a JDK
 folder import (M) ───────► owner's and small-engine manuals
```

## Maybes

| Pick | Suggestion |
|---|---|
| TED water + natural disaster | Promote to Want, default-256 (1.5 GB). Same value as the TED growing picks. |
| Gutenberg LCC U + V | default-256 next to `gutenberg-en-lcc-t` (1.6 GB). |
| Hy-MT3 translator | Promote and make it the translation answer instead of Argos Translate: ai pack, default-256 (where llama-server first ships), after the llama bump. Tiers below 256 then have no machine translation; the Finnish–English dictionaries cover them. |
| Kotus Swedish dictionary + word list | fi-reference, finland-64, once the table builder exists. |
| Vision projectors (mmproj) | Promote: do it with the llama bump, since GLM-OCR and Inkling need mtmd anyway. |
| Size-delta guard | Fold into the vault-update epic. |
| WFS-dump builder | Promote if Traficom data is WFS-only (see below). |
| wikipedia_en_top1m_mini | core, pack only (4.9 GB). |
| Gutenberg Top 2500 | core, pack only ("optional", as you said). |
| One More Library | core, pack only. |
| DuckDB CLI, monolith, rclone, Typst | computing, pack only. (zim-tools was also in the tier-1 bundle, so it is placed in tools-base above.) |
| TED medicine/health | default-512 (7.8 GB). |
| Canadian Prepper extra | default-512 (5.5 GB). |
| wikipedia_en_knots_maxi | Skip: `zimgit-knots` covers it. |
| armypubs ZIM | default-1tb (7.7 GiB). |
| mdwiki full | default-2tb, as you suggested (10.75 GB). |
| Symptom → remedy tables | survival-medical, pack only, after the table builder. |
| Stack Exchange pets, lifehacks, coffee, vegetarianism | default-128 (small). |
| Plant databases | homesteading, pack only, after the table builder. |
| Köppen-Geiger + WorldClim | homesteading, pack only. |
| TED energy/engineering | default-512. |
| Wikipedia subject subsets | sciences, pack only. |
| Stack Exchange astronomy, space, stats | default-512. |
| Gutenberg LCC Q | default-1tb (12 GB). |
| Algorithms, Open Data Structures, Go by Example | computing, pack only. |
| freeCodeCamp + postmarketOS | computing, pack only. |
| lua.org | Drop: `devdocs-lua` in the bundle covers the reference. |
| Stack Exchange computing set | default-2tb, next to Stack Overflow. |
| Corey Schafer | default-2tb, as you suggested. |
| Nebula | communications, pack only. |
| TED cyber + Tor SE etc., Cloudflare Learning | cybersec-reference, pack only. |
| Qwen3-Reranker | ai pack only, after EmbeddingGemma 2 lands. |
| Muse Glimmer, Mellum | Extra recipes in a new `ai/extras` pack: not in the RAM-tier packs, so presets don't pick them. |
| GeoNames | maps, default-256 (a world gazetteer to match the world basemap). |
| Finlex | fi-reference, pack only ("optional"). |
| wikipedia_fi_all_mini | finland-128 (1 GB). |
| MML municipal boundaries | fi-maps, finland-32 (24 MB). |
| SYKE flood hazard zones | fi-maps, after the WFS-dump builder. |
| Briggs & Stratton / Honda engine manuals | Same treatment as owner's manuals: docs + import. |


### Budget after placement

Roughly, with everything above: finland-2 ≈ 1.6 GB, finland-32 ≈ 27.5 GB (limit ~30.4), finland-64 ≈ 56 GB (limit ~60.8), finland-128 ≈ 116 GB (limit ~121.6). The map items still waiting on research (Photon, Traficom, Mapterhorn, orthophoto) would all land in finland-64 or above, and that tier has only about 4–5 GB left, so any of them over ~1 GB should start at finland-128 or finland-256.

## Notes on your comments

### A. Programming docs aren't in any preset today

`devdocs-python`, `-rust`, `-go`, `-javascript`, `-c` and `-cpp` exist but sit only in the `computing` and `engineering` packs, so no default preset ships them (an open backlog task already covers checking programming coverage across tiers). The whole DevDocs set, old and new, is under ~150 MB, so I'd put all of it in default-64 when the bundle lands.

### B. URL check and "upgrade available"

There are two different questions here, and Svalbard answers neither today:

1. **Is the catalog stale?** A scheduled CI job resolves every recipe and asks upstream two things. For `url_pattern` recipes: does the newest match still resolve (HEAD 200)? If not, the check fails, which would have caught `ready-gov` and the Gutenberg move. For pinned recipes (GitHub release assets, Hugging Face commits, llama-server's `b11146`, dated Kiwix pins like WikEM): is there a newer release? If yes, it reports "upgrade available: llama-server b11146 → b12xxx" as a warning, not a failure. The output is one report listing broken URLs and recipes with newer upstreams.
2. **Is my vault stale?** `svalbard update` detects changes through the recipe's acquisition hash, which includes `url_pattern` itself but not the date it resolved to. So when Kiwix publishes a newer monthly ZIM, the recipe hasn't changed and the vault never shows an update. The new vault-update plan says `update <id>` re-fetches such items, but only if you ask. The fix is to record the resolved date at install time and have `status` compare it with the newest upstream match when online. That shows "newer snapshot available (2026-08 → 2026-11)".

Part 1 is S. Part 2 is M: it touches the manifest and status, and belongs under the vault-update epic.

### C. Generic table builder, with an example

Today the `reference-static` builder can build exactly one dataset. `host-cli/internal/builder/reference_static.go` has Go structs for Fimea's XML (`Laakevalmiste`, `Pakkaus`, `Laakeaine`), a hand-written `CREATE TABLE medicines`, and a hand-written FTS index. The recipe's `tables:` block (`name: medicines`, `fts_columns: [...]`) looks generic, but the code ignores everything except `source_format: xml` and always writes the Fimea schema. So each new dataset (FDC, openFDA, Fineli, place names, HYG stars) would mean another ~150 lines of Go.

The generic builder moves the mapping into the recipe. It reads CSV, JSON/JSONL, or XML (from a file or a zip member), maps fields to columns, and adds FTS on the columns you name. For example, USDA FoodData Central would become pure YAML (file and field names as FDC publishes them; the URL is illustrative):

```yaml
id: usda-fdc
type: sqlite
build:
  family: reference-table
  source_url: https://fdc.nal.usda.gov/fdc-datasets/FoodData_Central_foundation_food_csv_<date>.zip
  tables:
    - name: foods
      from: { zip_member: "*/food.csv", format: csv }
      columns:
        fdc_id: { field: fdc_id, type: integer, key: true }
        name: { field: description }
        category: { field: food_category_id, type: integer }
      fts: [name]
    - name: nutrients
      from: { zip_member: "*/food_nutrient.csv", format: csv }
      columns:
        fdc_id: { field: fdc_id, type: integer, index: true }
        nutrient_id: { field: nutrient_id, type: integer }
        amount: { field: amount, type: real }
viewer:
  name: Food nutrients (USDA)
  search: true
```

On the stick, the existing search viewer finds "oats" in `foods_fts`, and the row shows its nutrients. openFDA is the same pattern with `format: json` and `records: results[]` (nested fields like `openfda.brand_name[0]`). HSY water posts would be `format: geojson`, with the lon/lat kept as columns. Fimea moves onto it as the regression test: same output schema, no Fimea-specific Go. The size is L because it changes what recipes may declare: once a recipe uses this schema it has to stay supported.

### G. llama.cpp bump, MTP defaults, Qwen3.8-Flash-Next

**Bump.** The latest release is v1.1.0 (2026-11-26, the same commit as b12774), and the newest tag is b12777. Release assets keep the names our recipe already uses. Between our b11146 and now, no speculative-decoding flag we pass was removed or renamed, and the default port change doesn't matter because the runtime always passes `--port`. Bump to v1.1.0 (b12774).

**Why September's MTP result may no longer hold.** Two Metal fixes, #29869 (v0.6.0) and #30065 (v0.7.0), fixed draft verification on M1–M4: before them, verification got slower with every drafted token. That fits our September null result. Published results after the fix: Qwen3.8-27B on M2 Ultra 21 → 30 tok/s, and DFlash2 2–3.4× on M3 Ultra. MoE models still lack the fix; #30047 is open.

**Which models have an MTP head:**

| Model | MTP |
|---|---|
| Qwen3.8-27B (our IQ4_XS, Q6_K_XL) | Built into the GGUF |
| Qwen3.5 0.8B, 4B | Built in |
| Qwen3.5 9B, Qwen3.6-35B-A3B | Our files lack it; unsloth publishes `-MTP-GGUF` variants. Re-pin those to get it. |
| Gemma 4 (all sizes) | Separate small `mtp-*.gguf` drafters |
| Inkling 2 Mini | DSpark drafter (llama.cpp ≥ b12600) |
| Muse Glimmer | DFlash drafter |
| Mellum2.1 | MTP drafter; measured only 1.00–1.05× on M2 Max |

**Default policy I'd ship.** Make MTP a setup default chosen per host, not an on/off for everyone:
- **CUDA + dense model with a built-in head:** on.
- **Metal + dense model:** expected on. MTP already gives a good speedup through MLX on your Mac, and the Metal fixes remove the reason GGUF didn't. Confirm with one re-measurement on the M4 Max at b12774, using MLX on the same prompts as the reference, before flipping the default.
- **Metal + MoE:** off until #30047 lands.
- **Separate drafter files:** they cost download and RAM, so they stay opt-in add-ons like `ai/mtp` today. The wizard can recommend one when the host is CUDA.

The existing backlog tasks for re-measuring MTP/DFlash2 and for reading the drafter pairing at runtime already cover the code side. The re-measure list: Qwen3.8-27B with the built-in head and with the DFlash2 drafter, Gemma 4 12B/31B with their drafters, and 35B-A3B MTP. Size: the bump is M; per-host MTP defaults add another M (the launcher needs to know the host's backend).

**Qwen3.8-Flash-Next for the largest tiers.**
- **Model:** Qwen's preview of the Qwen4 architecture (2026-08-26). 125B parameters, 6B active, a 51B n-gram table that lives on disk and is read on demand, a 4B MTP layer, 262k context, vision.
- **Recommended file:** unsloth UD-Q2_0. 70.5 GB download, about 41.7 GB resident in memory, so it runs on a 64 GB host.
- **Licence:** `qwen-community-1.0`, not Apache. Personal use on a stick is fine; the restrictions hit hosted-model services and big products. It needs the licence flag to say "custom licence".
- **Placement:** a new pack `ai/flash-next` (it doesn't fit the 61 GB target of `ai/models-64gb-ram`) in default-1tb and up, both of which have room (≈260 GB and ≈900 GB). The ggml-org MTP file (2.2 GB) is an opt-in add-on.

Before it can ship:
- Recipes must be able to download split GGUFs. They can't today.
- The runtime's `ExtraFlags` matches any file name containing "qwen3.8", so it would give Flash-Next the 27B's fixed chat template. That match needs to be narrowed.
- Speed with the n-gram table read from a USB stick is unmeasured. On a slow stick it could be poor, so measure on the target drive before making it the 1tb default.

Notes: `research_notes/Svalbard picks follow up/llama_mtp_models.md`.

### E. Reverse-engineering tools (instead of just Cutter)

Ghidra is already in the catalog (`apps/ghidra.yaml`, 12.1.4, cybersec-ctf). Ghidra 12.2 came out on 2026-10-22 and needs JDK 25. Its extensions, such as GhidraMCP 2.0 (which lets an agent drive Ghidra), are tied to the Ghidra version, so bump Ghidra, the JDK and extensions together.

Proposed `re-tools` pack, pack only like the other cybersec packs:

| Set | Tools | Why |
|---|---|---|
| Essential | Temurin JDK, Ghidra + GhidraMCP, pwndbg (bundles gdb + Python), ImHex (hex editor with pattern language), Detect It Easy (identify packers and compilers), jadx + apktool (Android), frida-tools (dynamic instrumentation), yara-x, hexyl | Covers static, dynamic, Android and triage. About 1–1.3 GB per platform. |
| Extended | Cutter, rizin, radare2 + r2ghidra, LIEF, upx, ILSpy | Cutter is the JDK-free GUI. ILSpy needs .NET 10 on the host. |
| Left out | Binary Ninja Free (forbids redistribution), IDA Free (account-gated download, cloud-only decompiler) | Licence. |

The shipped JDK is the same item as the Java runtime pick, so one recipe serves Ghidra, jadx, apktool, Photon and BRouter. One pin conflict: pwntools refuses unicorn 2.1.3/2.1.4, so don't pin unicorn 2.1.4 in the shared Python env.

Notes: `research_notes/Svalbard picks follow up/re_tools_and_ai_skills.md`.

### F. AI skills as recipes

This works offline: all three harnesses on the stick read skills from local disk.

| Collection | Licence | Ship? |
|---|---|---|
| `mattpocock/skills` (39 engineering skills) | MIT | Yes |
| `obra/superpowers` (15 process skills) | MIT | Yes |
| `anthropics/skills` | Apache-2.0 per skill, except `doc-coauthoring` | Yes, without that one |
| `trailofbits/skills` + `skills-curated` | CC BY-SA | Yes, with attribution; share-alike applies |
| encidxvii reverse-engineering skill | Apache-2.0 | Yes |
| radareorg/radare2-skills, cellebrite ghidra-rpc, P4nda0s and haikow RE skills, ComposioHQ awesome list | none | No: no licence means no right to redistribute |

Shape: one recipe per collection, pinned to a commit tarball and unpacked to `skills/<collection>/`, plus an `ai/skills` pack that rides with `ai/harnesses` (default-256). The five shippable collections are under 20 MB together.

The runtime then points each harness at them:
- **opencode:** `skills.paths` in the opencode.json the runtime already writes.
- **pi:** the `skills` key in its settings.json.
- **goose:** has no extra-path setting, so the runtime copies or links the folders into its config `skills/`. Links may fail on exFAT; unverified.

That needs a new `type: skills` with an include list so a collection can be trimmed. It's M, or L if goose forces copying or if small local models pick the wrong skill when shown 70+ descriptions. In that case the include list per RAM tier becomes the opinionated default, for example a dozen skills on the 8 GB tier. Two existing inbox tasks (a svalbard skill for the drive's harnesses, and an opt-in agent skill with the CLI) belong in the same epic. One task says pi runs with `--no-skills`; that flag isn't in the code. Host skills are kept out anyway because `HOME` and `PI_CODING_AGENT_DIR` point onto the drive.

### D. pdf.js, a PDF app, and whether to ship a browser

The runtime serves the whole drive over HTTP, and PDFs open in the host browser's own viewer, which every current browser has. So pdf.js (6.2 MiB) only adds value on phones and old browsers that download PDFs instead of showing them, and as the base for in-PDF search later. Keep it as an M, not urgent. A separate PDF app isn't worth it: there's no small cross-platform viewer that beats the browser's. Don't ship a portable browser either. The smallest usable ones are 100+ MB per platform, need signing on macOS, and go stale fast; the host's browser is always there.

### H. Maps: address search, nautical charts, orthophoto, hillshade

Two findings break current assumptions:

- **The kapsi tile service is gone.** Its tile URLs return 404 or 410 Gone ("use MML's own service"). `recipes/maps/mml-maastokartta.yaml` (in `fi-maps`, so in every finland preset from 32 up) points at it. No `raster-tms` builder exists anywhere in the repo either, so that item has never been able to build. Filed as a high-priority inbox task. kapsi's file downloads still work.
- **DVV stopped publishing addresses in March 2025.** The open national set is now SYKE's Ryhti `open_address` (3.87 M addresses, CC BY 4.0, keyless CSV paging).

**Address search.** The research agent downloaded all of Ryhti (1.0 GB) and built an SQLite FTS5 index: 387 MB, with most queries in 0.4–3 ms. OSM has only about 1.1 M Finnish addresses, so Photon (OSM-only, needs Java 21) would find about 29% of what Ryhti does. Recommendation: a Ryhti + MML place-names gazetteer table behind a new `/api/geocode` in the Go runtime, which already uses FTS5. That's M, or L if it has to handle typos and Finnish inflections ("Mannerheimintiellä"). Drop Photon. Placement: fi-maps, finland-64 (387 MB). Ranking needs tuning: "aleksanterinkatu 52" put Oulu ahead of Helsinki.

**Traficom nautical charts.** Filed as a backlog task.
- The raster chart WMTS (48 layers) is keyless and CC BY 4.0, with the required notice "Not for navigational use". A z0–14 harvest is about 2 GB as WebP.
- Väylä fairways and navigation aids: CC BY 4.0 WFS, about 120 MB.
- Depth data (about 1.9 GB) is under the hydrographic licence, which allows modification only for non-commercial use, so it is opt-in only.
- There is no S-57 vector data and no bulk download, so this needs the harvest and WFS-dump builders. Placement once built: fi-maps, finland-256 (2 GB doesn't fit lower).

**Orthophoto** (your "depends on the size"): Uusimaa z0–16 is about 1.7 GB, z0–17 about 6.5 GB, and all of Finland z0–14 about 6 GB. These are estimates. The source is 1.07 TB of JPEG2000 on kapsi, or MML's API-keyed tile service. Suggest Uusimaa z0–16 in finland-256 once a raster builder exists.

**Hillshade.** MML 32 m hillshade is 288 MB for all of Finland (117 GeoTIFFs). Mapterhorn z0–12 is 113 MB for Uusimaa and 2.5 GB for a Finland bounding box. Use MML for finland-64 and skip Mapterhorn unless the world map needs terrain.

### I. CAD, SDR and speech apps (with your "stale but stable is fine")

All of these are GUI apps, so they wait on the AppImage/dmg convention. Placement once it exists:

| Set | Apps | Pack, tier |
|---|---|---|
| Making (essential) | OrcaSlicer 2.5.2, OpenSCAD (dated nightly: notarized, native on both Mac chips; re-pin yearly), FreeCAD 1.1.4, KiCad 10.0.8, Inkscape 1.4.4, Candle (UGS on Linux) | engineering, default-512: ~2.5 GiB on macOS arm64, ~1.7 GiB on Linux |
| Making in the browser (no convention needed) | miniPaint, draw.io, JSCAD web, three.js editor: no build step, fit the existing app-bundle type | engineering, default-64 |
| Making in the browser (needs a Node build) | OpenSCAD playground, Excalidraw, CascadeStudio | later |
| SDR | SDR++ (nightly; the Mac build is unsigned, so it needs a Gatekeeper workaround), gqrx, rtl-sdr tools, SatDump (1.2.2 on macOS/Windows, nightly AppImage on Linux), WSJT-X 3.0.2, JS8Call | communications, default-256 |
| Speech | whisper.cpp (speech to text), sherpa-onnx + Piper Finnish voice (text to speech) | ai, default-256 |
| Dropped | Vosk (no current macOS wheel, no Finnish model), Argos Translate (Hy-MT3 replaces it; its stanza dependency also pulls in torch), PythonSCAD (no Intel Mac or Linux arm64 build), PrusaSlicer (no Linux build any more) | — |

The browser apps are the cheap win: they run today from the stick's web server.

### J. World Factbook and Python docs

- **World Factbook.** The CIA discontinued it on 2026-02-04, so there will be no new editions, community or agent-made. The Kiwix ZIM (388 MiB) is the 2020 edition. The freshest data is `factbook.json` (CC0, frozen 2026-01-22). If you want it: the JSON through the table builder (small, searchable) beats the old ZIM. Leave it as a Maybe.
- **Python docs.** `devdocs-python` (4.6 MiB, now in default-64) is enough. The only worthwhile extra is the PEPs ZIM (9 MiB, `zimit/peps.python_en_all`); skip the 3.6 GB docs.python.org ZIM.
