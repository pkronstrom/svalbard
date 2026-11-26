---
name: pdf-file
description: Make a PDF document (headings, text, lists, tables, images) from simple markdown, and merge, split, take pages from, read the text of, or fill the form fields of existing PDFs. Runs offline with the drive's Python (py-docs).
requires: [py-docs]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned (keep the quotes
around it in commands: a drive's path can hold spaces), for example
`/Volumes/SVALBARD/skills/skills-core/pdf-file`. Write your files in the
user's working folder, never inside `<folder>`.

1. To make a PDF, write its content as markdown in the working folder:

   ```text
   # Report title

   A paragraph. **Bold** and *italic* work.

   ## Section

   - a bullet
   - another

   | Item | Count |
   |---|---|
   | Seals | 4 |

   ![Pump diagram](pump.png)

   <<<pagebreak>>>

   ## Next page
   ```

2. Build it (add `--size letter` for US paper; A4 is the default; images are
   found relative to the markdown file):

   ```sh
   py-docs "<folder>"/scripts/pdf_tool.py create report.md report.pdf
   ```

3. For existing PDFs, run `py-docs "<folder>"/scripts/pdf_tool.py` with one of:
   - join files: `merge all.pdf a.pdf b.pdf`
   - take pages: `pages in.pdf out.pdf 2-4,7`
   - one file per page: `split in.pdf pages/`
   - read the text: `text in.pdf`, or `text in.pdf --pages 3`
   - forms: `fields form.pdf` lists the field names; write
     `{"field name": "value"}` to `values.json`, then
     `fill form.pdf values.json filled.pdf`
   - page count, size, title: `info in.pdf`
4. Check your result. It must print `ok`:

   ```sh
   sh "<folder>"/examples/check.sh report.pdf
   ```

   Add `--pages N` to require N pages, `--text "words"` to require text.
5. Tell the user the file's full path.

## Limits

- Text is set in Helvetica, which covers Western European letters; other
  scripts need a TrueType font registered in a reportlab script.
- `text` reads text that is in the PDF. A scanned page is a picture and
  gives nothing; say so rather than guessing its content.
- A password-protected PDF cannot be merged or read until it is unlocked
  with its password.
- For layouts beyond the markdown above, write a reportlab script (platypus
  `SimpleDocTemplate`) and run it with `py-docs script.py`.
