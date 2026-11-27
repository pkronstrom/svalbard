# Inter (font)

A clear sans-serif for text and interfaces, variable weight 100 to 900, Latin characters. Name the package in a CSS `url()` and `svalbard-drive html pack` puts the font into the page; never link Google Fonts or a CDN.

## Use

```html
<style>
@font-face {
  font-family: "Inter";
  font-weight: 100 900;
  font-display: swap;
  src: url("@fontsource-variable/inter") format("woff2");
}
body { font-family: "Inter", system-ui, sans-serif; }
</style>
```

## Notes

- Any weight works: `font-weight: 450`. Pair it with Fraunces (`@fontsource-variable/fraunces`) for headings.
- Only Latin characters are included; others fall back to the system font.

## Reference

`@fontsource-variable__inter.reference.md` has the package's README.
