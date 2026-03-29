import os
import platform
import shutil
import subprocess
from pathlib import Path

from rich.console import Console
from rich.panel import Panel
from rich.prompt import Confirm, Prompt
from rich.table import Table

from svalbard.commands import init_drive, sync_drive
from svalbard.presets import list_presets, load_preset

console = Console()

# Filesystem types that indicate network mounts
NETWORK_FS_TYPES = {"smbfs", "nfs", "nfs4", "afpfs", "cifs", "9p", "fuse.sshfs"}

# Directories/names to skip entirely
SKIP_NAMES_MACOS = {"Macintosh HD", "com.apple.TimeMachine.localsnapshots"}
SKIP_MARKERS = {"Backups.backupdb", ".timemachine"}


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


def _is_time_machine(path: Path) -> bool:
    """Check if a volume looks like a Time Machine backup."""
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
                if v.name in SKIP_NAMES_MACOS:
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
        if _is_time_machine(v):
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
    available = list_presets()
    result = []
    for preset_name in available:
        preset = load_preset(preset_name)
        if preset.region != region:
            continue
        content_gb = sum(s.size_gb for s in preset.sources)
        result.append((preset_name, content_gb, content_gb <= free_gb))
    result.sort(key=lambda x: x[1])
    return result


def available_regions() -> list[str]:
    """Return canonical preset regions discovered from preset files."""
    return sorted({load_preset(name).region for name in list_presets()})


def run_wizard():
    """Run the interactive setup wizard."""
    console.print(Panel(
        "[bold]Svalbard — Seed Vault for Human Knowledge[/bold]\n\n"
        "This wizard will help you set up an offline knowledge drive.",
        style="blue",
    ))

    # Step 1: Target
    console.print("\n[bold]Step 1/4 — Target[/bold]")
    console.print("Where should the kit be provisioned?\n")

    volumes = detect_volumes()
    choices = {}
    idx = 1
    for v in volumes:
        svalbard_path = str(Path(v["path"]) / "svalbard")
        size_info = f"{v['total_gb']:.0f} GB total, {v['free_gb']:.0f} GB free"
        if v["network"]:
            console.print(f"  [dim][bold]{idx}[/bold]) {svalbard_path}  [network]  ({size_info})[/dim]")
        else:
            console.print(f"  [bold]{idx}[/bold]) {svalbard_path}  ({size_info})")
        choices[str(idx)] = svalbard_path
        idx += 1

    # Home directory option — always present
    home_svalbard = Path.home() / "svalbard"
    home_label = "~/svalbard/"
    if home_svalbard.exists():
        try:
            usage = shutil.disk_usage(home_svalbard)
            home_label += f"  ({usage.free / 1e9:.0f} GB free)"
        except OSError:
            pass
    else:
        home_label += "  (home directory)"
    console.print(f"  [bold]{idx}[/bold]) {home_label}")
    choices[str(idx)] = str(home_svalbard)
    idx += 1

    console.print(f"  [bold]c[/bold]) Custom path...")

    valid_choices = list(choices.keys()) + ["c"]
    choice = Prompt.ask("\n  Select", choices=valid_choices)
    if choice == "c":
        target_path = Prompt.ask("  Enter path")
    else:
        target_path = choices[choice]

    # Step 2: Region
    console.print("\n[bold]Step 2/4 — Region[/bold]")
    # Check free space — use parent if target doesn't exist yet
    space_check = Path(target_path)
    while not space_check.exists() and space_check.parent != space_check:
        space_check = space_check.parent
    try:
        usage = shutil.disk_usage(space_check)
        free_gb = usage.free / 1e9
    except OSError:
        free_gb = 0

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

    # Step 3: Preset
    console.print("\n[bold]Step 3/4 — Preset[/bold]")
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
    for i, (name, content_gb, fits) in enumerate(all_presets, 1):
        p = load_preset(name)
        preset_choices[str(i)] = name
        if fits:
            recommended = str(i)
            over = ""
            marker = ""
        else:
            over = f"  (needs ~{content_gb - free_gb:.0f} GB more)"
            marker = "[dim]"

        line = f"  [bold]{i}[/bold]) {marker}{name:15s}  ~{content_gb:.0f} GB  — {p.description}{over}"
        if marker:
            line += "[/dim]"
        console.print(line)

    # Mark recommended after printing all lines
    if recommended:
        console.print(f"\n  [green]Enter = {preset_choices[recommended]} (recommended)[/green]")

    default_choice = recommended or "1"
    choice = Prompt.ask("\n  Select", choices=list(preset_choices.keys()), default=default_choice)
    preset_name = preset_choices[choice]
    preset = load_preset(preset_name)

    # Step 4: Review
    sources = preset.sources
    total_gb = sum(s.size_gb for s in sources)

    console.print(f"\n[bold]Step 4/4 — Review[/bold]")

    table = Table(show_header=True, header_style="bold")
    table.add_column("Source")
    table.add_column("Type")
    table.add_column("Size", justify="right")
    table.add_column("Description")

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

    init_drive(target_path, preset_name)
    if Confirm.ask("\n  Start downloading now?", default=True):
        sync_drive(target_path)
