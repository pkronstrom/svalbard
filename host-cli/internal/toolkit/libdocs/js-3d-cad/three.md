# Three.js

3D scenes in WebGL: a mesh, lights, a camera the user turns with the mouse. Write the page with bare imports and pack it with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
```

Addons come from `three/addons/...`. There is no `THREE.OrbitControls` global in this version. The addons on this drive: `controls/OrbitControls.js` and `exporters/STLExporter.js`.

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Cube</title>
<style>body { margin: 0 } canvas { display: block }</style></head>
<body>
<script type="module">
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
const scene = new THREE.Scene();
scene.background = new THREE.Color(0xf0f0f0);
const camera = new THREE.PerspectiveCamera(50, innerWidth / innerHeight, 0.1, 1000);
camera.position.set(40, 30, 40);
const renderer = new THREE.WebGLRenderer({ antialias: true });
renderer.setSize(innerWidth, innerHeight);
document.body.appendChild(renderer.domElement);
scene.add(new THREE.HemisphereLight(0xffffff, 0x444444, 2));
const light = new THREE.DirectionalLight(0xffffff, 2);
light.position.set(30, 50, 20);
scene.add(light);
const mesh = new THREE.Mesh(new THREE.BoxGeometry(20, 10, 20), new THREE.MeshStandardMaterial({ color: 0x4a7c59 }));
scene.add(mesh);
const controls = new OrbitControls(camera, renderer.domElement);
renderer.setAnimationLoop(() => { controls.update(); renderer.render(scene, camera); });
addEventListener("resize", () => {
  camera.aspect = innerWidth / innerHeight; camera.updateProjectionMatrix(); renderer.setSize(innerWidth, innerHeight);
});
</script>
</body></html>
```

## Notes

- Units are whatever you choose; for 3D printing use millimetres, as JSCAD does.
- Show a JSCAD solid: build a `THREE.BufferGeometry` from its polygons (the parametric-cad skill's template does it), then `geometry.computeVertexNormals()`.
- STL from a Three.js mesh: `new STLExporter().parse(mesh, { binary: true })` from `three/addons/exporters/STLExporter.js`. For printable parts prefer JSCAD's `@jscad/stl-serializer`.

## Reference

`three.reference.md` has the package's README only: the three.js manual is HTML and not on this drive.
