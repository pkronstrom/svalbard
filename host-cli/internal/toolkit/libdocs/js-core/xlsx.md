# SheetJS (xlsx)

Reads and writes Excel workbooks (.xlsx, .xls), CSV and ODS in the page: export a table, or read a file the user picks. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import * as XLSX from "xlsx";
```

## Example: export rows, read a picked file

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Sheets</title></head>
<body>
<button id="save">Download stock.xlsx</button>
<input type="file" id="file" accept=".xlsx,.xls,.csv,.ods">
<pre id="out"></pre>
<script type="module">
import * as XLSX from "xlsx";
const rows = [{ item: "Water", count: 12 }, { item: "Rice", count: 4 }];
document.getElementById("save").onclick = () => {
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, XLSX.utils.json_to_sheet(rows), "Stock");
  XLSX.writeFile(wb, "stock.xlsx");
};
document.getElementById("file").onchange = async e => {
  const wb = XLSX.read(await e.target.files[0].arrayBuffer());
  const first = wb.Sheets[wb.SheetNames[0]];
  document.getElementById("out").textContent = JSON.stringify(XLSX.utils.sheet_to_json(first).slice(0, 5), null, 2);
};
</script>
</body></html>
```

## Notes

- `XLSX.utils.table_to_book(document.querySelector("table"))` turns an HTML table into a workbook.
- `XLSX.utils.sheet_to_csv(sheet)` gives CSV text.

## Reference

`xlsx.reference.md` (`skill_read`, file `xlsx.reference.md`): the README and the type definitions (every `XLSX.utils` function).
