#!/bin/sh
# Offline check for word-file.
#   sh check.sh FILE.docx [--heading TEXT] [--cell TEXT]   validate a
#                       document: it opens, has a heading and some text
#   sh check.sh         self-test in a temporary folder: make a document
#                       from letter.txt, read it back, fill a template
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
TOOL="$SKILL/scripts/docx_tool.py"
if [ $# -gt 0 ]; then
  exec py-docs "$SKILL/examples/validate.py" "$@"
fi
# The self-test works in its own temporary folder, never the caller's,
# where the agent's files are.
REF=$(mktemp -d "${TMPDIR:-/tmp}/word-check.XXXXXX")
trap 'case $REF in */word-check.??????) rm -rf "$REF" ;; esac' EXIT
cd "$REF"
py-docs "$TOOL" create "$SKILL/examples/letter.txt" example-plan.docx
py-docs "$SKILL/examples/validate.py" example-plan.docx --heading "Garden plan 2027" --cell "Fence wire"
py-docs "$TOOL" read example-plan.docx > example-plan.txt
grep -q "^# Garden plan 2027" example-plan.txt || { echo "read lacks the title heading"; exit 1; }
grep -q "^- Order seed potatoes by March" example-plan.txt || { echo "read lacks the bullet"; exit 1; }
grep -q "^| Seed potatoes | 40 |" example-plan.txt || { echo "read lacks the table row"; exit 1; }
grep -q "^2. Beans along the south fence" example-plan.txt || { echo "read lacks the numbered item"; exit 1; }
printf '# Notice\n\nDear {{name}}, the meeting is on {{date}}.\n' > example-template.md
py-docs "$TOOL" create example-template.md example-template.docx
printf '{"name": "Aino", "date": "5 March"}\n' > example-values.json
py-docs "$TOOL" fill example-template.docx example-values.json example-notice.docx
py-docs "$TOOL" read example-notice.docx | grep -q "^Dear Aino, the meeting is on 5 March." || { echo "fill did not replace the placeholders"; exit 1; }
echo "word-file: ok"
