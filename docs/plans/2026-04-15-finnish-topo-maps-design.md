# Finnish Topo Maps from kartat.kapsi.fi — Design

## Summary

Two new map recipes that bring MML (National Land Survey of Finland) topographic
maps to Svalbard, sourced from the community-maintained kartat.kapsi.fi mirror:

1. **mml-maastokartta** — Raster topo tiles scraped from kapsi.fi TMS
2. **mml-topo-uusimaa** — Vector topo from maastotietokanta GeoPackage

Both produce PMTiles and integrate with the existing MapLibre viewer.

## Recipes

### mml-maastokartta (raster)

- Build family: `raster-tms` (new)
- Source: `https://tiles.kartat.kapsi.fi/maastokartta/{z}/{x}/{y}.jpg`
- Default bbox: Uusimaa (23.8,59.8,26.8,61.7)
- Default zoom: 0–13 (~1–2 GB)
- Pipeline: download tiles → MBTiles (SQLite) → `pmtiles convert` → raster PMTiles
- Beautiful pre-rendered MML cartography, zero styling work

### mml-topo-uusimaa (vector)

- Build family: `mml-topo` (new)
- Source: kapsi.fi all-Finland GeoPackage (~22 GB maasto + ~11 GB korkeus, cached)
- All 133 maastotietokanta feature classes included
- Tables grouped into 8 MML taustakartta source-layers:
  vesisto_alue, vesisto_viiva, maasto_alue, maankaytto, liikenne, rakennus, korkeus, maastoaluereuna
- Features carry `kohdeluokka` attribute for style filtering
- Pipeline: download → unzip → ogr2ogr clip per table → merge per source-layer → tippecanoe multi-layer → vector PMTiles
- Estimated output: 200–500 MB for Uusimaa

## Viewer Changes

- Multiple basemap support (radio buttons when >1 basemap)
- Raster tile type: `tile_type: raster` in recipe viewer config
- MML vector topo: `tile_type: mml-topo` with simplified ~15-layer style using MML backgroundmap.json colours and kohdeluokka filtering
- Backward compatible: existing Protomaps vector basemaps work unchanged

## Style Source

The vector topo style is a simplified adaptation of MML's official
[backgroundmap.json](https://github.com/nlsfi/avoin-karttakuva.maanmittauslaitos.fi/blob/master/vectortiles/stylejson/v20/backgroundmap.json)
(~100 layers → ~15 key layers). Key colours:
- Background: #dceacc (MML pale green)
- Water: hsl(200,80%,85%)
- Contours: rgba(252,179,110,1) (orange-brown)
- Major roads: #cc4444 (red)
- Buildings: hsl(0,0%,65%) (gray)
- Fields: #f9f4d2 (pale yellow)
- Forest: hsl(110,40%,65%)

## Future Work

- **Cache prune command** — `~/.cache/svalbard/build/` has no list/clean tooling
- **Full MML style** — Integrate complete backgroundmap.json (~100 layers) as loadable style
- **Glyphs/sprites** — Bundle MML glyphs and sprite sheets for fully offline place names and map symbols
- **Finland-wide recipes** — Raster at z0–15 (~20–50 GB), vector all-Finland (~5–10 GB) for larger tiers
- **Peruskartta variant** — Higher-detail 1:20k raster tiles from `tiles.kartat.kapsi.fi/peruskartta`
- **MML API integration** — Per-municipality download via OGC API Processes (avoids 22 GB full download)

## License

All MML data: CC-BY-4.0, attribution: "National Land Survey of Finland (Maanmittauslaitos)"
