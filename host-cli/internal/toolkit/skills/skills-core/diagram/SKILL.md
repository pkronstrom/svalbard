---
name: diagram
description: Draw flowcharts, sequence diagrams, gantt charts, mind maps, class, state and entity diagrams from text with Mermaid, in one HTML page that opens offline and prints well.
requires: [js-core]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces). Work in the user's
working folder, never inside `<folder>`.

1. Copy the template:

   ```sh
   cp -n "<folder>"/templates/diagram.html diagram.html
   ```

   `-n` keeps a file of that name already there: then pick another name.

2. Edit the region `EDIT: diagrams`: the page title, then one `<section>`
   per diagram with a heading and a `<pre class="mermaid">` holding the
   diagram text. The first line names the type:
   - `flowchart TD` (or `LR`): `A[Box] --> B{Question}`, `B -- yes --> C`
   - `sequenceDiagram`: `participant A`, `A->>B: message`, `B-->>A: reply`
   - `gantt`: `dateFormat YYYY-MM-DD`, `section Name`, `Task :a1, 2026-06-01, 3d`
   - `mindmap`, `classDiagram`, `stateDiagram-v2`, `erDiagram`, `pie`,
     `timeline`
   Write `&lt;` and `&amp;` for `<` and `&` inside the text. Delete the
   sections you do not need.
3. Pack it into one file:

   ```sh
   svalbard-drive html pack diagram.html -o diagram-packed.html
   ```

4. Check your result. It must print `ok`; a diagram with a syntax error
   fails it:

   ```sh
   sh "<folder>"/examples/check.sh diagram-packed.html
   ```

5. Tell the user the packed file's full path. It opens by double-click;
   the browser's print command makes a PDF.

## Syntax help

The Mermaid quickstart and its reference file have every diagram type with
examples: `skill_search` "mermaid" with `kind` library, then `skill_read`
its file `mermaid.reference.md` with a section such as "Flowcharts Syntax".
Mermaid loads with `<script src="mermaid"></script>`: the package's name,
which `svalbard-drive html pack` replaces with the library. Never a URL.
