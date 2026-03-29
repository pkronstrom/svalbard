"""Primer CLI entry point."""

import click
from rich.console import Console

console = Console()


@click.group(invoke_without_command=True)
@click.pass_context
def main(ctx: click.Context) -> None:
    """Primer — Offline knowledge kit provisioner."""
    if ctx.invoked_subcommand is None:
        from pathlib import Path

        from primer.manifest import Manifest

        cwd = Path.cwd()
        if Manifest.exists(cwd):
            from primer.commands import show_status

            show_status(str(cwd))
            _show_menu(str(cwd))
        else:
            console.print("[bold]Primer[/bold] -- Offline knowledge kit provisioner")
            console.print("\nNo drive found. Run [bold]primer wizard[/bold] to get started.")
            console.print("Run [bold]primer --help[/bold] for all commands.")


def _show_menu(path: str):
    console.print("\n  [bold][s][/bold] Sync (check for updates)")
    console.print("  [bold][a][/bold] Audit report")
    console.print("  [bold][w][/bold] Wizard (reconfigure)")
    console.print("  [bold][q][/bold] Quit")
    choice = console.input("\n  > ")
    if choice == "s":
        from primer.commands import sync_drive

        sync_drive(path)
    elif choice == "a":
        console.print("[yellow]audit:[/yellow] not yet implemented")
    elif choice == "w":
        from primer.wizard import run_wizard

        run_wizard()


@main.command()
def wizard() -> None:
    """Interactive setup wizard."""
    from primer.wizard import run_wizard

    run_wizard()


@main.command()
@click.argument("path")
@click.option("--preset", required=True, help="Preset name (e.g. nordic-128)")
def init(path: str, preset: str) -> None:
    """Initialize a drive with a preset."""
    from primer.commands import init_drive

    init_drive(path, preset)


@main.command()
@click.argument("path", default=".")
def sync(path: str) -> None:
    """Download/update content on initialized drive."""
    from primer.commands import sync_drive

    sync_drive(path)


@main.command()
@click.argument("path", default=".")
def status(path: str) -> None:
    """Show what's downloaded, what's stale."""
    from primer.commands import show_status

    show_status(path)


@main.command()
@click.argument("path", default=".")
def audit(path: str) -> None:
    """Generate LLM-ready gap analysis report."""
    console.print("[yellow]audit:[/yellow] not yet implemented")
