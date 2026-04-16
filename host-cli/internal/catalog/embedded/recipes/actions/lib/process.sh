#!/usr/bin/env bash
# Shared: process management

SVALBARD_PIDS=()

cleanup_processes() {
    echo ""
    echo "Shutting down..."
    for pid in "${SVALBARD_PIDS[@]}"; do
        kill "$pid" 2>/dev/null && echo "  Stopped PID $pid" || true
    done
}

trap_cleanup() {
    trap cleanup_processes SIGINT SIGTERM
}

wait_for_services() {
    if [ ${#SVALBARD_PIDS[@]} -gt 0 ]; then
        echo ""
        echo "Services running. Press Ctrl+C to stop all."
        wait
    fi
}
