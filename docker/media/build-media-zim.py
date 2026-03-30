#!/usr/bin/env python3
"""Download media, generate a simple offline site, and package it as a ZIM."""

from __future__ import annotations

import argparse
import json
import mimetypes
import shutil
import subprocess
import sys
from dataclasses import dataclass, replace
from datetime import datetime
from html import escape
from pathlib import Path
from urllib.parse import urlparse
import re

from libzim.writer import Creator, FileProvider, Hint, Item, StringProvider


MEDIA_EXTS = {".mp4", ".m4v", ".mov", ".mkv", ".webm", ".mp3", ".m4a", ".aac", ".ogg", ".opus", ".wav"}
SUBTITLE_EXTS = {".vtt", ".srt"}
THUMB_EXTS = {".jpg", ".jpeg", ".png", ".webp"}


def _slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "item"


def _run(cmd: list[str], *, cwd: Path | None = None, capture_output: bool = False) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=str(cwd) if cwd else None,
        check=True,
        capture_output=capture_output,
        text=True,
    )


def _format_duration(seconds: int | float | None) -> str:
    if not seconds:
        return ""
    total = int(seconds)
    hours, rem = divmod(total, 3600)
    minutes, secs = divmod(rem, 60)
    if hours:
        return f"{hours}:{minutes:02d}:{secs:02d}"
    return f"{minutes}:{secs:02d}"


@dataclass
class MediaItem:
    slug: str
    title: str
    source_url: str
    media_path: Path
    description: str = ""
    duration: str = ""
    uploader: str = ""
    thumbnail_path: Path | None = None
    subtitle_paths: list[Path] | None = None
    audio_only: bool = False


class HtmlItem(Item):
    def __init__(self, path: str, title: str, content: str, *, is_front: bool = False):
        super().__init__()
        self._path = path
        self._title = title
        self._content = content
        self._is_front = is_front

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
        self._path = path
        self._filepath = filepath
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


def _collect_yt_dlp_items(download_dir: Path, source_url: str, *, audio_only: bool) -> list[MediaItem]:
    items: list[MediaItem] = []
    info_files = sorted(download_dir.glob("*.info.json"))
    for info_file in info_files:
        info = json.loads(info_file.read_text() or "{}")
        base_name = info_file.name.removesuffix(".info.json")
        media_path = next(
            (
                path
                for path in sorted(download_dir.glob(f"{base_name}.*"))
                if path.suffix.lower() in MEDIA_EXTS
            ),
            None,
        )
        if media_path is None:
            continue
        thumbnail_path = next(
            (
                path
                for path in sorted(download_dir.glob(f"{base_name}.*"))
                if path.suffix.lower() in THUMB_EXTS
            ),
            None,
        )
        subtitles = [
            path for path in sorted(download_dir.glob(f"{base_name}*"))
            if path.suffix.lower() in SUBTITLE_EXTS
        ]
        title = info.get("title") or media_path.stem
        items.append(
            MediaItem(
                slug=_slugify(f"{title}-{info.get('id', media_path.stem)}"),
                title=title,
                source_url=info.get("webpage_url") or info.get("original_url") or source_url,
                media_path=media_path,
                description=info.get("description", ""),
                duration=_format_duration(info.get("duration")),
                uploader=info.get("uploader") or info.get("channel", ""),
                thumbnail_path=thumbnail_path,
                subtitle_paths=subtitles,
                audio_only=audio_only,
            )
        )
    if items:
        return items

    for media_path in sorted(download_dir.iterdir()):
        if media_path.suffix.lower() not in MEDIA_EXTS:
            continue
        items.append(
            MediaItem(
                slug=_slugify(media_path.stem),
                title=media_path.stem,
                source_url=source_url,
                media_path=media_path,
                audio_only=audio_only,
            )
        )
    return items


def _read_yle_metadata(url: str) -> dict:
    result = _run(["yle-dl", "--showmetadata", url], capture_output=True)
    raw = result.stdout.strip()
    if not raw:
        return {}
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        first_line = raw.splitlines()[0]
        return json.loads(first_line)


