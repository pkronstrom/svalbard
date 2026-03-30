#!/usr/bin/env bash
# Shared: port utilities

find_free_port() {
    local start="${1:-8080}"
    local port="$start"
    while [ "$port" -lt "$((start + 100))" ]; do
        if ! (echo >/dev/tcp/localhost/"$port") 2>/dev/null; then
            echo "$port"
            return 0
        fi
        port=$((port + 1))
    done
    echo "$start"
}
