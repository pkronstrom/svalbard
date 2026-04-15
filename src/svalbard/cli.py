"""Svalbard CLI entry point."""

import shutil
from pathlib import Path

import click
from rich.console import Console

from svalbard.commands import sync_drive
from svalbard.drive_config import load_snapshot_preset, write_drive_snapshot
from svalbard.importer import run_import
from svalbard.bundle import run_bundle_build
from svalbard.attach import attach_local_source, detach_local_source, resolve_drive_path
from svalbard.manifest import Manifest
from svalbard.paths import workspace_root as resolve_workspace_root
from svalbard.presets import copy_preset_to_workspace, list_presets, load_preset
from svalbard.wizard import pick_sources_via_packs, run_wizard, write_custom_preset

console = Console()


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


@main.command("attach")
@click.argument("source_id", required=False)
@click.argument("path", required=False, default=".")
@click.option("--browse", is_flag=True, help="Browse and select pack content interactively")
@click.option("--workspace", default=None, help="Workspace root")
def attach_command(
    source_id: str | None,
    path: str,
    browse: bool,
    workspace: str | None,
) -> None:
    """Attach a local source or browse pack content for an existing drive."""
    if browse:
        drive_arg = source_id if source_id and path == "." else path
        drive_path = resolve_drive_path(drive_arg)
        manifest_path = drive_path / "manifest.yaml"
        manifest = Manifest.load(manifest_path)
        workspace_root = (
            resolve_workspace_root(workspace)
            if workspace is not None
            else Path(manifest.workspace_root).resolve()
            if manifest.workspace_root
            else resolve_workspace_root()
        )
        try:
            free_gb = shutil.disk_usage(drive_path).free / 1e9
        except OSError:
            free_gb = 0
        try:
            preset = load_preset(manifest.preset, workspace=workspace_root)
        except (FileNotFoundError, ValueError, KeyError):
            preset = load_snapshot_preset(drive_path)
        if preset is not None:
            checked_ids = {source.id for source in preset.sources if not source.id.startswith("local:")}
        else:
            checked_ids = {entry.id for entry in manifest.entries if not entry.id.startswith("local:")}
        selected_ids = pick_sources_via_packs(
            free_gb=free_gb,
            workspace=workspace_root,
            checked_ids=checked_ids,
        )
        preset_name = write_custom_preset(
            selected_ids,
            workspace=workspace_root,
            target_size_gb=free_gb,
            region=manifest.region,
            description=f"Attached selection for {drive_path.name}",
        )
        write_drive_snapshot(
            drive_path,
            preset_name=preset_name,
            workspace_root=workspace_root,
            local_source_ids=manifest.local_sources,
        )
        manifest.preset = preset_name
        manifest.save(manifest_path)
        sync_drive(str(drive_path))
        console.print(f"[green]Updated preset:[/green] {preset_name}")
        return

    if not source_id:
        raise click.UsageError("SOURCE_ID is required unless --browse is used")

    drive_path = resolve_drive_path(path)
    attach_local_source(drive_path, source_id, workspace=workspace)
    console.print(f"[green]Attached:[/green] {source_id}")


@main.command("detach")
@click.argument("source_id")
@click.argument("path", required=False, default=".")
@click.option("--workspace", default=None, help="Workspace root")
def detach_command(source_id: str, path: str, workspace: str | None) -> None:
    """Detach a local source from an existing drive."""
    drive_path = resolve_drive_path(path)
    detach_local_source(drive_path, source_id)
    console.print(f"[green]Detached:[/green] {source_id}")


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
