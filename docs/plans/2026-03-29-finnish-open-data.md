# Finnish Open Data Sources for Svalbard

Catalog of open Finnish datasets worth evaluating for inclusion in Nordic presets. All CC BY 4.0 unless noted. Organized by source organization with survival/knowledge relevance notes.

## Status

**Assessment phase** — nothing added yet. Each dataset needs evaluation for:
- Actual download size and format practicality
- Whether it adds value over what OSM + maastokartat already cover
- How to package for offline use (viewer requirements, format conversion)
- Which preset tiers it belongs in (32 GB vs 2 TB is a big range)

---

## 1. Maanmittauslaitos (MML) — National Land Survey

Download portal: https://tiedostopalvelu.maanmittauslaitos.fi/tp/kartta
Product catalog: https://www.maanmittauslaitos.fi/kartat-ja-paikkatieto/aineistot-ja-rajapinnat

We already include MML maastokartat (raster topo maps) and plan to include OSM. These are the additional MML datasets worth considering.

### Maastotietokanta (Topographic Database — vector)
- **What**: Raw vector data behind the maastokartta raster maps. Buildings, roads, waterways, contour lines, land cover, power lines, fences as queryable features.
- **Why**: Enables queries like "find all bridges within 5km" or "locate power lines" — far more powerful than raster maps for programmatic use.
- **Format**: GeoPackage, Shapefile, GML
- **Size**: ~20–40 GB for all Finland
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/maastotietokanta
- **MML product page**: https://www.maanmittauslaitos.fi/kartat-ja-paikkatieto/aineistot-ja-rajapinnat/tuotteet/maastotietokanta

### Korkeusmalli 10 m (DEM)
- **What**: Digital elevation model, 10 m grid, ~1.4 m vertical accuracy. Derived from LiDAR.
- **Why**: Terrain analysis, flood risk, drainage prediction, route planning, finding sheltered locations.
- **Format**: GeoTIFF (per map sheet)
- **Size**: ~15–60 GB for all Finland (estimates vary)
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/korkeusmalli-10-m
- **MML product page**: https://www.maanmittauslaitos.fi/kartat-ja-paikkatieto/aineistot-ja-rajapinnat/tuotteet/korkeusmalli-10-m
- **Note**: 2 m DEM also exists but is many hundreds of GB — impractical except for selected regions.

### Paikannimirekisteri (Place Name Register)
- **What**: ~800,000 geographic place names with coordinates. Finnish, Swedish, Sámi. Covers natural features, populated places.
- **Why**: Finnish place names encode survival info: "Lähde-" = spring, "Suo-" = swamp, "Kallio-" = bedrock, "Lampi-" = pond, "Koski-" = rapids. Searchable water/terrain finder.
- **Format**: GeoPackage, Shapefile, GML, Excel
- **Size**: ~200–500 MB
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/nimisto
- **MML product page**: https://www.maanmittauslaitos.fi/kartat-ja-paikkatieto/aineistot-ja-rajapinnat/tuotteet/paikannimirekisteri

### Kiinteistörekisterikartta (Cadastral Map)
- **What**: Property boundaries, parcel identifiers, property unit types.
- **Why**: Identify state-owned vs. private land. Less critical for immediate survival, more for longer-term settlement.
- **Format**: GeoPackage, Shapefile, GML
- **Size**: ~5–15 GB
- **MML product page**: https://www.maanmittauslaitos.fi/kartat-ja-paikkatieto/aineistot-ja-rajapinnat/tuotteet/kiinteistorekisterikartta

### Ortoilmakuvat (Aerial Orthophotos)
- **What**: Color aerial imagery, ~0.5 m resolution. Updated regionally on a cycle.
- **Why**: See actual buildings, clearings, paths, water clarity — things topo maps abstract away.
- **Format**: JPEG2000, GeoTIFF
- **Size**: Many terabytes for full Finland — only practical as curated regional subsets
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/maanmittauslaitoksen-ortokuva
- **MML product page**: https://www.maanmittauslaitos.fi/kartat-ja-paikkatieto/aineistot-ja-rajapinnat/tuotteet/ortoilmakuvat

---

## 2. SYKE — Finnish Environment Institute

Download portal: https://paikkatieto.ymparisto.fi/lapio/
Direct ZIP downloads: https://wwwd3.ymparisto.fi/d3/gis_data/spesific/
Open data overview: https://www.syke.fi/avoindata

### Pohjavesialueet (Groundwater Areas)
- **What**: All classified groundwater areas/aquifer zones with boundaries, classification, estimated yield.
- **Why**: Answers "where can I find clean water?" — critical survival data.
- **Format**: Shapefile (ZIP)
- **Size**: ~25 MB
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/pohjavesialueet
- **Direct download**: https://wwwd3.ymparisto.fi/d3/gis_data/spesific/pohjavesialueet.zip

