"""Primer CLI entry point."""

import click
from rich.console import Console

console = Console()


@click.group(invoke_without_command=True)
@click.pass_context
def main(ctx: click.Context) -> None:
    """Primer — Offline knowledge kit provisioner."""
    if ctx.invoked_subcommand is None:
        console.print(
            "\n[bold]Welcome to Primer![/bold]\n"
            "\nGet started by running [cyan]primer wizard[/cyan] "
            "to set up your first knowledge kit.\n"
        )


@main.command()
def wizard() -> None:
    """Interactive setup wizard for a new knowledge kit."""
    console.print("[yellow]wizard:[/yellow] not yet implemented")


@main.command()
@click.argument("path")
@click.option("--preset", default=None, help="Preset configuration to use.")
def init(path: str, preset: str | None) -> None:
    """Initialise a new knowledge kit at PATH."""
    console.print("[yellow]init:[/yellow] not yet implemented")


@main.command()
@click.argument("path", default=".")
def sync(path: str) -> None:
    """Download / update assets for the kit at PATH."""
    console.print("[yellow]sync:[/yellow] not yet implemented")


@main.command()
@click.argument("path", default=".")
def status(path: str) -> None:
    """Show status of the kit at PATH."""
    console.print("[yellow]status:[/yellow] not yet implemented")


@main.command()
@click.argument("path", default=".")
def audit(path: str) -> None:
    """Audit the kit at PATH for integrity and freshness."""
    console.print("[yellow]audit:[/yellow] not yet implemented")
