# Svalbard: new content, data and map candidates (2026-11-21)

Research only; nothing in the repo was changed. Sources were checked against `recipes/{content,datasets,maps}`,
`presets/packs/*`, MASTERPLAN.md (Follow-up queue, Wishlist), `docs/plans/2026-09-30-catalog-update-audit.md`
§4–5, and older plans (`2026-03-30-deferred-sources-and-tools.md`, `2026-04-02-pack-tier-composition-plan.md`).

How things were checked:
- **Kiwix**: I downloaded the whole OPDS catalog (`https://library.kiwix.org/catalog/v2/entries?count=-1`,
  3,682 entries) and compared it with the 125 Kiwix filenames the recipes already use.
- **Verified = y**: an HTTP HEAD (following redirects) returned 200 and a Content-Length on 2026-11-21. Sizes
  are those byte counts (GB = 10^9).
- **Kiwix directory listings**: these still work under `download.kiwix.org/zim/<cat>/`, which redirects to
  `lb.download.kiwix.org`. Only the bare `/zim/` root now redirects to `hub.kiwix.org`. The resolver's date
  regex `(\d{4}-\d{2}[a-z]?)` already accepts suffixed dates such as `2026-08a`.
- **Effort**: **S** = recipe YAML plus a pack/preset entry, with an existing type (zim, pdf, direct download).
  **M** = needs a builder (pmtiles extract, GDAL, crawl, XML→sqlite). **L** = needs a new runtime capability.
- **Dup check**: **NEW** = not in the catalog and not in earlier plans. **RE-CONFIRM** = already proposed
  somewhere; the plan is named.

---

## Top-5 shortlist

| # | Candidate | Size | Effort | Why now |
|---|---|---|---|---|
| 1 | **Kiwix OSM map ZIMs**: `maps_en_nordics` (Europe/world for big default tiers) | 5.67 GB (Europe 26.7, world 77.7) | S | New Kiwix category in 2026: a vector OSM map with city search that kiwix-serve shows directly. Covers Finland, Sweden, Norway, Denmark, Faroe Islands and Iceland. Works today, whereas `osm-finland`/`osm-uusimaa` wait on the missing `osm-extract` Go handler. Region-neutral presets currently have no real basemap (only Natural Earth). |
| 2 | **Wikivoyage English** (`all` nopic/maxi; `europe` nopic for the 2 GB tier) | 0.27 / 1.07 GB (Europe 0.08) | S | Travel, phrasebooks, "stay safe" and "cope" sections, offline city guides. Only Finnish Wikivoyage (20 MB) is in the catalog. Good value for size in every tier. |
| 3 | **Swedish Wikipedia + Swedish Wiktionary** | top_nopic 0.53 · top_maxi 1.59 · all_nopic 7.61 · all_maxi 16.78 · wiktionary 0.51 GB | S | Swedish is Finland's second official language. An April plan listed `wikipedia-sv-*` for the Finland tiers, but no recipe was ever added. The November 2026 build is fresh. |
| 4 | **OpenStax nursing/medical PDF set** (Fundamentals of Nursing, Clinical Nursing Skills, Pharmacology, Med-Surg, Maternal-Newborn, Nutrition for Nurses; optionally A&P 2e and Microbiology) | ~0.40 GB core (+0.81 GB optional) | S (6–8 `type: pdf` recipes) | Deferred in March because no ZIM existed. Stable direct PDF URLs now come from the OpenStax CMS API. Practical nursing procedures and drug reference are exactly what `survival-medical` lacks. Live editions are now **CC BY-NC-SA 4.0**. |
| 5 | **Mapterhorn terrain (Finland/Nordics extract)** | planet 355.6 GB; Finland extract unmeasured, estimated 0.5–3 GB at maxzoom 10–11 | M | Already a candidate in the audit; re-confirmed here. The single planet PMTiles is now verified, with per-source licences: Finland NLS 2 m (CC BY 4.0), Sweden Lantmäteriet 1 m (CC0), Norway Kartverket 1 m (CC BY 4.0). `go-pmtiles` is already in `tools-base` for `pmtiles extract --bbox`. The viewer needs a raster-dem/hillshade layer. |

