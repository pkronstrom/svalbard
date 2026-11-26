#!/bin/sh
# Offline check for word-file. With no argument: make plan.docx from
# letter.txt and validate its heading, lists and table. With a file: validate
# that document (it opens, has a heading and some text).
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
TOOL="$SKILL/scripts/docx_tool.py"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
py-docs "$TOOL" create "$SKILL/examples/letter.txt" plan.docx
py-docs "$SKILL/examples/validate.py" plan.docx --heading "Garden plan 2027" --cell "Fence wire"
py-docs "$TOOL" read plan.docx > plan.txt
grep -q "^# Garden plan 2027" plan.txt || { echo "read lacks the title heading"; exit 1; }
grep -q "^- Order seed potatoes by March" plan.txt || { echo "read lacks the bullet"; exit 1; }
grep -q "^| Seed potatoes | 40 |" plan.txt || { echo "read lacks the table row"; exit 1; }
grep -q "^2. Beans along the south fence" plan.txt || { echo "read lacks the numbered item"; exit 1; }
printf '# Notice\n\nDear {{name}}, the meeting is on {{date}}.\n' > template.md
py-docs "$TOOL" create template.md template.docx
printf '{"name": "Aino", "date": "5 March"}\n' > values.json
py-docs "$TOOL" fill template.docx values.json notice.docx
py-docs "$TOOL" read notice.docx | grep -q "^Dear Aino, the meeting is on 5 March." || { echo "fill did not replace the placeholders"; exit 1; }
echo "word-file: ok"
