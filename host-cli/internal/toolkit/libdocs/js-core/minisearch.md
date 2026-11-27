# MiniSearch

Full-text search over records held in the page: a search box over notes, items or rows. Pack the page with `svalbard-drive html pack`; never link a CDN.

## Import

```js
import MiniSearch from "minisearch";
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Search</title></head>
<body>
<input id="q" placeholder="Search"><ul id="hits"></ul>
<script type="module">
import MiniSearch from "minisearch";
const docs = [
  { id: 1, title: "Water filter", text: "Boil or filter water before drinking." },
  { id: 2, title: "Fire", text: "Keep dry tinder and a ferro rod." }
];
const index = new MiniSearch({ fields: ["title", "text"], storeFields: ["title"] });
index.addAll(docs);
document.getElementById("q").addEventListener("input", e => {
  const hits = index.search(e.target.value, { prefix: true, fuzzy: 0.2 });
  document.getElementById("hits").innerHTML = hits.map(h => `<li>${h.title}</li>`).join("");
});
</script>
</body></html>
```

## Notes

- Every record needs a unique `id`. `storeFields` are returned with each hit.
- `index.autoSuggest(text)` suggests completions.

## Reference

`minisearch.reference.md` (`skill_read`, file `minisearch.reference.md`): the README and the type definitions.
