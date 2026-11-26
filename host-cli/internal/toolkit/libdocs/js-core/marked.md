# Marked

Markdown to HTML in the page. Clean the result with DOMPurify before showing text the page did not write. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import { marked } from "marked";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Notes</title></head>
<body>
<textarea id="src" rows="6" cols="60"># Notes

- one
- **two**</textarea>
<div id="out"></div>
<script type="module">
import { marked } from "marked";
import DOMPurify from "dompurify";
const render = () => document.getElementById("out").innerHTML = DOMPurify.sanitize(marked.parse(document.getElementById("src").value));
document.getElementById("src").addEventListener("input", render);
render();
</script>
</body></html>
```

## Notes

- `marked.parse(text, { gfm: true, breaks: true })`: GitHub tables and line breaks.
- A Markdown file next to the page: `import text from "./notes.txt"` (packed as text) and `marked.parse(text)`.

## Reference

The Marked docs at this version are in `marked.reference.md` (`skill_read`, file `marked.reference.md`).
