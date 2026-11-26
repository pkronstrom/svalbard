# Turf

Geospatial analysis on GeoJSON: area, length, distance, centroids, buffers, points in polygons, bounding boxes. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import * as turf from "@turf/turf";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Turf</title></head>
<body>
<pre id="out"></pre>
<script type="module">
import * as turf from "@turf/turf";
const helsinki = turf.point([24.94, 60.17]);
const tallinn = turf.point([24.75, 59.44]);
const square = turf.polygon([[[24, 60], [25, 60], [25, 61], [24, 61], [24, 60]]]);
document.getElementById("out").textContent = [
  `distance ${turf.distance(helsinki, tallinn, { units: "kilometers" }).toFixed(1)} km`,
  `area ${(turf.area(square) / 1e6).toFixed(0)} km2`,
  `Helsinki inside: ${turf.booleanPointInPolygon(helsinki, square)}`
].join("\n");
</script>
</body></html>
```

## Notes

- Coordinates are `[longitude, latitude]`.
- `turf.buffer(feature, 5, { units: "kilometers" })`, `turf.bbox(geojson)`, `turf.centroid(feature)`, `turf.simplify(geojson, { tolerance: 0.01 })`.

## Reference

`@turf__turf.reference.md` (`skill_read`, file `@turf__turf.reference.md`): every Turf module's README, one section per function.
