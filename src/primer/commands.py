import signal
from datetime import datetime
from pathlib import Path

from rich.console import Console
from rich.table import Table

from primer.downloader import download_sources, fetch_sha256_sidecar
from primer.manifest import Manifest, ManifestEntry
from primer.models import Source
from primer.presets import load_preset
from primer.readme_generator import generate_drive_readme
from primer.resolver import resolve_url
from primer.serve_generator import generate_serve_sh
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
        enabled_groups=sorted(enabled_groups),
    )
    manifest.save(drive_path / "manifest.yaml")

    generate_serve_sh(drive_path)
    generate_drive_readme(drive_path)

    console.print(f"[bold green]Initialized:[/bold green] {drive_path}")
    console.print(f"  Preset: {preset.name}")
    console.print(f"  Sources: {len(sources)}")
    console.print(f"  Estimated size: {sum(s.size_gb for s in sources):.1f} GB")
    console.print(f"\nRun [bold]primer sync[/bold] to download content.")


def sync_drive(path: str, update: bool = False, force: bool = False):
    """Download/update content on an initialized drive."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found. Run primer init or primer wizard first.[/red]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)
    manifest_path = drive_path / "manifest.yaml"

    console.print(f"[bold]Syncing:[/bold] {manifest.preset} → {drive_path}")

    active_sources = preset.sources_for_options(set(manifest.enabled_groups))

    # Determine what needs downloading
    console.print("\n[bold]Resolving latest versions...[/bold]")
    downloads: list[tuple[str, str, Path, Source]] = []
    skipped = 0
    updated = 0

    for source in active_sources:
        existing = manifest.entry_by_id(source.id)
        dest_dir = drive_path / TYPE_DIRS.get(source.type, "other")

        if existing and not force:
            # File already downloaded — check if it still exists on disk
            existing_path = dest_dir / existing.filename
            if existing_path.exists():
                if not update:
                    # Default: skip already-downloaded sources
                    skipped += 1
                    continue
                # --update: resolve URL and check for newer version
                try:
                    url = resolve_url(source)
                    if url == existing.url:
                        console.print(f"  [dim]Current:[/dim] {source.id}")
                        skipped += 1
                        continue
                    else:
                        console.print(f"  [yellow]Update available:[/yellow] {source.id}")
                        updated += 1
                        downloads.append((source.id, url, dest_dir, source))
                except Exception as e:
                    console.print(f"  [red]FAIL[/red] {source.id}: {e}")
                    continue
            # File in manifest but missing from disk — re-download
            elif not existing_path.exists():
                console.print(f"  [yellow]Missing from disk:[/yellow] {source.id}")

        # Resolve URL for new/missing sources
        if not any(d[0] == source.id for d in downloads):
            try:
                url = resolve_url(source)
                downloads.append((source.id, url, dest_dir, source))
                console.print(f"  [green]OK[/green] {source.id}")
            except Exception as e:
                console.print(f"  [red]FAIL[/red] {source.id}: {e}")

    if skipped:
        console.print(f"  [dim]{skipped} already downloaded (use --update to check for newer versions)[/dim]")

    if not downloads:
        console.print("\n[bold green]Everything up to date.[/bold green]")
        manifest.last_synced = datetime.now().isoformat(timespec="seconds")
        manifest.save(manifest_path)
        return

    # Collect checksums: from preset yaml or .sha256 sidecars
    checksums: dict[str, str] = {}
    for source_id, url, dest_dir, source in downloads:
        if source.sha256:
            checksums[source_id] = source.sha256
        elif source.type == "zim":
            sidecar = fetch_sha256_sidecar(url)
            if sidecar:
                checksums[source_id] = sidecar
                console.print(f"  [dim]Checksum fetched: {source_id}[/dim]")

    console.print(f"\n[bold]Downloading {len(downloads)} file(s)...[/bold]")

    # Download with crash-safe manifest saving
    interrupted = False

    def _handle_interrupt(sig, frame):
        nonlocal interrupted
        interrupted = True
        console.print("\n[yellow]Interrupted — saving progress...[/yellow]")

    old_handler = signal.signal(signal.SIGINT, _handle_interrupt)

    try:
        for source_id, url, dest_dir, source in downloads:
            if interrupted:
                break

            try:
                source_checksums = {}
                if source_id in checksums:
                    source_checksums[source_id] = checksums[source_id]
                results = download_sources(
                    [(source_id, url, dest_dir)],
                    checksums=source_checksums,
                )
                r = results[0]

                if r.success and r.filepath:
                    # Delete old file if this is an update with a different filename
                    existing = manifest.entry_by_id(source_id)
                    if existing and existing.filename != r.filepath.name:
                        old_path = dest_dir / existing.filename
                        if old_path.exists():
                            old_path.unlink()
                            console.print(f"  [dim]Removed old: {existing.filename}[/dim]")

                    entry = ManifestEntry(
                        id=source_id,
                        type=source.type,
                        filename=r.filepath.name,
                        size_bytes=r.filepath.stat().st_size,
                        tags=source.tags,
                        depth=source.depth,
                        downloaded=datetime.now().isoformat(timespec="seconds"),
                        url=url,
                        checksum_sha256=r.sha256,
                    )
                    manifest.entries = [e for e in manifest.entries if e.id != source_id]
                    manifest.entries.append(entry)

                    # Save manifest after each successful download
                    manifest.last_synced = datetime.now().isoformat(timespec="seconds")
                    manifest.save(manifest_path)

            except Exception as e:
                console.print(f"  [red]Failed: {source_id}: {e}[/red]")
    finally:
        signal.signal(signal.SIGINT, old_handler)
        # Final save
        manifest.last_synced = datetime.now().isoformat(timespec="seconds")
        manifest.save(manifest_path)

    succeeded = sum(
        1
        for sid, _, _, _ in downloads
        if manifest.entry_by_id(sid)
        and manifest.entry_by_id(sid).downloaded >= manifest.last_synced[:10]
    )
    total = len(downloads)
    failed = total - succeeded
    if interrupted:
        console.print(f"\n[yellow]Interrupted:[/yellow] {succeeded}/{total} downloaded. Run primer sync to continue.")
    else:
        console.print(f"\n[bold green]Done:[/bold green] {succeeded} downloaded, {failed} failed.")


def show_status(path: str, check_updates: bool = False):
    """Show drive status."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[dim]No primer drive found at this path.[/dim]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_preset(manifest.preset)
    active_sources = preset.sources_for_options(set(manifest.enabled_groups))

    # Resolve URLs if checking for updates
    resolved_urls: dict[str, str] = {}
    if check_updates:
        console.print("[dim]Checking for updates...[/dim]")
        for source in active_sources:
            try:
                resolved_urls[source.id] = resolve_url(source)
            except Exception:
                pass

    table = Table(title=f"Primer — {manifest.preset}")
    table.add_column("ID", style="cyan")
    table.add_column("Type")
    table.add_column("Size", justify="right")
    table.add_column("Downloaded")
    table.add_column("Status")

    total_bytes = 0
    for source in sorted(active_sources, key=lambda s: s.id):
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
