"""Svalbard CLI entry point."""

import click
from rich.console import Console

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
        from svalbard.wizard import run_wizard

        run_wizard()


@main.command()
def wizard() -> None:
    """Interactive setup wizard."""
    from svalbard.wizard import run_wizard

    run_wizard()


@main.command()
@click.argument("path")
@click.option("--preset", required=True, help="Preset name (e.g. nordic-128)")
def init(path: str, preset: str) -> None:
    """Initialize a drive with a preset."""
    from svalbard.commands import init_drive

    init_drive(path, preset)


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


@main.command()
@click.argument("config", required=False)
@click.option("--all", "crawl_all", is_flag=True, help="Run all configs in crawl/")
@click.option("--drive", default=".", help="Drive path for output")
def crawl(config: str | None, crawl_all: bool, drive: str) -> None:
    """Crawl websites into ZIM files using Zimit (requires Docker)."""
    from pathlib import Path as P

    from svalbard.crawler import (
        check_docker,
        ensure_zimit_image,
        list_crawl_configs,
        load_crawl_config,
        run_crawl,
    )

    if not check_docker():
        console.print("[red]Docker is not available. Install Docker to use svalbard crawl.[/red]")
        return

    if not ensure_zimit_image():
        console.print("[red]Failed to pull Zimit image.[/red]")
        return

    drive_path = P(drive)
    crawl_dir = P.cwd() / "crawl"

    if crawl_all:
        configs = list_crawl_configs(crawl_dir)
        if not configs:
            console.print("[yellow]No crawl configs found in crawl/[/yellow]")
            console.print("[dim]Copy a .yaml.example to .yaml and edit it.[/dim]")
            return
        for cfg_path in configs:
            cfg = load_crawl_config(cfg_path)
            console.print(f"\n[bold]Crawling: {cfg.name}[/bold]")
            run_crawl(cfg, drive_path)
    elif config:
        cfg_path = P(config)
        if not cfg_path.exists():
            cfg_path = crawl_dir / config
        if not cfg_path.exists():
            console.print(f"[red]Config not found: {config}[/red]")
            return
        cfg = load_crawl_config(cfg_path)
        console.print(f"[bold]Crawling: {cfg.name}[/bold]")
        run_crawl(cfg, drive_path)
    else:
        console.print("Usage: svalbard crawl <config.yaml> or svalbard crawl --all")
        configs = list_crawl_configs(crawl_dir)
        if configs:
            console.print(f"\nAvailable configs in crawl/:")
            for c in configs:
                console.print(f"  {c.name}")
