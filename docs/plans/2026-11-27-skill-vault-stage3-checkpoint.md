# Skill vault stage 3 checkpoint: JS libraries, packing and HTML skills

Status: built and verified 2026-11-27 on branch `skill-vault-stage3` (cut from `main` at df98eba, stages 1 and 2 merged); all eight tasks of the [plan](2026-11-26-skill-vault-stage3-js-libraries-plan.md) with its owner direction (smallest footprint, one archive per recipe), stage 3 of the [design](2026-11-26-skill-vault-and-js-libraries-design.md). Not yet merged. An independent review of df98eba..68fa36d ran beside the definition-of-done tests; its findings are fixed in commits on top (see "Review"). Local `main` was merged twice (127cc23, 5a9378d) without conflicts.

## Resulting user path

1. **Install.** `default-256` (and any vault with `ai/harnesses`) gets `skills-core`, whose deps now bring `js-core` as well as `py-docs`; the new `ai/artifacts` pack, which `default-256` extends, holds `js-3d-cad`, `js-maps` and `js-media`. `svalbard add js-maps` works alone. Deselecting `skills-core` drops `js-core` and `py-docs`.
2. **What lands on the stick.** Each JS recipe is one archive, `lib/js/<id>.zip`, holding a trimmed `node_modules` tree and `svalbard-js.json`; nothing is unpacked. Beside it, loose, `lib/js/<id>/docs/` holds one quickstart and one reference file per library, which the drive's index reads. `maplibre-vendor` stays loose for the map viewer, now from the same pinned npm tarballs as `js-maps`.
3. **Find.** `skill_search` finds the HTML skills and each library once by its quickstart; a match inside a reference file names it, and `skill_read` lists and reads it (`file: "chart.js.reference.md"`).
4. **Build.** A skill's template is copied and edited in its marked regions; a page imports libraries by package name and its data files by path.
5. **Pack.** `svalbard-drive html pack page.html -o out.html` writes one file that opens by double-click without a network, or refuses with the cause (a CDN link, a fetch of a file beside the page, a missing file, a library measured `served`). `--dir` writes a folder for the drive's server (the offline map).
6. **Check.** Each HTML skill's `examples/check.sh` packs the page again and, with `scripts/html-check.sh`, loads it offline in headless Chromium and Firefox with a condition that it drew; `scripts/skill-run-check.sh` runs them in its sandbox.
7. **Natural Earth.** `svalbard add natural-earth` installs `data/natural_earth_vector.gpkg` (885 MB, unzipped); `vault_databases` lists its 183 layers and `query_sql` reads them. A vault from `main` with the zip at its root is fixed by `svalbard update natural-earth`, which reuses the zip.

## Contracts to keep

