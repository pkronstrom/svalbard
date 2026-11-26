#!/bin/sh
# Offline check for parametric-cad.
#   sh check.sh PART.stl      validate an STL: header, triangles, watertight,
#                             positive volume
#   sh check.sh PAGE.html     validate a packed part page: nothing left to
#                             pack, it packs again without a problem, and
#                             with SVALBARD_HTML_CHECK set it loads offline in
#                             Chromium and Firefox with a model shown, and in
#                             Chromium its #export button downloads an STL
#                             that passes the checks above
#   sh check.sh               self-test, in a temporary folder: the template
#                             packed, four gears checked watertight in the
#                             page, and its STL exported and validated
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
PY=py-docs
command -v py-docs >/dev/null 2>&1 || PY=python3
TMP=$(mktemp -d "${TMPDIR:-/tmp}/cad-check.XXXXXX")
trap 'case $TMP in */cad-check.??????) rm -rf "$TMP" ;; esac' EXIT
SHOWN='window.svalbardModel && window.svalbardModel.triangles > 0'
# Every edge of the preview mesh joins exactly two triangles, for several
# gears: tiny, coarse, many teeth without a bore, and the default.
SWEEP='(() => {
  const set = (k, v) => { const el = document.getElementById(k); el.value = v; el.dispatchEvent(new Event("change")); };
  const sealed = () => { const p = window.svalbardModel.positions, edges = new Map(), key = i => [p[i], p[i + 1], p[i + 2]].map(v => v.toFixed(3)).join(",");
    for (let t = 0; t < p.length; t += 9) { const v = [key(t), key(t + 3), key(t + 6)];
      for (let e = 0; e < 3; e++) { const k = [v[e], v[(e + 1) % 3]].sort().join("|"); edges.set(k, (edges.get(k) || 0) + 1); } }
    return [...edges.values()].every(n => n === 2); };
  const cases = [[1, 12, 3, 2], [4, 9, 10, 8], [1, 120, 4, 0], [2, 20, 6, 5]];
  for (const [m, z, t, b] of cases) { set("module", m); set("teeth", z); set("thickness", t); set("bore", b); if (!sealed()) return false; }
  return true; })()'

validate_page() {
  if grep -Eq '<script[^>]*[[:space:]]src=|^[[:space:]]*import[[:space:]].*from[[:space:]]*["'"'"'][^./"'"'"']' "$1"; then
    echo "$1 is not packed: run svalbard-drive html pack on the page and check the file it writes"
    exit 1
  fi
  if ! svalbard-drive html pack "$1" -o "$TMP/repacked.html" >"$TMP/pack.log" 2>&1; then
    cat "$TMP/pack.log"
    echo "$1 does not work offline (problems above)"
    exit 1
  fi
  if [ -z "${SVALBARD_HTML_CHECK:-}" ]; then
    echo "no browser check: SVALBARD_HTML_CHECK is not set"
    return
  fi
  "$SVALBARD_HTML_CHECK" --browser firefox --expect "$SHOWN" "$1" || { echo "$1 fails in Firefox (above)"; exit 1; }
  "$SVALBARD_HTML_CHECK" --browser chromium --expect "$2" --click "#export" --download "$TMP/export.stl" "$1" || { echo "$1 fails in Chromium (above)"; exit 1; }
  $PY "$SKILL/examples/validate_stl.py" "$TMP/export.stl" --min-triangles 12
}

case "${1:-}" in
  *.stl|*.STL) exec $PY "$SKILL/examples/validate_stl.py" "$1" ;;
  ?*) validate_page "$1" "$SHOWN"; echo "parametric-cad: ok"; exit 0 ;;
esac
cp "$SKILL/templates/gear.html" "$TMP/example-gear.html"
svalbard-drive html pack "$TMP/example-gear.html" -o "$TMP/example-packed.html" >/dev/null
validate_page "$TMP/example-packed.html" "$SWEEP"
echo "parametric-cad: ok"