Close runners-up (S): the **CIA World Factbook final snapshot** (#6, the Factbook has been discontinued, so this
ZIM is now the archive copy) and the **DevDocs + MDN batch** (#8, unblocks the "programming pack tiers"
curation item).

---

## Full ranked list

### 1. Kiwix OSM map ZIMs (openZIM `maps2zim`): NEW
- **What/why**: OSM vector maps packed into a ZIM by openZIM's `maps2zim` (GPL-3 scraper), with city search,
  viewed in kiwix-serve. The catalog has 193 regional maps. No Finland-only file exists; Finland is inside
  `maps_en_nordics` ("Finland, Sweden, Norway, Denmark, Faroe and Iceland").
- **URLs (verified y)**:
  - `https://download.kiwix.org/zim/maps/maps_en_nordics_2026-08a.zim`: 5,668,621,465 B
  - `https://download.kiwix.org/zim/maps/maps_en_europe_2026-08.zim`: 26,704,567,346 B
  - `https://download.kiwix.org/zim/maps/maps_en_all_2026-06.zim`: 77,731,456,103 B
  - Also: `maps_en_sweden` 2.13 GB, `maps_en_norway` 4.21 GB, `maps_en_estonia` 0.27 GB
- **Recipe pattern**: `url_pattern: https://download.kiwix.org/zim/maps/maps_en_nordics_{date}.zim`. The
  resolver regex handles the `a` suffix.
- **License**: ODbL-1.0 (OpenStreetMap contributors). Check the style/font attribution in the ZIM metadata.
- **Pack**: `fi-maps` (finland-64+). Also a new region-neutral `world-maps` pack, or default-512 (Europe) and
  default-1tb/2tb (world).
- **Effort**: S. Open question: keep `osm-finland` (z15 PMTiles in Svalbard's own MapLibre viewer, plus overlay
  layers) or treat this as the "works now" basemap. They complement each other: this file is search-first,
  while the PMTiles viewer is where the overlays live.

### 2. Wikivoyage English: NEW
- **URLs (verified y)**:
  - `https://download.kiwix.org/zim/wikivoyage/wikivoyage_en_all_nopic_2026-09.zim`: 272,495,142 B
  - `…/wikivoyage_en_all_maxi_2026-09.zim`: 1,071,168,673 B
  - `…/wikivoyage_en_europe_nopic_2026-09.zim`: 83,536,915 B (fits default-2)
- **License**: CC BY-SA 4.0.
- **Pack**: `core` or `fi-reference`. Use nopic for 32/64 and maxi for 128+.
- **Effort**: S.

### 3. Swedish Wikipedia + Wiktionary (and small Swedish extras): RE-CONFIRM
- `2026-04-02-pack-tier-composition-plan.md` lists `wikipedia-sv-nopic` and `wikipedia-sv-all`; no recipe exists.
- **URLs (verified y)**:
  - `https://download.kiwix.org/zim/wikipedia/wikipedia_sv_top_nopic_2026-10.zim`: 534,604,693 B
  - `…/wikipedia_sv_top_maxi_2026-10.zim`: 1,591,761,200 B
  - `…/wikipedia_sv_all_nopic_2026-11.zim`: 7,612,178,417 B
  - `…/wikipedia_sv_all_maxi_2026-11.zim`: 16,781,607,280 B (mini: 3.97 GB)
  - `https://download.kiwix.org/zim/wiktionary/wiktionary_sv_all_nopic_2026-09.zim`: 506,790,912 B
  - Small extras: `gutenberg_sv_all_2026-11a` 284 MB, `phet_sv_all_2026-08` 101 MB, `wikisource_sv_all_nopic` 174 MB,
    and Northern Sámi `wikipedia_se_all_maxi_2026-10` 53 MB (verified y)
- **License**: CC BY-SA 4.0 (Gutenberg: public domain in the US; PhET: CC BY 4.0).
- **Pack**: `fi-reference`. Suggested tiers: finland-64 top_nopic, finland-256 top_maxi, finland-1tb all_nopic,
  finland-2tb all_maxi.
- **Effort**: S.

### 4. OpenStax nursing/medical PDFs: RE-CONFIRM (deferred 2026-03-30 as "no ZIM")
- **Source of URLs**: `https://openstax.org/apps/cms/api/v2/pages/?type=books.Book&fields=title,high_resolution_pdf_url,license_name,book_state&limit=250`
  returns 129 books with stable PDF URLs.
