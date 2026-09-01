# Build-family inventory

Generated from the canonical root catalog before implementation. Status describes the current Go path, not intended behavior.

| Recipe | Family | Structured data | Current path | Status | Target |
|---|---|---|---|---|---|
| `ghidra` | `app-bundle` | `source_url` | Go converts source archive to pipeline | works | retain Go handler |
| `sqliteviz` | `app-bundle` | `source_url` | Go converts source archive to pipeline | works | retain Go handler |
| `mml-map-sheets` | `app-bundle` | `assets[]` | arrays discarded; falls to missing script | broken | typed asset downloads in Go |
| `duckdb-wasm` | `app-bundle` | `assets[]` | arrays discarded; falls to missing script | broken | typed asset downloads in Go |
| `opensourcelowtech` | `zimit` | explicit `steps[]` | Go pipeline invokes pinned zimit image | works | compile steps to procedures |
| `wikipedia-en-medicine-compact` | `zim-compact` | source, width, quality | family script derived but absent | broken | Go transform + tools-base writer/checker |
| `fimea` | `reference-static` | `tables[]`, XML source | table array discarded; script absent | broken | typed Go ingestion |
| `lipas-recreation` | `vector-service` | WFS service/layers | nested data unavailable; script absent | broken | Go handler + typed tools-base calls |
| `luonnonsuojelualueet` | `vector-static` | archive/SRS/layers | script absent | broken | Go handler + typed tools-base calls |
| `pohjavesialueet` | `vector-static` | archive/SRS/layers | script absent | broken | Go handler + typed tools-base calls |
| `virtavesien-lohikalakannat` | `vector-static` | archive/SRS/layers | script absent | broken | Go handler + typed tools-base calls |
| `mml-maastokartta` | `raster-tms` | tile URL/bounds/zooms | script absent | broken | Go tile orchestration + tools-base |
| `mml-topo-uusimaa` | `mml-topo` | archives/bounds | script absent | broken | Go handler + tools-base |
| `osm-finland` | `osm-extract` | bounds/maxzoom | script absent | broken | Go handler + pmtiles/tippecanoe |
| `osm-uusimaa` | `osm-extract` | bounds/maxzoom | script absent | broken | Go handler + pmtiles/tippecanoe |
| `svalbard-python` | `python-venv` | Python spec/packages | Go handler invokes uv or Docker | partial | dependency ordering + tools-base uv |
| `makeityourself` | `custom` | ignored `builder`, PDF source | derives missing `custom.py` and checkout-relative mount | broken | Go controller + typed ZIM/browser tool calls |

## Cross-cutting defects

1. `BuildSpec.UnmarshalYAML` retains unknown scalar values only; arrays/maps such as `assets` and `tables` disappear.
2. `buildItem` derives `<family>.py` rather than consuming `builder` and mounts a directory relative to the vault, not the catalog or binary.
3. Apply schedules downloads and builds in one flat worker pool, so declared recipe dependencies do not establish execution order.
4. Resource estimates expose final `size_gb`, not source transfer, temporary work space, or tools-image cost.
5. Build progress has callback-specific shapes rather than one event contract.
6. The tools image is published, but runtime code points at `latest` rather than a release-controlled tag or digest.
