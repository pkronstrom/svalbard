# Chart.js

Bar, line, pie, doughnut, scatter and radar charts on a canvas. Write the page with a bare import and pack it with `svalbard-drive html pack page.html -o out.html`; never link a CDN.

## Import

```js
import Chart from "chart.js/auto";
```

`chart.js/auto` registers every chart type. There is no `window.Chart` global and no CDN script tag.

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Sales</title></head>
<body>
<canvas id="chart" width="640" height="320"></canvas>
<script type="module">
import Chart from "chart.js/auto";
new Chart(document.getElementById("chart"), {
  type: "bar",
  data: {
    labels: ["Jan", "Feb", "Mar"],
    datasets: [{ label: "Sales", data: [120, 90, 150] }]
  },
  options: { responsive: false, plugins: { title: { display: true, text: "Sales by month" } } }
});
</script>
</body></html>
```

## Notes

- Change `type` to `line`, `pie`, `doughnut`, `scatter` (data as `{x, y}` points) or `radar`.
- Several series: more objects in `datasets`. Stacked bars: `options.scales.x.stacked` and `y.stacked` set to `true`.
- To redraw with new data, change `chart.data` and call `chart.update()`.

## Reference

The Chart.js docs at this version are in `chart.js.reference.md` (read it with `skill_read`, file `chart.js.reference.md`): chart types, axes, legend, tooltip and animation options.
