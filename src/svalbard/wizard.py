import os
import platform
import shutil
import subprocess
from datetime import datetime
from pathlib import Path

import yaml
from rich.console import Console
from rich.panel import Panel
from rich.prompt import Confirm, Prompt
from rich.table import Table

from svalbard.commands import init_drive, sync_drive
from svalbard.local_sources import load_local_sources
from svalbard.paths import workspace_root as resolve_workspace_root
from svalbard.picker import build_picker_tree, run_picker
from svalbard.presets import list_presets, load_preset, local_presets_dir

console = Console()

# Filesystem types that indicate network mounts
NETWORK_FS_TYPES = {"smbfs", "nfs", "nfs4", "afpfs", "cifs", "9p", "fuse.sshfs"}

# Directories/names to skip entirely
SKIP_NAMES_MACOS = {
    "Macintosh HD",
    "com.apple.TimeMachine.localsnapshots",
    ".timemachine",
    ".MobileBackups",
}
SKIP_MARKERS = {"Backups.backupdb", ".timemachine"}
SKIP_PATH_PARTS = {
    ".timemachine",
    ".MobileBackups",
    "Backups.backupdb",
    "com.apple.TimeMachine.localsnapshots",
}


def _wizard_preset_names() -> list[str]:
    """Return user-facing preset names, excluding test-only presets."""
    workspace = resolve_workspace_root()
    return [
        name for name in list_presets(workspace=workspace)
        if not name.startswith("test-")
    ]


def _parse_mount_types() -> dict[str, str]:
    """Parse mount output to get {mount_point: fs_type} mapping."""
    result = {}
    try:
        output = subprocess.check_output(["mount"], text=True, stderr=subprocess.DEVNULL)
        for line in output.splitlines():
            # macOS: /dev/disk4s1 on /Volumes/KINGSTON (msdos, local, nodev, ...)
            # Linux: /dev/sdb1 on /media/user/KINGSTON type vfat (rw,nosuid,...)
            if " on " not in line:
                continue
            rest = line.split(" on ", 1)[1]
            if " type " in rest:
                # Linux format
                mount_point, remainder = rest.split(" type ", 1)
                fs_type = remainder.split()[0].rstrip(",")
            elif " (" in rest:
                # macOS format
                mount_point = rest.rsplit(" (", 1)[0]
                fs_type = rest.rsplit(" (", 1)[1].split(",")[0].rstrip(")")
            else:
                continue
            result[mount_point.strip()] = fs_type.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        pass
    return result


def _should_skip_volume(path: Path) -> bool:
    """Check if a volume looks like a system or Time Machine mount."""
    if path.name in SKIP_NAMES_MACOS:
        return True
    if any(part in SKIP_PATH_PARTS for part in path.parts):
        return True
    return any((path / marker).exists() for marker in SKIP_MARKERS)


def detect_volumes() -> list[dict]:
    """Detect mounted volumes with size info and classification.

    Returns volumes sorted: local/USB first (ascending by size), network last.
    """
    mount_types = _parse_mount_types()
    candidates: list[Path] = []

    system = platform.system()
    if system == "Darwin":
        volumes_path = Path("/Volumes")
        if volumes_path.exists():
            for v in volumes_path.iterdir():
                if _should_skip_volume(v):
                    continue
                candidates.append(v)
    else:
        # Linux: check /media/$USER/ and /mnt/
        media_user = Path(f"/media/{os.getenv('USER', '')}")
        if media_user.exists():
            candidates.extend(media_user.iterdir())
        mnt = Path("/mnt")
        if mnt.exists():
            for v in mnt.iterdir():
                if v.is_mount():
                    candidates.append(v)

    volumes = []
    for v in candidates:
        if _should_skip_volume(v):
            continue
        try:
            usage = shutil.disk_usage(v)
        except (PermissionError, OSError):
            continue

        fs_type = mount_types.get(str(v), "")
        is_network = fs_type in NETWORK_FS_TYPES

        volumes.append({
            "path": str(v),
            "name": v.name,
            "total_gb": usage.total / 1e9,
            "free_gb": usage.free / 1e9,
            "network": is_network,
        })

    # Sort: local drives first (ascending by size), then network drives
    volumes.sort(key=lambda v: (v["network"], v["total_gb"]))
    return volumes


