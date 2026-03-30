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

# Canonical type→directory mapping (shared with commands.py)
TYPE_DIRS = {
    "zim": "zim",
    "pmtiles": "maps",
    "pdf": "books",
    "epub": "books",
    "gguf": "models",
    "binary": "bin",
    "app": "apps",
    "iso": "infra",
    "sqlite": "data",
}

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
    zim_count = _count_files(drive_path / TYPE_DIRS["zim"], "*.zim")
    if zim_count > 0:
        lines.append("[browse]")
        lines.append(
            f"Browse encyclopedias — {zim_count} ZIM files"
            f"\t.svalbard/actions/browse.sh"
        )
        lines.append("")

    # ── Search ──────────────────────────────────────────────────────────
    search_db = drive_path / "data" / "search.db"
    if search_db.exists():
        lines.append("[search]")
        lines.append(
            f"Search all content"
            f"\t.svalbard/actions/search.sh"
        )
        lines.append("")

    # ── Maps ────────────────────────────────────────────────────────────
    pmtiles_count = _count_files(drive_path / TYPE_DIRS["pmtiles"], "*.pmtiles")
    if pmtiles_count > 0:
        lines.append("[maps]")
        lines.append(
            f"View maps — {pmtiles_count} tile layers"
            f"\t.svalbard/actions/maps.sh"
        )
        lines.append("")

    # ── AI ──────────────────────────────────────────────────────────────
    gguf_entries = [e for e in manifest.entries if e.type == "gguf"]
    if gguf_entries:
        lines.append("[ai]")
        for entry in gguf_entries:
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            model_path = f"{TYPE_DIRS['gguf']}/{entry.filename}"
            lines.append(
                f"Chat with {desc}"
                f"\t.svalbard/actions/chat.sh\t{model_path}"
            )
        lines.append("")

    # ── Apps ────────────────────────────────────────────────────────────
    apps_dir = drive_path / TYPE_DIRS["app"]
    app_sources = [s for s in preset.sources if s.type == "app"]
    visible_apps = []
    if apps_dir.exists():
        for source in app_sources:
            app_dir = apps_dir / source.id
            if app_dir.exists():
                visible_apps.append(source)
    if visible_apps:
        lines.append("[apps]")
        for source in visible_apps:
            lines.append(
                f"Open {source.description or source.id}"
                f"\t.svalbard/actions/apps.sh\t{source.id}"
            )
        lines.append("")

    # ── Data ────────────────────────────────────────────────────────────
    db_entries = [e for e in manifest.entries if e.type == "sqlite"]
    if db_entries:
        lines.append("[data]")
        for entry in sorted(db_entries, key=lambda e: e.id):
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            lines.append(
                f"Query {desc}"
                f"\t.svalbard/actions/apps.sh\tsqliteviz"
            )
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
    has_checksums = any(e.checksum_sha256 for e in manifest.entries)
    if has_checksums:
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
    IFS=$'\t' read -r label script args <<< "$line"
    LABELS+=("$label")
    SCRIPTS+=("$script")
    ARGS+=("${args:-}")
    GROUPS+=("$current_group")
done < "$ENTRIES_FILE"

# Main loop
while true; do
    echo ""
    echo "${BOLD}Svalbard${NC}"
    echo "─────────────────────────────────────────"
    prev_group=""
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

    read -rp "  > " choice
    case "${choice:-}" in
        q|Q|"") exit 0 ;;
        *[!0-9]*) continue ;;
    esac
    if [ "$choice" -ge 1 ] 2>/dev/null && [ "$choice" -le "${#LABELS[@]}" ]; then
        idx=$((choice - 1))
    else
        continue
    fi

    script="${SCRIPTS[$idx]}"
    args="${ARGS[$idx]}"

    if [ ! -f "$DRIVE_ROOT/$script" ]; then
        echo "${RED}Script not found: $script${NC}"
        read -rp "Press Enter to continue..."
        continue
    fi

    chmod +x "$DRIVE_ROOT/$script" 2>/dev/null || true
    # Run action script, tolerating non-zero exit (e.g. Ctrl+C stops a service)
    set +e
    if [ -n "$args" ]; then
        "$DRIVE_ROOT/$script" "$args"
    else
        "$DRIVE_ROOT/$script"
    fi
    set -e

    echo ""
    read -rp "Press Enter to return to menu..."
done
'''


# ── Public API ──────────────────────────────────────────────────────────────


def _generate_checksums(svalbard_dir: Path, manifest: Manifest) -> None:
    """Write checksums.sha256 from manifest entries that have checksums."""
    lines = []
    for entry in sorted(manifest.entries, key=lambda e: e.id):
        if not entry.checksum_sha256:
            continue
        # Build relative path from drive root
        if entry.relative_path:
            rel_path = entry.relative_path
        elif entry.platform:
            rel_path = f"bin/{entry.platform}/{entry.filename}"
        else:
            type_dir = TYPE_DIRS.get(entry.type, "other")
            rel_path = f"{type_dir}/{entry.filename}"
        lines.append(f"{entry.checksum_sha256}  {rel_path}")

    if lines:
        (svalbard_dir / "checksums.sha256").write_text("\n".join(lines) + "\n")


def _make_executable(path: Path) -> None:
    path.chmod(path.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


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
            _make_executable(dest)

    # Copy lib scripts
    if LIB_DIR.exists():
        for script in LIB_DIR.glob("*.sh"):
            dest = lib_dest / script.name
            shutil.copy2(script, dest)

    # Generate entries.tab
    manifest = Manifest.load(drive_path / "manifest.yaml")
    tab_content = _build_entries(drive_path, manifest, preset_name)
    (svalbard_dir / "entries.tab").write_text(tab_content)

    # Generate checksums.sha256 from manifest entries
    _generate_checksums(svalbard_dir, manifest)

    # Write run.sh
    run_sh = drive_path / "run.sh"
    run_sh.write_text(RUN_SH)
    _make_executable(run_sh)

    return run_sh
