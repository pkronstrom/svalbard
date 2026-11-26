#!/bin/sh
# Calls one tool of the drive's MCP server and prints the result's text, for
# an agent that has a shell but not the drive's tools.
#
#   sh mcp_call.sh <tool> ['<JSON arguments>']
#   sh mcp_call.sh search '{"query": "water filter", "limit": 5}'
#
# The drive is $DRIVE_ROOT, else $DRIVE, else the drive this script is on.
set -eu
if [ $# -lt 1 ]; then
  echo "usage: sh mcp_call.sh <tool> ['<JSON arguments>']" >&2
  exit 2
fi
tool=$1
args=${2:-}
[ -n "$args" ] || args='{}'
drive=${DRIVE_ROOT:-${DRIVE:-}}
if [ -z "$drive" ]; then
  drive=$(cd "$(dirname "$0")/../../../.." && pwd)
fi
if [ ! -x "$drive/run" ]; then
  echo "no drive at $drive (set DRIVE_ROOT)" >&2
  exit 2
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mcp-call.XXXXXX")
cleanup() {
  case $tmp in */mcp-call.??????) rm -rf "$tmp" ;; esac
}
trap cleanup EXIT
mkfifo "$tmp/in"
"$drive/run" mcp <"$tmp/in" >"$tmp/out" 2>"$tmp/err" &
server=$!
exec 3>"$tmp/in"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"mcp_call.sh","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"$tool\",\"arguments\":$args}}" >&3
# The server stops reading when its input closes, so keep it open until the
# answer arrives (at most two minutes).
n=0
until grep -q '"id":2,' "$tmp/out" 2>/dev/null; do
  n=$((n + 1))
  if [ $n -gt 1200 ] || ! kill -0 $server 2>/dev/null; then
    break
  fi
  sleep 0.1
done
exec 3>&-
wait $server || true
line=$(grep '"id":2,' "$tmp/out" || true)
if [ -z "$line" ]; then
  echo "no answer from the drive's MCP server: $(tail -n 1 "$tmp/err")" >&2
  exit 1
fi
case $line in
  *'"error":{'*)
    echo "$line" >&2
    exit 1 ;;
esac
text=$(printf '%s\n' "$line" | sed -e 's/^.*"content":\[{"type":"text","text":"//' -e 's/"}\],"isError":[a-z]*}}$//' -e 's/"}\]}}$//' \
  -e 's/\\n/\
/g' -e 's/\\t/	/g' -e 's/\\"/"/g' -e 's/\\u0026/\&/g' -e 's/\\u003c/</g' -e 's/\\u003e/>/g' -e 's/\\\\/\\/g')
printf '%s\n' "$text"
case $line in
  *'"isError":true'*) exit 1 ;;
esac
