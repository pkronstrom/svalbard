## Context

Vector WFS/archive handlers establish typed tools-base execution. Remaining map families need explicit upstream data contracts and potentially large bounded downloads.

## Goals / Non-Goals

**Goals:** reproducible sources, resource estimates, resumable staging, typed PMTiles output, attribution preservation.

**Non-Goals:** arbitrary GIS command recipes or unbounded world-tile crawling.

## Decisions

- OSM recipes declare a versioned PMTiles/PBF source rather than silently choosing “latest”.
- Raster-TMS requires bounds and min/max zoom and computes tile count/download estimate before apply.
- MML-topo explicitly declares source archives, SRS, bounds, and output layers.
- Go performs acquisition/scheduling; GDAL/tippecanoe/pmtiles execute only through tools-base.

## Risks / Trade-offs

Source services and licenses vary. No implementation begins until representative source URLs and expected layer fixtures are approved.
