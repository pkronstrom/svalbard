# Matter.js

2D rigid-body physics: falling boxes, collisions, springs, a mouse that drags bodies. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import Matter from "matter-js";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Physics</title></head>
<body>
<script type="module">
import Matter from "matter-js";
const { Engine, Render, Runner, Bodies, Composite, Mouse, MouseConstraint } = Matter;
const engine = Engine.create();
const render = Render.create({ element: document.body, engine, options: { width: 600, height: 400, wireframes: false } });
const ground = Bodies.rectangle(300, 390, 600, 20, { isStatic: true });
const boxes = Array.from({ length: 12 }, (_, i) => Bodies.rectangle(100 + i * 35, 50 - i * 20, 30, 30));
Composite.add(engine.world, [ground, ...boxes]);
Composite.add(engine.world, MouseConstraint.create(engine, { mouse: Mouse.create(render.canvas) }));
Render.run(render);
Runner.run(Runner.create(), engine);
</script>
</body></html>
```

## Notes

- Bodies: `rectangle`, `circle`, `polygon`, `fromVertices`. Options: `restitution` (bounce), `friction`, `isStatic`.
- Read positions each frame with `Matter.Events.on(engine, "afterUpdate", ...)` to draw with your own canvas or p5.

## Reference

`matter-js.reference.md` has the package's README only: the Matter.js API pages are generated and not on this drive.
