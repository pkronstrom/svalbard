import signal
import shutil
import re
import subprocess
import platform as host_platform
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

from rich.console import Console
from rich.table import Table

from svalbard.builder import BuildResult, check_tools, run_build
from svalbard.downloader import download_sources, fetch_sha256_sidecar
from svalbard.drive_config import load_snapshot_preset, write_drive_snapshot
from svalbard.local_sources import (
    active_sources_for_manifest,
    load_local_sources,
)
from svalbard.manifest import LocalSourceSnapshot, Manifest, ManifestEntry
from svalbard.models import Source
from svalbard.paths import workspace_root as resolve_workspace_root
from svalbard.presets import builtin_recipe_ids, load_preset
from svalbard.readme_generator import generate_drive_readme
from svalbard.resolver import resolve_url
from svalbard.toolkit_generator import generate_toolkit
from svalbard.taxonomy import compute_coverage, load_taxonomy
from svalbard.viewer_generator import generate_map_viewer

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
    "sqlite": "data",
    "toolchain": "tools/platformio/packages",
}

TYPE_GROUPS = {
    "zim": "reference",
    "pdf": "books",
    "epub": "books",
    "pmtiles": "maps",
    "gguf": "models",
    "binary": "tools",
    "app": "tools",
    "iso": "tools",
    "sqlite": "regional",
}

# Archive suffixes that need extraction for binary-type sources
_ARCHIVE_SUFFIXES = {".tar.gz", ".tar.xz", ".tar.bz2", ".tgz", ".zip"}


def _is_archive(path: Path) -> bool:
    name = path.name.lower()
    return any(name.endswith(s) for s in _ARCHIVE_SUFFIXES)



@dataclass
class DownloadJob:
    source_id: str
    source_type: str
    url: str
    dest_dir: Path
    source: Source
    platform: str = ""


def _detect_host_platform() -> str:
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


