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

    # Move binaries from subdirectories up to dest_dir
    # Archives often nest files inside a versioned folder
    for subdir in "$dest_dir"/*/; do
        [ -d "$subdir" ] || continue
        for f in "$subdir"*; do
            [ -f "$f" ] || continue
            local base="${f##*/}"
            # Skip if already exists at dest level
            [ -f "$dest_dir/$base" ] && continue
            mv "$f" "$dest_dir/$base"
        done
        # Clean up empty subdirectory
        rmdir "$subdir" 2>/dev/null || true
    done

    # Make all files executable
    chmod +x "$dest_dir"/* 2>/dev/null || true
}

find_binary() {
    local name="$1"
    local bin_dir="$DRIVE_ROOT/bin"
    local platform
    platform="$(detect_platform)"

    for dir in "$bin_dir/$platform" "$bin_dir"; do
        # Direct executable match
        if [ -x "$dir/$name" ]; then
            echo "$dir/$name"
            return 0
        fi

        # Look for archives that might contain this binary
        if [ -d "$dir" ]; then
            for archive in "$dir"/*.tar.gz "$dir"/*.tar.xz "$dir"/*.tar.bz2 "$dir"/*.tgz "$dir"/*.zip; do
                [ -f "$archive" ] || continue
                # Extract and retry
                _extract_archive "$archive" "$dir"
                if [ -x "$dir/$name" ]; then
                    echo "$dir/$name"
                    return 0
                fi
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
