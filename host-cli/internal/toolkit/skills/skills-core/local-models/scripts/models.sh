#!/bin/sh
# Lists the drive's chat models with what each needs, against this
# computer's memory, and recommends one. Reads only; downloads nothing.
#
#   sh models.sh
#
# Rule of thumb: the model file, plus its context memory (about 8% of the
# file per 8K tokens, at least 0.5 GB), plus 1 GB, must stay under 75% of
# the computer's RAM. Fast is 8K tokens of context, Balanced 32K, Max 128K.
set -eu
drive=${DRIVE_ROOT:-${DRIVE:-$(cd "$(dirname "$0")/../../../.." && pwd)}}
case $(uname -s) in
  Darwin) ram=$(sysctl -n hw.memsize) ;;
  Linux) ram=$(awk '/^MemTotal:/ { print $2 * 1024 }' /proc/meminfo) ;;
  *) ram=0 ;;
esac
echo "drive: $drive"
awk -v r="$ram" 'BEGIN { printf "memory: %.0f GB, models may use %.1f GB (75%%)\n", r / 2^30, r * 0.75 / 2^30 }'
if [ -x "$drive/run" ]; then
  "$drive/run" settings 2>/dev/null | sed -n 's/^  \(ai\.[a-z_]*\) *\(.*\)/setting: \1 = \2/p' || true
fi
found=0
list=""
for file in "$drive"/models/*.gguf; do
  [ -e "$file" ] || continue
  name=${file##*/}
  case $name in *mmproj*) continue ;; esac
  size=$(ls -lL "$file" | awk '{ print $5 }')
  list="$list$name $size
"
  found=1
done
if [ $found -eq 0 ]; then
  echo "no chat models in $drive/models"
  exit 0
fi
echo "models (file size, largest context that fits):"
printf '%s' "$list" | sort -k2,2n | awk -v r="$ram" '
  {
    gb = $2 / 2^30
    per8k = gb * 0.08; if (per8k < 0.5) per8k = 0.5
    budget = r * 0.75 / 2^30
    fit = "does not fit"
    if (gb + 1 + per8k * 16 <= budget) fit = "fits Max (128K)"
    else if (gb + 1 + per8k * 4 <= budget) fit = "fits Balanced (32K)"
    else if (gb + 1 + per8k <= budget) fit = "fits Fast (8K)"
    printf "  %s %.1f GB, %s\n", $1, gb, fit
    if (fit != "does not fit") { best = $1; bestfit = fit }
  }
  END {
    if (best != "") printf "recommended: %s (the largest that fits; %s)\n", best, bestfit
    else print "recommended: none (no model fits this computer)"
  }'
