#!/bin/sh
# Runs one PlatformIO command in a project folder through the drive's
# Embedded Dev Shell, offline, and exits with PlatformIO's status.
#
#   sh pio.sh <project folder> run                 # build the default board
#   sh pio.sh <project folder> run -e esp32s3      # build another board
#   sh pio.sh <project folder> run -t upload       # flash: needs a board on USB
#
# The first use on a computer prepares PlatformIO's cache from the drive (a
# few minutes, about 5 GB). SVALBARD_PIO_CACHE chooses where; on macOS it
# must not be under /tmp.
set -eu
if [ $# -lt 2 ]; then
  echo "usage: sh pio.sh <project folder> <pio arguments...>" >&2
  exit 2
fi
project=$(cd "$1" && pwd)
shift
drive=${DRIVE_ROOT:-${DRIVE:-$(cd "$(dirname "$0")/../../../.." && pwd)}}
[ -f "$project/platformio.ini" ] || { echo "$project has no platformio.ini" >&2; exit 2; }
quote() { printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"; }
command="cd $(quote "$project") && pio"
for arg in "$@"; do
  command="$command $(quote "$arg")"
done
printf '%s\nexit $?\n' "$command" | SHELL=/bin/bash "$drive/run" embedded
