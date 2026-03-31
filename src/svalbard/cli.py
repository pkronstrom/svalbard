"""Svalbard CLI entry point."""

import click
from rich.console import Console

from svalbard.importer import run_import
from svalbard.attach import attach_local_source, detach_local_source, resolve_drive_path
from svalbard.paths import workspace_root as resolve_workspace_root
from svalbard.presets import copy_preset_to_workspace, list_presets

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
                from svalbard.wizard import run_wizard

                run_wizard()
            else:
                console.print("Run [bold]svalbard --help[/bold] for all commands.")


def _show_menu(path: str):
    console.print("\n  [bold][s][/bold] Sync (check for updates)")
    console.print("  [bold][a][/bold] Audit report")
    console.print("  [bold][p][/bold] Provision laptop (install apps)")
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
    elif choice == "p":
        from svalbard.provision import run_provision

        run_provision()
    elif choice == "w":
        from svalbard.wizard import run_wizard

        run_wizard()


@main.command()
@click.option("--path", default=None, help="Target drive path (skip target selection)")
@click.option("--preset", default=None, help="Preset name (skip region and preset selection)")
def wizard(path: str | None, preset: str | None) -> None:
    """Interactive setup wizard."""
    from svalbard.wizard import run_wizard

    run_wizard(target_path=path, preset_name=preset)


@main.command()
@click.argument("path")
@click.option("--preset", required=True, help="Preset name (e.g. finland-128)")
@click.option("--workspace", default=None, help="Workspace root")
def init(path: str, preset: str, workspace: str | None) -> None:
    """Initialize a drive with a preset."""
    from svalbard.commands import init_drive

    init_drive(path, preset, workspace_root=str(resolve_workspace_root(workspace)))


@main.command()
@click.argument("path", default=".")
@click.option("--update", is_flag=True, help="Check for and download newer versions")
@click.option("--force", is_flag=True, help="Re-download everything")
def sync(path: str, update: bool, force: bool) -> None:
    """Download/update content on initialized drive."""
    from svalbard.commands import sync_drive

    sync_drive(path, update=update, force=force)


@main.command()
@click.argument("path", default=".")
@click.option("--check", is_flag=True, help="Check for available updates (hits network)")
def status(path: str, check: bool) -> None:
    """Show what's downloaded, what's stale."""
    from svalbard.commands import show_status

    show_status(path, check_updates=check)


@main.command()
def provision() -> None:
    """Install desktop apps on this laptop (via Homebrew)."""
    from svalbard.provision import run_provision

    run_provision()


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
@click.option("--kind", type=click.Choice(["auto", "local", "web", "media"]), default="auto", show_default=True)
@click.option("--runner", type=click.Choice(["auto", "docker", "host"]), default="auto", show_default=True)
@click.option("--quality", type=click.Choice(["1080p", "720p", "480p", "360p", "source"]), default="720p", show_default=True)
@click.option("--audio-only", is_flag=True, help="Ingest as audio-only media where supported")
@click.option("-o", "--output", "output_name", default=None, help="Output artifact name")
@click.option("--workspace", default=None, help="Workspace root")
def import_command(
    input_values: tuple[str, ...],
    kind: str,
    runner: str,
    quality: str,
    audio_only: bool,
    output_name: str | None,
    workspace: str | None,
) -> None:
    """Import local content or remote URLs as workspace-local sources."""
    if len(input_values) > 1:
        console.print("[red]Error:[/red] multiple inputs require --bundle NAME")
        raise SystemExit(1)
    source_id = run_import(
        input_values[0],
        workspace_root=resolve_workspace_root(workspace),
        kind=kind,
        runner=runner,
        quality=quality,
        audio_only=audio_only,
        output_name=output_name,
    )
    console.print(f"[green]Imported:[/green] {source_id}")


@main.command("attach")
@click.argument("source_id")
@click.argument("path", required=False, default=".")
@click.option("--workspace", default=None, help="Workspace root")
def attach_command(source_id: str, path: str, workspace: str | None) -> None:
    """Attach a local source to an existing drive."""
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
