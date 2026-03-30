#!/usr/bin/env bash
# Shared: platform detection

detect_platform() {
    local os arch
    case "$(uname -s)" in
        Darwin*) os="macos" ;;
        Linux*)  os="linux" ;;
        *)       os="unknown" ;;
    esac
    case "$(uname -m)" in
        x86_64)       arch="x86_64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)            arch="unknown" ;;
    esac
    echo "${os}-${arch}"
}
