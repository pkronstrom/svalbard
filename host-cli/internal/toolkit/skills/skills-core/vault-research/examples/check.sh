#!/bin/sh
# Offline check for vault-research.
#   sh check.sh ANSWER.md   the answer cites a source on this drive: a
#                           source's name inside parentheses or on a line
#                           that says "source"
#   sh check.sh             self-test: list the drive's sources over its MCP
#                           server, search for a word from their
#                           descriptions and read the top result
# On a drive without a search index there is nothing to test: the check
# prints SKIP and exits 77.
set -eu
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
CALL="$SKILL/scripts/mcp_call.sh"
drive=${DRIVE_ROOT:-${DRIVE:-$(cd "$SKILL/../../.." && pwd)}}
export DRIVE_ROOT="$drive"
if [ ! -f "$drive/data/search.db" ]; then
  echo "nothing to search: $drive has no search index"
  exit 77
fi
answer=""
if [ $# -gt 0 ]; then
  answer=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
  [ -s "$answer" ] || { echo "$1 is missing or empty"; exit 1; }
fi
# Work in a temporary folder, never the caller's.
REF=$(mktemp -d "${TMPDIR:-/tmp}/research-check.XXXXXX")
trap 'case $REF in */research-check.??????) rm -rf "$REF" ;; esac' EXIT
cd "$REF"
sh "$CALL" vault_sources > sources.json
tr '{' '\n' < sources.json | sed -n 's/.*"id":"\([^"]*\)".*"description":"\([^"]*\)".*/\1 \2/p' > sources.txt
[ -s sources.txt ] || { echo "vault_sources listed no source"; exit 1; }
if [ -n "$answer" ]; then
  # What counts as a citation: text in parentheses, and lines naming a source.
  { grep -o '([^)]*)' "$answer" || true; grep -i 'source' "$answer" || true; } | tr 'A-Z' 'a-z' > cites.txt
  [ -s cites.txt ] || { echo "$1 cites nothing: no (Title, source) and no Sources line"; exit 1; }
  # A source's names: its id up to the first "_" or "." (four letters or
  # more), and its description up to " — ".
  sed 's/^\([^ _.]*\)[^ ]* .*/\1/' sources.txt | tr 'A-Z' 'a-z' | awk 'length >= 4' > keys.txt
  sed -e 's/^[^ ]* //' -e 's/ — .*//' sources.txt | tr 'A-Z' 'a-z' | awk 'length >= 4' >> keys.txt
  while IFS= read -r key; do
    if grep -qF -- "$key" cites.txt; then
      echo "$1: ok (cites $key)"
      exit 0
    fi
  done < keys.txt
  echo "$1 cites no source on this drive"
  exit 1
fi
# Search for a long word from a source's description until one finds
# something, then read that result.
for word in $(sed 's/^[^ ]* //' sources.txt | tr -cs 'A-Za-z' '\n' | awk 'length >= 6' | head -n 5); do
  sh "$CALL" search "{\"query\": \"$word\", \"limit\": 3}" > results.json || continue
  path=$(sed -n 's/^\[{"source":"\([^"]*\)","path":"\([^"]*\)".*/\1|\2/p' results.json)
  [ -n "$path" ] || continue
  source=${path%%|*}
  sh "$CALL" search_read "{\"source\": \"$source\", \"path\": \"${path#*|}\"}" > article.txt
  [ -s article.txt ] || { echo "search_read of ${path#*|} in $source returned nothing"; exit 1; }
  echo "vault-research: ok (\"$word\" found ${path#*|} in $source, $(wc -c < article.txt | tr -d ' ') bytes read)"
  exit 0
done
echo "no search for words from the sources' descriptions found anything"
exit 1
