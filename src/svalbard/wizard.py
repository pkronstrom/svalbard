import shutil
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


def detect_volumes() -> list[dict]:
    """Detect mounted volumes with size info."""
    volumes = []
    volumes_path = Path("/Volumes")
    if not volumes_path.exists():
        return volumes
    for v in sorted(volumes_path.iterdir()):
        if v.name == "Macintosh HD":
            continue
        try:
            usage = shutil.disk_usage(v)
            volumes.append({
                "path": str(v),
                "name": v.name,
                "total_gb": usage.total / 1e9,
                "free_gb": usage.free / 1e9,
            })
        except (PermissionError, OSError):
            continue
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
    for i, v in enumerate(volumes, 1):
        label = f"{v['name']} ({v['total_gb']:.0f} GB total, {v['free_gb']:.0f} GB free)"
        console.print(f"  [bold]{i}[/bold]) {v['path']}  [dim]{label}[/dim]")
        choices[str(i)] = v["path"]
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
