# Papa Parse

Parses CSV text or a file the user picks into rows, and turns rows back into CSV. Write the page with a bare import and pack it with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import Papa from "papaparse";
```

## Example: CSV in the page, and another file the user picks

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>CSV</title></head>
<body>
<input type="file" id="file" accept=".csv,text/csv">
<pre id="out"></pre>
<script type="module">
import Papa from "papaparse";
import text from "./data.csv";            // packed into the page as text
const show = rows => document.getElementById("out").textContent = JSON.stringify(rows.slice(0, 5), null, 2);
show(Papa.parse(text, { header: true, dynamicTyping: true, skipEmptyLines: true }).data);
document.getElementById("file").addEventListener("change", e => {
  Papa.parse(e.target.files[0], { header: true, dynamicTyping: true, skipEmptyLines: true, complete: r => show(r.data) });
});
</script>
</body></html>
```

## Notes

- `header: true` gives objects keyed by the first row; `dynamicTyping: true` turns numbers into numbers.
- `Papa.unparse(rows)` makes CSV text again (for a download, put it in a `Blob`).
- Never `Papa.parse("data.csv", {download: true})`: that fetches, and a page opened by double-click cannot.

## Reference

`papaparse.reference.md` (`skill_read`, file `papaparse.reference.md`) has the package's README.
