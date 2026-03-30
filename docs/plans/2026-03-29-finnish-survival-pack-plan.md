# Finnish Survival Pack Plan

## Status

**Planning phase** — based on dataset review and prioritization discussion on 2026-03-29.

---

## Goal

Define the first practical Finnish regional data pack for Svalbard.

This pack should optimize for:

- real survival usefulness in Finland
- offline browser-based map viewing first
- low to moderate ingest complexity
- preserving original datasets while also producing a simple map UX

The first pack is **not** "all Finnish open data." It is a curated regional bundle centered on the datasets that are most likely to help someone make field decisions.

---

## Product Direction

The first Finnish pack should be a **general survival pack** with strong support for:

- shelter and route finding via `Retkikartta`
- water sourcing via `Pohjavesialueet`
- food gathering via plant and fish datasets
- wilderness context via protected-area layers

The map UX is the primary interface. Data should be usable in a browser on the drive without requiring QGIS or other desktop GIS tools.

`Kiwix` / `ZIM` should continue to be used for article-like reference content. Most Finnish open datasets discussed here are geodata and should remain geodata rather than being converted to ZIM.

---

## Packaging Model

For the regional pack, the data model should be:

- preserve the original dataset in a stable offline format
- derive a browser-viewable map representation
- add a short human-readable note explaining what the layer means and how to use it

### Canonical preservation formats

| Data type | Preserve as | Notes |
|----------|-------------|-------|
| Vector geodata | `GeoPackage` | Preferred canonical vector format when possible |
| Raster geodata | `GeoTIFF` | Preferred canonical raster format |
| Tabular/reference data | `CSV`, `SQLite`, or original vendor format | Depends on source |

### Browser-facing formats

| Use case | Derived format |
|----------|----------------|
| Interactive offline map layers | `PMTiles` |
| Searchable non-map reference | small `SQLite` / `JSON` / `CSV` index plus HTML viewer |
| Documentation / explanation | Markdown or generated HTML |

### Rule of thumb

- Preserve raw geodata.
- Derive `PMTiles` for browser viewing.
- Do not convert GIS datasets into `ZIM` unless the source is actually article-like content.

---

## Simple Ingest Architecture (v1)

The ingest system should stay deliberately small in the first implementation.

The goal is not to build a general workflow engine. The goal is to make it easy to:

- declare datasets in a simple config
- reuse logic across similar source types
- resume interrupted work
- rerun datasets when the source or config changes

### Source families

The first implementation only needs a few source families:

| Family | Meaning | Typical examples |
|-------|---------|------------------|
| `vector-static` | Downloadable vector geodata file or archive | `Pohjavesialueet`, `Virtavesien lohikalakannat`, `Luonnonsuojelu- ja erämaa-alueet` |
| `vector-service` | Service-backed vector data that must be snapshotted | `Kasviatlas` |
| `reference-static` | Downloadable structured reference dataset | `Fimea`, `Joukahainen` |
| `custom` | Source with special fetch logic but standard downstream outputs | `Retkikartta` |

This should cover the first pack without introducing unnecessary abstraction.

### Pipeline stages

Each dataset should move through the same small set of stages:

1. `fetch`
2. `normalize`
3. `package`
4. `describe`

Meaning:

- `fetch`: download or snapshot the source data
- `normalize`: convert it into a stable canonical local format
- `package`: build browser-facing or searchable outputs
- `describe`: write metadata and human-facing notes

This keeps the user-facing mental model simple while still allowing custom source logic inside a stage when needed.

### Minimal shared handlers

The first implementation should prefer a few shared handlers rather than many small nodes.

Suggested v1 handler set:

- `FetchSource`
- `NormalizeSource`
- `PackageSource`
- `DescribeSource`

Inside those handlers, source-family-specific behavior can branch by configuration.

For example:

- `vector-static` fetch may do direct download plus archive unpacking
- `vector-service` fetch may do a WFS or similar snapshot
- `reference-static` normalize may convert XML/TXT/CSV into a local searchable form
- `custom` may use source-specific code only for the fetch step

### Why not more nodes?

