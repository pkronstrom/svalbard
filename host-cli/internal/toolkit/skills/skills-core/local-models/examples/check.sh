#!/bin/sh
# Offline check for local-models. With no argument: list the drive's chat
# models and check that the one models.sh recommends is a model file on the
# drive that fits this computer. With a file (the agent's choice, a model
# file name on its first line): the same for that model. It writes nothing
# in the current folder.
set -eu
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
drive=${DRIVE_ROOT:-${DRIVE:-$(cd "$SKILL/../../.." && pwd)}}
export DRIVE_ROOT="$drive"
if [ $# -gt 0 ]; then
  [ -s "$1" ] || { echo "$1 is missing or empty"; exit 1; }
  choice=$(sed -n '1{s/^[[:space:]]*//;s/[[:space:]]*$//;p;}' "$1")
  choice=${choice##*/}
fi
# Work in a temporary folder, never the caller's.
REF=$(mktemp -d "${TMPDIR:-/tmp}/models-check.XXXXXX")
trap 'case $REF in */models-check.??????) rm -rf "$REF" ;; esac' EXIT
cd "$REF"
sh "$SKILL/scripts/models.sh" > models.txt
cat models.txt
if [ $# -eq 0 ]; then
  choice=$(sed -n 's/^recommended: \([^ ]*\).*/\1/p' models.txt)
fi
[ -n "$choice" ] || { echo "no model chosen"; exit 1; }
case $choice in *.gguf) ;; *) choice=$choice.gguf ;; esac
[ -e "$drive/models/$choice" ] || { echo "$choice is not in $drive/models"; exit 1; }
line=$(grep "^  $choice " models.txt || true)
[ -n "$line" ] || { echo "$choice is not a chat model models.sh lists"; exit 1; }
case $line in
  *"does not fit"*) echo "$choice does not fit this computer's memory"; exit 1 ;;
esac
echo "local-models: ok ($choice)"
