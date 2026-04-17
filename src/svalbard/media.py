"""Media ingest runner for video/audio sources."""

from __future__ import annotations

import subprocess
from pathlib import Path

from svalbard.docker import TOOLS_IMAGE, has_docker, ensure_tools_image


def probe_media_url(url: str, *, runner: str = "auto") -> bool:
    """Return True when the media backend can resolve the URL."""
    if runner not in {"auto", "docker"}:
        return False
    if not has_docker() or not ensure_tools_image():
        return False
    result = subprocess.run(
        [
            "docker", "run", "--rm", TOOLS_IMAGE,
            "python", "/usr/local/bin/build-media-zim.py",
            "probe", "--url", url,
        ],
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
    if not has_docker() or not ensure_tools_image():
        raise RuntimeError("Docker is not available or failed to build tools image.")

    output_path = workspace_root / "library" / output_name
    staging_dir = workspace_root / "library" / ".staging" / output_path.stem
    output_path.parent.mkdir(parents=True, exist_ok=True)
    staging_dir.mkdir(parents=True, exist_ok=True)

    cmd = [
        "docker", "run", "--rm",
        "-v", f"{workspace_root}:/workspace",
        TOOLS_IMAGE,
        "python", "/usr/local/bin/build-media-zim.py",
        "build",
        "--url", url,
        "--output", f"/workspace/library/{output_name}",
        "--staging", f"/workspace/library/.staging/{output_path.stem}",
        "--quality", quality,
    ]
    if audio_only:
        cmd.append("--audio-only")
    result = subprocess.run(cmd)
    if result.returncode != 0:
        raise RuntimeError(f"Media ingest failed for {url}")
    return output_path
