# Catalog update audit — 2026-09-30

Findings from a full link check (310 URLs / 181 recipes), tool/app release audit, open-data review, and ZIM library scan. Audited against the working tree, i.e. on top of the uncommitted [catalog refresh](2026-09-30-catalog-refresh.md). Raw link-check output (script, `results.json`, full Kiwix index `all.tsv`) was kept only in the session scratchpad — rerun the check before closing this out.

Status 2026-11-21: remaining open items moved to the task tracker; this checklist is the audit record.

User decisions: rebalance presets toward their target size; implement local custom presets; specialization packs (cybersec, computing, sciences, …) stay opt-in but should be offered by a wizard/web "pack builder"; fix all updates and broken pins.

## Phase 1 reconciliation

The resumed [catalog refresh results](2026-09-30-catalog-refresh-results.md) supersede the initial model proposals below. Completed: invalid Binwalk pin replaced by legacy v2.3.4 source (Python builder remains blocked); zstd removed from tools-base; Qwen 4B/9B refreshed to revision-pinned Unsloth quants; Qwen3.8 27B IQ4/Q6 tiers added; Gemma 4 12B QAT added; refreshed model revisions pinned; MTP shipped as an optional matching-drafter pack. Nomic remains the embedding default. The original checklist remains as audit history; these completed catalog items must not be implemented again. Live chat-template/MTP conversations and Python installation are still unverified. Remaining items in this audit belong to Phase 1b or their linked user-path phases.

## 1. Broken pins and links (fix first)

- [x] Resolver ignores Kiwix letter-suffixed re-releases (`2026-09b`) — regex now `(\d{4}-\d{2}[a-z]?)`, test added (`host-cli/internal/resolver`). Picks up nhs-medicines 2026-09b, 3dprinting 2026-07a, scoutwiki 2026-08a.
- [x] `maplibre-vendor`: `maplibre-gl@6.11.2/dist/maplibre-gl.js` 404 — v6 is ESM-only; `mapview/generate.go` loads it as a classic script. Pin **5.24.0** (verified 200) or port viewer to ES modules.
- `duckdb-wasm`: (css → `shell_bg.wasm` done; launcher page still missing) `shell.css` doesn't exist in `duckdb-wasm-shell@1.32.0`; bundle also lacks `shell_bg.wasm` (1.5 MB) and an index page — nothing on the drive consumes this bundle today. Replace css with `shell_bg.wasm`, add a launcher page, or drop the recipe.
- [x] **Blocker for all python-package recipes:** `pythonvenv.go` calls `uv pip download`, which does not exist in uv 0.12.21 (Docker image) or 0.11.7 — angr, pwntools, sqlmap, volatility3, binwalk cannot build. Switch to `python -m pip download --only-binary=:all: --platform … --python-version …` in the tools image (or `uv pip install --target` per platform). — fixed: builder now uses `uv python install --no-bin` (python-build-standalone per platform) + `uv pip install --target --python-platform` per package, wheels-only first then source fallback; per-platform skip with warning when no wheels. Linux floor glibc 2.28 (Pi OS bookworm), macOS arm64 14.0 / x86_64 13.0. E2E: all 5 packages × 4 platforms built (~1.8 GB), run on macos-arm64 + linux x64/arm64. Gaps: angr on linux-arm64 (z3 wheel needs glibc 2.38) and macos-x86_64 (no wheel).
- [x] `binwalk==2.4.3` doesn't exist (PyPI has only 2.1.0; last Python tag is v2.3.4, v3 is Rust with no prebuilt assets). Options: `binwalk @ https://github.com/ReFirmLabs/binwalk/archive/refs/tags/v2.3.4.tar.gz` (sdist — test against the platform-targeted wheel download), or build v3. — now `binwalk @ OSPG/binwalk v2.4.3` tarball (the fork the old pin came from; MIT, archived 2024-11). Upstream 2.3.4 crashes on py3.12; Rust v3 still has no release binaries.
- [x] `tool-openocd-esp32`: PlatformIO espressif32 v7.1.3 requires `~2.1100.0` → pin **2.1100.20220706** (all 4 platforms verified 200, incl. darwin_x86_64).
- [x] `toybox`: floating `landley.net/toybox/bin/` → pin `downloads/binaries/0.8.14/toybox-{x86_64,aarch64}` (verified).
- [x] `angr` 10.0.0 → 10.0.1 (released today).
- [x] `qwen-9b`: 404 — bartowski renamed to `Qwen_Qwen3.5-9B-Q4_K_M.gguf`.
- [x] `luonnonsuojelualueet`: SYKE split the zip into `_valtio.zip`, `_yksityinen.zip`, `_eramaa.zip` → `vector-static` needs multi-source support (or three layers). — done: `vector-static` now takes `assets` (one shapefile zip each, merged into one layer); test added.
- `mml-maastokartta`: kapsi `maastokartta` tiles gone (kapsi-tiles.y2k.fi 410). Options: MML WMTS with API key from env, or `maastokarttarasteri` sheets / `peruskartta_3067`. Builder family not implemented yet anyway.
- [x] `opensourcelowtech`: domain lapsed (redirects to DropCatch) — delete recipe (unused).
- [x] `practical-action`: gone from download.kiwix.org — remove from default-32, fi-survival, homesteading; delete recipe.
- [x] `{date}` patterns matching nothing:
  - [x] `wikipedia-en-100-maxi/-mini`: flavours dropped, now single `wikipedia_en_100_{date}` (318 MB) — collapse to one recipe; re-check default-2 budget and test-comprehensive.
  - [x] `wikipedia-en-all`: duplicate of `wikipedia-en-maxi` (`_all_maxi_`) — delete, simplify default-2tb.
  - [x] `wikipedia-fi-all` → `wikipedia_fi_all_maxi_{date}` (7.4 GB).
  - [x] `ifixit` → `/zim/ifixit/ifixit_en_all_{date}.zim` (3.3 GB).
  - [x] `khan-academy` → `other/khanacademy_en_all_2023-03.zim`, **168 GB** (not 10) — move out of default-128 / fi-reference to 1tb+.
