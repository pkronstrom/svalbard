#!/usr/bin/env python3
"""Download a Wikipedia ZIM and reprocess it with resized images.

Pipeline (all inside a single Docker container):
  1. zim-compact — read ZIM, resize images, extract content + redirects TSV
  2. zimwriterfs — repack into a new ZIM

Requirements: Docker with svalbard-tools:v2 image

Usage:
    python3 wikipedia-dithered-zim.py --source-zim input.zim --output out.zim --width 200 --quality 40
    python3 wikipedia-dithered-zim.py --source-url URL --output library/out.zim
"""

from __future__ import annotations

import argparse
import logging
import subprocess
import sys
import tempfile
from pathlib import Path
from urllib.parse import urlparse

import httpx

log = logging.getLogger("wikipedia-dithered")

DEFAULT_WIDTH = 200
DEFAULT_QUALITY = 40
DOCKER_IMAGE = "svalbard-tools:v2"
WORK_VOLUME = "svalbard-zim-work"

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
DEFAULT_OUTPUT_DIR = PROJECT_ROOT / "library"


def _docker_run(
    cmd: list[str],
    mounts: dict[str, str] | None = None,
    volumes: dict[str, str] | None = None,
    capture: bool = False,
) -> subprocess.CompletedProcess | None:
    """Run a command in the svalbard-tools container."""
    docker_cmd = ["docker", "run", "--rm"]
    for host_path, container_path in (mounts or {}).items():
        docker_cmd.extend(["-v", f"{host_path}:{container_path}"])
    for vol_name, container_path in (volumes or {}).items():
        docker_cmd.extend(["-v", f"{vol_name}:{container_path}"])
    docker_cmd.append(DOCKER_IMAGE)
    docker_cmd.extend(cmd)
    log.info("docker: %s", " ".join(cmd))
    if capture:
        return subprocess.run(docker_cmd, capture_output=True, text=True)
    subprocess.run(docker_cmd, check=True)
    return None


def _volume_create(name: str) -> None:
    subprocess.run(["docker", "volume", "create", name], capture_output=True, check=True)


def _volume_remove(name: str) -> None:
    subprocess.run(["docker", "volume", "rm", "-f", name], capture_output=True)


def download_zim(url: str, dest: Path) -> Path:
    """Download a ZIM file with resume support."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    existing_size = dest.stat().st_size if dest.exists() else 0
    headers = {}
    if existing_size > 0:
        headers["Range"] = f"bytes={existing_size}-"

    with httpx.stream("GET", url, headers=headers, follow_redirects=True, timeout=60) as r:
        if r.status_code == 416:
            log.info("already downloaded: %s", dest.name)
            return dest
        total = int(r.headers.get("content-length", 0))
        if r.status_code == 206:
            mode = "ab"
            log.info("resuming %s (%d MB remaining)", dest.name, total // (1024 * 1024))
        else:
            mode = "wb"
            existing_size = 0
            log.info("downloading %s (%d MB)", dest.name, total // (1024 * 1024))
        r.raise_for_status()
        downloaded = 0
        with open(dest, mode) as f:
            for chunk in r.iter_bytes(chunk_size=1024 * 1024):
                f.write(chunk)
                downloaded += len(chunk)
                if total > 0 and downloaded % (50 * 1024 * 1024) == 0:
                    pct = (existing_size + downloaded) / (existing_size + total) * 100
                    log.info("  %.0f%%", pct)
    log.info("download complete: %s", dest.name)
    return dest


def rewrite_zim(
    source_path: Path,
    output_path: Path,
    *,
    max_width: int = DEFAULT_WIDTH,
    quality: int = DEFAULT_QUALITY,
) -> None:
    """Rewrite a ZIM with resized images via Docker."""
    output_path.parent.mkdir(parents=True, exist_ok=True)

    _volume_remove(WORK_VOLUME)
    _volume_create(WORK_VOLUME)

    mounts = {
        str(source_path.parent.resolve()): "/input",
        str(output_path.parent.resolve()): "/output",
    }
    volumes = {WORK_VOLUME: "/work"}
    source_name = source_path.name
    output_name = output_path.name

    # Phase 1: zim-compact reads ZIM, resizes images, writes content + redirects
    log.info("phase 1: zim-compact (width=%d, quality=%d)", max_width, quality)
    result = _docker_run(
        [
            "zim-compact",
            f"--width={max_width}",
            f"--quality={quality}",
            f"--redirects=/work/redirects.tsv",
            f"/input/{source_name}",
            "/work/extracted",
        ],
        mounts=mounts, volumes=volumes, capture=True,
    )

    if result and result.returncode != 0:
        log.error("zim-compact failed:\n%s\n%s", result.stdout, result.stderr)
        raise RuntimeError("zim-compact failed")

    # Parse metadata from zim-compact stdout
    meta = {}
    for line in (result.stdout if result else "").splitlines():
        if "=" in line:
            key, _, val = line.partition("=")
            meta[key.strip()] = val.strip()
    log.info("zim-compact stderr:\n%s", result.stderr if result else "")

    main_page = meta.get("main_page", "index")
    language = meta.get("language", "eng")
    title = meta.get("title", "Wikipedia (compact)")
    description = meta.get("description", "Wikipedia with resized images")
    creator = meta.get("creator", "Wikipedia contributors")

    # Phase 2: zimwriterfs repacks into ZIM
    log.info("phase 2: zimwriterfs (main=%s)", main_page)
    _docker_run(
        [
            "sh", "-c",
            f"""zimwriterfs \
  '--welcome={main_page}' \
  --illustration=illustration.png \
  '--language={language}' \
  '--title={title}' \
  '--description={description}' \
  '--creator={creator}' \
  --publisher=Svalbard \
  --name=wikipedia-compact \
  --withoutFTIndex \
  --redirects=/work/redirects.tsv \
  '--threads='$(nproc) \
  /work/extracted \
  '/output/{output_name}'""",
        ],
        mounts=mounts, volumes=volumes,
    )

    _volume_remove(WORK_VOLUME)
    log.info("done: %s", output_path)


def main():
    parser = argparse.ArgumentParser(description="Download and reprocess a Wikipedia ZIM")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--source-url", help="URL to download source ZIM")
    source.add_argument("--source-zim", type=Path, help="Path to existing source ZIM")
    parser.add_argument(
        "--output", type=Path,
        default=DEFAULT_OUTPUT_DIR / "wikipedia-en-medicine-compact.zim",
    )
    parser.add_argument("--width", type=int, default=DEFAULT_WIDTH)
    parser.add_argument("--quality", type=int, default=DEFAULT_QUALITY)
    parser.add_argument("--workdir", type=Path, help="Download directory")
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(name)s %(message)s",
        datefmt="%H:%M:%S",
    )

    if args.source_zim:
        source_zim = args.source_zim
        if not source_zim.exists():
            log.error("not found: %s", source_zim)
            sys.exit(1)
    else:
        work = args.workdir or Path(tempfile.mkdtemp(prefix="zim-dither-"))
        filename = Path(urlparse(args.source_url).path).name
        source_zim = download_zim(args.source_url, work / filename)

    rewrite_zim(source_zim, args.output, max_width=args.width, quality=args.quality)

    in_mb = source_zim.stat().st_size / (1024 * 1024)
    out_mb = args.output.stat().st_size / (1024 * 1024)
    log.info("result: %.0f MB → %.0f MB (%.0f%%)", in_mb, out_mb, out_mb / in_mb * 100)


if __name__ == "__main__":
    main()
