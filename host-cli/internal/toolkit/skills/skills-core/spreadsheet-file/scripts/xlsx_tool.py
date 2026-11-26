"""Make and read .xlsx workbooks with openpyxl. Run with py-docs:

  py-docs xlsx_tool.py create DATA.csv|DATA.json OUT.xlsx [--sheet NAME] [--total [COLUMNS]]
  py-docs xlsx_tool.py read IN.xlsx [--sheet NAME] [--values]
  py-docs xlsx_tool.py info IN.xlsx

create: the first CSV row (or the JSON objects' keys) becomes a bold header.
Numbers become numbers; a cell that starts with "=" is kept as a formula.
JSON may be a list of objects, a list of lists, or {"Sheet name": rows, ...}
for several sheets. --total adds a "Total" row with a SUM formula under every
numeric column; --total Units,Cost only under the columns named.

read: prints a sheet as CSV. Formulas print as formulas; --values prints
the values a spreadsheet program last saved instead (empty for formulas in
a file no spreadsheet program has opened yet).
"""
import argparse
import csv
import json
import sys
from pathlib import Path

from openpyxl import Workbook, load_workbook
from openpyxl.styles import Font, PatternFill
from openpyxl.utils import get_column_letter


def number(text):
    """Return text as int or float when it is one, else unchanged."""
    if not isinstance(text, str):
        return text
    s = text.strip()
    if s.startswith("="):
        return s
    try:
        return int(s)
    except ValueError:
        pass
    try:
        return float(s)
    except ValueError:
        return text


def load_rows(path):
    """Return {sheet name: rows} from a CSV or JSON file."""
    p = Path(path)
    if p.suffix.lower() == ".json":
        data = json.loads(p.read_text(encoding="utf-8"))
        sheets = data if isinstance(data, dict) else {p.stem: data}
        out = {}
        for name, rows in sheets.items():
            if rows and isinstance(rows[0], dict):
                header = list(rows[0].keys())
                for row in rows[1:]:
                    header += [k for k in row if k not in header]
                rows = [header] + [[row.get(k, "") for k in header] for row in rows]
            out[str(name)] = rows
        return out
    with open(p, newline="", encoding="utf-8-sig") as f:
        sample = f.read(4096)
        f.seek(0)
        try:
            dialect = csv.Sniffer().sniff(sample, delimiters=",;\t")
        except csv.Error:
            dialect = csv.excel
        return {p.stem: [row for row in csv.reader(f, dialect) if row]}


def write_sheet(ws, rows, total):
    header_fill = PatternFill("solid", fgColor="DDE4EE")
    for r, row in enumerate(rows, start=1):
        for c, value in enumerate(row, start=1):
            cell = ws.cell(row=r, column=c, value=value if r == 1 else number(value))
            if r == 1:
                cell.font = Font(bold=True)
                cell.fill = header_fill
    ws.freeze_panes = "A2"
    last = len(rows)
    if total and last >= 2:
        t = last + 1
        ws.cell(row=t, column=1, value="Total").font = Font(bold=True)
        for c in range(2, len(rows[0]) + 1):
            if total != "*" and str(rows[0][c - 1]).strip() not in [t.strip() for t in total.split(",")]:
                continue
            values = [number(row[c - 1]) for row in rows[1:] if c - 1 < len(row)]
            if values and all(isinstance(v, (int, float)) for v in values):
                col = get_column_letter(c)
                ws.cell(row=t, column=c, value=f"=SUM({col}2:{col}{last})").font = Font(bold=True)
    for c in range(1, ws.max_column + 1):
        width = max(len(str(ws.cell(row=r, column=c).value or "")) for r in range(1, ws.max_row + 1))
        ws.column_dimensions[get_column_letter(c)].width = min(max(width + 2, 8), 60)


def create(args):
    sheets = load_rows(args.data)
    wb = Workbook()
    wb.remove(wb.active)
    for name, rows in sheets.items():
        title = (args.sheet if args.sheet and len(sheets) == 1 else name)[:31]
        write_sheet(wb.create_sheet(title), rows, args.total)
    wb.save(args.out)
    print(f"wrote {args.out}: " + ", ".join(f"{ws.title} ({ws.max_row} rows)" for ws in wb.worksheets))


def pick_sheet(wb, name):
    if not name:
        return wb.worksheets[0]
    if name not in wb.sheetnames:
        sys.exit(f"no sheet {name!r}; sheets: {', '.join(wb.sheetnames)}")
    return wb[name]


def read(args):
    wb = load_workbook(args.xlsx, data_only=args.values)
    ws = pick_sheet(wb, args.sheet)
    out = csv.writer(sys.stdout, lineterminator="\n")
    for row in ws.iter_rows(values_only=True):
        out.writerow(["" if v is None else v for v in row])


def info(args):
    wb = load_workbook(args.xlsx)
    for ws in wb.worksheets:
        header = [c.value for c in ws[1]] if ws.max_row else []
        print(f"{ws.title}: {ws.max_row} rows x {ws.max_column} columns ({ws.dimensions}); header: {header}")


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)
    c = sub.add_parser("create")
    c.add_argument("data")
    c.add_argument("out")
    c.add_argument("--sheet")
    c.add_argument("--total", nargs="?", const="*", metavar="COLUMNS")
    c.set_defaults(fn=create)
    r = sub.add_parser("read")
    r.add_argument("xlsx")
    r.add_argument("--sheet")
    r.add_argument("--values", action="store_true")
    r.set_defaults(fn=read)
    i = sub.add_parser("info")
    i.add_argument("xlsx")
    i.set_defaults(fn=info)
    args = p.parse_args()
    args.fn(args)


if __name__ == "__main__":
    main()
