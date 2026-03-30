"""Cross-ZIM indexing engine.

Scans a drive for ZIM files, compares against the search database to
find new or changed files, and incrementally indexes them using FTS5.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Callable

from svalbard.search_db import SearchDB
from svalbard.zim_extract import extract_articles, article_count as zim_article_count

# ── Scanning ─────────────────────────────────────────────────────────


def scan_zim_files(drive_path: str | Path) -> list[Path]:
    """Return all .zim files inside ``drive_path/zim/``, sorted by name."""
    zim_dir = Path(drive_path) / "zim"
    if not zim_dir.is_dir():
        return []
    return sorted(p for p in zim_dir.iterdir() if p.suffix == ".zim" and p.is_file())


# ── Checksum helpers ─────────────────────────────────────────────────


def _file_checksum(path: Path) -> str:
    """Quick checksum based on file size and modification time."""
    stat = path.stat()
    return f"{stat.st_size}:{stat.st_mtime}"


def _meta_key(filename: str) -> str:
    return f"checksum:{filename}"


# ── IndexPlan ────────────────────────────────────────────────────────

BYTES_PER_ARTICLE_ESTIMATE = 600  # rough average for fast strategy

# Tier ordering for upgrade detection
_TIER_RANK = {"fast": 0, "standard": 1, "semantic": 2}


@dataclass
class IndexPlan:
    """Description of work produced by :func:`estimate_index`."""

    total_zims: int = 0
    new_zims: int = 0
    already_indexed: int = 0
    changed_zims: int = 0
    missing_zims: int = 0
    strategy: str = "fast"
    estimated_articles: int = 0
    estimated_db_bytes: int = 0
    files_to_index: list[Path] = field(default_factory=list)


# ── Estimation ───────────────────────────────────────────────────────


def estimate_index(
    drive_path: str | Path,
    db: SearchDB,
    strategy: str = "fast",
) -> IndexPlan:
    """Scan ZIMs and compare against the DB to build an :class:`IndexPlan`."""
    zim_files = scan_zim_files(drive_path)
    indexed = db.indexed_filenames()

    plan = IndexPlan(total_zims=len(zim_files), strategy=strategy)
    on_disk_names: set[str] = set()

    # Detect tier upgrade: if requested strategy is higher than current,
    # re-index everything to get fuller content
    current_tier = db.get_meta("tier") or "none"
    upgrading = _TIER_RANK.get(strategy, 0) > _TIER_RANK.get(current_tier, -1)

    for zf in zim_files:
        name = zf.name
        on_disk_names.add(name)
        current_checksum = _file_checksum(zf)
        stored_checksum = db.get_meta(_meta_key(name))

        if name not in indexed:
            # Completely new file
            plan.new_zims += 1
            plan.files_to_index.append(zf)
        elif upgrading:
            # Tier upgrade — re-index with fuller content
            plan.changed_zims += 1
            plan.files_to_index.append(zf)
        elif stored_checksum != current_checksum:
            # File changed since last index
            plan.changed_zims += 1
            plan.files_to_index.append(zf)
        else:
            plan.already_indexed += 1

    # Files in DB but no longer on disk
    plan.missing_zims = len(indexed - on_disk_names)

    # Rough estimate: use ZIM entry count for files to index
    for zf in plan.files_to_index:
        try:
            plan.estimated_articles += zim_article_count(zf)
        except Exception:
            # If we can't read the ZIM for estimation, guess
            plan.estimated_articles += 1000

    plan.estimated_db_bytes = plan.estimated_articles * BYTES_PER_ARTICLE_ESTIMATE
    return plan


# ── Indexing ─────────────────────────────────────────────────────────

_BATCH_SIZE = 10_000


def run_index(
    drive_path: str | Path,
    db: SearchDB,
    strategy: str = "fast",
    on_progress: Callable[[str, int, int], None] | None = None,
) -> IndexPlan:
    """Index new and changed ZIM files into *db*.

    Parameters
    ----------
    drive_path:
        Root of the drive containing a ``zim/`` subdirectory.
    db:
        An open :class:`SearchDB` instance.
    strategy:
        ``"fast"`` truncates article bodies to 500 chars;
        ``"standard"`` stores the full body text.
    on_progress:
        Optional callback ``(filename, articles_done, total_files)``.

    Returns
    -------
    IndexPlan
        The plan that was executed (useful for reporting).
    """
    plan = estimate_index(drive_path, db, strategy=strategy)

    if not plan.files_to_index:
        return plan

    max_body = 500 if strategy == "fast" else 0

    # Speed up bulk insert
    db.conn.execute("PRAGMA synchronous=OFF")

    try:
        total_files = len(plan.files_to_index)
        for file_idx, zf in enumerate(plan.files_to_index):
            filename = zf.name
            source_id = db.upsert_source(filename, title=filename)

            # If re-indexing a changed file, remove stale articles first
            if db.get_meta(_meta_key(filename)) is not None:
                db.delete_source_articles(source_id)

            # Extract and insert in batches
            batch: list[dict] = []
            articles_done = 0

            for path, title, body in extract_articles(zf, max_body_chars=max_body):
                batch.append(
                    {
                        "source_id": source_id,
                        "path": path,
                        "title": title,
                        "body": body,
                    }
                )
                if len(batch) >= _BATCH_SIZE:
                    db.insert_articles_batch(batch)
                    articles_done += len(batch)
                    batch = []
                    if on_progress:
                        on_progress(filename, articles_done, total_files)

            # Flush remaining
            if batch:
                db.insert_articles_batch(batch)
                articles_done += len(batch)
                if on_progress:
                    on_progress(filename, articles_done, total_files)

            # Store checksum so future runs skip this file
            db.set_meta(_meta_key(filename), _file_checksum(zf))

    finally:
        # Restore safe sync
        db.conn.execute("PRAGMA synchronous=FULL")

    # Record build metadata
    db.set_meta("tier", strategy)
    db.set_meta("indexed_at", datetime.now(timezone.utc).isoformat())

    return plan
