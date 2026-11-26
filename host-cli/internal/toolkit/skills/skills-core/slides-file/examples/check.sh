#!/bin/sh
# Offline check for slides-file. With no argument: make deck.pptx from
# outline.json (title slide, bullet slides, one image) and from the same deck
# in markdown, and validate both. With a file: validate that presentation (it
# opens, has slides, the first has a title).
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
TOOL="$SKILL/scripts/pptx_tool.py"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
cp "$SKILL/examples/outline.json" outline.json
py-docs -c "from PIL import Image, ImageDraw; im = Image.new('RGB', (800, 600), 'white'); ImageDraw.Draw(im).rectangle((300, 100, 500, 550), outline='black', width=6); im.save('tank.png')"
py-docs "$TOOL" create outline.json deck.pptx
py-docs "$SKILL/examples/validate.py" deck.pptx --slides 4 --title "Winter readiness" --pictures 1
py-docs "$TOOL" read deck.pptx > deck.txt
grep -q "^## 3. Water" deck.txt || { echo "read lacks the Water slide"; exit 1; }
grep -q "^  - Test the damper" deck.txt || { echo "read lacks the second-level bullet"; exit 1; }
grep -q "^Notes: Mention the tank level chart." deck.txt || { echo "read lacks the notes"; exit 1; }
cat > deck.md <<'MD'
# Winter readiness

Village meeting, December

## Heating

- Wood for 14 weeks is stacked
  - Sweep the chimney

## Water

- Keep 20 litres per person

![](tank.png)

Notes: Mention the tank level chart.
MD
py-docs "$TOOL" create deck.md deck-md.pptx
py-docs "$SKILL/examples/validate.py" deck-md.pptx --slides 3 --title "Winter readiness" --pictures 1
echo "slides-file: ok"
