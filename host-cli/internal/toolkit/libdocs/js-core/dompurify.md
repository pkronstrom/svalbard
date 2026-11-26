# DOMPurify

Cleans HTML (from a file, the user or Markdown) so it cannot run scripts when put into the page. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import DOMPurify from "dompurify";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Clean</title></head>
<body>
<div id="out"></div>
<script type="module">
import DOMPurify from "dompurify";
const dirty = '<p>Hello <img src=x onerror="alert(1)"><script>alert(2)<\/script></p>';
document.getElementById("out").innerHTML = DOMPurify.sanitize(dirty);
</script>
</body></html>
```

## Notes

- Use it whenever text the page did not write goes into `innerHTML`, for example `DOMPurify.sanitize(marked.parse(markdown))`.
- `DOMPurify.sanitize(html, { USE_PROFILES: { html: true } })` keeps HTML only (no SVG or MathML).

## Reference

`dompurify.reference.md` (`skill_read`, file `dompurify.reference.md`): the README and the type definitions with every option.
