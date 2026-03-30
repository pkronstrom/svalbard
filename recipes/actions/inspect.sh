#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"

ui_header "Drive contents"

if [ -f "$DRIVE_ROOT/manifest.yaml" ]; then
    preset="$(grep '^preset:' "$DRIVE_ROOT/manifest.yaml" | cut -d' ' -f2-)"
    region="$(grep '^region:' "$DRIVE_ROOT/manifest.yaml" | cut -d' ' -f2-)"
    created="$(grep '^created:' "$DRIVE_ROOT/manifest.yaml" | cut -d' ' -f2-)"
    echo "  Preset:  $preset"
    echo "  Region:  $region"
    echo "  Created: $created"
    echo ""
fi

for dir in zim maps models data apps books bin; do
    full="$DRIVE_ROOT/$dir"
    [ -d "$full" ] || continue
    count="$(find "$full" -type f 2>/dev/null | wc -l | tr -d ' ')"
    [ "$count" -eq 0 ] && continue
    size="$(du -sh "$full" 2>/dev/null | cut -f1)"
    printf "  %-10s %4s files  %8s\n" "$dir/" "$count" "$size"
done

echo ""

if [ -d "$DRIVE_ROOT/zim" ]; then
    ui_header "ZIM files"
    find "$DRIVE_ROOT/zim" -name "*.zim" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}' | sort -k2
fi

if [ -d "$DRIVE_ROOT/models" ]; then
    ui_header "Models"
    find "$DRIVE_ROOT/models" -name "*.gguf" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}'
fi

if [ -d "$DRIVE_ROOT/data" ]; then
    ui_header "Databases"
    find "$DRIVE_ROOT/data" -name "*.sqlite" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}'
fi

if [ -d "$DRIVE_ROOT/maps" ]; then
    ui_header "Map tiles"
    find "$DRIVE_ROOT/maps" -name "*.pmtiles" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}'
fi
