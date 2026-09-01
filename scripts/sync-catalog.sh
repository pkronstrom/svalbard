#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

mirror_tree() {
  local source=$1
  local destination=$2

  if [[ $destination == host-cli/internal/catalog/embedded/* ]]; then
    while IFS= read -r path; do
      rm -f "$path"
    done < <(git ls-files --cached --others --exclude-standard "$destination")
  else
    rm -rf "$destination"
  fi
  mkdir -p "$destination"
  while IFS= read -r path; do
    local relative=${path#"$source/"}
    case "$source/$relative" in
      recipes/builders/test_web_video_zim.py) continue ;;
    esac
    mkdir -p "$destination/$(dirname "$relative")"
    cp -p "$path" "$destination/$relative"
  done < <(git ls-files --cached --others --exclude-standard "$source")
}

check_catalog() {
  local work
  work=$(mktemp -d)
  trap 'rm -rf "$work"' RETURN

  mirror_tree recipes "$work/recipes"
  mirror_tree presets "$work/presets"

  if ! diff -qr --exclude='__pycache__' --exclude='*.local.yaml' \
       "$work/recipes" host-cli/internal/catalog/embedded/recipes ||
     ! diff -qr --exclude='__pycache__' --exclude='*.local.yaml' \
       "$work/presets" host-cli/internal/catalog/embedded/presets; then
    printf '%s\n' "embedded catalog is stale; run scripts/sync-catalog.sh" >&2
    return 1
  fi
}

case "${1:-sync}" in
  --check)
    check_catalog
    ;;
  sync)
    mirror_tree recipes host-cli/internal/catalog/embedded/recipes
    mirror_tree presets host-cli/internal/catalog/embedded/presets
    ;;
  *)
    printf 'usage: %s [sync|--check]\n' "$0" >&2
    exit 2
    ;;
esac
