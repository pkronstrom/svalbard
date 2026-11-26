# JSCAD STL serializer

Turns JSCAD solids into an STL file for slicers and 3D printers. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import { serialize } from "@jscad/stl-serializer";
```

## Example: download a cube as STL

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>STL</title></head>
<body>
<button id="stl">Download cube.stl</button>
<script type="module">
import { primitives } from "@jscad/modeling";
import { serialize } from "@jscad/stl-serializer";
document.getElementById("stl").onclick = () => {
  const solid = primitives.cuboid({ size: [20, 20, 20] });
  const data = serialize({ binary: true }, solid);     // an array of ArrayBuffers
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob(data, { type: "model/stl" }));
  a.download = "cube.stl";
  a.click();
};
</script>
</body></html>
```

## Notes

- `serialize({ binary: false }, ...solids)` writes text STL. Several solids go into one file.
- Binary STL: 80-byte header, a 4-byte triangle count, then 50 bytes per triangle.

## Reference

`@jscad__stl-serializer.reference.md` has the package's README.
