#!/usr/bin/env bash
# Cross-compile svalbard-drive for all supported platforms.
# Output goes to host-cli/internal/toolkit/embedded/<platform>/svalbard-drive
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DRIVE_SRC="$REPO_ROOT/drive-runtime"
EMBED_DIR="$REPO_ROOT/host-cli/internal/toolkit/embedded"

PLATFORMS=(
    "darwin:arm64:macos-arm64"
    "darwin:amd64:macos-x86_64"
    "linux:arm64:linux-arm64"
    "linux:amd64:linux-x86_64"
)

echo "Building svalbard-drive for ${#PLATFORMS[@]} platforms..."

for entry in "${PLATFORMS[@]}"; do
    IFS=: read -r goos goarch platform <<< "$entry"
    outdir="$EMBED_DIR/$platform"
    mkdir -p "$outdir"
    echo "  $platform (GOOS=$goos GOARCH=$goarch)"
    (cd "$DRIVE_SRC" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -ldflags="-s -w" -o "$outdir/svalbard-drive" \
        ./cmd/svalbard-drive)
done

echo "Done. Binaries in $EMBED_DIR"
ls -lh "$EMBED_DIR"/*/svalbard-drive
