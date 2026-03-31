# Preset Restructure Design

Restructure presets from `nordic-*` to `finland-*` + `default-*`, bake maps in,
add vector tile downloader, support multi-platform tool binaries.

## Status

**Design phase** — brainstormed 2026-03-29, not yet implemented.

---

## 1. File Structure

```
presets/
  finland-32.yaml
  finland-64.yaml
  finland-128.yaml
  finland-256.yaml
  finland-512.yaml
  finland-1tb.yaml
  finland-2tb.yaml
  default-32.yaml
  default-64.yaml
  default-128.yaml
```

Each file is **self-contained** — one file = one complete drive configuration.
No inheritance, no merging, no cross-file references. Open it, see everything
that goes on the drive. Add or remove a source by editing that one file.

The wizard discovers presets by parsing filenames: `{region}-{size}.yaml`.
Available regions = unique prefixes found. Available tiers per region = matching files.

`default-*` presets contain English-only, region-neutral content for non-Finnish
users to test the tool without downloading Finnish-specific data.

---

## 2. Preset YAML Schema

```yaml
name: finland-128
description: Finnish + English reference, Finland maps and topo
target_size_gb: 128
region: finland

sources:
  # ── Reference ──────────────────────────
  - id: wikipedia-en
    type: zim
    group: reference
    tags: [general-reference]
    depth: comprehensive
    size_gb: 25.0
    url_pattern: "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim"
    description: English Wikipedia without pictures

  # ── Maps ───────────────────────────────
  - id: osm-finland
    type: pmtiles
    group: maps
    size_gb: 1.5
    extract:
      source: "https://build.protomaps.com/{date}.pmtiles"
      bbox: "19.0,59.5,32.0,70.5"
      maxzoom: 14
    description: OpenStreetMap Finland extract

  # ── Tools ──────────────────────────────
  - id: kiwix-serve
    type: binary
    group: tools
    size_gb: 0.2
    platforms:
      linux-x86_64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_linux-x86_64.tar.gz"
      linux-arm64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_linux-aarch64.tar.gz"
      macos-x86_64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_macos-x86_64.tar.gz"
      macos-arm64: "https://download.kiwix.org/release/kiwix-tools/kiwix-tools_macos-arm64.tar.gz"
    description: Serves ZIM files as local website
```

### Schema changes from current

| Field | Change |
|-------|--------|
| `group` | **New.** Display grouping (reference, practical, education, maps, regional, models, tools). |
| `extract` | **New.** For PMTiles sources that need region extraction from a planet file. |
| `platforms` | **New.** Multi-platform binary URLs. Used instead of `url` for tools. |
| `optional_group` | **Removed.** Maps and tools are always included. |
| `replaces` | **Removed.** Each preset file is standalone. |

A source has one of: `url`, `url_pattern`, `extract`, or `platforms`. Not multiple.

### Display groups

| Group | Content |
|-------|---------|
| `reference` | Wikipedia, Wiktionary, Gutenberg |
| `practical` | WikiHow, iFixit, Stack Exchanges |
| `education` | Khan Academy, Wikibooks |
| `maps` | OSM extracts, topo maps |
| `regional` | Country-specific data (flora, hydrology, flood zones, etc.) |
| `models` | GGUF LLMs |
| `tools` | kiwix-serve, go-pmtiles, llama-server, CyberChef |

---

## 3. Maps Strategy

Maps are baked into presets — no longer an optional group. Coverage scales with tier.

### 3.1 OSM Street Maps (PMTiles)

Use `pmtiles extract` to pull regional data from the Protomaps daily planet build
**without downloading the full ~107 GB file**. The CLI fetches only tiles within
the bounding box via HTTP range requests.

```bash
pmtiles extract https://build.protomaps.com/20260328.pmtiles finland.pmtiles \
  --bbox=19.0,59.5,32.0,70.5 --maxzoom=14
```

The `extract` field in the preset YAML triggers this during `svalbard sync`.

| Tier | Coverage | Bbox | Est. size |
|------|----------|------|-----------|
| finland-64 | Finland | `19.0,59.5,32.0,70.5` | ~1.5 GB |
| finland-128 | Finland | `19.0,59.5,32.0,70.5` | ~1.5 GB |
| finland-256 | Nordics | `3.0,54.0,32.0,72.0` | ~4 GB |
| finland-512 | Europe | (full extract) | ~18 GB |
| finland-1tb+ | Planet | (full file) | ~107 GB |

### 3.2 Topographic Maps — Vector Tile Downloader

Build a vector tile downloader into svalbard (`svalbard.topo` module or similar)
that fetches tiles from national mapping agency endpoints, packs them into MBTiles,
and converts to PMTiles.

The drive includes a MapLibre GL JS viewer (single HTML file ~1 MB) with
appropriate styles for rendering the vector tiles offline.

#### Finland — MML (Maanmittauslaitos)

