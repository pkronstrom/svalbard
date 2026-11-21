#!/usr/bin/env bash
# Compatibility entry point; the Go runtime owns offline cache preparation.
set -euo pipefail
: "${DRIVE_ROOT:?Open the embedded shell through the vault's run menu}"
exec "$DRIVE_ROOT/run" embedded
