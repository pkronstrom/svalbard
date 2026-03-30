"""Cross-ZIM indexing engine.

Scans a drive for ZIM files, compares against the search database to
find new or changed files, and incrementally indexes them using FTS5.
Optionally embeds articles for semantic search via llama-server.
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

# Text tiers that require re-extraction (semantic reuses standard text)
_TEXT_TIER = {"fast": "fast", "standard": "standard", "semantic": "standard"}


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
    needs_text_reindex: bool = False
    needs_embedding: bool = False
    articles_to_embed: int = 0


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

    # Detect tier upgrade: if requested text tier is higher than current,
    # re-index everything to get fuller content
    current_tier = db.get_meta("tier") or "none"
    current_text_tier = _TEXT_TIER.get(current_tier, "none")
    requested_text_tier = _TEXT_TIER.get(strategy, "fast")
    text_upgrading = _TIER_RANK.get(requested_text_tier, 0) > _TIER_RANK.get(current_text_tier, -1)

    for zf in zim_files:
        name = zf.name
        on_disk_names.add(name)
        current_checksum = _file_checksum(zf)
        stored_checksum = db.get_meta(_meta_key(name))

        if name not in indexed:
            # Completely new file
            plan.new_zims += 1
            plan.files_to_index.append(zf)
        elif text_upgrading:
            # Tier upgrade — re-index with fuller content
            plan.changed_zims += 1
            plan.files_to_index.append(zf)
            plan.needs_text_reindex = True
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

    # Semantic tier: check how many articles still need embedding
    if strategy == "semantic":
        plan.needs_embedding = True
        total_articles = db.article_count() + plan.estimated_articles
        already_embedded = db.embedding_count()
        plan.articles_to_embed = max(total_articles - already_embedded, 0)
        if plan.articles_to_embed > 0 and not plan.files_to_index:
            # No text reindex needed but embeddings are — still work to do
            pass

    return plan


# ── Indexing ─────────────────────────────────────────────────────────

_BATCH_SIZE = 10_000
_EMBED_BATCH_SIZE = 32


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
        ``"standard"`` stores the full body text;
        ``"semantic"`` does standard text + embedding vectors.
    on_progress:
        Optional callback ``(phase, items_done, items_total)``.

    Returns
    -------
    IndexPlan
        The plan that was executed (useful for reporting).
    """
    plan = estimate_index(drive_path, db, strategy=strategy)

    has_text_work = bool(plan.files_to_index)
    has_embed_work = strategy == "semantic"

    if not has_text_work and not has_embed_work:
        return plan

    # ── Phase 1: Text extraction ─────────────────────────────────
    if has_text_work:
        max_body = 500 if strategy == "fast" else 0

        # Speed up bulk insert
        db.conn.execute("PRAGMA synchronous=OFF")

        try:
            total_files = len(plan.files_to_index)
            cumulative_articles = 0

            for file_idx, zf in enumerate(plan.files_to_index):
                filename = zf.name
                source_id = db.upsert_source(filename, title=filename)

                # If re-indexing a changed file, remove stale articles first
                if db.get_meta(_meta_key(filename)) is not None:
                    db.delete_source_articles(source_id)

                # Extract and insert in batches
                batch: list[dict] = []
                file_articles = 0

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
                        file_articles += len(batch)
                        cumulative_articles += len(batch)
                        batch = []
                        if on_progress:
                            on_progress(filename, cumulative_articles, plan.estimated_articles)

                # Flush remaining
                if batch:
                    db.insert_articles_batch(batch)
                    file_articles += len(batch)
                    cumulative_articles += len(batch)
                    if on_progress:
                        on_progress(filename, cumulative_articles, plan.estimated_articles)

                # Store checksum so future runs skip this file
                db.set_meta(_meta_key(filename), _file_checksum(zf))

        finally:
            # Restore safe sync
            db.conn.execute("PRAGMA synchronous=FULL")

        # Record text tier
        text_tier = _TEXT_TIER.get(strategy, strategy)
        db.set_meta("tier", text_tier)
        db.set_meta("indexed_at", datetime.now(timezone.utc).isoformat())

    # ── Phase 2: Embedding (semantic only) ───────────────────────
    if has_embed_work:
        _run_embedding_phase(drive_path, db, on_progress)
        db.set_meta("tier", "semantic")
        db.set_meta("indexed_at", datetime.now(timezone.utc).isoformat())

    return plan


def find_embedding_model(drive_path: str | Path) -> Path | None:
    """Find an embedding model GGUF on the drive."""
    models_dir = Path(drive_path) / "models"
    if not models_dir.is_dir():
        return None
    # Look for nomic or other embedding models
    for pattern in ["*nomic*embed*", "*embed*", "*bge*"]:
        matches = list(models_dir.glob(pattern))
        if matches:
            return matches[0]
    return None


def _run_embedding_phase(
    drive_path: str | Path,
    db: SearchDB,
    on_progress: Callable[[str, int, int], None] | None = None,
) -> None:
    """Embed all unembedded articles via llama-server."""
    from svalbard.embedder import embed_batch, start_embedding_server, vectors_to_blob

    model_path = find_embedding_model(drive_path)
    if model_path is None:
        raise RuntimeError(
            "No embedding model found in models/. "
            "Download nomic-embed-text-v1.5 or add it to your preset."
        )

    db.ensure_embeddings_table()
    total = db.article_count()
    done = db.embed_resume_point()

    if done >= total:
        return  # all embedded

    # Start llama-server in embedding mode (find on drive or system PATH)
    from svalbard.embedder import _find_llama_server
    if on_progress:
        on_progress("Starting embedding server...", 0, total)
    llama_bin = _find_llama_server(drive_path)
    proc = start_embedding_server(str(model_path), llama_server_path=llama_bin)
    if on_progress:
        on_progress("Embedding articles...", 0, total)
    try:
        while True:
            rows = db.unembedded_articles(after_id=done, limit=_EMBED_BATCH_SIZE)
            if not rows:
                break

            article_ids = [r[0] for r in rows]
            # Combine title + body for embedding
            texts = [f"{r[1]} {r[2]}" for r in rows]

            vectors = embed_batch(texts)
            blobs = vectors_to_blob(vectors)

            db.insert_embeddings_batch(list(zip(article_ids, blobs)))
            done = article_ids[-1]

            if on_progress:
                on_progress("Embedding", db.embedding_count(), total)
    finally:
        proc.kill()
        proc.wait()
