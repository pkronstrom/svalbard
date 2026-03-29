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

SIZE_PRESETS = {
    32: "nordic-32",
    64: "nordic-64",
    128: "nordic-128",
    256: "nordic-256",
    512: "nordic-512",
    1024: "nordic-1tb",
    2048: "nordic-2tb",
}

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


def find_best_preset(size_gb: float, region: str = "nordic") -> str | None:
    """Find the largest preset that fits the given size."""
    available = list_presets()
    best = None
    for size, preset_name in sorted(SIZE_PRESETS.items()):
        if preset_name in available and size <= size_gb:
            best = preset_name
    return best


def run_wizard():
    """Run the interactive setup wizard."""
    console.print(Panel(
        "[bold]Svalbard — Seed Vault for Human Knowledge[/bold]\n\n"
        "This wizard will help you set up an offline knowledge drive.",
        style="blue",
    ))

    # Step 1: Target
    console.print("\n[bold]Step 1/5 — Target[/bold]")
    console.print("Where should the kit be provisioned?\n")

    volumes = detect_volumes()
    choices = {}
    idx = 1
    for v in volumes:
        size_info = f"{v['total_gb']:.0f} GB total, {v['free_gb']:.0f} GB free"
        if v["network"]:
            console.print(f"  [dim][bold]{idx}[/bold]) {v['path']}  [network]  ({size_info})[/dim]")
        else:
            console.print(f"  [bold]{idx}[/bold]) {v['path']}  ({size_info})")
        choices[str(idx)] = v["path"]
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

    # Step 2: Budget
    console.print("\n[bold]Step 2/5 — Budget[/bold]")
    try:
        usage = shutil.disk_usage(target_path)
        default_gb = int(usage.total / 1e9 * 0.9)
    except OSError:
        default_gb = 128
    budget_gb = int(Prompt.ask(
        f"  How much space to use (GB)?", default=str(default_gb),
    ))

    # Step 3: Region
    console.print("\n[bold]Step 3/5 — Region[/bold]")
    console.print("  [bold]1[/bold]) Nordic (Finland, Nordics, Northern Europe)")
    console.print("  [dim]2) US (coming soon)[/dim]")
    console.print("  [dim]3) Global (coming soon)[/dim]")
    Prompt.ask("  Select", default="1", choices=["1"])

    # Find best preset
    preset_name = find_best_preset(budget_gb)
    if not preset_name:
        console.print(f"[red]No preset found for {budget_gb} GB. Minimum is 32 GB.[/red]")
        return
    preset = load_preset(preset_name)

    # Step 4: Options
    console.print(f"\n[bold]Step 4/5 — Options[/bold] (preset: {preset.name})")
    enabled_groups = set()
    if Confirm.ask("  Include offline maps?", default=True):
        enabled_groups.add("maps")
    if budget_gb >= 512:
        if Confirm.ask("  Include LLM models?", default=True):
            enabled_groups.add("models")
        if Confirm.ask("  Include app installers (Kiwix.dmg, etc.)?", default=False):
            enabled_groups.add("installers")
        if Confirm.ask("  Include Linux ISO + package cache?", default=False):
            enabled_groups.add("infra")

    # Step 5: Review
    sources = preset.sources_for_options(enabled_groups)
    total_gb = sum(s.size_gb for s in sources)

    console.print(f"\n[bold]Step 5/5 — Review[/bold]")
    table = Table()
    table.add_column("Category")
    table.add_column("Sources", justify="right")
    table.add_column("Size", justify="right")
    by_type: dict[str, list] = {}
    for s in sources:
        by_type.setdefault(s.type, []).append(s)
    for type_name, type_sources in sorted(by_type.items()):
        size = sum(s.size_gb for s in type_sources)
        table.add_row(type_name.upper(), str(len(type_sources)), f"{size:.1f} GB")
    console.print(table)
    console.print(f"\n  [bold]Target:[/bold]  {target_path}")
    console.print(f"  [bold]Preset:[/bold]  {preset.name}")
    console.print(f"  [bold]Total:[/bold]   {total_gb:.1f} GB / {budget_gb} GB")

    if not Confirm.ask("\n  Proceed?", default=True):
        console.print("[dim]Cancelled.[/dim]")
        return

    init_drive(target_path, preset_name, enabled_groups)
    if Confirm.ask("\n  Start downloading now?", default=True):
        sync_drive(target_path)