- **Endpoint:** `https://avoin-karttakuva.maanmittauslaitos.fi/vectortiles/wmts/1.0.0/taustakartta/default/v20/WGS84_Pseudo-Mercator/{z}/{x}/{y}.pbf`
- **Format:** Mapbox Vector Tiles 2.1 (PBF)
- **Content:** Roads, railways, buildings, admin borders, names, waterways, land use, elevations
- **License:** CC BY 4.0 — attribute "National Land Survey of Finland"
- **API key:** Required (free registration at omatili.maanmittauslaitos.fi)
- **Est. size z0-14:** ~5-10 GB
- **Alt. source:** Kapsi.fi mirror (`tiles.kartat.kapsi.fi`) — no API key needed, raster TMS tiles, daily updates. Better for raster but much larger (~37-92 GB).
- **Existing tool:** [heikkipora/maanmittauslaitos-mvt](https://github.com/heikkipora/maanmittauslaitos-mvt) — downloads MML vector tiles into MBTiles. Reference for implementation.

#### Sweden — Lantmäteriet

- **Endpoint:** Vector tiles via Geodata Portal
- **License:** CC0 (public domain — no restrictions)
- **Download:** https://www.lantmateriet.se/en/geodata/geodata-portal/
- **Products:** Topografi Visning (vector tiles), Topographic web map (raster)
- **Est. size:** ~5-10 GB vector

> **NOTE:** Verify current endpoint URLs and download method before implementing.
> Lantmäteriet APIs have changed over time.

#### Norway — Kartverket

- **Vector tiles:** `https://cache.kartverket.no/test/vectortiles/landtopo/{z}/{x}/{y}.mvt`
- **TileJSON:** `https://cache.kartverket.no/test/vectortiles/tilejson/landtopo.json`
- **Style:** `https://vectortiles.kartverket.no/styles/v1/landtopo/style.json`
- **License:** CC BY 4.0 — attribute "Kartverket"
- **Content:** 21 layers, zoom 0-19, detailed 1:3000 at high zooms
- **Constraint:** WMTS raster cache has 10,000 tiles/day rate limit. Vector tile endpoint limit unclear — needs testing.
- **Alt. bulk source:** Geonorge.no offers N50 vector downloads (GeoJSON, FGDB) for all of Norway, which could be rendered to tiles locally.

> **NOTE:** The `/test/` path suggests this may not be the final production endpoint.
> Verify before relying on it. Consider Geonorge.no bulk vector data as primary
> source if rate limits are a problem.

### 3.3 Map coverage per tier

| Tier | OSM | Topo |
|------|-----|------|
| finland-32 | (none) | (none) |
| finland-64 | Finland ~1.5 GB | Finland MML ~5 GB |
| finland-128 | Finland ~1.5 GB | Finland MML ~5 GB |
| finland-256 | Nordics ~4 GB | Finland + Sweden + Norway ~15 GB |
| finland-512 | Europe ~18 GB | Finland + Sweden + Norway ~15 GB |
| finland-1tb+ | Planet ~107 GB | Finland + Sweden + Norway ~15 GB |
| default-* | (none, or user's bbox?) | (none) |

### 3.4 Licensing

All three Nordic topo sources are openly licensed but require attribution:

- **Finland (MML):** CC BY 4.0 — "Contains data from the National Land Survey of Finland Topographic Database, [date]"
- **Sweden (Lantmäteriet):** CC0 — no attribution required
- **Norway (Kartverket):** CC BY 4.0 — "Contains data from Kartverket"

> **IMPORTANT:** Verify current license terms before each release. License terms
> can change. Check each agency's open data page. The drive's generated README
> should include attribution for all included datasets.

The `serve.sh` generated README and the manifest should include proper attribution
strings for all topo data included on the drive.

---

## 4. Multi-Platform Tool Binaries

The drive should work on any machine it's plugged into. Tool sources use the
`platforms` field instead of `url`:

```yaml
- id: kiwix-serve
  type: binary
  group: tools
  size_gb: 0.2
  platforms:
    linux-x86_64: "https://..."
    linux-arm64: "https://..."
    macos-x86_64: "https://..."
    macos-arm64: "https://..."
  description: Serves ZIM files as local website
```

The downloader fetches **all** platform URLs, placing binaries in
`bin/{platform}/`. The existing `serve.sh` already detects the platform
and looks in the right subdirectory.

Total overhead for all platforms of all tools: ~500 MB. Negligible on 64 GB+.
The 32 GB preset may want to include only linux-x86_64 + current platform
to save space.

Tools to bundle:

| Tool | Purpose | Platforms |
|------|---------|-----------|
| kiwix-serve | Serve ZIM files (Wikipedia etc.) | linux-x86_64, linux-arm64, macos-x86_64, macos-arm64 |
| go-pmtiles | Serve PMTiles map files | same |
| llama-server | Serve GGUF LLM models (512 GB+ tiers) | same |
| CyberChef | Browser-based data analysis (HTML, no binary) | all (single HTML app) |

---

## 5. Vector Tile Downloader

Build a `svalbard.vectortiles` module that:

1. Takes a tile endpoint URL template, bbox, and zoom range
2. Fetches all tiles within the bbox at each zoom level
3. Packs them into an MBTiles file (SQLite with tile table)
4. Converts MBTiles → PMTiles via `pmtiles convert` (or native Python)
5. Bundles the appropriate MapLibre GL JS style

This is triggered during `svalbard sync` for sources with the `extract` field
(for OSM PMTiles extraction) or a new `vectortiles` field (for topo map scraping).

### Preset YAML for topo sources

```yaml
- id: mml-maastokartta
  type: pmtiles
  group: maps
  size_gb: 5.0
  vectortiles:
    endpoint: "https://avoin-karttakuva.maanmittauslaitos.fi/vectortiles/wmts/1.0.0/taustakartta/default/v20/WGS84_Pseudo-Mercator/{z}/{x}/{y}.pbf"
    bbox: "19.0,59.5,32.0,70.5"
    maxzoom: 14
    format: pbf
    attribution: "National Land Survey of Finland"
  description: MML topographic map vector tiles
```

### Implementation approach

- Write our own downloader in Python (httpx + async for speed)
- Reference [heikkipora/maanmittauslaitos-mvt](https://github.com/heikkipora/maanmittauslaitos-mvt) for MBTiles schema
- Use `pmtiles` CLI for MBTiles → PMTiles conversion (already bundled)
- Include rate limiting / politeness (configurable delay between requests)
- Resume support: track downloaded tiles, skip existing ones on restart
- Progress bar via Rich (consistent with rest of svalbard)

### Tile count estimates

For Finland bbox at z0-14 (~55% fill ratio):

| Zoom | Tiles | Cumulative |
|------|-------|------------|
| 0-10 | ~1,900 | ~1,900 |
| 11 | ~2,400 | ~4,300 |
| 12 | ~9,500 | ~13,800 |
| 13 | ~38,000 | ~51,800 |
| 14 | ~152,000 | ~203,800 |

At ~20 KB avg per vector tile: ~4 GB. Reasonable.

---

## 6. Wizard Changes

The wizard becomes 4 steps:

1. **Target** — volume picker (with `svalbard/` subdir, `~/svalbard/` option)
2. **Preset** — pick region + tier based on free space (show all tiers, recommend largest fitting)
3. **Options** — toggle LLM models, installers, infra (only for 512 GB+ tiers)
4. **Review** — itemized source list grouped by `group` field

Step 2 flow:

```
Step 2/4 — Preset

  Region:
  1) finland    — Finnish + English, MML topo maps
  2) default    — English only, no region-specific data

  Select [1]:

  Presets for finland (122 GB free):

  1) finland-32          ~29 GB  — Survival essentials
  2) finland-64          ~55 GB  — Reference + Finland maps
  3) finland-128         ~97 GB  — Full reference + topo       <-- recommended
  4) finland-256        ~180 GB  — Nordic maps, richer data     (needs ~58 GB more)

  Select [3]:
```

### presets.py changes

- `list_presets()` → returns all preset names
- `list_regions()` → returns unique region prefixes (parsed from filenames)
- `presets_for_region(region)` → returns presets for that region
- `presets_for_space(free_gb, region)` → returns presets with fits flag
- `parse_preset()` → handles new fields (`group`, `extract`, `platforms`, `vectortiles`)

### models.py changes

- `Source.group` field (str, default `""`)
- `Source.extract` field (optional dict: `{source, bbox, maxzoom}`)
- `Source.platforms` field (optional dict: `{platform: url}`)
- `Source.vectortiles` field (optional dict: `{endpoint, bbox, maxzoom, format, attribution}`)
- Remove `Source.optional_group`
- Remove `Source.replaces`

---

## 7. Implementation Order

1. Rename preset files `nordic-*` → `finland-*`, add `group` field to all sources
2. Remove `optional_group` and `replaces` from schema and code
3. Update `presets.py` with region-aware listing
4. Update wizard for region selection + new step flow
5. Create `default-32`, `default-64`, `default-128` presets
6. Add `platforms` support to downloader (multi-platform binary fetching)
7. Add `extract` support to downloader (pmtiles extract integration)
8. Build `svalbard.vectortiles` module (tile scraper → MBTiles → PMTiles)
9. Add MML topo source to finland presets
10. Add Swedish + Norwegian topo sources to finland-256+ presets
11. Bundle MapLibre GL JS viewer with topo styles
12. Update `serve.sh` and drive README generation with proper attribution
13. Update tests throughout

---

## Open Questions

- **MML API key:** Required for vector tiles. Should svalbard prompt for it during
  setup, or use Kapsi.fi mirror (no key, but raster only) as fallback?
- **Norway rate limits:** If Kartverket vector tile endpoint has strict limits,
  use Geonorge.no bulk N50 vector data + local tile rendering (Planetiler) instead?
- **MapLibre GL JS styles:** MML publishes MapBox GL styles. Need to verify they
  work with MapLibre and adapt for offline use (no external font/sprite URLs).
- **32 GB preset tools:** Bundle all platforms (~500 MB) or just the most common
  (linux-x86_64 + macos-arm64, ~200 MB) to save space?
- **Offline GIS viewer:** For regional geodata (GeoPackage/Shapefile), should we
  bundle a lightweight viewer, or just include raw files + documentation?
  See `2026-03-29-finnish-open-data.md` for the full regional data catalog.
