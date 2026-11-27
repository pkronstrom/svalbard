#!/bin/sh
# Offline check for html-app.
#   sh check.sh PAGE.html   validate a packed page: nothing left to pack, it
#                           packs again without a problem, and with
#                           SVALBARD_HTML_CHECK set it loads offline in
#                           Chromium and Firefox without an error and shows
#                           text
#   sh check.sh             self-test: pack the template in a temporary
#                           folder; its list renders and its fonts load
set -e
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
TMP=$(mktemp -d "${TMPDIR:-/tmp}/html-app-check.XXXXXX")
trap 'case $TMP in */html-app-check.??????) rm -rf "$TMP" ;; esac' EXIT

validate() {
  if grep -Eq '<script[^>]*[[:space:]]src=|<link[^>]*[[:space:]]href=["'"'"']?[^"'"'"'#d]|^[[:space:]]*import[[:space:]].*from[[:space:]]*["'"'"'][^./"'"'"']' "$1"; then
    echo "$1 is not packed: run svalbard-drive html pack on the page and check the file it writes"
    exit 1
  fi
  if ! svalbard-drive html pack "$1" -o "$TMP/repacked.html" >"$TMP/pack.log" 2>&1; then
    cat "$TMP/pack.log"
    echo "$1 does not work offline (problems above)"
    exit 1
  fi
  if [ -n "${SVALBARD_HTML_CHECK:-}" ]; then
    if "$SVALBARD_HTML_CHECK" --expect "$2" "$1"; then :; else
      status=$?
      [ $status -eq 77 ] || { echo "$1 fails in a browser (above)"; exit 1; }
      echo "browser check skipped: a browser is missing"
    fi
  else
    echo "no browser check: SVALBARD_HTML_CHECK is not set"
  fi
}

if [ $# -gt 0 ]; then
  validate "$1" 'document.body.innerText.trim().length > 0'
  echo "html-app: ok"
  exit 0
fi
cp "$SKILL/templates/app.html" "$TMP/example-app.html"
svalbard-drive html pack "$TMP/example-app.html" -o "$TMP/example-packed.html" >/dev/null
validate "$TMP/example-packed.html" '(async () => { await document.fonts.ready; return document.querySelectorAll("#items li").length === 2 && getComputedStyle(document.querySelector("h1")).fontFamily.includes("Fraunces") && document.fonts.check("16px \"Fraunces\"") && document.fonts.check("16px \"Inter\""); })()'
echo "html-app: ok"
