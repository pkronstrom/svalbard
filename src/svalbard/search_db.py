"""SQLite FTS5 cross-ZIM search index.

Provides full-text search across articles extracted from multiple ZIM files.
Each ZIM is tracked as a *source*; articles are linked to their source so
the entire contribution of a ZIM can be added or removed atomically.
"""

from __future__ import annotations

import sqlite3
from pathlib import Path
from typing import Any

_SCHEMA_VERSION = "1"

_SCHEMA_SQL = """\
-- Key-value metadata (schema version, build timestamp, …)
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT
);

-- One row per indexed ZIM file.
CREATE TABLE IF NOT EXISTS sources (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    filename  TEXT NOT NULL UNIQUE,
    title     TEXT NOT NULL DEFAULT '',
    indexed_at TEXT  -- ISO-8601
);

-- Extracted article text, linked to its source.
CREATE TABLE IF NOT EXISTS articles (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    path      TEXT NOT NULL,
    title     TEXT NOT NULL DEFAULT '',
    body      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source_id);

-- FTS5 virtual table (content-sync'd to articles).
CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
    title,
    body,
    content='articles',
    content_rowid='id'
);

-- Triggers to keep FTS5 in sync with articles.
CREATE TRIGGER IF NOT EXISTS articles_ai AFTER INSERT ON articles BEGIN
    INSERT INTO articles_fts(rowid, title, body)
    VALUES (new.id, new.title, new.body);
END;

CREATE TRIGGER IF NOT EXISTS articles_ad AFTER DELETE ON articles BEGIN
    INSERT INTO articles_fts(articles_fts, rowid, title, body)
    VALUES ('delete', old.id, old.title, old.body);
END;

CREATE TRIGGER IF NOT EXISTS articles_au AFTER UPDATE ON articles BEGIN
    INSERT INTO articles_fts(articles_fts, rowid, title, body)
    VALUES ('delete', old.id, old.title, old.body);
    INSERT INTO articles_fts(rowid, title, body)
    VALUES (new.id, new.title, new.body);
END;
"""


