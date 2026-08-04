#!/usr/bin/env python3
"""Mirror a website, pull its embedded YouTube videos local, pack it into one ZIM.

Plain website ZIMs leave YouTube embeds as remote iframes, so a tutorial site
loses the videos that carry most of its instructional value. This mirrors the
site with wget, downloads each embedded video with yt-dlp, rewrites the embeds
to point at the local files, and packs the result.

Usage:
    web-video-zim.py --source-url https://example.org/ --output /vault/zim/example.zim
"""

from __future__ import annotations

import argparse
import mimetypes
import re
import subprocess
import sys
from datetime import datetime, timezone
from html import escape
from pathlib import Path
from urllib.parse import urlparse

from libzim.writer import Creator, FileProvider, Hint, Item, StringProvider

MEDIA_DIR = "_svalbard/media"

# ponytail: regex, not an HTML parser — the tools image has no bs4, and these
# two embed shapes cover every real YouTube embed. Swap in a parser if a site
# shows up whose markup this mangles.
IFRAME_RE = re.compile(
    r"""<iframe[^>]*?\ssrc=["'][^"']*?(?:youtube\.com|youtube-nocookie\.com)/embed/"""
    r"""(?P<id>[A-Za-z0-9_-]{11})[^"']*["'][^>]*>(?:\s*</iframe>)?""",
    re.IGNORECASE,
)
WATCH_RE = re.compile(
    r"""(?P<attr>\shref=["'])(?P<url>[^"']*?(?:youtube\.com/watch\?v=|youtu\.be/)"""
    r"""(?P<id>[A-Za-z0-9_-]{11})[^"']*)["']""",
    re.IGNORECASE,
)


class HtmlItem(Item):
    def __init__(self, path: str, title: str, content: str, *, is_front: bool = False):
        super().__init__()
        self._path, self._title, self._content, self._is_front = path, title, content, is_front

    def get_path(self) -> str:
        return self._path

    def get_title(self) -> str:
        return self._title

    def get_mimetype(self) -> str:
        return "text/html"

    def get_contentprovider(self):
        return StringProvider(self._content)

    def get_hints(self):
        return {Hint.FRONT_ARTICLE: self._is_front}


class StaticFileItem(Item):
    def __init__(self, path: str, filepath: Path):
        super().__init__()
        self._path, self._filepath = path, filepath
        self._mimetype = mimetypes.guess_type(str(filepath))[0] or "application/octet-stream"

    def get_path(self) -> str:
        return self._path

    def get_title(self) -> str:
        return self._filepath.name

    def get_mimetype(self) -> str:
        return self._mimetype

    def get_contentprovider(self):
        return FileProvider(str(self._filepath))

    def get_hints(self):
        return {Hint.FRONT_ARTICLE: False}


def mirror_site(url: str, mirror: Path) -> None:
    """Mirror the site into mirror/ with links rewritten for offline use."""
    mirror.mkdir(parents=True, exist_ok=True)
    # ponytail: same-host only. --span-hosts would pull CDN assets too, but
    # --mirror recurses infinitely, so it would also crawl every outbound link
    # on the page (news sites, facebook, ...). CDN-hosted CSS is the price.
    cmd = [
        "wget", "--mirror", "--page-requisites", "--convert-links",
        "--adjust-extension", "--no-host-directories",
        "--timeout=30", "--tries=3", "-e", "robots=off",
        "--directory-prefix", str(mirror), url,
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True)
    # wget exits 4-8 for individual fetch failures (dead links, 404 favicons);
    # the mirror is still usable, so only a missing mirror is fatal.
    if proc.returncode not in (0, 4, 5, 6, 7, 8):
        sys.exit(f"wget failed ({proc.returncode}): {proc.stderr[-500:]}")
    if not any(mirror.rglob("*.html")):
        sys.exit("wget produced no HTML pages")


def discover(mirror: Path) -> dict[str, list[Path]]:
    """Map each YouTube video id to the pages embedding or linking it."""
    found: dict[str, list[Path]] = {}
    for page in sorted(mirror.rglob("*.html")):
        text = page.read_text(encoding="utf-8", errors="replace")
        ids = {m.group("id") for m in IFRAME_RE.finditer(text)}
        ids |= {m.group("id") for m in WATCH_RE.finditer(text)}
        for vid in ids:
            found.setdefault(vid, []).append(page)
    return found


def download(video_id: str, media: Path, quality: str) -> Path | None:
    """Download one video as mp4 plus its thumbnail. Returns the mp4, or None."""
    videos = media / "videos"
    videos.mkdir(parents=True, exist_ok=True)
    existing = list(videos.glob(f"{video_id}.*"))
    if existing:
        return existing[0]

    height = quality.rstrip("p")
    cmd = [
        "yt-dlp",
        "-f", f"bestvideo[height<={height}][ext=mp4]+bestaudio[ext=m4a]/best[height<={height}]",
        "--merge-output-format", "mp4",
        "--write-thumbnail", "--convert-thumbnails", "jpg",
        "--no-playlist", "--retries", "3",
        "-o", str(videos / f"{video_id}.%(ext)s"),
        f"https://www.youtube.com/watch?v={video_id}",
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True)
    if proc.returncode != 0:
        print(f"  ! {video_id}: download failed, leaving embed remote", flush=True)
        return None
    mp4 = videos / f"{video_id}.mp4"
    return mp4 if mp4.exists() else None