The source catalog is not large enough yet to justify a full graph-based system.

For v1, a smaller shared interface is better because:

- the same contributors who define sources should be able to understand the whole pipeline
- there are only a few real source patterns so far
- over-abstraction would slow down the first useful pack

If later work reveals clearly different recurring behaviors, the internals can be split further without changing the top-level dataset model.

---

## Dataset Configuration Model

Dataset configuration should be declarative and intentionally approachable.

Most datasets should be addable without writing Python.

### Design goals

- simple enough that someone can define a source by editing YAML
- powerful enough to reuse shared ingest logic
- flexible enough to allow custom source adapters where necessary

### Example minimal config

```yaml
id: pohjavesialueet
family: vector-static
source:
  url: https://example.invalid/pohjavesialueet.zip
normalize:
  layer_name: pohjavesialueet
package:
  map: true
  raw: true
viewer:
  name: Groundwater Areas
  category: water
license:
  attribution: Finnish Environment Institute
```

### Expected config shape

| Section | Purpose |
|--------|---------|
| `id` | Stable dataset identifier |
| `family` | Which shared ingest path the dataset uses |
| `source` | URL, endpoint, or source-specific fetch parameters |
| `normalize` | Canonicalization hints such as layer names or field choices |
| `package` | Whether to produce map layers, raw archives, reference indexes, or both |
| `viewer` | Labels and categorization for offline UX |
| `license` | Attribution and license text |

### Config philosophy

- most datasets should select a source family and provide parameters
- only unusual sources should require custom code
- the complexity should live in reusable ingest helpers, not in each dataset definition

---

## Progress, Resume, and Update Tracking (v1)

Tracking should also stay simple.

The first implementation does not need a workflow database or complex run history.

### State model

Each dataset should have a small state file that records:

- `source_version`
- `checksum` if available
- `config_hash`
- `completed_stage`
- artifact paths

Example:

```yaml
id: pohjavesialueet
source_version: https://example.invalid/pohjavesialueet_2026.zip
checksum: abc123
config_hash: def456
completed_stage: package
artifacts:
  raw: regional/finland/raw/pohjavesialueet.zip
  canonical: regional/finland/canonical/pohjavesialueet.gpkg
  packaged: maps/regional/pohjavesialueet.pmtiles
```

### Resume behavior

On rerun:

- if the stage is already complete and the expected artifact exists, skip it
- if an artifact is missing, continue from the missing stage
- if the run was interrupted, resume from the last incomplete stage

### Update behavior

For v1, update logic should be intentionally basic:

- if `source_version` or `checksum` changed, rerun from `fetch`
- if `config_hash` changed, rerun from `normalize`
- if the packaged artifact is missing, rerun from the missing stage
- otherwise skip

This is enough for small and medium straightforward downloads without over-building the system.

### Why this is enough for now

The first-wave datasets are mostly static downloads or small service snapshots.

A simple stage-based state file should be sufficient to support:

- resumable downloads
- resumable transforms
- easy inspection of what happened
- straightforward updates when a source changes

More advanced invalidation or provenance logic should only be added if a real source requires it.

---

## First Pack: Included Now

These are the recommended first-wave datasets for the Finnish survival pack.

### 1. Retkikartta

- **Why include:** Highest practical value. Gives huts, wilderness cabins, shelters, trails, campfire sites, and water points.
- **Why now:** Even though it may need some validation around bulk export, it is the single most useful Finnish operational layer.
- **Preserve as:** Original geodata export if available.
- **Present as:** Browser-visible point and line layers in the map UI.
- **Role in pack:** Core layer.

### 2. Pohjavesialueet

- **Why include:** Direct water relevance. Answers a first-order survival question.
- **Why now:** Small, understandable, and map-native.
- **Preserve as:** `GeoPackage` or original official static GIS format.
- **Present as:** Polygon layer with simple legend and short explanation about what groundwater area classification means.
- **Role in pack:** Core layer.

### 3. Kasviatlas

