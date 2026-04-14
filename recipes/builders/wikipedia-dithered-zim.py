#!/usr/bin/env python3
"""Download a Wikipedia ZIM and reprocess it with dithered images.

Reads the source ZIM via libzim, writes images to a temp dir, calls
the Go zim-dither tool to batch-process them, then writes a new ZIM
with dithered images via libzim.

Requirements: pip install libzim httpx
External:     build-tools/zim-dither (Go binary)

Usage:
    python3 wikipedia-dithered-zim.py --source-url URL --output library/out.zim
    python3 wikipedia-dithered-zim.py --source-zim input.zim --output library/out.zim
"""

from __future__ import annotations

import argparse
import logging
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from urllib.parse import urlparse

import httpx
from libzim.reader import Archive
from libzim.writer import Creator, Hint, Item, StringProvider

log = logging.getLogger("wikipedia-dithered")

DEFAULT_WIDTH = 400
DEFAULT_COLORS = 8

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent
DEFAULT_OUTPUT_DIR = PROJECT_ROOT / "library"

IMAGE_MIMES = {
    "image/jpeg": ".jpg",
    "image/png": ".png",
    "image/gif": ".gif",
    "image/webp": ".webp",
}


def find_zim_dither() -> str:
    """Find the zim-dither binary."""
    candidate = PROJECT_ROOT / "build-tools" / "zim-dither"
    if candidate.exists():
        return str(candidate)
    found = shutil.which("zim-dither")
    if found:
        return found
    raise FileNotFoundError(
        "zim-dither not found. Build it:\n"
        "  cd build-tools && go build -o zim-dither ./cmd/zim-dither/"
    )


# ── ZIM item wrappers ────────────────────────────────────────────────────


class PassthroughItem(Item):
    def __init__(self, path: str, title: str, mimetype: str, content: bytes, is_front: bool = False):
        super().__init__()
        self._path = path
        self._title = title
        self._mimetype = mimetype
        self._content = content
        self._is_front = is_front

    def get_path(self) -> str:
        return self._path

    def get_title(self) -> str:
        return self._title

    def get_mimetype(self) -> str:
        return self._mimetype

    def get_contentprovider(self):
        return StringProvider(self._content)

    def get_hints(self) -> dict:
        return {Hint.FRONT_ARTICLE: self._is_front}


# ── Core pipeline ────────────────────────────────────────────────────────


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
) -> None:
    """Read a ZIM, dither images via Go tool, write new ZIM."""
    binary = find_zim_dither()
    archive = Archive(str(source_path))
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(prefix="zim-dither-imgs-") as img_dir:
        img_dir = Path(img_dir)

        # Phase 1: extract images from ZIM to temp dir
        log.info("phase 1: extracting images from %s", source_path.name)
        image_entries: dict[str, str] = {}  # zim_path -> temp_filename
        total = archive.entry_count
        img_count = 0

        for i in range(total):
            try:
                entry = archive._get_entry_by_id(i)
                item = entry.get_item()
            except Exception:
                continue

            if item.mimetype not in IMAGE_MIMES:
                continue

            ext = IMAGE_MIMES[item.mimetype]
            safe_name = f"{img_count:06d}{ext}"
            content = item.content.tobytes()

            # Skip tiny images (icons, spacers)
            if len(content) < 512:
                continue

            (img_dir / safe_name).write_bytes(content)
            image_entries[entry.path] = safe_name
            img_count += 1

        log.info("extracted %d images", img_count)

        # Phase 2: batch dither via Go tool
        log.info("phase 2: dithering (width=%d, colors=%d)", max_width, num_colors)
        subprocess.run(
            [
                binary, "batch",
                "--width", str(max_width),
                "--colors", str(num_colors),
                "--dither", "bayer",
                str(img_dir),
            ],
            check=True,
        )

        # Build lookup: original name -> dithered PNG path
        dithered: dict[str, Path] = {}
        for zim_path, temp_name in image_entries.items():
            stem = Path(temp_name).stem
            # Go tool outputs .jpg by default
            for ext in (".jpg", ".png"):
                candidate = img_dir / (stem + ext)
                if candidate.exists():
                    dithered[zim_path] = candidate
                    break
            else:
                orig = img_dir / temp_name
                if orig.exists():
                    dithered[zim_path] = orig

        log.info("dithered %d images", len(dithered))

        # Phase 3: write new ZIM
        log.info("phase 3: writing %s", output_path.name)

        main_path = ""
        try:
            main_path = archive.main_entry.path
        except Exception:
            pass

        illustration_data = None
        try:
            illustration_data = archive.get_illustration_item(48).content.tobytes()
        except Exception:
            pass

        with Creator(str(output_path)).config_indexing(True, "eng") as creator:
            if main_path:
                creator.set_mainpath(main_path)
            if illustration_data:
                creator.add_illustration(48, illustration_data)

            # Copy metadata
            for key in ["Title", "Description", "Language", "Creator", "Publisher", "Date", "Name"]:
                try:
                    value = archive.get_metadata(key).tobytes().decode("utf-8", errors="replace")
                    if key == "Description":
                        value += " [dithered 8-color images]"
                    creator.add_metadata(key, value)
                except Exception:
                    pass

            entries_written = 0
            images_replaced = 0

            for i in range(total):
                try:
                    entry = archive._get_entry_by_id(i)
                except Exception:
                    continue

                path = entry.path
                title = entry.title

                if path.startswith("M/"):
                    continue

                try:
                    item = entry.get_item()
                    mimetype = item.mimetype
                    content = item.content.tobytes()
                except Exception:
                    continue

                # Replace image with dithered version
                if path in dithered:
                    dithered_path = dithered[path]
                    dithered_content = dithered_path.read_bytes()
                    dithered_mime = "image/jpeg" if dithered_path.suffix == ".jpg" else "image/png"
                    creator.add_item(PassthroughItem(path, title, dithered_mime, dithered_content))
                    images_replaced += 1
                else:
                    is_front = mimetype.startswith("text/html")
                    creator.add_item(PassthroughItem(path, title, mimetype, content, is_front))

                entries_written += 1
                if entries_written % 5000 == 0:
                    log.info("  %d/%d entries written", entries_written, total)

    log.info("done: %d entries, %d images dithered", entries_written, images_replaced)


# ── CLI ──────────────────────────────────────────────────────────────────


def main():
    parser = argparse.ArgumentParser(description="Download and dither a Wikipedia ZIM")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--source-url", help="URL to download source ZIM")
    source.add_argument("--source-zim", type=Path, help="Path to existing source ZIM")
    parser.add_argument(
        "--output", type=Path,
        default=DEFAULT_OUTPUT_DIR / "wikipedia-en-medicine-dithered.zim",
    )
    parser.add_argument("--width", type=int, default=DEFAULT_WIDTH)
    parser.add_argument("--colors", type=int, default=DEFAULT_COLORS)
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

    rewrite_zim(source_zim, args.output, max_width=args.width, num_colors=args.colors)

    in_mb = source_zim.stat().st_size / (1024 * 1024)
    out_mb = args.output.stat().st_size / (1024 * 1024)
    log.info("result: %.0f MB → %.0f MB (%.0f%%)", in_mb, out_mb, out_mb / in_mb * 100)


if __name__ == "__main__":
    main()
