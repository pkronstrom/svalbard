"""Shared Docker helpers for the svalbard tools image."""

from __future__ import annotations

import logging
import subprocess
from pathlib import Path

log = logging.getLogger(__name__)

TOOLS_IMAGE = "svalbard-tools:v1"
_PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent


def has_docker() -> bool:
    """Return True if Docker is available and the daemon is running."""
    try:
        result = subprocess.run(
            ["docker", "info"], capture_output=True, timeout=10,
        )
        return result.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False


def ensure_tools_image() -> bool:
    """Build the svalbard-tools Docker image if not already present."""
    result = subprocess.run(
        ["docker", "image", "inspect", TOOLS_IMAGE],
        capture_output=True,
    )
    if result.returncode == 0:
        return True
    dockerfile = _PROJECT_ROOT / "Dockerfile"
    if not dockerfile.exists():
        log.warning("Dockerfile not found at %s", dockerfile)
        return False
    log.info("Building %s Docker image...", TOOLS_IMAGE)
    result = subprocess.run(
        ["docker", "build", "-t", TOOLS_IMAGE, "-f", str(dockerfile), str(_PROJECT_ROOT)],
        capture_output=True, text=True,
    )
    return result.returncode == 0


def run_container(
    cmd: list[str],
    mounts: dict[str, str] | None = None,
    **kwargs,
) -> subprocess.CompletedProcess:
    """Run a command in the svalbard-tools container with volume mounts."""
    docker_cmd = ["docker", "run", "--rm"]
    for host_path, container_path in (mounts or {}).items():
        docker_cmd.extend(["-v", f"{host_path}:{container_path}"])
    docker_cmd.append(TOOLS_IMAGE)
    docker_cmd.extend(cmd)
    log.info("Running: %s", " ".join(docker_cmd))
    return subprocess.run(docker_cmd, **kwargs)