- **URLs (verified y, Last-Modified April 2026)**, all under `https://assets.openstax.org/oscms-prodcms/media/documents/`:
  - `Fundamentals_of_Nursing_-_WEB.pdf`: 96.6 MB
  - `Clinical-Nursing-Skills-WEB.pdf`: 95.7 MB
  - `Pharmacology-WEB.pdf`: 47.2 MB
  - `Medical-Surgical_Nursing-WEB.pdf`: 72.8 MB
  - `Maternal-Newborn_Nursing-WEB.pdf`: 60.8 MB
  - `Nutrition_for_Nurses-WEB.pdf`: 31.6 MB
  - Optional: `anatomy-and-physiology-2e_-_WEB.pdf` 476 MB, `microbiology_-_WEB.pdf` 333 MB
- **License**: **CC BY-NC-SA 4.0** for live editions. OpenStax switched from CC BY; retired first editions stay
  CC BY 4.0. Redistribution must keep the per-page "Access for free at openstax.org" attribution.
- **Pack**: `survival-medical` (nursing set, 64+). `sciences` for A&P and Microbiology. `computing` could take
  "Introduction to Python Programming".
- **Effort**: S per book (`type: pdf` already exists, as in `who-basic-emergency-care`). Pin by filename.
  Needs PDF indexing for search (follow-up item "Non-ZIM document indexing").

### 5. Mapterhorn terrain PMTiles (extract): RE-CONFIRM (audit §5 "Hillshade")
- **URL (verified y)**: `https://download.mapterhorn.com/planet.pmtiles`: 355,579,263,191 B, Last-Modified 2026-09-11.
  Attribution and licences: `https://download.mapterhorn.com/attribution.json` (151 sources).
- **Licences**: fi = NLS Finland 2 m DEM, CC BY 4.0; se = Lantmäteriet 1 m, CC0; no = Kartverket 1 m, CC BY 4.0;
  ee = Estonian open-data licence; global = Copernicus GLO-30 (free and open licence). Attribution must be
  listed per source.
- **Pack**: `fi-maps`. A Nordics bbox could sit in a future region-neutral maps pack.
- **Effort**: M. Run `pmtiles extract planet.pmtiles fi-terrain.pmtiles --bbox=19.5,59.0,32.0,70.1 --maxzoom=11`
  (go-pmtiles is in tools-base), plus a MapLibre `raster-dem` (terrarium) hillshade layer in the viewer. The
  size estimate is unmeasured: run `--dry-run` first. The smaller fallback is the kapsi `vinovalovarjoste`
  GeoTIFFs (`http://kartat.kapsi.fi/files/vinovalovarjoste/hila_{8,32,64,128,512}m/etrs-tm35fin-n2000/*.tif`,
  verified y, listing only).

### 6. CIA World Factbook, final snapshot: NEW
- **URL (verified y)**: `https://download.kiwix.org/zim/other/theworldfactbook_en_all_2026-02.zim`: 407,350,139 B.
  Kiwix describes it as "now-discontinued", so this snapshot will not be refreshed.
- **License**: public domain (US government work).
- **Pack**: `core` / `fi-reference`, every tier from 32.
- **Effort**: S.

### 7. US Army publications (armypubs.army.mil, zimit crawl): NEW
- **URL (verified y)**: `https://download.kiwix.org/zim/zimit/armypubs_en_all_2024-12.zim`: 8,232,396,997 B.
- **What**: crawl of the Army Publishing Directorate. It should contain the public-release ATP 3-50.21
  *Survival*, ATP 3-90.97 *Mountain Warfare and Cold Weather Operations*, and first-aid TCs. Contents were not
  inspected, and the build is from 2024-12, so it is stale.
- **License**: public domain (US government work, public-release publications).
- **Pack**: `fi-survival` / `survival-medical` 128+. It complements the existing `fas-military-medicine`.
- **Effort**: S. Note that direct PDFs on `irp.fas.org/doddir/army/*.pdf` return **202 JavaScript challenges**
  to curl, so they cannot be automated. The ZIM is the reliable route.

