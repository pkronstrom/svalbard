#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

trap_cleanup

# Serve the entire drive root on one port — no CORS issues.
# PMTiles JS reads /maps/*.pmtiles via byte-range requests,
# map viewer loads from /apps/map/index.html on the same origin.
port="$(find_free_port 8081)"

DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
if [ -n "$DUFS_BIN" ]; then
    "$DUFS_BIN" --bind "127.0.0.1" --port "$port" --allow-all --render-try-index "$DRIVE_ROOT" &
elif command -v python3 >/dev/null 2>&1; then
    python3 -m http.server "$port" --bind 127.0.0.1 --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
else
    ui_error "No file server available (install dufs or python3)."
    exit 1
fi
SVALBARD_PIDS+=($!)
sleep 1

if [ -f "$DRIVE_ROOT/apps/map/index.html" ]; then
    open_browser "http://localhost:$port/apps/map/"
    ui_status "Map viewer: http://localhost:$port/apps/map/"
fi
ui_status "Files: http://localhost:$port"
wait_for_services
