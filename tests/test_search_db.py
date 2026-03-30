"""Tests for svalbard.search_db — SQLite FTS5 cross-ZIM search index."""

from pathlib import Path

import pytest

from svalbard.search_db import SearchDB


@pytest.fixture
def db(tmp_path: Path) -> SearchDB:
    """Return an open SearchDB backed by a temp file."""
    return SearchDB(tmp_path / "search.db")


# ── Schema ────────────────────────────────────────────────────────────

def test_schema_tables_exist(db: SearchDB):
    """Opening a DB should create meta, sources, articles, and articles_fts."""
    cursor = db.conn.execute(
        "SELECT name FROM sqlite_master WHERE type IN ('table', 'view') ORDER BY name"
    )
    names = {row[0] for row in cursor.fetchall()}
    assert "meta" in names
    assert "sources" in names
    assert "articles" in names
    assert "articles_fts" in names


def test_wal_journal_mode(db: SearchDB):
    """Database should use WAL journal mode."""
    mode = db.conn.execute("PRAGMA journal_mode").fetchone()[0]
    assert mode == "wal"


# ── Meta ──────────────────────────────────────────────────────────────

def test_meta_set_and_get(db: SearchDB):
    """set_meta / get_meta round-trip."""
    db.set_meta("schema_version", "1")
    assert db.get_meta("schema_version") == "1"


def test_meta_get_missing_returns_none(db: SearchDB):
    """get_meta for a missing key returns None."""
    assert db.get_meta("no_such_key") is None


def test_meta_upsert(db: SearchDB):
    """set_meta overwrites an existing key."""
    db.set_meta("k", "v1")
    db.set_meta("k", "v2")
    assert db.get_meta("k") == "v2"


# ── Sources ───────────────────────────────────────────────────────────

def test_upsert_source_insert(db: SearchDB):
    """upsert_source inserts a new source row."""
    db.upsert_source("wiki.zim", title="Wikipedia")
    assert db.source_count() == 1


def test_upsert_source_update(db: SearchDB):
    """upsert_source with same filename updates rather than duplicates."""
    db.upsert_source("wiki.zim", title="Wikipedia v1")
    db.upsert_source("wiki.zim", title="Wikipedia v2")
    assert db.source_count() == 1
    # Verify the title was updated
    row = db.conn.execute(
        "SELECT title FROM sources WHERE filename = ?", ("wiki.zim",)
    ).fetchone()
    assert row[0] == "Wikipedia v2"


def test_indexed_filenames(db: SearchDB):
    """indexed_filenames returns the set of source filenames."""
    db.upsert_source("a.zim", title="A")
    db.upsert_source("b.zim", title="B")
    assert db.indexed_filenames() == {"a.zim", "b.zim"}


# ── Articles ──────────────────────────────────────────────────────────

def _seed_articles(db: SearchDB):
    """Helper: add a source and a batch of articles."""
    db.upsert_source("wiki.zim", title="Wikipedia")
    source_id = db.conn.execute(
        "SELECT id FROM sources WHERE filename = ?", ("wiki.zim",)
    ).fetchone()[0]
    db.insert_articles_batch([
        {"source_id": source_id, "path": "/A", "title": "Alpha", "body": "The quick brown fox"},
        {"source_id": source_id, "path": "/B", "title": "Beta", "body": "Slow green turtle"},
        {"source_id": source_id, "path": "/C", "title": "Foxes", "body": "Fox fox fox fox fox"},
    ])
    return source_id


def test_insert_articles_batch_and_count(db: SearchDB):
    """insert_articles_batch populates articles; article_count reflects it."""
    _seed_articles(db)
    assert db.article_count() == 3


def test_fts_search_finds_match(db: SearchDB):
    """search_fts returns matching articles."""
    _seed_articles(db)
    results = db.search_fts("fox")
    assert len(results) >= 1
    titles = [r["title"] for r in results]
    assert "Alpha" in titles or "Foxes" in titles


def test_fts_search_no_match(db: SearchDB):
    """search_fts returns empty list when nothing matches."""
    _seed_articles(db)
    assert db.search_fts("xylophone") == []


def test_fts_ranking(db: SearchDB):
    """More-relevant result (more occurrences of term) should rank first."""
    _seed_articles(db)
    results = db.search_fts("fox")
    # "Foxes" has 5 occurrences vs "Alpha" with 1, so it should rank higher
    assert results[0]["title"] == "Foxes"


def test_fts_search_limit(db: SearchDB):
    """search_fts respects a limit parameter."""
    _seed_articles(db)
    results = db.search_fts("fox", limit=1)
    assert len(results) == 1


# ── Delete source ─────────────────────────────────────────────────────

def test_delete_source_articles(db: SearchDB):
    """delete_source_articles removes articles for a source."""
    source_id = _seed_articles(db)
    db.delete_source_articles(source_id)
    assert db.article_count() == 0


def test_delete_source_cascades(db: SearchDB):
    """delete_source removes the source and its articles + FTS entries."""
    source_id = _seed_articles(db)
    db.delete_source(source_id)
    assert db.source_count() == 0
    assert db.article_count() == 0
    # FTS should also be empty
    assert db.search_fts("fox") == []


# ── Stats ─────────────────────────────────────────────────────────────

def test_stats(db: SearchDB):
    """stats returns a dict with source_count and article_count."""
    _seed_articles(db)
    s = db.stats()
    assert s["source_count"] == 1
    assert s["article_count"] == 3
