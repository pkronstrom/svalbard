# v0.6.0 — Documents, wiki, Open WebUI, settings and vault updates

<!-- Draft for review. Cut only after the map viewer fix lands; see the TODOs. -->

Your own document collections, a live wiki and Open WebUI on the stick, a settings screen, and in-place updates of an installed vault.

## Highlights
- **Document collections**: list your own PDFs (URLs, files or whole folders) in `~/.config/svalbard/recipes/<id>.yaml`; they download into `books/<id>/`, their pages are indexed, and agents can find, read, cite and open them. The vault keeps a copy of each collection's recipe, so another computer can still plan and apply it.
- **Vault updates**: `svalbard update` reinstalls items whose recipe changed, swapping each in only once its new copy is in place; `svalbard catalog` lists the catalog, `init --preset/--add` creates a vault without the wizard, and `status` reports versions.
- **Wiki**: a Wiki menu entry starts SilverBullet behind a guarded local proxy; agents get tools to list, read, search and, behind a setting, write its pages.
- **Open WebUI**: an opt-in recipe and `./run` action, with the drive's tools served over Streamable HTTP and optional workspace file tools.
- **Settings**: recipes declare the settings they need; `./run settings` and a Settings screen in the drive menu store them on the drive, and launches honour them.
- **Workspace**: a workspace folder with path checks, served read-only, with file tools for agents.
- **Agents**: direct drive tools for pi, Goose telemetry off, and an agent skill shipped with the CLI (`svalbard skill print`, `skill install`).

## Catalog
- About 50 new recipes from a source survey: The Last Codex, Be Ready UK, Simple English Wikipedia, Wikivoyage EN, phone-sized English and Finnish Wikipedias, PhET EN, MDN, 21 DevDocs references, five TED growing collections, Learn AI Anywhere, US Army automotive manuals (TM 9-8000, TM 9-2700), the suomi.fi and SPEK 72 tuntia preparedness guides, Exploit-DB and MITRE ATT&CK.
- Browser apps that run from the stick: draw.io (offline mode), miniPaint and JSCAD.
- Map layers: SYKE river network and flood-risk areas, HSY public water posts. <!-- TODO: confirm they render once the viewer fix lands. -->
- **MML topographic map is back**: the new `raster-sheets` builder renders MML's 1:500k–1:50k map sheets from kartat.kapsi.fi into a WebP raster PMTiles (Uusimaa, zooms 0–14, about 650 MB), replacing the retired kapsi tile service. <!-- TODO: re-add mml-maastokartta to fi-maps in the release commit. -->
- SilverBullet 3.0.1 joins the default presets; the opt-in Open WebUI recipe has a menu entry.
- Fixed renamed Kiwix series (ready.gov, Gutenberg LCC moved to /zim/books/), relabelled `usda-nutrition` as the home-canning guide it is, and re-pinned Qwen3.8-27B IQ4_XS to unsloth's UD3.1 quant.

## Search and indexing
- PDF pages under `books/` are indexed; keyword search falls back to partial matches, and an empty search says what the drive holds.
- Index fixes: hyphenated compounds broken at a line end, macOS AppleDouble files, symlinked `books/`, unreadable files, re-indexing after a title change.

## Fixes
- `app-bundle` recipes with a single source URL install again (sqliteviz in tools-base failed on every apply).
- Downloads send a `svalbard` User-Agent; some hosts refuse Go's default.
- Per-platform binaries stay in `bin/<platform>/`; items with no download for the vault's platforms are skipped instead of failing the apply.
- Removals are reported as removals and delete their download records.
- A closed terminal stops an action as Ctrl+C does; settings save on FAT drives, which refuse chmod.
- TUI: typing in the vault path, returning from Browse, an honest Import screen and measured free space.

## Known gaps
- fi-maps still lists the OSM basemaps and `mml-topo-uusimaa`, whose build families are not implemented yet.
<!-- TODO: drop or reword this once the viewer fix and the fi-maps cleanup land. -->

The `svalbard-tools` image is pinned to 0.6.0. It adds GDAL's PNG, WebP and JPEG drivers for raster map builds.

**Full changelog:** https://github.com/pkronstrom/svalbard/compare/v0.5.0...v0.6.0
