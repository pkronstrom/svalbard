# run.sh Toolkit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the monolithic `serve.sh` with a modular `run.sh` toolkit — a thin launcher that reads `entries.tab` and dispatches to self-contained action scripts, all assembled at build time.

**Architecture:** Action scripts live in `recipes/actions/` and shared helpers in `recipes/actions/lib/`. A new `toolkit_generator.py` copies them to `.svalbard/` on the drive and generates `entries.tab` based on actual drive content. `run.sh` is a ~40-line launcher that parses the tab file and shows a menu. New tool recipes (toybox, dufs, fzf, 7z, zstd) follow the existing `kiwix-serve.yaml` pattern.

**Tech Stack:** Bash (action scripts, run.sh), Python (generator), YAML (recipes)

**Design doc:** `docs/plans/2026-03-30-run-sh-toolkit-design.md`

---

### Task 1: Create shared lib scripts

The foundation — helper functions used by all action scripts.

**Files:**
- Create: `recipes/actions/lib/platform.sh`
- Create: `recipes/actions/lib/binaries.sh`
- Create: `recipes/actions/lib/ports.sh`
- Create: `recipes/actions/lib/ui.sh`
- Create: `recipes/actions/lib/process.sh`

**Step 1: Write `recipes/actions/lib/platform.sh`**

Extracted from the existing `serve_generator.py` SERVE_SH template.

```bash
#!/usr/bin/env bash
# Shared: platform detection

detect_platform() {
    local os arch
    case "$(uname -s)" in
        Darwin*) os="macos" ;;
        Linux*)  os="linux" ;;
        *)       os="unknown" ;;
    esac
    case "$(uname -m)" in
        x86_64)       arch="x86_64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)            arch="unknown" ;;
    esac
    echo "${os}-${arch}"
}
```

**Step 2: Write `recipes/actions/lib/binaries.sh`**

```bash
#!/usr/bin/env bash
# Shared: binary discovery

find_binary() {
    local name="$1"
    local bin_dir="$DRIVE_ROOT/bin"
    local platform
    platform="$(detect_platform)"
    # Platform-specific dir first, then generic bin/, then PATH
    for dir in "$bin_dir/$platform" "$bin_dir"; do
        if [ -x "$dir/$name" ]; then
            echo "$dir/$name"
            return 0
        fi
    done
    if command -v "$name" >/dev/null 2>&1; then
        command -v "$name"
        return 0
    fi
    return 1
}
```

**Step 3: Write `recipes/actions/lib/ports.sh`**

```bash
#!/usr/bin/env bash
# Shared: port utilities

find_free_port() {
    local start="${1:-8080}"
    local port="$start"
    while [ "$port" -lt "$((start + 100))" ]; do
        if ! (echo >/dev/tcp/localhost/"$port") 2>/dev/null; then
            echo "$port"
            return 0
        fi
        port=$((port + 1))
    done
    echo "$start"
}
```

**Step 4: Write `recipes/actions/lib/ui.sh`**

```bash
#!/usr/bin/env bash
# Shared: terminal UI helpers

# Colors (auto-disabled if not a terminal)
if [ -t 1 ]; then
    BOLD=$'\033[1m'
    DIM=$'\033[2m'
    RED=$'\033[0;31m'
    GREEN=$'\033[0;32m'
    YELLOW=$'\033[0;33m'
    CYAN=$'\033[0;36m'
    NC=$'\033[0m'
else
    BOLD="" DIM="" RED="" GREEN="" YELLOW="" CYAN="" NC=""
fi

# Print a section header
ui_header() {
    echo ""
    echo "${BOLD}$1${NC}"
    echo "─────────────────────────────────────────"
}

# Print a status line
ui_status() {
    echo "  ${GREEN}$1${NC}"
}

# Print an error
ui_error() {
    echo "  ${RED}$1${NC}" >&2
}

# Open URL in default browser
open_browser() {
    local url="$1"
    case "$(uname -s)" in
        Darwin*) open "$url" ;;
        Linux*)  xdg-open "$url" 2>/dev/null || echo "  Open: $url" ;;
    esac
}
```

**Step 5: Write `recipes/actions/lib/process.sh`**

```bash
#!/usr/bin/env bash
# Shared: process management

SVALBARD_PIDS=()

cleanup_processes() {
    echo ""
    echo "Shutting down..."
    for pid in "${SVALBARD_PIDS[@]}"; do
        kill "$pid" 2>/dev/null && echo "  Stopped PID $pid" || true
    done
}

trap_cleanup() {
    trap cleanup_processes SIGINT SIGTERM
}

wait_for_services() {
    if [ ${#SVALBARD_PIDS[@]} -gt 0 ]; then
        echo ""
        echo "Services running. Press Ctrl+C to stop all."
        wait
    fi
}
```

**Step 6: Commit**

```bash
git add recipes/actions/lib/
git commit -m "feat: add shared lib scripts for run.sh toolkit"
```

---

### Task 2: Create action scripts

The individual actions that entries.tab dispatches to.