class SearchDB:
    """Thin wrapper around an SQLite database with FTS5 full-text search."""

    def __init__(self, path: str | Path) -> None:
        self.path = Path(path)
        self.conn = sqlite3.connect(str(self.path))
        self.conn.execute("PRAGMA journal_mode=WAL")
        self.conn.execute("PRAGMA foreign_keys=ON")
        self._ensure_schema()

    # ── Schema bootstrap ──────────────────────────────────────────────

    def _ensure_schema(self) -> None:
        self.conn.executescript(_SCHEMA_SQL)
        if self.get_meta("schema_version") is None:
            self.set_meta("schema_version", _SCHEMA_VERSION)

    # ── Meta ──────────────────────────────────────────────────────────

    def set_meta(self, key: str, value: str) -> None:
        self.conn.execute(
            "INSERT INTO meta (key, value) VALUES (?, ?) "
            "ON CONFLICT(key) DO UPDATE SET value = excluded.value",
            (key, value),
        )
        self.conn.commit()

    def get_meta(self, key: str) -> str | None:
        row = self.conn.execute(
            "SELECT value FROM meta WHERE key = ?", (key,)
        ).fetchone()
        return row[0] if row else None

    # ── Sources ───────────────────────────────────────────────────────

    def upsert_source(self, filename: str, *, title: str = "") -> int:
        """Insert or update a source; returns the source id."""
        self.conn.execute(
            "INSERT INTO sources (filename, title) VALUES (?, ?) "
            "ON CONFLICT(filename) DO UPDATE SET title = excluded.title",
            (filename, title),
        )
        self.conn.commit()
        row = self.conn.execute(
            "SELECT id FROM sources WHERE filename = ?", (filename,)
        ).fetchone()
        return row[0]

    def delete_source(self, source_id: int) -> None:
        """Remove a source and all its articles (triggers clean up FTS)."""
        self.delete_source_articles(source_id)
        self.conn.execute("DELETE FROM sources WHERE id = ?", (source_id,))
        self.conn.commit()

    def indexed_filenames(self) -> set[str]:
        """Return the set of filenames currently in the sources table."""
        rows = self.conn.execute("SELECT filename FROM sources").fetchall()
        return {r[0] for r in rows}

    def source_count(self) -> int:
        return self.conn.execute("SELECT COUNT(*) FROM sources").fetchone()[0]

    # ── Articles ──────────────────────────────────────────────────────

    def insert_articles_batch(self, articles: list[dict[str, Any]]) -> None:
        """Bulk-insert articles.  Each dict must have source_id, path, title, body."""
        self.conn.executemany(
            "INSERT INTO articles (source_id, path, title, body) "
            "VALUES (:source_id, :path, :title, :body)",
            articles,
        )
        self.conn.commit()

    def delete_source_articles(self, source_id: int) -> None:
        """Delete all articles belonging to a source (triggers update FTS)."""
        self.conn.execute(
            "DELETE FROM articles WHERE source_id = ?", (source_id,)
        )
        self.conn.commit()

    def article_count(self) -> int:
        return self.conn.execute("SELECT COUNT(*) FROM articles").fetchone()[0]

    # ── Search ────────────────────────────────────────────────────────

    def search_fts(self, query: str, *, limit: int = 20) -> list[dict[str, Any]]:
        """Full-text search.  Returns dicts with title, body, path, rank."""
        rows = self.conn.execute(
            "SELECT a.title, a.body, a.path, a.source_id, f.rank "
            "FROM articles_fts f "
            "JOIN articles a ON a.id = f.rowid "
            "WHERE articles_fts MATCH ? "
            "ORDER BY f.rank "
            "LIMIT ?",
            (query, limit),
        ).fetchall()
        return [
            {"title": r[0], "body": r[1], "path": r[2], "source_id": r[3], "rank": r[4]}
            for r in rows
        ]

    # ── Embeddings ────────────────────────────────────────────────────

    def ensure_embeddings_table(self) -> None:
        """Create embeddings table if it doesn't exist."""
        self.conn.executescript(
            "CREATE TABLE IF NOT EXISTS embeddings ("
            "    article_id INTEGER PRIMARY KEY REFERENCES articles(id),"
            "    vector     BLOB NOT NULL"
            ");"
        )

    def insert_embeddings_batch(self, embeddings: list[tuple]) -> None:
        """Insert (article_id, vector_blob) tuples."""
        self.conn.executemany(
            "INSERT OR REPLACE INTO embeddings (article_id, vector) VALUES (?, ?)",
            embeddings,
        )
        self.conn.commit()

    def get_embeddings(self, article_ids: list[int]) -> dict[int, bytes]:
        """Fetch embedding vectors for given article IDs."""
        if not article_ids:
            return {}
        placeholders = ",".join("?" for _ in article_ids)
        rows = self.conn.execute(
            f"SELECT article_id, vector FROM embeddings "
            f"WHERE article_id IN ({placeholders})",
            article_ids,
        ).fetchall()
        return {row[0]: row[1] for row in rows}

    def embed_resume_point(self) -> int:
        """Return the last embedded article_id, or 0."""
        self.ensure_embeddings_table()
        row = self.conn.execute(
            "SELECT MAX(article_id) FROM embeddings"
        ).fetchone()
        return row[0] if row[0] is not None else 0

    def unembedded_articles(
        self, after_id: int = 0, limit: int = 1000
    ) -> list[tuple]:
        """Return (id, title, body) for articles not yet embedded."""
        self.ensure_embeddings_table()
        return self.conn.execute(
            "SELECT a.id, a.title, a.body FROM articles a "
            "LEFT JOIN embeddings e ON a.id = e.article_id "
            "WHERE e.article_id IS NULL AND a.id > ? "
            "ORDER BY a.id LIMIT ?",
            (after_id, limit),
        ).fetchall()

    # ── Stats ─────────────────────────────────────────────────────────

    def stats(self) -> dict[str, int]:
        return {
            "source_count": self.source_count(),
            "article_count": self.article_count(),
        }