### 8. DevDocs batch + MDN: NEW (serves the Follow-up item "programming pack tiers")
- **DevDocs**: present now are c, cpp, go, javascript, python, rust. Missing and available (each under 50 MB,
  most under 5 MB): bash, git, docker, sqlite, postgresql, html, css, typescript, node, nginx, cmake, gcc,
  kubernetes, ansible, terraform, numpy, pandas, pytorch, zig, lua, kotlin, openjdk, react, vue, svelte, qt.
  Sampled `https://download.kiwix.org/zim/devdocs/devdocs_en_bash_2026-10.zim` (572 KB) and `devdocs_en_git_2026-10.zim`
  (1.6 MB): verified y.
- **MDN**: `https://download.kiwix.org/zim/zimit/developer.mozilla.org_en_all_2026-11.zim`: 587,508,281 B, verified y,
  built 2026-11-20.
- **Licenses**: DevDocs content uses each upstream project's licence; MDN prose is CC BY-SA 2.5 and its code
  samples are CC0/MIT.
- **Pack**: `computing`. Tier the DevDocs set into every tier (tiny), and add MDN at 64+.
- **Effort**: S (many small recipes; consider a single "devdocs-extra" pack entry list).
- **Also available**: `lua.org_en_all` 380 MB, `jeffe.cs.illinois.edu_en_all` (Erickson *Algorithms*) 517 MB,
  `opendatastructures.org` 116 MB, `gobyexample.com`, `htdp.org`. All verified or in the catalog.

### 9. English Wikipedia "mini" (all articles, lead section only): NEW
- **URL (verified y)**: `https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_mini_2026-09.zim`: 14,386,799,064 B.
- **Why**: covers every article. It sits between top-maxi (7 GB, top articles only) and all-nopic (52.7 GB),
  so it suits 64/128 tiers that cannot afford nopic. (The audit skipped the *Finnish* all_mini; English mini was
  never evaluated.)
- **License**: CC BY-SA 4.0. **Pack**: `fi-reference` / default-64 or default-128. **Effort**: S.

### 10. GeoNames gazetteer → sqlite FTS: RE-CONFIRM (audit §5)
- **URLs (verified y, regenerated daily)**:
  - `https://download.geonames.org/export/dump/FI.zip`: 13.9 MB
  - `…/cities500.zip`: 13.9 MB
  - `…/allCountries.zip`: 422.6 MB
- **License**: CC BY 4.0.
- **Pack**: `fi-maps` (FI plus neighbours), default (cities500). It gives offline "where is X" lookup without
  the map viewer.
- **Effort**: M (`reference-static` family like `fimea`, with a TSV loader). Pin by snapshot date and sha256.

### 11. SYKE lake depth areas/contours + river network: RE-CONFIRM (audit §5, URLs now found)
- **URLs (verified y)**:
  - `https://wwwd3.ymparisto.fi/d3/gis_data/spesific/syvyysalue.zip`: 217.0 MB (2016)
  - `…/syvyyskayra.zip`: 120.3 MB (2016)
  - `…/uomaverkosto.zip`: 75.1 MB (2026-03)
- **License**: CC BY 4.0 (SYKE).
- **Pack**: `fi-maps`. **Effort**: M (`vector-static` family, the same pattern as `pohjavesialueet`).

### 12. Species reference: Wikispecies (+ iNaturalist taxa, FinBIF checklist): NEW / RE-CONFIRM
- **Wikispecies (NEW)**: `https://download.kiwix.org/zim/other/wikispecies_en_all_nopic_2026-10.zim`: 1,669,551,465 B
  (maxi 3.55 GB). Verified y. CC BY-SA 4.0.
- **FinBIF checklist 2025 (RE-CONFIRM, audit)**: PDF `https://cdn.laji.fi/files/checklists/2025/Lajiluettelo2025_Checklist2025.pdf`,
  2.66 MB, verified y. FinBIF default licence is CC BY 4.0. No xlsx at the guessed path (404); a structured
  export needs the API.
- **iNaturalist taxa (NEW)**: `https://inaturalist-open-data.s3.amazonaws.com/taxa.csv.gz`: 40.2 MB, verified y
  (2026-10-27). CC0 metadata. Photos are licensed per photo; the full metadata tarball is 36.7 GB and is skipped.
