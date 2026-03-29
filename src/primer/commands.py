from datetime import datetime
from pathlib import Path

from rich.console import Console
from rich.table import Table

from primer.downloader import download_sources
from primer.manifest import Manifest, ManifestEntry
from primer.models import Source
from primer.presets import load_preset
from primer.resolver import resolve_url
from primer.taxonomy import compute_coverage, load_taxonomy

console = Console()

TYPE_DIRS = {
    "zim": "zim",
    "pmtiles": "maps",
    "pdf": "books",
    "epub": "books",
    "gguf": "models",
    "binary": "bin",
    "app": "apps",
    "iso": "infra",
}


def init_drive(path: str, preset_name: str, enabled_groups: set[str] | None = None):
    """Initialize a drive with a preset."""
    drive_path = Path(path)
    drive_path.mkdir(parents=True, exist_ok=True)

    preset = load_preset(preset_name)
    if enabled_groups is None:
        enabled_groups = {"maps"}

    sources = preset.sources_for_options(enabled_groups)

    manifest = Manifest(
        preset=preset_name,
        region=preset.region,
        target_path=str(drive_path),
        created=datetime.now().isoformat(timespec="seconds"),
    )
    manifest.save(drive_path / "manifest.yaml")

    console.print(f"[bold green]Initialized:[/bold green] {drive_path}")
    console.print(f"  Preset: {preset.name}")
    console.print(f"  Sources: {len(sources)}")
    console.print(f"  Estimated size: {sum(s.size_gb for s in sources):.1f} GB")
    console.print(f"\nRun [bold]primer sync[/bold] to download content.")


def sync_drive(path: str):
    """Download/update content on an initialized drive."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found. Run primer init or primer wizard first.[/red]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)

    console.print(f"[bold]Syncing:[/bold] {manifest.preset} -> {drive_path}")
    console.print("\n[bold]Resolving latest versions...[/bold]")

    downloads = []
    for source in preset.sources:
        try:
            url = resolve_url(source)
            dest_dir = drive_path / TYPE_DIRS.get(source.type, "other")
            downloads.append((source.id, url, dest_dir))
            console.print(f"  [green]OK[/green] {source.id}")
        except Exception as e:
            console.print(f"  [red]FAIL[/red] {source.id}: {e}")

    console.print(f"\n[bold]Downloading {len(downloads)} files...[/bold]")
    results = download_sources(downloads)

    for r in results:
        if r.success and r.filepath:
            source = next((s for s in preset.sources if s.id == r.source_id), None)
            if source:
                entry = ManifestEntry(
                    id=r.source_id,
                    type=source.type,
                    filename=r.filepath.name,
                    size_bytes=r.filepath.stat().st_size,
                    tags=source.tags,
                    depth=source.depth,
                    downloaded=datetime.now().isoformat(timespec="seconds"),
                    url=str(r.filepath),
                )
                manifest.entries = [e for e in manifest.entries if e.id != r.source_id]
                manifest.entries.append(entry)

    manifest.last_synced = datetime.now().isoformat(timespec="seconds")
    manifest.save(drive_path / "manifest.yaml")

    succeeded = sum(1 for r in results if r.success)
    failed = sum(1 for r in results if not r.success)
    console.print(f"\n[bold green]Done:[/bold green] {succeeded} downloaded, {failed} failed.")


def show_status(path: str):
    """Show drive status."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[dim]No primer drive found at this path.[/dim]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")

    table = Table(title=f"Primer -- {manifest.preset}")
    table.add_column("ID", style="cyan")
    table.add_column("Type")
    table.add_column("Size", justify="right")
    table.add_column("Downloaded")

    total_bytes = 0
    for e in sorted(manifest.entries, key=lambda x: x.id):
        size_str = f"{e.size_bytes / 1e9:.1f} GB" if e.size_bytes > 1e9 else f"{e.size_bytes / 1e6:.0f} MB"
        table.add_row(e.id, e.type, size_str, e.downloaded[:10] if e.downloaded else "--")
        total_bytes += e.size_bytes

    console.print(table)
    console.print(f"\n  Total: {total_bytes / 1e9:.1f} GB | Last synced: {manifest.last_synced or '--'}")
