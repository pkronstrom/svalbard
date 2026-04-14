#!/usr/bin/env python3
"""Download a Wikipedia ZIM and reprocess it with dithered images.

Downloads the source ZIM from Kiwix, then runs zim-dither to resize
and Bayer-dither all images to 8-color indexed PNGs. Produces a much
smaller ZIM with visually useful images.

Requirements:
    - zim-dither binary (build-tools/cmd/zim-dither)
    - zimdump and zimwriterfs on PATH (svalbard-tools Docker)

Usage:
    python3 wikipedia-dithered-zim.py --source-url URL --output library/out.zim
    python3 wikipedia-dithered-zim.py --source-zim input.zim --output library/out.zim
"""

from __future__ import annotations

import argparse
import hashlib
import logging
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from urllib.parse import urlparse

import httpx

log = logging.getLogger("wikipedia-dithered")

DEFAULT_WIDTH = 400
DEFAULT_COLORS = 8
DEFAULT_DITHER = "bayer"

# Resolve paths relative to svalbard project root
SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
DEFAULT_OUTPUT_DIR = PROJECT_ROOT / "library"
ZIM_DITHER_BIN = PROJECT_ROOT / "build-tools" / "zim-dither"


def find_zim_dither() -> str:
    """Find the zim-dither binary."""
    # Check project build first
    if ZIM_DITHER_BIN.exists():
        return str(ZIM_DITHER_BIN)
    # Check PATH
    found = shutil.which("zim-dither")
    if found:
        return found
    raise FileNotFoundError(
        "zim-dither not found. Build it: cd build-tools && go build ./cmd/zim-dither/"
    )


def download_zim(url: str, dest: Path) -> Path:
    """Download a ZIM file with resume support."""
    dest.parent.mkdir(parents=True, exist_ok=True)

    # Check for partial download
    existing_size = dest.stat().st_size if dest.exists() else 0
    headers = {}
    if existing_size > 0:
        headers["Range"] = f"bytes={existing_size}-"
        log.info("resuming download from %d bytes", existing_size)

    with httpx.stream("GET", url, headers=headers, follow_redirects=True, timeout=60) as r:
        if r.status_code == 416:
            log.info("file already fully downloaded")
            return dest

        total = int(r.headers.get("content-length", 0))
        if r.status_code == 206:
            mode = "ab"
            log.info("downloading %s (resuming, %d bytes remaining)", url, total)
        else:
            mode = "wb"
            existing_size = 0
            log.info("downloading %s (%d MB)", url, total // (1024 * 1024))

        r.raise_for_status()
        downloaded = 0
        with open(dest, mode) as f:
            for chunk in r.iter_bytes(chunk_size=1024 * 1024):
                f.write(chunk)
                downloaded += len(chunk)
                if total > 0 and downloaded % (50 * 1024 * 1024) == 0:
                    pct = (existing_size + downloaded) / (existing_size + total) * 100
                    log.info("  %.0f%% downloaded", pct)

    log.info("download complete: %s", dest)
    return dest


def run_dither(
    input_zim: Path,
    output_zim: Path,
    *,
    width: int = DEFAULT_WIDTH,
    colors: int = DEFAULT_COLORS,
    dither: str = DEFAULT_DITHER,
) -> None:
    """Run zim-dither on a ZIM file."""
    binary = find_zim_dither()
    cmd = [
        binary,
        "--width", str(width),
        "--colors", str(colors),
        "--dither", dither,
        "--verbose",
        str(input_zim),
        str(output_zim),
    ]
    log.info("running: %s", " ".join(cmd))
    subprocess.run(cmd, check=True)


def main():
    parser = argparse.ArgumentParser(
        description="Download and dither a Wikipedia ZIM"
    )
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--source-url", help="URL to download the source ZIM from")
    source.add_argument("--source-zim", type=Path, help="Path to existing source ZIM")
    parser.add_argument(
        "--output", type=Path,
        default=DEFAULT_OUTPUT_DIR / "wikipedia-dithered.zim",
        help="Output ZIM path (default: library/wikipedia-dithered.zim)",
    )
    parser.add_argument("--width", type=int, default=DEFAULT_WIDTH)
    parser.add_argument("--colors", type=int, default=DEFAULT_COLORS)
    parser.add_argument("--dither", default=DEFAULT_DITHER)
    parser.add_argument("--workdir", type=Path, help="Working directory for downloads")
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(name)s %(message)s",
        datefmt="%H:%M:%S",
    )

    # Get or download source ZIM
    if args.source_zim:
        source_zim = args.source_zim
        if not source_zim.exists():
            log.error("source ZIM not found: %s", source_zim)
            sys.exit(1)
    else:
        work = args.workdir or Path(tempfile.mkdtemp(prefix="zim-dither-"))
        filename = Path(urlparse(args.source_url).path).name
        source_zim = download_zim(args.source_url, work / filename)

    # Dither
    run_dither(
        source_zim, args.output,
        width=args.width, colors=args.colors, dither=args.dither,
    )

    # Report
    in_mb = source_zim.stat().st_size / (1024 * 1024)
    out_mb = args.output.stat().st_size / (1024 * 1024)
    log.info("result: %.0f MB → %.0f MB (%.0f%%)", in_mb, out_mb, out_mb / in_mb * 100)


if __name__ == "__main__":
    main()
