# pdf-lib

Creates and edits PDF files in the page: a printable report, a filled form, merged documents. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import { PDFDocument, StandardFonts, rgb } from "pdf-lib";
```

## Example: make a PDF and download it

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>PDF</title></head>
<body>
<button id="make">Download report.pdf</button>
<script type="module">
import { PDFDocument, StandardFonts, rgb } from "pdf-lib";
document.getElementById("make").onclick = async () => {
  const pdf = await PDFDocument.create();
  const page = pdf.addPage([595, 842]);           // A4 in points
  const font = await pdf.embedFont(StandardFonts.Helvetica);
  page.drawText("Supply report", { x: 50, y: 780, size: 24, font, color: rgb(0.1, 0.2, 0.5) });
  page.drawText("Water: 12 l", { x: 50, y: 740, size: 12, font });
  const bytes = await pdf.save();
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob([bytes], { type: "application/pdf" }));
  a.download = "report.pdf";
  a.click();
};
</script>
</body></html>
```

## Notes

- Standard fonts cover Latin-1 only; for other characters embed a TTF with `pdf.embedFont(bytes)` after `pdf.registerFontkit`, which this drive does not ship.
- Edit a picked PDF: `PDFDocument.load(await file.arrayBuffer())`; forms: `pdf.getForm().getTextField("name").setText("x")`.

## Reference

`pdf-lib.reference.md` (`skill_read`, file `pdf-lib.reference.md`): the README with many examples and the API's type definitions.
