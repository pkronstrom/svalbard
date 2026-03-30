#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

PMTILES_BIN="$(find_binary pmtiles 2>/dev/null || find_binary go-pmtiles 2>/dev/null || true)"
if [ -z "$PMTILES_BIN" ]; then
    ui_error "pmtiles not found."
    exit 1
fi

trap_cleanup

tile_port="$(find_free_port 8081)"
echo "Starting tile server on port $tile_port..."
"$PMTILES_BIN" serve "$DRIVE_ROOT/maps" --port "$tile_port" &
SVALBARD_PIDS+=($!)

if [ -f "$DRIVE_ROOT/apps/map/index.html" ]; then
    app_port="$(find_free_port 8083)"
    DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
    if [ -n "$DUFS_BIN" ]; then
        "$DUFS_BIN" --bind "127.0.0.1" --port "$app_port" "$DRIVE_ROOT" &
    elif command -v python3 >/dev/null 2>&1; then
        python3 -m http.server "$app_port" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
    else
        ui_error "No file server available for map viewer."
        wait_for_services
        exit 1
    fi
    SVALBARD_PIDS+=($!)
    sleep 1
    open_browser "http://localhost:$app_port/apps/map/"
    ui_status "Map viewer: http://localhost:$app_port/apps/map/"
else
    ui_status "Tile server: http://localhost:$tile_port"
fi

ui_status "Tiles: http://localhost:$tile_port"
wait_for_services