- **Why include:** Best current candidate for a practical wild-plant layer. It provides distribution information for Finnish vascular plants and includes bilberry presence indirectly via plant occurrence.
- **Why now:** It is more directly tied to edible and useful plants than broad vegetation-zone layers.
- **Important limitation:** This is a distribution atlas, not a yield map. It answers "does this plant occur in this area?" better than "is this a good harvest spot this week?"
- **Preserve as:** Snapshot of official atlas data export or transformed canonical vector form.
- **Present as:** Searchable species layers and/or preselected practical plant overlays.
- **Role in pack:** Core food/foraging layer.

### 4. Virtavesien lohikalakannat

- **Why include:** Clear fishing relevance. More operationally useful than generic biodiversity data for a first survival pack.
- **Why now:** It is directly tied to protein acquisition and should render cleanly as a thematic map layer.
- **Preserve as:** Official GIS download in canonical vector form.
- **Present as:** River/stream layer with salmonid presence styling.
- **Role in pack:** Core food/fishing layer.

### 5. Luonnonsuojelu- ja erämaa-alueet

- **Why include:** Good habitat and wilderness context. Useful for intact ecosystems, game/fish/forage expectations, and understanding where the landscape is least disturbed.
- **Why now:** Easy to explain, small enough to package, and visually clear as polygons.
- **Preserve as:** Official GIS download in canonical vector form.
- **Present as:** Protected-area and wilderness overlay with attribution.
- **Role in pack:** Core context layer.

### 6. Fimea pharmaceutical registry

- **Why include:** High practical value in crisis conditions. Helps identify active ingredients, substitutions, and medicine relationships.
- **Why now:** Not map-based, but strong survival/reference value and likely lightweight to store.
- **Preserve as:** Original `XML` / `TXT` or normalized local reference snapshot.
- **Present as:** Searchable sidecar reference, not a map layer.
- **Role in pack:** Reference sidecar.

### 7. Joukahainen

- **Why include:** Very small Finnish language reference dataset with low storage cost.
- **Why now:** It is nearly free to carry and can support future Finnish search, labeling, or language-reference tooling.
- **Preserve as:** Original `XML`.
- **Present as:** Searchable sidecar reference, not a map layer.
- **Role in pack:** Reference sidecar.

---

## Included-Now Summary Table

| Dataset | Usefulness | Packaging difficulty | Map / reference |
|--------|------------|----------------------|-----------------|
| `Retkikartta` | Very high | Medium | Map |
| `Pohjavesialueet` | Very high | Easy | Map |
| `Kasviatlas` | High | Medium | Map |
| `Virtavesien lohikalakannat` | High | Easy-Medium | Map |
| `Luonnonsuojelu- ja erämaa-alueet` | High | Easy | Map |
| `Fimea` | High | Easy | Reference |
| `Joukahainen` | Medium | Easy | Reference |

---

## Deferred Sources

These are worth keeping in the overall catalog, but should be deferred from the first pack.

### Paikannimirekisteri

- **Reason deferred:** Useful mainly as search and labels, not as a strong first-wave operational layer.
- **Why still interesting:** Finnish place names may encode hints about springs, swamps, rapids, ponds, and terrain.
- **Bring back later as:** Search index plus label layer, not as the headline dataset.

### Lintuatlas

- **Reason deferred:** Useful but secondary compared with shelter, water, fish, and plant layers.
- **Why still interesting:** Bird distribution has hunting/protein value and compact reference value.
- **Bring back later as:** Optional thematic layer and compact field-reference sidecar.

### Metsäkasvillisuusvyöhykkeet

- **Reason deferred:** Too coarse to be a primary operational layer in the first browser map.
- **Why still interesting:** Gives ecological and climate context for what vegetation types dominate different regions.
- **Bring back later as:** Background context overlay.

### CORINE maanpeite

- **Reason deferred:** Useful, but broader terrain context is slightly less urgent than the selected first-wave layers.
- **Why still interesting:** Strong land-cover overview for forests, bogs, water, urban areas, and open land.
- **Bring back later as:** Terrain/land-use expansion layer.

### Tulvavaaravyöhykkeet

