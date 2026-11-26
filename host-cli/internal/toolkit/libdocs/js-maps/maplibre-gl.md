# MapLibre GL JS

Interactive vector maps: pan, zoom, styled layers, markers and popups, over the drive's PMTiles maps. Pack the page with `svalbard-drive html pack --dir`: a map reads its tiles from the drive, so it opens through the drive's server, not by double-click.

## Import

```js
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { Protocol } from "pmtiles";
```

## Example: a GeoJSON layer on a blank map (no tiles)

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Map</title>
<style>body { margin: 0 } #map { position: absolute; inset: 0 }</style></head>
<body>
<div id="map"></div>
<script type="module">
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import places from "./places.geojson";
const map = new maplibregl.Map({
  container: "map",
  style: { version: 8, sources: {}, layers: [{ id: "bg", type: "background", paint: { "background-color": "#dfe9ef" } }] },
  center: [24.94, 60.17], zoom: 9
});
map.on("load", () => {
  map.addSource("places", { type: "geojson", data: places });
  map.addLayer({ id: "places", type: "circle", source: "places", paint: { "circle-radius": 6, "circle-color": "#b7472a" } });
});
map.on("click", "places", e => new maplibregl.Popup().setLngLat(e.lngLat).setHTML(e.features[0].properties.name).addTo(map));
</script>
</body></html>
```

## Over a PMTiles map on the drive

```js
const protocol = new Protocol();
maplibregl.addProtocol("pmtiles", protocol.tile);
// style source: { type: "vector", url: "pmtiles://" + new URL("../maps/osm-uusimaa.pmtiles", location).href }
```

The offline-map skill has a tested template with the drive's map styles; use it for anything over PMTiles.

## Reference

`maplibre-gl.reference.md` (`skill_read`, file `maplibre-gl.reference.md`): the README, the guides and the type definitions (every Map option and method).