- [x] `gtfobins.github.io` → `gtfobins.org`.
- [x] License URLs 404: `libretexts.org/licensing` (8 recipes), luke.fi (foraging-habitats-fi). — libretexts → commons.libretexts.org/licensing, luke.fi → /en/statistics.
- [x] Fetcher: `caido.download` 404s on HEAD but serves GET — don't rely on HEAD. — confirmed false positive (GET 302→file); only relevant if the fetcher ever HEAD-probes.

## 2. Size drift (`size_gb` vs resolved file, >30%)

- [x] `gutenberg` 10 → **206 GB** (in default-128!) — replace with `gutenberg_en_lcc-{t,s,r}` subsets (12 / 4.2 / 1.9 GB); full set 1tb+ only.  — done: `gutenberg-en-lcc-{t,s,r}` recipes added; default-128 gets s+r, default-256 adds t, full gutenberg + khan-academy moved to default-1tb; fi-reference uses the subsets. Top-100 is now a single `wikipedia-en-100` recipe (318 MB).
- [x] `wikimed` points at `mdwiki_en_all` (10 GB); intended `mdwiki_en_all_maxi` (2.1 GB).
- [x] Understated: wikem 0.04→0.35, wiktionary-en 5→8.5, SE electronics 2→3.9, SE math 5→6.9, natural-earth 0.3→0.42.
- [x] Resized 2026-11-21 from upstream Content-Length: cyberchef 0.015→0.083 (11.6.0 zip 83.4 MB), gentoo-wiki 0.069→0.092 (2026-07 ZIM 92 MB).
- [x] Overstated: SE cooking 1.5→0.23, amateur-radio 0.5→0.07, survival 0.5→0.14, biology 1.0→0.4, chemistry 1.0→0.4, engineering 0.5→0.24, diy 3→1.9, serverfault 5→1.5; wikipedia-fi-nopic 5→2.6; wiktionary-fi 2→0.43.
- [x] `fas-military-medicine` → newer `zimit/irp.fas.org_en_military-medicine_{date}` (2026-05).
- [x] `scripts/sync-catalog.sh` failed on files deleted from disk but still tracked → now skips missing paths.
- [x] `scripts/check-size-drift.sh` (2026-11-21; `host-cli/cmd/size-drift`) reports recipes whose `size_gb` drifts more than 30% and 10 MB from upstream Content-Length. It needs the network, so it is not in `make verify`. First run: 27 drifted under the plain 30% rule; WikEM no longer exists on Kiwix.

## 3. Tool versions (after today's refresh)