def rewrite(page: Path, mirror: Path, local: dict[str, Path]) -> None:
    """Point embeds and links at local media; leave failed downloads remote."""
    text = page.read_text(encoding="utf-8", errors="replace")
    # Relative prefix so pages in subdirectories still resolve the media dir.
    up = "../" * len(page.relative_to(mirror).parent.parts)

    def player(match: re.Match) -> str:
        vid = match.group("id")
        if vid not in local:
            return match.group(0)
        src = f"{up}{MEDIA_DIR}/videos/{vid}.mp4"
        poster = mirror / MEDIA_DIR / "videos" / f"{vid}.jpg"
        poster_attr = f' poster="{up}{MEDIA_DIR}/videos/{vid}.jpg"' if poster.exists() else ""
        watch = f"https://www.youtube.com/watch?v={vid}"
        return (
            f'<video controls preload="metadata" style="max-width:100%"{poster_attr}>'
            f'<source src="{escape(src)}" type="video/mp4"></video>'
            f'<p><small>Source: <a href="{escape(watch)}">{escape(watch)}</a></small></p>'
        )

    def link(match: re.Match) -> str:
        vid = match.group("id")
        if vid not in local:
            return match.group(0)
        return f'{match.group("attr")}{up}{MEDIA_DIR}/videos/{vid}.mp4"'

    # Watch links first: the iframe replacement inserts a provenance link back
    # to YouTube, and running WATCH_RE afterwards would rewrite it to the local
    # file, destroying the "where did this come from" trail.
    page.write_text(IFRAME_RE.sub(player, WATCH_RE.sub(link, text)), encoding="utf-8")


def pack(mirror: Path, output: Path, title: str, source_url: str) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    main = "index.html" if (mirror / "index.html").exists() else next(
        p.relative_to(mirror).as_posix() for p in sorted(mirror.rglob("*.html"))
    )
    zim = Creator(str(output))
    zim.config_indexing(True, "eng")
    zim.config_clustersize(2048)
    zim.set_mainpath(main)
    with zim:
        zim.add_metadata("Title", title)
        zim.add_metadata("Description", f"Offline archive of {source_url} with local video")
        zim.add_metadata("Language", "eng")
        zim.add_metadata("Creator", "Svalbard")
        zim.add_metadata("Publisher", "Svalbard")
        zim.add_metadata("Date", datetime.now(timezone.utc).strftime("%Y-%m-%d"))
        zim.add_metadata("Tags", "website;offline;video")
        for path in sorted(mirror.rglob("*")):
            if not path.is_file():
                continue
            rel = path.relative_to(mirror).as_posix()
            if path.suffix.lower() in (".html", ".htm"):
                zim.add_item(HtmlItem(
                    rel,
                    title if rel == main else path.stem,
                    path.read_text(encoding="utf-8", errors="replace"),
                    is_front=rel == main,
                ))
            else:
                zim.add_item(StaticFileItem(rel, path))


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--source-url", required=True)
    ap.add_argument("--output", required=True, help="path of the ZIM to write")
    ap.add_argument("--workdir", default="/work")
    ap.add_argument("--title", default="")
    ap.add_argument("--quality", default="480p", help="max video height, e.g. 480p")
    ap.add_argument("--max-videos", type=int, default=0, help="0 = no limit")
    args = ap.parse_args()

    workdir = Path(args.workdir)
    mirror = workdir / "mirror"
    title = args.title or urlparse(args.source_url).netloc

    print(f"Mirroring {args.source_url}", flush=True)
    mirror_site(args.source_url, mirror)
    print(f"  {sum(1 for _ in mirror.rglob('*.html'))} pages", flush=True)

    found = discover(mirror)
    ids = sorted(found)
    if args.max_videos:
        ids = ids[: args.max_videos]
    print(f"Found {len(found)} video(s); downloading {len(ids)} at {args.quality}", flush=True)

    local: dict[str, Path] = {}
    for vid in ids:
        path = download(vid, mirror / MEDIA_DIR, args.quality)
        if path:
            local[vid] = path
            print(f"  + {vid}", flush=True)

    for page in sorted({p for pages in found.values() for p in pages}):
        rewrite(page, mirror, local)

    print(f"Packing {args.output}", flush=True)
    pack(mirror, Path(args.output), title, args.source_url)
    print(f"Done: {len(local)}/{len(found)} videos local", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
