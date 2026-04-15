"""Svalbard CLI entry point."""

import shutil
from dataclasses import dataclass
from pathlib import Path

import click
from rich.console import Console
from rich.prompt import Prompt
from rich.table import Table

from svalbard.commands import remove_source_artifacts, sync_drive
from svalbard.drive_config import load_snapshot_preset, write_drive_snapshot
from svalbard.importer import run_import
from svalbard.bundle import run_bundle_build
from svalbard.local_sources import load_local_sources
from svalbard.manifest import Manifest
from svalbard.membership import (
    add_local_source_to_drive,
    remove_local_source_from_drive,
    resolve_drive_path,
)
from svalbard.paths import workspace_root as resolve_workspace_root
from svalbard.presets import (
    _source_from_recipe,
    builtin_recipe_ids,
    copy_preset_to_workspace,
    list_presets,
    load_preset,
    recipe_data_by_id,
)
from svalbard.wizard import pick_sources_via_packs, run_wizard, write_custom_preset

console = Console()


@dataclass
class BuiltinSelectionReview:
    added_ids: list[str]
    removed_ids: list[str]
    will_download_ids: list[str]
    will_remove_ids: list[str]
    current_total_gb: float
    selected_total_gb: float
    size_delta_gb: float


@click.group(invoke_without_command=True)
@click.pass_context
def main(ctx: click.Context) -> None:
    """Svalbard — Seed vault for human knowledge."""
    if ctx.invoked_subcommand is None:
        from pathlib import Path

        from svalbard.manifest import Manifest

        cwd = Path.cwd()
        if Manifest.exists(cwd):
            from svalbard.commands import show_status

            show_status(str(cwd))
            _show_menu(str(cwd))
        else:
            from rich.prompt import Confirm

            console.print("[bold]Svalbard[/bold] — Seed vault for human knowledge\n")
            console.print("No drive found in current directory.")
            if Confirm.ask("  Run setup wizard?", default=True):
                run_wizard()
            else:
                console.print("Run [bold]svalbard --help[/bold] for all commands.")


def _show_menu(path: str):
    console.print("\n  [bold][s][/bold] Sync (check for updates)")
    console.print("  [bold][a][/bold] Audit report")
    console.print("  [bold][w][/bold] Wizard (reconfigure)")
    console.print("  [bold][q][/bold] Quit")
    choice = console.input("\n  > ")
    if choice == "s":
        from svalbard.commands import sync_drive

        sync_drive(path)
    elif choice == "a":
        from pathlib import Path as P

        from svalbard.audit import generate_audit

        click.echo(generate_audit(P(path)))
    elif choice == "w":
        run_wizard()


def _drive_workspace_root(manifest: Manifest, workspace: str | None) -> Path:
    if workspace is not None:
        return resolve_workspace_root(workspace)
    if manifest.workspace_root:
        return Path(manifest.workspace_root).resolve()
    return resolve_workspace_root()


def _current_builtin_source_ids(drive_path: Path, manifest: Manifest, workspace_root: Path) -> set[str]:
    try:
        preset = load_preset(manifest.preset, workspace=workspace_root)
    except (FileNotFoundError, ValueError, KeyError):
        preset = load_snapshot_preset(drive_path)
    if preset is not None:
        return {source.id for source in preset.sources if not source.id.startswith("local:")}
    return {entry.id for entry in manifest.entries if not entry.id.startswith("local:")}


def _persist_drive_builtin_selection(
    drive_path: Path,
    manifest: Manifest,
    workspace_root: Path,
    selected_ids: set[str],
) -> str:
    try:
        target_size_gb = shutil.disk_usage(drive_path).free / 1e9
    except OSError:
        target_size_gb = 0

    preset_name = write_custom_preset(
        selected_ids,
        workspace=workspace_root,
        target_size_gb=target_size_gb,
        region=manifest.region,
        description=f"Vault selection for {drive_path.name}",
    )
    write_drive_snapshot(
        drive_path,
        preset_name=preset_name,
        workspace_root=workspace_root,
        local_source_ids=manifest.local_sources,
    )
    manifest.preset = preset_name
    manifest.save(drive_path / "manifest.yaml")
    return preset_name