- **Pack**: `sciences` / `homesteading` 128+, with the `foraging-habitats-fi` / Pinkka builder as companions.
- **Effort**: S for the Wikispecies ZIM and the FinBIF PDF. M for a taxa → sqlite FTS name lookup
  (scientific/Finnish/Swedish names).

### 13. Canadian Prepper sibling video ZIMs: NEW (same channel as the existing `canadian-prepper-winter`)
- **URLs (verified y)**:
  - `https://download.kiwix.org/zim/videos/canadian-prepper_en_preppingfood_2026-10.zim`: 2,194,586,332 B
  - `…/canadian-prepper_en_bugoutroll_2026-10.zim`: 879,089,689 B
  - `…/canadian-prepper_en_bugoutconcepts_2026-08.zim`: 2,886,219,997 B
- **License**: creator content packaged by Kiwix (same terms as the existing recipe).
- **Pack**: `fi-survival` 256+ (`preppingfood` first). **Effort**: S.

### 14. Reticulum manual (PDF): RE-CONFIRM (commented TODO in `communications.yaml`)
- **URL (verified y)**: `https://reticulum.network/manual/Reticulum%20Manual.pdf`: 6.7 MB.
- **License**: "Reticulum License". It is MIT-style but adds two clauses: no use in systems built to harm
  people, and **no use in AI/ML training datasets**. Local RAG/semantic indexing is not training, but record
  this in the recipe's licence notes and consider excluding it from any export.
- **Pack**: `communications`. **Effort**: S (`type: pdf`). The version is not pinned in the filename, so pin a
  sha256.

### 15. Meshtastic documentation: RE-CONFIRM (commented TODO, deferred-sources plan)
- **Source**: `github.com/meshtastic/meshtastic` (Docusaurus site and docs; GPL-3.0; pushed 2026-11-16). No Kiwix ZIM exists.
- **Pack**: `communications` (and `esp32-dev` overlap: Meshtastic runs on ESP32 LoRa boards).
- **Effort**: M. Either build the Docusaurus site from a pinned tag and run warc2zim, or crawl meshtastic.org
  with the existing `wget` + `warc2zim` steps pattern (as in `root-simple.local.yaml`). Estimated 50–100 MB
  (figure from the pack TODO).

### 16. Nordic civil-preparedness guides: NEW (license check needed)
- **Sweden MCF "In case of crisis or war"** (English, publication MCF0180, **June 2026**):
  `https://rib.msb.se/filer/pdf/31523.pdf`, 4.26 MB, verified y (downloaded, title page read). Many other
  languages are linked from mcf.se.
- **Finland "72 tuntia" household guide** (SPEK, 2025-01): `https://72tuntia.fi/wp-content/uploads/2025/01/spek_72_tuntia_varautuminen_kotona_2025_01_21_v01.pdf`,
  2.9 MB, verified y.
- **Finland Suomi.fi "Häiriö- ja kriisitilanteisiin varautuminen"** (Interior Ministry, Nov 2024): a JS web
  guide at `suomi.fi/oppaat/varautuminen`; no PDF found (a 15-language summary PDF exists but its URL was not
  found).
- **License**: none stated. These are public-authority and NGO brochures intended for every household; neither
  has a CC licence. Personal vaults are low risk, but redistribution needs confirmation from MCF and SPEK.
- **Pack**: `fi-survival` (all tiers, tiny). **Effort**: S for the PDFs. M/L for the Suomi.fi guide, which is
  an SPA and needs zimit/a browser crawl because wget will not do.

### 17. Finlex consolidated legislation (Finland): NEW
- **URLs (verified y, regenerated daily)**:
  - `https://www.finlex.fi/files/open-data/archives/statute-consolidated.zip`: 4,415,533,481 B
  - `…/statute-foreign-language-translation.zip`: 160,940,406 B (unofficial English/Swedish translations)
- **What**: Akoma Ntoso XML plus attachments and PDFs. The text-only extraction will be far smaller than the zip.
- **License**: CC BY 4.0, with a privacy-use restriction.
- **Pack**: `fi-reference` (finland-128+; translations at 64). Useful laws include the Emergency Powers Act,
  Rescue Act, Everyman's rights, Hunting Act and Water Act.
- **Effort**: M (XML → sqlite FTS or HTML → ZIM; keep the translations as a cheap first step).

