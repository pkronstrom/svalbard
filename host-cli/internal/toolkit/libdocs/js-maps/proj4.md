# proj4

Converts coordinates between map projections, for example Finnish ETRS-TM35FIN (EPSG:3067) to longitude and latitude. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import proj4 from "proj4";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>proj4</title></head>
<body>
<pre id="out"></pre>
<script type="module">
import proj4 from "proj4";
proj4.defs("EPSG:3067", "+proj=utm +zone=35 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs +type=crs");
const [lon, lat] = proj4("EPSG:3067", "EPSG:4326", [385700, 6672100]);
document.getElementById("out").textContent = `${lat.toFixed(5)} N, ${lon.toFixed(5)} E`;
</script>
</body></html>
```

## Notes

- EPSG:4326 (longitude, latitude) and EPSG:3857 (web Mercator) are built in; define others with `proj4.defs(code, proj4String)`.
- Argument order is always `[x, y]`, so longitude first.

## Reference

`proj4.reference.md` has the package's README.
