# Math.js

Expressions, units, matrices, fractions, big numbers and statistics: calculators and converters. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import * as math from "mathjs";
```

## Example: a unit-aware calculator

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Calculator</title></head>
<body>
<input id="expr" value="12.7 cm to inch" size="40"> <output id="out"></output>
<script type="module">
import * as math from "mathjs";
const run = () => {
  try { document.getElementById("out").textContent = math.format(math.evaluate(document.getElementById("expr").value), { precision: 6 }); }
  catch (e) { document.getElementById("out").textContent = e.message; }
};
document.getElementById("expr").addEventListener("input", run);
run();
</script>
</body></html>
```

## Notes

- `math.evaluate("sqrt(3^2 + 4^2)")`, `math.evaluate("5 km/h to m/s")`, `math.mean([1, 2, 3])`, `math.std(values)`, `math.multiply(A, B)`.
- A scope keeps variables: `const scope = {}; math.evaluate("x = 3", scope); math.evaluate("x * 2", scope)`.

## Reference

The Math.js docs at this version are in `mathjs.reference.md` (`skill_read`, file `mathjs.reference.md`): expression syntax, units, data types, and the function groups.