**Files:**
- Create: `recipes/actions/browse.sh`
- Create: `recipes/actions/maps.sh`
- Create: `recipes/actions/chat.sh`
- Create: `recipes/actions/apps.sh`
- Create: `recipes/actions/serve-all.sh`
- Create: `recipes/actions/share.sh`
- Create: `recipes/actions/inspect.sh`
- Create: `recipes/actions/verify.sh`

**Step 1: Write `recipes/actions/browse.sh`**

This script receives an optional argument. With no args it shows the baked-in
submenu (from entries.tab lines with `browse.sh` as script). With a specific
ZIM path it launches kiwix-serve for just that file.

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

KIWIX_BIN="$(find_binary kiwix-serve 2>/dev/null || true)"
if [ -z "$KIWIX_BIN" ]; then
    ui_error "kiwix-serve not found. Run 'Provision tools' from the menu."
    exit 1
fi

# Collect ZIM files
zim_files=()
if [ -d "$DRIVE_ROOT/zim" ]; then
    while IFS= read -r f; do
        [ -n "$f" ] && zim_files+=("$f")
    done < <(find "$DRIVE_ROOT/zim" -name "*.zim" -type f 2>/dev/null | sort)
fi

if [ ${#zim_files[@]} -eq 0 ]; then
    ui_error "No ZIM files found in zim/"
    exit 1
fi

# If a specific ZIM path was passed as $1, serve just that
if [ -n "${1:-}" ] && [ -f "$DRIVE_ROOT/zim/$1" ]; then
    trap_cleanup
    port="$(find_free_port 8080)"
    echo "Starting kiwix-serve on port $port..."
    "$KIWIX_BIN" --port "$port" "$DRIVE_ROOT/zim/$1" &
    SVALBARD_PIDS+=($!)
    sleep 1
    open_browser "http://localhost:$port"
    ui_status "Kiwix: http://localhost:$port"
    wait_for_services
    exit 0
fi

# Default: serve all ZIM files
trap_cleanup
port="$(find_free_port 8080)"
echo "Starting kiwix-serve on port $port with ${#zim_files[@]} ZIM files..."
"$KIWIX_BIN" --port "$port" "${zim_files[@]}" &
SVALBARD_PIDS+=($!)
sleep 1
open_browser "http://localhost:$port"
ui_status "Kiwix: http://localhost:$port"
wait_for_services
```

**Step 2: Write `recipes/actions/maps.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

PMTILES_BIN="$(find_binary go-pmtiles 2>/dev/null || true)"
if [ -z "$PMTILES_BIN" ]; then
    ui_error "go-pmtiles not found."
    exit 1
fi

trap_cleanup

tile_port="$(find_free_port 8081)"
echo "Starting tile server on port $tile_port..."
"$PMTILES_BIN" serve "$DRIVE_ROOT/maps" --port "$tile_port" &
SVALBARD_PIDS+=($!)

# Serve the map viewer app via python or dufs
if [ -f "$DRIVE_ROOT/apps/map/index.html" ]; then
    app_port="$(find_free_port 8083)"
    DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
    if [ -n "$DUFS_BIN" ]; then
        "$DUFS_BIN" --bind "127.0.0.1" --port "$app_port" "$DRIVE_ROOT" &
    elif command -v python3 >/dev/null 2>&1; then
        python3 -m http.server "$app_port" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
    else
        ui_error "No file server available for map viewer."
        wait_for_services
        exit 1
    fi
    SVALBARD_PIDS+=($!)
    sleep 1
    open_browser "http://localhost:$app_port/apps/map/"
    ui_status "Map viewer: http://localhost:$app_port/apps/map/"
else
    ui_status "Tile server: http://localhost:$tile_port"
fi

ui_status "Tiles: http://localhost:$tile_port"
wait_for_services
```

**Step 3: Write `recipes/actions/chat.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

LLAMA_BIN="$(find_binary llama-server 2>/dev/null || true)"
if [ -z "$LLAMA_BIN" ]; then
    ui_error "llama-server not found."
    exit 1
fi

# If a specific model was passed, use it; otherwise pick the first .gguf
model="${1:-}"
if [ -z "$model" ]; then
    model="$(find "$DRIVE_ROOT/models" -name "*.gguf" -type f 2>/dev/null | head -1)"
fi
if [ -z "$model" ] || [ ! -f "$model" ]; then
    ui_error "No GGUF model found in models/"
    exit 1
fi

trap_cleanup
port="$(find_free_port 8082)"
echo "Starting llama-server on port $port with $(basename "$model")..."
"$LLAMA_BIN" -m "$model" --port "$port" &
SVALBARD_PIDS+=($!)
sleep 2
open_browser "http://localhost:$port"
ui_status "LLM: http://localhost:$port"
wait_for_services
```

**Step 4: Write `recipes/actions/apps.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

app_name="${1:?Usage: apps.sh <app-name>}"
app_dir="$DRIVE_ROOT/apps/$app_name"

if [ ! -d "$app_dir" ]; then
    ui_error "App not found: $app_dir"
    exit 1
fi

trap_cleanup
port="$(find_free_port 8083)"

DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
if [ -n "$DUFS_BIN" ]; then
    "$DUFS_BIN" --bind "127.0.0.1" --port "$port" "$DRIVE_ROOT" &
elif command -v python3 >/dev/null 2>&1; then
    python3 -m http.server "$port" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
else
    ui_error "No file server available (need dufs or python3)."
    exit 1
fi
SVALBARD_PIDS+=($!)
sleep 1
open_browser "http://localhost:$port/apps/$app_name/"
ui_status "$app_name: http://localhost:$port/apps/$app_name/"
wait_for_services
```

**Step 5: Write `recipes/actions/serve-all.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

BIND="${1:-127.0.0.1}"
trap_cleanup

ui_header "Starting all services"

# Kiwix
KIWIX_BIN="$(find_binary kiwix-serve 2>/dev/null || true)"
zim_files=()
if [ -d "$DRIVE_ROOT/zim" ]; then
    while IFS= read -r f; do
        [ -n "$f" ] && zim_files+=("$f")
    done < <(find "$DRIVE_ROOT/zim" -name "*.zim" -type f 2>/dev/null)
fi
if [ -n "$KIWIX_BIN" ] && [ ${#zim_files[@]} -gt 0 ]; then
    port="$(find_free_port 8080)"
    "$KIWIX_BIN" --port "$port" --address "$BIND" "${zim_files[@]}" &
    SVALBARD_PIDS+=($!)
    ui_status "Kiwix:  http://$BIND:$port"
fi

# PMTiles
PMTILES_BIN="$(find_binary go-pmtiles 2>/dev/null || true)"
if [ -n "$PMTILES_BIN" ] && [ -d "$DRIVE_ROOT/maps" ]; then
    port="$(find_free_port 8081)"
    "$PMTILES_BIN" serve "$DRIVE_ROOT/maps" --port "$port" &
    SVALBARD_PIDS+=($!)
    ui_status "Maps:   http://$BIND:$port"
fi

# LLM
LLAMA_BIN="$(find_binary llama-server 2>/dev/null || true)"
model="$(find "$DRIVE_ROOT/models" -name "*.gguf" -type f 2>/dev/null | head -1 || true)"
if [ -n "$LLAMA_BIN" ] && [ -n "$model" ]; then
    port="$(find_free_port 8082)"
    "$LLAMA_BIN" -m "$model" --port "$port" --host "$BIND" &
    SVALBARD_PIDS+=($!)
    ui_status "LLM:    http://$BIND:$port"
fi

# Apps / static file server
DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
if [ -n "$DUFS_BIN" ]; then
    port="$(find_free_port 8083)"
    "$DUFS_BIN" --bind "$BIND" --port "$port" "$DRIVE_ROOT" &
    SVALBARD_PIDS+=($!)
    ui_status "Files:  http://$BIND:$port"
    [ -d "$DRIVE_ROOT/apps/map" ] && ui_status "Map:    http://$BIND:$port/apps/map/"
elif command -v python3 >/dev/null 2>&1; then
    port="$(find_free_port 8083)"
    python3 -m http.server "$port" --bind "$BIND" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
    SVALBARD_PIDS+=($!)
    ui_status "Files:  http://$BIND:$port"
fi

wait_for_services
```

**Step 6: Write `recipes/actions/share.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

# Detect LAN IP
lan_ip() {
    case "$(uname -s)" in
        Darwin*) ipconfig getifaddr en0 2>/dev/null || echo "0.0.0.0" ;;
        Linux*)  hostname -I 2>/dev/null | awk '{print $1}' || echo "0.0.0.0" ;;
    esac
}

IP="$(lan_ip)"

ui_header "Sharing drive on local network"

# Serve files with dufs (preferred) or python3 fallback
DUFS_BIN="$(find_binary dufs 2>/dev/null || true)"
trap_cleanup
port="$(find_free_port 8080)"

if [ -n "$DUFS_BIN" ]; then
    "$DUFS_BIN" --bind "0.0.0.0" --port "$port" "$DRIVE_ROOT" &
else
    python3 -m http.server "$port" --bind "0.0.0.0" --directory "$DRIVE_ROOT" >/dev/null 2>&1 &
fi
SVALBARD_PIDS+=($!)

echo ""
echo "  ${BOLD}http://${IP}:${port}${NC}"
echo ""
echo "  Tell others to open this address in their browser."

wait_for_services
```

**Step 7: Write `recipes/actions/inspect.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"

ui_header "Drive contents"

# Read from manifest
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
    # Get size (du -sh)
    size="$(du -sh "$full" 2>/dev/null | cut -f1)"
    printf "  %-10s %4s files  %8s\n" "$dir/" "$count" "$size"
done

echo ""

# List ZIM files with sizes
if [ -d "$DRIVE_ROOT/zim" ]; then
    ui_header "ZIM files"
    find "$DRIVE_ROOT/zim" -name "*.zim" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}' | sort -k2
fi

# List models
if [ -d "$DRIVE_ROOT/models" ]; then
    ui_header "Models"
    find "$DRIVE_ROOT/models" -name "*.gguf" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}'
