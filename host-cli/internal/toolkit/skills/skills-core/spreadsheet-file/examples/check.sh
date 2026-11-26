#!/bin/sh
# Offline check for spreadsheet-file. With no argument: build the example
# workbook from sales.csv with a total row and validate it. With a file:
# validate that workbook (it opens and has a header and data); add
# --need-sum to also want a SUM formula (a total row).
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
py-docs "$SKILL/scripts/xlsx_tool.py" create "$SKILL/examples/sales.csv" sales.xlsx --total Units
py-docs "$SKILL/examples/validate.py" sales.xlsx --expect-b2 120 --need-sum
py-docs "$SKILL/scripts/xlsx_tool.py" read sales.xlsx > sales-read.csv
grep -q "^Total,=SUM(B2:B4),$" sales-read.csv || { echo "read did not print the total row with one formula"; exit 1; }
echo "spreadsheet-file: ok"
