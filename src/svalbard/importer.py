"""Unified import command orchestration."""

from __future__ import annotations

import re
from pathlib import Path
from urllib.parse import parse_qs, urlparse

from svalbard.commands import add_local_source
from svalbard.crawler import (
    ensure_zimit_image,
    register_generated_zim,
    run_url_crawl,
)
from svalbard.docker import has_docker
from svalbard.media import probe_media_url, run_media_ingest
from svalbard.paths import workspace_root as resolve_workspace_root


def _slugify(value: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return slug or "source"


def _is_existing_path(value: str) -> bool:
    return Path(value).expanduser().exists()


def _is_http_url(value: str) -> bool:
    parsed = urlparse(value)
    return parsed.scheme in {"http", "https"} and bool(parsed.netloc)


def _looks_like_media_url(value: str) -> bool:
    host = urlparse(value).netloc.lower()
    return any(
        marker in host
        for marker in ("youtube.com", "youtu.be", "areena.yle.fi")
    )


def detect_import_kind(value: str, *, kind: str = "auto", runner: str = "auto") -> str:
    """Resolve import input kind."""
    if kind != "auto":
        return kind
    if _is_existing_path(value):
        return "local"
    if not _is_http_url(value):
        raise ValueError(f"Input is neither an existing path nor an http(s) URL: {value}")
    if _looks_like_media_url(value):
        return "media"
    if probe_media_url(value, runner=runner):
        return "media"
    return "web"


def _normalize_output_name(value: str | None, fallback_slug: str) -> str:
    if value:
        return value if value.endswith(".zim") else f"{value}.zim"
    return f"{fallback_slug}.zim"


def _youtube_slug(parsed) -> str | None:
    host = parsed.netloc.lower().replace("www.", "")
    query = parse_qs(parsed.query)
    path = parsed.path.strip("/")

    if host == "youtu.be" and path:
        return _slugify(f"youtube-video-{path.split('/')[-1]}")

    if "youtube.com" not in host:
        return None

    video_id = query.get("v", [None])[0]
    playlist_id = query.get("list", [None])[0]

    if video_id:
        return _slugify(f"youtube-video-{video_id}")
    if playlist_id:
        return _slugify(f"youtube-playlist-{playlist_id}")
    return None


def _default_output_slug(value: str, kind: str) -> str:
    if kind == "local":
        return _slugify(Path(value).stem)
    parsed = urlparse(value)
    host = parsed.netloc.lower().replace("www.", "")
    path = parsed.path.strip("/")
    if kind == "media":
        youtube_slug = _youtube_slug(parsed)
        if youtube_slug:
            return youtube_slug
        query = parse_qs(parsed.query)
        if query.get("list"):
            return _slugify(f"{host}-playlist")
        if path:
            return _slugify(path.split("/")[-1])
    return _slugify(f"{host}-{path or kind}")


def run_import(
    value: str,
    *,
    workspace_root: Path | str | None = None,
    kind: str = "auto",
    runner: str = "auto",
    quality: str = "720p",
    audio_only: bool = False,
    output_name: str | None = None,
    source_type: str | None = None,
) -> str:
    """Import a local path or remote URL as a workspace-local source."""
    root = resolve_workspace_root(workspace_root)
    resolved_kind = detect_import_kind(value, kind=kind, runner=runner)
    resolved_runner = runner if runner != "auto" else ("host" if resolved_kind == "local" else "docker")

    if resolved_kind == "local":
        return add_local_source(Path(value), workspace_root=root, source_type=source_type)

    if resolved_runner != "docker":
        raise ValueError("Remote import currently supports only the docker runner")
    if not has_docker():
        raise RuntimeError("Docker is not available. Install Docker to use remote import.")

    slug = _default_output_slug(value, resolved_kind)
    normalized_output = _normalize_output_name(output_name, slug)

    if resolved_kind == "web":
        if not ensure_zimit_image():
            raise RuntimeError("Failed to pull Zimit image.")
        artifact = run_url_crawl(value, normalized_output, root)
        return register_generated_zim(
            workspace_root=root,
            artifact_path=artifact,
            origin_url=value,
            kind="web",
            runner=resolved_runner,
            tool="zimit",
        )

    artifact = run_media_ingest(
        value,
        normalized_output,
        root,
        quality=quality,
        audio_only=audio_only,
        runner=resolved_runner,
    )
    tool = "yle-dl + ffmpeg + libzim" if "areena.yle.fi" in urlparse(value).netloc.lower() else "yt-dlp + ffmpeg + libzim"
    return register_generated_zim(
        workspace_root=root,
        artifact_path=artifact,
        origin_url=value,
        kind="media",
        runner=resolved_runner,
        tool=tool,
        quality=quality,
        audio_only=audio_only,
    )
