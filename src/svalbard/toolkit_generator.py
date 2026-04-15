"""Generate the drive runtime toolkit on the drive.

Copies action scripts and lib helpers from recipes/actions/ to .svalbard/
on the drive, and generates runtime.json based on actual drive content.
"""

import json
import os
import platform as host_platform
import shutil
import stat
import subprocess
import tempfile
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
DRIVE_RUNTIME_DIR = _PROJECT_ROOT / "drive-runtime"
_RUNTIME_BINARY_CACHE: dict[str, Path] | None = None


def _detect_host_runtime_platform() -> str:
    system = host_platform.system().lower()
    machine = host_platform.machine().lower()

    if system == "darwin":
        if machine in {"arm64", "aarch64"}:
            return "macos-arm64"
        if machine in {"x86_64", "amd64"}:
            return "macos-x86_64"
    if system == "linux":
        if machine in {"arm64", "aarch64"}:
            return "linux-arm64"
        if machine in {"x86_64", "amd64"}:
            return "linux-x86_64"
    raise ValueError(f"Unsupported host platform: system={system} machine={machine}")


def _filter_runtime_binaries(
    binaries: dict[str, Path],
    platform_filter: str | None,
) -> dict[str, Path]:
    if not platform_filter:
        return binaries

    normalized = platform_filter.strip().lower()
    aliases = {
        "host": _detect_host_runtime_platform(),
        "arm64": "arm64",
        "aarch64": "arm64",
        "x86_64": "x86_64",
        "x64": "x86_64",
        "amd64": "x86_64",
        "macos-arm64": "macos-arm64",
        "macos-x86_64": "macos-x86_64",
        "linux-arm64": "linux-arm64",
        "linux-x86_64": "linux-x86_64",
    }
    if normalized not in aliases:
        raise ValueError(f"Unsupported platform filter: {platform_filter}")

    resolved = aliases[normalized]
    if resolved in {"arm64", "x86_64"}:
        return {
            platform: binary
            for platform, binary in binaries.items()
            if platform.endswith(f"-{resolved}")
        }
    return {resolved: binaries[resolved]}


# ── runtime.json generation ─────────────────────────────────────────────────


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


def _visible_chat_entries(manifest: Manifest) -> list:
    embed_keywords = {"embed", "nomic-embed", "bge-", "e5-", "arctic-embed"}
    gguf_entries = [e for e in manifest.entries if e.type == "gguf"]
    return [
        e for e in gguf_entries
        if not any(keyword in e.id.lower() for keyword in embed_keywords)
    ]


def _available_ai_clients(manifest: Manifest) -> list[str]:
    client_ids = {"opencode": "OpenCode", "crush": "Crush", "goose": "Goose"}
    available_ids = {entry.id for entry in manifest.entries if entry.type == "binary"}
    return [client_id for client_id in client_ids if client_id in available_ids]


def _build_runtime_config(drive_path: Path, manifest: Manifest, preset_name: str) -> dict:
    """Build runtime.json content based on what's on the drive."""
    preset = load_snapshot_preset(drive_path) or load_preset(
        preset_name,
        workspace=manifest.workspace_root or None,
    )
    actions: list[dict] = []

    def add_action(section: str, label: str, action: str, args: dict[str, str] | None = None) -> None:
        actions.append({
            "section": section,
            "label": label,
            "action": action,
            "args": args or {},
        })

    # ── Browse ──────────────────────────────────────────────────────────
    zim_count = _count_files(drive_path / TYPE_DIRS["zim"], "*.zim")
    if zim_count > 0:
        add_action(
            "browse",
            f"Browse encyclopedias — {zim_count} ZIM files",
            "browse",
        )

    # ── Search ──────────────────────────────────────────────────────────
    search_db = drive_path / "data" / "search.db"
    if search_db.exists():
        add_action("search", "Search all content", "search")

    # ── Maps ────────────────────────────────────────────────────────────
    pmtiles_count = _count_files(drive_path / TYPE_DIRS["pmtiles"], "*.pmtiles")
    if pmtiles_count > 0:
        add_action(
            "maps",
            f"View maps — {pmtiles_count} tile layers",
            "maps",
        )

    # ── AI ──────────────────────────────────────────────────────────────
    chat_entries = _visible_chat_entries(manifest)
    if chat_entries:
        for entry in chat_entries:
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            model_path = f"{TYPE_DIRS['gguf']}/{entry.filename}"
            add_action(
                "ai",
                f"Chat with {desc}",
                "chat",
                {"model": model_path},
            )
        if "llama-server" in {entry.id for entry in manifest.entries if entry.type == "binary"}:
            client_labels = {"opencode": "OpenCode", "crush": "Crush", "goose": "Goose"}
            for client_id in _available_ai_clients(manifest):
                add_action(
                    "ai",
                    f"{client_labels[client_id]} with local model",
                    "agent",
                    {"client": client_id},
                )

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
        for source in visible_apps:
            add_action(
                "apps",
                f"Open {source.description or source.id}",
                "apps",
                {"app": source.id},
            )

    # ── Data ────────────────────────────────────────────────────────────
    db_entries = [e for e in manifest.entries if e.type == "sqlite"]
    sqliteviz_available = (drive_path / "apps" / "sqliteviz").exists()
    if db_entries and sqliteviz_available:
        for entry in sorted(db_entries, key=lambda e: e.id):
            source = next((s for s in preset.sources if s.id == entry.id), None)
            desc = source.description if source else entry.id
            add_action(
                "data",
                f"Query {desc}",
                "apps",
                {"app": "sqliteviz", "dataset": entry.id},
            )

    # ── Embedded Dev ───────────────────────────────────────────────────
    toolchain_entries = [e for e in manifest.entries if e.type == "toolchain"]
    if toolchain_entries:
        add_action("embedded", "Open embedded dev shell", "embedded-shell")

    # ── Serve ───────────────────────────────────────────────────────────
    has_services = zim_count > 0 or pmtiles_count > 0 or bool(chat_entries)
    if has_services:
        add_action("serve", "Serve everything", "serve-all")
        add_action("serve", "Share on local network", "share")

    # ── Info ─────────────────────────────────────────────────────────────
    add_action("info", "List drive contents", "inspect")
    has_checksums = any(e.checksum_sha256 for e in manifest.entries)
    if has_checksums:
        add_action("info", "Verify checksums", "verify")

    return {
        "version": 1,
        "preset": preset_name,
        "actions": actions,
    }


