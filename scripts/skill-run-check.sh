#!/usr/bin/env bash
# Runs the offline check of each skill on a vault: examples/check.sh with the
# drive's tools first on PATH and the network denied. Prints PASS or FAIL per
# skill and exits non-zero when any fails.
#
#   scripts/skill-run-check.sh <vault> [skill|path/to/skill-folder ...]
#
# A skill is named by its folder name (found under <vault>/skills/*/) or given
# as a path to a skill folder. With none named, every skill on the vault runs.
set -uo pipefail

if [[ $# -lt 1 ]]; then
  printf 'usage: %s <vault> [skill|skill-folder ...]\n' "$0" >&2
  exit 2
fi
vault=$(cd "$1" && pwd) || exit 2
shift

case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) platform=macos-arm64 ;;
  Darwin-x86_64) platform=macos-x86_64 ;;
  Linux-aarch64 | Linux-arm64) platform=linux-arm64 ;;
  Linux-x86_64) platform=linux-x86_64 ;;
  *) printf 'unsupported platform %s\n' "$(uname -sm)" >&2; exit 2 ;;
esac

# offline runs a command with the network denied.
offline() {
  case $platform in
    macos-*) sandbox-exec -p '(version 1)(allow default)(deny network*)(allow network* (local unix))' "$@" ;;
    linux-*) unshare -rn "$@" ;;
  esac
}

skills=()
if [[ $# -eq 0 ]]; then
  for dir in "$vault"/skills/*/*/; do
    [[ -f $dir/examples/check.sh ]] && skills+=("${dir%/}")
  done
else
  for name in "$@"; do
    if [[ $name == */* ]]; then
      skills+=("$(cd "$name" && pwd)")
      continue
    fi
    found=""
    for dir in "$vault"/skills/*/"$name"; do
      [[ -d $dir ]] && found=$dir
    done
    if [[ -z $found ]]; then
      printf 'FAIL %s: not on the vault\n' "$name"
      skills+=("")
      continue
    fi
    skills+=("$found")
  done
fi
if [[ ${#skills[@]} -eq 0 ]]; then
  printf 'no skills with examples/check.sh on %s\n' "$vault" >&2
  exit 1
fi

failed=0
for dir in "${skills[@]}"; do
  [[ -n $dir ]] || { failed=1; continue; }
  name=$(basename "$dir")
  check=$dir/examples/check.sh
  if [[ ! -f $check ]]; then
    printf 'FAIL %s: no examples/check.sh\n' "$name"
    failed=1
    continue
  fi
  work=$(mktemp -d "${TMPDIR:-/tmp}/skill-check-$name.XXXXXX")
  log=$work.log
  (
    cd "$work" &&
      DRIVE=$vault DRIVE_ROOT=$vault \
      PATH="$vault/bin/$platform:$vault/.svalbard/runtime/$platform:$PATH" \
      offline sh "$check"
  ) >"$log" 2>&1
  status=$?
  if [[ $status -eq 0 ]]; then
    printf 'PASS %s\n' "$name"
  else
    last=$(grep -v '^[[:space:]]*$' "$log" | tail -n 1)
    printf 'FAIL %s: %s\n' "$name" "${last:-exit $status}"
    failed=1
  fi
  if [[ -n ${SKILL_CHECK_KEEP:-} ]]; then
    printf '  kept %s and %s\n' "$work" "$log"
  else
    case $work in
      */skill-check-"$name".??????) rm -rf "$work" "$log" ;;  # mktemp's own
    esac
  fi
done
exit $failed
