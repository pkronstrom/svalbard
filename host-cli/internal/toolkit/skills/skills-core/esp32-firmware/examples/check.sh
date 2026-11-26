#!/bin/sh
# Offline check for esp32-firmware. With no argument: copy the drive's ESP32
# example, build it with scripts/pio.sh and validate firmware.bin. With an
# argument (a project folder or a firmware.bin): validate that firmware.
# The first build on a computer prepares PlatformIO's cache (a few minutes,
# about 5 GB; SVALBARD_PIO_CACHE chooses where).
set -eu
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
drive=${DRIVE_ROOT:-${DRIVE:-$(cd "$SKILL/../../.." && pwd)}}
export DRIVE_ROOT="$drive"

validate() {
  bin=$1
  if [ -d "$bin" ]; then
    bin=$(ls "$bin"/.pio/build/*/firmware.bin 2>/dev/null | head -n 1)
    [ -n "$bin" ] || { echo "$1 has no .pio/build/*/firmware.bin; build it first"; exit 1; }
  fi
  [ -s "$bin" ] || { echo "$bin is missing or empty"; exit 1; }
  # An ESP32 application image starts with the byte 0xE9.
  magic=$(od -An -tx1 -N1 "$bin" | tr -d ' \n')
  [ "$magic" = e9 ] || { echo "$bin is not an ESP32 image (first byte $magic, want e9)"; exit 1; }
  echo "esp32-firmware: ok ($bin, $(wc -c < "$bin" | tr -d ' ') bytes)"
}

if [ $# -gt 0 ]; then
  validate "$1"
  exit 0
fi
cp -R "$drive/tools/platformio/examples/blink" blink
sh "$SKILL/scripts/pio.sh" blink run
validate blink
