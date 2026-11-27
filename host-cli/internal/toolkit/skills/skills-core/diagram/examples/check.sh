#!/bin/sh
# Offline check for diagram.
#   sh check.sh PAGE.html   validate a packed diagram page: nothing left to
#                           pack, it packs again without a problem, and with
#                           SVALBARD_HTML_CHECK set every diagram renders
#                           offline in Chromium and Firefox, none as a
#                           syntax error
#   sh check.sh             self-test: pack the template (flowchart,
#                           sequence, gantt) in a temporary folder
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
EXPECT='(() => { const d = [...document.querySelectorAll("pre.mermaid")]; return d.length > 0 && d.every(p => p.querySelector("svg")) && !document.body.innerText.includes("Syntax error"); })()'
TMP=$(mktemp -d "${TMPDIR:-/tmp}/diagram-check.XXXXXX")
trap 'case $TMP in */diagram-check.??????) rm -rf "$TMP" ;; esac' EXIT

validate() {
  if grep -Eq '<script[^>]*[[:space:]]src=|^[[:space:]]*import[[:space:]].*from[[:space:]]*["'"'"'][^./"'"'"']' "$1"; then
    echo "$1 is not packed: run svalbard-drive html pack on the page and check the file it writes"
    exit 1
  fi
  if ! svalbard-drive html pack "$1" -o "$TMP/repacked.html" >"$TMP/pack.log" 2>&1; then
    cat "$TMP/pack.log"
    echo "$1 does not work offline (problems above)"
    exit 1
  fi
  if [ -n "${SVALBARD_HTML_CHECK:-}" ]; then
    if "$SVALBARD_HTML_CHECK" --wait 20 --expect "$EXPECT" "$1"; then :; else
      status=$?
      [ $status -eq 77 ] || { echo "$1 fails in a browser (above): a diagram did not render"; exit 1; }
      echo "browser check skipped: a browser is missing"
    fi
  else
    echo "no browser check: SVALBARD_HTML_CHECK is not set"
  fi
}

if [ $# -gt 0 ]; then
  validate "$1"
  echo "diagram: ok"
  exit 0
fi
cp "$SKILL/templates/diagram.html" "$TMP/example-diagram.html"
svalbard-drive html pack "$TMP/example-diagram.html" -o "$TMP/example-packed.html" >/dev/null
[ "$(grep -c '<pre class="mermaid">' "$TMP/example-packed.html")" -eq 3 ] || { echo "the template should hold three diagrams"; exit 1; }
validate "$TMP/example-packed.html"
echo "diagram: ok"
