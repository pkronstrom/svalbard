#!/usr/bin/env bash
# Publish the same graphical wrappers installed by the optional recipe.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd -P)
out=${1:?usage: package-web-launcher.sh output-directory}
if [[ $# != 1 ]]; then echo "Usage: package-web-launcher.sh output-directory" >&2; exit 2; fi
mkdir -p "$out"
out=$(cd "$out" && pwd -P)
stage=$(mktemp -d "$out/.launcher-package.XXXXXXXX")
trap 'rm -rf "$stage"' EXIT
source_dir="$root/host-cli/internal/weblauncher/package"
contents="Svalbard.app/Contents"

mkdir -p "$stage/$contents/MacOS" "$stage/$contents/Resources"
install -m 644 "$source_dir/$contents/Info.plist" "$stage/$contents/"
install -m 755 "$source_dir/$contents/MacOS/svalbard-launcher" "$stage/$contents/MacOS/"
install -m 644 "$source_dir/$contents/Resources/svalbard.icns" "$stage/$contents/Resources/"
install -m 755 "$source_dir/Svalbard-Linux" "$source_dir/Svalbard.desktop" "$stage/"

# Stable member order, UTC times and permissions make the transport ZIP
# byte-identical when the three recipe files have not changed.
export TZ=UTC
touch -t 200001010000.00 "$stage/$contents/Info.plist" "$stage/$contents/MacOS/svalbard-launcher" "$stage/$contents/Resources/svalbard.icns"
(cd "$stage" && zip -q -X Svalbard-macOS.zip "$contents/Info.plist" "$contents/MacOS/svalbard-launcher" "$contents/Resources/svalbard.icns" &&
    unzip -tq Svalbard-macOS.zip &&
    shasum -a 256 Svalbard-macOS.zip Svalbard-Linux Svalbard.desktop > SHA256SUMS)

# A packaging destination can already hold a previous release. Never replace
# an unrelated application or a personal bundle with extra files.
if [[ -e "$out/Svalbard.app" ]]; then
    python3 - "$out/Svalbard.app" <<'PY'
import plistlib, sys
from pathlib import Path
app = Path(sys.argv[1])
expected = {"Contents/Info.plist", "Contents/MacOS/svalbard-launcher", "Contents/Resources/svalbard.icns"}
actual = {p.relative_to(app).as_posix() for p in app.rglob("*") if p.is_file() or p.is_symlink()}
if app.is_symlink() or actual != expected or plistlib.loads((app/"Contents/Info.plist").read_bytes()).get("CFBundleIdentifier") != "net.bembu.svalbard.launcher":
    raise SystemExit("Choose an output directory that does not contain a different Svalbard.app")
PY
    mv "$out/Svalbard.app" "$stage/previous.app"
fi
if ! mv "$stage/Svalbard.app" "$out/Svalbard.app"; then
    if [[ -d "$stage/previous.app" ]]; then mv "$stage/previous.app" "$out/Svalbard.app"; fi
    exit 1
fi
for file in Svalbard-macOS.zip Svalbard-Linux Svalbard.desktop SHA256SUMS; do
    mv "$stage/$file" "$out/$file"
done
(cd "$out" && shasum -a 256 -c SHA256SUMS)
