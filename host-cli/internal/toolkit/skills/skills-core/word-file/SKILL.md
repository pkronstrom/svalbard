---
name: word-file
description: Make a Word document (.docx) with headings, paragraphs, lists, tables and images from simple markdown, read a .docx as text, or fill {{placeholders}} in a .docx template. Runs offline with the drive's Python (py-docs).
requires: [py-docs]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces), for example
`/Volumes/SVALBARD/skills/skills-core/word-file`. Write your files in the
user's working folder, never inside `<folder>`.

1. Write the content as markdown in the working folder:

   ```text
   # Document title

   A paragraph. **Bold** and *italic* work.

   ## Section

   1. a numbered step
   2. another

   - a bullet

   | Item | Cost |
   |---|---|
   | Seeds | 40 |

   ![Garden map](map.png)

   <<<pagebreak>>>
   ```

   `#` is the title, `##` and `###` are headings. Images are found relative
   to the markdown file and set 6 inches wide.
2. Build the document:

   ```sh
   py-docs "<folder>"/scripts/docx_tool.py create plan.md plan.docx
   ```

3. Read it back and compare with what the user asked for:

   ```sh
   py-docs "<folder>"/scripts/docx_tool.py read plan.docx
   ```

4. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh plan.docx
   ```

   Add `--heading "Document title"` to require that heading.
5. Tell the user the file's full path.

## Other tasks

- Read someone's .docx: `read file.docx` prints it as markdown, tables
  included, in document order.
- Fill a template: put `{{name}}` style placeholders in a .docx (made with
  `create`, or the user's own), write `{"name": "Aino"}` to `values.json`,
  then `py-docs "<folder>"/scripts/docx_tool.py fill template.docx values.json out.docx`.
  It names any placeholder left without a value.
- Fonts, margins, headers and footers: write a python-docx script
  (`docx.Document()`, `section.header`, `run.font.size = Pt(11)`) and run it
  with `py-docs script.py`.
- A PDF is not made here; the pdf-file skill makes PDFs from the same
  markdown.