def _compute_builtin_selection_review(
    *,
    manifest: Manifest,
    current_ids: set[str],
    selected_ids: set[str],
    source_lookup: dict[str, object],
) -> BuiltinSelectionReview:
    added_ids = sorted(selected_ids - current_ids)
    removed_ids = sorted(current_ids - selected_ids)
    downloaded_ids = {entry.id for entry in manifest.entries}
    will_download_ids = [source_id for source_id in added_ids if source_id not in downloaded_ids]
    will_remove_ids = [source_id for source_id in removed_ids if manifest.entries_by_id(source_id)]
    current_total_gb = sum(source_lookup[source_id].size_gb for source_id in current_ids if source_id in source_lookup)
    selected_total_gb = sum(
        source_lookup[source_id].size_gb for source_id in selected_ids if source_id in source_lookup
    )
    return BuiltinSelectionReview(
        added_ids=added_ids,
        removed_ids=removed_ids,
        will_download_ids=will_download_ids,
        will_remove_ids=will_remove_ids,
        current_total_gb=round(current_total_gb, 1),
        selected_total_gb=round(selected_total_gb, 1),
        size_delta_gb=round(selected_total_gb - current_total_gb, 1),
    )


def _review_builtin_selection(review: BuiltinSelectionReview) -> str:
    console.print("\n[bold]Review Selection[/bold]")
    table = Table(show_header=True, header_style="bold", box=None, padding=(0, 1))
    table.add_column("Change", no_wrap=True)
    table.add_column("Sources")
    table.add_row("Added", ", ".join(review.added_ids) if review.added_ids else "[dim]none[/dim]")
    table.add_row("Removed", ", ".join(review.removed_ids) if review.removed_ids else "[dim]none[/dim]")
    table.add_row(
        "Will download",
        ", ".join(review.will_download_ids) if review.will_download_ids else "[dim]none[/dim]",
    )
    table.add_row(
        "Will remove",
        ", ".join(review.will_remove_ids) if review.will_remove_ids else "[dim]none[/dim]",
    )
    console.print(table)
    delta_style = "green" if review.size_delta_gb <= 0 else "yellow"
    console.print(f"  [bold]Current total:[/bold]  {review.current_total_gb:.1f} GB")
    console.print(f"  [bold]Selected total:[/bold] {review.selected_total_gb:.1f} GB")
    console.print(f"  [bold]Estimated delta:[/bold] [{delta_style}]{review.size_delta_gb:+.1f} GB[/{delta_style}]")
    console.print("  [dim]a apply  b back to picker  q cancel[/dim]")
    return Prompt.ask("\n  Select", choices=["a", "b", "q"], default="b")


def _browse_drive_builtin_sources(drive_path: Path, workspace: str | None) -> str:
    manifest_path = drive_path / "manifest.yaml"
    manifest = Manifest.load(manifest_path)
    workspace_root = _drive_workspace_root(manifest, workspace)
    try:
        free_gb = shutil.disk_usage(drive_path).free / 1e9
    except OSError:
        free_gb = 0
    try:
        preset = load_preset(manifest.preset, workspace=workspace_root)
    except (FileNotFoundError, ValueError, KeyError):
        preset = load_snapshot_preset(drive_path)
    if preset is not None:
        current_ids = {source.id for source in preset.sources if not source.id.startswith("local:")}
    else:
        current_ids = {entry.id for entry in manifest.entries if not entry.id.startswith("local:")}
    checked_ids = set(current_ids)
    source_lookup = {
        source_id: _source_from_recipe(recipe_data_by_id(source_id))
        for source_id in builtin_recipe_ids()
    }

    while True:
        selected_ids = pick_sources_via_packs(
            free_gb=free_gb,
            workspace=workspace_root,
            checked_ids=checked_ids,
        )
        if selected_ids is None:
            return ""
        review = _compute_builtin_selection_review(
            manifest=manifest,
            current_ids=current_ids,
            selected_ids=selected_ids,
            source_lookup=source_lookup,
        )
        action = _review_builtin_selection(review)
        if action == "q":
            return ""
        if action == "b":
            checked_ids = selected_ids
            continue
        preset_name = _persist_drive_builtin_selection(drive_path, manifest, workspace_root, selected_ids)
        sync_drive(str(drive_path))
        return preset_name


