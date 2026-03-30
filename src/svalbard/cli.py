"""Svalbard CLI entry point."""

import click
from rich.console import Console

from svalbard.add import run_add
from svalbard.crawler import (
    check_docker,
    ensure_zimit_image,
    register_crawled_zim,
    run_config_crawl,
    run_url_crawl,
)
from svalbard.local_sources import workspace_root as resolve_workspace_root

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
def wizard(path: str | None) -> None:
    """Interactive setup wizard."""
    from svalbard.wizard import run_wizard

    run_wizard(target_path=path)


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


@main.command("add")
@click.argument("input_value")
@click.option("--kind", type=click.Choice(["auto", "local", "web", "media"]), default="auto", show_default=True)
@click.option("--runner", type=click.Choice(["auto", "docker", "host"]), default="auto", show_default=True)
@click.option("--quality", type=click.Choice(["1080p", "720p", "480p", "360p", "source"]), default="720p", show_default=True)
@click.option("--audio-only", is_flag=True, help="Ingest as audio-only media where supported")
@click.option("-o", "--output", "output_name", default=None, help="Output artifact name")
@click.option("--workspace", default=None, help="Workspace root")
def add_command(
    input_value: str,
    kind: str,
    runner: str,
    quality: str,
    audio_only: bool,
    output_name: str | None,
    workspace: str | None,
) -> None:
    """Add local content or remote URLs as workspace-local sources."""
    source_id = run_add(
        input_value,
        workspace_root=resolve_workspace_root(workspace),
        kind=kind,
        runner=runner,
        quality=quality,
        audio_only=audio_only,
        output_name=output_name,
    )
    console.print(f"[green]Registered:[/green] {source_id}")


@main.group()
def crawl() -> None:
    """Crawl websites into workspace-local ZIM files using Zimit."""


@crawl.command("url")
@click.argument("url")
@click.option("-o", "--output", "output_name", required=True, help="Output ZIM filename")
@click.option("--workspace", default=None, help="Workspace root")
def crawl_url(url: str, output_name: str, workspace: str | None) -> None:
    """Crawl one URL and register the generated ZIM locally."""
    from pathlib import Path as P

    if not check_docker():
        console.print("[red]Docker is not available. Install Docker to use svalbard crawl.[/red]")
        return
    if not ensure_zimit_image():
        console.print("[red]Failed to pull Zimit image.[/red]")
        return

    root = resolve_workspace_root(workspace)
    artifact = run_url_crawl(url, output_name, root)
    source_id = register_crawled_zim(root, artifact, url)
    console.print(f"[green]Created local source:[/green] {source_id}")


@crawl.command("config")
@click.argument("config")
@click.option("--workspace", default=None, help="Workspace root")
def crawl_config(config: str, workspace: str | None) -> None:
    """Run a crawl config and register the resulting local sources."""
    from pathlib import Path as P

    if not check_docker():
        console.print("[red]Docker is not available. Install Docker to use svalbard crawl.[/red]")
        return
    if not ensure_zimit_image():
        console.print("[red]Failed to pull Zimit image.[/red]")
        return

    root = resolve_workspace_root(workspace)
    config_path = P(config)
    if not config_path.exists():
        console.print(f"[red]Config not found: {config}[/red]")
        return
    artifacts = run_config_crawl(config_path, root)
    console.print(f"[green]Generated {len(artifacts)} artifact(s).[/green]")


@main.group()
def local() -> None:
    """Manage workspace-local sources."""


@local.command("add")
@click.argument("path")
@click.option("--workspace", default=None, help="Workspace root")
@click.option("--type", "source_type", default=None, help="Source type override")
def local_add(path: str, workspace: str | None, source_type: str | None) -> None:
    """Register a local file or directory as a reusable local source."""
    from pathlib import Path as P

    from svalbard.commands import add_local_source

    source_id = add_local_source(P(path), workspace_root=resolve_workspace_root(workspace), source_type=source_type)
    console.print(f"[green]Registered:[/green] {source_id}")


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
    console.print(f"\n[bold]Index plan[/bold]")
    console.print(f"  Total ZIMs:       {plan.total_zims}")
    console.print(f"  New:              {plan.new_zims}")
    console.print(f"  Already indexed:  {plan.already_indexed}")
    console.print(f"  Changed:          {plan.changed_zims}")
    console.print(f"  Missing from disk:{plan.missing_zims}")

    # 5. Nothing to do?
    if not plan.files_to_index:
        console.print("\n[green]Index is up to date.[/green]")
        return

    # 6. Show estimated DB size
    est_mb = plan.estimated_db_bytes / (1024 * 1024)
    console.print(f"\n  Estimated DB size: {est_mb:.1f} MB")
    strategy_desc = {
        "fast": "indexes titles and article summaries — quick to build, good for simple keyword lookups",
        "standard": "indexes full article text — slower to build, finds matches deeper in content",
        "semantic": "adds AI-powered meaning search on top of standard — understands synonyms and related concepts, requires an embedding model",
    }
    console.print(f"  Strategy: [bold]{strategy}[/bold] — {strategy_desc[strategy]}")

    # 7. Confirmation
    if not yes:
        from rich.prompt import Confirm

        if not Confirm.ask("\n  Proceed with indexing?", default=True):
            console.print("Aborted.")
            return

    # 8. Index with progress bar
    from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskID

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TextColumn("{task.completed}/{task.total} articles"),
        console=console,
    ) as progress:
        task = progress.add_task("Indexing", total=plan.estimated_articles)

        def on_progress(filename: str, articles_done: int, total_files: int):
            progress.update(task, description=f"[cyan]{filename}[/cyan]")
            progress.update(task, completed=articles_done)

        result = run_index(drive_path, db, strategy=strategy, on_progress=on_progress)

    # 9. Final stats
    stats = db.stats()
    console.print(f"\n[green]Indexing complete.[/green]")
    console.print(f"  Sources: {stats['source_count']}")
    console.print(f"  Articles: {stats['article_count']}")
