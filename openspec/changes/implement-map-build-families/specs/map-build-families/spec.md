## ADDED Requirements

### Requirement: Reproducible map builds
OSM, raster-TMS, and MML recipes SHALL declare bounded/versioned sources and produce verified PMTiles.

#### Scenario: Plan map build
- **WHEN** a map build is planned
- **THEN** source version, bounds, zoom range, estimated transfer/work size, and tools requirement are visible

#### Scenario: Build map
- **WHEN** an approved map recipe executes
- **THEN** Go orchestration and typed tools-base calls produce one verified attributed PMTiles artifact
