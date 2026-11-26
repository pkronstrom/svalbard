#!/bin/sh
# Offline check for pdf-file.
#   sh check.sh FILE.pdf [--pages N] [--text WORDS]   validate a PDF: it
#                       opens, has pages and extractable text
#   sh check.sh         self-test in a temporary folder: make a 3-page PDF
#                       with a picture, merge it with itself, extract text,
#                       take pages out, split it, and fill a form
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
TOOL="$SKILL/scripts/pdf_tool.py"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
# The self-test works in its own temporary folder, never the caller's,
# where the agent's files are.
REF=$(mktemp -d "${TMPDIR:-/tmp}/pdf-check.XXXXXX")
trap 'case $REF in */pdf-check.??????) rm -rf "$REF" ;; esac' EXIT
cd "$REF"
cp "$SKILL/examples/report.txt" example-report.md
py-docs -c "from PIL import Image, ImageDraw; im = Image.new('RGB', (400, 200), 'white'); ImageDraw.Draw(im).rectangle((40, 60, 140, 180), fill='steelblue'); im.save('example-wells.png')"
printf '\n![Litres per day](example-wells.png)\n\nMis-nested **bold *and** italic* still renders.\n' >> example-report.md
py-docs "$TOOL" create example-report.md example-report.pdf
py-docs "$SKILL/examples/validate.py" example-report.pdf --pages 3 --text "1,240 litres"
py-docs -c "import sys; from pypdf import PdfReader; n = sum(len(p.images) for p in PdfReader('example-report.pdf').pages); sys.exit(0 if n == 1 else 'want 1 image in example-report.pdf, found %d' % n)"
py-docs "$TOOL" merge example-both.pdf example-report.pdf example-report.pdf
py-docs "$SKILL/examples/validate.py" example-both.pdf --pages 6
py-docs "$TOOL" text example-both.pdf --pages 5 > example-page5.txt
grep -q "east pump filter" example-page5.txt || { echo "page 5 text lacks the task list"; exit 1; }
py-docs "$TOOL" pages example-report.pdf example-tasks.pdf 2-3
py-docs "$SKILL/examples/validate.py" example-tasks.pdf --pages 2 --text "Order spare seals"
py-docs "$TOOL" split example-report.pdf example-split
[ "$(ls example-split | wc -l | tr -d ' ')" = 3 ] || { echo "split did not write 3 files"; exit 1; }
py-docs "$SKILL/examples/make_form.py" example-form.pdf
py-docs "$TOOL" fields example-form.pdf > example-fields.txt
grep -q "^name" example-fields.txt || { echo "form fields not listed"; exit 1; }
printf '{"name": "Kari Nordmann", "litres": "1240"}\n' > example-values.json
py-docs "$TOOL" fill example-form.pdf example-values.json example-filled.pdf
py-docs "$TOOL" fields example-filled.pdf > example-filled.txt
grep -q "Kari Nordmann" example-filled.txt || { echo "filled form lacks the name"; exit 1; }
echo "pdf-file: ok"
