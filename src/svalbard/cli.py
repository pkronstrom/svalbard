"""Svalbard CLI entry point."""

import click
from rich.console import Console

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
def wizard() -> None:
    """Interactive setup wizard."""
    from svalbard.wizard import run_wizard

    run_wizard()


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
