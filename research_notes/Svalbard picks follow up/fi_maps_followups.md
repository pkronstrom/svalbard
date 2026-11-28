# Finland maps follow-ups: address search, nautical charts, orthophoto, hillshade

Checked 2026-11-26. This file adds to `../Svalbard catalog new sources/finland_open_data.md` and `maps_and_ai.md` and does not repeat them. "Measured" means I ran the request or computation from this machine on 2026-11-26. "Estimate" marks arithmetic built on measured samples. "Unverified" marks claims I could not confirm.

## Read this first: two findings that change existing assumptions

1. **The kapsi TMS tile service is gone.** `https://tiles.kartat.kapsi.fi/{ortokuva,peruskartta,taustakartta}/{z}/{x}/{y}.jpg` returns `307` to `https://kapsi-tiles.y2k.fi/...`. That host's TLS certificate covers only `kartat-ng.kapsi.fi` and `tiles.kartat.kapsi.fi`, so the redirect fails certificate validation. Over plain HTTP it returns `410 Gone` with the body: *"Resource is no longer available. Please use MML's public service for the moment as we don't have enough human resources for maintaining this properly."* `https://tiles.kartat.kapsi.fi/maastokartta/...`, the URL in the `mml-maastokartta` recipe, returns `404`. (Measured.) The kapsi **file** mirror (`http://kartat.kapsi.fi/files/...`) still works. The earlier note in `maps_and_ai.md` (orthophoto "plugs straight into the existing `raster-tms` builder") is therefore wrong twice: the TMS is dead, and `raster-tms` has no Go handler anyway (`docs/plans/2026-11-21-pack-operations-inventory.md:70`).
2. **DVV stopped publishing building addresses.** The last open DVV extract is dated 3.2.2025, and distribution ended on 14.3.2025. The national open address set now comes from SYKE's Ryhti register, under CC BY 4.0, served as OGC API Features (details in §1).

## 1. Offline address search for Finland

### Facts

