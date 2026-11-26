#!/bin/sh
# Offline check for vault-research. With no argument: list the drive's
# sources over its MCP server, search for a word from the first ones'
# descriptions and read the top result. With a file (an answer the agent
# wrote): it must cite a source that is on this drive. A drive without a
# search index passes with a note: there is nothing to search.
set -eu
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
CALL="$SKILL/scripts/mcp_call.sh"
drive=${DRIVE_ROOT:-${DRIVE:-$(cd "$SKILL/../../.." && pwd)}}
export DRIVE_ROOT="$drive"
if [ ! -f "$drive/data/search.db" ]; then
  echo "vault-research: ok (skipped: $drive has no search index)"
  exit 0
fi
sh "$CALL" vault_sources > sources.json
# Name keys of each source: the id up to its first "_" or ".", and the first
# word of its description, lower case, four letters or more.
tr '{' '\n' < sources.json | sed -n 's/.*"id":"\([^"]*\)".*"description":"\([^"]*\)".*/\1 \2/p' > sources.txt
[ -s sources.txt ] || { echo "vault_sources listed no source"; exit 1; }
if [ $# -gt 0 ]; then
  answer=$(tr 'A-Z' 'a-z' < "$1")
  for key in $(sed 's/^\([^ _.]*\)[^ ]* \([A-Za-z0-9-]*\).*/\1 \2/' sources.txt | tr 'A-Z' 'a-z'); do
    [ ${#key} -ge 4 ] || continue
    case $answer in *"$key"*) echo "$1: ok (cites $key)"; exit 0 ;; esac
  done
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
