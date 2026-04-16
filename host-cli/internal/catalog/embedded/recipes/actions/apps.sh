#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

app_name="${1:?Usage: apps.sh <app-name>}"
app_dir="$DRIVE_ROOT/apps/$app_name"

if [ ! -d "$app_dir" ]; then
    ui_error "App not found: $app_dir"
    exit 1
fi

trap_cleanup
port="$(find_free_port 8083)"

DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
if [ -n "$DUFS_BIN" ]; then
    "$DUFS_BIN" --bind "127.0.0.1" --port "$port" "$DRIVE_ROOT" &
elif command -v python3 >/dev/null 2>&1; then
    python3 -m http.server "$port" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
else
    ui_error "No file server available (need dufs or python3)."
    exit 1
fi
SVALBARD_PIDS+=($!)
sleep 1
open_browser "http://localhost:$port/apps/$app_name/"
ui_status "$app_name: http://localhost:$port/apps/$app_name/"
wait_for_services