**Ryhti open addresses (SYKE), the current national address register**
- OGC API Features collection `open_address` at `https://paikkatiedot.ymparisto.fi/geoserver/ryhti_building/ogc/features/v1/collections/open_address/items`. `numberMatched` is **3,873,864** (measured). Each record has a point (CRS84/EPSG:3067), `address_name_fin`/`_swe`, house number parts and letter, `postal_code`, `postal_office_fin`/`_swe`, `municipality_number` and `building_key`. Records carry `modified_timestamp_utc` values up to 2026-11-17, so the register is live. — [CKAN rakennetun-ympariston-tietojarjestelman-rakennusten-osoitetiedot (cc-by-4.0)](https://avoindata.suomi.fi/data/fi/dataset/rakennetun-ympariston-tietojarjestelman-rakennusten-osoitetiedot)
- It accepts `f=text/csv`, `limit=100000` and `startIndex` paging without a key. A 100 k-row page at offset 3.8 M took 4.4 s, and CSV averages about 274 bytes per row, so a full pull is about **1.06 GB of CSV in 39 requests** (measured). The WFS `CountDefault` is 100 000. There is also an INSPIRE AD collection at `.../ryhti_inspire_ad/ogc/features/v1` (`AD.Address`).
- OpenAddresses' Finland source (`sources/fi/countrywide-fi.json`) is this same SYKE `open_address` table. It is cached as `open_address.csv.zip`, **364,013,895 bytes**, last modified 2026-10-09, licence CC BY 4.0, attribution "Syke" (measured with a range GET). — [OA source](https://github.com/openaddresses/openaddresses/blob/master/sources/fi/countrywide-fi.json)
- The last DVV extract is still on Paituli: `https://nic.funet.fi/index/geodata/dvv/osoitteet/2025/osoiteet_2025.{shp,dbf,...}` (shp 105 MB, dbf 832 MB), 3,872,156 buildings, extracted 3.2.2025, CC BY 4.0 (`DVV_ehdot_CC.txt`). Its readme says: *"Rakennusten osoitetiedot ja äänestysalue - aineiston jakelu päättyy 14.3.2025."* Treat it as frozen. — [funet dvv/osoitteet/2025](https://nic.funet.fi/index/geodata/dvv/osoitteet/2025/)

**OSM addresses (for comparison)**
- Overpass found **1,113,868** Finnish objects with `addr:housenumber` (153,481 nodes, 959,046 ways, 1,341 relations) on 2026-11-26 (measured). That is about 29 % of Ryhti's count.
- The Photon Finland dump (`photon-dump-finland-1.0-latest.jsonl.zst`, 244.4 MB, already in `maps_and_ai.md`) decompresses to **4.17 GB**, has **1,904,425** JSONL lines, and **1,122,185** of them contain `housenumber`. Its data timestamp is 2026-11-21 (measured).
- The Geofabrik Finland PBF is **778,227,827 bytes**, last modified 2026-11-25 (measured).

**MML addresses**
- MTK carries addresses only as road-segment address ranges ("tiestö osoitteilla"), not address points (community post; the attribute list is unverified). — [OSM community thread](https://community.openstreetmap.org/t/gdal-tukee-maastotietokannan-mtk-gml-tiedostomuotoa/64497/53)
- The MML place-name gazetteer (Nimistö, ~800 k names, CC BY 4.0, kapsi) is already listed in `finland_open_data.md` #6. Combine it with the address table to get place, village and island search.

**Photon** (version, Java and dump are in `maps_and_ai.md`). The newest additions:
- `photon-1.3.0.jar` is 98.2 MB. Photon is built on OpenSearch, with the server embedded. The README's planet guidance is 95 GB of disk and "at least 64GB RAM"; it suggests `-Xmx8G`-style heaps for smaller setups. It publishes no RAM figure for a country import. — [README](https://raw.githubusercontent.com/komoot/photon/master/README.md); [releases](https://github.com/komoot/photon/releases)
- Unverified: a Finland index probably runs in a 1–2 GB heap with a few GB of disk. I did not run it.

**Nominatim**: needs PostgreSQL 12+ (13+ recommended), PostGIS 3+ and osm2pgsql 1.8+, and "a minimum of 2GB of RAM is required". — [Nominatim install docs](https://nominatim.org/release-docs/latest/admin/Installation/)

**Pelias**: needs Node.js LTS and Elasticsearch 7 or 8, and libpostal "requires about 4GB of disk space". It is normally run from Docker images. — [Pelias requirements](https://github.com/pelias/documentation/blob/master/requirements.md)

**Runtime fit**: the drive runtime already links `github.com/ncruces/go-sqlite3` with FTS5 (`drive-runtime/internal/mcp/search.go:22`). The map server is a plain `http.FileServer` over the drive root (`drive-runtime/internal/maps/maps.go:26`), so a geocoder needs one more handler there.

### Measured SQLite size

I built a prototype from an 83,864-row Ryhti sample (random, because the API orders by UUID) with Python sqlite3. The table held street_fi, street_sv, number, postcode, post office fi/sv, municipality code, lon and lat:

| Variant | Bytes/row | Extrapolated to 3.87 M rows |
|---|---|---|
| table only | 67 | ~258 MB |
| + FTS5 external-content index on street/num/post office, no prefix index | 104 | **~403 MB** |
| + FTS5 with `prefix='2 3'` | 130 | ~505 MB |

**Full build (measured)**: I paged all of Ryhti as CSV (39 requests, 170 s, 1.0 GB). The pull returned 3,873,864 rows with as many unique `address_key` values, so offset paging was stable. The SQLite file has a **260 MB table and is 387 MB with the FTS5 index** (no prefix index). It holds 259,439 distinct street+municipality pairs. Example queries:

| Query | Time |
|---|---|
| `kauppakatu 1 kuopio` | 2.9 ms |
| `aleksanterinkatu 52` | 0.4 ms |
| `ulvilan*` | 0.7 ms |
| `mannerheimintie 1*` (short prefix, no prefix index) | 160 ms |

Ranking needs work. `aleksanterinkatu 52` returned Oulu before Helsinki, so a population or municipality boost, or results grouped by town, would help. Gzipped, the sample DB shrank only ~50 %, so ship it uncompressed.

### Inferences

| Option | Install/runtime cost on the stick | Data | Verdict |
|---|---|---|---|
| Photon + Finland dump | JRE 21 (~50–200 MB per platform) + 98 MB jar + an OpenSearch index built at install time; a JVM on every host | OSM only (~1.1 M addresses, plus POIs and places) | Good POI and place search, poor address coverage; adds Java to the runtime |
| Nominatim | PostgreSQL + PostGIS + Python | OSM | Too heavy for a stick |
| Pelias | Node + Elasticsearch + libpostal (4 GB) | OSM + OA + WOF | Too heavy |
| **SQLite FTS5 in the Go runtime** | ~0.4–0.5 GB file, no new binary | Ryhti 3.87 M official addresses + MML place names (+ optionally OSM POI names from the Photon dump) | **Recommended** |
| SQLite in the browser (sql.js-httpvfs / wa-sqlite with HTTP range reads) | same file, no server code | same | Fallback if the server endpoint is unwanted; slower and more JS to vendor |

**Recommendation: a SQLite FTS5 gazetteer served by the Go runtime, size M.** The builder pages Ryhti CSV (or the OA zip as a mirror) and the MML place-name GML into one `fi-geocode.sqlite`. The runtime adds `/api/geocode?q=` to the maps server: an FTS5 query with a prefix match on the last token, ranked by bm25 and then by "has house number", returning GeoJSON. The map viewer gets a search box that flies to the result. Reverse geocoding can use an R*-tree or a simple lat/lon grid index. It stays an M unless the user wants fuzzy matching (typos, inflected forms such as "Mannerheimintiellä"). That needs a trigram tokenizer, which roughly doubles the index, or a Finnish stemmer, and becomes an L.

## 2. Traficom nautical charts

### Facts: what is open

**Raster charts (WMTS), the main find**
- `https://julkinen.traficom.fi/rasteripalvelu/wmts?request=getcapabilities` (keyless, 200, measured) serves **48 layers** of scanned/rendered paper charts: chart folios A–V (each with "erikoiskartat"), coastal charts, harbour charts, leisure (veneily) charts, general charts 1:100k/250k, and the combined `Traficom:Merikarttasarjat public`. The tile matrix sets are `WGS84_Pseudo-Mercator` (levels 0–18), `ETRS89_TM35-FIN` (0–15) and `EPSG:3395_FTA`, served as PNG. The REST template is `.../wmts/rest/Traficom:<layer>/{style}/WGS84_Pseudo-Mercator/WGS84_Pseudo-Mercator:{z}/{y}/{x}?format=image/png`.
- **Licence, quoted from the capabilities `AccessConstraints`**: *"This dataset produced by the Finnish Transport Agency is licensed under a Creative Commons Attribution 4.0 International License … 'Source: Finnish Transport Agency. Not for navigational use. Does not meet the requirements for official nautical charts.'"* (FI: "Lähde: Traficom. Ei navigointikäyttöön. Ei täytä virallisen merikartan vaatimuksia."). `Fees: NONE`. (Measured.)
- Traficom's download-service page lists this WMTS among "background data that are not available for download" in the download basket. A secondary summary also says it is "designed as a background map for general non-commercial applications", but I could not load that page (404 to WebFetch), so the quote is unverified. — [download-and-view-service](https://traficom.fi/en/transport-system/geoinformationsmaterial/download-and-view-service)
- Sampled tile sizes from `Merikarttasarjat public` (12 tiles per zoom, measured): z12 ~95 KB, z13 ~70 KB, z14 ~40 KB, z15 ~11 KB (upscaled), z16 returns 400. Blank or out-of-coverage tiles are 788 B. On 11 tiles, WebP q80 came to **32 %** of the PNG bytes and lossless WebP to 71 %.

**S-57 view WMS**: `https://julkinen.traficom.fi/s57/wms` (portrayal only). Its constraints are CC BY 4.0 with the same "Not for navigational use. Does not meet the requirements for appropriate nautical charts." notice (measured). The S-57 cells themselves are **not** open: "S-57 vector data intended for navigation is licensed and distributed through Traficom's partner Primar"; S-57/S-101/S-102 testing products are free but "not intended for navigation or commercial use", by agreement. — [access-hydrographic-data-and-licenses](https://traficom.fi/en/hydrography/hydrographic-data-services/access-hydrographic-data-and-licenses)

**Open WFS (CC BY 4.0)**: `https://julkinen.traficom.fi/inspirepalvelu/avoin/wfs`. It has 37 feature types (measured): chart product division (`tuotejako_*`), territorial sea, EEZ, berths, pontoons, dams, gates, shoreline constructions, pilot boarding places, VTS areas, navigational warnings, inland reference levels, D-areas and meeting/overtaking prohibition areas. **It has no depths, fairways or navaids.**

**Restricted WFS/WMS (keyless; hydrographic licence)**: `https://julkinen.traficom.fi/inspirepalvelu/rajoitettu/wfs` has 14 types (measured):
- feature counts: `DepthArea_A` **149,457**, `DepthContour_L` **201,767**, `Sounding_P` **270,002**, `UnsurveyedArea_A` 232
- other types: TSS parts, deep-water and two-way routes, precautionary area, dredged area
- output: GeoJSON is available, in EPSG:3067; `CountDefault` is 1,000,000

**Hydrographic data licence** (full text is in the restricted services' `AccessConstraints`, measured):
- Grant, quoted: *"data … can for other than navigational purposes freely be: copied, distributed and published, modified and utilised non-commercially, combined with other products and, used as part of an application or service. Navigational purposes refer to all maritime operations, such as route planning, positioning, determining the best direction and ensuring the vessel's safe passage."*
- Duties: name the Licensor, the data and the transfer time (example: *"contains information received from the Finnish Transport and Communications Agency's nautical chart data base 06/2020"*); include the licence or a link to it; do not imply endorsement.
- Also on [use-and-licences-data](https://traficom.fi/en/transport-system/geoinformationsmaterial/use-and-licences-data).

**1 m bathymetric GeoTIFF**: open only for the EEZ sea area and the Vuoksi and Kymijoki water systems, via the Oskari basket (emailed link, shapefile/GeoTIFF). It is under the "limited hydrographic data license" and "may not be used for navigation purposes or commercial use". — [bathymetric-data](https://traficom.fi/en/hydrography/hydrographic-data-services/bathymetric-data); [Oskari](https://julkinen.traficom.fi/oskari/?lang=en)

**Fairways and navaids (Väylävirasto, CC BY 4.0)**: `https://avoinapi.vaylapilvi.fi/vaylatiedot/ows` (keyless WFS/WMS; capabilities say CC BY 4.0). The CKAN entries all state "Aineisto ei sovellu navigointikäyttöön", and several say the data is refreshed daily. — [CKAN e.g. merenkulun-turvalaitteet-vesivaylaaineisto](https://avoindata.suomi.fi/data/fi/dataset/merenkulun-turvalaitteet-vesivaylaaineisto)

`vesivaylatiedot:*` counts with GeoJSON bytes estimated from 500-feature samples (measured):

| Layer | Features | Est. GeoJSON |
|---|---|---|
| `turvalaitteet_uusi` (navaids) | 35,176 | 54 MB |
| `navigointilinjat_uusi` | 12,887 | 10 MB |
| `vaylaalueet_uusi` (fairway areas) | 3,810 | ~5 MB* |
| `vaylat_uusi` | 1,950 | 3 MB |
| `valosektorit_uusi` | 6,636 | 2 MB |
| `vesiliikennemerkit_uusi` | 5,694 | 4 MB |
| `loistot_uusi` (lights) | 4,891 | 3 MB |
| `taululinjat_uusi` | 2,632 | 2 MB |
| `rajoitusalue_a_uusi` | 1,456 | 24 MB |
| `vesivaylien_nopeusrajoitusalueet` | 914 | 10 MB |
| `ruoppausalueet_uusi` | 1,421 | 3 MB |

\*The first 500 polygons may be smaller than the average.

**Bulk files**: none. Paituli's `vayla/` holds only Digiroad and a 2018 terms PDF ([funet vayla](https://nic.funet.fi/index/geodata/vayla/)). kapsi has no Traficom data. The CKAN entries list WFS/WMS only.

**OpenSeaMap/OSM**: 52,339 `seamark:type` nodes inside Finland's OSM boundary (Overpass, measured). This is ODbL, and the pipeline notes are in `maps_and_ai.md`.

### Estimates

- **Vector dump, whole coast**: Väylä navaids, fairways and restrictions come to about **120 MB of GeoJSON**. Traficom depth areas (~1.45 GB GeoJSON at 9.7 KB/feature), contours (~0.40 GB) and soundings (~0.09 GB) come to about **1.9 GB**. Tiled to vector PMTiles at z6–14, the result is probably 0.3–0.6 GB (unverified; tippecanoe ratio assumed). Requests: about 1 M features in ~10 WFS pages for Traficom, and one page per Väylä layer.
- **Raster pyramid of `Merikarttasarjat public`**: tiles whose centre falls in the coastal, folio, leisure, harbour and inland chart footprints (measured from `tuotejako_*` polygons): z13 **22,205**, z14 **88,738**, z12 ~5.6 k, z10–11 ~5.3 k (with general charts). That is about **122 k tiles for z0–14**, or ~5–6 GB as PNG and **~1.7–2 GB as WebP q80**. z15 adds ~355 k tiles that are mostly upscaled, so it is not worth it.
- **Legality**: the WMTS capabilities grant CC BY 4.0 with a mandatory "not for navigational use" notice, so copying and redistributing tiles is licensed. I found no published rate limit or bulk-harvest prohibition. The "background data, not downloadable" wording means the WMTS is not in the download basket, not that copying is forbidden. Harvest politely (low concurrency, a User-Agent with contact details), or email copyright@traficom.fi first. The depth layers are a different matter: they fall under the hydrographic licence, which allows only **non-commercial modification**. That fits a personal vault, but every derived PMTiles must carry the Traficom notice, the dump month and the licence link. Svalbard should fetch them at build time on the user's machine rather than redistribute prebuilt tiles.

### Recommendation

Build two items, each size M:
- `traficom-charts-raster`: WMTS → PMTiles z0–14 WebP, coast plus Saimaa folios, ~2 GB, CC BY 4.0 with the notice.
- `vayla-fairways-navaids`: Väylä WFS → vector PMTiles, ~50–100 MB, CC BY 4.0.

Make depth areas and contours a separate opt-in item (M) because of the hydrographic licence. A raster-WMTS builder (Go, bbox and zoom, throttled) would also serve MML orthophoto (§3). It is the missing `raster-tms` family under a new name, so build it once.

## 3. MML orthophoto

### Facts

- **kapsi files**: `http://kartat.kapsi.fi/files/orto/etrs-tm35fin/` (also `ftp://tiedostot.kartat.kapsi.fi/orto`, `rsync://tiedostot.kartat.kapsi.fi/mml/orto`; rsync timed out from here). Subfolders: `mara_v_25000_50` (colour, 0.5 m), `mara_vv_25000_50` (false-colour/CIR, 0.5 m), `mara_v_20000_50`, `mara_mv_*` (B/W, older) and `mavi_*`. Files are JPEG2000, organised as `<year>/<100k sheet>/<NNm>/1/<sheet>.jp2`; a 1:10 000 sheet is ~100–108 MB. The kapsi front page lists "Väriorto" and "Väri-infra" updated 06.11.2025. — [kartat.kapsi.fi](https://kartat.kapsi.fi/); [listing](http://kartat.kapsi.fi/files/orto/etrs-tm35fin/mara_v_25000_50/)
- **Data volume (crawled, measured)**: `mara_v_25000_50` holds **22,778 JP2 files, 2.28 TB** across flight years 2008–2026. The newest image per sheet is **10,332 sheets, 1.07 TB**, which is national 0.5 m coverage. A single year runs 1.2–182 GB (2025: 1,616 files, 170 GB).
- **The kapsi TMS is dead** (see the top of this file).
- **MML open WMTS** (`avoin-karttakuva.maanmittauslaitos.fi`) needs an API key; requests without one return 401 (measured). It serves "Orthophoto (most recent)" as JPEG at Pseudo-Mercator **levels 0–16**, and "More detailed zoom levels require an agreement". It is "intended for testing the interface and for small-scale use", and under load "some requests will be rejected (503)". — [MML WMTS page](https://www.maanmittauslaitos.fi/en/node/11151)
- **Tile size proxy**: MML tiles could not be sampled (no key, kapsi TMS dead, Helsinki's WMTS returned 403). I used swisstopo SWISSIMAGE rural tiles (20 per zoom, measured): JPEG ~21.5–24 KB per 256 px tile at z12–17, and **WebP q75 recompressed ~11.6–13.9 KB**. Finnish forest and fields look similar, and sea tiles are far smaller.

### Estimates (tile counts measured, bytes estimated)

| Area / zooms | Tiles | JPEG | WebP |
|---|---|---|---|
| Uusimaa recipe bbox (23.8,59.8,26.8,61.7), z0–16 | 517,924 | ~8.5–11 GB | **~5–6 GB** |
| same bbox, z0–17 | 2,066,705 | ~35–44 GB | **~20–24 GB** |
| Uusimaa land only (~9,100 km², clipped to the region polygon), z0–16 | ~135 k | ~3 GB | **~1.7 GB** |
| Uusimaa land only, z0–17 | ~535 k | ~11.8 GB | **~6.5 GB** |
| Finland (bbox 883 k tiles, ~50 % is Finland), z0–14 | ~440 k | ~10.5 GB | **~6 GB** |
| Finland, z0–15 | ~1.76 M | ~40 GB | **~23 GB** |

At 60.5°N, z16 is 1.18 m/px and z17 is 0.59 m/px, so z17 is the native 0.5 m resolution. At 64°N, z14 is 4.2 m/px.

### Inferences

- z17 is not legally harvestable from the open WMTS (z0–16 only). It must be rendered from the kapsi JP2s. Uusimaa land needs ~500 sheets, about 50 GB of downloads, plus GDAL with OpenJPEG.
- z0–16 from the WMTS needs a free MML API key and ~135 k requests for a clipped Uusimaa. That is not "small-scale use", so ask MML first, or render from JP2.
- Recommended default: **Uusimaa land, z0–16 WebP, ~1.7 GB**. Offer z17 as an opt-in (~6.5 GB). Offer Finland-wide only at z0–14 (~6 GB), which suits 64 GB+ sticks.
- Effort: **M** if built from JP2s with GDAL in the existing Docker toolchain (gdalwarp to 3857, then gdal2tiles or `gdal_translate -of MBTiles` with WebP and `pmtiles convert`). It becomes **L** if GDAL is not acceptable in tools-base.

## 4. Hillshade and terrain

- **MML hillshade (vinovalovarjoste) 32 m**: `http://kartat.kapsi.fi/files/vinovalovarjoste/hila_32m/etrs-tm35fin-n2000/*.tif`. **117 GeoTIFFs, 288 MB for the whole of Finland** (crawled, measured). The 8 m version (`hila_8m/`) is 1,432 files, **4.64 GB**. Other resolutions: 2, 64, 128 and 512 m. CC BY 4.0. A Finland 32 m raster PMTiles at z0–11 would probably be ~150–300 MB as WebP (unverified).
- **Mapterhorn** (`download_urls.json` version 0.0.13, measured):
  - Bytes read from the planet archive's directory for z0–12: **Uusimaa bbox ≈ 113 MB**; **Finland bbox (19.0,59.5,31.6,70.1) ≈ 2.54 GB**. The bbox includes parts of Sweden, Norway, Russia, Estonia and sea.
  - Mapterhorn also has **high-resolution z13–15/16 archives over all of Finland**: 15 z6 cells, `6-35-14` … `6-37-18.pmtiles`, totalling 203.7 GB. These cells also cover neighbouring countries.
  - For Uusimaa, `6-36-18.pmtiles` (23.6 GB) contributes z13 314 MB, z14 1.16 GB, z15 3.90 GB and z16 6 MB, about **5.4 GB** for z13–16.
  - Commands: `pmtiles extract https://download.mapterhorn.com/planet.pmtiles --bbox=23.8,59.8,26.8,61.7 --maxzoom=12` (~113 MB), plus optionally `pmtiles extract https://download.mapterhorn.com/6-36-18.pmtiles --bbox=… --maxzoom=14` (~1.5 GB).
  - The source of the high-resolution Finland data is unverified; it is plausibly MML DEM 2 m. — [Mapterhorn data access](https://mapterhorn.com/data-access/); [download_urls.json](https://download.mapterhorn.com/download_urls.json)
- Inference: a Finland vault gets the best terrain value from Mapterhorn z0–12 (Finland ~2.5 GB, Uusimaa ~0.1 GB) plus Uusimaa z13–14 (~1.5 GB), rendered by MapLibre's `raster-dem` hillshade. The MML 32 m hillshade is the cheap, all-Finland raster alternative (288 MB of source), but it needs a GeoTIFF-to-tiles builder.

## Unverified / open

- Photon RAM and disk for a Finland-only index.
- Traficom's "general non-commercial applications" wording for the WMTS (page 404 to WebFetch). Whether Traficom objects to bulk WMTS harvesting: not asked.
- Real orthophoto tile sizes from MML imagery (a swisstopo proxy was used). Generating them from a downloaded JP2 sheet with a GDAL Docker image was not permitted in this session.
- Mapterhorn's Finland high-resolution source and licence per region.
- MTK road address-range attribute names.
