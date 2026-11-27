#!/bin/sh
# Offline check for geodata.
#   sh check.sh PAGE.html   validate a packed map page: nothing left to pack,
#                           it packs again without a problem, and with
#                           SVALBARD_HTML_CHECK set it loads in Chromium and
#                           Firefox offline with the map's shapes drawn
#   sh check.sh             self-test, in a temporary folder: the exporter on
#                           a fixture GeoPackage (byte orders, envelopes, Z,
#                           empty and unsupported geometries) and, when the
#                           drive has Natural Earth, on its countries, then
#                           the template packed with them
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
EXPECT='document.querySelectorAll("#map path").length >= 3 && (() => { const c = document.getElementById("top"); const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data; for (let i = 3; i < d.length; i += 4) if (d[i]) return true; return false; })()'
TMP=$(mktemp -d "${TMPDIR:-/tmp}/geodata-check.XXXXXX")
trap 'case $TMP in */geodata-check.??????) rm -rf "$TMP" ;; esac' EXIT

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
    if "$SVALBARD_HTML_CHECK" --expect "$EXPECT" "$1"; then :; else
      status=$?
      [ $status -eq 77 ] || { echo "$1 fails in a browser (above)"; exit 1; }
      echo "browser check skipped: a browser is missing"
    fi
  else
    echo "no browser check: SVALBARD_HTML_CHECK is not set"
  fi
}

if [ $# -gt 0 ]; then
  validate "$1"
  echo "geodata: ok"
  exit 0
fi
cd "$TMP"
TOOL="$SKILL/scripts/gpkg_to_geojson.py"
py-docs "$SKILL/examples/make_fixture.py" example.gpkg
py-docs "$TOOL" example.gpkg places example-places.geojson --columns NAME,VALUE > example-out.txt
cat example-out.txt
grep -q "5 features" example-out.txt || { echo "the fixture should give 5 features"; exit 1; }
grep -q "unsupported: 1 features skipped (WKB geometry type 7)" example-out.txt || { echo "the GeometryCollection was not reported as unsupported"; exit 1; }
grep -q "unsupported: 1 features skipped (truncated geometry)" example-out.txt || { echo "the truncated geometry was not reported as unsupported"; exit 1; }
py-docs -c "
import json, sys
f = {x['properties']['NAME']: x['geometry'] for x in json.load(open('example-places.geojson'))['features']}
want = {'point-le': ('Point', [24.94, 60.17]), 'polygon-be': ('Polygon', [20.6, 59.8]), 'multipolygon-z': ('MultiPolygon', [20.6, 59.8])}
for name, (kind, first) in want.items():
    g = f[name]
    c = g['coordinates']
    while isinstance(c[0], list): c = c[0]
    if g['type'] != kind or c != first: sys.exit('%s: %s %s, want %s %s' % (name, g['type'], c, kind, first))
if f['empty'] is not None or f['empty-point'] is not None: sys.exit('an empty geometry is not null')
"
# A path with "#" in it opens read-only and leaves nothing behind.
mkdir "odd#dir" && cp example.gpkg "odd#dir/x.gpkg"
py-docs "$TOOL" "odd#dir/x.gpkg" places example-odd.geojson >/dev/null
[ ! -e odd ] || { echo "a path with # created a stray file"; exit 1; }
DATA="${DRIVE_ROOT:-${DRIVE:-$SKILL/../../..}}/data/natural_earth_vector.gpkg"
if [ -f "$DATA" ]; then
  py-docs "$TOOL" natural_earth_vector.gpkg ne_110m_admin_0_countries countries.geojson --columns NAME,POP_EST
  py-docs -c "
import json, sys
features = json.load(open('countries.geojson'))['features']
if len(features) != 177: sys.exit('%d countries, want 177' % len(features))
fin = [f for f in features if f['properties']['NAME'] == 'Finland'][0]['geometry']['coordinates']
pts = []
stack = [fin]
while stack:
    c = stack.pop()
    if isinstance(c[0], (int, float)): pts.append(c)
    else: stack.extend(c)
box = (min(p[0] for p in pts), min(p[1] for p in pts), max(p[0] for p in pts), max(p[1] for p in pts))
want = (20.6, 59.8, 31.6, 70.1)
if any(abs(a - b) > 0.5 for a, b in zip(box, want)): sys.exit('Finland bbox %s, want %s' % (box, want))
print('Natural Earth: 177 countries, Finland bbox %s' % [round(v, 2) for v in box])
"
else
  echo "the drive has no Natural Earth GeoPackage; the template is checked with the fixture"
  cp example-places.geojson countries.geojson
fi
cp "$SKILL/templates/choropleth.html" example-map.html
if [ ! -f "$DATA" ]; then
  sed -i.bak -e 's/value: "POP_EST"/value: "VALUE"/' -e 's/scale: "log"/scale: "linear"/' example-map.html
fi
svalbard-drive html pack example-map.html -o example-packed.html >/dev/null
validate "$TMP/example-packed.html"
echo "geodata: ok"
