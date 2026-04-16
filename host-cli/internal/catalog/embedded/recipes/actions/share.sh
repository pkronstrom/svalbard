#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

lan_ip() {
    case "$(uname -s)" in
        Darwin*) ipconfig getifaddr en0 2>/dev/null || echo "0.0.0.0" ;;
        Linux*)  hostname -I 2>/dev/null | awk '{print $1}' || echo "0.0.0.0" ;;
    esac
}

IP="$(lan_ip)"

ui_header "Sharing drive on local network"

DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
trap_cleanup
port="$(find_free_port 8080)"

if [ -n "$DUFS_BIN" ]; then
    "$DUFS_BIN" --bind "0.0.0.0" --port "$port" "$DRIVE_ROOT" &
elif command -v python3 >/dev/null 2>&1; then
    python3 -m http.server "$port" --bind "0.0.0.0" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
else
    ui_error "No file server available (need dufs or python3)."
    exit 1
fi
SVALBARD_PIDS+=($!)

echo ""
echo "  ${BOLD}http://${IP}:${port}${NC}"
echo ""
echo "  Tell others to open this address in their browser."

wait_for_services