fi

# List databases
if [ -d "$DRIVE_ROOT/data" ]; then
    ui_header "Databases"
    find "$DRIVE_ROOT/data" -name "*.sqlite" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}'
fi

# List maps
if [ -d "$DRIVE_ROOT/maps" ]; then
    ui_header "Map tiles"
    find "$DRIVE_ROOT/maps" -name "*.pmtiles" -type f -exec ls -lh {} \; 2>/dev/null | \
        awk '{printf "  %-8s %s\n", $5, $NF}'
fi
```

**Step 8: Write `recipes/actions/verify.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"

ui_header "Verifying drive integrity"

# Find sha256sum (toybox, coreutils, or macOS shasum)
SHA_CMD=""
SHA_BIN="$(find_binary sha256sum 2>/dev/null || true)"
if [ -n "$SHA_BIN" ]; then
    SHA_CMD="$SHA_BIN"
elif command -v sha256sum >/dev/null 2>&1; then
    SHA_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
    SHA_CMD="shasum -a 256"
else
    ui_error "No SHA-256 tool found."
    exit 1
fi

if [ ! -f "$DRIVE_ROOT/.svalbard/checksums.sha256" ]; then
    ui_error "No checksum file found. Verification not available for this drive."
    exit 1
fi

passed=0
failed=0
missing=0

