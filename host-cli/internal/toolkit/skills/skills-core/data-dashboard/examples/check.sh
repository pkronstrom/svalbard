#!/bin/sh
# Offline check for data-dashboard.
#   sh check.sh PAGE.html   validate a packed dashboard: nothing is left to
#                           pack, it packs again without a problem (no
#                           network, no fetch, no missing file), and with
#                           SVALBARD_HTML_CHECK set it loads in Chromium and
#                           Firefox offline with a chart drawn and table rows
#   sh check.sh             self-test: pack the template with example data
#                           in a temporary folder
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
EXPECT='document.querySelectorAll("#table tbody tr").length > 0 && (() => { const c = document.getElementById("chart1"); const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data; for (let i = 3; i < d.length; i += 4) if (d[i]) return true; return false; })()'
TMP=$(mktemp -d "${TMPDIR:-/tmp}/dashboard-check.XXXXXX")
trap 'case $TMP in */dashboard-check.??????) rm -rf "$TMP" ;; esac' EXIT

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
    "$SVALBARD_HTML_CHECK" --expect "$EXPECT" "$1" || { echo "$1 fails in a browser (above)"; exit 1; }
  else
    echo "no browser check: SVALBARD_HTML_CHECK is not set"
  fi
}

if [ $# -gt 0 ]; then
  validate "$1"
  echo "data-dashboard: ok"
  exit 0
fi
cp "$SKILL/templates/dashboard.html" "$TMP/example-dashboard.html"
cp "$SKILL/examples/example-data.csv" "$TMP/data.csv"
svalbard-drive html pack "$TMP/example-dashboard.html" -o "$TMP/example-packed.html" >/dev/null
validate "$TMP/example-packed.html"
echo "data-dashboard: ok"
