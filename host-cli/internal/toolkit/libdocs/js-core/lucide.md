# Lucide icons

About 2,000 line icons as SVG. Put `<i data-lucide="name">` in the page and replace them with one call. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import * as lucide from "lucide";
lucide.createIcons({ icons: lucide.icons });
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Icons</title></head>
<body>
<button><i data-lucide="download"></i> Save</button>
<i data-lucide="map-pin" style="color: crimson; width: 32px; height: 32px"></i>
<script type="module">
import * as lucide from "lucide";
lucide.createIcons({ icons: lucide.icons });
</script>
</body></html>
```

## Notes

- Names are kebab-case: `arrow-right`, `chart-bar`, `triangle-alert`. The reference file lists every name in this version.
- Size and colour come from CSS (`width`, `height`, `color`); stroke width from `stroke-width` on the element.
- Icons added later: call `lucide.createIcons({ icons: lucide.icons })` again.

## Reference

`lucide.reference.md` (`skill_read`, file `lucide.reference.md`): the vanilla-JS guide and the full list of icon names.
