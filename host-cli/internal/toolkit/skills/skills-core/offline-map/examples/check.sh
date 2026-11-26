#!/bin/sh
# Offline check for offline-map.
#   sh check.sh FOLDER          validate a map packed with --dir (FOLDER holds
#                               index.html): it packs again without a
#                               problem, and with SVALBARD_HTML_CHECK set it
#                               draws the drive's basemap and its places
#                               through a server, offline, in Chromium and
#                               Firefox
#   sh check.sh                 self-test: pack the template over the
#                               drive's first PMTiles map in a temporary
#                               folder
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
DRIVE_DIR="${DRIVE_ROOT:-${DRIVE:-$SKILL/../../..}}"
EXPECT='window.svalbardMap && window.svalbardMap.features > 0 && document.getElementById("status").textContent === ""'
# The template also says how many basemap features it drew.
TEMPLATE_EXPECT='window.svalbardMap && window.svalbardMap.basemap > 0 && document.getElementById("status").textContent === ""'
TMP=$(mktemp -d "${TMPDIR:-/tmp}/map-check.XXXXXX")
trap 'case $TMP in */map-check.??????) rm -rf "$TMP" ;; esac' EXIT

if ! ls "$DRIVE_DIR"/maps/*.pmtiles >/dev/null 2>&1; then
  echo "the drive has no PMTiles map in $DRIVE_DIR/maps: nothing to test"
  exit 77
fi

# served PAGE_DIR: put the page beside the drive's maps folder in a
# temporary server root, as the drive's maps server serves the drive.
validate() {
  dir=${1%/}
  [ -f "$dir/index.html" ] || { echo "$dir has no index.html: pack with --dir"; exit 1; }
  if ! svalbard-drive html pack "$dir/index.html" --dir -o "$TMP/repacked" >"$TMP/pack.log" 2>&1; then
    cat "$TMP/pack.log"
    echo "$dir does not work offline (problems above)"
    exit 1
  fi
  if [ -z "${SVALBARD_HTML_CHECK:-}" ]; then
    echo "no browser check: SVALBARD_HTML_CHECK is not set"
    return
  fi
  mkdir -p "$TMP/served/workspace"
  ln -s "$(cd "$DRIVE_DIR/maps" && pwd)" "$TMP/served/maps"
  cp -R "$dir" "$TMP/served/workspace/page"
  "$SVALBARD_HTML_CHECK" --wait 20 --serve "$TMP/served" --expect "$EXPECT" "$TMP/served/workspace/page/index.html" || { echo "$dir fails in a browser (above)"; exit 1; }
}

if [ $# -gt 0 ]; then
  validate "$1"
  echo "offline-map: ok"
  exit 0
fi
# The largest map is the basemap (OSM extracts); overlays are small.
base=$(basename "$(ls -S "$DRIVE_DIR"/maps/*.pmtiles | head -n 1)")
sed "s/basemap: \"osm-uusimaa.pmtiles\"/basemap: \"$base\"/" "$SKILL/templates/map.html" > "$TMP/example-map.html"
svalbard-drive html pack "$TMP/example-map.html" --dir -o "$TMP/example-map" >/dev/null
EXPECT=$TEMPLATE_EXPECT
validate "$TMP/example-map"
echo "offline-map: ok"
