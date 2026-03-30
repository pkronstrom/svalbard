#!/usr/bin/env bash
# Shared: terminal UI helpers

if [ -t 1 ]; then
    BOLD=$'\033[1m'
    DIM=$'\033[2m'
    RED=$'\033[0;31m'
    GREEN=$'\033[0;32m'
    YELLOW=$'\033[0;33m'
    CYAN=$'\033[0;36m'
    NC=$'\033[0m'
else
    BOLD="" DIM="" RED="" GREEN="" YELLOW="" CYAN="" NC=""
fi

ui_header() {
    echo ""
    echo "${BOLD}$1${NC}"
    echo "─────────────────────────────────────────"
}

ui_status() {
    echo "  ${GREEN}$1${NC}"
}

ui_error() {
    echo "  ${RED}$1${NC}" >&2
}

open_browser() {
    local url="$1"
    case "$(uname -s)" in
        Darwin*) open "$url" ;;
        Linux*)  xdg-open "$url" 2>/dev/null || echo "  Open: $url" ;;
    esac
}
