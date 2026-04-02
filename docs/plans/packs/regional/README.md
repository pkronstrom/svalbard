# Pack: Regional

Notes on expanding beyond Finland to other regional content packs.

## Audience

Users in specific geographic regions who need localized content.

## Status

Idea -- collecting candidates for future regional packs

---

## What Makes a Regional Pack

The Finland pack (documented in `2026-03-29-finnish-survival-pack-plan.md`) established a pattern for what a regional pack contains. A regional pack is not "everything about a country" -- it is a curated bundle of locally relevant offline content that complements the universal default presets.

### Core Components (every regional pack)

| Component | Purpose | Format | Example (Finland) |
|-----------|---------|--------|-------------------|
| Local language Wikipedia | General reference in local language | ZIM | `wikipedia-fi-all` |
| Local language Wiktionary | Dictionary and translation | ZIM | `wiktionary-fi` |
| OSM basemap extract | Offline maps for the region | PMTiles | Finland z15 extract (~0.7-1 GB) |
| Emergency information | Local emergency numbers, procedures, agencies | static HTML/PDF | Finnish emergency services reference |

### Extended Components (where available)

| Component | Purpose | Format | Example (Finland) |
|-----------|---------|--------|-------------------|
| Government open geodata | Terrain, water, land use, protected areas | GeoPackage + PMTiles | SYKE, MML datasets |
| Local flora/fauna guides | Species identification, foraging | ZIM or custom build | Pinkka, Kasviatlas |
| Shelter/infrastructure data | Huts, trails, campfire sites, shelters | GeoPackage + PMTiles | LIPAS, Retkikartta |
| Pharmaceutical/medical registry | Local medicine names and substitutions | SQLite | Fimea |
| Legal reference | Key laws relevant to land access, foraging, self-defense | static HTML/PDF | Everyman's rights, hunting regulations |
| Local language resources | Grammar, phrasebook, learning material | ZIM or static | Joukahainen |
| Topographic data | Elevation models, contour lines | GeoTIFF + PMTiles | MML Korkeusmalli 10m |
| Agricultural data | Soil types, crop zones, agricultural land | GeoPackage + PMTiles | Maatalousmaa |

### What Does Not Belong in a Regional Pack

- Content that is already in the universal default preset (English Wikipedia, English medical references, etc.)
- Raw data dumps without a clear use case or viewer
- Commercial or restricted-license content requiring per-user agreements
- Extremely large datasets (>50 GB) unless broken into optional tiers

---

## The Finland Pattern

The Finland regional pack is the first and most developed. Its structure serves as a template:

**Map layers** (GeoPackage canonical + PMTiles browser): Retkikartta/LIPAS structures, Pohjavesialueet (groundwater), Kasviatlas (plant distribution), Virtavesien lohikalakannat (salmonid rivers), Luonnonsuojelu- ja eraamaa-alueet (protected areas).

**Reference sidecars** (SQLite + HTML viewer): Fimea pharmaceutical registry, Joukahainen language reference.

**OSM basemap**: Finland PMTiles extract from Protomaps daily build.

**Local Wikipedia**: `wikipedia-fi-all` or `wikipedia-fi-nopic` depending on tier.

**Expansion waves**: Water/shelter, foraging/species, terrain/mobility, long-term settlement -- each wave adds coherent capability rather than random data.

Key decisions that should carry forward to other regions:
- Preserve raw geodata in canonical formats; derive browser-viewable PMTiles
- Every map layer gets a short human-readable explanation
- Reference sidecars get FTS5 search
- Pack tiers scale from 2 GB (essentials) through 32-512 GB (comprehensive)

---

## Candidate Regions

### Tier 1: Strong Open Data, Clear Use Case

These regions have mature open data ecosystems and are strong candidates for the next regional packs.

| Region | Languages | Key Open Data Sources | Notes |
|--------|-----------|----------------------|-------|
| **Sweden** | sv | Lantmateriet (open topo/geodata since 2020), Naturvardsverket (protected areas), SCB statistics, SMHI (weather/hydrology), Artportalen (species observations) | Very similar to Finland. Allemansratten (freedom to roam). Swedish Wikipedia already in recipes |
| **Norway** | no | Kartverket (mapping authority, open data), Miljodirektoratet (environment), Artsdatabanken (species), Geonorge.no portal | Strong open geodata tradition. Norwegian Wikipedia already in recipes |
| **Denmark** | da | Dataforsyningen (geodata portal), Miljoportalen, Danmarks Miljoportal, Artsportalen.dk | Smaller land area, flatter terrain. Danish Wikipedia already in recipes |
| **Estonia** | et | Maa-amet (land board, open geodata), Estonian Nature Observation database, Riigi Teataja (legal) | Small country, strong digital infrastructure. Estonian Wikipedia already in recipes |

### Tier 2: Good Open Data, More Complexity

| Region | Languages | Key Open Data Sources | Notes |
|--------|-----------|----------------------|-------|
| **Germany** | de | BKG (federal geodata), BfN (nature conservation), Geoportal.de, OpenData portals per Bundesland | Federal structure means data is spread across 16 states. Large area. German Wikipedia in recipes |
| **Switzerland** | de, fr, it | swisstopo (excellent open geodata since 2021), BAFU (environment), map.geo.admin.ch | Multilingual complexity. Excellent topo data quality |
| **Austria** | de | BEV (mapping), data.gv.at portal, Naturschutzbund | Smaller, German-language, alpine terrain |
| **United Kingdom** | en | OS OpenData, Natural England, NRW (Wales), NatureScot, DEFRA, data.gov.uk | Extensive open data. English-language simplifies text content |
| **France** | fr | IGN (geodata), INPN (biodiversity), data.gouv.fr, Geoportail | Large country, strong open data policies. French Wikipedia would need recipe |