def _resolve_source_reference(raw_source_id: str, workspace_root: Path) -> tuple[str, str]:
    if raw_source_id.startswith("local:"):
        local_ids = {source.id for source in load_local_sources(workspace_root)}
        if raw_source_id not in local_ids:
            raise click.UsageError(f"Local source not found: {raw_source_id}")
        return raw_source_id, "local"

    builtin_ids = builtin_recipe_ids()
    local_matches = [
        source.id
        for source in load_local_sources(workspace_root)
        if raw_source_id in {source.id, source.id.split(":", 1)[-1]}
    ]
    builtin_match = raw_source_id if raw_source_id in builtin_ids else None

    if builtin_match and local_matches:
        raise click.UsageError(
            f"Ambiguous source '{raw_source_id}'; use 'local:{raw_source_id}' for the workspace source or the exact built-in id."
        )
    if local_matches:
        return local_matches[0], "local"
    if builtin_match:
        return builtin_match, "builtin"
    raise click.UsageError(f"Source not found: {raw_source_id}")


@main.command()
@click.option("--path", default=None, help="Target drive path (skip target selection)")
@click.option("--preset", default=None, help="Preset name (skip region and preset selection)")
@click.option(
    "--platform",
    default=None,
    help="Download only matching binary platforms: host, arm64, x86_64, macos-arm64, macos-x86_64, linux-arm64, linux-x86_64",
)
def wizard(path: str | None, preset: str | None, platform: str | None) -> None:
    """Interactive setup wizard."""
    run_wizard(target_path=path, preset_name=preset, platform=platform)


@main.command()
@click.argument("path")
@click.option("--preset", default=None, help="Preset name to pre-check before browsing")
@click.option(
    "--platform",
    default=None,
    help="Download only matching binary platforms: host, arm64, x86_64, macos-arm64, macos-x86_64, linux-arm64, linux-x86_64",
)
@click.option("--workspace", default=None, help="Workspace root")
def init(path: str, preset: str, platform: str | None, workspace: str | None) -> None:
    """Initialize a drive via the interactive pack picker."""
    kwargs = {
        "target_path": path,
        "preset_name": preset,
        "browse_only": True,
        "platform": platform,
    }
    if workspace is not None:
        kwargs["workspace"] = workspace
    run_wizard(**kwargs)


@main.command()
@click.argument("path", default=".")
@click.option("--update", is_flag=True, help="Check for and download newer versions")
@click.option("--force", is_flag=True, help="Re-download everything")
@click.option("--parallel", "-j", default=5, type=int, help="Parallel downloads (default: 5)")
@click.option(
    "--platform",
    default=None,
    help="Download only matching binary platforms: host, arm64, x86_64, macos-arm64, macos-x86_64, linux-arm64, linux-x86_64",
)
def sync(path: str, update: bool, force: bool, parallel: int, platform: str | None) -> None:
    """Download/update content on initialized drive."""
    from svalbard.commands import sync_drive

    sync_drive(path, update=update, force=force, parallel=parallel, platform_filter=platform)


@main.command()
@click.argument("path", default=".")
@click.option("--check", is_flag=True, help="Check for available updates (hits network)")
def status(path: str, check: bool) -> None:
    """Show what's downloaded, what's stale."""
    from svalbard.commands import show_status

    show_status(path, check_updates=check)


