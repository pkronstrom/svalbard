# TopoJSON client

Turns TopoJSON (compact shared-border geometry) into GeoJSON for D3 or MapLibre, and draws shared borders once. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import * as topojson from "topojson-client";
```

## Example

```js
import * as topojson from "topojson-client";
import topology from "./world.topojson.json";   // packed into the page
const countries = topojson.feature(topology, topology.objects.countries);   // a GeoJSON FeatureCollection
const borders = topojson.mesh(topology, topology.objects.countries, (a, b) => a !== b);
```

## Notes

- `topology.objects` names the layers in the file.
- GeoJSON from the drive's GeoPackages (the geodata skill) needs no TopoJSON.

## Reference

`topojson-client.reference.md` has the package's README with the whole API.
