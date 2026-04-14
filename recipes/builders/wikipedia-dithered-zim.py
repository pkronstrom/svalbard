#!/usr/bin/env python3
"""Download a Wikipedia ZIM and reprocess it with resized/dithered images.

Uses the svalbard-tools Docker container for all heavy operations:
  1. zimdump  — extract ZIM to filesystem
  2. zim-dither — resize (and optionally dither) all images
  3. zimwriterfs — repack into a new ZIM

Requirements: Docker with svalbard-tools:v2 image

Usage:
    python3 wikipedia-dithered-zim.py --source-url URL --output library/out.zim
    python3 wikipedia-dithered-zim.py --source-zim input.zim --output library/out.zim
    python3 wikipedia-dithered-zim.py --source-zim input.zim --output out.zim --no-dither --width 200 --quality 40
"""

from __future__ import annotations

import argparse
import json
import logging
import subprocess
import sys
import tempfile
from pathlib import Path
from urllib.parse import urlparse

import httpx

log = logging.getLogger("wikipedia-dithered")

DEFAULT_WIDTH = 400
DEFAULT_COLORS = 8
DEFAULT_QUALITY = 40
DOCKER_IMAGE = "svalbard-tools:v2"

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
DEFAULT_OUTPUT_DIR = PROJECT_ROOT / "library"


def _docker_run(cmd: list[str], mounts: dict[str, str] | None = None) -> None:
    """Run a command in the svalbard-tools container."""
    docker_cmd = ["docker", "run", "--rm"]
    for host_path, container_path in (mounts or {}).items():
        docker_cmd.extend(["-v", f"{host_path}:{container_path}"])
    docker_cmd.append(DOCKER_IMAGE)
    docker_cmd.extend(cmd)
    log.info("docker: %s", " ".join(cmd))
    subprocess.run(docker_cmd, check=True)


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
    num_colors: int = DEFAULT_COLORS,
    quality: int = DEFAULT_QUALITY,
    no_dither: bool = False,
) -> None:
    """Rewrite a ZIM with resized/dithered images via Docker."""
    work_dir = Path(tempfile.mkdtemp(prefix="zim-reimage-"))
    extract_dir = work_dir / "extracted"
    extract_dir.mkdir()
    output_path.parent.mkdir(parents=True, exist_ok=True)

    mounts = {
        str(source_path.parent.resolve()): "/input",
        str(work_dir): "/work",
        str(output_path.parent.resolve()): "/output",
    }

    source_name = source_path.name
    output_name = output_path.name

    # Phase 1: Extract ZIM
    log.info("phase 1: extracting %s", source_name)
    _docker_run(
        ["zimdump", "dump", "--dir=/work/extracted", f"/input/{source_name}"],
        mounts=mounts,
    )

    # Phase 2: Resize/dither images
    mode = "resize-only" if no_dither else "dither"
    log.info("phase 2: %s (width=%d, quality=%d)", mode, max_width, quality)
    dither_cmd = [
        "zim-dither", "batch",
        "--width", str(max_width),
        "--quality", str(quality),
    ]
    if no_dither:
        dither_cmd.append("--no-dither")
    else:
        dither_cmd.extend(["--colors", str(num_colors), "--dither", "bayer"])
    dither_cmd.append("/work/extracted")
    _docker_run(dither_cmd, mounts=mounts)

    # Find the main page and illustration from the source ZIM
    log.info("phase 2.5: reading source ZIM metadata")
    result = subprocess.run(
        ["docker", "run", "--rm",
         "-v", f"{source_path.parent.resolve()}:/input",
         DOCKER_IMAGE, "zimdump", "info", f"/input/{source_name}"],
        capture_output=True, text=True,
    )
    main_page = "index"
    illustration = ""
    for line in result.stdout.splitlines():
        if line.startswith("main page:"):
            main_page = line.split(":", 1)[1].strip()
        if line.startswith("favicon:"):
            illustration = line.split(":", 1)[1].strip()

    # Create a simple illustration if the extracted one isn't a standard file
    illustration_flag = []
    illust_path = extract_dir / illustration
    if illust_path.exists():
        illustration_flag = [f"--illustration={illustration}"]
    else:
        # Create a minimal 48x48 PNG
        import struct, zlib
        def _minimal_png() -> bytes:
            width, height = 48, 48
            raw = b""
            for _ in range(height):
                raw += b"\x00" + b"\x80\x80\x80" * width
            compressed = zlib.compress(raw)
            def chunk(ctype, data):
                c = ctype + data
                return struct.pack(">I", len(data)) + c + struct.pack(">I", zlib.crc32(c) & 0xffffffff)
            ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
            return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", compressed) + chunk(b"IEND", b"")
        (extract_dir / "illustration.png").write_bytes(_minimal_png())
        illustration_flag = ["--illustration=illustration.png"]

    # Phase 3: Repack into ZIM
    log.info("phase 3: repacking into %s (main=%s)", output_name, main_page)
    _docker_run(
        [
            "zimwriterfs",
            f"--welcome={main_page}",
            *illustration_flag,
            "--language=eng",
            "--title=Wikipedia (compact)",
            "--description=Wikipedia with resized images",
            "--creator=Wikipedia contributors",
            "--publisher=Svalbard",
            "--name=wikipedia-compact",
            "--withoutFTIndex",
            "/work/extracted",
            f"/output/{output_name}",
        ],
        mounts=mounts,
    )

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
    parser.add_argument("--colors", type=int, default=DEFAULT_COLORS)
    parser.add_argument("--quality", type=int, default=DEFAULT_QUALITY)
    parser.add_argument("--no-dither", action="store_true", help="Resize only, no dithering")
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

    rewrite_zim(
        source_zim, args.output,
        max_width=args.width, num_colors=args.colors,
        quality=args.quality, no_dither=args.no_dither,
    )

    in_mb = source_zim.stat().st_size / (1024 * 1024)
    out_mb = args.output.stat().st_size / (1024 * 1024)
    log.info("result: %.0f MB → %.0f MB (%.0f%%)", in_mb, out_mb, out_mb / in_mb * 100)


if __name__ == "__main__":
    main()
