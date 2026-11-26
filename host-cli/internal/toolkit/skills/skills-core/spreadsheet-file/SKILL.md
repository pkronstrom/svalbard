---
name: spreadsheet-file
description: Make an Excel spreadsheet (.xlsx) from numbers, a CSV or JSON file, with a bold header, formulas and a total row, or read an .xlsx back as CSV. Runs offline with the drive's Python (py-docs).
requires: [py-docs]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned, for example
`/Volumes/SVALBARD/skills/skills-core/spreadsheet-file`. Write your files in
the user's working folder, never inside `<folder>`.

1. Write the data as a CSV file in the working folder, header row first:

   ```text
   Item,Amount
   Rent,800
   Food,350
   ```

   A cell that starts with `=` is a formula, for example `=B2*C2`.
2. Make the workbook:

   ```sh
   py-docs <folder>/scripts/xlsx_tool.py create budget.csv budget.xlsx --total Amount
   ```

   `--total Amount` adds a "Total" row with `=SUM(...)` under that column;
   `--total` alone totals every numeric column; leave it out for no total.
3. Read it back and compare with what the user asked for:

   ```sh
   py-docs <folder>/scripts/xlsx_tool.py read budget.xlsx
   ```

4. Check your result. It must print `ok` (drop `--need-sum` when there is no
   total row):

   ```sh
   sh <folder>/examples/check.sh budget.xlsx --need-sum
   ```

5. Tell the user the file's full path. Formula results appear when the file
   is opened in a spreadsheet program (LibreOffice, Excel, Numbers).

## Other inputs and commands

- JSON instead of CSV: a list of objects (`[{"Item": "Rent", "Amount": 800}]`),
  a list of rows, or `{"Sheet 1": rows, "Sheet 2": rows}` for several sheets.
- `--sheet NAME` names the sheet.
- `py-docs <folder>/scripts/xlsx_tool.py info file.xlsx` lists sheets, sizes
  and headers; `read file.xlsx --sheet NAME` reads another sheet.
- `read --values` prints the values a spreadsheet program saved instead of
  formulas; for a file made here, formulas have no saved values yet.
- Charts, number formats, colours: write a short script with openpyxl and
  run it with `py-docs script.py`. `<folder>/references/openpyxl.md` has
  tested snippets.
