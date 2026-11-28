# Finnish open datasets for an offline vault (gap list vs. current catalog)

Checked 2026-11-26. "Verified" below means an HTTP HEAD/GET from this machine returned 200 and, where shown, the byte size and Last-Modified date come from that response. Items the catalog already covers are excluded: osm-finland/osm-uusimaa, mml-maastokartta (kapsi raster tiles), mml-topo-uusimaa (MTK vector, Uusimaa only), mml-map-sheets, pohjavesialueet, luonnonsuojelualueet, virtavesien-lohikalakannat, lipas-recreation, foraging-habitats-fi (recipe exists, builder unwired), fimea (Perusrekisteri.xml), wikipedia/wiktionary/wikibooks/wikisource/wikivoyage-fi, gutenberg-fi, scoutwiki-fi, phet-fi.

## What does avoindata.fi offer in maps, terrain, water, health, emergency, food and nature?

### Takeaway
avoindata.fi now redirects to **avoindata.suomi.fi**. Its CKAN API works without a key at `https://avoindata.suomi.fi/data/api/3/action/package_search`. Nearly everything useful is CC BY 4.0. The strongest offline gaps are SYKE hydrography and hazards (lake shorelines, depth contours, river network, catchments, flood zones), MML place names, elevation and hillshade, Finlex consolidated legislation, Kotus dictionaries and word list, the official suomi.fi preparedness guide PDFs, and GTFS timetables. The portal has no national civil-defence shelter dataset, no Metsähallitus hut dataset, and no Fingrid grid map.