while IFS='  ' read -r expected_hash filepath; do
    [ -z "$expected_hash" ] && continue
    [[ "$expected_hash" == \#* ]] && continue
    full_path="$DRIVE_ROOT/$filepath"
    if [ ! -f "$full_path" ]; then
        echo "  ${YELLOW}MISSING${NC}  $filepath"
        missing=$((missing + 1))
        continue
    fi
    actual_hash="$($SHA_CMD "$full_path" | awk '{print $1}')"
    if [ "$actual_hash" = "$expected_hash" ]; then
        echo "  ${GREEN}OK${NC}       $filepath"
        passed=$((passed + 1))
    else
        echo "  ${RED}FAIL${NC}     $filepath"
        failed=$((failed + 1))
    fi
done < "$DRIVE_ROOT/.svalbard/checksums.sha256"

echo ""
echo "  Passed: $passed  Failed: $failed  Missing: $missing"
```

**Step 9: Commit**

```bash
git add recipes/actions/
git commit -m "feat: add modular action scripts for run.sh toolkit"
```

---

### Task 3: Write the toolkit generator

Python module that assembles `.svalbard/` on the drive at build time.

**Files:**
- Create: `src/svalbard/toolkit_generator.py`
- Test: `tests/test_toolkit_generator.py`

**Step 1: Write failing test**

```python
# tests/test_toolkit_generator.py
from pathlib import Path
from svalbard.toolkit_generator import generate_toolkit


def test_generate_toolkit_creates_run_sh(tmp_path):
    """run.sh should be created at the drive root."""
    # Minimal manifest and preset setup
    manifest_data = {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim", "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": ["general-reference"], "depth": "comprehensive"},
        ],
    }
    _write_manifest(tmp_path, manifest_data)
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    assert (tmp_path / "run.sh").exists()
    assert (tmp_path / ".svalbard" / "entries.tab").exists()
    assert (tmp_path / ".svalbard" / "actions" / "browse.sh").exists()
    assert (tmp_path / ".svalbard" / "lib" / "ui.sh").exists()


def test_entries_tab_omits_sections_without_content(tmp_path):
    """Sections for missing content should not appear."""
    manifest_data = {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim", "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    }
    _write_manifest(tmp_path, manifest_data)
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[browse]" in tab_content
    assert "[ai]" not in tab_content  # no models on drive
    assert "[maps]" not in tab_content  # no pmtiles


def test_entries_tab_includes_maps_when_present(tmp_path):
    """Maps section should appear when pmtiles exist."""
    manifest_data = {
        "preset": "finland-128",
        "region": "finland",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "osm-finland", "type": "pmtiles", "filename": "osm-finland.pmtiles",
             "size_bytes": 500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    }
    _write_manifest(tmp_path, manifest_data)
    (tmp_path / "maps").mkdir()
    (tmp_path / "maps" / "osm-finland.pmtiles").touch()

    generate_toolkit(tmp_path, "finland-128")

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[maps]" in tab_content


def _write_manifest(drive_path, data):
    import yaml
    (drive_path / "manifest.yaml").write_text(yaml.dump(data))
```

**Step 2: Run tests to verify they fail**

Run: `uv run pytest tests/test_toolkit_generator.py -v`
Expected: FAIL — `toolkit_generator` module doesn't exist

**Step 3: Write `src/svalbard/toolkit_generator.py`**

```python
"""Generate the run.sh toolkit on the drive.

Copies action scripts and lib helpers from recipes/actions/ to .svalbard/
on the drive, and generates entries.tab based on actual drive content.
"""

import os
import shutil
import stat
from pathlib import Path

from svalbard.manifest import Manifest
from svalbard.presets import load_preset

_PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
ACTIONS_DIR = _PROJECT_ROOT / "recipes" / "actions"
LIB_DIR = ACTIONS_DIR / "lib"


# ── entries.tab generation ──────────────────────────────────────────────────

def _count_files(directory: Path, pattern: str) -> int:
    if not directory.exists():
        return 0
    return len(list(directory.glob(pattern)))


def _human_size(size_bytes: int) -> str:
    if size_bytes >= 1e9:
        return f"{size_bytes / 1e9:.1f} GB"
    if size_bytes >= 1e6:
        return f"{size_bytes / 1e6:.0f} MB"
    return f"{size_bytes / 1e3:.0f} KB"


def _build_entries(drive_path: Path, manifest: Manifest, preset_name: str) -> str:
    """Build entries.tab content based on what's on the drive."""
    lines = [f"# Svalbard · {preset_name} — run.sh menu"]
    lines.append("# Format: label\\tscript\\targs")
    lines.append("")

    preset = load_preset(preset_name)

    # ── Browse ──────────────────────────────────────────────────────────
    zim_count = _count_files(drive_path / "zim", "*.zim")
    if zim_count > 0:
        lines.append("[browse]")
        lines.append(f"Browse encyclopedias — {zim_count} ZIM files\t.svalbard/actions/browse.sh")

        # Individual ZIM entries grouped by type
        zim_entries = [e for e in manifest.entries if e.type == "zim"]
        for entry in sorted(zim_entries, key=lambda e: e.id):
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            size = _human_size(entry.size_bytes)
            lines.append(f"  {desc}  {size}\t.svalbard/actions/browse.sh\t{entry.filename}")
        lines.append("")

    # ── Maps ────────────────────────────────────────────────────────────
    pmtiles_count = _count_files(drive_path / "maps", "*.pmtiles")
    if pmtiles_count > 0:
        lines.append("[maps]")
        lines.append(f"View maps — {pmtiles_count} tile layers\t.svalbard/actions/maps.sh")
        lines.append("")

    # ── AI ──────────────────────────────────────────────────────────────
    gguf_entries = [e for e in manifest.entries if e.type == "gguf"]
    if gguf_entries:
        lines.append("[ai]")
        for entry in gguf_entries:
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            lines.append(f"Chat with {desc}\t.svalbard/actions/chat.sh\t{drive_path / 'models' / entry.filename}")
        lines.append("")

    # ── Apps ────────────────────────────────────────────────────────────
    apps_dir = drive_path / "apps"
    app_sources = [s for s in preset.sources if s.type == "app"]
    if apps_dir.exists() and app_sources:
        lines.append("[apps]")
        for source in app_sources:
            app_dir = apps_dir / source.id
            if app_dir.exists():
                lines.append(f"Open {source.description or source.id}\t.svalbard/actions/apps.sh\t{source.id}")
        lines.append("")

    # ── Data ────────────────────────────────────────────────────────────
    db_count = _count_files(drive_path / "data", "*.sqlite")
    if db_count > 0:
        lines.append("[data]")
        db_entries = [e for e in manifest.entries if e.type == "sqlite"]
        for entry in sorted(db_entries, key=lambda e: e.id):
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            lines.append(f"Query {desc}\t.svalbard/actions/apps.sh\tsqliteviz")
        lines.append("")

    # ── Serve ───────────────────────────────────────────────────────────
    has_services = zim_count > 0 or pmtiles_count > 0 or bool(gguf_entries)
    if has_services:
        lines.append("[serve]")
        lines.append("Serve everything\t.svalbard/actions/serve-all.sh")
        lines.append("Share on local network\t.svalbard/actions/share.sh")
        lines.append("")

    # ── Info ─────────────────────────────────────────────────────────────
    lines.append("[info]")
    lines.append("List drive contents\t.svalbard/actions/inspect.sh")
    lines.append("Verify checksums\t.svalbard/actions/verify.sh")
    lines.append("")

    return "\n".join(lines)


# ── run.sh template ─────────────────────────────────────────────────────────

RUN_SH = r'''#!/usr/bin/env bash
set -euo pipefail

DRIVE_ROOT="$(cd "$(dirname "$0")" && pwd)"
export DRIVE_ROOT

ENTRIES_FILE="$DRIVE_ROOT/.svalbard/entries.tab"
if [ ! -f "$ENTRIES_FILE" ]; then
    echo "Error: entries.tab not found. Is this a Svalbard drive?"
    exit 1
fi

source "$DRIVE_ROOT/.svalbard/lib/ui.sh"

# Parse entries into arrays
declare -a LABELS SCRIPTS ARGS GROUPS
current_group=""
while IFS= read -r line || [ -n "$line" ]; do
    [[ -z "$line" || "$line" == \#* ]] && continue
    if [[ "$line" =~ ^\[(.+)\]$ ]]; then
        current_group="${BASH_REMATCH[1]}"
        continue
    fi
    # Skip indented lines (submenu items for browse)
    [[ "$line" == \ * ]] && continue
    IFS=$'\t' read -r label script args <<< "$line"
    LABELS+=("$label")
    SCRIPTS+=("$script")
    ARGS+=("${args:-}")
    GROUPS+=("$current_group")
done < "$ENTRIES_FILE"

# Try fzf for menu, fall back to numbered list
show_menu() {
    local FZF_BIN
    FZF_BIN="$(source "$DRIVE_ROOT/.svalbard/lib/platform.sh" && source "$DRIVE_ROOT/.svalbard/lib/binaries.sh" && find_binary fzf 2>/dev/null || true)"

    if [ -n "$FZF_BIN" ]; then
        local choice
        choice="$(printf '%s\n' "${LABELS[@]}" | "$FZF_BIN" --prompt="Svalbard> " --height=~20 --reverse)" || return 1
        for i in "${!LABELS[@]}"; do
            if [ "${LABELS[$i]}" = "$choice" ]; then
                echo "$i"
                return 0
            fi
        done
        return 1
    fi

    # Fallback: numbered menu
    echo ""
    echo "${BOLD}Svalbard${NC}"
    echo "─────────────────────────────────────────"
    local prev_group=""
    for i in "${!LABELS[@]}"; do
        if [ "${GROUPS[$i]}" != "$prev_group" ]; then
            echo ""
            prev_group="${GROUPS[$i]}"
        fi
        printf "  ${CYAN}%2d${NC}) %s\n" "$((i + 1))" "${LABELS[$i]}"
    done
    echo ""
    printf "  ${DIM} q) Quit${NC}\n"
    echo ""

    local choice
    read -rp "  > " choice
    case "$choice" in
        q|Q) return 1 ;;
        *[!0-9]*) return 1 ;;
    esac
    if [ "$choice" -ge 1 ] 2>/dev/null && [ "$choice" -le "${#LABELS[@]}" ]; then
        echo "$((choice - 1))"
        return 0
    fi
    return 1
}

# Main loop
while true; do
    idx="$(show_menu)" || exit 0
    script="${SCRIPTS[$idx]}"
    args="${ARGS[$idx]}"

    if [ ! -f "$DRIVE_ROOT/$script" ]; then
        echo "${RED}Script not found: $script${NC}"
        read -rp "Press Enter to continue..."
        continue
    fi

    chmod +x "$DRIVE_ROOT/$script" 2>/dev/null || true
    if [ -n "$args" ]; then
        "$DRIVE_ROOT/$script" "$args"
    else
        "$DRIVE_ROOT/$script"
    fi

    echo ""
    read -rp "Press Enter to return to menu..."
done
'''


# ── Public API ──────────────────────────────────────────────────────────────

def generate_toolkit(drive_path: Path, preset_name: str) -> Path:
    """Assemble the full .svalbard/ toolkit on the drive.

    1. Copy action scripts to .svalbard/actions/
    2. Copy lib helpers to .svalbard/lib/
    3. Generate entries.tab
    4. Write run.sh
    """
    svalbard_dir = drive_path / ".svalbard"
    actions_dest = svalbard_dir / "actions"
    lib_dest = svalbard_dir / "lib"

    # Clean and recreate
    if svalbard_dir.exists():
        shutil.rmtree(svalbard_dir)
    actions_dest.mkdir(parents=True)
    lib_dest.mkdir(parents=True)

    # Copy action scripts
    if ACTIONS_DIR.exists():
        for script in ACTIONS_DIR.glob("*.sh"):
            dest = actions_dest / script.name
            shutil.copy2(script, dest)
            dest.chmod(dest.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

    # Copy lib scripts
    if LIB_DIR.exists():
        for script in LIB_DIR.glob("*.sh"):
            dest = lib_dest / script.name
            shutil.copy2(script, dest)

    # Generate entries.tab
    manifest = Manifest.load(drive_path / "manifest.yaml")
    tab_content = _build_entries(drive_path, manifest, preset_name)
    (svalbard_dir / "entries.tab").write_text(tab_content)

    # Write run.sh
    run_sh = drive_path / "run.sh"
    run_sh.write_text(RUN_SH)
    run_sh.chmod(run_sh.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

    return run_sh
```

**Step 4: Run tests to verify they pass**

Run: `uv run pytest tests/test_toolkit_generator.py -v`
Expected: All 3 PASS

**Step 5: Commit**

```bash
git add src/svalbard/toolkit_generator.py tests/test_toolkit_generator.py
git commit -m "feat: add toolkit_generator for building run.sh on drives"
```

---

### Task 4: Wire generator into sync and init

Replace `serve_generator` with `toolkit_generator` in the build pipeline.

**Files:**
- Modify: `src/svalbard/commands.py:22` — change import
- Modify: `src/svalbard/commands.py:231` — change `_init_drive` call
- Modify: `src/svalbard/commands.py:529-533` — change post-sync regeneration

**Step 1: Write failing test**

```python
# Add to tests/test_commands.py (or create new test file)
def test_init_creates_run_sh_not_serve_sh(tmp_path):
    """init should create run.sh, not serve.sh."""
    # This test depends on existing test infrastructure in test_commands.py
    # for setting up a preset and running init_drive.
    from svalbard.commands import init_drive
    init_drive(str(tmp_path), "default-32")
    assert (tmp_path / "run.sh").exists()
    assert not (tmp_path / "serve.sh").exists()
```

**Step 2: Run test to verify it fails**

Run: `uv run pytest tests/test_commands.py::test_init_creates_run_sh_not_serve_sh -v`
Expected: FAIL — `serve.sh` still exists, `run.sh` doesn't

**Step 3: Update `src/svalbard/commands.py`**

Replace the import at line 23:
```python
# Old:
from svalbard.serve_generator import generate_serve_sh
# New:
from svalbard.toolkit_generator import generate_toolkit
```

In `_init_drive` (around line 231), replace:
```python
# Old:
generate_serve_sh(drive_path)
# New:
generate_toolkit(drive_path, preset_name)
```

In post-sync regeneration (around line 529-533), replace:
```python
# Old:
generate_serve_sh(drive_path)
# New:
generate_toolkit(drive_path, manifest.preset)
```

**Step 4: Run tests**

Run: `uv run pytest tests/ -v`
Expected: All PASS (existing tests still work, new test passes)

**Step 5: Commit**

```bash
git add src/svalbard/commands.py tests/test_commands.py
git commit -m "feat: wire toolkit_generator into sync and init, replacing serve_generator"
```

---

### Task 5: Add tool recipes for bundled binaries

New recipe YAML files for toybox, dufs, fzf, 7z, zstd.

**Files:**
- Create: `recipes/tools/toybox.yaml`
- Create: `recipes/tools/dufs.yaml`
- Create: `recipes/tools/fzf.yaml`
- Create: `recipes/tools/7z.yaml`
- Create: `recipes/tools/zstd.yaml`

**Step 1: Write `recipes/tools/toybox.yaml`**

```yaml
id: toybox
type: binary
group: tools
tags:
- computing
depth: reference-only
size_gb: 0.003
platforms:
  linux-x86_64: https://landley.net/toybox/bin/toybox-x86_64
  linux-arm64: https://landley.net/toybox/bin/toybox-aarch64
description: Toybox — 200+ POSIX utilities in a single binary (httpd, sha256sum, grep, sed, find, wget, tar, nc)
license:
  id: 0BSD
  attribution: Rob Landley
```

**Step 2: Write `recipes/tools/dufs.yaml`**

```yaml
id: dufs
type: binary
group: tools
tags:
- computing
depth: reference-only
size_gb: 0.003
platforms:
  linux-x86_64: https://github.com/sigoden/dufs/releases/latest/download/dufs-v0.43.0-x86_64-unknown-linux-musl.tar.gz
  linux-arm64: https://github.com/sigoden/dufs/releases/latest/download/dufs-v0.43.0-aarch64-unknown-linux-musl.tar.gz
  macos-arm64: https://github.com/sigoden/dufs/releases/latest/download/dufs-v0.43.0-aarch64-apple-darwin.tar.gz
  macos-x86_64: https://github.com/sigoden/dufs/releases/latest/download/dufs-v0.43.0-x86_64-apple-darwin.tar.gz
description: Dufs — HTTP file server with upload, resume, directory UI
license:
  id: MIT
  attribution: sigoden
```

**Step 3: Write `recipes/tools/fzf.yaml`**

```yaml
id: fzf
type: binary
group: tools
tags:
- computing
depth: reference-only
size_gb: 0.003
platforms:
  linux-x86_64: https://github.com/junegunn/fzf/releases/latest/download/fzf-0.61.1-linux_amd64.tar.gz
  linux-arm64: https://github.com/junegunn/fzf/releases/latest/download/fzf-0.61.1-linux_arm64.tar.gz
  macos-arm64: https://github.com/junegunn/fzf/releases/latest/download/fzf-0.61.1-darwin_arm64.tar.gz
  macos-x86_64: https://github.com/junegunn/fzf/releases/latest/download/fzf-0.61.1-darwin_amd64.tar.gz
description: fzf — fuzzy finder for interactive menu selection
license:
  id: MIT
  attribution: Junegunn Choi
```

**Step 4: Write `recipes/tools/7z.yaml`**

```yaml
id: 7z
type: binary
group: tools
tags:
- computing
depth: reference-only
size_gb: 0.001
platforms:
  linux-x86_64: https://7-zip.org/a/7z2409-linux-x64.tar.xz
  linux-arm64: https://7-zip.org/a/7z2409-linux-arm64.tar.xz
  macos-arm64: https://7-zip.org/a/7z2409-mac.tar.xz
description: 7-Zip — universal archive tool (zip, 7z, tar, gz, xz, rar, iso)
license:
  id: LGPL-2.1-or-later
  attribution: Igor Pavlov
```

**Step 5: Write `recipes/tools/zstd.yaml`**

```yaml
id: zstd
type: binary
group: tools
tags:
- computing
depth: reference-only
size_gb: 0.002
platforms:
  linux-x86_64: https://github.com/facebook/zstd/releases/latest/download/zstd-v1.5.7-linux-x86_64-musl.tar.gz
  linux-arm64: https://github.com/facebook/zstd/releases/latest/download/zstd-v1.5.7-linux-arm64-musl.tar.gz
  macos-arm64: https://github.com/facebook/zstd/releases/latest/download/zstd-v1.5.7-macos-arm64.tar.gz
  macos-x86_64: https://github.com/facebook/zstd/releases/latest/download/zstd-v1.5.7-macos-x86_64.tar.gz
description: Zstandard — fast real-time compression
license:
  id: BSD-3-Clause
  attribution: Meta / Yann Collet
```

**Step 6: Verify recipes load correctly**

Run: `uv run python -c "from svalbard.presets import _build_recipe_index; idx = _build_recipe_index(); assert 'toybox' in idx; assert 'dufs' in idx; assert 'fzf' in idx; assert '7z' in idx; assert 'zstd' in idx; print('OK:', list(sorted(k for k in idx if k in ('toybox','dufs','fzf','7z','zstd'))))"``
Expected: `OK: ['7z', 'dufs', 'fzf', 'toybox', 'zstd']`

**Step 7: Commit**

```bash
git add recipes/tools/toybox.yaml recipes/tools/dufs.yaml recipes/tools/fzf.yaml recipes/tools/7z.yaml recipes/tools/zstd.yaml
git commit -m "feat: add tool recipes for toybox, dufs, fzf, 7z, zstd"
```

---

### Task 6: Add toolkit binaries to presets

Include the new tool recipes in all preset YAML files.

**Files:**
- Modify: `presets/default-32.yaml`
- Modify: `presets/default-64.yaml`
- Modify: `presets/default-128.yaml`
- Modify: `presets/default-256.yaml`
- Modify: `presets/default-512.yaml`
- Modify: `presets/default-1tb.yaml`
- Modify: `presets/default-2tb.yaml`
- Modify: `presets/finland-32.yaml`
- Modify: `presets/finland-64.yaml`
- Modify: `presets/finland-128.yaml`
- Modify: `presets/finland-256.yaml`
- Modify: `presets/finland-512.yaml`
- Modify: `presets/finland-1tb.yaml`
- Modify: `presets/finland-2tb.yaml`

**Step 1: Add base toolkit to every preset**

Append to the `# ── Tools` section of every preset YAML:
```yaml
- toybox
- dufs
- fzf
- 7z
```

**Step 2: Add zstd to 128 GB+ presets**

Append to 128+ presets:
```yaml
- zstd
```

**Step 3: Verify presets still load**

Run: `uv run pytest tests/test_presets.py -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add presets/
git commit -m "feat: add toolkit binaries (toybox, dufs, fzf, 7z, zstd) to all presets"
```

---

### Task 7: Update README to reference run.sh

**Files:**
- Modify: `src/svalbard/readme_generator.py:86-93` — change serve.sh reference to run.sh

**Step 1: Update the quick-start section**

In `readme_generator.py`, change the generated README content from:
```python
"./serve.sh",
```
to:
```python
"./run.sh",
```

And update the description text:
```python
"The script shows a menu of everything available on this drive — browse",
"encyclopedias, view maps, chat with an LLM, open tools, and more.",
```

**Step 2: Verify README generates correctly**

Run: `uv run python -c "
from pathlib import Path
from svalbard.readme_generator import generate_drive_readme
# Quick sanity check that the template renders
import inspect
src = inspect.getsource(generate_drive_readme)
assert 'run.sh' in src
print('OK')
"`

**Step 3: Commit**

```bash
git add src/svalbard/readme_generator.py
git commit -m "docs: update generated README to reference run.sh instead of serve.sh"
```

---

### Task 8: Clean up old serve_generator

**Files:**
- Delete: `src/svalbard/serve_generator.py`
- Modify: `src/svalbard/commands.py` — remove any remaining serve_generator references

**Step 1: Verify no remaining imports**

Run: `uv run grep -r "serve_generator" src/ tests/`
Expected: should only show the file itself and possibly old test imports

**Step 2: Delete `serve_generator.py`**

```bash
git rm src/svalbard/serve_generator.py
```

**Step 3: Run full test suite**

Run: `uv run pytest tests/ -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add -A
git commit -m "refactor: remove serve_generator, fully replaced by toolkit_generator"
```

---

### Summary

| Task | What | Files |
|------|------|-------|
| 1 | Shared lib scripts | `recipes/actions/lib/*.sh` |
| 2 | Action scripts | `recipes/actions/*.sh` |
| 3 | Toolkit generator | `src/svalbard/toolkit_generator.py` + tests |
| 4 | Wire into build pipeline | `src/svalbard/commands.py` |
| 5 | Tool recipes | `recipes/tools/{toybox,dufs,fzf,7z,zstd}.yaml` |
| 6 | Add tools to presets | `presets/*.yaml` |
| 7 | Update README generator | `src/svalbard/readme_generator.py` |
| 8 | Remove serve_generator | `src/svalbard/serve_generator.py` |
