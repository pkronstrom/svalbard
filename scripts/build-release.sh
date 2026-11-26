#!/usr/bin/env bash
# Build release archives of the svalbard host CLI for every supported host:
#
#   scripts/build-release.sh <version> <out-dir> [repo-root]
#
# Each archive (svalbard_<version>_<platform>.tar.gz) holds the svalbard
# binary, with the drive runtime for all four platforms embedded, plus
# LICENSE and README.md. SHA256SUMS lists the archives. repo-root defaults
# to this script's checkout; the release workflow passes an older tag's
# checkout to backfill a release.
set -euo pipefail

version=${1:?usage: build-release.sh <version> <out-dir> [repo-root]}
out=${2:?usage: build-release.sh <version> <out-dir> [repo-root]}
root=${3:-"$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"}
root=$(cd "$root" && pwd)
mkdir -p "$out"
out=$(cd "$out" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

# Embeds svalbard-drive for every platform into the host CLI.
"$root/scripts/build-drive-runtime.sh"

for entry in darwin:arm64:macos-arm64 darwin:amd64:macos-x86_64 linux:arm64:linux-arm64 linux:amd64:linux-x86_64; do
    IFS=: read -r goos goarch platform <<< "$entry"
    name="svalbard_${version}_${platform}"
    mkdir -p "$stage/$name"
    echo "Building $name"
    (cd "$root/host-cli" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath \
        -ldflags "-s -w -X github.com/pkronstrom/svalbard/host-cli/internal/cli.Version=$version" \
        -o "$stage/$name/svalbard" ./cmd/svalbard)
    cp "$root/LICENSE" "$root/README.md" "$stage/$name/"
    tar -C "$stage" -czf "$out/$name.tar.gz" "$name"
done

cd "$out"
if command -v sha256sum >/dev/null; then
    sha256sum svalbard_"${version}"_*.tar.gz > SHA256SUMS
else
    shasum -a 256 svalbard_"${version}"_*.tar.gz > SHA256SUMS
fi
cat SHA256SUMS
