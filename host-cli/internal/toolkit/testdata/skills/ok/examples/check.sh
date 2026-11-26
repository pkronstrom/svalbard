#!/bin/sh
# Passes offline: writes a file in the work folder and reads it back.
echo hello > out.txt
[ "$(cat out.txt)" = hello ] || { echo "out.txt does not say hello"; exit 1; }
[ -n "$DRIVE" ] && [ -d "$DRIVE" ] || { echo "DRIVE is not the vault"; exit 1; }
echo ok
