#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

DB="$DRIVE_ROOT/data/search.db"
if [ ! -f "$DB" ]; then
    ui_error "Search index not found at data/search.db"
    ui_error "Run 'Build search index' first."
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    ui_error "python3 not found. Required for search API server."
    exit 1
fi

# Parse arguments
PORT="${1:-9090}"
BIND="${2:-127.0.0.1}"
KIWIX_PORT="${3:-8080}"

export DB

trap_cleanup

SEARCH_SERVER="$DRIVE_ROOT/.svalbard/lib/search-server.py"
if [ ! -f "$SEARCH_SERVER" ]; then
    ui_error "Search server not found: $SEARCH_SERVER"
    exit 1
fi

ui_header "Svalbard Search API"
ui_status "Listening on http://${BIND}:${PORT}"
ui_status "Kiwix redirect target: http://localhost:${KIWIX_PORT}"
echo ""

python3 "$SEARCH_SERVER" "$PORT" "$BIND" "$KIWIX_PORT" &
SVALBARD_PIDS+=($!)

wait_for_services
