"""Website-to-ZIM crawler using Zimit (Docker-based)."""

import re
import subprocess
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

import yaml
from rich.console import Console

console = Console()

ZIMIT_IMAGE = "ghcr.io/openzim/zimit:3.0"


def _slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "crawl"


@dataclass
class CrawlSite:
    url: str
    scope: str = "domain"  # page, prefix, host, domain
    max_pages: int = 0


@dataclass
class CrawlConfig:
    name: str
    output: str  # relative path, e.g. "generated/nordic-emergency.zim"
    description: str = ""
    tags: list[str] = field(default_factory=list)
    sites: list[CrawlSite] = field(default_factory=list)
    max_size_mb: int = 0
    time_limit_minutes: int = 0


def load_crawl_config(path: Path) -> CrawlConfig:
    """Load a crawl config from YAML."""
    with open(path) as f:
        data = yaml.safe_load(f)

    sites = []
    for site in data.get("sites", []):
        site_data = dict(site)
        if "page_limit" in site_data and "max_pages" not in site_data:
            site_data["max_pages"] = site_data.pop("page_limit")
        sites.append(CrawlSite(**{k: v for k, v in site_data.items() if k in CrawlSite.__dataclass_fields__}))

    limits = data.get("limits") or data.get("defaults", {})
    output = data.get("output") or f"generated/{_slugify(data['name'])}.zim"
    return CrawlConfig(
        name=data["name"],
        output=output,
        description=data.get("description", ""),
        tags=data.get("tags", []),
        sites=sites,
        max_size_mb=limits.get("max_size_mb", limits.get("size_limit_mb", 0)),
        time_limit_minutes=limits.get("time_limit_minutes", limits.get("timeout_minutes", 0)),
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


def run_url_crawl(
    url: str,
    output_name: str,
    workspace_root: Path,
    *,
    scope: str = "domain",
    page_limit: int = 0,
    size_limit_mb: int = 0,
    time_limit_minutes: int = 0,
) -> Path:
    """Run a single direct URL crawl and return the generated artifact path."""
    output_path = workspace_root / "generated" / output_name
    output_path.parent.mkdir(parents=True, exist_ok=True)
    cmd = [
        "docker", "run", "--rm",
        "-v", f"{output_path.parent}:/output",
        ZIMIT_IMAGE,
        "--url", url,
        "--output", f"/output/{output_path.name}",
    ]
    if scope != "domain":
        cmd.extend(["--scopeType", scope])
    if page_limit > 0:
        cmd.extend(["--limit", str(page_limit)])
    if size_limit_mb > 0:
        cmd.extend(["--sizeLimit", str(size_limit_mb * 1024 * 1024)])
    if time_limit_minutes > 0:
        cmd.extend(["--timeLimit", str(time_limit_minutes * 60)])
    result = subprocess.run(cmd)
    if result.returncode != 0:
        raise RuntimeError(f"Crawl failed for {url}")
    return output_path


def register_generated_zim(
    workspace_root: Path,
    artifact_path: Path,
    origin_url: str,
    kind: str,
    runner: str,
    tool: str,
    quality: str = "",
    audio_only: bool = False,
    source_id: str | None = None,
) -> str:
    """Register a generated ZIM as a local source and write source metadata."""
    from svalbard.commands import add_local_source

    source_id = add_local_source(
        artifact_path,
        workspace_root=workspace_root,
        source_type="zim",
        source_id=source_id or artifact_path.stem,
    )
    slug = source_id.split(":", 1)[1]
    metadata_path = artifact_path.parent / f"{slug}.source.yaml"
    relative_artifact = artifact_path.relative_to(workspace_root).as_posix()
    metadata = {
        "artifact": relative_artifact,
        "origin_url": origin_url,
        "kind": kind,
        "runner": runner,
        "tool": tool,
        "created": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "size_bytes": artifact_path.stat().st_size,
    }
    if quality:
        metadata["quality"] = quality
    if audio_only:
        metadata["audio_only"] = True
    metadata_path.write_text(yaml.safe_dump(metadata, sort_keys=False))
    return source_id


def register_crawled_zim(
    workspace_root: Path,
    artifact_path: Path,
    origin_url: str,
    source_id: str | None = None,
) -> str:
    """Register a generated crawl artifact using the legacy web-specific metadata."""
    return register_generated_zim(
        workspace_root=workspace_root,
        artifact_path=artifact_path,
        origin_url=origin_url,
        kind="web",
        runner="docker",
        tool="zimit",
        source_id=source_id,
    )


def run_config_crawl(config_path: Path, workspace_root: Path) -> list[Path]:
    """Run crawl config jobs and return generated artifacts."""
    config = load_crawl_config(config_path)
    if len(config.sites) != 1:
        raise ValueError("Config crawl currently supports single-site configs only")
    artifacts: list[Path] = []
    output_name = Path(config.output).name
    for site in config.sites:
        artifact = run_url_crawl(
            site.url,
            output_name,
            workspace_root,
            scope=site.scope,
            page_limit=site.max_pages,
            size_limit_mb=config.max_size_mb,
            time_limit_minutes=config.time_limit_minutes,
        )
        register_crawled_zim(workspace_root, artifact, site.url)
        artifacts.append(artifact)
    return artifacts


def list_crawl_configs(crawl_dir: Path) -> list[Path]:
    """List available crawl configs (*.yaml, not *.yaml.example)."""
    if not crawl_dir.exists():
        return []
    return sorted(
        p for p in crawl_dir.glob("*.yaml")
        if not p.name.endswith(".example")
    )
