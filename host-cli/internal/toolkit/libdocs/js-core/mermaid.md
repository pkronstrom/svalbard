# Mermaid

Flowcharts, sequence diagrams, gantt charts, class and state diagrams, pie charts and mind maps drawn from text. Mermaid is a classic script that sets a global: load it with a `<script src="mermaid">` naming the package (not a URL), and pack the page with `svalbard-drive html pack`; never link a CDN.

## Load

```html
<script src="mermaid"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>
```

## Example

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>Process</title></head>
<body>
<pre class="mermaid">
flowchart TD
  A[Collect water] --> B{Clear?}
  B -- yes --> C[Boil 1 minute]
  B -- no --> D[Filter through cloth] --> C
  C --> E[Store covered]
</pre>
<script src="mermaid"></script>
<script>mermaid.initialize({ startOnLoad: true, theme: "default" });</script>
</body></html>
```

## Notes

- Each diagram is a `<pre class="mermaid">` holding the diagram text. Escape `<` and `&` in it as `&lt;` and `&amp;`.
- First line names the type: `flowchart LR`, `sequenceDiagram`, `gantt`, `classDiagram`, `stateDiagram-v2`, `pie`, `mindmap`, `erDiagram`, `timeline`.
- Render later or from code: `const { svg } = await mermaid.render("id1", text)`.
- Themes: `default`, `neutral`, `dark`, `forest`, `base`.

## Reference

The Mermaid syntax docs at this version are in `mermaid.reference.md` (`skill_read`, file `mermaid.reference.md`): one section per diagram type, with examples.
