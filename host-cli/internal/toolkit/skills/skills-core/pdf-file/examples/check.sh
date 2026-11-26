#!/bin/sh
# Offline check for pdf-file. With no argument: make a 3-page PDF from
# report.txt, merge it with itself (6 pages), extract text, take pages out,
# and fill a form. With a file: validate that PDF (it opens, has pages and
# extractable text).
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
TOOL="$SKILL/scripts/pdf_tool.py"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
cp "$SKILL/examples/report.txt" report.md
py-docs -c "from PIL import Image, ImageDraw; im = Image.new('RGB', (400, 200), 'white'); ImageDraw.Draw(im).rectangle((40, 60, 140, 180), fill='steelblue'); im.save('wells.png')"
printf '\n![Litres per day](wells.png)\n' >> report.md
py-docs "$TOOL" create report.md report.pdf
py-docs "$SKILL/examples/validate.py" report.pdf --pages 3 --text "1,240 litres"
py-docs -c "import sys; from pypdf import PdfReader; n = sum(len(p.images) for p in PdfReader('report.pdf').pages); sys.exit(0 if n == 1 else 'want 1 image in report.pdf, found %d' % n)"
py-docs "$TOOL" merge both.pdf report.pdf report.pdf
py-docs "$SKILL/examples/validate.py" both.pdf --pages 6
py-docs "$TOOL" text both.pdf --pages 5 > page5.txt
grep -q "east pump filter" page5.txt || { echo "page 5 text lacks the task list"; exit 1; }
py-docs "$TOOL" pages report.pdf tasks.pdf 2-3
py-docs "$SKILL/examples/validate.py" tasks.pdf --pages 2 --text "Order spare seals"
py-docs "$TOOL" split report.pdf split
[ "$(ls split | wc -l | tr -d ' ')" = 3 ] || { echo "split did not write 3 files"; exit 1; }
py-docs "$SKILL/examples/make_form.py" form.pdf
py-docs "$TOOL" fields form.pdf > fields.txt
grep -q "^name" fields.txt || { echo "form fields not listed"; exit 1; }
printf '{"name": "Kari Nordmann", "litres": "1240"}\n' > values.json
py-docs "$TOOL" fill form.pdf values.json filled.pdf
py-docs "$TOOL" fields filled.pdf > filled.txt
grep -q "Kari Nordmann" filled.txt || { echo "filled form lacks the name"; exit 1; }
echo "pdf-file: ok"