- **One archive per js-lib recipe, read in place.** `lib/js/<id>.zip`: sorted names, one fixed modification time, Deflate, so the same sources give the same bytes and checksum (`writeLibraryArchive`; js-lock builds the same archive to record its size). `svalbard-js.json` maps a package folder to its self-contained build (`main:` in the recipe), which a bare import of the package resolves to.
- **Docs are loose and written only when their bytes differ** (`writeLibraryDocs`): the quickstart is frontmatter generated from the recipe's `libraries:` (package, version, licence, portability, entry, `requires: [<id>]`) plus the embedded body; the reference file is embedded as generated. Files Svalbard did not write go; the folders of deselected recipes go; `lib/js/` stays (a missing index root is not a prune). A user js-lib recipe without `libraries:` gets each top-level package's README from its archive.
- **`<name>.reference.md` belongs to `<name>.md`** on the drive: one library result, the reference as its file.
- **A package is a library of at most one recipe** (catalog error otherwise); nested and hoisted dependencies may repeat. Portability is one of `single-file`, `served`, `directory`, or empty (unmeasured).
- **Pins come from the generators.** `build-tools/cmd/js-lock` writes `bundle:`, `archive:` and each library's `version`/`quickstart`, from `recipes/js/<id>.lock.json`; `build-tools/cmd/js-docs` writes the reference files and `libraries[].reference`. Hand fields are kept. An empty `include:` or `main:` stays out of the acquisition hash, so installed bundle recipes are not flagged for update.
- **`html pack` single-file output** has no network resource (HTML attributes, CSS `url()`/`@import`, literal `import()`/`fetch`/`importScripts`/`Worker`), no relative fetch (`fetch`, `d3.csv/json/…`, XHR with a relative literal), no local file reference, and no library whose quickstart says `served`/`directory`; nothing is written when any is found. A page's bare import resolves in the recipe that documents the package as a library (else the one archive that holds it, else an error naming the drive's libraries); a file inside an archive resolves bare imports up its own `node_modules` chain.
- **The page's own code is scanned strictly, library code is not**: any `http(s)://` or `//` string, `import.meta.url`, workers from files, computed `import()`, p5 `load*("file")`. Scripts html pack writes carry `data-svalbard-packed` and are not scanned again when a packed page or folder is packed again (the skills' checks do).
- **Licences travel with a packed page**: the trailing comment holds each bundled package's name, version and SPDX id, then every licence and NOTICE text (and a build's `*.LICENSE.txt`) once. js-lock refuses a kept package without a licence file unless the lock spec's `license_files` pins one, which is installed beside the package as a plain bundle source.
- **A page's module scripts are one bundle**, in their order, placed in the first module script; `--dir` writes `script.js`, refuses a folder holding the page or files it did not write (`.svalbard-pack` marks its own), swaps a new folder in, and copies the files beside the page that the page fetches or names.
- **The run check's sandbox** (macOS) reaches only the Firefox control port and the check's page server on loopback, binds local Unix sockets, and runs with proxy variables unset; `html-check.sh` detects an outer sandbox by trying to start one.
- **Budgets** (`libdocs_catalog_test.go`): per library at most 2 stick files and 8 files / 6 MB in the archive; per recipe the stick files and allocated size at 128 KB clusters in `recipeBudgets`; the include lists must agree with the recorded sizes, and a library keeps no source map, no `src/` tree and no minified copy beside an unminified one.
- **Reference files mark network instructions** of upstream docs (`not-for-this-stick` fences, "Not for this drive:" lines); `make verify` runs the offline lint on every quickstart and on the unmarked lines of every reference file.
- **The drive context** names `svalbard-drive html pack` only when a js-lib recipe is installed; every area on is 260 words without an index, 258 with one.

## Libraries