### Cited Findings
**Portal and API**
- `https://www.avoindata.fi/data/api/3/action/package_search` returns HTTP 301 to `https://avoindata.suomi.fi:443/data/api/3/action/package_search`. Clients must follow redirects or use the new host. No key is needed (verified with curl). — [avoindata.suomi.fi CKAN API](https://avoindata.suomi.fi/data/api/3/action/package_search?q=ranta10)

**Water and hydrography (SYKE, CC BY 4.0, all verified, no key)**
- **Ranta10 shoreline 1:10 000**, a topologically clean water-body dataset derived from MML MTK. It covers lakes, rivers wider than 5 m, and sea and islands. Three files:
  - `https://wwwd3.ymparisto.fi/d3/gis_data/spesific/ranta10jarvet.zip`: 341 MB, modified 2026-05-27
  - `.../ranta10joet.zip`: 220 MB, 2026-03-19
  - `.../ranta10meret.zip`: 112 MB, 2025-03-14

  Source: [CKAN ranta10-rantaviiva-1-10-000](https://avoindata.suomi.fi/data/fi/dataset/ranta10-rantaviiva-1-10-000)
- **Lake and river depth data**: depth areas `.../syvyysalue.zip` (217 MB) and depth contours `.../syvyyskayra.zip` (120 MB). Both were last modified 2016-09-21, so the data is static. Sounding points are also offered via WFS. — [CKAN jarvien-ja-jokien-syvyysaineisto](https://avoindata.suomi.fi/data/fi/dataset/jarvien-ja-jokien-syvyysaineisto)
- **River network (Uomaverkosto)**: `.../uomaverkosto.zip`, 75 MB, 2026-03-19. — [CKAN uomaverkosto](https://avoindata.suomi.fi/data/fi/dataset/uomaverkosto)
- **Catchment division (Valuma-aluejako)**: five hierarchy levels with more than 40 000 catchments. `.../valumaalueet.zip`, 259 MB, 2026-05-27. — [CKAN valuma-aluejako](https://avoindata.suomi.fi/data/fi/dataset/valuma-aluejako)
- **Järvirekisteri (lake register)** holds name, ID, area and shoreline length for every lake over 1 ha. CKAN points only to the generic page `http://www.syke.fi/avointieto`, and the guessed URL `.../jarvirekisteri.zip` returned 404. — [CKAN jarvirekisteri](https://avoindata.suomi.fi/data/fi/dataset/jarvirekisteri)
- **Springs and seepages (Lähteikköjen esiintyminen)**: 48 102 known springs, spring complexes and seepages as polygons with a 50 m buffer. Available only via WFS at `https://paikkatiedot.ymparisto.fi/geoserver/syke_lahteikotesiintyminenmuuttuneisuus/wfs`. — [CKAN lahteikkojen-esiintyminen-ja-muuttuneisuus](https://avoindata.suomi.fi/data/fi/dataset/lahteikkojen-esiintyminen-ja-muuttuneisuus)

**Hazards (SYKE, CC BY 4.0)**
- **Flood hazard zones, basic scenarios**:
  - `https://sykedata.ymparisto.fi/gisdata-1/tulva/tulvavaaravyohykkeet/tulvavaaravyohykkeet_meri.zip`: 5.59 GB, 2025-11-25
  - `.../tulvavaaravyohykkeet_vesisto.zip`: 5.65 GB, 2026-03-19

  WMS, WFS and WCS endpoints and an ArcGIS REST service are also offered. — [CKAN tulvavaaravyohykkeet](https://avoindata.suomi.fi/data/fi/dataset/tulvavaaravyohykkeet-perusskenaariot-flood-hazard-zones-basic-scenarios)
- **Significant flood-risk areas**: `https://wwwd3.ymparisto.fi/d3/gis_data/spesific/tulvariskialueet.zip`, only 84 KB, 2024-11-18. — [CKAN tulvariskialueet](https://avoindata.suomi.fi/data/fi/dataset/tulvariskialueet)

**Water points (HSY, capital region)**
- Public water posts where residents can freely fill containers, with address and coordinates, updated October 2026. Shapefile `https://avoidatastr.blob.core.windows.net/avoindata/AvoinData/1_Vesi_ja_viemarit/Vesipostit/Shp/HSY_Vesipostit_shp.zip` is 5.3 KB, modified 2026-11-02. CC BY 4.0. — [CKAN paakaupunkiseudun-vesipostit](https://avoindata.suomi.fi/data/fi/dataset/paakaupunkiseudun-vesipostit)

**Climate (FMI, CC BY 4.0)**
- **Climate normals for 1971–2000, 1981–2010 and 1991–2020**: pressure, temperature, humidity, precipitation, snow depth, wind and sunshine per station. Served by WFS stored query `fmi::observations::weather::monthly::30year::simple` on `https://opendata.fmi.fi/wfs`. A test query with `fmisid=101932` returned 12 monthly values per parameter without a key. Example parameters: CLAP1M, EVPP1M, GLOBP1M, NDPR*, NDTAN*, NDTAX*. — [CKAN saahavaintojen-ilmastolliset-vertailuarvot](https://avoindata.suomi.fi/data/fi/dataset/saahavaintojen-ilmastolliset-vertailuarvot); [FMI WFS](https://opendata.fmi.fi/wfs?service=WFS&request=GetCapabilities)
- The CSC Paituli mirror on funet holds FMI 10 km gridded daily and monthly climate grids: temperature, precipitation, snow, radiation and others. — [nic.funet.fi/index/geodata/ilmatiede](https://nic.funet.fi/index/geodata/ilmatiede/)

**Geology (GTK, CC BY 4.0)**
- Soil 1:1 000 000 and 1:200 000 and bedrock 1:1 000 000 to 1:100 000 are listed, but resources point to the `hakku.gtk.fi` download portal rather than direct files. — [CKAN maapera-1-1-000-000](https://avoindata.suomi.fi/data/fi/dataset/maapera-1-1-000-000); [CKAN kalliopera-1-1-000-0001](https://avoindata.suomi.fi/data/fi/dataset/kalliopera-1-1-000-0001)

**Nautical**
- Traficom nautical chart product division and chart layers are offered only as open WMS and WFS (`https://julkinen.traficom.fi/inspirepalvelu/avoin/wfs`). No bulk ENC download is listed. — [CKAN suomalaiset-merikartat-tuotejako](https://avoindata.suomi.fi/data/fi/dataset/suomalaiset-merikartat-tuotejako)

**Forest (Metsäkeskus, CC BY 4.0)**
- Forest resource stands and grid data are offered via `https://avoin.metsakeskus.fi/rajapinnat/v1/stand/ows`. Downloads are split by region and the dataset is large. — [CKAN metsavarakuviot](https://avoindata.suomi.fi/data/fi/dataset/metsavarakuviot)

**Recreation**
- **Virgis 2013**: nationwide recreation areas, trails and services built mainly from Metsähallitus and University of Jyväskylä source data. Offered only as a WMS on lounaispaikka. It is old and largely superseded by LIPAS. — [CKAN virgis](https://avoindata.suomi.fi/data/fi/dataset/luonnon-virkistyskayttomahdollisuudet-suomessa-virgis-2013)

**Services directory (DVV, CC0)**
- **Suomi.fi Palvelutietovaranto (PTV)**: every public service and service point, including health stations and rescue services, with addresses. JSON/REST API with weekly updates. — [CKAN palvelutietovaranto](https://avoindata.suomi.fi/data/fi/dataset/palvelutietovaranto); [API](https://api.palvelutietovaranto.suomi.fi/swagger/ui/)

**Searches with no national result**
- `vaestonsuoja` matched only HSY's capital-region buildings dataset.
- `autiotupa`, `laavu`, `retkikartta`, `fingrid`, `apteekki`, `kaivot` and `jäätilanne` returned 0 hits.

Source: CKAN queries run 2026-11-26 against [avoindata.suomi.fi](https://avoindata.suomi.fi/data/api/3/action/package_search).

### Inferences
**Recommended additions, in priority order**

| # | Candidate | Publisher | URL | Format / size | License | Cadence | Auth | Packaging |
|---|---|---|---|---|---|---|---|---|
| 1 | Suomi.fi "Varautunut pärjää paremmin" preparedness summary, 16 language variants | Sisäministeriö / DVV | `https://cdn.verkkopalvelu.suomi.fi/files/varautunut-parjaa-paremmin_SM_suomi-e093f829ad4179e1b727c8f283a3e113.pdf` (plus _ruotsi, _englanti, _venaja, _ukraina, _viro, _selkosuomi, the three Sámi variants, etc.) | PDF, ~1.2 MB each | not stated (gap) | ad hoc | none | raw PDFs plus an index HTML |
| 2 | SPEK 72 tuntia "Varautuminen kotona" guide | SPEK | `https://72tuntia.fi/wp-content/uploads/2025/01/spek_72_tuntia_varautuminen_kotona_2025_01_21_v01.pdf` | PDF, 2.9 MB | not stated (gap) | ad hoc (2025-01) | none | raw PDF |
| 3 | Finlex consolidated legislation | Oikeusministeriö | `https://www.finlex.fi/files/open-data/archives/statute-consolidated.zip` | Akoma Ntoso XML zip, 4.4 GB | CC BY 4.0 | daily | none | build static HTML or SQLite FTS from the XML. Consider filtering to key acts (pelastuslaki, valmiuslaki, metsästyslaki, jokamiehenoikeudet-related acts) |
| 4 | Kotus Nykysuomen sanalista 2024 | Kotus | `https://kaino.kotus.fi/lataa/nykysuomensanalista2024.txt` (same bytes as .csv) | TSV/CSV, 2.9 MB | CC BY 4.0 | rare (2024) | none | raw file, or SQLite plus a search page |
| 5 | Kotus dictionaries: Suomen murteiden sanakirja, Suomi–ruotsi-suursanakirja, Vanhan kirjasuomen sanakirja, Karjalan kielen sanakirja | Kotus | `https://kaino.kotus.fi/lataa/{sms,suru,vks,kks}.zip` | XML zip: 47.7, 11.9, 18.0, 16.7 MB | CC BY 4.0 | irregular | none | XML to static HTML or SQLite FTS. Suomi–ruotsi has the most practical value |
| 6 | MML place names (Nimistö, ~800 000 places) | MML via kapsi | `http://kartat.kapsi.fi/files/nimisto_oapif/placenames/etrs89/gml/placenames_2026_09.zip`; `.../places/etrs89/gml/places_2026_09.zip` | GML zip, 53 MB / 46 MB | CC BY 4.0 | 3×/year (Jan/May/Sep) | none (kapsi) | SQLite gazetteer for offline geocoding in the map viewer |
| 7 | SYKE Ranta10 lakes, rivers and sea | SYKE | see findings above | SHP zip, 112–341 MB | CC BY 4.0 | ~yearly | none | PMTiles overlay |
| 8 | SYKE lake depth contours and areas | SYKE | `.../syvyyskayra.zip`, `.../syvyysalue.zip` | SHP zip, 120 / 217 MB | CC BY 4.0 | static since 2016 | none | PMTiles overlay (fishing, ice safety) |
| 9 | MML hillshade (vinovalovarjoste), 32 m | MML via kapsi | `http://kartat.kapsi.fi/files/vinovalovarjoste/hila_32m/etrs-tm35fin-n2000/*.tif` | GeoTIFF tiles, 40 KB–1.8 MB each | CC BY 4.0 | occasional | none | merge to a raster PMTiles under the OSM basemap |
| 10 | MML DEM 10 m (korkeusmalli) | MML via kapsi | `http://kartat.kapsi.fi/files/korkeusmalli/hila_10m/etrs-tm35fin-n2000/<sheet>/` | GeoTIFF/zip tiles, ~6–8 MB each | CC BY 4.0 | static (2023 mirror dates) | none (kapsi) | terrain-RGB PMTiles, or contours derived per region |
| 11 | MML MTK height features (contours, spot heights) | MML via kapsi | `http://kartat.kapsi.fi/files/maastotietokanta/geopackage_korkeus/mtkkorkeus.zip` | GPKG zip, 11.2 GB, 2026-11-04 | CC BY 4.0 | monthly | none | clip per region to vector PMTiles (Uusimaa first, like mml-topo-uusimaa) |
| 12 | SYKE river network and catchments | SYKE | `.../uomaverkosto.zip` (75 MB), `.../valumaalueet.zip` (259 MB) | SHP zip | CC BY 4.0 | ~yearly | none | PMTiles overlay |
| 13 | SYKE flood risk areas plus flood hazard zones | SYKE | `tulvariskialueet.zip` (84 KB); hazard zones 5.6 GB ×2 | SHP zip | CC BY 4.0 | ~yearly | none | add risk areas now. Hazard zones only clipped per region, or simplified |
| 14 | GTFS: HSL and VR rail | HSL / Fintraffic | `https://infopalvelut.storage.hsldev.com/gtfs/hsl.zip` (93 MB, daily); `https://rata.digitraffic.fi/api/v1/trains/gtfs-all.zip` (13.9 MB) | GTFS zip | HSL CC BY 4.0; rail licence not verified | daily | none | raw GTFS plus stops layer in PMTiles. Timetables go stale quickly, so the stop and route geometry has more lasting value |
| 15 | FMI climate normals 1991–2020 | FMI | WFS stored query above | XML, dump to CSV (small) | CC BY 4.0 | per 30-year period | none | CSV/SQLite table plus a static HTML per station (frost dates, snow depth) |
| 16 | HSY public water posts | HSY | see findings above | SHP, 5 KB | CC BY 4.0 | occasional | none | GeoJSON overlay |
| 17 | MML municipal boundaries 1:10k, 2026 | MML via kapsi | `http://kartat.kapsi.fi/files/kuntajako/kuntajako_10k/etrs89/gpkg/TietoaKuntajaosta_2026_10k.zip` | GPKG zip, 24 MB | CC BY 4.0 | yearly | none | PMTiles boundary layer and point-in-polygon lookup |
| 18 | Fineli food composition DB | THL | `https://fineli.fi/fineli/fi/avoin-data` (linked from thl.fi/avoindata) | CSV/zip (unverified) | CC BY 4.0 (per a University of Helsinki page) | ~yearly releases | none expected | SQLite plus an HTML lookup (Finnish foods, which USDA lacks) |

- The SYKE `wwwd3.ymparisto.fi/d3/gis_data/spesific/` directory is the same host the catalog already uses for pohjavesialueet and luonnonsuojelualueet. The existing recipe pattern can probably be reused unchanged for items 7, 8, 12 and 13.
- The flood hazard zones (5.6 GB each) are too large for the 4 GB fi-maps target unless they are clipped to a region. The 84 KB flood risk areas file is the cheap first step.

### Gaps
- The 16 suomi.fi summary PDFs and the SPEK 72 tuntia guide do not state a licence. Redistribution on a stick should be confirmed (Finnish copyright §9 exempts laws and decisions, not guides).
- Fineli could not be verified. Every request to `fineli.fi` from this machine (curl with a browser User-Agent, and WebFetch) returned HTTP 403, so the file URL, size and current release number are unconfirmed. The third-party converter [mafredri/fineli-sql](https://github.com/mafredri/fineli-sql) (MIT, last push 2017, covers Releases 16 and 18) shows the data was once distributed as downloadable files. The CC BY 4.0 licence comes from a [University of Helsinki page](https://www.helsinki.fi/en/infrastructures/climate-smart-food-and-nutrition/services/food-consumption-and-nutrient-intake), not from THL directly.
- No direct bulk file URL was found for the Järvirekisteri (lake register). The guessed `jarvirekisteri.zip` returned 404.
- The springs dataset is WFS-only. Its size and feature-paging limits were not tested.
- The rail GTFS (digitraffic) licence was not confirmed.

## Maanmittauslaitos (MML) open data: what can be bulk-downloaded with and without a key?

### Takeaway
All MML open products are CC BY 4.0. The official OGC APIs (`avoin-paikkatieto.maanmittauslaitos.fi`) return 401 without an API key. The kapsi.fi mirror needs no key and serves nationwide files: the full MTK GeoPackage (22.8 GB) and MTK height GeoPackage (11.2 GB), refreshed 2026-11-04; place names (2026-09); DEM 10 m and 2 m tiles; hillshade at 2–512 m; elevation zones; municipal boundaries; and small-scale map rasters. The catalog already uses kapsi for maastokartta tiles, map sheets and MTK (Uusimaa clip), but not for names, elevation, hillshade or boundaries.

### Cited Findings
- `https://avoin-paikkatieto.maanmittauslaitos.fi/geographic-names/features/v1/collections?f=json` returned HTTP 401 without an API key (tested 2026-11-26).
- CKAN lists the official download paths as Karttapaikka file service and OGC API Processes, for example `https://avoin-paikkatieto.maanmittauslaitos.fi/tiedostopalvelu/ogcproc/v1/`. — [CKAN nimisto](https://avoindata.suomi.fi/data/fi/dataset/nimisto); [CKAN korkeusmalli-10-m](https://avoindata.suomi.fi/data/fi/dataset/korkeusmalli-10-m)
- The MML place-name products contain about 800 000 named places and their names in different languages. — [CKAN nimisto](https://avoindata.suomi.fi/data/fi/dataset/nimisto)
- Korkeusmalli 10 m covers all of Finland at 10 m × 10 m cells with 1.4 m vertical accuracy, as open data. — [CKAN korkeusmalli-10-m](https://avoindata.suomi.fi/data/fi/dataset/korkeusmalli-10-m)
- Rinnevarjoste (hillshade) is produced at 2, 8, 32, 64, 128 and 512 m pixel sizes. The 2 m version comes from DEM 2 m and the others from DEM 10 m. — [CKAN rinnevarjoste-korkeusmalli](https://avoindata.suomi.fi/data/fi/dataset/rinnevarjoste-korkeusmalli)
- kapsi mirror contents seen in its directory listings on 2026-11-26 ([kartat.kapsi.fi/files](http://kartat.kapsi.fi/files/)):

  | Path | Contents |
  |---|---|
  | `maastotietokanta/geopackage_maasto/mtkmaasto.zip` | 22 784 416 228 bytes, 2026-11-04 |
  | `maastotietokanta/geopackage_korkeus/mtkkorkeus.zip` | 11 220 799 651 bytes, 2026-11-04 |
  | `nimisto_oapif/placenames/etrs89/gml/placenames_2026_09.zip` | 53.3 MB, 2026-10-01 |
  | `nimisto_oapif/places/etrs89/gml/places_2026_09.zip` | 45.7 MB |
  | `nimisto_oapif/mapnames_{25..8000}` | cartographic name-placement sets for 25k–8000k map scales |
  | `korkeusmalli/hila_10m/etrs-tm35fin-n2000/` | 38 map-sheet folders; per-tile files are ~6–8 MB (e.g. `L4111.zip` 7.1 MB) |
  | `korkeusmalli/hila_2m/` | DEM 2 m tiles |
  | `vinovalovarjoste/hila_{2,8,32,64,128,512}m/` | hillshade; 32 m tiles are 38 KB–1.8 MB, some updated 2026-09 |
  | `korkeusvyohyke/` | elevation zones |
  | `kuntajako/kuntajako_10k/etrs89/gpkg/TietoaKuntajaosta_2026_10k.zip` | 24.4 MB, 2026-06-10 |
  | other | `maastokartta_100k`, `maastokartta_250k` (shp by sheet, 2026-06), `yleiskartta_1000k`, `yleiskartta_4500k`, `orto/`, `laser/`, `3D-rakennukset/`, `kiintopisterekisteri/` |
- The CSC Paituli mirror (`https://nic.funet.fi/index/geodata/`) is a second keyless source. It carries MML, SYKE, Luke, FMI, Kotus, Väylä (Digiroad 2025, including `pysakit/`) and Ruokavirasto field parcels, plus a `download_paituli_data.ps1` script. — [nic.funet.fi geodata](https://nic.funet.fi/index/geodata/)
- Digiroad 2025 is on Paituli at `vayla/digiroad/2025/Digiroad_K/` with a `LICENCE.TXT`. — [funet digiroad 2025](https://nic.funet.fi/index/geodata/vayla/digiroad/2025/)

### Inferences
- **Highest-value MML gaps**:
  - Place names: about 100 MB raw, much smaller as SQLite. They would give the map viewer offline search for lakes, hills and villages, which OSM covers unevenly in the wilderness.
  - 32 m hillshade: small, and makes the OSM basemap readable for terrain.
  - The `mtkkorkeus` contours clipped to Uusimaa, matching the existing `mml-topo-uusimaa` pattern.
- A nationwide 10 m DEM is likely several GB (38 sheet folders, each with many ~7 MB tiles). Total size was not measured. Terrain-RGB PMTiles at reduced zoom is the probable fit.
- Paikkatietoikkuna is only a viewer. It offers nothing to bulk-download beyond the products above.

### Gaps
- The total size of the nationwide DEM 10 m mirror and of 32 m hillshade was not summed.
- Whether kapsi's 2023-dated DEM folders reflect the current MML product was not checked.
- The cost of getting an MML API key (free registration) and its rate limits were not researched. The kapsi mirror makes them unnecessary.

## Agency-specific sources: SYKE, FMI, Luke, Ruokavirasto/Fineli, Fimea, THL, Finlex, Kotus, National Library, 72 tuntia, shelters, Digitransit, Traficom, Fingrid, Metsähallitus

### Takeaway
**Confirmed, bulk and keyless:**
- Finlex: daily ZIP packages; consolidated law is 4.4 GB
- Kotus: dictionaries as XML and the Nykysuomen sanalista
- SYKE: shapefile ZIPs
- FMI: climate normals via WFS
- suomi.fi and SPEK preparedness PDFs
- HSL and rail GTFS

**Blocked or absent:**
- Fineli: every request from here returned 403
- Civil-defence shelter locations: no open national dataset
- Metsähallitus/Retkikartta huts and trails: no open download found; LIPAS, already in the catalog, is the practical substitute
- Digitransit nationwide router data: needs a key
- Fingrid: not found on the portal
- Kielitoimiston sanakirja: no bulk download, only the derived word list

### Cited Findings
**Finlex**
- 15 ZIP packages are listed on the [Finlex download page](https://www.finlex.fi/fi/avoin-data/lataa-aineistoja) and refreshed once a day, last updated 25.11.2026:

  | Package | Size |
  |---|---|
  | Ajantasaistettu lainsäädäntö (consolidated) | 4.1 GB |
  | Suomen säädöskokoelma | 12.7 GB |
  | Säädöskäännökset | 153 MB |
  | Saamenkieliset säädökset | 71.9 MB |
  | Viranomaisten määräyskokoelmat | 2.5 GB |
  | Hallituksen esitykset | 13.2 GB |
  | Työehtosopimukset | 630 MB |
  | Sopimussarja | 734 MB |

- Verified: `https://www.finlex.fi/files/open-data/archives/statute-consolidated.zip` is 4 415 662 236 bytes, Last-Modified 2026-11-25.
- Documents are Akoma Ntoso XML. The REST API at `opendata.finlex.fi` needs no registration but may be volume-limited, and the data is CC BY 4.0. — [Finlex open data](https://data.finlex.fi/en/open-data); [Finlex avoin data](https://www.finlex.fi/fi/avoin-data)
- No KKO or KHO court decision package appears in the download list. — [Finlex download page](https://www.finlex.fi/fi/avoin-data/lataa-aineistoja)

**Kotus**
- The Kotus "Aineistot verkossa" page lists direct XML downloads under CC BY 4.0:
  - `https://kaino.kotus.fi/lataa/sms.zip` (Suomen murteiden sanakirja)
  - `suru.zip` (Suomi–ruotsi-suursanakirja)
  - `vks.zip`, `kks.zip`, `fsob.zip`, `fo.zip`
  - `suomi-somali.zip`, `suomi-kurmandzi.zip`, `suomi-sorani.zip`
  - `algu-16.3.2023.zip`

  Source: [Kotus aineistot verkossa](https://kotus.fi/kotus/kieliaineistot/aineistot-verkossa/)
- Verified sizes: sms 47.7 MB (2025-10-10), suru 11.9 MB (2025-11-06), vks 18.0 MB (2026-11-19), kks 16.7 MB (2023-11-29).
- The Nykysuomen sanalista download is `https://kaino.kotus.fi/lataa/nykysuomensanalista2024.txt` (2.86 MB, 2024-05-15). The info page licenses it CC BY 4.0. — [Nykysuomen sanalista](https://kotus.fi/sanakirjat/kielitoimiston-sanakirja/nykysuomen-sana-aineistot/nykysuomen-sanalista/)
- The Kielitoimiston sanakirja itself is linked only as a web dictionary, with no download. — [Kotus kieliaineistot](https://kotus.fi/kotus/kieliaineistot/)

**Preparedness guides**
- Sisäministeriö published the "Häiriö- ja kriisitilanteisiin varautuminen" guide on 18 Nov 2024 at suomi.fi/varautuminen, which redirects to `https://www.suomi.fi/oppaat/varautuminen`. It also has a printable PDF summary in 15 languages. — [Valtioneuvosto release](https://valtioneuvosto.fi/-/16079645/vaeston-varautumista-hairioihin-ja-kriiseihin-ohjeistetaan-suomi.fin-uudella-oppaalla)
- The guide's chapters:
  - power, water and telecom outages
  - storms and floods
  - radiation
  - hazardous substances
  - military conflict and civil defence
  - cyber
  - payment disruptions
  - pandemics
  - wildfires
  - a checklist

  Source: [suomi.fi/oppaat/varautuminen](https://www.suomi.fi/oppaat/varautuminen)
- The summaries page links 16 PDFs on `cdn.verkkopalvelu.suomi.fi/files/varautunut-parjaa-paremmin_SM_<lang>-<hash>.pdf`. Languages: suomi, ruotsi, englanti, venäjä, ukraina, viro, arabia, dari, farsi, somali, sorani, selkosuomi, selkoruotsi, and inarin-, koltan- and pohjoissaame. The Finnish file is 1.18 MB (verified). — [suomi.fi printable summaries](https://www.suomi.fi/oppaat/varautuminen/tulostettavat-ja-viittomakieliset-varautumisohjeet/varautumisohjeiden-tulostettavat-tiivistelmat)
- 72tuntia.fi links one PDF, `spek_72_tuntia_varautuminen_kotona_2025_01_21_v01.pdf` (2.94 MB, verified). — [72tuntia.fi](https://72tuntia.fi/)

**Civil-defence shelters**
- No open location dataset was found. Sisäministeriö published only counts and regional distribution of shelters (about 4.8 million places) in 2023, and Helsinki lists individual public shelters as suomi.fi service entries. — [Sisäministeriö shelter report](https://valtioneuvosto.fi/en/-/1410869/finland-has-civil-defence-shelters-for-about-4.8-million-people); [Helsinki shelters on suomi.fi](https://www.suomi.fi/services/civil-defence-shelters-the-social-services-health-care-and-rescue-services-division-city-of-helsinki/bd8e4b77-bdf9-4ece-9393-540958a4893c)

**Metsähallitus / Retkikartta**
- A Metsähallitus presentation describes Retkikartta as a viewer over datasets maintained for other purposes. No open API or download for huts, lean-tos or campfire sites was found. — [Retkikartta presentation 2022](https://www.hel.fi/hel2/tietokeskus/data/dokumentit/HelLovesDev/2022_Retkeily_ja_luonto/Tapani_Mikkola_Retkikartta_HRI_Retkeily_ja_luonto_webinaari_19052022.pdf)

**Transit (Digitransit, HSL, rail)**
- `https://api.digitransit.fi/routing-data/v3/finland/router-finland.zip` returned 401, so a key is needed.
- HSL GTFS `https://infopalvelut.storage.hsldev.com/gtfs/hsl.zip` returned 200 (93.1 MB, modified 2026-11-26). HSL data is CC BY 4.0. — [CKAN hsl-reittiopas-api](https://avoindata.suomi.fi/data/fi/dataset/hsl-reittiopas-api)
- Fintraffic rail GTFS `https://rata.digitraffic.fi/api/v1/trains/gtfs-all.zip` returned 200 (13.9 MB).
- City GTFS sets for Oulu, Vaasa, Kuopio and Tampere are on CKAN. — [CKAN gtfs search](https://avoindata.suomi.fi/data/api/3/action/package_search?q=gtfs)

**National Library (Digi)**
- Digitised newspapers from 1771–1911 (or pre-1919, depending on the source) are offered as ready-made open-data packages and via a download tool. Packages include ALTO XML. — [Kansalliskirjasto data services](https://www.kansalliskirjasto.fi/en/node/1955); [UH Comhis guide](https://wiki.helsinki.fi/xwiki/bin/view/Comhis/Comhis/En%20-%20Digi.kansalliskirjasto.fi%20Data/)

**Luke**
- `data.luke.fi` timed out from here. Paituli mirrors these Luke rasters: `forest_snow_damage_risk`, `forest_wind_damage_sensitivity`, `puulajit` (tree species), `soil-database`, `erosion_risk`, `carbon_stock`. — [funet luke](https://nic.funet.fi/index/geodata/luke/)

**THL**
- THL's open data entry on CKAN is a single link to `thl.fi/avoindata`. That page links to `https://fineli.fi/fineli/fi/avoin-data` for Fineli. — [CKAN thl-n-avoin-data](https://avoindata.suomi.fi/data/fi/dataset/thl-n-avoin-data)

### Inferences
- For fi-survival:
  - The suomi.fi preparedness PDFs and the SPEK 72 h guide are the single most on-mission gap. They are tiny, official and Finnish, and directly cover power and water outages and radiation.
  - Fineli is the next most valuable, if access can be resolved from a non-blocked network.
- For fi-reference:
  - Kotus Suomi–ruotsi plus the Nykysuomen sanalista together come to about 15 MB.
  - Finlex consolidated law is 4.4 GB raw XML. It probably shrinks a lot as filtered SQLite FTS or HTML, but that is unmeasured.
- LIPAS, already in the catalog, is the realistic stand-in for Metsähallitus huts and trails. Whether LIPAS includes autiotupa/laavu types should be checked against the existing `lipas-recreation` recipe before any Metsähallitus work.
- A shelter layer cannot be built from open data nationally. OSM `amenity=shelter`/`building=bunker` tags (already in osm-finland) are the only nationwide proxy, and their coverage is unknown.

### Gaps
- Fineli's file URL and size, and whether the 403 comes from geo or IP blocking, are unresolved.
- No Fingrid open dataset of transmission lines or substations was found on the portal (0 hits). `data.fingrid.fi` is a time-series API, and its key requirement was not verified.
- Fimea and Lääkeinfo: Fimea Perusrekisteri is already in the catalog. Pharmaca Fennica / Lääkeinfo.fi has no open bulk data that I found.
- Ruokavirasto: only field-parcel GIS (Paituli). No mushroom or food-safety guidance dataset was found.
- Yle Areena was not investigated. It is streaming content with no open bulk licence and is unlikely to be redistributable.
- Exact URLs for Digi open-data packages were not verified (`digi.kansalliskirjasto.fi/opendata`).

## Are there already-packaged offline Finnish resources?

### Takeaway
Beyond what the catalog already has (Finnish Wikipedia, Wiktionary, Wikibooks, Wikisource and Wikivoyage ZIMs, Gutenberg-fi, Partiowiki, PhET-fi, and Protomaps OSM Finland), the ready-made offline Finnish packages are the mirrors themselves: kapsi.fi (MML) and CSC Paituli on funet (MML, SYKE, FMI, Luke, Väylä, Kotus GIS). Paituli ships a bulk download script. No Finnish preparedness ZIM was found.

### Cited Findings
- kapsi.fi mirrors MML products as plain HTTP directory trees, refreshed monthly for MTK (2026-11-04). — [kartat.kapsi.fi/files](http://kartat.kapsi.fi/files/)
- Paituli at `nic.funet.fi/index/geodata` mirrors: dvv, hy, ilmatiede, kotus (dialect-area GIS `pitajat`, `sms_rajat`), luke, mml, ruokavirasto, syke, tilastokeskus and vayla. It includes `download_paituli_data.ps1` and Digiroad 2025 with a licence file. — [funet geodata](https://nic.funet.fi/index/geodata/)
- Kotus already publishes its dictionaries as ready XML ZIPs (see the previous section). — [Kotus aineistot verkossa](https://kotus.fi/kotus/kieliaineistot/aineistot-verkossa/)

### Inferences
- The kapsi and Paituli mirrors can be the acquisition source for most new map items. They are keyless, stable and plain HTTP, which fits the existing recipe style used for `mml-maastokartta` and `mml-topo-uusimaa`.
- No Kiwix ZIM exists for suomi.fi guides or 72tuntia, so they would need to be captured as PDFs (simplest), or via zimit if the HTML guide is wanted.

### Gaps
- The Kiwix library was not searched for a zimit capture of suomi.fi or other Finnish government sites. This should be confirmed against `download.kiwix.org/zim/zimit/`.