def _download_with_yle_dl(url: str, download_dir: Path, quality: str, audio_only: bool) -> list[MediaItem]:
    metadata = _read_yle_metadata(url)
    cmd = ["yle-dl", "--destdir", str(download_dir)]
    if quality != "source" and not audio_only:
        cmd.extend(["--resolution", quality.removesuffix("p")])
    cmd.append(url)
    _run(cmd)

    media_paths = [path for path in sorted(download_dir.iterdir()) if path.suffix.lower() in MEDIA_EXTS]
    items: list[MediaItem] = []
    if isinstance(metadata, list):
        metadata_list = metadata
    else:
        metadata_list = [metadata] if metadata else []
    for index, media_path in enumerate(media_paths):
        item_meta = metadata_list[index] if index < len(metadata_list) else {}
        items.append(
            MediaItem(
                slug=_slugify(f"{item_meta.get('title', media_path.stem)}-{index + 1}"),
                title=item_meta.get("title") or media_path.stem,
                source_url=item_meta.get("url") or url,
                media_path=media_path,
                description=item_meta.get("description", ""),
                duration=_format_duration(item_meta.get("duration")),
                uploader=item_meta.get("publisher", "Yle Areena"),
                audio_only=audio_only,
            )
        )
    return items


def _download_with_yt_dlp(url: str, download_dir: Path, quality: str, audio_only: bool) -> list[MediaItem]:
    cmd = [
        "yt-dlp",
        "--no-progress",
        "--restrict-filenames",
        "--write-info-json",
        "--write-thumbnail",
        "--convert-thumbnails",
        "jpg",
        "--write-subs",
        "--write-auto-subs",
        "--sub-format",
        "vtt",
        "--sub-langs",
        "all,-live_chat",
        "-o",
        "%(playlist_index&{}- |)s%(title).120B [%(id)s].%(ext)s",
    ]
    if audio_only:
        cmd.extend(["-x", "--audio-format", "m4a", "--audio-quality", "0"])
    elif quality != "source":
        cmd.extend(["-S", f"res:{quality.removesuffix('p')}"])
    cmd.append(url)
    _run(cmd, cwd=download_dir)
    return _collect_yt_dlp_items(download_dir, url, audio_only=audio_only)


def _normalize_video(input_path: Path, output_path: Path, quality: str) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    if quality == "source" and input_path.suffix.lower() == ".mp4":
        shutil.copy2(input_path, output_path)
        return

    height = {
        "1080p": "1080",
        "720p": "720",
        "480p": "480",
        "360p": "360",
        "source": "1080",
    }[quality]
    crf = {
        "1080p": "26",
        "720p": "27",
        "480p": "28",
        "360p": "29",
        "source": "26",
    }[quality]
    audio_bitrate = {
        "1080p": "128k",
        "720p": "96k",
        "480p": "96k",
        "360p": "64k",
        "source": "128k",
    }[quality]
    _run(
        [
            "ffmpeg",
            "-y",
            "-i",
            str(input_path),
            "-vf",
            f"scale=-2:{height}:force_original_aspect_ratio=decrease",
            "-c:v",
            "libx264",
            "-preset",
            "slow",
            "-crf",
            crf,
            "-c:a",
            "aac",
            "-b:a",
            audio_bitrate,
            "-movflags",
            "+faststart",
            str(output_path),
        ]
    )


def _normalize_audio(input_path: Path, output_path: Path) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    if input_path.suffix.lower() == ".m4a":
        shutil.copy2(input_path, output_path)
        return
    _run(
        [
            "ffmpeg",
            "-y",
            "-i",
            str(input_path),
            "-vn",
            "-c:a",
            "aac",
            "-b:a",
            "96k",
            str(output_path),
        ]
    )


def _copy_optional(path: Path | None, dest: Path | None) -> Path | None:
    if path is None or dest is None:
        return None
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(path, dest)
    return dest


def _normalize_items(items: list[MediaItem], site_dir: Path, quality: str, audio_only: bool) -> list[MediaItem]:
    normalized: list[MediaItem] = []
    for item in items:
        media_rel = Path("audio" if audio_only else "videos") / f"{item.slug}{'.m4a' if audio_only else '.mp4'}"
        media_dest = site_dir / media_rel
        if audio_only:
            _normalize_audio(item.media_path, media_dest)
        else:
            _normalize_video(item.media_path, media_dest, quality)

        thumb_dest = None
        if item.thumbnail_path:
            thumb_rel = Path("thumbs") / f"{item.slug}{item.thumbnail_path.suffix.lower() or '.jpg'}"
            thumb_dest = _copy_optional(item.thumbnail_path, site_dir / thumb_rel)

        subs_dest: list[Path] = []
        for index, subtitle in enumerate(item.subtitle_paths or []):
            sub_rel = Path("subs") / f"{item.slug}-{index + 1}{subtitle.suffix.lower()}"
            copied = _copy_optional(subtitle, site_dir / sub_rel)
            if copied is not None:
                subs_dest.append(copied)

        normalized.append(
            replace(
                item,
                media_path=media_dest,
                thumbnail_path=thumb_dest,
                subtitle_paths=subs_dest,
                audio_only=audio_only,
            )
        )
    return normalized


