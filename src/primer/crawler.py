"""Website-to-ZIM crawler using Zimit (Docker-based)."""

import subprocess
from dataclasses import dataclass, field
from pathlib import Path

import yaml
from rich.console import Console

console = Console()

ZIMIT_IMAGE = "ghcr.io/openzim/zimit:3.0"


@dataclass
class CrawlSite:
    url: str
    scope: str = "domain"  # page, prefix, host, domain
    max_pages: int = 0


@dataclass
class CrawlConfig:
    name: str
    output: str  # relative path, e.g. "zim/custom/nordic-emergency.zim"
    description: str = ""
    tags: list[str] = field(default_factory=list)
    sites: list[CrawlSite] = field(default_factory=list)
    max_size_mb: int = 0
    time_limit_minutes: int = 0


def load_crawl_config(path: Path) -> CrawlConfig:
    """Load a crawl config from YAML."""
    with open(path) as f:
        data = yaml.safe_load(f)

    sites = [CrawlSite(**s) for s in data.get("sites", [])]

    limits = data.get("limits", {})
    return CrawlConfig(
        name=data["name"],
        output=data["output"],
        description=data.get("description", ""),
        tags=data.get("tags", []),
        sites=sites,
        max_size_mb=limits.get("max_size_mb", 0),
        time_limit_minutes=limits.get("time_limit_minutes", 0),
    )


def check_docker() -> bool:
    """Check if Docker is available and running."""
    try:
        result = subprocess.run(
            ["docker", "info"],
            capture_output=True, timeout=10,
        )
        return result.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False


def ensure_zimit_image() -> bool:
    """Pull Zimit image if not already present."""
    result = subprocess.run(
        ["docker", "image", "inspect", ZIMIT_IMAGE],
        capture_output=True,
    )
    if result.returncode == 0:
        return True

    console.print(f"[bold]Pulling {ZIMIT_IMAGE}...[/bold]")
    result = subprocess.run(["docker", "pull", ZIMIT_IMAGE])
    return result.returncode == 0


def run_crawl(config: CrawlConfig, drive_path: Path) -> bool:
    """Run a Zimit crawl for a single config. Returns True on success."""
    output_path = drive_path / config.output
    output_path.parent.mkdir(parents=True, exist_ok=True)

    if output_path.exists():
        console.print(f"  [dim]Already exists: {config.output}[/dim]")
        return True

    success = True
    for site in config.sites:
        console.print(f"  [bold]Crawling:[/bold] {site.url}")

        cmd = [
            "docker", "run", "--rm",
            "-v", f"{output_path.parent}:/output",
            ZIMIT_IMAGE,
            "--url", site.url,
            "--output", f"/output/{output_path.name}",
        ]

        if site.scope != "domain":
            cmd.extend(["--scopeType", site.scope])
        if site.max_pages > 0:
            cmd.extend(["--limit", str(site.max_pages)])
        if config.max_size_mb > 0:
            cmd.extend(["--sizeLimit", str(config.max_size_mb * 1024 * 1024)])
        if config.time_limit_minutes > 0:
            cmd.extend(["--timeLimit", str(config.time_limit_minutes * 60)])

        result = subprocess.run(cmd)
        if result.returncode != 0:
            console.print(f"  [red]Crawl failed: {site.url}[/red]")
            success = False

    return success


def list_crawl_configs(crawl_dir: Path) -> list[Path]:
    """List available crawl configs (*.yaml, not *.yaml.example)."""
    if not crawl_dir.exists():
        return []
    return sorted(
        p for p in crawl_dir.glob("*.yaml")
        if not p.name.endswith(".example")
    )