### 18. Overture places (Finland/Nordics extract): RE-CONFIRM (audit §5)
- **URL (verified y)**: `https://overturemaps-extras-us-west-2.s3.amazonaws.com/tiles/2026-11-18.0/places.pmtiles`:
  20,234,219,226 B (the 2026-10-21.0 release is also live; addresses.pmtiles is 32.9 GB).
- **License**: places CDLA-Permissive-2.0. Overture labels these tiles "for inspection, not production", so
  prefer building from GeoParquet for a stable schema (v2.0.0 in Oct 2026; `categories` was replaced by `taxonomy`).
- **Pack**: `fi-maps`. **Effort**: M (pmtiles extract by bbox and a viewer POI layer). Only the latest releases
  are kept, so mirror the extract.

### 19. PhET English: NEW
- **URL (verified y)**: `https://download.kiwix.org/zim/phet/phet_en_all_2026-08.zim`: 108,981,169 B.
- **License**: CC BY 4.0. **Pack**: `sciences` / default (every tier; only `phet-fi` exists). **Effort**: S.

### 20. TED energy talks: correction to the audit
- The audit skipped TED "solar" as astronomy. There is a separate **`ted_mul_solar-energy`**
  (`https://download.kiwix.org/zim/ted/ted_mul_solar-energy_2026-09.zim`, 540,182,341 B, verified y). Also
  available: `ted_mul_wind-energy` 174 MB and `ted_mul_renewable-energy` 1.07 GB.
- **License**: CC BY-NC-ND 4.0 (TED). **Pack**: `homesteading` 256+. **Effort**: S. These are lectures, so
  practical value is low to medium.

### 21. ESP32 hardware reference + ESP-IDF docs: NEW (for `esp32-dev`, which has no docs today)
- **ESP32 TRM**: `https://www.espressif.com/sites/default/files/documentation/esp32_technical_reference_manual_en.pdf`, verified y (200, application/pdf).
- **ESP32 datasheet**: `https://www.espressif.com/sites/default/files/documentation/esp32_datasheet_en.pdf`, verified y.
- **ESP-IDF programming guide** matching the pinned `framework-espidf 4.60100.0` (IDF v6.1): no PDF found at
  the usual `docs.espressif.com/.../esp-idf-en-vX-esp32.pdf` paths (404).
- **License**: the TRM and datasheet are Espressif copyright, freely downloadable but with no redistribution
  grant (confirm). ESP-IDF docs are Apache-2.0 (in-repo `docs/`).
- **Pack**: `embedded/esp32-dev`. **Effort**: S for the PDFs. M for IDF docs (Sphinx build from the pinned IDF
  tag, or a wget crawl of `docs.espressif.com/projects/esp-idf/en/v6.1/esp32/`, then warc2zim).

### 22. USDA FoodData Central + Fineli: RE-CONFIRM (audit §5)
- **SR Legacy (verified y)**: `https://fdc.nal.usda.gov/fdc-datasets/FoodData_Central_sr_legacy_food_csv_2018-04.zip`, 6.07 MB.
  Final release; public domain/CC0.
- **Foundation Foods**: the current filename was not found because the download page is JS-rendered. The
  page lists a Dec 2025 release about 3.4 MB.
- **Fineli** (THL, CC BY 4.0): fineli.fi returns **403 to automated clients**, so no URL was verified. A
  browser fetch or a contact with THL is needed.
- **Pack**: `fi-survival` (replaces the stale `usda-nutrition`, whose recipe is actually the 2015 canning guide).
- **Effort**: M (`reference-static` sqlite FTS).

### 23. Norway Kartverket N50 topographic vectors: NEW (archive plans mention it; no recipe)
- **URL (verified y)**: `https://nedlasting.geonorge.no/geonorge/Basisdata/N50Kartdata/FGDB/Basisdata_0000_Norge_25833_N50Kartdata_FGDB.zip`: 4,078,160,550 B.
  Weekly updates.
- **License**: CC BY 4.0 (Kartverket).
- **Pack**: a future Nordic maps pack. **Effort**: M (GDAL → tippecanoe through `vector-static`).
- **Sweden equivalent**: Lantmäteriet Topografi 50 is CC0 but requires a Geotorget account, so it has no
  stable anonymous URL. Not viable for now; `maps_en_nordics` covers Sweden with OSM.

