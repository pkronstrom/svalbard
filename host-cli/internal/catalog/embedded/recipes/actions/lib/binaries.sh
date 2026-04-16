#!/usr/bin/env bash
# Shared: binary discovery with on-demand archive extraction
# Requires: platform.sh sourced first

_extract_archive() {
    local archive="$1"
    local dest_dir="$2"
    local name="${archive##*/}"

    echo "  Extracting $name..." >&2
    case "$name" in
        *.tar.gz|*.tgz)  tar xzf "$archive" -C "$dest_dir" ;;
        *.tar.xz)        tar xJf "$archive" -C "$dest_dir" ;;
        *.tar.bz2)       tar xjf "$archive" -C "$dest_dir" ;;
        *.zip)           unzip -qo "$archive" -d "$dest_dir" ;;
        *)               return 1 ;;
    esac

    # Promote all files and symlinks from subdirectories up to dest_dir
    # Archives often nest files inside versioned folders (possibly multi-level)
    while IFS= read -r f; do
        local base="${f##*/}"
        [ -e "$dest_dir/$base" ] && continue
        mv "$f" "$dest_dir/$base"
    done < <(find "$dest_dir" -mindepth 2 \( -type f -o -type l \) 2>/dev/null)
    # Clean up now-empty subdirectories
    find "$dest_dir" -mindepth 1 -type d -empty -delete 2>/dev/null || true

    # Make all files executable
    chmod +x "$dest_dir"/* 2>/dev/null || true
}

find_binary() {
    local name="$1"
    local bin_dir="$DRIVE_ROOT/bin"
    local platform
    platform="$(detect_platform)"

    for dir in "$bin_dir/$platform" "$bin_dir"; do
        if [ -d "$dir" ]; then
            for subdir in "$dir"/*/; do
                [ -d "$subdir" ] || continue
                if [ -x "$subdir/$name" ]; then
                    echo "$subdir/$name"
                    return 0
                fi
                for archive in "$subdir"/*.tar.gz "$subdir"/*.tar.xz "$subdir"/*.tar.bz2 "$subdir"/*.tgz "$subdir"/*.zip; do
                    [ -f "$archive" ] || continue
                    _extract_archive "$archive" "$subdir"
                    if [ -x "$subdir/$name" ]; then
                        echo "$subdir/$name"
                        return 0
                    fi
                done
            done
        fi
    done

    # Fall back to system PATH
    if command -v "$name" >/dev/null 2>&1; then
        command -v "$name"
        return 0
    fi
    return 1
}
