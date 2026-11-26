---
name: slides-file
description: Make a PowerPoint presentation (.pptx) from an outline, with a title slide, bullet slides, pictures and speaker notes, or read a .pptx as text. Runs offline with the drive's Python (py-docs).
requires: [py-docs]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces), for example
`/Volumes/SVALBARD/skills/skills-core/slides-file`. Write your files in the
user's working folder, never inside `<folder>`.

1. Write the outline as markdown in the working folder, one `##` per slide:

   ```text
   # Winter readiness

   Village meeting, December

   ## Heating

   - Wood for 14 weeks is stacked
     - Sweep the chimney first

   ## Water

   - Keep 20 litres per person

   ![](tank.png)

   Notes: Mention the tank level chart.
   ```

   `#` and the line under it make the title slide. Indent a bullet two
   spaces for a sub-point. A picture goes on the right of the bullets, or
   fills a slide without bullets; pictures are found relative to the outline.
   Keep a slide to about six bullets.
2. Build the presentation (16:9; add `--4x3` for old projectors):

   ```sh
   py-docs "<folder>"/scripts/pptx_tool.py create deck.md deck.pptx
   ```

3. Read it back and compare with what the user asked for:

   ```sh
   py-docs "<folder>"/scripts/pptx_tool.py read deck.pptx
   ```

4. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh deck.pptx
   ```

   Add `--slides N` to require N slides (the title slide counts).
5. Tell the user the file's full path.

## Other inputs

- A JSON outline works too:
  `{"title": "...", "subtitle": "...", "slides": [{"title": "...", "bullets": ["point", ["sub-point"]], "image": "pic.png", "notes": "..."}]}`.
- A chart for a slide: draw it as a PNG first (for example with Pillow in a
  `py-docs` script, or export it from a spreadsheet program) and add it with
  `![](chart.png)`.
- `read` also prints the text of other people's presentations, slide by
  slide, with their notes.
