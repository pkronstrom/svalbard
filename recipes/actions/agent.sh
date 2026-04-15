#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

client_name="${1:?Usage: agent.sh <client-name>}"

client_bin="$(find_binary "$client_name" 2>/dev/null || true)"
[ -n "$client_bin" ] || { ui_error "$client_name not found."; exit 1; }

llama_bin="$(find_binary llama-server 2>/dev/null || true)"
[ -n "$llama_bin" ] || { ui_error "llama-server not found."; exit 1; }

embed_keywords=("embed" "nomic-embed" "bge-" "e5-" "arctic-embed")
models=()
while IFS= read -r model; do
    lower="$(basename "$model" | tr '[:upper:]' '[:lower:]')"
    skip=0
    for keyword in "${embed_keywords[@]}"; do
        if [[ "$lower" == *"$keyword"* ]]; then
            skip=1
            break
        fi
    done
    [ "$skip" -eq 0 ] && models+=("$model")
done < <(find "$DRIVE_ROOT/models" -name "*.gguf" -not -name "._*" -type f 2>/dev/null | sort)

[ "${#models[@]}" -gt 0 ] || { ui_error "No chat-capable GGUF models found in models/"; exit 1; }

select_model() {
    if [ "${#models[@]}" -eq 1 ]; then
        echo "${models[0]}"
        return 0
    fi

    ui_header "Choose model for $client_name"
    for i in "${!models[@]}"; do
        printf "  %2d) %s\n" "$((i + 1))" "$(basename "${models[$i]}")"
    done
    echo ""
    read -rp "  > " choice
    [[ "$choice" =~ ^[0-9]+$ ]] || return 1
    (( choice >= 1 && choice <= ${#models[@]} )) || return 1
    echo "${models[$((choice - 1))]}"
}

model="${2:-}"
if [ -z "$model" ]; then
    model="$(select_model)" || { ui_error "Invalid model selection."; exit 1; }
fi
[ -f "$model" ] || { ui_error "Model not found: $model"; exit 1; }

wait_for_llama() {
    local port="$1"
    if ! command -v curl >/dev/null 2>&1; then
        sleep 3
        return 0
    fi

    for _ in $(seq 1 30); do
        if curl -fsS "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    return 1
}

cleanup_on_exit() {
    cleanup_processes
}

trap cleanup_on_exit EXIT INT TERM

port="$(find_free_port 8082)"
model_name="$(basename "$model" .gguf)"
base_url="http://127.0.0.1:${port}/v1"

ui_status "Starting llama-server with ${model_name}"
"$llama_bin" -m "$model" --jinja --host 127.0.0.1 --port "$port" &
SVALBARD_PIDS+=($!)

wait_for_llama "$port" || { ui_error "llama-server did not become healthy in time."; exit 1; }

ui_status "Launching ${client_name} against ${model_name}"
export OPENAI_API_KEY="local"
export OPENAI_BASE_URL="$base_url"
export OPENAI_API_BASE="$base_url"
export OPENAI_MODEL="$model_name"
export OPENAI_DEFAULT_MODEL="$model_name"

"$client_bin"
