import signal
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

from rich.console import Console
from rich.table import Table

from svalbard.downloader import download_sources, fetch_sha256_sidecar
from svalbard.manifest import Manifest, ManifestEntry
from svalbard.models import Source
from svalbard.presets import load_preset
from svalbard.readme_generator import generate_drive_readme
from svalbard.resolver import resolve_url
from svalbard.serve_generator import generate_serve_sh
from svalbard.taxonomy import compute_coverage, load_taxonomy

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


@dataclass
class DownloadJob:
    source_id: str
    source_type: str
    url: str
    dest_dir: Path
    source: Source
    platform: str = ""


def expand_source_downloads(source: Source, drive_path: Path) -> list[DownloadJob]:
    """Expand one source into one or more concrete download jobs."""
    if source.platforms:
        jobs = []
        for platform, url in sorted(source.platforms.items()):
            jobs.append(
                DownloadJob(
                    source_id=source.id,
                    source_type=source.type,
                    url=url,
                    dest_dir=drive_path / "bin" / platform,
                    source=source,
                    platform=platform,
                )
            )
        return jobs

    return [
        DownloadJob(
            source_id=source.id,
            source_type=source.type,
            url=resolve_url(source),
            dest_dir=drive_path / TYPE_DIRS.get(source.type, "other"),
            source=source,
        )
    ]


def init_drive(path: str, preset_name: str):
    """Initialize a drive with a preset."""
    drive_path = Path(path)
    drive_path.mkdir(parents=True, exist_ok=True)

    preset = load_preset(preset_name)
    sources = preset.sources

    manifest = Manifest(
        preset=preset.name,
        region=preset.region,
        target_path=str(drive_path),
        created=datetime.now().isoformat(timespec="seconds"),
    )
    manifest.save(drive_path / "manifest.yaml")

    generate_serve_sh(drive_path)
    generate_drive_readme(drive_path)

    console.print(f"[bold green]Initialized:[/bold green] {drive_path}")
    console.print(f"  Preset: {preset.name}")
    console.print(f"  Sources: {len(sources)}")
    console.print(f"  Estimated size: {sum(s.size_gb for s in sources):.1f} GB")
    console.print(f"\nRun [bold]svalbard sync[/bold] to download content.")