Everything else verified current with all platform assets resolving (see refresh plan). Remaining:
- `nmap-static` 7.94SVN — ernw/static-toolbox dead since 2023; find another static source or build.
- `zstd` listed in tools-base but deferred (no prebuilt) — build it or drop from the pack.
- ESP32: add `macos-x86_64` to xtensa / ulp / openocd (registry has builds); add `toolchain-riscv32-esp` recipe for C3/C6.
- Only `framework-espidf` and `tool-esptoolpy` carry sha256 — covered by Phase 2 acquisition tracking.

### Missing tools (ranked; all 4/4 platforms unless noted)
- ripgrep 15.2.0 (tools-base; coding harnesses expect `rg`)
- jq 1.8.2 + yq 4.54.1 (tools-base)
- duckdb CLI 1.5.6 (tools-base; pre-cache spatial extension)
- pandoc 3.12 (computing)
- rclone 1.75.1, restic 0.19.1 (tools-base)
- arduino-cli 1.5.1 (esp32-dev; cache cores)
- meshtastic CLI 2.7.11 (python-package, communications)
- syncthing 2.1.5, croc 11.5.4 (communications)
- ffmpeg static + yt-dlp (two vendors, pin dated builds)
- tealdeer + tldr pages, micro editor
- rtl_433 (no mac-x64; check linkage)
- Skipped: whisper.cpp (no release assets — needs a build handler), Piper, GDAL, qsv.

## 4. New ZIMs (ranked)

- [x] `gutenberg_fi_all` (1.5 GB) — fi-reference, finland-64+
- [x] `gutenberg_en_lcc-{t,s,r}` — added (see §2); could also join homesteading / survival-medical packs
- [x] `libretexts.org_en_workforce` (490 MB; HVAC, welding, electrical) — homesteading/engineering 64+
- [x] `openwrt.org_en_all` (496 MB) — communications/computing 64+
- [x] `restarters_en_all_maxi` (17 MB), `wikivet.net_en_all_maxi` (54 MB), `gardenology.org_en_all` (107 MB) — small, every tier
- [x] SE: medicalsciences (58 MB, every tier), mechanics, bicycles, homebrew, earthscience, crafts; dsp/iot/retrocomputing/gis (128+) — added all except retrocomputing
- [x] Finnish small: `wikibooks_fi`, `phet_fi`, `wikivoyage_fi`, `wikisource_fi_nopic`, `scoutwiki.org_fi` (partiotaidot) — all tiers, fixes finland-32
- [x] Cheap Finnish Wikipedia flavours: `wikipedia_fi_top_nopic` 431 MB / `_top_maxi` 1.6 GB / `_all_mini` 1 GB for 32/64 tiers — top_nopic (finland-32) and top_maxi (finland-256) added; all_mini skipped
- [x] canadian-prepper videos (winterprepping etc.), trueprepper (1.3 GB) — fi-survival 128+
- [x] OPSEC: privacydefence opsecbible + anonymousplanet — cybersec-reference
- [x] devhints, postmarketos, openstreetmap-wiki_nopic, topic Wikipedia slices (computer/math/physics nopic) — devhints + openstreetmap-wiki added; postmarketos and topic slices skipped
- [x] TED solar/electricity/farming — homesteading 256+ — electricity + farming added (the "solar" ZIM is solar-system astronomy, skipped)
- [x] Big-tier only: `survivorlibrary.com_en_all` (205 GB), `stackoverflow.com_en_all` (107 GB, first refresh since 2023)
- Stale upstream, watch: cdc-travelers-health, ready-gov, medlineplus, lowtech-magazine, zimgit-*, libretexts bio/chem/eng/med.

## 5. Open data

Existing recipes — pin by date + sha256 (fits Phase 2):
- `osm-finland/uusimaa`: Protomaps dailies are pruned (~62 kept); `osm-extract` family has no Go handler (nor `raster-tms`, `mml-topo`). Pin `build_date` + basemap major v4 + b3sum from builds.json; fall forward on pruning.
- `mml-topo-uusimaa`: pulls 22.8 GB all-Finland zip for one region — use MML OGC API bbox. **MML KMTK replaces Maastotietokanta from autumn 2026** — plan migration.
- `natural-earth`: add `version: 5.1.2` + sha256.
- `fimea` (daily regen), `pohjavesialueet`, `lipas-recreation`: snapshot date + sha256; fimea writes a `meta` table.
- `foraging-habitats-fi`: url TBD; `metsa-structures-pmtiles.py` referenced in builders README doesn't exist.
- `usda-nutrition` (SR 2015 ZIM) superseded — see FoodData Central below.

