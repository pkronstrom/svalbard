#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

LLAMA_BIN="$(find_binary llama-server 2>/dev/null || true)"
if [ -z "$LLAMA_BIN" ]; then
    ui_error "llama-server not found."
    exit 1
fi

model="${1:-}"
if [ -z "$model" ]; then
    model="$(find "$DRIVE_ROOT/models" -name "*.gguf" -not -name "._*" -type f 2>/dev/null | head -1)"
fi
if [ -z "$model" ] || [ ! -f "$model" ]; then
    ui_error "No GGUF model found in models/"
    exit 1
fi

trap_cleanup
port="$(find_free_port 8082)"
echo "Starting llama-server on port $port with $(basename "$model")..."
"$LLAMA_BIN" -m "$model" --port "$port" &
SVALBARD_PIDS+=($!)
sleep 2
open_browser "http://localhost:$port"
ui_status "LLM: http://localhost:$port"
wait_for_services
