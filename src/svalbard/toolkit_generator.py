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
set -uo pipefail

DRIVE_ROOT="$(cd "$(dirname "$0")" && pwd)"
export DRIVE_ROOT

ENTRIES_FILE="$DRIVE_ROOT/.svalbard/entries.tab"
if [ ! -f "$ENTRIES_FILE" ]; then
    echo "Error: entries.tab not found. Is this a Svalbard drive?"
    exit 1
fi

# Colors
if [ -t 1 ]; then
    B=$'\033[1m'; D=$'\033[2m'; C=$'\033[36m'; R=$'\033[31m'; N=$'\033[0m'
else
    B="" D="" C="" R="" N=""
fi

# Parse entries from tab file
LABELS=()
SCRIPTS=()
ARGS=()
GROUPS=()
grp=""
while IFS=$'\t' read -r lbl scr arg <&3; do
    [ -z "$lbl" ] && continue
    [[ "$lbl" = \#* ]] && continue
    if [[ "$lbl" = \[*\] ]]; then
        grp="${lbl:1:${#lbl}-2}"
        continue
    fi
    LABELS+=("$lbl")
    SCRIPTS+=("${scr:-}")
    ARGS+=("${arg:-}")
    GROUPS+=("$grp")
done 3< "$ENTRIES_FILE"

n=${#LABELS[@]}
if [ "$n" -eq 0 ]; then
    echo "No menu entries found."
    exit 1
fi

# Main loop
while true; do
    echo ""
    echo "${B}Svalbard${N}"
    echo "────────────────────────────────"
    pg=""
    for (( i=0; i<n; i++ )); do
        [ "${GROUPS[$i]}" != "$pg" ] && { [ -n "$pg" ] && echo ""; pg="${GROUPS[$i]}"; }
        printf " ${C}%2d${N}) %s\n" "$((i+1))" "${LABELS[$i]}"
    done
    printf "\n ${D} q) Quit${N}\n\n"
    read -rp " > " choice

    case "${choice:-}" in
        q|Q|"") exit 0 ;;
    esac
    [[ "$choice" =~ ^[0-9]+$ ]] || continue
    (( choice >= 1 && choice <= n )) || continue
    idx=$((choice - 1))

    scr="$DRIVE_ROOT/${SCRIPTS[$idx]}"
    if [ ! -f "$scr" ]; then
        echo "${R}Not found: ${SCRIPTS[$idx]}${N}"
        read -rp "Enter to continue..."
        continue
    fi

    source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
    source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
    source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
    source "$DRIVE_ROOT/.svalbard/lib/process.sh"

    chmod +x "$scr" 2>/dev/null || true
    set +e
    if [ -n "${ARGS[$idx]}" ]; then
        "$scr" "${ARGS[$idx]}"
    else
        "$scr"
    fi
    set -e
    echo ""
    read -rp "Enter to return..."
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
