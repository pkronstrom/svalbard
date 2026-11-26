#!/bin/sh
# Offline check for generative-art.
#   sh check.sh PAGE.html   validate a packed sketch: nothing left to pack,
#                           it packs again without a problem, and with
#                           SVALBARD_HTML_CHECK set it draws on its canvas
#                           offline in Chromium and Firefox
#   sh check.sh             self-test: pack the template in a temporary
#                           folder; it draws, and the same seed draws the
#                           same picture again
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
DRAWN='(() => { const c = document.querySelector("canvas"); if (!c) return false; const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data; const first = [d[0], d[1], d[2]]; for (let i = 0; i < d.length; i += 4) if (d[i] !== first[0] || d[i + 1] !== first[1] || d[i + 2] !== first[2]) return true; return false; })()'
REPEAT='(() => { const c = document.querySelector("canvas"); const shot = () => c.toDataURL(); const a = shot(); const seed = document.getElementById("seed"); seed.value = 7; seed.dispatchEvent(new Event("input")); const b = shot(); seed.value = 42; seed.dispatchEvent(new Event("input")); return a === shot() && a !== b; })()'
TMP=$(mktemp -d "${TMPDIR:-/tmp}/art-check.XXXXXX")
trap 'case $TMP in */art-check.??????) rm -rf "$TMP" ;; esac' EXIT

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
    "$SVALBARD_HTML_CHECK" --expect "$2" "$1" || { echo "$1 fails in a browser (above)"; exit 1; }
  else
    echo "no browser check: SVALBARD_HTML_CHECK is not set"
  fi
}

if [ $# -gt 0 ]; then
  validate "$1" "$DRAWN"
  echo "generative-art: ok"
  exit 0
fi
cp "$SKILL/templates/sketch.html" "$TMP/example-sketch.html"
svalbard-drive html pack "$TMP/example-sketch.html" -o "$TMP/example-packed.html" >/dev/null
validate "$TMP/example-packed.html" "$DRAWN && $REPEAT"
echo "generative-art: ok"
