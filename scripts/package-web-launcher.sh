#!/usr/bin/env bash
# Package a fresh, empty vault built by the same host CLI used for real media.
# A single-target archive carries only that target's shared native runtime.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd -P)
out=${1:?usage: package-web-launcher.sh output-directory [all|macos-arm64|macos-x86_64|linux-arm64|linux-x86_64]}
target=${2:-all}
case "$target" in all|macos-arm64|macos-x86_64|linux-arm64|linux-x86_64) ;; *) echo "Unsupported platform $target" >&2; exit 2;; esac
if [[ ! -x $root/bin/svalbard ]]; then echo "Run make build first." >&2; exit 1; fi
mkdir -p "$out"
out=$(cd "$out" && pwd -P)
stage=$(mktemp -d "$out/.launcher-package.XXXXXXXX")
trap 'rm -rf "$stage"' EXIT
"$root/bin/svalbard" init "$stage/Svalbard" --add web-launcher --platform "$target" --apply
cp "$root/docs/web-launcher.md" "$stage/Svalbard/Launcher guide.md"
cp "$root/LICENSE" "$root/THIRD-PARTY-NOTICES.md" "$stage/Svalbard/"
python3 - "$stage" "$out" "$target" <<'PY'
import hashlib
from pathlib import Path
import sys
import zipfile

stage, out = map(Path, sys.argv[1:3])
target = sys.argv[3]
name = f"Svalbard-Launcher-{target}.zip"
archive = stage / name
vault = stage / "Svalbard"
with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as bundle:
    for file in sorted(vault.rglob("*")):
        if not file.is_file():
            continue
        parts = file.relative_to(vault).parts
        # Even an empty index pass opens the provisioner's writer lock.
        if parts == ("data", "index.lock") or file.name == ".DS_Store":
            continue
        if parts[:2] == (".svalbard", "runtime") and target != "all" and parts[2] != target:
            continue
        bundle.write(file, file.relative_to(stage))
with zipfile.ZipFile(archive) as bundle:
    bad = bundle.testzip()
    if bad:
        raise RuntimeError(f"Archive verification failed for {bad}")
with archive.open("rb") as data:
    digest = hashlib.file_digest(data, "sha256").hexdigest()
archive.replace(out / name)
line = f"{digest}  {name}\n"
(out / (name+".sha256")).write_text(line)
print(line, end="")
PY
