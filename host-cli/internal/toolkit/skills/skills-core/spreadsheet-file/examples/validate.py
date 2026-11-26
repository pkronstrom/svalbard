"""Validate a workbook: it opens and its first sheet has a header row and at
least one data row. --need-sum also wants a SUM formula in that sheet,
--expect-b2 a value in cell B2."""
import argparse
import sys

import openpyxl


def main():
    p = argparse.ArgumentParser()
    p.add_argument("xlsx")
    p.add_argument("--expect-b2")
    p.add_argument("--need-sum", action="store_true")
    a = p.parse_args()
    try:
        wb = openpyxl.load_workbook(a.xlsx)
    except Exception as e:
        sys.exit(f"{a.xlsx} does not open as a workbook: {e}")
    ws = wb.worksheets[0]
    if ws.max_row < 2:
        sys.exit(f"{a.xlsx}: sheet {ws.title!r} has {ws.max_row} row(s), want a header and data")
    if not any(c.value not in (None, "") for c in ws[1]):
        sys.exit(f"{a.xlsx}: the first row is empty, want a header")
    sums = [c.coordinate for row in ws.iter_rows() for c in row
            if isinstance(c.value, str) and c.value.upper().startswith("=SUM(")]
    if a.need_sum and not sums:
        sys.exit(f"{a.xlsx}: no SUM formula in sheet {ws.title!r}")
    if a.expect_b2 is not None and str(ws["B2"].value) != a.expect_b2:
        sys.exit(f"{a.xlsx}: B2 is {ws['B2'].value!r}, want {a.expect_b2}")
    print(f"{a.xlsx}: ok ({ws.max_row} rows, SUM in {', '.join(sums) or 'no cell'})")


if __name__ == "__main__":
    main()
