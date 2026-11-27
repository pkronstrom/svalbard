#!/usr/bin/env bash
# Runs the offline check of each skill on a vault: examples/check.sh with the
# drive's tools first on PATH and the network denied. Prints PASS, FAIL or
# SKIP (the check exited 77: nothing to test on this vault) per skill and
# exits non-zero when any fails.
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

# offline runs a command with the network denied. On macOS every socket is
# denied, Unix sockets included (DNS through mDNSResponder, Docker's
# socket), apart from two loopback ports kept for the HTML skills' browser
# checks (Firefox's control port and the check's own page server) and
# binding a local Unix socket (Chromium's singleton); no other service on
# this computer is reachable, and proxy variables are unset. A request handed
# to another process over XPC (`open https://...`) is not stopped. On Linux
# the new network namespace has only its own loopback, brought up for the
# same reason, and Unix sockets on the file system (systemd-resolved,
# docker.sock) stay reachable. Both catch what a skill's own scripts would
# do; neither is a hard boundary.
port() { node -e 'const s=require("net").createServer().listen(0,"127.0.0.1",()=>{console.log(s.address().port);s.close()})' 2>/dev/null || echo 9; }
export HTML_CHECK_FIREFOX_PORT=$(port) HTML_CHECK_SERVE_PORT=$(port)
unset http_proxy HTTP_PROXY https_proxy HTTPS_PROXY all_proxy ALL_PROXY ftp_proxy FTP_PROXY no_proxy NO_PROXY
offline() {
  case $platform in
    macos-*)
      local rules='(version 1)(allow default)(deny network*)(allow network-bind (local unix))(allow network-inbound (local unix))'
      for p in "$HTML_CHECK_FIREFOX_PORT" "$HTML_CHECK_SERVE_PORT"; do
        rules+="(allow network-bind (local ip \"localhost:$p\"))(allow network-inbound (local ip \"localhost:$p\"))(allow network-outbound (remote ip \"localhost:$p\"))"
      done
      sandbox-exec -p "$rules" "$@" ;;
    linux-*) unshare -rn sh -c 'ip link set lo up 2>/dev/null || true; exec "$@"' sh "$@" ;;
  esac
}

# The HTML skills load their pages in headless browsers through
# scripts/html-check.sh, inside the sandbox above.
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export SVALBARD_HTML_CHECK=${SVALBARD_HTML_CHECK:-$here/html-check.sh}

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
  elif [[ $status -eq 77 ]]; then
    # The check had nothing to test on this vault (exit 77, as automake's
    # test harness uses it): reported apart, not as a pass or a failure.
    printf 'SKIP %s: %s\n' "$name" "$(grep -v '^[[:space:]]*$' "$log" | tail -n 1)"
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
