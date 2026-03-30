"""ZIM text extraction utilities for cross-ZIM search indexing."""

from __future__ import annotations

import html
import re
from collections.abc import Iterator
from pathlib import Path

_TAG_RE = re.compile(r"<[^>]+>")
_WHITESPACE_RE = re.compile(r"\s+")


def strip_html(raw: str) -> str:
    """Remove HTML tags, decode entities, and collapse whitespace."""
    text = _TAG_RE.sub("", raw)
    text = html.unescape(text)
    text = _WHITESPACE_RE.sub(" ", text).strip()
    return text


def truncate_text(text: str, max_chars: int) -> str:
    """Truncate text at a sentence or word boundary within *max_chars*.

    Prefers cutting after a sentence-ending period.  Falls back to the last
    word boundary that fits.
    """
    if len(text) <= max_chars:
        return text

    # Try to find the last sentence boundary (". ") within the limit.
    window = text[:max_chars]
    # Look for the last ". " or a "." right at the boundary.
    last_period = window.rfind(". ")
    if last_period == -1 and window.endswith("."):
        last_period = len(window) - 1
    if last_period != -1:
        return text[: last_period + 1]

    # Fall back to last word boundary.
    truncated = window.rsplit(" ", 1)[0]
    return truncated


def extract_articles(
    zim_path: str | Path,
    max_body_chars: int = 500,
) -> Iterator[tuple[str, str, str]]:
    """Yield ``(path, title, body)`` for every HTML article in *zim_path*.

    Requires the ``libzim`` package (imported lazily so the rest of the
    module stays usable without it).  Redirect entries and non-HTML items
    are skipped.
    """
    from libzim.reader import Archive  # type: ignore[import-untyped]

    archive = Archive(str(zim_path))

    for entry_idx in range(archive.entry_count):
        entry = archive._get_entry_by_id(entry_idx)

        # Skip redirects.
        if entry.is_redirect:
            continue

        item = entry.get_item()
        mimetype = item.mimetype

        if mimetype not in ("text/html", "text/html; charset=utf-8"):
            continue

        path = entry.path
        title = entry.title or path

        raw_body = bytes(item.content).decode("utf-8", errors="replace")
        body = truncate_text(strip_html(raw_body), max_body_chars)

        yield path, title, body


def article_count(zim_path: str | Path) -> int:
    """Return the article count stored in the ZIM metadata (no iteration)."""
    from libzim.reader import Archive  # type: ignore[import-untyped]

    archive = Archive(str(zim_path))
    return archive.entry_count