### CORINE Maanpeite 2018 (Land Cover)
- **What**: Land cover classification at 20 m resolution. Forest types, arable land, pasture, peat bogs, marshes, water, urban. Finnish version has extra detail beyond EU standard.
- **Why**: Know what terrain surrounds you — where are fields (food), bogs (fuel/avoid), forests by type.
- **Format**: GeoTIFF raster, also vector versions
- **Size**: ~100–200 MB compressed
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/corine-maanpeite-2018

### Valuma-aluejako (Watershed Boundaries)
- **What**: Hierarchical watershed boundaries at 5 levels of detail, covering all of Finland.
- **Why**: Understand water flow, upstream/downstream contamination risk.
- **Format**: Shapefile (ZIP)
- **Size**: ~29 MB (1990 version direct download available)
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/valuma-aluejako
- **Direct download**: https://wwwd3.ymparisto.fi/d3/gis_data/spesific/valumaalueet.zip

### Ranta10 — Rantaviiva (Shoreline Database)
- **What**: Detailed shorelines of all Finnish lakes (180k+), rivers, and coastline at 1:10,000.
- **Why**: Precise water body outlines for navigation and water access planning.
- **Format**: Shapefile, GeoPackage
- **Size**: ~500 MB – 1 GB

### Luonnonsuojelu- ja erämaa-alueet (Protected Areas & Wilderness)
- **What**: Boundaries of nature reserves (state and private), wilderness areas, Natura 2000 sites.
- **Why**: Protected areas = intact ecosystems, better game/fish/forage, undisturbed water sources.
- **Format**: Shapefile (ZIP)
- **Size**: ~69 MB total
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/luonnonsuojelu-ja-eramaa-alueet

### Tulvavaaravyöhykkeet (Flood Hazard Zones)
- **What**: Flood risk maps for different recurrence intervals.
- **Why**: Don't build your shelter in a flood zone.
- **Format**: Shapefile (ZIP), ESRI REST
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/tulvavaaravyohykkeet-perusskenaariot-flood-hazard-zones-basic-scenarios

### Maatalousmaa 2021 (Agricultural Land)
- **What**: All agricultural parcels in Finland + what was grown on each.
- **Why**: Shows where arable land is and what crops it supports.
- **Format**: ZIP
- **Size**: ~344 MB
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/maatalousmaa-2021

### Metsäkasvillisuusvyöhykkeet (Forest Vegetation Zones)
- **What**: Finland divided into boreal sub-zones with vegetation type boundaries.
- **Why**: What you can grow, what wild plants to expect, what tree species dominate.
- **Format**: Shapefile (ZIP)
- **Size**: ~537 KB (tiny)
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/metsakasvillisuusvyohykkeet

### Virtavesien lohikalakannat (Salmon/Trout Stocks)
- **What**: Presence and distribution of salmon and trout in Finnish rivers and streams.
- **Why**: Where to fish for protein.
- **Format**: Shapefile (ZIP)
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/virtavesien-lohikalakannat

### Pintavesien vedenlaatu — VESLA (Surface Water Quality)
- **What**: Historical physicochemical water quality measurements from Finnish lakes, rivers, Baltic Sea.
- **Why**: Which water sources are safe to drink (with/without treatment).
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/pintavesien-tilan-vedenlaatuaineisto-vesla

---

## 3. GTK — Geological Survey of Finland

Download portal: https://hakku.gtk.fi

### Maaperä 1:200 000 (Surficial Geology / Soil Map)
- **What**: Surface soil/sediment types: clay, silt, sand, gravel, till, peat, bedrock outcrops.
- **Why**: Find building ground (avoid clay/peat), locate sand/gravel for construction, identify well-drained areas, find clay for pottery/sealing, know what's passable in rain.
- **Format**: ESRI File Geodatabase, Shapefile
- **Size**: ~1–3 GB (1:200k full coverage)
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/maapera-1-200-000-maalajit2
- **Note**: 1:20k version also exists with better detail but only partial coverage (~5–15 GB).

### Kallioperä 1:100 000 (Bedrock Map)
- **What**: Bedrock lithology (granite, gneiss, schist, quartzite, limestone, etc.) and structural features.
- **Why**: Stone for construction/tools, limestone for lime production, fault zones indicate springs.
- **Format**: Shapefile, ESRI File Geodatabase
- **Size**: ~500 MB – 2 GB
- **Avoindata (1:100k)**: https://avoindata.suomi.fi/data/fi/dataset/kalliopera-1-100-0001
- **Avoindata (1:1M)**: https://avoindata.suomi.fi/data/fi/dataset/kalliopera-1-1-000-000

