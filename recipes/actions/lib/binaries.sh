#!/usr/bin/env bash
# Shared: binary discovery
# Requires: platform.sh sourced first

find_binary() {
    local name="$1"
    local bin_dir="$DRIVE_ROOT/bin"
    local platform
    platform="$(detect_platform)"
    for dir in "$bin_dir/$platform" "$bin_dir"; do
        if [ -x "$dir/$name" ]; then
            echo "$dir/$name"
            return 0
        fi
    done
    if command -v "$name" >/dev/null 2>&1; then
        command -v "$name"
        return 0
    fi
    return 1
}
