# openpyxl snippets

Run a script with `py-docs script.py`. openpyxl 3.1 is installed; it writes
formulas but does not calculate them.

## Open, edit, save

```python
from openpyxl import load_workbook
wb = load_workbook("budget.xlsx")
ws = wb.active            # or wb["Sheet name"]
ws["C1"] = "Share"
for row in range(2, ws.max_row + 1):
    ws[f"C{row}"] = f"=B{row}/SUM(B$2:B${ws.max_row})"
wb.save("budget.xlsx")
```

## Number formats and widths

```python
for cell in ws["B"][1:]:
    cell.number_format = "#,##0.00"      # "0%" for percent, "yyyy-mm-dd" for dates
ws.column_dimensions["A"].width = 24
```

## Bar chart

```python
from openpyxl.chart import BarChart, Reference
chart = BarChart()
chart.title = "Amount by item"
data = Reference(ws, min_col=2, min_row=1, max_row=ws.max_row)       # header + values
cats = Reference(ws, min_col=1, min_row=2, max_row=ws.max_row)
chart.add_data(data, titles_from_data=True)
chart.set_categories(cats)
ws.add_chart(chart, "E2")
```

`LineChart` and `PieChart` work the same way. Leave a total row out of the
references or it becomes a bar.

## Colour a cell

```python
from openpyxl.styles import Font, PatternFill
ws["A1"].font = Font(bold=True, color="FFFFFF")
ws["A1"].fill = PatternFill("solid", fgColor="2F5597")
```