Archives (on the vault, byte for byte the size js-lock recorded): js-core 2,990 KB in 72 files, js-3d-cad 529 KB in 28, js-maps 549 KB in 25, js-media 396 KB in 14 (after the review's licence files). On the stick, at 128 KB clusters: js-core 33 files, 9.38 MB allocated (budget 33 / 11 MB); js-3d-cad 7 files, 1.38 MB (7 / 2 MB); js-maps 11 files, 2.62 MB (11 / 3 MB); js-media 7 files, 1.62 MB (7 / 2 MB). The whole stage adds 58 files and about 15 MB to a stick, against the design's estimate of 57 to 112 MB.

| Library | Version | Archive files | Archive bytes | Quickstart | Reference |
|---|---|---|---|---|---|
| alpinejs | 3.17.5 | 2 (+ LICENSE.md) | 55 KB | 1.5 KB | 215 KB (docs) |
| @picocss/pico | 2.1.1 | 4 | 92 KB | 1.5 KB | 8 KB (README; gap) |
| lucide | 1.81.0 | 4 | 501 KB | 1.3 KB | 37 KB (guide, 2,123 icon names) |
| chart.js | 4.6.2 | 6 | 508 KB | 1.5 KB | 291 KB (docs) |
| d3 | 7.9.2 | 4 | 278 KB | 2.2 KB | 806 KB (docs) |
| papaparse | 5.7.1 | 4 | 24 KB | 1.6 KB | 4 KB (README; gap) |
| dompurify | 3.5.0 | 5 | 166 KB | 1.2 KB | 63 KB (README, types) |
| minisearch | 7.2.0 | 4 | 92 KB | 1.4 KB | 69 KB (README, types) |
| fflate | 0.8.3 | 4 | 120 KB | 1.3 KB | 156 KB (docs) |
| marked | 18.3.0 | 4 | 63 KB | 1.2 KB | 59 KB (docs) |
| mermaid | 12.3.0 | 4 | 5,438 KB | 1.6 KB | 647 KB (syntax docs) |
| xlsx (SheetJS CE) | 0.20.3 | 4 | 1,001 KB | 1.6 KB | 29 KB (README, types; gap) |
| pdf-lib | 1.17.1 | 4 | 578 KB | 1.7 KB | 268 KB (README, types) |
| mathjs | 15.3.0 | 6 | 668 KB | 1.4 KB | 257 KB (docs) |
| @fontsource-variable/inter | 5.3.0 | 4 | 55 KB | 1.0 KB | 3 KB (README) |
| @fontsource-variable/fraunces | 5.3.0 | 4 | 44 KB | 1.0 KB | 3 KB (README) |
| three | 0.187.0 | 7 | 2,136 KB | 2.5 KB | 3 KB (README; gap) |
| @jscad/modeling | 2.13.0 | 4 | 252 KB | 2.0 KB | 107 KB (README, types) |
| @jscad/stl-serializer | 2.1.23 | 6 | 12 KB | 1.4 KB | 4 KB (README) |
| maplibre-gl | 5.24.1 | 5 | 1,128 KB | 2.1 KB | 572 KB (guides, types) |
| pmtiles | 4.5.0 | 3 (+ LICENSE) | 17 KB | 1.1 KB | 14 KB (README, types) |
| @turf/turf | 7.4.0 | 4 | 543 KB | 1.5 KB | 281 KB (every module's README) |
| proj4 | 2.23.0 | 4 | 145 KB | 1.2 KB | 10 KB (README) |
| topojson-client | 3.1.0 | 4 | 25 KB | 1.0 KB | 16 KB (README) |
| p5 | 1.11.13 | 4 | 1,077 KB | 1.6 KB | 12 KB (README; gap) |
| tone | 15.1.22 | 5 | 353 KB | 1.3 KB | 462 KB (README, types) |
| matter-js | 0.20.0 | 4 | 95 KB | 1.6 KB | 9 KB (README; gap) |

Every library is its one browser build: the self-contained file where the project publishes one (named by `main:`), else its ESM closure as esbuild reads it (Chart.js with `@kurkle/color`; Three.js `three.module.js`, `three.core.js` and the two addons the quickstart names; the STL serializer with `@jscad/array-utils`; PMTiles with fflate).

**Documentation gaps** (the reference is the README alone, or README and types): Pico CSS (docs are the picocss.com site; closing it: convert the site's pages, S), Papa Parse (docs.html; S), SheetJS (docs.sheetjs.com, separate repository, Apache-2.0; M), Three.js (about 1,000 HTML manual pages, types in `@types/three`; M to L by how much), p5.js 1.x and Matter.js (generated API sites; M each).

## Portability (measured)

Each quickstart's example was packed on a vault with the four recipes and loaded by file://, network off, with a condition that it drew and, for exports, a valid download (`scripts/html-check.sh`, Chromium 158 and Firefox 161 headless, macOS arm64).

| Library | Chromium file:// | Firefox file:// | Through a server | Level |
|---|---|---|---|---|
| every library but PMTiles (26) | pass | pass | not needed | single-file |
| pmtiles (MapLibre over `osm-uusimaa.pmtiles`) | fails: fetch of the archive blocked from origin null | fails: no content length for the range read | pass, both | served |

Conditions: chart pixels; D3 paths; Mermaid SVG; both fonts loaded; Alpine's list; Lucide SVGs; WebGL canvas for Three.js; MapLibre canvas over GeoJSON; downloads of a 12-triangle binary STL (watertight), a zip, a PDF and an xlsx. A `--dir` map folder works through a server and not from file:// (module scripts by src are blocked there). The map viewer, after `svalbard update maplibre-vendor` (byte-identical files), loads its layers through a server in both browsers.

Negative controls of the checker: a CDN script, an uncaught error and a relative fetch fail (Firefox allows a fetch from the same folder; Chromium does not, so `html pack` refuses it).

## HTML skills

| Skill | Requires | Template | Run check (vault, network denied) | SKILL.md tokens |
|---|---|---|---|---|
| html-app | js-core | Pico, Alpine, fonts, themes, localStorage list | PASS | 581 |
| data-dashboard | js-core | Chart.js ×2, sortable table, load another CSV/JSON | PASS | 561 |
| geodata | js-core, py-docs | `gpkg_to_geojson.py`; D3 choropleth + top-10 bar chart | PASS (fixture GeoPackage and Natural Earth: 177 countries, Finland bbox 20.65, 59.85, 31.52, 70.16) | 749 |
| offline-map | js-maps, type:pmtiles | MapLibre + PMTiles basemap, GeoJSON places | PASS (served, basemap features drawn) | 654 |
| diagram | js-core | Mermaid flowchart, sequence, gantt | PASS (a broken diagram fails) | 549 |
| parametric-cad | js-3d-cad | involute spur gear, Three.js preview, STL | PASS (4 gears watertight in the page; exported STL 2,496 triangles, 7,182 mm³, watertight) | 567 |
| generative-art | js-media | seeded p5 1.x flow field, sliders, PNG | PASS (same seed, same picture) | 615 |

All 14 skills on the vault passed `scripts/skill-run-check.sh` in 2 min 16 s. A sweep of 144 gears (module 0.5 to 10, 8 to 150 teeth, bores 0, 3 and 12 mm) had no open edge.

**Whole-context budget at the 8K tier** (data-dashboard), Qwen3.5 4B's tokenizer: pi's first request on this vault (skills and JS libraries, no wiki), as llama-server counted it, is 3,434 tokens; the skill (582), the Chart.js quickstart (486) and the template (1,475, after trimming a comment and three CSS declarations) add 2,543, for 5,977: just under the plan's ~6,000. The template is the largest part; split it if a model at 8K runs out. Nothing was run at 8K: the definition-of-done runs and model checks ran at 32K.

## Definition-of-done tests

pi 1.6.1 from the drive's menu action (`run __native-agent pi <model>`), Balanced tier (32K), one run each in an empty work folder; `HOME`/`XDG_*` in scratch; `sandbox-exec` denying every connection except localhost and Unix sockets (pi talks to the drive's llama-server); a no-op `open` first on PATH. "Without orientation" is pi's generated `AGENTS.md` emptied for the run (kept as a user edit) and restored afterwards; pi still has the tool descriptions. The vault: the stage 2 test vault (WikEM-type ZIMs, two PDFs, osm-uusimaa, qwen-4b, qwen-27b, py-docs, skills-core) plus `natural-earth` and the four JS recipes, keyword index; not `default-256` (team-lead instruction).

Pass is judged on a copy of the packed file alone in an empty folder (as on another computer), by `scripts/html-check.sh` in Chromium and Firefox by file://, network off: test 1 wants at least 100 SVG paths, a chart drawn and the data inside ("Finland" in the file); test 2 wants a canvas and, in Chromium, the page's "Download STL" to save an STL that `validate_stl.py` finds watertight with a positive volume.

| Run | Model | Orientation | Result | Time | Tool calls (failed) | What happened |
|---|---|---|---|---|---|---|
| map | Qwen3.8 27B | yes | **pass** | 3.4 min | 12 (0) | `vault_sources`, `skill_search`, `skill_read` geodata, one `query_sql`, exported 177 countries, edited the settings (one fumbled edit fixed with perl), packed (651 KB) and ran the check |
| gear | Qwen3.8 27B | yes | **pass** | 17.8 min | 4 (0) | found and read parametric-cad, packed the template with its defaults and ran the check; most of the time was thinking |
| map | Qwen3.8 27B | no | **pass** | 35.2 min | 9 (0) | looked at `vault_databases` and the 10m countries first, then found geodata the same way; 691 KB |
| gear | Qwen3.8 27B | no | **pass** | 17.2 min | 6 (0) | as with orientation |
| map | Qwen3.5 4B | yes | **pass** | 17.8 min | 25 (6) | found geodata at once, but ran the exporter from inside the skill folder (`cd` there), so `countries.geojson` landed in the skill folder; failed packs (import path wrong) until it copied the file to its work folder; ran the check |
| gear | Qwen3.5 4B | yes | **pass** | 28.8 min | 12 (3) | read the template and wrote it out again whole (unchanged geometry), packed, then ran `check.sh` by a relative path, copied the skill's scripts into its work folder and ran `validate_stl.py` on the HTML file |
| map | Qwen3.5 4B | no | **pass** | 16.8 min | 15 (3) | found geodata; again wrote `countries.geojson` inside the skill folder; tried `-o map.html` over its own page (html pack refused); packed `population-map.html` and ran the check |
| gear | Qwen3.5 4B | no | **pass** | 15.7 min | 6 (0) | copied, packed and checked the template as written |

All eight produced a single file that works by double-click in both browsers with the data inside; every exported STL had 2,496 triangles, watertight, 7,182 mm³. Every run found its skill with `skill_search` first, with or without the orientation: on artifact building the drive context made no difference to the outcome on either model (the 27B without it took a detour through the databases first). The difference between the models is in following the steps: the 4B model left the work folder (twice it wrote into the skill folder, which the next apply restores), wrote files whole instead of editing, and fumbled the check's path; the 27B model followed the steps as written. Times include the model's thinking (Qwen3.8 at medium) and are pi's session span.

Two skill wordings changed after these runs: every HTML skill now says to run each command from the working folder and not `cd` into the skill folder, and copies the template with `cp -n`.

MODEL_CHECKS_PLACEHOLDER

## Review

An independent review of df98eba..68fa36d (with two helpers) found twelve confirmed problems and five plausible ones; all confirmed ones and the contained plausible ones are fixed, each with a test that fails without the fix where one could be written:

1. **Data race in `html pack`** (esbuild calls the plugin from several goroutines; `go test -race` reported it): the bundled-package set and each archive's package.json cache are under mutexes; `make verify` runs the htmlpack tests with `-race`.
2. **The check let network and file:// loads through** (remote `d3.json`/XHR, runtime-built URLs, SVG `href`, `<base>`, `<meta refresh>`, workers, `import.meta.url`, computed `import()`, p5 `loadImage("file")`, `type=" module "`, source-map comments): the page's own code is scanned before bundling for any `http(s)://` or `//` string (XML namespaces of www.w3.org allowed) and those load forms; the output check covers the rest; library code is not strictly scanned. A URL built at run time from parts (`fetch(base + "x")`) still cannot be seen.
3. **`<script src="app.js">` beside the page** was taken for a package: a file wins.
4. **Licence obligations**: the packed file's comment now holds the licence and NOTICE texts of every bundled package (once each) after the names and SPDX ids; js-lock keeps a build's `*.LICENSE.txt` (Math.js, Tone.js) and refuses a kept package without a licence file unless the spec pins its text (alpinejs, pmtiles now installed beside the package). Mermaid's self-contained build carries only two `@license` banners for its many bundled dependencies; that is upstream's packaging and stays as it is.
5. **A user js-lib recipe could take over a built-in library**: every package a recipe provides (its libraries, or all top-level packages when it lists none) is unique across recipes, and `html pack` refuses a package two recipes document.
6. **`MarkCurrent` compared the GeoPackage's checksum with the zip's pin**: the pin is compared only with an entry that is the downloaded file; a GeoPackage must be under `data/`.
7. **`--dir` could overwrite the page and user files**: it refuses a folder holding the page or files it did not write, writes a new folder and swaps it in (marker `.svalbard-pack`).
8. **`--dir` dropped relative fetches and assets**: they are copied into the folder or reported missing.
9. **The macOS run-check sandbox reached every localhost service** (a proxy on 127.0.0.1 got out): only the Firefox control port and the check's page server are reachable, proxy variables are unset, `html-check.sh` no longer allows outbound Unix sockets and detects an outer sandbox instead of trusting a variable. A probe check could reach neither a local web server nor the network through it as a proxy.
10. **Budgets checked recorded numbers**: the test now checks the include lists themselves and the content rules (no source maps, no `src/`, no minified copy beside an unminified one).
11. **One corrupt archive stopped every pack**: it is skipped with a warning; a page that needs it fails naming it.
12. **Lower severity**: `generative-art` no longer suggests `loadImage("file")`; `html-check.mjs` treats only the `--serve` origin as local, reports Firefox's failed local loads, watches Chromium workers and returns SKIP when a browser is missing (the skills' checks then say "browser check skipped"); `offline-map` draws raster basemaps and names the drive from the skill's folder; `gpkg_to_geojson.py` quotes its SQLite address, writes empty points as null and reports truncated geometries; a drive with only JS libraries gets the packing sentence; `Result.Notices` lists the bundled libraries; js-lock drops libraries that left the spec, requires a sha256 for direct tarballs and refuses https-to-http redirects; the skills copy templates with `cp -n`.

Plausible, fixed: a page's module scripts are bundled together in order (shared modules and libraries once); a script holding `<!--` and `<script` goes in as a data URL so the HTML parser cannot swallow the rest; user js-lib recipes are copied into the vault; Chromium workers are watched. Plausible, recorded: Natural Earth's URL is unversioned but pinned, so an upstream re-release fails fresh installs with a checksum mismatch until the pin is regenerated; installing needs the 0.45 GB zip and the 0.89 GB GeoPackage on the stick at once, and `size_gb` counts only the latter.

Found while re-checking: packing a packed page again (as the skills' checks do) refused Mermaid's inlined code, which holds XML namespace addresses; scripts html pack writes are now marked `data-svalbard-packed` and skipped by the strict scan.

## Differences from the plan

- **Owner direction applied**: one archive per recipe read in place (not `node_modules` trees on the stick), one build per library, no `.d.ts` trees, reference files, budgets in a catalog test. js-lock computes each library's file set by bundling its entries with esbuild (the closure) instead of hand-written include globs, and records `extracted`/`archive` sizes; the plan's `include:` remains for user recipes and the generator's output (exact names, escaped).
- **Natural Earth's zip is not single-member** (it also holds VERSION, README.md, CHANGELOG): apply installs its one `.gpkg`. `vault_databases` lists a GeoPackage's layers from `gpkg_contents` (183, not its 925 tables), `query_describe` leaves columns out above 25 tables, and blobs come back as their size.
- **Self-contained builds by `main:`**: D3, Mermaid, Lucide, Math.js, pdf-lib, SheetJS, Turf, proj4, topojson-client, p5, Tone, Matter.js and the fonts are their one file; a bare import resolves to it. Mermaid's build is a classic script that sets a global, so it loads as `<script src="mermaid">`, which `html pack` inlines from the archive.
- **Fonts**: `@fontsource-variable/inter` and `@fontsource-variable/fraunces` (one Latin variable woff2 each), used through CSS `url("@fontsource-variable/inter")`, which `html pack` resolves to the font.
- **Provisional decisions (for the owner):**
  1. *Reference files are generated at development time and embedded in host-cli* (4.3 MB of Markdown in `host-cli/internal/toolkit/libdocs/`), from GitHub raw files at each version's tag and the npm tarballs; not fetched at apply. Why: repository archives for docs are hundreds of MB (three.js, Mermaid), and apply would need GitHub.
  2. *offline-map output goes to `<drive>/workspace/<name>/`* with `--dir`, opened at the maps server's address plus `/workspace/<name>/` (the maps server serves the drive at its root, so `/maps/<file>` is the tile source).
  3. *The run-check sandbox allows loopback* (the browsers' control channels) *and Chromium binding its own Unix socket*; outbound Unix sockets stay denied. `html-check.sh` confines itself the same way when run alone. Chromium runs with `--no-sandbox` and Firefox with its content sandbox off inside that sandbox (they cannot nest).
  4. *User recipes may be js-lib recipes* (pinned bundle, libraries), installed as an archive like the built-in ones.
  5. *The drive context lost three phrases* to fit the html pack sentence in 260 words: "they are on this drive", "They fail.", and "Pass the source and path from a search result" became "Pass a search result's source and path".
- **Quickstarts use `requires: [<recipe>]` frontmatter generated at apply**, not hand-written frontmatter, so portability and versions come from the recipe alone.
- **`html pack` keeps module scripts as inline ESM** (`type="module"`), not IIFE: deferred like the original, and top-level await works.

## Scope and limits

- Verified on macOS arm64 only (Chromium 158, Firefox 161 headless). Linux and Intel runtimes build; html pack and the browser checks were not run there. Safari not checked (best effort by design).
- The definition-of-done runs used a minimal vault, not `default-256` (team-lead instruction), with Qwen3.8 27B and Qwen3.5 4B at the Balanced tier (32K).
- `html-check.sh` needs Node 22 or later (built-in WebSocket); it is a development tool, not on the stick.
- A page that imports a library twice in two module scripts bundles it twice.
- Hardware APIs (Web Serial, WebUSB) were not exercised; no library here uses them.
