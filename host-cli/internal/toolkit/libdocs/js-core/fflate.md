# fflate

Zip and gzip in the page: build a .zip of several files for download, or read one the user picks. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import { zipSync, unzipSync, strToU8, strFromU8 } from "fflate";
```

## Example: download a zip

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Zip</title></head>
<body>
<button id="save">Download notes.zip</button>
<script type="module">
import { zipSync, strToU8 } from "fflate";
document.getElementById("save").onclick = () => {
  const zipped = zipSync({ "notes.txt": strToU8("Hello"), "data/rows.csv": strToU8("a,b\n1,2\n") });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob([zipped], { type: "application/zip" }));
  a.download = "notes.zip";
  a.click();
};
</script>
</body></html>
```

## Notes

- Read a zip the user picks: `unzipSync(new Uint8Array(await file.arrayBuffer()))` gives `{ name: Uint8Array }`; `strFromU8` turns bytes into text.
- `gzipSync` / `gunzipSync` for single streams.

## Reference

The fflate docs at this version are in `fflate.reference.md` (`skill_read`, file `fflate.reference.md`).
