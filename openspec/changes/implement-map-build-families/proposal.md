## Why

OSM extract, raster-TMS, and MML-topo recipes require source/version and tiling decisions beyond the vector handlers. They should be implemented after the tools-base map seam is reviewed rather than guessed inside the first heavy-family slice.

## What Changes

- Define stable source inputs for OSM/Protomaps extracts.
- Implement bounded raster-TMS acquisition.
- Implement MML topo archive processing.
- Produce verified PMTiles through Go orchestration and typed tools-base calls.

## Capabilities

### New Capabilities
- `map-build-families`: OSM, raster, and MML map recipes build reproducibly into PMTiles.

### Modified Capabilities

None.

## Impact

Depends on builder foundation and vector geodata handlers. No host tool installation.