### Tier 3: Partial Open Data, Higher Effort

| Region | Languages | Key Open Data Sources | Notes |
|--------|-----------|----------------------|-------|
| **North America (US/Canada)** | en, fr | USGS, NOAA, NPS, Parks Canada, Canadian GeoGratis, StatCan | Enormous area; would need to be broken into sub-regions (Pacific NW, Northeast, etc.) |
| **Australia/NZ** | en | Geoscience Australia, BOM, Atlas of Living Australia, LINZ (NZ) | English-language, good open data, unique ecosystems |
| **Baltics (Latvia, Lithuania)** | lv, lt | Various national geodata portals | Smaller datasets, languages less commonly supported in Kiwix |
| **Iceland** | is | Landmalingar Islands, Natturfraedistofnun Islands | Small population, unique geological/ecological interest |
| **Japan** | ja | GSI (Geospatial Information Authority), Biodiversity Center, MLIT | Excellent geodata, but Japanese-language packaging adds complexity |

---

## Source Candidates by Category

These are source types to investigate for each new region, not specific URLs.

| Category | What to Look For | Typical Format |
|----------|-----------------|----------------|
| National mapping authority | Topo maps, elevation data, administrative boundaries | GeoPackage, GeoTIFF, WMS/WFS |
| Environmental agency | Protected areas, water bodies, land cover, flood zones | GeoPackage, Shapefile |
| Biodiversity portal | Species observations, habitat maps, red lists | API, CSV, GeoJSON |
| Emergency services | Emergency numbers, civil protection, shelter locations | HTML, PDF |
| Pharmaceutical registry | Medicine database with local brand names | XML, CSV, SQLite |
| Legal access rights | Freedom to roam, hunting/fishing regulations, foraging law | PDF, HTML |
| Wikipedia | Local language edition | ZIM (check Kiwix library) |
| Wiktionary | Local language dictionary | ZIM (check Kiwix library) |
| OSM extract | Regional map tiles | PMTiles via Protomaps extract |
| Weather/climate | Climate normals, weather pattern references | CSV, GeoTIFF |
| Agricultural authority | Soil maps, crop zones, growing season data | GeoPackage, GeoTIFF |

---

## Tiering Notes for Regional Packs

Regional packs should scale in the same tiers as the Finland pack:

- **Minimal** (~2 GB): Local language Wikipedia (nopic), OSM basemap, emergency reference.
- **Core** (~8-15 GB): Add local language Wikipedia (full), key geodata layers (protected areas, water, shelter), pharmaceutical reference.
- **Extended** (~30-60 GB): Add terrain data, species data, agricultural data, topographic detail.
- **Comprehensive** (~100-200 GB): Add elevation models, orthophotos, full species databases, historical maps.

Not every region needs all tiers. A minimal pack is useful even for regions with limited open data.

---

## Relationship to Existing Content

Several regional Wikipedia and Wiktionary ZIMs are already in recipes:

| Language | Wikipedia | Wiktionary | Notes |
|----------|-----------|------------|-------|
| Finnish (fi) | `wikipedia-fi-all`, `wikipedia-fi-nopic` | `wiktionary-fi` | Active Finland pack |
| Swedish (sv) | `wikipedia-sv-all`, `wikipedia-sv-nopic` | -- | Ready for Sweden pack |
| Norwegian (no) | `wikipedia-no-all`, `wikipedia-no-nopic` | -- | Ready for Norway pack |
| Danish (da) | `wikipedia-da-all`, `wikipedia-da-nopic` | -- | Ready for Denmark pack |
| Estonian (et) | `wikipedia-et-nopic` | -- | Partial; no full version |
| German (de) | `wikipedia-de-all`, `wikipedia-de-nopic` | -- | Ready for DACH region |
| Russian (ru) | `wikipedia-ru-nopic` | -- | Partial; no full version |
| English (en) | multiple variants | `wiktionary-en` | Default preset, not regional |

The existing Finland presets (`finland-2` through `finland-2tb`) demonstrate the tier scaling model. New regions would follow the same `{region}-{size}` naming convention.

---

## Implementation Priority

1. **Nordics first**: Sweden, Norway, Denmark share similar open data ecosystems, similar legal frameworks (freedom to roam), and similar ecological context. Much of the Finland tooling (PMTiles pipeline, geodata ingest) can be reused.
2. **Estonia**: Already has Wikipedia in recipes. Small country, manageable scope. Good testing ground.
3. **DACH (Germany/Austria/Switzerland)**: Large user base, excellent open data, but federal complexity (especially Germany) increases effort.
4. **UK**: English-language simplifies content, OS OpenData is well-structured.
5. **Everything else**: Driven by user interest and contributor availability.

---

## Open Questions

- Should Nordic countries be one combined "Nordics" pack or separate per-country packs? A combined pack would share the OSM basemap and reduce duplication, but each country has its own language, geodata sources, and legal framework.
- What is the minimum viable regional pack? Is local Wikipedia + OSM basemap enough to justify a pack, or should there be at least one geodata layer?
- How to handle multilingual regions (Switzerland, Belgium, Canada)? Multiple Wikipedia ZIMs per pack, or let the user choose?
- Should regional packs include translated/localized versions of universal content (e.g., Finnish-language medical guides) or only region-specific content?
- How to manage the combinatorial explosion of region x tier presets? Finland already has 8 preset files. If we add 5 more regions, that is 40+ preset files.
- Is there a standard way to describe "freedom to roam" or "public access rights" across regions, so users can quickly understand what is legally permissible in each country?
- Should regional packs include offline routing data (Valhalla tiles) for the region, or should routing be a separate cross-cutting feature?
