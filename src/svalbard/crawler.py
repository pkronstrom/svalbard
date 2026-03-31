"""Website-to-ZIM helpers for the unified import flow."""

import subprocess
from datetime import datetime, timezone
from pathlib import Path

import yaml
from rich.console import Console

from svalbard.docker import has_docker

console = Console()

ZIMIT_IMAGE = "ghcr.io/openzim/zimit:3.0"


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
