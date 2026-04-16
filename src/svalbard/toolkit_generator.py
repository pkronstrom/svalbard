"""Generate the drive runtime toolkit on the drive."""

import copy
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
DOCS_DIR = _PROJECT_ROOT / "recipes" / "actions" / "docs"
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


# ── actions.json generation ─────────────────────────────────────────────────


def _count_files(directory: Path, pattern: str) -> int:
    if not directory.exists():
        return 0
    return len(list(directory.glob(pattern)))


def _visible_chat_entries(manifest: Manifest) -> list:
    embed_keywords = {"embed", "nomic-embed", "bge-", "e5-", "arctic-embed"}
    gguf_entries = [e for e in manifest.entries if e.type == "gguf"]
    return [
        e for e in gguf_entries
        if not any(keyword in e.id.lower() for keyword in embed_keywords)
    ]


def _available_ai_clients(manifest: Manifest) -> list[str]:
    client_ids = {"opencode": "OpenCode", "goose": "Goose"}
    available_ids = {entry.id for entry in manifest.entries if entry.type == "binary"}
    return [client_id for client_id in client_ids if client_id in available_ids]


GROUP_DEFAULTS = {
    "search": {
        "label": "Search",
        "description": "Search across indexed archives and documents.",
        "order": 100,
    },
    "library": {
        "label": "Library",
        "description": "Browse packaged offline archives and documents.",
        "order": 200,
    },
    "maps": {
        "label": "Maps",
        "description": "Open offline map layers in the browser.",
        "order": 300,
    },
    "local-ai": {
        "label": "Local AI",
        "description": "Chat with local models and launch bundled AI clients.",
        "order": 400,
    },
    "tools": {
        "label": "Tools",
        "description": "Inspect the drive and launch bundled utilities.",
        "order": 500,
    },
}


def _titleize_identifier(value: str) -> str:
    return value.replace("-", " ").replace("_", " ").title()


def _source_menu(source) -> dict:
    if source and isinstance(source.menu, dict):
        return source.menu
    return {}


def _source_label(source, fallback: str) -> str:
    menu = _source_menu(source)
    return menu.get("label") or (source.description if source else "") or fallback


def _source_description(source, fallback: str) -> str:
    menu = _source_menu(source)
    return menu.get("description") or (source.description if source else "") or fallback


def _source_order(source, default: int) -> int:
    menu = _source_menu(source)
    return int(menu.get("order", default))


def _source_subheader(source, default: str | None = None) -> str | None:
    menu = _source_menu(source)
    return menu.get("subheader", default)


def _builtin_action(name: str, args: dict[str, str] | None = None) -> dict:
    return {
        "type": "builtin",
        "config": {
            "name": name,
            "args": args or {},
        },
    }


def _source_action(source, builtin_name: str, builtin_args: dict[str, str] | None = None) -> dict:
    if source and isinstance(source.action, dict) and source.action:
        return copy.deepcopy(source.action)
    return _builtin_action(builtin_name, builtin_args)


def _ensure_group(
    groups: dict[str, dict],
    group_id: str,
    default_order: int,
    label: str | None = None,
    description: str | None = None,
    order: int | None = None,
) -> dict:
    meta = GROUP_DEFAULTS.get(group_id, {})
    group = groups.setdefault(group_id, {
        "id": group_id,
        "label": label or meta.get("label") or _titleize_identifier(group_id),
        "description": description or meta.get("description") or f"Open {group_id} items.",
        "order": order if order is not None else int(meta.get("order", default_order)),
        "items": [],
    })
    if label:
        group["label"] = label
    if description:
        group["description"] = description
    if order is not None:
        group["order"] = order
    return group


def _add_group_item(
    groups: dict[str, dict],
    group_id: str,
    item_id: str,
    label: str,
    description: str,
    action: dict,
    *,
    subheader: str | None = None,
    order: int = 100,
    group_label: str | None = None,
    group_description: str | None = None,
    group_order: int | None = None,
) -> None:
    group = _ensure_group(
        groups,
        group_id,
        default_order=1000 + len(groups),
        label=group_label,
        description=group_description,
        order=group_order,
    )
    item = {
        "id": item_id,
        "label": label,
        "description": description,
        "action": action,
        "order": order,
        "_index": len(group["items"]),
    }
    if subheader:
        item["subheader"] = subheader
    group["items"].append(item)


