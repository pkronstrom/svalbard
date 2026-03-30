#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

BIND="${1:-127.0.0.1}"
trap_cleanup

ui_header "Starting all services"

KIWIX_BIN="$(find_binary kiwix-serve 2>/dev/null || true)"
zim_files=()
if [ -d "$DRIVE_ROOT/zim" ]; then
    while IFS= read -r f; do
        [ -n "$f" ] && zim_files+=("$f")
    done < <(find "$DRIVE_ROOT/zim" -name "*.zim" -type f 2>/dev/null)
fi
if [ -n "$KIWIX_BIN" ] && [ ${#zim_files[@]} -gt 0 ]; then
    port="$(find_free_port 8080)"
    kiwix_port="$port"
    "$KIWIX_BIN" --port "$port" --address "$BIND" "${zim_files[@]}" &
    SVALBARD_PIDS+=($!)
    ui_status "Kiwix:  http://$BIND:$port"
fi

PMTILES_BIN="$(find_binary go-pmtiles 2>/dev/null || true)"
if [ -n "$PMTILES_BIN" ] && [ -d "$DRIVE_ROOT/maps" ]; then
    port="$(find_free_port 8081)"
    "$PMTILES_BIN" serve "$DRIVE_ROOT/maps" --port "$port" &
    SVALBARD_PIDS+=($!)
    ui_status "Maps:   http://$BIND:$port"
fi

LLAMA_BIN="$(find_binary llama-server 2>/dev/null || true)"
model="$(find "$DRIVE_ROOT/models" -name "*.gguf" -type f 2>/dev/null | head -1 || true)"
if [ -n "$LLAMA_BIN" ] && [ -n "$model" ]; then
    port="$(find_free_port 8082)"
    "$LLAMA_BIN" -m "$model" --port "$port" --host "$BIND" &
    SVALBARD_PIDS+=($!)
    ui_status "LLM:    http://$BIND:$port"
fi

DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
if [ -n "$DUFS_BIN" ]; then
    port="$(find_free_port 8083)"
    "$DUFS_BIN" --bind "$BIND" --port "$port" "$DRIVE_ROOT" &
    SVALBARD_PIDS+=($!)
    ui_status "Files:  http://$BIND:$port"
    [ -d "$DRIVE_ROOT/apps/map" ] && ui_status "Map:    http://$BIND:$port/apps/map/"
elif command -v python3 >/dev/null 2>&1; then
    port="$(find_free_port 8083)"
    python3 -m http.server "$port" --bind "$BIND" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
    SVALBARD_PIDS+=($!)
    ui_status "Files:  http://$BIND:$port"
fi

SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -n "$SQLITE_BIN" ] && [ -f "$DRIVE_ROOT/data/search.db" ]; then
    port="$(find_free_port 8084)"
    export DB="$DRIVE_ROOT/data/search.db" SQLITE_BIN KIWIX_PORT="${kiwix_port:-8080}"
    "$DRIVE_ROOT/.svalbard/actions/search-server.sh" "$port" "$BIND" "${kiwix_port:-8080}" &
    SVALBARD_PIDS+=($!)
    ui_status "Search: http://$BIND:$port"
fi

wait_for_services
