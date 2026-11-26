# Pico CSS

Class-light CSS: plain HTML elements (headings, forms, buttons, tables, cards) look finished without classes. Import it from a module script and pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import "@picocss/pico";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Form</title></head>
<body>
<main class="container">
  <h1>Supplies</h1>
  <article>
    <header>Add an item</header>
    <form><input name="item" placeholder="Item"><button type="submit">Add</button></form>
  </article>
  <table><thead><tr><th>Item</th><th>Count</th></tr></thead><tbody><tr><td>Water</td><td>12</td></tr></tbody></table>
</main>
<script type="module">import "@picocss/pico";</script>
</body></html>
```

## Notes

- `<main class="container">` centres the page; `<article>` is a card; `<details>` is an accordion; `role="group"` joins buttons.
- Dark and light follow the system; force one with `<html data-theme="light">`.
- Colours: override the `--pico-primary` CSS variables in a `<style>` after the import.

## Reference

`@picocss__pico.reference.md` (`skill_read`, file `@picocss__pico.reference.md`) has the package's README.
