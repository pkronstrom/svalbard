#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

KIWIX_BIN="$(find_binary kiwix-serve 2>/dev/null || true)"
if [ -z "$KIWIX_BIN" ]; then
    ui_error "kiwix-serve not found. Run 'Provision tools' from the menu."
    exit 1
fi

zim_files=()
if [ -d "$DRIVE_ROOT/zim" ]; then
    while IFS= read -r f; do
        [ -n "$f" ] && zim_files+=("$f")
    done < <(find "$DRIVE_ROOT/zim" -name "*.zim" -type f 2>/dev/null | sort)
fi

if [ ${#zim_files[@]} -eq 0 ]; then
    ui_error "No ZIM files found in zim/"
    exit 1
fi

# If a specific ZIM filename was passed as $1, serve just that
if [ -n "${1:-}" ] && [ -f "$DRIVE_ROOT/zim/$1" ]; then
    trap_cleanup
    port="$(find_free_port 8080)"
    echo "Starting kiwix-serve on port $port..."
    "$KIWIX_BIN" --port "$port" "$DRIVE_ROOT/zim/$1" &
    SVALBARD_PIDS+=($!)
    sleep 1
    open_browser "http://localhost:$port"
    ui_status "Kiwix: http://localhost:$port"
    wait_for_services
    exit 0
fi

# Default: serve all ZIM files
trap_cleanup
port="$(find_free_port 8080)"
echo "Starting kiwix-serve on port $port with ${#zim_files[@]} ZIM files..."
"$KIWIX_BIN" --port "$port" "${zim_files[@]}" &
SVALBARD_PIDS+=($!)
sleep 1
open_browser "http://localhost:$port"
ui_status "Kiwix: http://localhost:$port"
wait_for_services
