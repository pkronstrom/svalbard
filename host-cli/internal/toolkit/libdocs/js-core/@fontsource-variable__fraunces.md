# Fraunces (font)

A soft display serif for headings and titles, variable weight 100 to 900, Latin characters. Name the package in a CSS `url()` and `svalbard-drive html pack` puts the font into the page; never link Google Fonts or a CDN.

## Use

```html
<style>
@font-face {
  font-family: "Fraunces";
  font-weight: 100 900;
  font-display: swap;
  src: url("@fontsource-variable/fraunces") format("woff2");
}
h1, h2, h3 { font-family: "Fraunces", Georgia, serif; font-weight: 650; }
</style>
```

## Notes

- Pair it with Inter (`@fontsource-variable/inter`) for body text.
- Only Latin characters are included; others fall back to the system font.

## Reference

`@fontsource-variable__fraunces.reference.md` has the package's README.
