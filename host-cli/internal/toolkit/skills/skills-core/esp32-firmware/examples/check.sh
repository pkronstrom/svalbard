#!/bin/sh
# Offline check for esp32-firmware.
#   sh check.sh PROJECT [-e ENV]   build the project as it is now (ENV
#                                  default: its default_envs) and validate
#                                  that firmware: a build that fails fails
#                                  the check, whatever an older build left
#   sh check.sh FIRMWARE.bin       validate one image file
#   sh check.sh                    self-test in a temporary folder: build
#                                  the drive's example, then show that a
#                                  broken source fails the check
# The first build on a computer prepares PlatformIO's cache (a few minutes,
# about 5 GB; SVALBARD_PIO_CACHE chooses where).
set -eu
SKILL="$(cd "$(dirname "$0")/.." && pwd)"
drive=${DRIVE_ROOT:-${DRIVE:-$(cd "$SKILL/../../.." && pwd)}}
export DRIVE_ROOT="$drive"

validate() {
  target=$1
  env=${2:-}
  if [ -d "$target" ]; then
    [ -f "$target/platformio.ini" ] || { echo "$target has no platformio.ini"; return 1; }
    [ -n "$env" ] || env=$(sed -n 's/^default_envs *= *\([^ ,]*\).*/\1/p' "$target/platformio.ini" | head -n 1)
    [ -n "$env" ] || env=esp32
    # Build now: PlatformIO keeps the last good firmware.bin when a build
    # fails, so an existing file proves nothing about the current source.
    if ! sh "$SKILL/scripts/pio.sh" "$target" run -e "$env" > "${TMPDIR:-/tmp}/esp32-check-build.$$.log" 2>&1; then
      grep -E 'error|Error' "${TMPDIR:-/tmp}/esp32-check-build.$$.log" | head -n 5
      rm -f "${TMPDIR:-/tmp}/esp32-check-build.$$.log"
      echo "$target does not build for $env"
      return 1
    fi
    rm -f "${TMPDIR:-/tmp}/esp32-check-build.$$.log"
    bin=$target/.pio/build/$env/firmware.bin
    [ -s "$bin" ] || { echo "$target built no firmware for $env ($bin)"; return 1; }
  else
    bin=$target
    [ -s "$bin" ] || { echo "$bin is missing or empty"; return 1; }
  fi
# An ESP32 application image starts with the byte 0xE9.
  magic=$(od -An -tx1 -N1 "$bin" | tr -d ' \n')
  [ "$magic" = e9 ] || { echo "$bin is not an ESP32 image (first byte $magic, want e9)"; return 1; }
  echo "esp32-firmware: ok ($bin, $(wc -c < "$bin" | tr -d ' ') bytes)"
}

if [ $# -gt 0 ]; then
  target=$1
  shift
  env=""
  if [ "${1:-}" = -e ] && [ $# -ge 2 ]; then
    env=$2
  fi
  validate "$target" "$env"
  exit $?
fi
# The self-test works in its own temporary folder, never the caller's,
# where the agent's files are.
REF=$(mktemp -d "${TMPDIR:-/tmp}/esp32-check.XXXXXX")
trap 'case $REF in */esp32-check.??????) rm -rf "$REF" ;; esac' EXIT
cd "$REF"
cp -R "$drive/tools/platformio/examples/blink" example-blink
validate example-blink
printf '\nthis is not C\n' >> example-blink/src/main.c
if validate example-blink > broken.txt; then
  echo "a project that no longer builds still passed"
  exit 1
fi
echo "esp32-firmware: ok (example builds; a broken source fails the check)"
