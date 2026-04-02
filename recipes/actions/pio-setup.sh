#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"

PLATFORM="$(detect_platform)"
PIO_CACHE="${SVALBARD_PIO_CACHE:-/tmp/svalbard-pio}"
PKG_DIR="$PIO_CACHE/packages"
DRIVE_PKG="$DRIVE_ROOT/tools/platformio/packages"
DRIVE_LIB="$DRIVE_ROOT/tools/platformio/lib"

mkdir -p "$PKG_DIR"

# Extract an archive preserving internal structure (no flattening)
_extract_toolchain() {
    local archive="$1"
    local name="$2"
    local dest="$PKG_DIR/$name"

    if [ -d "$dest" ] && [ -f "$dest/.extracted" ]; then
        return 0
    fi

    echo "  Extracting $name..."
    rm -rf "$dest"
    mkdir -p "$dest"

    case "$archive" in
        *.tar.gz|*.tgz)  tar xzf "$archive" -C "$dest" --strip-components=1 ;;
        *.tar.xz)        tar xJf "$archive" -C "$dest" --strip-components=1 ;;
        *.zip)           unzip -qo "$archive" -d "$dest" ;;
    esac

    touch "$dest/.extracted"
}

# Derive package name from archive filename
# e.g. "toolchain-xtensa-esp-elf-linux_x86_64-14.2.0+20251107.tar.gz" -> "toolchain-xtensa-esp-elf"
_pkg_name() {
    local filename="${1##*/}"
    # Remove platform suffix and version: strip from first occurrence of -linux_ -darwin_ -windows_ or a digit preceded by dash
    echo "$filename" | sed -E 's/-(linux|darwin|windows)_.*//' | sed -E 's/-[0-9]+\..*//'
}

# Extract platform-specific packages
if [ -d "$DRIVE_PKG/$PLATFORM" ]; then
    for archive in "$DRIVE_PKG/$PLATFORM"/*.tar.gz "$DRIVE_PKG/$PLATFORM"/*.tar.xz "$DRIVE_PKG/$PLATFORM"/*.zip; do
        [ -f "$archive" ] || continue
        pkg_name="$(_pkg_name "$archive")"
        _extract_toolchain "$archive" "$pkg_name"
    done
fi

# Extract shared (cross-platform) packages — archives directly in packages/
for archive in "$DRIVE_PKG"/*.tar.gz "$DRIVE_PKG"/*.tar.xz "$DRIVE_PKG"/*.zip; do
    [ -f "$archive" ] || continue
    pkg_name="$(_pkg_name "$archive")"
    _extract_toolchain "$archive" "$pkg_name"
done

# Link global libraries from stick if available
if [ -d "$DRIVE_LIB" ] && [ ! -L "$PIO_CACHE/lib" ]; then
    ln -sfn "$DRIVE_LIB" "$PIO_CACHE/lib"
fi

export PLATFORMIO_CORE_DIR="$PIO_CACHE"
export PLATFORMIO_BUILD_DIR="/tmp/svalbard-pio-build"

echo ""
echo "Embedded dev shell ready."
echo "  Toolchains: $PKG_DIR"
if [ -d "$DRIVE_LIB" ]; then
    echo "  Libraries:  $DRIVE_LIB"
fi
echo "  Build dir:  $PLATFORMIO_BUILD_DIR"
echo ""
echo "  pio init --board esp32dev --project-option 'framework=espidf'"
echo "  pio run"
echo "  pio run -t upload"
echo "  pio device monitor"
echo ""

# Drop into a subshell with the configured environment
exec "$SHELL"
