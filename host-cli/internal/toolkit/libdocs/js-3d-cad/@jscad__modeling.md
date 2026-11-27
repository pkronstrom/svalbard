# JSCAD modeling

Parametric solids for 3D printing, in millimetres: primitives, booleans, transforms, extrusions. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import { primitives, booleans, transforms, extrusions, geometries, measurements } from "@jscad/modeling";
```

## Example: a plate with a hole, measured

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Plate</title></head>
<body>
<pre id="out"></pre>
<script type="module">
import { primitives, booleans, transforms, measurements } from "@jscad/modeling";
const { cuboid, cylinder } = primitives;
const { subtract } = booleans;
const { translate } = transforms;
const plate = subtract(
  cuboid({ size: [40, 30, 4] }),
  translate([10, 0, 0], cylinder({ radius: 3.2, height: 10, segments: 48 }))
);
const [min, max] = measurements.measureBoundingBox(plate);
document.getElementById("out").textContent = `size ${max.map((v, i) => (v - min[i]).toFixed(1)).join(" x ")} mm, volume ${measurements.measureVolume(plate).toFixed(0)} mm3`;
</script>
</body></html>
```

## Notes

- Primitives: `cuboid({size})`, `cylinder({radius, height, segments})`, `sphere({radius})`, `circle`, `rectangle`, `polygon({points})`.
- 2D to 3D: `extrusions.extrudeLinear({ height }, shape2d)`; `extrusions.extrudeRotate` for turned parts.
- Booleans: `union`, `subtract`, `intersect`. Transforms: `translate`, `rotate([x, y, z] in radians)`, `scale`, `center`, `mirror`.
- A solid's triangles for display: `geometries.geom3.toPolygons(solid)` gives polygons with `vertices` (each `[x, y, z]`).
- Export: `@jscad/stl-serializer`.

## Reference

`@jscad__modeling.reference.md` (`skill_read`, file `@jscad__modeling.reference.md`): the README and the type definitions of every function.
