"""Generate the run.sh toolkit on the drive.

Copies action scripts and lib helpers from recipes/actions/ to .svalbard/
on the drive, and generates entries.tab based on actual drive content.
"""

import os
import shutil
import stat
import subprocess
from pathlib import Path

from svalbard.drive_config import load_snapshot_preset
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
    "toolchain": "tools/platformio/packages",
}

_PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
ACTIONS_DIR = _PROJECT_ROOT / "recipes" / "actions"
LIB_DIR = ACTIONS_DIR / "lib"
DOCS_DIR = ACTIONS_DIR / "docs"


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

    preset = load_snapshot_preset(drive_path) or load_preset(
        preset_name,
        workspace=manifest.workspace_root or None,
    )

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
    # Exclude embedding models — they're not for chat
    _EMBED_KEYWORDS = {"embed", "nomic-embed", "bge-", "e5-", "arctic-embed"}
    gguf_entries = [e for e in manifest.entries if e.type == "gguf"]
    chat_entries = [
        e for e in gguf_entries
        if not any(kw in e.id.lower() for kw in _EMBED_KEYWORDS)
    ]
    if chat_entries:
        lines.append("[ai]")
        for entry in chat_entries:
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            model_path = f"{TYPE_DIRS['gguf']}/{entry.filename}"
            lines.append(
                f"Chat with {desc}"
                f"\t.svalbard/actions/chat.sh\t{model_path}"
            )
        lines.append("")

    # ── Apps ────────────────────────────────────────────────────────────
    # Exclude vendor/support libraries (maplibre-vendor etc.) — they're not user-facing
    _VENDOR_KEYWORDS = {"vendor", "maplibre-vendor"}
    apps_dir = drive_path / TYPE_DIRS["app"]
    app_sources = [s for s in preset.sources if s.type == "app"]
    visible_apps = []
    if apps_dir.exists():
        for source in app_sources:
            if source.id in _VENDOR_KEYWORDS:
                continue
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
    sqliteviz_available = (drive_path / "apps" / "sqliteviz").exists()
    if db_entries and sqliteviz_available:
        lines.append("[data]")
        for entry in sorted(db_entries, key=lambda e: e.id):
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            lines.append(
                f"Query {desc}"
                f"\t.svalbard/actions/apps.sh\tsqliteviz"
            )
        lines.append("")

    # ── Embedded Dev ───────────────────────────────────────────────────
    toolchain_entries = [e for e in manifest.entries if e.type == "toolchain"]
    if toolchain_entries:
        lines.append("[embedded]")
        lines.append(
            "Open embedded dev shell"
            "\t.svalbard/actions/pio-setup.sh"
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
DRIVE_ROOT="$(cd "$(dirname "$0")" && pwd)"
export DRIVE_ROOT

ENTRIES="$DRIVE_ROOT/.svalbard/entries.tab"
[ -f "$ENTRIES" ] || { echo "entries.tab not found"; exit 1; }

# Parse
labels=(); scripts=(); args=(); groups=(); g=""
while IFS=$'\t' read -r l s a <&3 || [ -n "$l" ]; do
    [[ -z "$l" || "$l" = \#* ]] && continue
    [[ "$l" = \[*\] ]] && { g="${l:1:${#l}-2}"; continue; }
    labels+=("$l"); scripts+=("${s:-}"); args+=("${a:-}"); groups+=("$g")
done 3< "$ENTRIES"

total=${#labels[@]}
[ "$total" -eq 0 ] && { echo "No entries."; exit 1; }

while true; do
    echo ""
    echo "Svalbard"
    echo "────────────────────────────────"
    for (( i=0; i<total; i++ )); do
        printf "  %2d) %s\n" "$((i+1))" "${labels[$i]}"
    done
    echo ""
    echo "   q) Quit"
    echo ""
    read -rp "  > " ch

    [[ "$ch" = q || "$ch" = Q || -z "$ch" ]] && exit 0
    [[ "$ch" =~ ^[0-9]+$ ]] || continue
    (( ch >= 1 && ch <= total )) 2>/dev/null || continue
    idx=$((ch - 1))

    target="$DRIVE_ROOT/${scripts[$idx]}"
    [ -f "$target" ] || { echo "Not found: ${scripts[$idx]}"; read -rp "Enter..."; continue; }

    export DRIVE_ROOT
    source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
    source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
    source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
    source "$DRIVE_ROOT/.svalbard/lib/ports.sh"
    source "$DRIVE_ROOT/.svalbard/lib/process.sh"

    chmod +x "$target" 2>/dev/null || true
    if [ -n "${args[$idx]}" ]; then
        "$target" "${args[$idx]}" || true
    else
        "$target" || true
    fi
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
    entries_path = svalbard_dir / "entries.tab"

    # Refresh toolkit-managed files but preserve config snapshots.
    for managed_dir in (actions_dest, lib_dest):
        if managed_dir.exists():
            shutil.rmtree(managed_dir, ignore_errors=True)
        if managed_dir.exists():
            # Fallback: subprocess rm for stubborn filesystems (FAT32/exFAT USB)
            subprocess.run(["rm", "-rf", str(managed_dir)], check=False)
    if entries_path.exists():
        entries_path.unlink()
    actions_dest.mkdir(parents=True)
    lib_dest.mkdir(parents=True)

    # Copy action scripts
    if ACTIONS_DIR.exists():
        for script in ACTIONS_DIR.glob("*.sh"):
            dest = actions_dest / script.name
            shutil.copy2(script, dest)
            _make_executable(dest)

    # Copy lib scripts and support files
    if LIB_DIR.exists():
        for pattern in ("*.sh", "*.py"):
            for script in LIB_DIR.glob(pattern):
                dest = lib_dest / script.name
                shutil.copy2(script, dest)

    # Generate entries.tab
    manifest = Manifest.load(drive_path / "manifest.yaml")
    tab_content = _build_entries(drive_path, manifest, preset_name)
    entries_path.write_text(tab_content)

    # Generate checksums.sha256 from manifest entries
    _generate_checksums(svalbard_dir, manifest)

    # Copy embedded dev docs if toolchain content is present
    if any(e.type == "toolchain" for e in manifest.entries):
        pio_dir = drive_path / "tools" / "platformio"
        pio_dir.mkdir(parents=True, exist_ok=True)
        guide = DOCS_DIR / "embedded-getting-started.md"
        if guide.exists():
            shutil.copy2(guide, pio_dir / "README.md")

    # Write run.sh
    run_sh = drive_path / "run.sh"
    run_sh.write_text(RUN_SH)
    _make_executable(run_sh)

    return run_sh