def _normalize_platform_filter(platform_filter: str | None) -> str | None:
    if not platform_filter:
        return None

    normalized = platform_filter.strip().lower()
    aliases = {
        "host": "host",
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
    if resolved == "host":
        return _detect_host_platform()
    return resolved


def _platform_matches(job_platform: str, platform_filter: str | None) -> bool:
    normalized = _normalize_platform_filter(platform_filter)
    if normalized is None:
        return True
    if normalized == "arm64":
        return job_platform.endswith("-arm64")
    if normalized == "x86_64":
        return job_platform.endswith("-x86_64")
    return job_platform == normalized


def _normalized_source_slug(source_id: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", source_id.lower()).strip("-")
    return slug or "local-source"


def _resolve_local_source_path(source: Source, root: Path) -> Path:
    path = Path(source.path)
    if not path.is_absolute():
        path = root / path
    return path


def _local_source_kind(path: Path) -> str:
    return "dir" if path.is_dir() else "file"


def _directory_size_bytes(path: Path) -> int:
    return sum(p.stat().st_size for p in path.rglob("*") if p.is_file())


def _has_nested_symlink(path: Path) -> bool:
    return any(child.is_symlink() for child in path.rglob("*"))


def _local_dest_path(source: Source, source_path: Path, drive_path: Path) -> Path:
    dest_dir = drive_path / TYPE_DIRS.get(source.type, "other")
    slug = _normalized_source_slug(source.id)
    if source_path.is_dir():
        return dest_dir / slug
    suffix = source_path.suffix or {
        "zim": ".zim",
        "pdf": ".pdf",
        "epub": ".epub",
        "pmtiles": ".pmtiles",
        "gguf": ".gguf",
        "sqlite": ".sqlite",
        "iso": ".iso",
    }.get(source.type, "")
    return dest_dir / f"{slug}{suffix}"


def _record_local_snapshot(manifest: Manifest, source: Source, source_path: Path) -> None:
    snapshot = LocalSourceSnapshot(
        id=source.id,
        path=source.path,
        kind=_local_source_kind(source_path),
        size_bytes=source.size_bytes or (
            _directory_size_bytes(source_path) if source_path.is_dir() else source_path.stat().st_size
        ),
        mtime=source_path.stat().st_mtime,
    )
    manifest.local_source_snapshots = [s for s in manifest.local_source_snapshots if s.id != source.id]
    manifest.local_source_snapshots.append(snapshot)


def add_local_source(
    path: Path | str,
    workspace_root: Path | str | None = None,
    source_type: str | None = None,
    source_id: str | None = None,
) -> str:
    """Register a local file or directory as a workspace local source."""
    root = Path(workspace_root).resolve() if workspace_root else resolve_workspace_root()
    artifact = Path(path).resolve()
    if not artifact.exists():
        raise FileNotFoundError(f"Local source path does not exist: {artifact}")

    inferred_type = source_type
    if inferred_type is None:
        suffix_map = {
            ".zim": "zim",
            ".pdf": "pdf",
            ".epub": "epub",
            ".pmtiles": "pmtiles",
            ".gguf": "gguf",
            ".sqlite": "sqlite",
            ".iso": "iso",
        }
        inferred_type = suffix_map.get(artifact.suffix.lower())
    if inferred_type is None:
        raise ValueError("Could not infer local source type; pass source_type explicitly")
    if artifact.is_dir() and _has_nested_symlink(artifact):
        raise ValueError("Directory-backed local sources cannot contain nested symlinks")

    raw_name = source_id or artifact.stem
    slug = _normalized_source_slug(raw_name)
    normalized_id = f"local:{slug.removeprefix('local-')}" if not raw_name.startswith("local:") else f"local:{slug.split('local-', 1)[-1]}"
    if normalized_id in builtin_recipe_ids():
        raise ValueError(f"Local source id '{normalized_id}' collides with built-in source id")
    local_dir = root / "local" / "recipes"
    local_dir.mkdir(parents=True, exist_ok=True)
    sidecar = local_dir / f"{slug.removeprefix('local-')}.yaml"
    for existing in local_dir.glob("*.yaml"):
        if existing == sidecar:
            continue
        if f"id: {normalized_id}\n" in existing.read_text():
            raise ValueError(f"Local source id '{normalized_id}' already exists")
    size_bytes = _directory_size_bytes(artifact) if artifact.is_dir() else artifact.stat().st_size
    try:
        rel_path = artifact.relative_to(root)
        stored_path = rel_path.as_posix()
    except ValueError:
        stored_path = str(artifact)

    sidecar.write_text(
        "\n".join(
            [
                f"id: {normalized_id}",
                f"type: {inferred_type}",
                f"display_group: {TYPE_GROUPS.get(inferred_type, 'practical')}",
                "strategy: local",
                f"path: {stored_path}",
                f"description: {artifact.stem}",
                f"size_bytes: {size_bytes}",
                "",
            ]
        )
    )
    return normalized_id


def expand_source_downloads(
    source: Source,
    drive_path: Path,
    platform_filter: str | None = None,
) -> list[DownloadJob]:
    """Expand one source into one or more concrete download jobs."""
    if source.platforms:
        jobs = []
        for platform, url in sorted(source.platforms.items()):
            if not _platform_matches(platform, platform_filter):
                continue
            if source.type == "toolchain":
                dest = drive_path / "tools" / "platformio" / "packages" / platform
            else:
                dest = drive_path / "bin" / platform
            jobs.append(
                DownloadJob(
                    source_id=source.id,
                    source_type=source.type,
                    url=url,
                    dest_dir=dest,
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


def _init_drive(
    path: str,
    preset_name: str,
    workspace_root_path: str = "",
    local_sources: list[str] | None = None,
    platform_filter: str | None = None,
):
    """Initialize a drive with a preset."""
    drive_path = Path(path)
    drive_path.mkdir(parents=True, exist_ok=True)

    preset = load_preset(preset_name, workspace=workspace_root_path or None)
    sources = preset.sources

    manifest = Manifest(
        preset=preset.name,
        region=preset.region,
        target_path=str(drive_path),
        created=datetime.now().isoformat(timespec="seconds"),
        workspace_root=workspace_root_path,
        local_sources=local_sources or [],
    )
    manifest.save(drive_path / "manifest.yaml")
    if workspace_root_path:
        write_drive_snapshot(
            drive_path,
            preset_name=preset.name,
            workspace_root=Path(workspace_root_path).resolve(),
            local_source_ids=local_sources or [],
        )

    generate_drive_readme(drive_path)

    # Generate map viewer if preset has pmtiles sources
    if any(s.type == "pmtiles" for s in sources):
        generate_map_viewer(drive_path, preset_name)

    generate_toolkit(drive_path, preset_name, platform_filter=platform_filter)

    console.print(f"[bold green]Initialized:[/bold green] {drive_path}")
    console.print(f"  Preset: {preset.name}")
    console.print(f"  Sources: {len(sources)}")
    console.print(f"  Estimated size: {sum(s.size_gb for s in sources):.1f} GB")
    console.print(f"\nRun [bold]svalbard sync[/bold] to download content.")
    return manifest


def init_drive(
    path: str,
    preset_name: str,
    workspace_root: str = "",
    local_sources: list[str] | None = None,
    platform_filter: str | None = None,
):
    return _init_drive(
        path,
        preset_name,
        workspace_root_path=workspace_root,
        local_sources=local_sources,
        platform_filter=platform_filter,
    )


def _artifact_path_for_build(source: Source, drive_path: Path) -> Path | None:
    """Return the expected artifact path for a build source, or None."""
    type_dir = TYPE_DIRS.get(source.type, "other")
    dest_dir = drive_path / type_dir
    if source.type == "app":
        app_dir = dest_dir / source.id
        if app_dir.exists() and any(app_dir.iterdir()):
            return app_dir
        return None
    ext = {"pmtiles": "pmtiles", "sqlite": "sqlite"}.get(source.type, source.type)
    path = dest_dir / f"{source.id}.{ext}"
    return path if path.exists() else None


def sync_drive(
    path: str,
    update: bool = False,
    force: bool = False,
    parallel: int = 5,
    platform_filter: str | None = None,
):
    """Download/update content on an initialized drive."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found. Run svalbard init or svalbard wizard first.[/red]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    workspace = manifest.workspace_root or None

    # Always load the live preset so new sources added after init are picked up.
    # Fall back to the frozen snapshot only when the live preset is unavailable
    # (e.g. drive moved to another machine without the workspace).
    try:
        preset = load_preset(manifest.preset, workspace=workspace)
    except (FileNotFoundError, ValueError, KeyError):
        preset = load_snapshot_preset(drive_path)
        if preset is None:
            console.print("[red]Cannot load preset — no live workspace or on-drive snapshot.[/red]")
            return

    # Refresh the on-drive snapshot so the drive stays self-describing.
    if workspace:
        try:
            write_drive_snapshot(
                drive_path,
                preset_name=manifest.preset,
                workspace_root=Path(workspace),
                local_source_ids=manifest.local_sources,
            )
        except Exception as e:
            console.print(f"[yellow]Warning: could not refresh snapshot: {e}[/yellow]")
    manifest_path = drive_path / "manifest.yaml"
    local_source_map = {
        source.id: source for source in load_local_sources(manifest.workspace_root or resolve_workspace_root())
    }
    selected_local_sources: list[Source] = []
    local_failures = 0
    for source_id in manifest.local_sources:
        source = local_source_map.get(source_id)
        if source is None:
            console.print(f"  [red]Missing local source:[/red] {source_id}")
            local_failures += 1
            continue
        selected_local_sources.append(source)

    console.print(f"[bold]Syncing:[/bold] {manifest.preset} → {drive_path}")

    # Split sources by strategy
    download_sources_list = [s for s in preset.sources if s.strategy != "build"]
    build_sources = [s for s in preset.sources if s.strategy == "build"]

    # ── Downloads ────────────────────────────────────────────────────────
    console.print("\n[bold]Resolving latest versions...[/bold]")
    downloads: list[DownloadJob] = []
    skipped = 0
    updated = 0

    for source in download_sources_list:
        try:
            jobs = expand_source_downloads(source, drive_path, platform_filter=platform_filter)
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

    has_downloads = bool(downloads)
    has_builds = bool(build_sources)
    has_locals = bool(selected_local_sources)

    if not has_downloads and not has_builds and not has_locals:
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

    # Download with crash-safe manifest saving
    interrupted = False

    def _handle_interrupt(sig, frame):
        nonlocal interrupted
        interrupted = True
        console.print("\n[yellow]Interrupted — saving progress...[/yellow]")

    old_handler = signal.signal(signal.SIGINT, _handle_interrupt)

    download_succeeded = 0
    download_failed = 0

    try:
        # Sort smallest first so small tools finish quickly while large ZIMs stream
        downloads.sort(key=lambda j: j.source.size_gb)

        if downloads:
            # Count cached vs pending
            cached = sum(
                1 for j in downloads
                if (j.dest_dir / j.url.rsplit("/", 1)[-1]).exists()
            )
            pending = len(downloads) - cached
            if cached:
                console.print(f"\n  [dim]{cached} file(s) already cached[/dim]")
            if pending:
                label = f"Downloading {pending} file(s)"
                if parallel > 1:
                    label += f" ({parallel} parallel)"
                console.print(f"[bold]{label}...[/bold]")
            else:
                console.print("[bold]All files cached — nothing to download.[/bold]")

        # Build download tuples and checksum map keyed by source_id
        dl_tuples = [(job.source_id, job.url, job.dest_dir) for job in downloads]
        dl_checksums = {}
        for job in downloads:
            checksum_key = f"{job.source_id}:{job.platform}"
            if checksum_key in checksums:
                dl_checksums[job.source_id] = checksums[checksum_key]

        results = download_sources(dl_tuples, checksums=dl_checksums, parallel=parallel)

        # Map results by source_id for lookup
        results_by_id = {r.source_id: r for r in results}
        job_by_id = {job.source_id: job for job in downloads}

        for r in results:
            if interrupted:
                break

            job = job_by_id[r.source_id]

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
                download_succeeded += 1

                # Save manifest after each successful download
                manifest.last_synced = datetime.now().isoformat(timespec="seconds")
                manifest.save(manifest_path)
            else:
                download_failed += 1

        # ── Builds ───────────────────────────────────────────────────────
        if build_sources and not interrupted:
            # Check which builds are needed
            pending_builds = []
            for source in build_sources:
                if force or _artifact_path_for_build(source, drive_path) is None:
                    pending_builds.append(source)
                else:
                    skipped += 1

            if pending_builds:
                # Check required tools
                families = list({s.build.get("family", "") for s in pending_builds})
                missing_tools = check_tools(families, drive_path=drive_path)
                if missing_tools:
                    console.print(
                        f"\n[red]Missing build tools:[/red] {', '.join(missing_tools)}"
                    )
                    console.print("  Install with: brew install tippecanoe gdal")
                    console.print("  Or use Docker (auto-detected if available)")
                    console.print("  For pmtiles: include go-pmtiles in your preset or go install github.com/protomaps/go-pmtiles@latest")
                else:
                    console.print(f"\n[bold]Building {len(pending_builds)} source(s)...[/bold]")
                    for source in pending_builds:
                        if interrupted:
                            break
                        console.print(f"  Building {source.id}...")
                        result = run_build(source, drive_path)
                        if result.success and result.artifact:
                            artifact = result.artifact
                            if artifact.is_dir():
                                size_bytes = sum(f.stat().st_size for f in artifact.rglob("*") if f.is_file())
                                filename = source.id
                            else:
                                size_bytes = artifact.stat().st_size
                                filename = artifact.name

                            entry = ManifestEntry(
                                id=source.id,
                                type=source.type,
                                filename=filename,
                                size_bytes=size_bytes,
                                tags=source.tags,
                                depth=source.depth,
                                downloaded=datetime.now().isoformat(timespec="seconds"),
                            )
                            manifest.entries = [
                                e for e in manifest.entries if e.id != source.id
                            ]
                            manifest.entries.append(entry)
                            manifest.last_synced = datetime.now().isoformat(timespec="seconds")
                            manifest.save(manifest_path)
                            console.print(f"  [green]OK[/green] {source.id}")
                        else:
                            download_failed += 1
                            console.print(f"  [red]FAIL[/red] {source.id}: {result.error}")

        if selected_local_sources and not interrupted:
            console.print(f"\n[bold]Importing {len(selected_local_sources)} local source(s)...[/bold]")
            root = Path(manifest.workspace_root).resolve() if manifest.workspace_root else resolve_workspace_root()
            for source in selected_local_sources:
                source_path = _resolve_local_source_path(source, root)
                dest_path = _local_dest_path(source, source_path, drive_path)
                dest_path.parent.mkdir(parents=True, exist_ok=True)
                if source_path.is_dir():
                    if dest_path.exists():
                        shutil.rmtree(dest_path)
                    shutil.copytree(source_path, dest_path, symlinks=False)
                    size_bytes = _directory_size_bytes(dest_path)
                    filename = dest_path.name
                    kind = "dir"
                else:
                    shutil.copy2(source_path, dest_path)
                    size_bytes = dest_path.stat().st_size
                    filename = dest_path.name
                    kind = "file"

                entry = ManifestEntry(
                    id=source.id,
                    type=source.type,
                    filename=filename,
                    size_bytes=size_bytes,
                    tags=source.tags,
                    depth=source.depth,
                    downloaded=datetime.now().isoformat(timespec="seconds"),
                    relative_path=dest_path.relative_to(drive_path).as_posix(),
                    kind=kind,
                    source_strategy="local",
                )
                manifest.entries = [e for e in manifest.entries if e.id != source.id]
                manifest.entries.append(entry)
                _record_local_snapshot(manifest, source, source_path)
                manifest.last_synced = datetime.now().isoformat(timespec="seconds")
                manifest.save(manifest_path)
                console.print(f"  [green]OK[/green] {source.id}")
    finally:
        signal.signal(signal.SIGINT, old_handler)
        # Final save
        manifest.last_synced = datetime.now().isoformat(timespec="seconds")
        manifest.save(manifest_path)

    # Regenerate map viewer and toolkit after sync
    active_sources = active_sources_for_manifest(manifest, preset)
    if any(s.type == "pmtiles" for s in active_sources):
        generate_map_viewer(drive_path, manifest.preset)
    generate_drive_readme(drive_path)
    generate_toolkit(drive_path, manifest.preset, platform_filter=platform_filter)

    total = len(downloads) + len(build_sources) + len(selected_local_sources) - skipped
    if interrupted:
        console.print(f"\n[yellow]Interrupted.[/yellow] Run svalbard sync to continue.")
    elif download_failed or local_failures:
        console.print(f"\n[bold green]Done:[/bold green] {download_failed} failed.")
    else:
        console.print(f"\n[bold green]Done:[/bold green] all sources synced.")


def show_status(path: str, check_updates: bool = False):
    """Show drive status."""
    drive_path = Path(path)
    if not Manifest.exists(drive_path):
        console.print("[dim]No svalbard drive found at this path.[/dim]")
        return

    manifest = Manifest.load(drive_path / "manifest.yaml")
    preset = load_snapshot_preset(drive_path) or load_preset(
        manifest.preset,
        workspace=manifest.workspace_root or None,
    )
    active_sources = active_sources_for_manifest(manifest, preset)

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
