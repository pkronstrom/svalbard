# PMTiles

Reads `.pmtiles` map archives (the drive's `maps/`) for MapLibre, by HTTP range requests. A page using it opens through the drive's server: a file opened by double-click cannot read ranges of another file.

## Import

```js
import { Protocol, PMTiles } from "pmtiles";
```

## Example: register the protocol, read a header

```js
import maplibregl from "maplibre-gl";
import { Protocol, PMTiles } from "pmtiles";
const protocol = new Protocol();
maplibregl.addProtocol("pmtiles", protocol.tile);
const url = new URL("../maps/osm-uusimaa.pmtiles", location).href;
const header = await new PMTiles(url).getHeader();   // bounds, zooms, tile type
```

Pack with `svalbard-drive html pack page.html --dir` and open the folder through the drive's server. The offline-map skill has the full page.

## Reference

`pmtiles.reference.md` (`skill_read`, file `pmtiles.reference.md`): the README and the type definitions.