### Turvevarojen kartoitus (Peat Resources)
- **What**: Locations and properties of peat deposits — depth, type, quality, volume.
- **Why**: Peat = fuel, insulation, water filtration. Historically heated Finnish homes.
- **Format**: Shapefile
- **Size**: ~200–500 MB

### Happamat sulfaattimaat (Acid Sulfate Soils)
- **What**: Acid sulfate soil distribution along coastal Finland.
- **Why**: Toxic for agriculture and water. Know where NOT to farm or dig wells.
- **Format**: Shapefile
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/happamat-sulfaattimaat-1-250-0001

---

## 4. Ilmatieteen laitos (FMI) — Finnish Meteorological Institute

Open data portal: https://opendata.fmi.fi/
Overview: https://www.ilmatieteenlaitos.fi/avoin-data

### Ilmastolliset vertailuarvot (Climate Normals 1991–2020)
- **What**: 30-year averages for ~400 stations: monthly temp, precip, snow depth, frost dates, growing season, wind, sunshine.
- **Why**: Essential for agriculture (when to plant, frost dates), shelter design (heating needs), seasonal planning.
- **Format**: WFS (XML/GML) — needs preprocessing to static files
- **Size**: ~50–200 MB when compiled
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/saahavaintojen-ilmastolliset-vertailuarvot1

### Hilamuotoiset kuukausiarvot (Gridded Monthly Climate)
- **What**: 1 km grid monthly temperature and precipitation from 1961 onwards.
- **Why**: Know exactly how much rain/snow falls where and when. Crop planning, shelter design.
- **Format**: WFS/GML — needs preprocessing
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/saahavaintojen-hilamuotoiset-kuukausiarvot

### Ilman radioaktiivisuusvalvonta (Radioactivity Baseline)
- **What**: Baseline radioactivity measurements at monitoring stations.
- **Why**: Needed to interpret future measurements in a nuclear scenario.
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/ilman-radioaktiivisuusvalvonta

---

## 5. Metsähallitus — Parks & Wildlife Finland

Open data: https://www.metsa.fi/avoin-data/

### Retkikartta Data (Trails, Huts, Shelters)
- **What**: National park boundaries, hiking trails, autiotuvat (open wilderness huts), laavut (lean-tos), campfire sites, drinking water points, nature trails.
- **Why**: A database of shelters, fire-making spots, maintained trails, and water points. Autiotupa locations alone could be life-saving.
- **Format**: GeoJSON, Shapefile, KML (via WFS)
- **Size**: ~100–500 MB
- **Portal**: https://www.retkikartta.fi

### Valtion maat (State Land Boundaries)
- **What**: Boundaries of state-owned land managed by Metsähallitus.
- **Why**: State forests have broader access rights — where you can most freely operate.
- **Format**: Shapefile/GeoPackage
- **Size**: ~50–200 MB

---

## 6. Väylävirasto — Transport Infrastructure Agency

Open data: https://vayla.fi/avoindata

### Digiroad (National Road Network)
- **What**: Complete road/street network with surface type (paved/gravel/dirt), speed limits, weight limits, width, bridge data, ferry routes, seasonal restrictions. Includes forest roads.
- **Why**: Authoritative road surface and condition attributes that OSM may lack, especially for metsäautotiet (forest roads).
- **Format**: GeoPackage, Shapefile, CSV
- **Size**: ~5–10 GB
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/digiroad
- **Download**: https://vayla.fi/avoindata/digiroad

### Vesiväylät (Waterways / Navigational Channels)
- **What**: Navigational channels, fairways, depth data for inland and coastal waterways.
- **Why**: Boat navigation in Finland's lake system (Saimaa etc.) and coastal waters.
- **Format**: Shapefile, GeoPackage
- **Size**: ~500 MB – 2 GB

---

## 7. Luke — Natural Resources Institute Finland

Open data: https://kartta.luke.fi/opendata/

### Monilähde-VMI (Multi-Source Forest Inventory)
- **What**: 16 m rasters: dominant tree species, age, timber volume, basal area, mean height/diameter.
- **Why**: Find construction timber (mature spruce), firewood (birch), resin (pine). Young forests indicate logging roads.
- **Format**: GeoTIFF
- **Size**: ~5–15 GB
- **Avoindata (stand data)**: https://avoindata.suomi.fi/data/fi/dataset/metsavarakuviot
- **Avoindata (grid data)**: https://avoindata.suomi.fi/data/fi/dataset/metsakeskuksen-hila-aineisto

