#!/usr/bin/env bash
# Report recipes whose size_gb drifted from the upstream Content-Length.
# Needs the network; exits 1 when any recipe drifted. Pass -threshold 0.2 etc.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/host-cli"
exec go run ./cmd/size-drift "$@"
