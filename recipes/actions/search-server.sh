#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

DB="$DRIVE_ROOT/data/search.db"
if [ ! -f "$DB" ]; then
    ui_error "Search index not found at data/search.db"
    ui_error "Run 'Build search index' first."
    exit 1
fi

SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -z "$SQLITE_BIN" ]; then
    ui_error "sqlite3 not found. Run 'Provision tools' from the menu."
    exit 1
fi

# Parse arguments
PORT="${1:-9090}"
BIND="${2:-127.0.0.1}"
KIWIX_PORT="${3:-8080}"

export DB SQLITE_BIN DRIVE_ROOT KIWIX_PORT

CGI_SCRIPT="$DRIVE_ROOT/.svalbard/lib/search-cgi.sh"
if [ ! -x "$CGI_SCRIPT" ]; then
    ui_error "CGI handler not found or not executable: $CGI_SCRIPT"
    exit 1
fi

SOCAT_BIN="$(command -v socat 2>/dev/null || true)"
if [ -z "$SOCAT_BIN" ]; then
    ui_error "socat not found."
    ui_error "Install socat to run the search API server."
    ui_error "  macOS:  brew install socat"
    ui_error "  Linux:  apt install socat"
    exit 1
fi

trap_cleanup

ui_header "Svalbard Search API"
ui_status "Listening on http://${BIND}:${PORT}"
ui_status "Kiwix redirect target: http://localhost:${KIWIX_PORT}"
echo ""

"$SOCAT_BIN" "TCP-LISTEN:${PORT},bind=${BIND},reuseaddr,fork" \
    "EXEC:${CGI_SCRIPT}" &
SVALBARD_PIDS+=($!)

wait_for_services