# ── runtime launcher helpers ────────────────────────────────────────────────


def _build_drive_runtime_binaries(platform_filter: str | None = None) -> dict[str, Path]:
    """Build platform-specific drive runtime binaries once per Python process."""
    global _RUNTIME_BINARY_CACHE
    if _RUNTIME_BINARY_CACHE is not None:
        return _filter_runtime_binaries(_RUNTIME_BINARY_CACHE, platform_filter)

    targets = {
        "macos-arm64": ("darwin", "arm64"),
        "macos-x86_64": ("darwin", "amd64"),
        "linux-arm64": ("linux", "arm64"),
        "linux-x86_64": ("linux", "amd64"),
    }

    build_root = Path(tempfile.mkdtemp(prefix="svalbard-drive-runtime-"))
    go_cache = build_root / ".gocache"
    go_cache.mkdir(parents=True, exist_ok=True)

    binaries: dict[str, Path] = {}
    for platform, (goos, goarch) in targets.items():
        out_dir = build_root / platform
        out_dir.mkdir(parents=True, exist_ok=True)
        output = out_dir / "svalbard-drive"
        env = os.environ.copy()
        env.update({
            "CGO_ENABLED": "0",
            "GOOS": goos,
            "GOARCH": goarch,
            "GOCACHE": str(go_cache),
        })
        subprocess.run(
            ["go", "build", "-o", str(output), "./cmd/svalbard-drive"],
            cwd=DRIVE_RUNTIME_DIR,
            env=env,
            check=True,
            capture_output=True,
            text=True,
        )
        _make_executable(output)
        binaries[platform] = output

    _RUNTIME_BINARY_CACHE = binaries
    return _filter_runtime_binaries(binaries, platform_filter)


# ── run.sh template ─────────────────────────────────────────────────────────

RUN_SH = r'''#!/usr/bin/env bash
set -euo pipefail
DRIVE_ROOT="$(cd "$(dirname "$0")" && pwd)"
export DRIVE_ROOT

case "$(uname -s):$(uname -m)" in
    Darwin:arm64) platform="macos-arm64" ;;
    Darwin:x86_64) platform="macos-x86_64" ;;
    Linux:aarch64|Linux:arm64) platform="linux-arm64" ;;
    Linux:x86_64) platform="linux-x86_64" ;;
    *)
        echo "Unsupported platform: $(uname -s) $(uname -m)" >&2
        exit 1
        ;;
esac

exec "$DRIVE_ROOT/.svalbard/runtime/$platform/svalbard-drive"
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


def generate_toolkit(drive_path: Path, preset_name: str, platform_filter: str | None = None) -> Path:
    """Assemble the full .svalbard/ toolkit on the drive.

    1. Copy action scripts to .svalbard/actions/
    2. Copy lib helpers to .svalbard/lib/
    3. Generate runtime.json
    4. Write run.sh
    """
    svalbard_dir = drive_path / ".svalbard"
    actions_dest = svalbard_dir / "actions"
    lib_dest = svalbard_dir / "lib"
    runtime_dest = svalbard_dir / "runtime"
    runtime_path = svalbard_dir / "runtime.json"
    entries_path = svalbard_dir / "entries.tab"

    # Refresh toolkit-managed files but preserve config snapshots.
    for managed_dir in (actions_dest, lib_dest, runtime_dest):
        if managed_dir.exists():
            shutil.rmtree(managed_dir, ignore_errors=True)
        if managed_dir.exists():
            # Fallback: subprocess rm for stubborn filesystems (FAT32/exFAT USB)
            subprocess.run(["rm", "-rf", str(managed_dir)], check=False)
    for managed_file in (runtime_path, entries_path):
        if managed_file.exists():
            managed_file.unlink()
    actions_dest.mkdir(parents=True)
    lib_dest.mkdir(parents=True)
    runtime_dest.mkdir(parents=True)

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

    # Generate runtime.json
    manifest = Manifest.load(drive_path / "manifest.yaml")
    runtime_config = _build_runtime_config(drive_path, manifest, preset_name)
    runtime_path.write_text(json.dumps(runtime_config, indent=2) + "\n")

    # Copy platform-specific Go drive runtime launchers
    for platform, binary in _build_drive_runtime_binaries(platform_filter=platform_filter).items():
        dest = runtime_dest / platform / "svalbard-drive"
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(binary, dest)
        _make_executable(dest)

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