def presets_for_space(free_gb: float, region: str = "finland") -> list[tuple[str, float, bool]]:
    """Return all presets for region, sorted smallest first.

    Uses actual content size (not label) so a 122 GB drive can fit finland-128.
    Returns [(preset_name, content_size_gb, fits), ...].
    """
    workspace = resolve_workspace_root()
    available = _wizard_preset_names()
    result = []
    for preset_name in available:
        preset = load_preset(preset_name, workspace=workspace)
        if preset.kind != "preset" or preset.region != region:
            continue
        content_gb = sum(s.size_gb for s in preset.sources)
        result.append((preset_name, content_gb, content_gb <= free_gb))
    result.sort(key=lambda x: x[1])
    return result


def available_regions() -> list[str]:
    """Return canonical preset regions discovered from preset files."""
    workspace = resolve_workspace_root()
    return sorted({
        p.region for name in _wizard_preset_names()
        if (p := load_preset(name, workspace=workspace)).kind == "preset" and p.region
    })


def local_sources_for_space(
    free_gb: float, root: Path | str | None = None
) -> list[tuple[object, float, bool]]:
    """Return discovered local sources with fit information."""
    result = []
    for source in load_local_sources(root):
        size_gb = source.size_bytes / 1e9 if source.size_bytes else source.size_gb
        result.append((source, size_gb, size_gb <= free_gb))
    result.sort(key=lambda item: item[1])
    return result


def load_pack_presets(workspace: Path | str | None = None) -> list:
    """Load all pack presets available in the workspace."""
    workspace_root = resolve_workspace_root(workspace)
    packs = []
    for name in list_presets(workspace=workspace_root):
        try:
            preset = load_preset(name, workspace=workspace_root)
        except (FileNotFoundError, ValueError, KeyError):
            continue
        if preset.kind == "pack":
            packs.append(preset)
    return sorted(packs, key=lambda preset: (preset.display_group or "Other", preset.name))


def pick_sources_via_packs(
    *,
    free_gb: float,
    workspace: Path | str | None = None,
    checked_ids: set[str] | None = None,
) -> set[str]:
    """Open the interactive pack picker with optional pre-checked source ids."""
    checked_ids = checked_ids or set()
    packs = load_pack_presets(workspace)
    pack_source_ids = {source.id for pack in packs for source in pack.sources}
    hidden_checked_ids = checked_ids - pack_source_ids
    tree = build_picker_tree(packs, checked_ids=checked_ids)
    return run_picker(tree, free_gb=free_gb) | hidden_checked_ids


def write_custom_preset(
    selected_ids: set[str],
    *,
    workspace: Path | str | None = None,
    target_size_gb: float = 0,
    region: str = "",
    description: str | None = None,
    timestamp: str | None = None,
) -> str:
    """Persist a picker selection as a workspace-local preset and return its name."""
    workspace_root = resolve_workspace_root(workspace)
    preset_name = f"custom-{timestamp or datetime.now().strftime('%Y%m%d-%H%M%S')}"
    preset_path = local_presets_dir(workspace_root) / f"{preset_name}.yaml"
    preset_path.parent.mkdir(parents=True, exist_ok=True)
    preset_path.write_text(
        yaml.safe_dump(
            {
                "name": preset_name,
                "kind": "preset",
                "description": description or f"Custom selection — {len(selected_ids)} sources",
                "target_size_gb": target_size_gb,
                "region": region,
                "sources": sorted(selected_ids),
            },
            sort_keys=False,
            allow_unicode=True,
        )
    )
    return preset_name


def _clear():
    """Clear terminal screen."""
    console.clear()


def _width() -> int:
    """Return a clamped width for wizard UI elements (80-120)."""
    return max(80, min(120, console.width - 2))


