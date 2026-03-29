"""Laptop provisioning — install desktop apps via Homebrew."""

import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path

from rich.console import Console
from rich.prompt import Confirm
from rich.table import Table

console = Console()

BREWFILE = Path(__file__).parent.parent.parent / "Brewfile"


@dataclass
class BrewEntry:
    kind: str       # "brew" or "cask"
    name: str       # formula/cask name
    comment: str    # description from inline comment
    category: str   # section header


def parse_brewfile(path: Path = BREWFILE) -> list[BrewEntry]:
    """Parse Brewfile into structured entries."""
    entries = []
    current_category = ""
    for line in path.read_text().splitlines():
        stripped = line.strip()
        if stripped.startswith("#") and not any(stripped.startswith(f"# {kw}") for kw in ["Run:", "CLI", "Desktop"]):
            # Section header comment
            candidate = stripped.lstrip("# ").strip()
            if candidate and not candidate.startswith("Run:") and not candidate.startswith("CLI ") and not candidate.startswith("Desktop "):
                current_category = candidate
            continue
        if not stripped.startswith(("brew ", "cask ")):
            continue
        # Parse: brew "name"  # comment  OR  cask "name"  # comment
        kind = stripped.split()[0]
        name_part = stripped.split('"')[1] if '"' in stripped else ""
        comment = stripped.split("# ", 1)[1].strip() if "# " in stripped else ""
        if name_part:
            entries.append(BrewEntry(kind=kind, name=name_part, comment=comment, category=current_category))
    return entries


def check_brew_installed() -> bool:
    return shutil.which("brew") is not None


def check_already_installed(entry: BrewEntry) -> bool:
    """Check if a brew formula or cask is already installed."""
    try:
        if entry.kind == "cask":
            result = subprocess.run(
                ["brew", "list", "--cask", entry.name],
                capture_output=True, timeout=10,
            )
        else:
            result = subprocess.run(
                ["brew", "list", "--formula", entry.name],
                capture_output=True, timeout=10,
            )
        return result.returncode == 0
    except Exception:
        return False


def install_brew() -> bool:
    """Guide user through Homebrew installation."""
    console.print("\n[yellow]Homebrew is not installed.[/yellow]")
    console.print("  Homebrew is required to install desktop apps.")
    console.print("  Install it from [bold]https://brew.sh[/bold]\n")
    console.print('  Run: [bold]/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"[/bold]\n')
    if Confirm.ask("  Run this now?", default=True):
        try:
            subprocess.run(
                ["/bin/bash", "-c", 'NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'],
                check=True,
            )
            console.print("[green]Homebrew installed.[/green]\n")
            return True
        except (subprocess.CalledProcessError, KeyboardInterrupt):
            console.print("[red]Homebrew installation failed. Install manually and re-run.[/red]")
            return False
    return False


def run_provision():
    """Interactive app provisioning via Homebrew."""
    console.print("\n[bold]Svalbard — Laptop Provisioning[/bold]")
    console.print("  Install desktop apps while you still have internet.\n")

    # Check brew
    if not check_brew_installed():
        if not install_brew():
            return
        if not check_brew_installed():
            console.print("[red]brew not found in PATH. Restart your terminal and try again.[/red]")
            return

    # Parse Brewfile
    if not BREWFILE.exists():
        console.print(f"[red]Brewfile not found at {BREWFILE}[/red]")
        return
    entries = parse_brewfile()
    if not entries:
        console.print("[yellow]No entries found in Brewfile.[/yellow]")
        return

    # Check what's already installed
    console.print("  Checking installed apps...\n")
    installed = set()
    for entry in entries:
        if check_already_installed(entry):
            installed.add(entry.name)

    # Display table with selection
    table = Table(title="Available Apps", show_lines=False, pad_edge=False)
    table.add_column("#", style="dim", width=3)
    table.add_column("App", min_width=22)
    table.add_column("Type", width=5, style="dim")
    table.add_column("Description")
    table.add_column("Status", width=12)

    current_cat = ""
    for i, entry in enumerate(entries, 1):
        if entry.category != current_cat:
            current_cat = entry.category
            table.add_section()
            table.add_row("", f"[bold]{current_cat}[/bold]", "", "", "")
        status = "[green]installed[/green]" if entry.name in installed else ""
        table.add_row(str(i), entry.name, entry.kind, entry.comment, status)

    console.print(table)

    not_installed = [e for e in entries if e.name not in installed]
    if not not_installed:
        console.print("\n[green]All apps are already installed.[/green]")
        return

    # Selection prompt
    console.print(f"\n  [bold]{len(not_installed)}[/bold] apps not yet installed.")
    console.print("  Enter numbers to toggle, [bold]a[/bold] = all, [bold]Enter[/bold] = install selected\n")

    # Default: select all not-installed
    selected = {e.name for e in not_installed}

    while True:
        # Show current selection
        labels = []
        for i, entry in enumerate(entries, 1):
            if entry.name in installed:
                continue
            marker = "[green]x[/green]" if entry.name in selected else " "
            labels.append(f"  [{marker}] {i:>2}. {entry.name}")
        console.print("\n".join(labels))

        choice = console.input("\n  Toggle #, [bold]a[/bold]ll, [bold]n[/bold]one, [bold]Enter[/bold] to install: ").strip().lower()

        if choice == "":
            break
        elif choice == "a":
            selected = {e.name for e in not_installed}
        elif choice == "n":
            selected.clear()
        else:
            # Parse space-separated numbers
            for part in choice.replace(",", " ").split():
                try:
                    idx = int(part)
                    if 1 <= idx <= len(entries):
                        name = entries[idx - 1].name
                        if name in installed:
                            continue
                        if name in selected:
                            selected.discard(name)
                        else:
                            selected.add(name)
                except ValueError:
                    pass

    if not selected:
        console.print("  Nothing selected.")
        return

    # Install
    to_install = [e for e in entries if e.name in selected]
    console.print(f"\n[bold]Installing {len(to_install)} app(s)...[/bold]\n")

    for entry in to_install:
        console.print(f"  Installing [bold]{entry.name}[/bold]...")
        try:
            cmd = ["brew", "install"]
            if entry.kind == "cask":
                cmd.append("--cask")
            cmd.append(entry.name)
            subprocess.run(cmd, check=True)
            console.print(f"  [green]{entry.name} installed.[/green]")
        except subprocess.CalledProcessError:
            console.print(f"  [red]{entry.name} failed.[/red]")
        except KeyboardInterrupt:
            console.print("\n  [yellow]Cancelled.[/yellow]")
            return

    console.print("\n[green]Done.[/green]")