@main.command()
@click.argument("path", default=".")
def audit(path: str) -> None:
    """Generate LLM-ready gap analysis report."""
    from pathlib import Path as P

    from svalbard.manifest import Manifest

    drive_path = P(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found.[/red]")
        return
    from svalbard.audit import generate_audit

    report = generate_audit(drive_path)
    click.echo(report)


@main.command("import")
@click.argument("input_values", nargs=-1, required=True)
@click.option("--bundle", "bundle_name", default=None, help="Package files into a named ZIM bundle")
@click.option("--title", default=None, help="Bundle title (default: derived from name)")
@click.option("--description", default=None, help="Bundle description")
@click.option("--language", default="eng", show_default=True, help="ISO-639-3 language code")
@click.option("--kind", type=click.Choice(["auto", "local", "web", "media"]), default="auto", show_default=True)
@click.option("--runner", type=click.Choice(["auto", "docker", "host"]), default="auto", show_default=True)
@click.option("--quality", type=click.Choice(["1080p", "720p", "480p", "360p", "source"]), default="720p", show_default=True)
@click.option("--audio-only", is_flag=True, help="Ingest as audio-only media where supported")
@click.option("-o", "--output", "output_name", default=None, help="Output artifact name")
@click.option("--workspace", default=None, help="Workspace root")
def import_command(
    input_values: tuple[str, ...],
    bundle_name: str | None,
    title: str | None,
    description: str | None,
    language: str,
    kind: str,
    runner: str,
    quality: str,
    audio_only: bool,
    output_name: str | None,
    workspace: str | None,
) -> None:
    """Import local content or remote URLs as workspace-local sources."""
    from pathlib import Path

    ws = resolve_workspace_root(workspace)

    if bundle_name:
        source_id = run_bundle_build(
            files=[Path(f) for f in input_values],
            name=bundle_name,
            workspace_root=ws,
            title=title,
            description=description,
            language=language,
        )
        console.print(f"[green]Bundle created:[/green] {source_id}")
        return

    if len(input_values) > 1:
        console.print("[red]Error:[/red] multiple inputs require --bundle NAME")
        raise SystemExit(1)

    source_id = run_import(
        input_values[0],
        workspace_root=ws,
        kind=kind,
        runner=runner,
        quality=quality,
        audio_only=audio_only,
        output_name=output_name,
    )
    console.print(f"[green]Imported:[/green] {source_id}")


@main.command("add")
@click.argument("source_id", required=False)
@click.argument("path", required=False, default=".")
@click.option("--browse", is_flag=True, help="Browse and select vault content interactively")
@click.option("--workspace", default=None, help="Workspace root")
def add_command(
    source_id: str | None,
    path: str,
    browse: bool,
    workspace: str | None,
) -> None:
    """Add a built-in or local source to an existing drive."""
    if browse:
        drive_arg = source_id if source_id and path == "." else path
        preset_name = _browse_drive_builtin_sources(resolve_drive_path(drive_arg), workspace)
        if not preset_name:
            console.print("[yellow]Cancelled.[/yellow]")
            return
        console.print(f"[green]Updated preset:[/green] {preset_name}")
        return

    if not source_id:
        raise click.UsageError("SOURCE_ID is required unless --browse is used")

    drive_path = resolve_drive_path(path)
    manifest = Manifest.load(drive_path / "manifest.yaml")
    workspace_root = _drive_workspace_root(manifest, workspace)
    resolved_source_id, source_kind = _resolve_source_reference(source_id, workspace_root)

    if source_kind == "local":
        add_local_source_to_drive(drive_path, resolved_source_id, workspace=workspace_root)
        console.print(f"[green]Added local source:[/green] {resolved_source_id}")
        return

    selected_ids = _current_builtin_source_ids(drive_path, manifest, workspace_root)
    if resolved_source_id in selected_ids:
        console.print(f"[yellow]Already present:[/yellow] {resolved_source_id}")
        return

    selected_ids.add(resolved_source_id)
    preset_name = _persist_drive_builtin_selection(drive_path, manifest, workspace_root, selected_ids)
    sync_drive(str(drive_path))
    console.print(f"[green]Added source:[/green] {resolved_source_id}")
    console.print(f"[green]Updated preset:[/green] {preset_name}")


@main.command("remove")
@click.argument("source_id")
@click.argument("path", required=False, default=".")
@click.option("--workspace", default=None, help="Workspace root")
def remove_command(source_id: str, path: str, workspace: str | None) -> None:
    """Remove a built-in or local source from an existing drive."""
    drive_path = resolve_drive_path(path)
    manifest = Manifest.load(drive_path / "manifest.yaml")
    workspace_root = _drive_workspace_root(manifest, workspace)
    resolved_source_id, source_kind = _resolve_source_reference(source_id, workspace_root)

    if source_kind == "local":
        remove_local_source_from_drive(drive_path, resolved_source_id)
        console.print(f"[green]Removed local source:[/green] {resolved_source_id}")
        return

    selected_ids = _current_builtin_source_ids(drive_path, manifest, workspace_root)
    if resolved_source_id not in selected_ids:
        console.print(f"[yellow]Not present:[/yellow] {resolved_source_id}")
        return

    selected_ids.remove(resolved_source_id)
    preset_name = _persist_drive_builtin_selection(drive_path, manifest, workspace_root, selected_ids)
    removed = remove_source_artifacts(drive_path, manifest, resolved_source_id)
    manifest.save(drive_path / "manifest.yaml")
    console.print(f"[green]Removed source:[/green] {resolved_source_id}")
    console.print(f"[green]Updated preset:[/green] {preset_name}")
    console.print(f"[green]Removed artifacts:[/green] {removed}")


@main.command("refresh")
@click.argument("source_id")
@click.argument("path", required=False, default=".")
@click.option(
    "--platform",
    default=None,
    help="Limit platformed binaries to host, arm64, x86_64, macos-arm64, macos-x86_64, linux-arm64, linux-x86_64",
)
def refresh_command(source_id: str, path: str, platform: str | None) -> None:
    """Delete and re-sync one built-in source from a drive."""
    drive_path = resolve_drive_path(path)
    if not Manifest.exists(drive_path):
        console.print("[red]No manifest found.[/red]")
        raise SystemExit(1)

    manifest = Manifest.load(drive_path / "manifest.yaml")
    removed = remove_source_artifacts(drive_path, manifest, source_id, platform_filter=platform)
    manifest.save(drive_path / "manifest.yaml")
    console.print(f"[green]Removed:[/green] {source_id} ({removed} artifact(s))")
    sync_drive(str(drive_path), platform_filter=platform, only_source_ids=[source_id])


@main.group("preset")
def preset_group() -> None:
    """Manage presets."""


@preset_group.command("list")
@click.option("--workspace", default=None, help="Workspace root")
def preset_list_command(workspace: str | None) -> None:
    """List built-in and workspace-owned presets."""
    for name in list_presets(workspace=resolve_workspace_root(workspace)):
        console.print(name)


@preset_group.command("copy")
@click.argument("source_name")
@click.argument("target_name")
@click.option("--workspace", default=None, help="Workspace root")
def preset_copy_command(source_name: str, target_name: str, workspace: str | None) -> None:
    """Copy a preset into the active workspace."""
    path = copy_preset_to_workspace(
        source_name,
        target_name,
        workspace=resolve_workspace_root(workspace),
    )
    console.print(f"[green]Copied preset:[/green] {path}")


@main.command()
@click.argument("target")
@click.option("--port", default=None, type=int, help="Port to serve on (default: random free port)")
def show(target: str, port: int | None) -> None:
    """Open or serve a file/recipe for viewing.

    TARGET can be a file path (e.g. library/zim/ifixit.zim) or a recipe ID
    (e.g. ifixit). File type determines the viewer:

    \b
      .zim      → serve via kiwix-serve (Docker) + open browser
      .pmtiles  → serve with local HTTP viewer + open browser
      .pdf/epub → open with system viewer
      .sqlite   → print schema + row counts, suggest datasette
    """
    from pathlib import Path

    file_path = Path(target)

    # If target looks like a file path (has extension or exists), use it directly.
    if file_path.suffix or file_path.exists():
        if not file_path.exists():
            console.print(f"[red]File not found:[/red] {file_path}")
            raise SystemExit(1)
        _show_file(file_path.resolve(), port)
        return

    # Otherwise, treat as a recipe ID — resolve to an artifact.
    _show_recipe(target, port)


def _find_free_port() -> int:
    """Find a free TCP port on localhost."""
    import socket

    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def _show_file(file_path, port: int | None) -> None:
    """Dispatch to the appropriate viewer based on file extension."""
    from pathlib import Path

    file_path = Path(file_path)
    suffix = file_path.suffix.lower()

    if suffix == ".zim":
        _serve_zim(file_path, port)
    elif suffix == ".pmtiles":
        _serve_pmtiles(file_path, port)
    elif suffix in (".pdf", ".epub"):
        _open_native(file_path)
    elif suffix in (".sqlite", ".db"):
        _inspect_sqlite(file_path)
    else:
        console.print(f"[yellow]Unknown file type:[/yellow] {suffix}")
        console.print("Trying system open...")
        _open_native(file_path)


def _serve_zim(file_path, port: int | None) -> None:
    """Serve a ZIM file via kiwix-serve in Docker."""
    import subprocess
    import webbrowser

    from svalbard.docker import has_docker

    if not has_docker():
        console.print("[red]Docker is required to serve ZIM files but is not available.[/red]")
        console.print("Install Docker Desktop or start the Docker daemon.")
        raise SystemExit(1)

    local_port = port or _find_free_port()
    container_name = f"svalbard-kiwix-{file_path.stem[:20]}-{local_port}"
    url = f"http://localhost:{local_port}"

    console.print(f"[bold]Serving:[/bold] {file_path.name}")
    console.print(f"[bold]URL:[/bold]     {url}")
    console.print(f"[dim]Press Ctrl-C to stop.[/dim]\n")

    docker_cmd = [
        "docker", "run", "--rm",
        "--name", container_name,
        "-p", f"{local_port}:8080",
        "-v", f"{file_path}:/data/{file_path.name}:ro",
        "ghcr.io/kiwix/kiwix-serve:latest",
        file_path.name,
    ]

    try:
        # Open browser after a short delay to let the server start.
        import threading

        def _open_browser():
            import time
            time.sleep(1.5)
            webbrowser.open(url)

        threading.Thread(target=_open_browser, daemon=True).start()

        subprocess.run(docker_cmd, check=True)
    except KeyboardInterrupt:
        console.print("\n[dim]Stopping kiwix-serve...[/dim]")
        subprocess.run(
            ["docker", "stop", container_name],
            capture_output=True,
        )
    except subprocess.CalledProcessError as exc:
        console.print(f"[red]kiwix-serve exited with code {exc.returncode}[/red]")
        raise SystemExit(1)


def _serve_pmtiles(file_path, port: int | None) -> None:
    """Serve a PMTiles file with a simple Python HTTP server + viewer page."""
    import http.server
    import threading
    import webbrowser

    local_port = port or _find_free_port()
    url = f"http://localhost:{local_port}"
    parent_dir = file_path.parent

    console.print(f"[bold]Serving:[/bold] {file_path.name}")
    console.print(f"[bold]URL:[/bold]     {url}/viewer.html")
    console.print(f"[dim]Press Ctrl-C to stop.[/dim]\n")

    # Write a minimal viewer HTML that loads the PMTiles file.
    viewer_html = f"""\
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>{file_path.stem} — PMTiles Viewer</title>
  <link rel="stylesheet" href="https://unpkg.com/maplibre-gl@3.6.2/dist/maplibre-gl.css" />
  <script src="https://unpkg.com/maplibre-gl@3.6.2/dist/maplibre-gl.js"></script>
  <script src="https://unpkg.com/pmtiles@3.0.6/dist/pmtiles.js"></script>
  <style>body{{margin:0}}#map{{width:100vw;height:100vh}}</style>
</head>
<body>
  <div id="map"></div>
  <script>
    let protocol = new pmtiles.Protocol();
    maplibregl.addProtocol("pmtiles", protocol.tile);
    new maplibregl.Map({{
      container: "map",
      style: {{
        version: 8,
        sources: {{
          pmtiles: {{
            type: "vector",
            url: "pmtiles://{url}/{file_path.name}",
          }}
        }},
        layers: [{{
          id: "bg",
          type: "background",
          paint: {{"background-color": "#e0e0e0"}}
        }}]
      }}
    }});
  </script>
</body>
</html>"""

    viewer_path = parent_dir / "viewer.html"
    _viewer_created = False
    if not viewer_path.exists():
        viewer_path.write_text(viewer_html)
        _viewer_created = True

    handler = http.server.SimpleHTTPRequestHandler

    class QuietHandler(handler):
        def log_message(self, format, *args):
            pass  # Suppress request logs

    try:
        server = http.server.HTTPServer(("127.0.0.1", local_port), QuietHandler)
        # Serve from the directory containing the PMTiles file.
        import os
        os.chdir(str(parent_dir))

        def _open_browser():
            import time
            time.sleep(1.0)
            webbrowser.open(f"{url}/viewer.html")

        threading.Thread(target=_open_browser, daemon=True).start()
        server.serve_forever()
    except KeyboardInterrupt:
        console.print("\n[dim]Server stopped.[/dim]")
    finally:
        if _viewer_created and viewer_path.exists():
            viewer_path.unlink()


def _open_native(file_path) -> None:
    """Open a file with the system default application."""
    import subprocess
    import sys

    console.print(f"[bold]Opening:[/bold] {file_path.name}")

    if sys.platform == "darwin":
        subprocess.run(["open", str(file_path)])
    elif sys.platform == "linux":
        subprocess.run(["xdg-open", str(file_path)])
    else:
        # Windows
        import os
        os.startfile(str(file_path))  # type: ignore[attr-defined]


def _inspect_sqlite(file_path) -> None:
    """Print schema and row counts for an SQLite database."""
    import sqlite3

    console.print(f"[bold]Database:[/bold] {file_path.name}")
    console.print(f"[dim]{file_path}[/dim]\n")

    try:
        conn = sqlite3.connect(str(file_path))
        cursor = conn.cursor()

        # Get all tables
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
        tables = [row[0] for row in cursor.fetchall()]

        if not tables:
            console.print("[yellow]No tables found.[/yellow]")
            conn.close()
            return

        from rich.table import Table

        table = Table(title="Tables", show_lines=False)
        table.add_column("Table", style="bold")
        table.add_column("Rows", justify="right")
        table.add_column("Columns")

        for tbl in tables:
            # Row count
            cursor.execute(f'SELECT COUNT(*) FROM "{tbl}"')
            row_count = cursor.fetchone()[0]

            # Column info
            cursor.execute(f'PRAGMA table_info("{tbl}")')
            cols = [row[1] for row in cursor.fetchall()]
            col_str = ", ".join(cols[:6])
            if len(cols) > 6:
                col_str += f" ... (+{len(cols) - 6})"

            table.add_row(tbl, f"{row_count:,}", col_str)

        console.print(table)

        # Check for FTS tables
        fts_tables = [t for t in tables if t.endswith("_fts") or "_fts_" in t]
        if fts_tables:
            console.print(f"\n[dim]FTS tables detected: {', '.join(fts_tables)}[/dim]")

        conn.close()

    except sqlite3.Error as exc:
        console.print(f"[red]SQLite error:[/red] {exc}")
        raise SystemExit(1)

    console.print(f"\n[bold]Tip:[/bold] For interactive exploration, run:")
    console.print(f"  datasette {file_path}")


def _show_recipe(recipe_id: str, port: int | None) -> None:
    """Resolve a recipe ID to a file and show it."""
    from pathlib import Path

    from svalbard.presets import _build_recipe_index

    recipe_index = _build_recipe_index()
    recipe = recipe_index.get(recipe_id)

    if recipe is None:
        console.print(f"[red]Unknown recipe or file:[/red] {recipe_id}")
        console.print("[dim]Tip: use a file path (e.g. zim/ifixit.zim) or a known recipe ID.[/dim]")
        raise SystemExit(1)

    source_type = recipe.get("type", "zim")
    ext_map = {
        "zim": ".zim",
        "pmtiles": ".pmtiles",
        "pdf": ".pdf",
        "epub": ".epub",
        "sqlite": ".sqlite",
    }
    ext = ext_map.get(source_type, f".{source_type}")

    # TYPE_DIRS from commands.py
    type_dirs = {
        "zim": "zim",
        "pmtiles": "maps",
        "pdf": "books",
        "epub": "books",
        "gguf": "models",
    }
    type_dir = type_dirs.get(source_type, "other")

    # Search for the artifact in current directory (drive root) or library/.
    cwd = Path.cwd()
    candidates = [
        cwd / type_dir / f"{recipe_id}{ext}",
        cwd / "library" / type_dir / f"{recipe_id}{ext}",
    ]

    # Also glob for partial matches (files downloaded with dates in names).
    search_dirs = [cwd / type_dir, cwd / "library" / type_dir]
    for search_dir in search_dirs:
        if search_dir.is_dir():
            for f in search_dir.iterdir():
                if f.name.startswith(recipe_id) and f.suffix == ext:
                    candidates.append(f)

    # Find the first existing candidate.
    found = None
    for candidate in candidates:
        if candidate.exists():
            found = candidate
            break

    if found is None:
        console.print(f"[bold]{recipe_id}[/bold] — {recipe.get('description', '')}")
        console.print(f"  Type: {source_type}, Size: ~{recipe.get('size_gb', '?')} GB\n")
        console.print(f"[yellow]Artifact not found on disk.[/yellow]")

        build_hint = "svalbard sync" if recipe.get("strategy") == "download" else "svalbard sync (build)"
        console.print(f"  To obtain it, run [bold]{build_hint}[/bold] on an initialized drive that includes this recipe.")

        from rich.prompt import Confirm

        if Confirm.ask("\n  Search for it across all drives?", default=False):
            # Look in common mount points.
            import glob as glob_mod
            pattern = f"**/{type_dir}/{recipe_id}*{ext}"
            console.print(f"  [dim]Searching {cwd} for {pattern}...[/dim]")
            matches = list(cwd.glob(pattern))
            if matches:
                console.print(f"  [green]Found:[/green] {matches[0]}")
                _show_file(matches[0], port)
            else:
                console.print("  [yellow]Not found.[/yellow]")
        return

    console.print(f"[bold]{recipe_id}[/bold] — {recipe.get('description', '')}")
    console.print(f"  [dim]{found}[/dim]\n")
    _show_file(found, port)


@main.command()
@click.argument("path", default=".")
@click.option("--strategy", type=click.Choice(["fast", "standard", "semantic"]), default="fast", help="Indexing strategy tier")
@click.option("--yes", "-y", is_flag=True, help="Skip confirmation prompt")
def index(path, strategy, yes):
    """Build search index over ZIM files on a drive."""
    from pathlib import Path as P

    from svalbard.indexer import run_index, scan_zim_files, estimate_index
    from svalbard.search_db import SearchDB

    drive_path = P(path)

    # 1. Check for ZIM files
    zim_files = scan_zim_files(drive_path)
    if not zim_files:
        console.print("No ZIM files found.")
        return

    # 2. Create data/ directory and open SearchDB
    data_dir = drive_path / "data"
    data_dir.mkdir(parents=True, exist_ok=True)
    db = SearchDB(data_dir / "search.db")

    # 3. Estimate work
    plan = estimate_index(drive_path, db, strategy=strategy)

    # 4. Display plan summary
    has_text_work = bool(plan.files_to_index)
    has_embed_work = plan.needs_embedding and plan.articles_to_embed > 0

    if not has_text_work and not has_embed_work:
        console.print("\n[green]Index is up to date.[/green]")
        return

    console.print(f"\n[bold]Index plan[/bold]")
    strategy_desc = {
        "fast": "indexes titles and article summaries — quick to build, good for simple keyword lookups",
        "standard": "indexes full article text — slower to build, finds matches deeper in content",
        "semantic": "adds AI-powered meaning search on top of standard — understands synonyms and related concepts",
    }
    console.print(f"  Strategy: [bold]{strategy}[/bold] — {strategy_desc[strategy]}")

    if has_text_work:
        console.print(f"\n  Text indexing:")
        console.print(f"    ZIMs to index:  {len(plan.files_to_index)} of {plan.total_zims}")
        if plan.new_zims:
            console.print(f"    New:            {plan.new_zims}")
        if plan.changed_zims:
            console.print(f"    Re-index:       {plan.changed_zims}")
        est_mb = plan.estimated_db_bytes / (1024 * 1024)
        console.print(f"    Estimated size: {est_mb:.1f} MB")

    if has_embed_work:
        # 768 dims * 4 bytes/float = ~3 KB per article
        embed_size_mb = plan.articles_to_embed * 768 * 4 / (1024 * 1024)
        # Rough: ~200-500 embeddings/sec on CPU, use 300 as conservative estimate
        embed_eta_min = plan.articles_to_embed / 300 / 60
        console.print(f"\n  Embedding:")
        console.print(f"    Articles to embed: {plan.articles_to_embed}")
        console.print(f"    Estimated size:    ~{embed_size_mb:.1f} MB")
        if embed_eta_min < 1:
            console.print(f"    Estimated time:    ~{plan.articles_to_embed / 300:.0f} seconds")
        else:
            console.print(f"    Estimated time:    ~{embed_eta_min:.0f} minutes")

    # 5. Confirmation
    if not yes:
        from rich.prompt import Confirm

        if not Confirm.ask("\n  Proceed?", default=True):
            console.print("Aborted.")
            return

    # 6. Index with progress bar
    from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TextColumn("{task.completed}/{task.total}"),
        console=console,
    ) as progress:
        task = progress.add_task("Indexing", total=plan.estimated_articles or plan.articles_to_embed)

        def on_progress(phase: str, done: int, total: int):
            progress.update(task, description=f"[cyan]{phase}[/cyan]", completed=done, total=total)

        result = run_index(drive_path, db, strategy=strategy, on_progress=on_progress)

    # 7. Final stats
    stats = db.stats()
    console.print(f"\n[green]Indexing complete.[/green]")
    console.print(f"  Tier: {stats.get('tier', strategy)}")
    console.print(f"  Sources: {stats['source_count']}")
    console.print(f"  Articles: {stats['article_count']}")
    if strategy == "semantic":
        console.print(f"  Embeddings: {db.embedding_count()}")

    # Regenerate toolkit so search appears in run.sh menu
    from svalbard.manifest import Manifest

    manifest_path = drive_path / "manifest.yaml"
    if manifest_path.exists():
        from svalbard.toolkit_generator import generate_toolkit

        manifest = Manifest.load(manifest_path)
        generate_toolkit(drive_path, manifest.preset)
        console.print(f"  [dim]Toolkit updated.[/dim]")
