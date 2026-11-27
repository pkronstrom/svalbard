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

# sandbox_profile PORT...: macOS sandbox rules that deny the network but
# these loopback ports and binding local Unix sockets.
sandbox_profile() {
  local rules='(version 1)(allow default)(deny network*)(allow network-bind (local unix))(allow network-inbound (local unix))'
  for p in "$@"; do
    rules+="(allow network-bind (local ip \"localhost:$p\"))(allow network-inbound (local ip \"localhost:$p\"))(allow network-outbound (remote ip \"localhost:$p\"))"
  done
  printf '%s' "$rules"
}

browser_args=()
[[ -n $chromium ]] && browser_args+=(--chromium "$chromium")
[[ -n $firefox ]] && browser_args+=(--firefox "$firefox")

# The browsers may reach only two loopback ports, Firefox's BiDi port and
# the --serve server's, and bind their own Unix sockets (Chromium's
# singleton); nothing else on this computer, no proxy, no DNS.
port() { node -e 'const s=require("net").createServer().listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})'; }
export HTML_CHECK_FIREFOX_PORT=${HTML_CHECK_FIREFOX_PORT:-$(port)}
export HTML_CHECK_SERVE_PORT=${HTML_CHECK_SERVE_PORT:-$(port)}
unset http_proxy HTTP_PROXY https_proxy HTTPS_PROXY all_proxy ALL_PROXY ftp_proxy FTP_PROXY no_proxy NO_PROXY
# The browsers' own sandboxes cannot start inside this one.
export HTML_CHECK_CONFINED=1

case "$(uname -s)" in
  Darwin)
    # Already inside a sandbox (scripts/skill-run-check.sh, which allows the
    # same two ports): a second cannot be started, and is not needed.
    if ! sandbox-exec -p '(version 1)(allow default)' /usr/bin/true 2>/dev/null; then
      exec node "$here/html-check.mjs" "${browser_args[@]}" "$@"
    fi
    exec sandbox-exec -p "$(sandbox_profile "$HTML_CHECK_FIREFOX_PORT" "$HTML_CHECK_SERVE_PORT")" \
      node "$here/html-check.mjs" "${browser_args[@]}" "$@" ;;
  Linux)
    exec unshare -rn sh -c 'ip link set lo up 2>/dev/null || true; exec "$@"' sh node "$here/html-check.mjs" "${browser_args[@]}" "$@" ;;
  *)
    exec node "$here/html-check.mjs" "${browser_args[@]}" "$@" ;;
esac