def sync_drive(path: str, update: bool = False, force: bool = False):
    """Download/update content on an initialized drive."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found. Run svalbard init or svalbard wizard first.[/red]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)
    manifest_path = drive_path / "manifest.yaml"

    console.print(f"[bold]Syncing:[/bold] {manifest.preset} → {drive_path}")

    active_sources = preset.sources

    # Determine what needs downloading
    console.print("\n[bold]Resolving latest versions...[/bold]")
    downloads: list[DownloadJob] = []
    skipped = 0
    updated = 0

    for source in active_sources:
        try:
            jobs = expand_source_downloads(source, drive_path)
        except Exception as e:
            console.print(f"  [red]FAIL[/red] {source.id}: {e}")
            continue

        for job in jobs:
            existing = manifest.entry_by_id(job.source_id, job.platform)

            if existing and not force:
                existing_path = job.dest_dir / existing.filename
                if existing_path.exists():
                    if not update:
                        skipped += 1
                        continue
                    if job.url == existing.url:
                        label = f"{source.id} [{job.platform}]" if job.platform else source.id
                        console.print(f"  [dim]Current:[/dim] {label}")
                        skipped += 1
                        continue

                    label = f"{source.id} [{job.platform}]" if job.platform else source.id
                    console.print(f"  [yellow]Update available:[/yellow] {label}")
                    updated += 1
                    downloads.append(job)
                    continue

                label = f"{source.id} [{job.platform}]" if job.platform else source.id
                console.print(f"  [yellow]Missing from disk:[/yellow] {label}")

            downloads.append(job)
            label = f"{source.id} [{job.platform}]" if job.platform else source.id
            console.print(f"  [green]OK[/green] {label}")

    if skipped:
        console.print(f"  [dim]{skipped} already downloaded (use --update to check for newer versions)[/dim]")

    if not downloads:
        console.print("\n[bold green]Everything up to date.[/bold green]")
        manifest.last_synced = datetime.now().isoformat(timespec="seconds")
        manifest.save(manifest_path)
        return

    # Collect checksums: from preset yaml or .sha256 sidecars
    checksums: dict[str, str] = {}
    for job in downloads:
        checksum_key = f"{job.source_id}:{job.platform}"
        if job.source.sha256:
            checksums[checksum_key] = job.source.sha256
        elif job.source.type == "zim":
            sidecar = fetch_sha256_sidecar(job.url)
            if sidecar:
                checksums[checksum_key] = sidecar
                console.print(f"  [dim]Checksum fetched: {job.source_id}[/dim]")

    console.print(f"\n[bold]Downloading {len(downloads)} file(s)...[/bold]")

    # Download with crash-safe manifest saving
    interrupted = False

    def _handle_interrupt(sig, frame):
        nonlocal interrupted
        interrupted = True
        console.print("\n[yellow]Interrupted — saving progress...[/yellow]")

    old_handler = signal.signal(signal.SIGINT, _handle_interrupt)

    try:
        for job in downloads:
            if interrupted:
                break

            try:
                source_checksums = {}
                checksum_key = f"{job.source_id}:{job.platform}"
                if checksum_key in checksums:
                    source_checksums[job.source_id] = checksums[checksum_key]
                results = download_sources(
                    [(job.source_id, job.url, job.dest_dir)],
                    checksums=source_checksums,
                )
                r = results[0]

                if r.success and r.filepath:
                    # Delete old file if this is an update with a different filename
                    existing = manifest.entry_by_id(job.source_id, job.platform)
                    if existing and existing.filename != r.filepath.name:
                        old_path = job.dest_dir / existing.filename
                        if old_path.exists():
                            old_path.unlink()
                            console.print(f"  [dim]Removed old: {existing.filename}[/dim]")

                    entry = ManifestEntry(
                        id=job.source_id,
                        type=job.source.type,
                        filename=r.filepath.name,
                        size_bytes=r.filepath.stat().st_size,
                        platform=job.platform,
                        tags=job.source.tags,
                        depth=job.source.depth,
                        downloaded=datetime.now().isoformat(timespec="seconds"),
                        url=job.url,
                        checksum_sha256=r.sha256,
                    )
                    manifest.entries = [
                        e for e in manifest.entries
                        if not (e.id == job.source_id and e.platform == job.platform)
                    ]
                    manifest.entries.append(entry)

                    # Save manifest after each successful download
                    manifest.last_synced = datetime.now().isoformat(timespec="seconds")
                    manifest.save(manifest_path)

            except Exception as e:
                console.print(f"  [red]Failed: {job.source_id}: {e}[/red]")
    finally:
        signal.signal(signal.SIGINT, old_handler)
        # Final save
        manifest.last_synced = datetime.now().isoformat(timespec="seconds")
        manifest.save(manifest_path)

    succeeded = sum(
        1
        for job in downloads
        if manifest.entry_by_id(job.source_id, job.platform)
        and manifest.entry_by_id(job.source_id, job.platform).downloaded >= manifest.last_synced[:10]
    )
    total = len(downloads)
    failed = total - succeeded
    if interrupted:
        console.print(f"\n[yellow]Interrupted:[/yellow] {succeeded}/{total} downloaded. Run svalbard sync to continue.")
    else:
        console.print(f"\n[bold green]Done:[/bold green] {succeeded} downloaded, {failed} failed.")


def show_status(path: str, check_updates: bool = False):
    """Show drive status."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[dim]No svalbard drive found at this path.[/dim]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)
    active_sources = preset.sources

    # Resolve URLs if checking for updates
    resolved_urls: dict[str, str] = {}
    if check_updates:
        console.print("[dim]Checking for updates...[/dim]")
        for source in active_sources:
            try:
                resolved_urls[source.id] = resolve_url(source)
            except Exception:
                pass

    table = Table(title=f"Svalbard — {manifest.preset}")
    table.add_column("ID", style="cyan")
    table.add_column("Type")
    table.add_column("Size", justify="right")
    table.add_column("Downloaded")
    table.add_column("Status")

    total_bytes = 0
    for source in sorted(active_sources, key=lambda s: s.id):
        if source.platforms:
            entries = [manifest.entry_by_id(source.id, platform) for platform in sorted(source.platforms)]
            present_entries = [entry for entry in entries if entry is not None]
            total_entry_bytes = sum(entry.size_bytes for entry in present_entries)
            total_bytes += total_entry_bytes
            size_str = (
                f"{total_entry_bytes / 1e9:.1f} GB"
                if total_entry_bytes > 1e9
                else f"{total_entry_bytes / 1e6:.0f} MB"
            ) if present_entries else f"~{source.size_gb:.1f} GB"
            date_str = max((entry.downloaded[:10] for entry in present_entries if entry.downloaded), default="--")
            missing_platforms = []
            for platform, entry in zip(sorted(source.platforms), entries):
                if entry is None:
                    missing_platforms.append(platform)
                    continue
                if not (drive_path / "bin" / platform / entry.filename).exists():
                    missing_platforms.append(platform)

            if missing_platforms:
                status = f"[yellow]△ {len(missing_platforms)} missing[/yellow]"
            else:
                status = "[green]✓[/green]"

            table.add_row(source.id, source.type, size_str, date_str, status)
            continue

        entry = manifest.entry_by_id(source.id)
        if entry:
            size_str = f"{entry.size_bytes / 1e9:.1f} GB" if entry.size_bytes > 1e9 else f"{entry.size_bytes / 1e6:.0f} MB"
            date_str = entry.downloaded[:10] if entry.downloaded else "--"
            total_bytes += entry.size_bytes

            # Check if file still exists on disk
            dest_dir = drive_path / TYPE_DIRS.get(source.type, "other")
            file_exists = (dest_dir / entry.filename).exists()

            if not file_exists:
                status = "[red]✗ file missing[/red]"
            elif check_updates and source.id in resolved_urls:
                if resolved_urls[source.id] != entry.url:
                    status = "[yellow]↑ update available[/yellow]"
                else:
                    status = "[green]✓ current[/green]"
            else:
                status = "[green]✓[/green]"

            table.add_row(source.id, source.type, size_str, date_str, status)
        else:
            size_str = f"~{source.size_gb:.1f} GB"
            table.add_row(source.id, source.type, size_str, "--", "[dim]✗ not downloaded[/dim]")

    console.print(table)
    console.print(f"\n  Total: {total_bytes / 1e9:.1f} GB | Last synced: {manifest.last_synced or '--'}")