def run_wizard(
    target_path: str | None = None,
    preset_name: str | None = None,
    browse_only: bool = False,
    workspace: Path | str | None = None,
):
    """Run the interactive setup wizard."""
    _clear()
    console.print(Panel(
        "[bold]Svalbard — Seed Vault for Human Knowledge[/bold]\n\n"
        "This wizard will help you set up an offline knowledge drive.",
        style="blue",
        width=60,
    ))

    # Step 1: Target
    if target_path is None:
        console.print("\n[bold]Step 1/4 — Target[/bold]")
        console.print("Where should the kit be provisioned?\n")

        volumes = detect_volumes()
        choices = {}
        idx = 1

        vol_table = Table(show_header=False, box=None, padding=(0, 1))
        vol_table.add_column("#", width=3, no_wrap=True)
        vol_table.add_column("Path", no_wrap=True)
        vol_table.add_column("Space", no_wrap=True)

        for v in volumes:
            svalbard_path = str(Path(v["path"]) / "svalbard")
            size_info = f"{v['free_gb']:.0f}/{v['total_gb']:.0f} GB"
            style = "dim" if v["network"] else ""
            label = f"{svalbard_path}  [network]" if v["network"] else svalbard_path
            vol_table.add_row(f"[bold]{idx}[/bold]", label, size_info, style=style)
            choices[str(idx)] = svalbard_path
            idx += 1

        # Home directory option
        home_svalbard = Path.home() / "svalbard"
        home_size = ""
        if home_svalbard.exists():
            try:
                usage = shutil.disk_usage(home_svalbard)
                home_size = f"{usage.free / 1e9:.0f} GB free"
            except OSError:
                pass
        vol_table.add_row(f"[bold]{idx}[/bold]", "~/svalbard/", home_size or "home directory")
        choices[str(idx)] = str(home_svalbard)
        idx += 1

        vol_table.add_row("[bold]c[/bold]", "Custom path...", "")
        console.print(vol_table)

        valid_choices = list(choices.keys()) + ["c"]
        choice = Prompt.ask("\n  Select", choices=valid_choices)
        if choice == "c":
            target_path = Prompt.ask("  Enter path")
        else:
            target_path = choices[choice]
    else:
        console.print(f"\n  [bold]Target:[/bold] {target_path}")

    # Check free space
    space_check = Path(target_path)
    while not space_check.exists() and space_check.parent != space_check:
        space_check = space_check.parent
    try:
        usage = shutil.disk_usage(space_check)
        free_gb = usage.free / 1e9
    except OSError:
        free_gb = 0

    workspace_root = resolve_workspace_root(workspace)
    selected_region = ""
    checked_ids: set[str] = set()

    if preset_name is not None:
        base_preset = load_preset(preset_name, workspace=workspace_root)
        selected_region = base_preset.region
        checked_ids = {source.id for source in base_preset.sources}
    elif not browse_only:
        _clear()
        console.print("\n[bold]Step 2 — Configure[/bold]")
        console.print("  How would you like to set up this drive?\n")
        console.print("  [bold]1[/bold]) Use a preset [dim](recommended)[/dim]")
        console.print("      Pre-configured for your drive size and region")
        console.print("  [bold]2[/bold]) Customize")
        console.print("      Browse all content and pick what you want")

        mode = Prompt.ask("\n  Select", choices=["1", "2"], default="1")
        if mode == "1":
            _clear()
            console.print("\n[bold]Step 3 — Region[/bold]")

            regions = available_regions()
            if not regions:
                console.print("[red]No preset regions available.[/red]")
                return

            region_choices = {str(i): region for i, region in enumerate(regions, 1)}
            for key, region in region_choices.items():
                console.print(f"  [bold]{key}[/bold]) {region}")

            default_region = "finland" if "finland" in regions else regions[0]
            default_region_key = next(
                key for key, region in region_choices.items() if region == default_region
            )
            region_choice = Prompt.ask(
                "\n  Select",
                choices=list(region_choices.keys()),
                default=default_region_key,
            )
            selected_region = region_choices[region_choice]

            _clear()
            console.print("\n[bold]Step 4 — Preset[/bold]")
            all_presets = presets_for_space(free_gb, region=selected_region) if free_gb > 0 else []

            if not all_presets:
                if free_gb > 0:
                    console.print(f"[red]No presets available for region '{selected_region}'.[/red]")
                else:
                    console.print("[red]Could not determine free space at target.[/red]")
                return

            console.print(f"  Presets ({free_gb:.0f} GB free):\n")
            preset_choices = {}
            recommended = None

            preset_table = Table(show_header=False, box=None, padding=(0, 1), width=_width())
            preset_table.add_column("#", width=3, no_wrap=True)
            preset_table.add_column("Name", width=14, no_wrap=True)
            preset_table.add_column("Size", width=7, no_wrap=True, justify="right")
            preset_table.add_column("Description", ratio=1)

            for i, (name, content_gb, fits) in enumerate(all_presets, 1):
                preset = load_preset(name, workspace=workspace_root)
                preset_choices[str(i)] = name
                style = "" if fits else "dim"
                if fits:
                    recommended = str(i)
                desc = preset.description
                if not fits:
                    desc += f" (needs ~{content_gb - free_gb:.0f} GB more)"
                preset_table.add_row(
                    f"[bold]{i}[/bold]",
                    name,
                    f"~{content_gb:.0f} GB",
                    desc,
                    style=style,
                )

            console.print(preset_table)
            if recommended:
                console.print(
                    f"\n  [green]Enter = {preset_choices[recommended]} (recommended)[/green]"
                )
            choice = Prompt.ask(
                "\n  Select",
                choices=list(preset_choices.keys()),
                default=recommended or "1",
            )
            preset_name = preset_choices[choice]
            base_preset = load_preset(preset_name, workspace=workspace_root)
            checked_ids = {source.id for source in base_preset.sources}

    _clear()
    console.print("\n[bold]Step 5 — Pack Picker[/bold]")
    selected_ids = pick_sources_via_packs(
        free_gb=free_gb,
        workspace=workspace_root,
        checked_ids=checked_ids,
    )
    preset_name = write_custom_preset(
        selected_ids,
        workspace=workspace_root,
        target_size_gb=free_gb,
        region=selected_region,
    )
    preset = load_preset(preset_name, workspace=workspace_root)

    selected_local_ids: list[str] = []
    selected_local_sources = []
    remaining_gb = max(free_gb - sum(s.size_gb for s in preset.sources), 0)
    local_candidates = local_sources_for_space(remaining_gb, root=workspace_root)
    if local_candidates:
        _clear()
        console.print("\n[bold]Step 4/5 — Local Sources[/bold]")
        console.print(f"  Optional local sources ({remaining_gb:.1f} GB remaining):\n")
        local_choices: dict[str, str] = {}

        local_table = Table(show_header=False, box=None, padding=(0, 1), width=_width())
        local_table.add_column("#", width=3, no_wrap=True)
        local_table.add_column("Source", width=18, no_wrap=True)
        local_table.add_column("Size", width=7, no_wrap=True, justify="right")
        local_table.add_column("Description", ratio=1)

        for i, (source, size_gb, fits) in enumerate(local_candidates, 1):
            style = "" if fits else "dim"
            desc = source.description or source.id
            if not fits:
                desc += " (too large)"
            local_table.add_row(f"[bold]{i}[/bold]", source.id, f"~{size_gb:.1f} GB", desc, style=style)
            local_choices[str(i)] = source.id

        console.print(local_table)

        raw = Prompt.ask(
            "\n  Select extra local sources (comma-separated, blank for none)",
            default="",
            show_default=False,
        ).strip()
        if raw:
            for part in [p.strip() for p in raw.split(",") if p.strip()]:
                source_id = local_choices.get(part)
                if source_id:
                    selected_local_ids.append(source_id)
            selected_local_sources = [
                source for source, _, _ in local_candidates if source.id in selected_local_ids
            ]

    # Step 5: Review
    sources = [*preset.sources, *selected_local_sources]
    total_gb = sum(s.size_gb for s in sources)

    _clear()
    console.print(f"\n[bold]Step 5/5 — Review[/bold]")

    table = Table(show_header=True, header_style="bold", width=_width())
    table.add_column("Source", no_wrap=True)
    table.add_column("Type", no_wrap=True)
    table.add_column("Size", justify="right", no_wrap=True)
    table.add_column("Description", ratio=1)

    by_type: dict[str, list] = {}
    for s in sources:
        by_type.setdefault(s.type, []).append(s)

    for type_name in sorted(by_type):
        type_sources = by_type[type_name]
        for s in sorted(type_sources, key=lambda s: s.id):
            size_str = f"{s.size_gb:.1f} GB" if s.size_gb >= 1 else f"{s.size_gb * 1024:.0f} MB"
            table.add_row(s.id, s.type.upper(), size_str, s.description or "")

    console.print(table)
    console.print(f"\n  [bold]Target:[/bold]  {target_path}")
    console.print(f"  [bold]Preset:[/bold]  {preset.name} — {preset.description}")
    console.print(f"  [bold]Total:[/bold]   {len(sources)} sources, {total_gb:.1f} GB / {free_gb:.0f} GB free")

    if not Confirm.ask("\n  Proceed?", default=True):
        console.print("[dim]Cancelled.[/dim]")
        return

    init_drive(
        target_path,
        preset_name,
        workspace_root=str(workspace_root),
        local_sources=selected_local_ids,
    )
    _clear()
    console.print(f"\n  [green]Initialized:[/green] {target_path}")
    console.print(f"  [bold]Preset:[/bold]  {preset.name} — {len(sources)} sources, {total_gb:.1f} GB\n")
    if Confirm.ask("  Start downloading now?", default=True):
        _clear()
        sync_drive(target_path)

        # Offer to build search index after sync
        from pathlib import Path as P

        zim_dir = P(target_path) / "zim"
        zim_count = len(list(zim_dir.glob("*.zim"))) if zim_dir.exists() else 0
        if zim_count > 0:
            console.print(f"\n[bold]Search Index[/bold]")
            console.print(f"  {zim_count} ZIM files available for cross-ZIM search.\n")
            console.print("  [bold]1[/bold]) Fast — keyword search (quick, small index)")
            console.print("  [bold]2[/bold]) Standard — full-text search (slower, better ranking)")
            console.print("  [bold]3[/bold]) Semantic — keyword + meaning (understands synonyms and related concepts)")
            console.print("  [bold]4[/bold]) Skip for now (run [bold]svalbard index[/bold] later)")

            from rich.prompt import Prompt as P2

            index_choice = P2.ask("\n  Select", choices=["1", "2", "3", "4"], default="4")
            if index_choice in ("1", "2", "3"):
                from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn

                from svalbard.indexer import run_index, estimate_index
                from svalbard.search_db import SearchDB

                strategy = {"1": "fast", "2": "standard", "3": "semantic"}[index_choice]
                drive = P(target_path)
                data_dir = drive / "data"
                data_dir.mkdir(parents=True, exist_ok=True)
                db = SearchDB(data_dir / "search.db")
                try:
                    plan = estimate_index(drive, db, strategy=strategy)
                    total = plan.estimated_articles or plan.articles_to_embed or 1

                    with Progress(
                        SpinnerColumn(),
                        TextColumn("[progress.description]{task.description}"),
                        BarColumn(bar_width=20),
                        TextColumn("{task.completed}/{task.total}"),
                        console=Console(width=min(120, console.width)),
                    ) as progress:
                        task = progress.add_task(f"Indexing ({strategy})", total=total)

                        def on_progress(phase: str, done: int, total: int):
                            progress.update(task, description=f"[cyan]{phase}[/cyan]", completed=done, total=total)

                        run_index(drive, db, strategy=strategy, on_progress=on_progress)

                    stats = db.stats()
                    console.print(
                        f"\n  [green]Done.[/green] {stats['source_count']} sources, "
                        f"{stats['article_count']} articles indexed."
                    )
                except ImportError as e:
                    console.print(f"\n  [yellow]Indexing skipped:[/yellow] {e}")
                    console.print("  Install libzim to enable indexing: pip install libzim")