- **Reason deferred:** Important for camp/shelter safety, but water, shelter, and food layers take priority.
- **Why still interesting:** Directly supports camp placement and settlement decisions.
- **Bring back later as:** Water-and-shelter expansion layer.

### Maatalousmaa

- **Reason deferred:** More useful for longer-term agriculture and settlement than first-line survival browsing.
- **Why still interesting:** Shows where arable land is and what crops it supports.
- **Bring back later as:** Food-production / settlement pack.

### Luke berry and mushroom yield data

- **Reason deferred:** Relevant, but current packaging path is unclear. The official material appears closer to seasonal monitoring and forecast services than to a clean, timeless nationwide base layer.
- **Why still interesting:** Direct foraging relevance, especially for berries.
- **Bring back later as:** Seasonal snapshot pack or derived foraging overlay once a clean export path is verified.

### Lajitietokeskus bulk data

- **Reason deferred:** Too broad and noisy for the first curated pack.
- **Why still interesting:** Probably the richest long-term source for edible, medicinal, poisonous, and ecologically relevant species data.
- **Bring back later as:** Curated species subsets rather than raw bulk ingest.

### Maastotietokanta, DEM, Digiroad, Luke forest inventory, orthophotos

- **Reason deferred:** Large, complex, or heavy enough to justify a separate implementation pass.
- **Why still interesting:** These are strong candidates for larger-capacity packs and more advanced terrain analysis.
- **Bring back later as:** High-capacity or advanced packs.

---

## Later Pack Expansion Ideas

The Finnish regional catalog should expand in coherent waves rather than by appending random layers.

### Wave 2: Water and Shelter Expansion

- `Paikannimirekisteri`
- `Tulvavaaravyöhykkeet`
- `CORINE maanpeite`
- optional settlement and shoreline context

Purpose:

- improve camp placement
- improve terrain interpretation
- improve water-related decision-making

### Wave 3: Foraging and Species Expansion

- curated `Lajitietokeskus` subsets
- berry-specific seasonal snapshots from Luke if a stable export path exists
- `Lintuatlas`
- selected edible / poisonous plant overlays

Purpose:

- move from broad habitat indicators to species-level field guidance
- support foraging and hazard avoidance with more precision

### Wave 4: Terrain and Mobility Expansion

- `Maastotietokanta`
- `Korkeusmalli 10 m`
- `Digiroad`
- `Ranta10`
- selected orthophoto subsets

Purpose:

- route planning
- terrain analysis
- settlement and movement decisions

### Wave 5: Long-Term Settlement and Production

- `Maatalousmaa`
- soil and geology layers from GTK
- broader climate normals and gridded climate products
- forest inventory layers

Purpose:

- agriculture
- construction-material sourcing
- long-term habitation planning

---

## Practical Notes for Implementation

When these sources are added to Svalbard, the pack should distinguish between:

- `map layers` for the offline browser
- `reference sidecars` for search and lookup
- `raw archives` preserved for future tooling

Recommended first implementation principle:

- make the first pack usable in a browser before optimizing for perfect data completeness

This means:

- a smaller number of clear, well-labeled layers is better than a large bundle of raw datasets
- each included layer should answer an obvious user question
- each layer should come with a short explanation of what it is and why it matters

---

## Provisional First-Pack Definition

**Map layers**

- `Retkikartta`
- `Pohjavesialueet`
- `Kasviatlas`
- `Virtavesien lohikalakannat`
- `Luonnonsuojelu- ja erämaa-alueet`

**Reference sidecars**

- `Fimea`
- `Joukahainen`

**Explicitly deferred**

- `Paikannimirekisteri`
- `Lintuatlas`
- `Metsäkasvillisuusvyöhykkeet`
- `CORINE maanpeite`
- `Tulvavaaravyöhykkeet`
- `Maatalousmaa`
- Luke berry/mushroom services pending clearer packaging
- raw `Lajitietokeskus` bulk ingest
- high-volume terrain datasets

---

## Open Questions

- What is the cleanest canonical snapshot workflow for `Kasviatlas`?
- Should practical plant layers be exposed as:
  - a generic atlas search interface, or
  - a curated list of preselected useful species first?
