"""Media ingest runner for video/audio sources."""

from __future__ import annotations

import subprocess
from pathlib import Path


MEDIA_IMAGE = "svalbard-media"
PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent


def ensure_media_image() -> bool:
    """Build the media Docker image if it does not already exist."""
    result = subprocess.run(["docker", "image", "inspect", MEDIA_IMAGE], capture_output=True)
    if result.returncode == 0:
        return True
    dockerfile = PROJECT_ROOT / "docker" / "media" / "Dockerfile"
    if not dockerfile.exists():
        return False
    result = subprocess.run(
        ["docker", "build", "-t", MEDIA_IMAGE, str(dockerfile.parent)],
        capture_output=True,
        text=True,
    )
    return result.returncode == 0


def probe_media_url(url: str, *, runner: str = "auto") -> bool:
    """Return True when the media backend can resolve the URL."""
    if runner not in {"auto", "docker"}:
        return False
    if not ensure_media_image():
        return False
    result = subprocess.run(
        ["docker", "run", "--rm", MEDIA_IMAGE, "probe", "--url", url],
        capture_output=True,
        text=True,
    )
    return result.returncode == 0


def run_media_ingest(
    url: str,
    output_name: str,
    workspace_root: Path,
    *,
    quality: str = "720p",
    audio_only: bool = False,
    runner: str = "docker",
) -> Path:
    """Run the media ingest backend and return the generated artifact path."""
    if runner != "docker":
        raise ValueError("Media ingest currently supports only the docker runner")
    if not ensure_media_image():
        raise RuntimeError("Failed to build media ingest image.")

    output_path = workspace_root / "generated" / output_name
    staging_dir = workspace_root / "generated" / ".staging" / output_path.stem
    output_path.parent.mkdir(parents=True, exist_ok=True)
    staging_dir.mkdir(parents=True, exist_ok=True)

    cmd = [
        "docker",
        "run",
        "--rm",
        "-v",
        f"{workspace_root}:/workspace",
        MEDIA_IMAGE,
        "build",
        "--url",
        url,
        "--output",
        f"/workspace/generated/{output_name}",
        "--staging",
        f"/workspace/generated/.staging/{output_path.stem}",
        "--quality",
        quality,
    ]
    if audio_only:
        cmd.append("--audio-only")
    result = subprocess.run(cmd)
    if result.returncode != 0:
        raise RuntimeError(f"Media ingest failed for {url}")
    return output_path