### Marjasato / Sienisato (Berry & Mushroom Yields)
- **What**: Regional wild berry (mustikka, puolukka, lakka) and mushroom yield patterns.
- **Why**: Foraging planning — which regions have good yields.
- **Size**: ~10–50 MB

---

## 8. Other Sources

### Luomus — Finnish Museum of Natural History

#### Lajitietokeskus (Biodiversity Information)
- **What**: Species data for 45k+ species, 45M+ observations. Plants, fungi, animals.
- **Why**: Digital field guide — which plants/mushrooms/berries are edible vs. poisonous.
- **Format**: CSV, GeoPackage, Shapefile (bulk)
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/lajitietokeskus
- **Portal**: https://laji.fi

#### Kasviatlas (Plant Atlas)
- **What**: Distribution maps for Finnish vascular plants.
- **Why**: Which edible/poisonous plants grow in which part of Finland.
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/kasviatlas

#### Lintuatlas (Bird Atlas)
- **What**: Breeding bird distributions across three atlas periods. Includes 17 MB PDF atlas.
- **Why**: Game bird locations = protein sources. Compact reference.
- **Format**: CSV (1.1 MB), PDF (17 MB)
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/lintuatlas

### Fimea — Finnish Medicines Agency

#### Pharmaceutical Registry
- **What**: All marketed medicines — active ingredients, dosages, forms, interchangeability, high-risk classification.
- **Why**: Know what medicines contain, what substitutes exist. Life-saving for rationing/substituting medical supplies.
- **Format**: XML, TXT
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/fimea-avoin-data

### Museovirasto — Finnish Heritage Agency

#### Muinaisjäännösrekisteri (Archaeological Sites)
- **What**: All registered archaeological sites — ancient dwellings, iron age forts, historical ruins.
- **Why**: Ancient sites were chosen for water, defense, fertile soil. Settlement location wisdom.
- **Format**: Shapefile, WFS
- **Size**: ~50–200 MB
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/muinaisjaannokset-muinaisjaannosrekisteri

### Joukahainen (Finnish Language Database)
- **What**: XML vocabulary database, ~100k+ words with forms and classifications.
- **Why**: Compact Finnish language reference. 610 KB — essentially free to include.
- **Avoindata**: https://avoindata.suomi.fi/data/fi/dataset/joukahainen-suomen-kielen-sanastotietokanta

---

## Priority Matrix

How to think about what to include in which preset tiers:

### Always include (tiny, enormous value)
These total under 1 GB and should go in every preset from nordic-32 up:
- Pohjavesialueet (~25 MB) — water
- Paikannimirekisteri (~300 MB) — navigation
- Metsäkasvillisuusvyöhykkeet (~537 KB) — ecology
- Luonnonsuojelualueet (~69 MB) — wilderness
- Joukahainen (~610 KB) — language
- Lintuatlas (~18 MB) — field guide
- Fimea pharmaceutical registry (small) — medicine
- FMI Climate Normals (~100 MB) — agriculture/planning

### Include from 128 GB up
- CORINE land cover (~200 MB)
- Valuma-aluejako (~29 MB)
- GTK maaperä 1:200k (~2 GB)
- GTK kallioperä (~1 GB)
- Retkikartta trails/huts (~200 MB)
- Tulvavaaravyöhykkeet (flood zones)
- Virtavesien lohikalakannat (fish stocks)
- Maatalousmaa (~344 MB)

### Include from 256 GB up
- Maastotietokanta vectors (~30 GB)
- Korkeusmalli 10 m DEM (~15–60 GB)
- Digiroad (~8 GB)
- Luke forest inventory (~10 GB)
- Lajitietokeskus bulk export
- Ranta10 shorelines (~700 MB)

### Include from 1 TB up
- Vesiväylät (~1 GB)
- GTK turvevarojen kartoitus (~300 MB)
- GTK pohjaveden syvyys (~500 MB)
- Happamat sulfaattimaat
- VESLA water quality
- Muinaisjäännösrekisteri
- Regional orthophotos (selected areas)

---

## Open Questions

- **Viewer requirements**: Most geo data needs QGIS or similar. Should we bundle a lightweight offline GIS viewer? Or pre-render everything to map tiles?
- **Format standardization**: Should everything be converted to GeoPackage for consistency?
- **FMI data preprocessing**: Climate normals are only available via WFS queries — need a build step to compile to static CSV/SQLite.
- **Lajitietokeskus scope**: The full 45M observation dump is huge. Better to curate a species identification subset focused on edible/medicinal/dangerous species?
- **Retkikartta access**: Need to verify bulk download availability vs. WFS-only.
- **MML download automation**: Their file service works per map sheet — need scripting to bulk-download all of Finland.