- For future Luke berry data, can we identify a stable export/API path suitable for repeatable offline snapshots?

---

## Resolved Questions (2026-03-30)

### Retkikartta Data Access

Retkikartta aggregates two data sources with fundamentally different licensing:

**LIPAS (University of Jyvaskyla) — CC BY 4.0, freely redistributable:**
- WFS at `https://lipas.fi/geoserver/lipas/ows`
- Covers ~66% of lean-to shelters, ~27% of campfire sites, significant trail coverage
- Bulk download works — all features in a single WFS request with CQL_FILTER
- Key type codes: 301 (lean-to), 302 (hut), 206 (campfire), 4405 (hiking trail), 4404 (nature trail), 4451 (canoe route)

**Metsahallitus own data — restricted, redistribution prohibited:**
- ~34% of lean-tos, ~73% of campfire places, ~81% of wilderness huts
- Available via undocumented REST API at `retkikartta-api.anderscloud.com`
- Terms of use prohibit distribution/publishing without written permission
- Finnish Copyright Act Section 12 excludes databases from private copying exception

**Decision:** LIPAS ships openly in presets. Metsahallitus structures are a user-fetched local recipe (`recipes/local/metsa-structures.yaml`, gitignored). Svalbard provides the recipe and tooling; the user runs the ingest themselves. Worth contacting Metsahallitus about data use agreement, but first pack does not block on it.

### Architecture: Unified Pipeline, Not Parallel Systems

The existing download system will be widened rather than replaced. Source gets a `strategy` field (`download` or `build`). Build sources declare their pipeline config. `sync_drive()` dispatches based on strategy. One system, one manifest, one preset format.

### OSM Basemap

Use `pmtiles extract` against the remote Protomaps daily build via HTTP range requests. No need to download the planet. Finland-only at z15 is ~700 MB-1 GB.

Basemap coverage scales with preset tier:
- 32-64 GB: Finland at z15 (~0.7-1 GB)
- 128 GB: Finland + Estonia + border Karelia at z15 (~1.5-2.5 GB)
- 256 GB: Nordics + Estonia + border Karelia/StPete at z15 (~4-8 GB)
- 512 GB+: Nordics + Baltics + border regions at z15 (~5-10 GB)

Basemap assets (MapLibre GL JS viewer, fonts/glyphs, sprites, style JSON) are ~15-20 MB total and ship in all tiers.

### Map Viewer

MapLibre GL JS + PMTiles protocol. ~250 KB JS payload. Supports toggleable layers, WebGL rendering, smooth interaction. Requires HTTP Range Request support from the local server.

### Data Exploration Tooling

All tiers get the core tooling stack (~50-65 MB total):
- SQLiteViz (~5-15 MB) — SQL queries + charting for SQLite/CSV
- DuckDB WASM Shell + spatial extension (~20-30 MB) — analytical SQL, reads GeoPackage/GeoJSON/CSV
- MapLibre viewer + basemap assets (~15-20 MB)

128 GB+ tiers additionally get:
- JupyterLite or Marimo WASM (~200-300 MB) — full Python notebooks via Pyodide

### Geodata Toolchain

External tools (shell out from Python):
- `ogr2ogr` (GDAL) — format conversion, reprojection (EPSG:3067 to EPSG:4326)
- `tippecanoe` — PMTiles generation from GeoJSON/FlatGeobuf

Python libraries:
- `fiona` / `geopandas` — when filtering or joining before export
- `lxml` — XML parsing for Fimea, Joukahainen
- `sqlite3` (stdlib) — building reference databases with FTS5

### Recipe System

Dataset configurations live in `recipes/` as YAML files:
- `recipes/*.yaml` — committed, open-data recipes (LIPAS, SYKE datasets, etc.)
- `recipes/local/*.yaml` — gitignored, personal/restricted recipes (Metsahallitus)

Svalbard auto-discovers from both directories. Local recipes produce artifacts that can be included in personal drive builds but are never distributed.

### Fimea and Joukahainen

Ship in the same preset as the map pack. They are tiny (sub-100 MB as SQLite) and directly useful. Not optional sidecars — core reference content.
