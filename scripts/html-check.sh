#!/usr/bin/env bash
# Loads an HTML page in headless Chromium and Firefox with the network off
# and fails on console errors, uncaught exceptions, failed loads or any
# request that leaves the computer. For checking packed pages and the HTML
# skills' templates while developing; nothing on the stick runs it.
#
#   scripts/html-check.sh [--browser chromium|firefox|both] [--expect JS]
#       [--wait N] [--serve DIR] [--click SELECTOR --download FILE] page.html
#
# Browsers: HTML_CHECK_CHROMIUM and HTML_CHECK_FIREFOX, else the usual names
# on PATH and in /Applications. Each runs headless with a fresh profile in a
# temporary folder, never the user's. The network is cut three ways: the
# page's requests are watched (any that would leave fails the check), the
# browsers' proxy is a closed port (Chromium also emulates being offline),
# and the whole run is confined to loopback (sandbox-exec on macOS, a new
# network namespace on Linux), which the browsers' own control channels
# need. Exits 77 (SKIP) when node or no browser is found.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
command -v node >/dev/null 2>&1 || { echo "SKIP: node is not installed"; exit 77; }

find_first() {
  for candidate in "$@"; do
    if [[ $candidate == /* ]]; then
      [[ -x $candidate ]] && { printf '%s\n' "$candidate"; return; }
    elif command -v "$candidate" >/dev/null 2>&1; then
      command -v "$candidate"; return
    fi
  done
}
chromium=${HTML_CHECK_CHROMIUM:-$(find_first chromium chromium-browser google-chrome google-chrome-stable \
  "/Applications/Chromium.app/Contents/MacOS/Chromium" "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")}
firefox=${HTML_CHECK_FIREFOX:-$(find_first firefox "/Applications/Firefox.app/Contents/MacOS/firefox")}

browser_args=()
[[ -n $chromium ]] && browser_args+=(--chromium "$chromium")
[[ -n $firefox ]] && browser_args+=(--firefox "$firefox")

case "$(uname -s)" in
  Darwin)
    export HTML_CHECK_CONFINED=1
    exec sandbox-exec -p '(version 1)(allow default)(deny network*)(allow network* (local unix))(allow network* (remote unix))(allow network-outbound (remote ip "localhost:*"))(allow network-bind (local ip "localhost:*"))(allow network-inbound (local ip "localhost:*"))' \
      node "$here/html-check.mjs" "${browser_args[@]}" "$@" ;;
  Linux)
    export HTML_CHECK_CONFINED=1
    exec unshare -rn sh -c 'ip link set lo up 2>/dev/null || true; exec "$@"' sh node "$here/html-check.mjs" "${browser_args[@]}" "$@" ;;
  *)
    exec node "$here/html-check.mjs" "${browser_args[@]}" "$@" ;;
esac
