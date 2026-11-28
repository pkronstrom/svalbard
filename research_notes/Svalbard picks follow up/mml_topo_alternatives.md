# Offline Finnish topographic base map: alternatives to the dead kapsi maastokartta TMS

Checked 2026-11-26. Adds to `fi_maps_followups.md` (same folder) and doesn't repeat it: the kapsi TMS shutdown, MML orthophoto, hillshade and Mapterhorn are covered there. "Measured" means I ran it from this machine on 2026-11-26. "Estimate" means arithmetic on measured samples. "Unverified" means I could not confirm it.

## Read this first

1. **kapsi still mirrors every MML raster map product as georeferenced PNG sheets, refreshed daily.** These are the 1:10k basic map (peruskarttarasteri, 1 m), 1:50k topographic map (4 m), 1:100k (8 m), 1:250k (32 m), 1:500k (64 m) and the background-map series. The raster maastokartta look can therefore be rebuilt offline from files: no API key, no tile harvesting. (Measured; see §2.)
2. **I proved the pipeline end to end on real sheets** with the repo's own tools image: kapsi PNG + `.pgw` → `gdal2tiles --xyz --tiledriver=WEBP` → MBTiles → `pmtiles convert` gives a valid WebP raster PMTiles that is z8–14 from 1:50k and z15–16 from 1:10k (§2.4).
3. **`ghcr.io/pkronstrom/svalbard-tools:0.5.0` cannot read PNG or write WebP/JPEG tiles.** Its GDAL 3.13.3 (Alpine 3.24) ships without the PNG/WEBP/JPEG plugins. `gdalinfo x.png` fails with *"plugin gdal_PNG.so is not available … You may install it with 'apk add gdal-driver-png'"* (measured). `Dockerfile:53-56` installs only `gdal-tools`. Every raster map family, including the orthophoto and Traficom work in `fi_maps_followups.md`, needs `gdal-driver-png gdal-driver-webp gdal-driver-jpeg` (plus `gdal-driver-jp2openjpeg` for orthophoto) added to tools-base. The image is also amd64-only (Docker prints a platform warning on Apple silicon), so builds run under emulation and are slow.
4. **MML will restyle the topographic map in 2027.** An MML presentation dated 12.5.2026 has these milestones:
   - Autumn 2026: the new Kansallinen maastotietokanta (KMTK) replaces Maastotietokanta and is published "tiedostoina" (as files).
   - 2027: "Karttatieto (vektori) julkaistaan" (the vector map data is published), "Maastokartan esitystyyli uudistuu" (the topographic map style is renewed), and the small-scale maps become more current.
   - KMTK GeoPackages will be downloadable "kunnittain tai omalla aluerajauksella" (per municipality or for a custom area). Per-sheet SHP/GML downloads go away after a transition period.

   Source: [MML, Maastotietokanta uudistuu, 12.5.2026](https://www.maanmittauslaitos.fi/media/32209/download) (PDF, read with pdftotext). This affects both the raster style (a built PMTiles changes look on rebuild) and the `mml-topo` vector schema.
5. **The viewer already renders raster PMTiles.** `host-cli/internal/mapview/viewer.html:126-138` accepts PNG/JPEG/WebP/AVIF archives as a `raster` source, and a `basemap` category is a radio choice. What's missing is the builder, not the viewer.

## 1. MML open WMTS (Karttakuvapalvelu), API key

**Facts**
- Open service `avoin-karttakuva.maanmittauslaitos.fi`: Topographic map (`maastokartta`), Background map (`taustakartta`), Plain map (`selkokartta`) and Orthophoto. Zoom levels are **ETRS-TM35FIN 0–13** and **WGS84 Pseudo-Mercator 0–16**. "More detailed zoom levels require an agreement." Contract service: PNG8, Pseudo-Mercator 0–18. — [MML WMTS page](https://www.maanmittauslaitos.fi/en/node/11151)
- The service is "intended for testing the interface and for small-scale use", and overload returns 503. The page gives no numeric limit. It recommends caching tiles "for a week" when there are no cache headers. — same page
- Without a key, capabilities return `401` (measured).
- Terms of use for the open interface:
  - "Interface service data are free of charge and fall under the NLS's open data licence", and the source must be indicated.
  - The service "is not intended for using high-volume services", and NLS may restrict a user if "high usage volumes interfere with the operation of the interface service".
  - The user must protect their API key.
  - The terms say nothing either way about bulk download or offline caching.

  — [Terms of use for the open interface](https://www.maanmittauslaitos.fi/en/terms-of-use-for-the-open-interface)
- Keys are personal and created in the My Account service. They are passed as Basic-auth username or `api-key=` and let NLS "monitor the utilisation rate … and contact the service users". — [API key instructions](https://www.maanmittauslaitos.fi/en/rajapinnat/api-avaimen-ohje)
- Product licence: CC BY 4.0 (quoted in §2's product description).

**Inferences**
- Harvest volume:
  - Uusimaa bbox z0–16 is ~518 k tiles (tile count from `fi_maps_followups.md` §3).
  - Finland z0–14 is ~440 k tiles, and z0–15 ~1.76 M.
  - That's legal under CC BY 4.0, but it is the "high-volume" use the terms say the service is not for. Each Svalbard user would need their own key, which adds secret-handling UX.
- It gives the exact MML zoom-dependent look with no composing work. Its other limits are the hard z16 cap, a z16 resolution of 1.19 m/px at 60°N, and throttling risk.
- Effort: **M** for a generic `raster-wmts` harvester (bbox, zoom, throttle, resume, key from env). It becomes an **L** if MML asks for an agreement or throttles hard. Use it as a fallback, not the default.

## 2. kapsi raster sheets (recommended source)

### 2.1 What's there (crawled 2026-11-26, measured)

All files are paletted PNG with a `.pgw` world file and no `.prj`; the SRS is EPSG:3067. They live under `http://kartat.kapsi.fi/files/<product>/…/etrs89/png/<200k>/<100k>/<sheet>.png`.

| Product (kapsi path) | Scale / px | Sheet size | Sheets | Size, all Finland | Sheets in Uusimaa bbox* | Size, Uusimaa bbox |
|---|---|---|---|---|---|---|
| `peruskarttarasteri_jhs180/painovari/1m/` | 1:10k print colour, 1 m | 12×12 km (12000² px) | 2,939 | **35.83 GB** | 254 | **3.88 GB** |
| `peruskarttarasteri_jhs180/painovari_ei_pehmennysta/1m/` | same, not anti-aliased | 12×12 km | 2,939 | 17.23 GB | – | – |
| `peruskarttarasteri_jhs180/taustavari_korkeuskayrilla/1m/` | background colour with contours | 12×12 km | 2,939 | 33.19 GB | – | – |
| `maastokarttarasteri_50k_jhs180/painovari/4m/` | 1:50k, 4 m | 48×24 km (12000×6000 px) | 413 | **5.63 GB** | 38 | **0.67 GB** |
| `maastokarttarasteri_100k_jhs180/kaikki/` | 1:100k, 8 m | 48×48 km | 217 | 1.35 GB | 20 | 0.16 GB |
| `maastokarttarasteri_250k_jhs180/kaikki/` | 1:250k, 32 m | 96×96 km | 65 | 0.14 GB | 6 | 0.02 GB |
| `maastokarttarasteri_500k_jhs180/kaikki/` | 1:500k, 64 m | – | 38 | 0.03 GB | – | – |
| `taustakarttasarja_jhs180/taustakartta_{5k,10k,20k,40k,80k,160k,…}` | background map, 0.5–2048 m | – | 16,935 | 45.8 GB (5k: 35.9, 10k: 6.5, 20k: 2.5) | 20k: 136 | 20k: 0.28 GB |
| `maastokartta_100k`, `maastokartta_250k` | **vector** MIF zips, not raster | – | 244 / 130 zips | 2.49 / 0.69 GB | – | – |

\*Sheets intersecting the recipe bbox `23.8,59.8,26.8,61.7` (TM35 projection of the bbox edges and world-file extents; my own UTM35/GRS80 code, checked against Helsinki at 385700 E / 6672127 N). The bbox includes sea and parts of neighbouring regions.

- File dates show daily refresh. 1,282 of the 2,939 basic-map sheets and 300 of the 413 1:50k sheets were modified in November 2026 (measured from the autoindex). The kapsi front page says data "päivitetään automaattisesti kerran vuorokaudessa yöllä" (updated automatically once a night) and offers HTTP, FTP and `rsync tiedostot.kartat.kapsi.fi::mml/`. Licence: MML open data under CC BY 4.0 since 16.1.2015, with no availability guarantee. — [kartat.kapsi.fi](https://kartat.kapsi.fi/)
- **Product facts from MML** ([product description, Topographic map (raster)](https://www.maanmittauslaitos.fi/en/maps-and-spatial-data/datasets-and-interfaces/product-descriptions/topographic-map-series-raster); the PDF snapshot is on [funet](https://www.nic.funet.fi/index/geodata/mml/peruskartta/2025/NLS_topographic_map_series_raster_2025.pdf)):
  - Coverage: entire Finland, EPSG:3067. Distribution format: PNG.
  - Pixel sizes: 1 m (1:10k), 4 m (1:50k), 8 m (1:100k), 32 m (1:250k), 64 m (1:500k) … 2048 m.
  - Variants: "Print colour, anti-aliased", "not anti-aliased", and "Background colour with contour lines".
  - Maintenance: 1:10k and 1:50k are updated "approximately 2‒3 days after" the Topographic Database. 1:100k–1:1M "are published once a year in July".
  - "Conditions applying to access and use: Attribution 4.0 International (CC BY 4.0)". The attribution must give the licensor, the dataset name and the delivery date, e.g. "contains data from the National Land Survey of Finland Topographic Database 06/2014".
  - SYKE and Väylävirasto hold copyright on route data, and SYKE on depth contours. This is noted under the same CC BY 4.0 conditions.

### 2.2 Paituli / funet mirror (fallback)

- `https://nic.funet.fi/index/geodata/mml/` has `maastokarttarasteri_{50k,100k,250k,500k}/`, `peruskartta/`, `peruskartta_painovari/` and `peruskartta_ei_kkayria/`. These are **yearly snapshots** (`2011/ 2015/ 2020/ 2025/`) and include a `.prj` per sheet. Example: `maastokarttarasteri_50k/2025/L4/L41/L413.png` is 25 MB with `.pgw` and `.prj` dated 2025-05/06. The terms PDFs are in the same folder (measured; [funet mml](https://nic.funet.fi/index/geodata/mml/)).
- Use it as a mirror when kapsi is down. It can be up to a year stale, but it is served over HTTPS by CSC.

### 2.3 MML's own file service (official, keyed)

- "OGC API Processes" file service at `https://avoin-paikkatieto.maanmittauslaitos.fi/tiedostopalvelu/ogcproc/v1/`:
  - Products include Maastokartta (rasteri), Taustakartta (rasteri), Ortoilmakuva and Maastotietokanta. You can select by bbox, polygon, map sheet, municipality or all of Finland.
  - "Käyttäjä tunnistetaan API-avaimella" (the user is identified by API key). The data is under CC BY 4.0 and the service is free and available 24/7.
  - Source: [Paikkatiedon tiedostopalvelu](https://www.maanmittauslaitos.fi/paikkatiedon-tiedostopalvelu) and [OGC API Processes page](https://www.maanmittauslaitos.fi/en/node/16143).
- It returns `401` without a key (measured). Output formats and size limits are **unverified**.
- This is the official route for **bbox-clipped Maastotietokanta GeoPackage**. It would remove the 22.8 GB national zip that `mml-topo-uusimaa` pulls today.

### 2.4 Measured conversion test

Test setup:
- Sheets: Helsinki centre `L4133L` (urban, 17.9 MB PNG) and `L4142L` (Espoo/Nurmijärvi rural, 20.4 MB), both from the 1:10k print-colour product, plus 1:50k `L413` (26.2 MB) and `L414` (24.3 MB).
- Command: `gdal_translate -of VRT -a_srs EPSG:3067 -expand rgb`, then `gdal2tiles --xyz -s EPSG:3067 -r average --tiledriver=WEBP --webp-quality=80`, run in the tools image after `apk add gdal-driver-png gdal-driver-webp gdal-driver-jpeg`.

Average interior-tile size (KB, WebP q80):

| Zoom (m/px at 60.2°N) | 1:10k urban | 1:10k rural | 1:50k L413 | 1:50k L414 |
|---|---|---|---|---|
| z12 (19.1) | 29.0 | 26.1 | 30.8 | 32.0 |
| z13 (9.6) | 25.5 | 27.3 | 29.2 | 30.0 |
| z14 (4.8) | 20.6 | 25.1 | 23.9 | 23.4 |
| z15 (2.4) | 15.4 | 18.9 | 17.3 | 16.2 |
| z16 (1.19) | 9.8 | 12.0 | – | – |
| z17 (0.6, upsampled) | 6.3 | 7.6 | – | – |

Format comparison on the rural sheet (60 z16 and 17 z14 tiles, re-encoded with cwebp/pngquant):

| Format | z16 avg | z14 avg |
|---|---|---|
| RGB PNG (gdal2tiles PNG) | 53.4 KB | 89.0 KB |
| pngquant PNG8 | 18.5 KB | 28.4 KB |
| **WebP q80** | **12.4 KB** | **22.0 KB** |
| WebP lossless | 36.6 KB | 60.4 KB |

**WebP q80 is the smallest**, about 1.5× smaller than PNG8 and 4× smaller than RGB PNG. It's visually acceptable for line art (not inspected side by side; unverified).

Tile bytes as a fraction of source PNG bytes (measured per sheet; this captures content density, including near-empty sea sheets):
- 1:10k: z15 = 0.34 / 0.38, z16 = 0.88 / 0.96.
- 1:50k: z13+z14 = 0.96 / 1.02, z14 alone = 0.73 / 0.78.

End-to-end check: the z8–14 tiles from 1:50k and the z15–16 tiles from 1:10k were packed into one MBTiles (a 40-line Python sqlite script) and run through `pmtiles convert`. Result: `tile type: webp`, z8–16, 3,231 tiles, 48.9 MB (measured).

### 2.5 Estimated output sizes (WebP q80, bbox and all of Finland)

Proposed zoom bands, matching each product's pixel size to Web-Mercator ground resolution:

| Zooms | Source | Uusimaa bbox | All Finland |
|---|---|---|---|
| z0–8 | 1:500k (64 m) | <0.01 GB | ~0.02 GB |
| z9–11 | 1:250k (32 m) | ~0.02 GB | ~0.1 GB |
| z12 | 1:100k (8 m) | ~0.1 GB | ~1 GB |
| z13–14 | 1:50k (4 m) | **~0.66 GB** | **~5.6 GB** |
| z15 | 1:10k (1 m) | ~1.4 GB | ~12.9 GB |
| z16 | 1:10k (1 m) | ~3.6 GB | ~33 GB |
| **Total z0–14** | | **~0.8 GB** | **~6.7 GB** |
| **Total z0–15** | | **~2.2 GB** | **~20 GB** |
| **Total z0–16** | | **~5.8 GB** | **~52 GB** |

Basis: measured ratios × crawled source bytes (estimate). The z≤12 rows are rough because I didn't render those products. Sanity check: the Uusimaa bbox is ~34,400 km², or ~385 k z16 tiles at ~9.4 KB, which gives ~3.6 GB, consistent with the table.

Source download needed for Uusimaa z0–16: ~4.7 GB. For Finland z0–14: ~7.1 GB, and for z0–16: ~43 GB.

Not measured:
- Build time. It was slow under amd64 emulation; four sheets × two formats took more than 10 minutes.
- Whether neighbouring sheets with different palettes mosaic cleanly in `gdalbuildvrt`. Expanding each sheet to RGB first, as I did, avoids palette mismatch.

## 3. MML vector products

**MML open vector tiles (Karttakuvapalvelu, Vektoritiilet)**
- Facts from [MML technical description (vector tiles)](https://www.maanmittauslaitos.fi/en/node/13251):
  - MVT 2.1 in EPSG:3857 and EPSG:3067. Versions v20 (2020-06) and v21 (2024-06).
  - Tiles: `…/vectortiles/taustakartta/wmts/1.0.0/taustakartta/default/v20/WGS84_Pseudo-Mercator/{z}/{y}/{x}.pbf?api-key=`. TileJSON is under `…/vectortiles/tilejson/…`.
  - Styles: v20 only, `taustakartta.json` (like the raster background map) and `backgroundmap.json` (more features, with contours), at `…/vectortiles/stylejson/v20/<name>.json?TileMatrixSet=…&api-key=`. The page lists **no maastokartta vector style**.
  - The API key is required (`401` measured). The page gives no maxzoom, licence or caching terms (maxzoom unverified).
- Style templates are in [nlsfi/avoin-karttakuva.maanmittauslaitos.fi](https://github.com/nlsfi/avoin-karttakuva.maanmittauslaitos.fi/tree/master/vectortiles/stylejson/v20):
  - `backgroundmap.json` has 113 layers over 11 source layers: `hallintoalue korkeus liikenne maankaytto maasto_alue maastoaluereuna nimisto rakennelma rakennus vesisto_alue vesisto_viiva`.
  - It uses glyphs "Liberation Sans NLSFI" (served by MML with a key). It has no sprite.
  - **The GitHub repo declares no licence** (GitHub API `license: null`, measured), so the style JSON's reuse terms are unverified. The data is CC BY 4.0 under the open-interface terms.
- Can it look like maastokartta offline? Not out of the box. Neither style reproduces the raster topo symbology (forest/swamp patterns, rock symbols, 1:50k colouring). You would have to author a maastokartta style. MML itself plans a new vector map product and style for 2027 ("Read this first", item 4), so authoring against today's schema is wasted work.
- The repo's existing `mml-topo-uusimaa` design (`docs/plans/2026-04-15-finnish-topo-maps-design.md`) already takes the self-built route: MTK GeoPackage → tippecanoe with taustakartta-like source layers → a simplified style. That also avoids harvesting keyed vector tiles.

**MTK on kapsi**
- `maastotietokanta/geopackage_maasto/mtkmaasto.zip` is **22,784,416,228 B** and `geopackage_korkeus/mtkkorkeus.zip` is **11,220,799,651 B**, both dated 04-Nov-2026 (measured).
- Per-sheet GML zips (`maastotietokanta/kaikki/etrs89/gml/L4/L41/L4111L_mtk.zip`, 5–9 MB each) are **frozen at March 2025** (measured; the kapsi front page says "Kaikki kohteet: 28.03.2025"). MML says per-sheet GML/SHP disappear under KMTK.
- Inference: `mml-topo` is an **L**. It needs a 34 GB download or a keyed bbox API, a moving schema (KMTK from autumn 2026), and style work. Keep it behind the raster map.

## 4. OSM-based topo alternatives

- **Protomaps basemap + Mapterhorn `raster-dem` + client-side contours.** [maplibre-contour](https://github.com/onthegomap/maplibre-contour) (BSD-3-Clause) draws contour lines in MapLibre GL JS from any raster-dem source, in a web worker ([MapLibre example](https://maplibre.org/maplibre-gl-js/docs/examples/contour-lines/)). Pointed at a local Mapterhorn PMTiles (sizes in `fi_maps_followups.md` §4), it should work offline; that is unverified because nobody documents an offline setup. The look is OSM cartography plus hillshade and contours, ODbL. Effort: the `osm-extract` family is the already-tracked **L**; hillshade and contours on top are **S–M** once a raster PMTiles path exists.
- **OpenTopoMap**: tiles are CC-BY-SA 3.0. Embedding is allowed only "sofern unser Server z.B. durch Massendownloads nicht zu stark beansprucht wird" (provided the server isn't overloaded, for example by mass downloads), so **no harvesting**. The style is on [GitHub](https://github.com/der-stefan/OpenTopoMap), but self-rendering needs Mapnik + PostGIS + DEM processing (**XL**, not a fit). — [opentopomap.org/about](https://opentopomap.org/about)
- **MapAnt Finland** (an orienteering-style map of all of Finland from MML lidar + MTK):
  - Tiles at `//www.mapant.fi/wmts/wmts.php?z={z}&x={x}&y={y}` in a custom TM35 Leaflet grid (from `mapant.js_dev.php`, measured).
  - The attribution credits only MML data "under the Creative Commons license". **MapAnt's own terms for its rendered tiles are not stated** (unverified).
  - Don't use it without asking the maintainers.
- **Thunderforest / commercial**: excluded, as the brief says.

## 5. How other apps cope

- *Maastokartta – Suomi* (iOS, "Topo maps – Finland", [App Store](https://apps.apple.com/app/apple-store/id727466441)):
  - v4.0.0: "Maastokartta is back, this time properly rebuilt". v5.1.0: offline downloads without PRO. v5.6.2 is dated Oct 31.
  - It doesn't name its tile source. A rebuild after the kapsi shutdown suggests self-hosted tiles rendered from MML files or a contract WMTS (inference, unverified).
- I found no OsmAnd or Locus documentation of a Finland MML source after the kapsi shutdown. The generic route is a custom online WMTS source with a personal `api-key` (unverified; see the search summary for [MML WMTS docs](https://www.maanmittauslaitos.fi/en/node/11151)).
- The GPXSee map definitions (`Maastokartta.xml`, `MapAnt.xml`) are behind an Anubis bot wall, so I couldn't read them.

## 6. Options compared

| # | Source → build | Key | Licence | Look | Uusimaa | Finland | Freshness | Effort |
|---|---|---|---|---|---|---|---|---|
| A | **kapsi PNG sheets → gdal2tiles WebP → PMTiles** (multi-scale bands) | none | CC BY 4.0 | exact MML raster cartography per scale | z0–14 ~0.8 GB; z0–15 ~2.2 GB; z0–16 ~5.8 GB | z0–14 ~6.7 GB; z0–15 ~20 GB; z0–16 ~52 GB | daily (1:10k/50k), yearly (≤1:100k) | **M** |
| A′ | Paituli yearly sheets (same pipeline) | none | CC BY 4.0 | same | same | same | yearly | (mirror for A) |
| A″ | MML OGC API Processes raster download (same pipeline) | personal | CC BY 4.0 | same | same | same | MML-current | M (+ key UX; formats unverified) |
| B | MML open WMTS harvest → PMTiles | personal | CC BY 4.0 | exact MML zoom mix | 518 k tiles, z0–16 ~5–6 GB (estimate) | z0–14 ~440 k tiles | live | M–L; "not for high volume" |
| C | MML vector tiles harvest + MML style | personal | data CC BY 4.0; style licence unverified | background map, not topo | unmeasured | unmeasured | live | L |
| D | MTK GPKG → tippecanoe (`mml-topo`) | none (kapsi) | CC BY 4.0 | custom style | 34 GB download for 0.2–0.5 GB output | 5–10 GB (design doc estimate) | KMTK change | L |
| E | Protomaps + Mapterhorn + maplibre-contour | none | ODbL + DEM licences | OSM topo-ish | ~0.1 GB DEM + OSM extract | ~2.5 GB DEM + OSM | per pin | L (osm-extract) + S |

## 7. Recommendation

**(a) Uusimaa default in finland-32/64: source A, kapsi sheets, z0–14 WebP, ~0.8 GB.**
- Bands: z0–8 1:500k, z9–11 1:250k, z12 1:100k, z13–14 1:50k.
- The current recipe's `size_gb: 1.0` and `max_zoom: 13` stay about right; z14 from 1:50k costs ~0.5 GB more than z13.
- Offer z15 (1:10k basic map, +1.4 GB) as the "detailed" option for finland-64.
- z16 (+3.6 GB) belongs in 128+, or pair it with the orthophoto there. z16 is the native 1:10k resolution and the most useful zoom for hiking, but it is the expensive one.

**(b) All of Finland in larger tiers: the same family and recipe with a Finland bbox.**
- finland-128: z0–14, ~6.7 GB.
- finland-256: z0–15, ~20 GB, optionally with Uusimaa z16 as a second item.
- All-Finland z0–16 (~52 GB) only as an explicit opt-in.
- Keep source B (WMTS harvest with a user key) as a documented fallback if kapsi disappears. Paituli (A′) is the keyless mirror.

**Code needed (one new family; nothing existing fits).**
- `vector-static` and `vector-service` are vector-only. `raster-tms` was never implemented, and its contract (`tms_url`) is dead.
- Replace it with a **`raster-sheets`** family (or keep the name `raster-tms` with a `source: sheets|wmts` switch, to also cover Traficom and orthophoto later). Sketch:

```yaml
build:
  family: raster-sheets
  srs: EPSG:3067
  bbox: "23.8,59.8,26.8,61.7"
  tile_format: webp
  quality: 80
  bands:
    - {min_zoom: 0,  max_zoom: 8,  base: http://kartat.kapsi.fi/files/maastokarttarasteri_500k_jhs180/kaikki/etrs89/png/}
    - {min_zoom: 9,  max_zoom: 11, base: http://kartat.kapsi.fi/files/maastokarttarasteri_250k_jhs180/kaikki/etrs89/png/}
    - {min_zoom: 12, max_zoom: 12, base: http://kartat.kapsi.fi/files/maastokarttarasteri_100k_jhs180/kaikki/etrs89/png/}
    - {min_zoom: 13, max_zoom: 14, base: http://kartat.kapsi.fi/files/maastokarttarasteri_50k_jhs180/painovari/4m/etrs89/png/}
  mirror: https://nic.funet.fi/index/geodata/mml/   # yearly fallback, different layout
```

- Go side: list sheets per band, either by walking the nginx autoindex or by computing the JHS 180 sheet grid from the bbox, then select sheets by `.pgw` extent. Download with resume and cache keyed by URL and file date. Give a size estimate before apply, as the openspec design requires.
- tools-base side:
  - Per band: `gdal_translate -of VRT -a_srs EPSG:3067 -expand rgb` per sheet, then `gdalbuildvrt`, then `gdal2tiles --xyz -z a-b --tiledriver=WEBP --webp-quality=80 -r average --processes=N` into one XYZ tree.
  - Then XYZ tree → MBTiles (small script) → `pmtiles convert`. Write attribution with the month: "Contains data from the National Land Survey of Finland, Topographic map raster MM/YYYY (CC BY 4.0)".
- Prerequisites:
  1. Add `gdal-driver-png gdal-driver-webp gdal-driver-jpeg` to tools-base (S).
  2. `BuildSpec.UnmarshalYAML` keeps only scalar unknown keys and drops arrays and maps (`openspec/changes/archive/2026-09-01-go-builder-pipeline/build-family-inventory.md:27`). A `bands:` list needs that fixed or typed fields (S–M).
- Overall: **M** (catalog + builder + tools-base + recipe + test). It becomes an **L** if the sheet listing must be computed from the JHS 180 grid rather than the autoindex, or if the 2027 restyle forces a re-render contract.

**Backlog fit.**
- The proposed map source contract already lists "MML WMTS with a key from env, or the sheets". This research supports choosing **the sheets** for raster, with WMTS as the fallback. For `mml-topo` it supports the OGC API Processes/KMTK bbox GeoPackage over the 22.8 GB zip.
- When implementing the build families, treat `raster-tms` as `raster-sheets` (+ optional WMTS mode).
- Stop shipping `mml-maastokartta` as the short-term fix.
- Stop shipping the current map, settle the source contract, add the tools-base drivers and BuildSpec arrays, and implement the raster family before re-adding `mml-maastokartta` to `fi-maps`.

## Unverified / open
- The OGC API Processes output formats and area limits for the raster products (key needed).
- The MML vector tiles maxzoom, and the licence of the nlsfi style JSONs.
- MapAnt's terms for its rendered tiles.
- Visual quality of WebP q80 on small map text (sizes measured, look not reviewed). Also z≤12 band sizes (products not rendered) and full-build time.
- Whether MML objects to a ~0.5 M-tile WMTS harvest per user. Not asked; email verkkopalvelut@maanmittauslaitos.fi before using option B ([general interface info](https://www.maanmittauslaitos.fi/en/maps-and-spatial-data/datasets-and-interfaces/general-information-about-interface-services)).

Scratch artefacts (crawl TSVs, sample tiles, sample PMTiles, scripts) are in the session scratchpad: `/private/tmp/claude-501/-Users-pkronstrom-Projects-own-svalbard/2fd22a2d-583e-439a-9d48-6da08ba2799a/scratchpad/`.