def _page_shell(title: str, body: str) -> str:
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{escape(title)}</title>
  <style>
    :root {{
      color-scheme: light;
      --bg: #f4f1e8;
      --card: #fffaf1;
      --ink: #1f1f1b;
      --muted: #635e55;
      --line: #d5cbb6;
      --accent: #1f5f8b;
    }}
    * {{ box-sizing: border-box; }}
    body {{
      margin: 0;
      font-family: Georgia, "Times New Roman", serif;
      background: radial-gradient(circle at top, #fbf7ee, var(--bg));
      color: var(--ink);
      line-height: 1.5;
    }}
    a {{ color: var(--accent); text-decoration: none; }}
    .wrap {{ max-width: 1040px; margin: 0 auto; padding: 28px 18px 56px; }}
    .hero {{ margin-bottom: 24px; }}
    .muted {{ color: var(--muted); }}
    .grid {{
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 18px;
    }}
    .card {{
      display: block;
      border: 1px solid var(--line);
      border-radius: 16px;
      overflow: hidden;
      background: var(--card);
      box-shadow: 0 10px 24px rgba(0,0,0,0.06);
    }}
    .thumb {{
      width: 100%;
      aspect-ratio: 16 / 9;
      background: linear-gradient(135deg, #cbc1ab, #ece2cd);
      object-fit: cover;
      display: block;
    }}
    .content {{ padding: 14px; }}
    .meta {{ font-size: 0.92rem; color: var(--muted); }}
    .player {{ width: 100%; max-width: 100%; border-radius: 14px; background: #000; }}
    pre {{
      white-space: pre-wrap;
      background: #f1e7d3;
      border-radius: 12px;
      padding: 14px;
      overflow-wrap: anywhere;
    }}
  </style>
</head>
<body>
  <div class="wrap">
    {body}
  </div>
</body>
</html>"""


def _make_index_page(title: str, source_url: str, items: list[MediaItem], *, audio_only: bool) -> str:
    cards = []
    for item in items:
        media_meta = "Audio" if audio_only else "Video"
        thumb = (
            f'<img class="thumb" src="{escape(item.thumbnail_path.relative_to(item.thumbnail_path.parents[1]).as_posix())}" alt="{escape(item.title)} thumbnail">'
            if item.thumbnail_path
            else '<div class="thumb"></div>'
        )
        cards.append(
            f"""
            <a class="card" href="items/{escape(item.slug)}.html">
              {thumb}
              <div class="content">
                <h3>{escape(item.title)}</h3>
                <div class="meta">{escape(media_meta)}{f" · {escape(item.duration)}" if item.duration else ""}</div>
                <div class="meta">{escape(item.uploader)}</div>
              </div>
            </a>
            """
        )
    body = f"""
    <section class="hero">
      <p class="muted">Imported by Svalbard</p>
      <h1>{escape(title)}</h1>
      <p><a href="{escape(source_url)}">{escape(source_url)}</a></p>
      <p class="muted">{len(items)} item(s)</p>
    </section>
    <section class="grid">
      {''.join(cards)}
    </section>
    """
    return _page_shell(title, body)


def _make_item_page(collection_title: str, item: MediaItem) -> str:
    media_rel = Path("..") / item.media_path.relative_to(item.media_path.parents[1])
    tracks = []
    for subtitle in item.subtitle_paths or []:
        sub_rel = Path("..") / subtitle.relative_to(subtitle.parents[1])
        tracks.append(f'<track kind="subtitles" src="{escape(sub_rel.as_posix())}">')
    if item.audio_only:
        player = f'<audio class="player" controls preload="metadata" src="{escape(media_rel.as_posix())}"></audio>'
    else:
        player = (
            f'<video class="player" controls preload="metadata" src="{escape(media_rel.as_posix())}">'
            f'{"".join(tracks)}</video>'
        )
    thumb = ""
    if item.thumbnail_path:
        thumb_rel = Path("..") / item.thumbnail_path.relative_to(item.thumbnail_path.parents[1])
        thumb = f'<p><img class="thumb" src="{escape(thumb_rel.as_posix())}" alt="{escape(item.title)} thumbnail"></p>'
    description = f"<pre>{escape(item.description)}</pre>" if item.description else ""
    body = f"""
    <p><a href="../index.html">Back to {escape(collection_title)}</a></p>
    <h1>{escape(item.title)}</h1>
    <p class="meta">{escape(item.uploader)}{f" · {escape(item.duration)}" if item.duration else ""}</p>
    <p><a href="{escape(item.source_url)}">{escape(item.source_url)}</a></p>
    {thumb}
    {player}
    {description}
    """
    return _page_shell(item.title, body)


def _write_site(site_dir: Path, title: str, source_url: str, items: list[MediaItem], *, audio_only: bool) -> None:
    items_dir = site_dir / "items"
    items_dir.mkdir(parents=True, exist_ok=True)
    (site_dir / "index.html").write_text(
        _make_index_page(title, source_url, items, audio_only=audio_only),
        encoding="utf-8",
    )
    for item in items:
        (items_dir / f"{item.slug}.html").write_text(
            _make_item_page(title, item),
            encoding="utf-8",
        )


def _build_zim(site_dir: Path, output_path: Path, title: str, source_url: str, *, audio_only: bool) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    zim = Creator(str(output_path))
    zim.config_indexing(True, "eng")
    zim.config_clustersize(2048)
    zim.set_mainpath("index.html")
    with zim:
        zim.add_metadata("Title", title)
        zim.add_metadata("Description", f"Offline media archive for {source_url}")
        zim.add_metadata("Language", "eng")
        zim.add_metadata("Creator", "Svalbard")
        zim.add_metadata("Publisher", "Svalbard")
        zim.add_metadata("Date", datetime.utcnow().strftime("%Y-%m-%d"))
        zim.add_metadata("Tags", "media;offline;audio" if audio_only else "media;offline;video")
        for path in sorted(site_dir.rglob("*")):
            if not path.is_file():
                continue
            relative_path = path.relative_to(site_dir).as_posix()
            if path.suffix.lower() == ".html":
                zim.add_item(
                    HtmlItem(
                        relative_path,
                        title if relative_path == "index.html" else path.stem,
                        path.read_text(encoding="utf-8"),
                        is_front=relative_path == "index.html",
                    )
                )
                continue
            zim.add_item(StaticFileItem(relative_path, path))


def probe(url: str) -> int:
    host = urlparse(url).netloc.lower()
    try:
        if "areena.yle.fi" in host:
            _read_yle_metadata(url)
            return 0
        _run(["yt-dlp", "--dump-single-json", "--skip-download", "--no-warnings", url], capture_output=True)
        return 0
    except subprocess.CalledProcessError:
        if "areena.yle.fi" in host:
            try:
                _run(["yt-dlp", "--dump-single-json", "--skip-download", "--no-warnings", url], capture_output=True)
                return 0
            except subprocess.CalledProcessError:
                return 1
        return 1


def build(url: str, output: Path, staging: Path, quality: str, audio_only: bool) -> int:
    downloads_dir = staging / "downloads"
    site_dir = staging / "site"
    shutil.rmtree(staging, ignore_errors=True)
    downloads_dir.mkdir(parents=True, exist_ok=True)
    site_dir.mkdir(parents=True, exist_ok=True)

    host = urlparse(url).netloc.lower()
    try:
        if "areena.yle.fi" in host:
            items = _download_with_yle_dl(url, downloads_dir, quality, audio_only)
            if not items:
                items = _download_with_yt_dlp(url, downloads_dir, quality, audio_only)
        else:
            items = _download_with_yt_dlp(url, downloads_dir, quality, audio_only)
    except subprocess.CalledProcessError as exc:
        print(exc.stderr or exc.stdout or str(exc), file=sys.stderr)
        return exc.returncode or 1

    if not items:
        print("No media items were downloaded", file=sys.stderr)
        return 1

    normalized = _normalize_items(items, site_dir, quality, audio_only)
    collection_title = normalized[0].uploader or normalized[0].title
    _write_site(site_dir, collection_title, url, normalized, audio_only=audio_only)
    _build_zim(site_dir, output, collection_title, url, audio_only=audio_only)
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Build a media ZIM archive")
    subparsers = parser.add_subparsers(dest="command", required=True)

    probe_parser = subparsers.add_parser("probe")
    probe_parser.add_argument("--url", required=True)

    build_parser = subparsers.add_parser("build")
    build_parser.add_argument("--url", required=True)
    build_parser.add_argument("--output", required=True)
    build_parser.add_argument("--staging", required=True)
    build_parser.add_argument("--quality", choices=["1080p", "720p", "480p", "360p", "source"], default="720p")
    build_parser.add_argument("--audio-only", action="store_true")

    args = parser.parse_args()
    if args.command == "probe":
        return probe(args.url)
    return build(args.url, Path(args.output), Path(args.staging), args.quality, args.audio_only)


if __name__ == "__main__":
    sys.exit(main())