New (ranked):
- Hillshade: kapsi `vinovalovarjoste` (small) or Mapterhorn terrain pmtiles (Finland extract)
- SYKE lake/river depths (`syvyysalue`, `syvyyskayra`, static since 2016) + `uomaverkosto` river network
- Traficom nautical depth/chart data (CC BY 4.0, "not for navigation")
- USDA FoodData Central (Foundation 2026-04 + SR Legacy) + **Fineli** (Finnish food composition) → sqlite FTS
- OSM survival POIs (water, springs, huts, shelters, fireplaces) from Geofabrik PBF via osmium
- Overture places/addresses (DVV addresses ended 2025-02; only latest Overture release kept → mirror)
- WHO EML 2025 (CC BY-NC-SA — check fit), openFDA labels subset
- FinBIF species checklist v1.36, FMI climate normals, GeoNames / MML place names
- Offline routing (Valhalla from pinned PBF) — needs runtime capability; larger project
- Metsäkeskus forest GeoPackage (due 2026-09) — input for foraging-habitats-fi
- Not viable: väestönsuoja registry (not public), PFAF (license), Digitraffic (real-time), SRAL repeaters (no export).

## 6. Models

Done 2026-09-30 (measured on M4 Max / Metal, llama-server b11146):
- [x] Runtime: stay on llama.cpp b11146 (ik_llama is CUDA/CPU-oriented, LM Studio doesn't expose drafters on macOS, MLX is Mac-only).
- [x] Qwen3.8-27B: `qwen-27b` = UD-IQ4_XS (13.27 GiB, 24 GB tier), `qwen-27b-hq` = UD-Q6_K_XL (23.56 GiB, 64 GB tier). MTP draft is the optional `ai/mtp` pack (1.28 GiB); no speedup measured on Metal (24.2→23.7 / 21.5→17.3 tok/s), CUDA reports +33–145%.
- [x] Gemma MTP dropped (measured −25…−40% at n=4); runtime never enables it; test added.
- [x] Fixed Qwen3.8 chat template shipped (`llamaserve/qwen3.8.jinja`): mid-conversation system/developer messages and pi's minimal/max effort no longer 500.
- [x] pi provider renamed `llama.cpp` → `svalbard` (pi 0.99.1 reserves the old id — generated config was broken). pi recommended default; pi/OpenCode/Goose all completed a tool task on Qwen3.8.
- [x] Roster: 8 GB e4b + qwen-4b · 16 GB gemma-4-12b + qwen-9b · 24 GB 26b-a4b + qwen-27b · 64 GB 31b + qwen-27b-hq (+ optional qwen-35b-a3b). All 14 model URLs verified; sizes in GiB; revisions pinned.

Open:
- MTP drafter as a real recipe layer (user direction): add `draft_for: [qwen-27b, qwen-27b-hq]` to `qwen-27b-mtp`; planner refuses/warns when no target is selected; runtime reads the pairing from recipe metadata instead of the hard-coded filename in `llamaserve.packagedDraft`, enabling MTP automatically when both are installed. Partly done 2026-11-21: `qwen-27b-mtp` declares `draft_for`; catalog load rejects unknown targets; `svalbard add` warns about a drafter with no selected target (`host-cli/internal/catalog/drafters.go`, `commands.DrafterWarnings`). Still open: the wizard review warning and the runtime reading the pairing (e.g. from `.svalbard/recipes.json`) instead of `packagedDraft`'s hard-coded names; first re-measure MTP/DFlash2 at llama.cpp ≥ b11840 (`docs/plans/research/2026-11-21-model-backend-updates.md`).
- Embedder selection picks the first file in `models/embed/` — make it explicit before adding an opt-in multilingual embedder (Qwen3-Embedding-0.6B).
- Unverified: Linux x64/arm64 and Intel Mac runs, MTP on CPU, Q6_K_XL live, 8 GB-tier models live, Svalbard MCP tools inside harnesses.
- Vision (`--mmproj`) still unsupported in the runtime.
- `qwen-0.8b` orphaned — keep or drop.

## 7. Presets and packs

- [x] **Local custom presets** — done: `~/.config/svalbard/presets/*.yaml` (XDG; plus `local/presets/` in dev), same `extends`/`-id` semantics via `ResolvePreset`, strict validation for user presets (unknown ids/extends, cycles, name clashes are errors), `preset copy` writes an `extends` stub (`--flatten` for old output), `add`/`remove` accept pack/preset names, wizard lists them as (custom). Open question: a bad user preset fails catalog load for every command — relax to warn-and-skip?
- [x] Rebalance tiers toward target size (currently: 32→20%, 64→46%, 128→44%, 256→44%, 512→26%, 1tb→24%, 2tb→13%) — after §2 size fixes, since several numbers change a lot. — done. Chain: finland-N extends default-N + finland-(N/2), re-removing superseded editions. Content (GiB) / % of real drive capacity: 2: 1.5 (74%) · 32: 17 / fi 23 (58/78%) · 64: 40 / fi 50 (67/83%) · 128: 91 / fi 100 (76/84%) · 256: 170 / fi 181 (71/76%) · 512: 366 / fi 381 (77/80%) · 1tb: 703 / fi 717 (74/75%) · 2tb: 1026 (54% — few curated large archives remain to add). AI: 256 gets harnesses + 8/16 GB models, 512 + 24 GB, 1tb + 64 GB. New recipes were also added to the matching specialization packs.
- [x] `default-2` / `finland-2` fit under 2 GB (now ~2.4 GB).
- [x] `finland-32` adds nothing over default-32 — add Finnish small ZIMs, fimea, wiktionary-fi, a Finnish Wikipedia flavour.
- [x] fi-reference pack: khan-academy (168 GB) + gutenberg (206 GB) blow its 30 GB target.
- [x] `makeityourself` now in default-256.
- `open-source-ecology` / `opensource-ecology` are the user's gitignored `.local.yaml` recipes and crawl different sites (WordPress vs the GVCS wiki) — not duplicates; keep the wiki one if trimming.
- `qwen-0.8b` orphan (see §6).
- Specialization packs (cybersec*, computing, sciences, engineering, communications, esp32-dev, homesteading) offered via wizard / web "pack builder" rather than baked into tiers.

## 7c. Final verification (2026-09-30)

- [x] `make verify` passes (52 packages, catalog in sync).
- [x] Full link check rerun: every download URL and `{date}` pattern resolves; only `mml-maastokartta` (open, §1) remains broken.
- Decide `size_gb` semantics for multi-platform recipes (python packages are per-platform: angr ~0.43 GB, pwntools ~0.15 GB each, recipes say 0.2 / 0.05).
- Docker builders write as root on Linux hosts — reinstall after pin change can fail on RemoveAll (pre-existing, all Docker builders).

## 7b. Python builder scripts (`recipes/builders/*.py`)

The Go pipeline now orchestrates every family; remaining Python scripts run in the tools Docker image.
- [x] `foraging-habitats-fi` removed from `fi-maps`: recipe has no URL and nothing reads its `builder:` field, so every finland-32+ apply ended with "1 item(s) failed".
- Port `foraging-habitats-fi-pmtiles.py` to a Go family (raster classify → vector → tippecanoe, like `vector-static`) or wire it as a Docker-script build step; then re-add to fi-maps. Input: LUKE MS-NFI 2023 (watch for a 2025 cycle).
- [x] Deleted dead code: `apply.go` `buildItem()` (never called), `wikipedia-compact-zim.py` (superseded by Go `zim-compact`), `media-zim.py` + its Dockerfile COPY (never invoked).
- `pinkka-sienet-fi-zim.py` has no recipe — add a Finnish mushroom-course recipe (fits fi-survival) or delete.
- Keep `web-video-zim.py` (backs `svalbard zim <url>`; depends on yt-dlp/libzim — Go port has low value).

## 8. Docs / housekeeping

- [x] README version badge v0.2.0 → v0.4.0.
- [x] Removed 13 stale `local/presets/custom-2026041*.yaml` (gitignored, not loaded by the Go CLI).
- [x] README preset table: default-512 claims "full-picture Wikipedia" (it has nopic; maxi arrives at 1tb). Rewrite table after rebalancing; update custom-preset section once §7 lands. — rewritten with measured sizes; custom-preset section added.
