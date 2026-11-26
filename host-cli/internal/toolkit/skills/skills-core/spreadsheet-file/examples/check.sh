#!/bin/sh
# Offline check for spreadsheet-file.
#   sh check.sh FILE.xlsx [--need-sum]   validate a workbook: it opens and
#                                        has a header and data; --need-sum
#                                        also wants a SUM formula (a total)
#   sh check.sh                          self-test: build the example
#                                        workbook in a temporary folder
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
# The self-test works in its own temporary folder, never the caller's,
# where the agent's files are.
REF=$(mktemp -d "${TMPDIR:-/tmp}/spreadsheet-check.XXXXXX")
trap 'case $REF in */spreadsheet-check.??????) rm -rf "$REF" ;; esac' EXIT
cd "$REF"
TOOL="$SKILL/scripts/xlsx_tool.py"
py-docs "$TOOL" create "$SKILL/examples/sales.csv" example-sales.xlsx --total Units
py-docs "$SKILL/examples/validate.py" example-sales.xlsx --expect-b2 120 --need-sum
py-docs "$TOOL" read example-sales.xlsx > example-read.csv
grep -q "^Total,=SUM(B2:B4),$" example-read.csv || { echo "read did not print the total row with one formula"; exit 1; }
# Text that looks numeric to Python stays text.
printf 'Name,Code,Phone\nNan,007,+358401234567\n' > example-people.csv
py-docs "$TOOL" create example-people.csv example-people.xlsx
py-docs -c "
import sys, openpyxl
row = [c.value for c in openpyxl.load_workbook('example-people.xlsx').active[2]]
sys.exit(0 if row == ['Nan', '007', '+358401234567'] else 'text cells changed: %r' % row)"
echo "spreadsheet-file: ok"