def _group_meta_from_source(source, group_id: str) -> dict[str, str | int] | None:
    menu = _source_menu(source)
    if menu.get("group") != group_id:
        return None
    meta: dict[str, str | int] = {}
    if menu.get("group_label"):
        meta["label"] = menu["group_label"]
    if menu.get("group_description"):
        meta["description"] = menu["group_description"]
    if "group_order" in menu:
        meta["order"] = int(menu["group_order"])
    return meta or None


def _build_actions_config(drive_path: Path, manifest: Manifest, preset_name: str) -> dict:
    """Build actions.json content based on what's on the drive."""
    preset = load_snapshot_preset(drive_path) or load_preset(
        preset_name,
        workspace=manifest.workspace_root or None,
    )
    source_by_id = {source.id: source for source in preset.sources}
    groups: dict[str, dict] = {}

    # ── Browse ──────────────────────────────────────────────────────────
    for entry in sorted((entry for entry in manifest.entries if entry.type == "zim"), key=lambda entry: entry.id):
        zim_path = drive_path / TYPE_DIRS["zim"] / entry.filename
        if not zim_path.exists():
            continue
        source = source_by_id.get(entry.id)
        menu = _source_menu(source)
        group_id = menu.get("group", "library")
        group_meta = _group_meta_from_source(source, group_id) or {}
        label = _source_label(source, _titleize_identifier(entry.id))
        description = _source_description(source, f"Open {label}.")
        _add_group_item(
            groups,
            group_id,
            item_id=entry.id,
            label=label,
            description=description,
            action=_source_action(source, "browse", {"zim": entry.filename}),
            subheader=_source_subheader(source, "Archives"),
            order=_source_order(source, 100),
            group_label=group_meta.get("label"),
            group_description=group_meta.get("description"),
            group_order=group_meta.get("order"),
        )

    # ── Search ──────────────────────────────────────────────────────────
    search_db = drive_path / "data" / "search.db"
    if search_db.exists():
        _add_group_item(
            groups,
            "search",
            item_id="search-all-content",
            label="Search all content",
            description="Query the on-drive search index across packaged sources.",
            action=_builtin_action("search"),
            order=100,
        )

    # ── Maps ────────────────────────────────────────────────────────────
    pmtiles_count = _count_files(drive_path / TYPE_DIRS["pmtiles"], "*.pmtiles")
    if pmtiles_count > 0:
        _add_group_item(
            groups,
            "maps",
            item_id="open-map-viewer",
            label="Open map viewer",
            description=f"View {pmtiles_count} offline map layer{'s' if pmtiles_count != 1 else ''} in the browser.",
            action=_builtin_action("maps"),
            order=100,
        )

    # ── AI ──────────────────────────────────────────────────────────────
    chat_entries = _visible_chat_entries(manifest)
    if chat_entries:
        for entry in chat_entries:
            source = source_by_id.get(entry.id)
            model_name = source.description if source else _titleize_identifier(entry.id)
            model_path = str(drive_path / TYPE_DIRS["gguf"] / entry.filename)
            _add_group_item(
                groups,
                "local-ai",
                item_id=entry.id,
                label=f"Chat with {model_name}",
                description=f"Start the local chat interface with {model_name}.",
                action=_source_action(source, "chat", {"model": model_path}),
                subheader=_source_subheader(source, "Chat Models"),
                order=_source_order(source, 100),
            )
        if "llama-server" in {entry.id for entry in manifest.entries if entry.type == "binary"}:
            client_labels = {"opencode": "OpenCode", "goose": "Goose"}
            for index, client_id in enumerate(_available_ai_clients(manifest), start=1):
                source = source_by_id.get(client_id)
                _add_group_item(
                    groups,
                    "local-ai",
                    item_id=client_id,
                    label=f"{client_labels[client_id]} with local model",
                    description=_source_description(
                        source,
                        f"Launch {client_labels[client_id]} against the local model runtime.",
                    ),
                    action=_source_action(source, "agent", {"client": client_id}),
                    subheader=_source_subheader(source, "AI Clients"),
                    order=_source_order(source, 200 + index),
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
            group_id = _source_menu(source).get("group", "tools")
            group_meta = _group_meta_from_source(source, group_id) or {}
            label = _source_label(source, _titleize_identifier(source.id))
            _add_group_item(
                groups,
                group_id,
                item_id=source.id,
                label=label,
                description=_source_description(source, f"Open {label}."),
                action=_source_action(source, "apps", {"app": source.id}),
                subheader=_source_subheader(source, "Apps"),
                order=_source_order(source, 200),
                group_label=group_meta.get("label"),
                group_description=group_meta.get("description"),
                group_order=group_meta.get("order"),
            )

    # ── Data ────────────────────────────────────────────────────────────
    db_entries = [e for e in manifest.entries if e.type == "sqlite"]
    sqliteviz_available = (drive_path / "apps" / "sqliteviz").exists()
    if db_entries and sqliteviz_available:
        for entry in sorted(db_entries, key=lambda e: e.id):
            source = source_by_id.get(entry.id)
            desc = source.description if source else _titleize_identifier(entry.id)
            _add_group_item(
                groups,
                "tools",
                item_id=f"query-{entry.id}",
                label=f"Query {desc}",
                description=f"Open {desc} in SQLiteViz.",
                action=_source_action(source, "apps", {"app": "sqliteviz", "dataset": entry.id}),
                subheader="Data",
                order=250,
            )

    # ── Embedded Dev ───────────────────────────────────────────────────
    toolchain_entries = [e for e in manifest.entries if e.type == "toolchain"]
    if toolchain_entries:
        _add_group_item(
            groups,
            "tools",
            item_id="embedded-shell",
            label="Open embedded dev shell",
            description="Drop into the bundled embedded development shell.",
            action=_builtin_action("embedded-shell"),
            subheader="Development",
            order=700,
        )

    # ── Serve ───────────────────────────────────────────────────────────
    zim_count = _count_files(drive_path / TYPE_DIRS["zim"], "*.zim")
    has_services = zim_count > 0 or pmtiles_count > 0 or bool(chat_entries)
    if has_services:
        _add_group_item(
            groups,
            "tools",
            item_id="serve-all",
            label="Serve everything",
            description="Start the available local services together.",
            action=_builtin_action("serve-all"),
            subheader="Sharing",
            order=300,
        )
        _add_group_item(
            groups,
            "tools",
            item_id="share-files",
            label="Share on local network",
            description="Share the drive contents over your local network.",
            action=_builtin_action("share"),
            subheader="Sharing",
            order=310,
        )

    # ── Info ─────────────────────────────────────────────────────────────
    _add_group_item(
        groups,
        "tools",
        item_id="inspect-drive",
        label="List drive contents",
        description="Show a terminal summary of the drive contents.",
        action=_builtin_action("inspect"),
        subheader="Drive",
        order=500,
    )
    has_checksums = any(e.checksum_sha256 for e in manifest.entries)
    if has_checksums:
        _add_group_item(
            groups,
            "tools",
            item_id="verify-checksums",
            label="Verify checksums",
            description="Hash managed files and compare them against the manifest checksums.",
            action=_builtin_action("verify"),
            subheader="Drive",
            order=510,
        )

    ordered_groups = []
    for group in sorted(groups.values(), key=lambda group: (group["order"], group["label"])):
        items = sorted(group["items"], key=lambda item: (item["order"], item["_index"]))
        for item in items:
            item.pop("_index", None)
        group["items"] = items
        ordered_groups.append(group)

    return {
        "version": 2,
        "preset": preset_name,
        "groups": ordered_groups,
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


# ── run template ────────────────────────────────────────────────────────────

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

    1. Remove legacy shell-runtime artifacts from older drives
    2. Generate actions.json
    3. Copy platform-specific Go launcher binaries
    4. Write run
    """
    svalbard_dir = drive_path / ".svalbard"
    actions_dest = svalbard_dir / "actions"
    lib_dest = svalbard_dir / "lib"
    runtime_dest = svalbard_dir / "runtime"
    actions_path = svalbard_dir / "actions.json"
    entries_path = svalbard_dir / "entries.tab"

    # Refresh toolkit-managed files but preserve config snapshots.
    for managed_dir in (actions_dest, lib_dest, runtime_dest):
        if managed_dir.exists():
            shutil.rmtree(managed_dir, ignore_errors=True)
        if managed_dir.exists():
            # Fallback: subprocess rm for stubborn filesystems (FAT32/exFAT USB)
            subprocess.run(["rm", "-rf", str(managed_dir)], check=False)
    for managed_file in (actions_path, entries_path):
        if managed_file.exists():
            managed_file.unlink()
    runtime_dest.mkdir(parents=True)

    # Generate actions.json
    manifest = Manifest.load(drive_path / "manifest.yaml")
    actions_config = _build_actions_config(drive_path, manifest, preset_name)
    actions_path.write_text(json.dumps(actions_config, indent=2) + "\n")

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

    # Write run
    run_file = drive_path / "run"
    run_file.write_text(RUN_SH)
    _make_executable(run_file)

    legacy_run_sh = drive_path / "run.sh"
    if legacy_run_sh.exists():
        legacy_run_sh.unlink()

    return run_file