### 24. Open textbooks: LibreTexts K-12 + SUNY Milne: NEW
- **URLs (verified y)**:
  - `https://download.kiwix.org/zim/libretexts/libretexts.org_en_k12_2026-01.zim`: 369,809,249 B
  - `https://download.kiwix.org/zim/zimit/milneopentextbooks.org_en_all_2026-11.zim`: 1,630,353,553 B
- **License**: mixed per book (mostly CC BY / CC BY-NC-SA).
- **Pack**: `sciences` / education tiers 128+. **Effort**: S.

---

## Smaller items seen (not ranked)
- **Stack Exchange not yet included**: retrocomputing 126 MB (the audit deferred it), aviation 511 MB,
  travel 462 MB, space 400 MB, pets 87 MB, fitness 80 MB, parenting 60 MB, lifehacks 49 MB, money 254 MB,
  law 187 MB, stats 1.64 GB, datascience 273 MB, ai 105 MB (possibly for the AI pack). All refreshed
  2026-10-19/20. Effort S.
- **Small practical extras**: `rabbitwaves.ca` (useful knowledge as comics, 71 MB),
  `publicdomainrecipes.com` (17 MB, public-domain recipes), `foss.cooking` (30 MB),
  `cloudflare.com_en_learning-center` (191 MB, networking/security primer, Dec 2025).
- **Big education**: `crashcourse_en_all` 23 GB (2026-05) for 1tb/2tb education tiers.
- **WikiMed staleness**: `mdwiki_en_all_maxi` (the `wikimed` recipe) was last built **2025-11**.
  `wikipedia_en_medicine_maxi_2026-10` (1.86 GB, verified y) is fresher, should WikiMed stall.
- **Protomaps world basemap**: the daily planet build `https://build.protomaps.com/20261120.pmtiles` is
  139.7 GB (verified y). A z0–6 world extract would give default presets a small basemap. It needs the same
  missing `osm-extract` Go handler as `osm-finland`, so item 1 is the quicker way to get a region-neutral map.
  Note that `build-metadata.protomaps.dev/builds.json` returned 400 today, so the audit's "pin from
  builds.json" plan needs a recheck.

## Checked and not viable (or blocked)
- **MSF Clinical/Essential Drugs guidelines**: all rights reserved, so no redistribution. The official app is free.
- **Hesperian "Where There Is No Doctor"** (the `survival-medical` TODO): Hesperian's Open Copyright Policy
  allows non-commercial copying, but **any digital-format distribution needs permission**
  (permissions@hesperian.org, per the 2025 front matter). Ask first. `zimgit-medicine` may already bundle an
  older edition; check before asking.
- **WHO IRIS PDFs** (Pocket Book of Hospital Care for Children, MCPC, Surgical Care at the District Hospital):
  `iris.who.int/bitstream/...` returns an HTML challenge to curl. Use a stable who.int mirror path, as the
  existing `who-basic-emergency-care` does with hlh.who.int. Licence is CC BY-NC-SA 3.0 IGO. Not verified.
- **IFRC First Aid Guidelines 2025**: ifrc.org returns 403 to curl. Not verified.
- **Kotus Kielitoimiston sanakirja**: not in Kotus's CC BY 4.0 open-data list. Only older dictionaries (Old
  Literary Finnish, Karelian XML) are open.
- **Metsähallitus huts/lean-tos/fire sites**: no open download found. One Metsähallitus dataset is explicitly
  restricted to administration use. OSM survival POIs (audit) remain the route.
- **Direct irp.fas.org PDFs** (Army survival/cold-weather manuals): blocked by a 202 JavaScript challenge. Use
  the armypubs ZIM (item 7).

## Notes for the catalog
- **Discovery**: the Kiwix OPDS endpoint `https://library.kiwix.org/catalog/v2/entries?count=-1` returns the
  full catalog (name, flavour, language, category, size, date, URL) in one 4.4 MB XML. It is a ready mechanism
  for the Follow-up item "Online version discovery" and for size-drift checks.
- **Licence changes worth recording**: OpenStax live editions moved to CC BY-NC-SA 4.0. Reticulum's licence
  adds a no-AI-training clause.
