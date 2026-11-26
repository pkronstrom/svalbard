#!/bin/sh
# Offline check for slides-file.
#   sh check.sh FILE.pptx [--slides N] [--title TEXT] [--pictures N]
#                       validate a presentation: it opens, has slides, the
#                       first has a title, nothing has no size or runs off
#   sh check.sh         self-test in a temporary folder: make a deck from
#                       outline.json and from markdown, read it back
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
TOOL="$SKILL/scripts/pptx_tool.py"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
# The self-test works in its own temporary folder, never the caller's,
# where the agent's files are.
REF=$(mktemp -d "${TMPDIR:-/tmp}/slides-check.XXXXXX")
trap 'case $REF in */slides-check.??????) rm -rf "$REF" ;; esac' EXIT
cd "$REF"
cp "$SKILL/examples/outline.json" example-outline.json
py-docs -c "from PIL import Image, ImageDraw; im = Image.new('RGB', (800, 600), 'white'); ImageDraw.Draw(im).rectangle((300, 100, 500, 550), outline='black', width=6); im.save('tank.png')"
py-docs "$TOOL" create example-outline.json example-deck.pptx
py-docs "$SKILL/examples/validate.py" example-deck.pptx --slides 4 --title "Winter readiness" --pictures 1
py-docs "$TOOL" read example-deck.pptx > example-deck.txt
grep -q "^## 3. Water" example-deck.txt || { echo "read lacks the Water slide"; exit 1; }
grep -q "^  - Test the damper" example-deck.txt || { echo "read lacks the second-level bullet"; exit 1; }
grep -q "^Notes: Mention the tank level chart." example-deck.txt || { echo "read lacks the notes"; exit 1; }
cat > example-deck.md <<'MD'
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
py-docs "$TOOL" create example-deck.md example-deck-md.pptx
py-docs "$SKILL/examples/validate.py" example-deck-md.pptx --slides 3 --title "Winter readiness" --pictures 1
echo "slides-file: ok"
