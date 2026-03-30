# Cross-ZIM Search Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add offline cross-ZIM full-text search with a Python indexer (`svalbard index`) and shell-based search on the drive (CLI + REST API).

**Architecture:** Python indexer extracts text from ZIM files via libzim, builds a SQLite FTS5 database. Shell scripts on the drive query it via bundled sqlite3 binary. Three tiers (fast → standard → semantic) layer on top of each other.

**Tech Stack:** Python + libzim + sqlite3 for indexing. Bash + sqlite3 CLI + kiwix-serve for runtime. Optional llama-server for semantic tier.

**Design doc:** `docs/plans/2026-03-30-cross-zim-search-design.md`

---

### Task 1: Search DB Module — Schema and Core Operations

Create the search database module that manages schema creation, metadata, and bulk operations.

**Files:**
- Create: `src/svalbard/search_db.py`
- Test: `tests/test_search_db.py`

**Step 1: Write the failing tests**

```python
# tests/test_search_db.py
import sqlite3
from pathlib import Path

from svalbard.search_db import SearchDB


def test_create_schema(tmp_path):
    """Opening a new DB should create all tables."""
    db = SearchDB(tmp_path / "search.db")
    db.close()

    conn = sqlite3.connect(tmp_path / "search.db")
    tables = {r[0] for r in conn.execute(
        "SELECT name FROM sqlite_master WHERE type IN ('table','view')"
    ).fetchall()}
    conn.close()

    assert "meta" in tables
    assert "sources" in tables
    assert "articles" in tables
    assert "articles_fts" in tables


def test_meta_get_set(tmp_path):
    """Should store and retrieve metadata key-value pairs."""
    db = SearchDB(tmp_path / "search.db")
    db.set_meta("tier", "fast")
    assert db.get_meta("tier") == "fast"
    assert db.get_meta("missing") is None
    db.close()


def test_upsert_source(tmp_path):
    """Should insert a new source and return its id."""
    db = SearchDB(tmp_path / "search.db")
    sid = db.upsert_source("wiki.zim", size_bytes=1000, checksum="abc123")
    assert sid == 1
    # Upsert same filename returns same id
    sid2 = db.upsert_source("wiki.zim", size_bytes=2000, checksum="def456")
    assert sid2 == 1
    db.close()


def test_insert_articles_batch(tmp_path):
    """Should insert articles and populate FTS index."""
    db = SearchDB(tmp_path / "search.db")
    sid = db.upsert_source("wiki.zim", size_bytes=1000, checksum="abc")
    articles = [
        (sid, "article/Audi", "Audi", "Audi is a German car manufacturer."),
        (sid, "article/BMW", "BMW", "BMW is a German automotive company."),
    ]
    db.insert_articles_batch(articles)

    # Verify FTS works
    results = db.search_fts("Audi", limit=10)
    assert len(results) == 1
    assert results[0]["title"] == "Audi"
    assert results[0]["source_filename"] == "wiki.zim"
    db.close()


def test_search_fts_ranking(tmp_path):
    """Results should be ranked by relevance."""
    db = SearchDB(tmp_path / "search.db")
    sid = db.upsert_source("wiki.zim", size_bytes=1000, checksum="abc")
    articles = [
        (sid, "a/1", "Audi A4 engine repair", "How to repair the Audi A4 engine."),
        (sid, "a/2", "German cars", "Audi and BMW are German car brands."),
        (sid, "a/3", "Physics", "Quantum mechanics is fascinating."),
    ]
    db.insert_articles_batch(articles)

    results = db.search_fts("Audi engine", limit=10)
    assert len(results) >= 1
    # First result should be the more relevant one
    assert "Audi A4" in results[0]["title"]
    db.close()


def test_source_filenames(tmp_path):
    """Should return set of indexed filenames."""
    db = SearchDB(tmp_path / "search.db")
    db.upsert_source("a.zim", size_bytes=100, checksum="a")
    db.upsert_source("b.zim", size_bytes=200, checksum="b")
    assert db.indexed_filenames() == {"a.zim", "b.zim"}
    db.close()


def test_article_count(tmp_path):
    """Should return total article count."""
    db = SearchDB(tmp_path / "search.db")
    sid = db.upsert_source("w.zim", size_bytes=100, checksum="a")
    db.insert_articles_batch([
        (sid, "a/1", "One", "First"),
        (sid, "a/2", "Two", "Second"),
    ])
    assert db.article_count() == 2
    db.close()


def test_delete_source_cascades(tmp_path):
    """Deleting a source should remove its articles and FTS entries."""
    db = SearchDB(tmp_path / "search.db")
    sid = db.upsert_source("old.zim", size_bytes=100, checksum="a")
    db.insert_articles_batch([(sid, "a/1", "Old", "Old article.")])
    assert db.article_count() == 1

    db.delete_source(sid)
    assert db.article_count() == 0
    assert "old.zim" not in db.indexed_filenames()
    db.close()
```

**Step 2: Run tests to verify they fail**

Run: `cd . && uv run pytest tests/test_search_db.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'svalbard.search_db'`

**Step 3: Implement search_db.py**

```python
# src/svalbard/search_db.py
"""Search index database — schema, writes, and queries over SQLite FTS5."""

import sqlite3
from pathlib import Path

SCHEMA_VERSION = "1"

_SCHEMA = """
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS sources (
    id            INTEGER PRIMARY KEY,
    filename      TEXT UNIQUE NOT NULL,
    size_bytes    INTEGER,
    checksum      TEXT,
    article_count INTEGER DEFAULT 0,
    indexed_at    TEXT
);

CREATE TABLE IF NOT EXISTS articles (
    id        INTEGER PRIMARY KEY,
    source_id INTEGER NOT NULL REFERENCES sources(id),
    path      TEXT NOT NULL,
    title     TEXT,
    body      TEXT,
    UNIQUE(source_id, path)
);

CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
    title, body,
    content='articles',
    content_rowid='id'
);

-- Keep FTS in sync with articles table
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
    def __init__(self, path: Path | str):
        self.path = Path(path)
        self.conn = sqlite3.connect(str(self.path))
        self.conn.execute("PRAGMA journal_mode=WAL")
        self.conn.executescript(_SCHEMA)
        existing_version = self.get_meta("version")
        if existing_version is None:
            self.set_meta("version", SCHEMA_VERSION)

    def close(self):
        self.conn.close()

    def set_meta(self, key: str, value: str):
        self.conn.execute(
            "INSERT OR REPLACE INTO meta(key, value) VALUES (?, ?)",
            (key, value),
        )
        self.conn.commit()

    def get_meta(self, key: str) -> str | None:
        row = self.conn.execute(
            "SELECT value FROM meta WHERE key = ?", (key,)
        ).fetchone()
        return row[0] if row else None

    def upsert_source(
        self, filename: str, size_bytes: int, checksum: str
    ) -> int:
        self.conn.execute(
            """INSERT INTO sources(filename, size_bytes, checksum)
               VALUES (?, ?, ?)
               ON CONFLICT(filename) DO UPDATE SET
                 size_bytes=excluded.size_bytes,
                 checksum=excluded.checksum""",
            (filename, size_bytes, checksum),
        )
        self.conn.commit()
        row = self.conn.execute(
            "SELECT id FROM sources WHERE filename = ?", (filename,)
        ).fetchone()
        return row[0]

    def insert_articles_batch(self, articles: list[tuple]):
        """Insert articles as (source_id, path, title, body) tuples."""
        self.conn.executemany(
            "INSERT OR REPLACE INTO articles(source_id, path, title, body) VALUES (?, ?, ?, ?)",
            articles,
        )
        self.conn.commit()

    def delete_source(self, source_id: int):
        self.conn.execute("DELETE FROM articles WHERE source_id = ?", (source_id,))
        self.conn.execute("DELETE FROM sources WHERE id = ?", (source_id,))
        self.conn.commit()

    def search_fts(self, query: str, limit: int = 20) -> list[dict]:
        rows = self.conn.execute(
            """SELECT a.id, s.filename, a.path, a.title,
                      snippet(articles_fts, 1, '>', '<', '...', 20) as snippet
               FROM articles_fts
               JOIN articles a ON a.id = articles_fts.rowid
               JOIN sources  s ON s.id = a.source_id
               WHERE articles_fts MATCH ?
               ORDER BY rank
               LIMIT ?""",
            (query, limit),
        ).fetchall()
        return [
            {
                "id": r[0],
                "source_filename": r[1],
                "path": r[2],
                "title": r[3],
                "snippet": r[4],
            }
            for r in rows
        ]

    def indexed_filenames(self) -> set[str]:
        rows = self.conn.execute("SELECT filename FROM sources").fetchall()
        return {r[0] for r in rows}

    def article_count(self) -> int:
        row = self.conn.execute("SELECT COUNT(*) FROM articles").fetchone()
        return row[0]

    def source_count(self) -> int:
        row = self.conn.execute("SELECT COUNT(*) FROM sources").fetchone()
        return row[0]

    def stats(self) -> dict:
        return {
            "sources": self.source_count(),
            "articles": self.article_count(),
            "tier": self.get_meta("tier") or "none",
            "version": self.get_meta("version") or "0",
        }
```

**Step 4: Run tests to verify they pass**

Run: `cd . && uv run pytest tests/test_search_db.py -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add src/svalbard/search_db.py tests/test_search_db.py
git commit -m "feat(search): add search database module with FTS5 schema"
```

---

### Task 2: ZIM Text Extractor

Extract article text from ZIM files using libzim. Add `libzim` as an optional dependency.

**Files:**
- Create: `src/svalbard/zim_extract.py`
- Modify: `pyproject.toml` (add libzim optional dep)
- Test: `tests/test_zim_extract.py`

**Step 1: Add libzim dependency**

In `pyproject.toml`, add an optional dependency group:

```toml
[project.optional-dependencies]
search = ["libzim>=3.1"]
```

Run: `cd . && uv sync --extra search`

**Step 2: Write the failing tests**

```python
# tests/test_zim_extract.py
import pytest

from svalbard.zim_extract import strip_html, truncate_text


def test_strip_html_removes_tags():
    assert strip_html("<p>Hello <b>world</b></p>") == "Hello world"


def test_strip_html_collapses_whitespace():
    assert strip_html("<p>Hello   \n\n  world</p>") == "Hello world"


def test_strip_html_handles_empty():
    assert strip_html("") == ""
    assert strip_html("<div></div>") == ""


def test_strip_html_decodes_entities():
    assert strip_html("&amp; &lt; &gt;") == "& < >"


def test_truncate_text_short_text():
    """Short text should not be truncated."""
    assert truncate_text("Hello world", 500) == "Hello world"


def test_truncate_text_long_text():
    """Long text should be truncated at sentence boundary."""
    text = "First sentence. Second sentence. " * 50
    result = truncate_text(text, 100)
    assert len(result) <= 120  # allow some slack for sentence boundary
    assert result.endswith(".")


def test_truncate_text_no_sentence_boundary():
    """If no sentence boundary, truncate at word boundary."""
    text = "word " * 200
    result = truncate_text(text, 100)
    assert len(result) <= 110
    assert not result.endswith(" ")
```

Note: We test `strip_html` and `truncate_text` as pure functions. The `extract_articles` function requires a real ZIM file, so we'll test it via integration in Task 3. If libzim is not installed, these tests still pass since they don't import libzim.

**Step 3: Run tests to verify they fail**

Run: `cd . && uv run pytest tests/test_zim_extract.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'svalbard.zim_extract'`

**Step 4: Implement zim_extract.py**

```python
# src/svalbard/zim_extract.py
"""Extract article text from ZIM files."""

import html
import re
from collections.abc import Iterator
from pathlib import Path


def strip_html(raw: str) -> str:
    """Remove HTML tags, decode entities, collapse whitespace."""
    text = re.sub(r"<[^>]+>", " ", raw)
    text = html.unescape(text)
    text = re.sub(r"\s+", " ", text).strip()
    return text


def truncate_text(text: str, max_chars: int) -> str:
    """Truncate text at a sentence or word boundary."""
    if len(text) <= max_chars:
        return text
    # Try to find last sentence boundary within limit
    truncated = text[:max_chars]
    last_period = truncated.rfind(". ")
    if last_period > max_chars // 2:
        return truncated[: last_period + 1]
    # Fall back to word boundary
    last_space = truncated.rfind(" ")
    if last_space > 0:
        return truncated[:last_space]
    return truncated


def extract_articles(
    zim_path: Path, max_body_chars: int = 500
) -> Iterator[tuple[str, str, str]]:
    """Yield (path, title, body) for each content article in a ZIM file.

    Skips redirects and metadata entries. Body is HTML-stripped and truncated.
    Requires libzim to be installed.
    """
    from libzim.reader import Archive  # type: ignore[import-untyped]

    archive = Archive(str(zim_path))
    for i in range(archive.entry_count):
        entry = archive._get_entry_by_id(i)
        if entry.is_redirect:
            continue
        item = entry.get_item()
        mimetype = item.mimetype
        if not mimetype.startswith("text/html"):
            continue
        path = entry.path
        title = entry.title or path.split("/")[-1]
        try:
            raw_content = bytes(item.content).decode("utf-8", errors="replace")
        except Exception:
            continue
        body = strip_html(raw_content)
        if not body:
            continue
        if max_body_chars > 0:
            body = truncate_text(body, max_body_chars)
        yield path, title, body


def article_count(zim_path: Path) -> int:
    """Return the article count for a ZIM without iterating all entries."""
    from libzim.reader import Archive  # type: ignore[import-untyped]

    archive = Archive(str(zim_path))
    return archive.article_count
```

**Step 5: Run tests to verify they pass**

Run: `cd . && uv run pytest tests/test_zim_extract.py -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add pyproject.toml src/svalbard/zim_extract.py tests/test_zim_extract.py
git commit -m "feat(search): add ZIM text extractor with HTML stripping"
```

---

### Task 3: Indexer Core — Scan, Estimate, Build Index

The main indexing logic: scan ZIM files, detect changes, estimate work, build index incrementally.

**Files:**
- Create: `src/svalbard/indexer.py`
- Test: `tests/test_indexer.py`

**Step 1: Write the failing tests**

```python
# tests/test_indexer.py
from pathlib import Path
from unittest.mock import patch, MagicMock

import pytest

from svalbard.indexer import scan_zim_files, estimate_index, IndexPlan, run_index
from svalbard.search_db import SearchDB


def test_scan_zim_files_finds_zims(tmp_path):
    """Should find all .zim files in zim/ directory."""
    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    (zim_dir / "wiki.zim").write_bytes(b"x" * 100)
    (zim_dir / "ifixit.zim").write_bytes(b"y" * 200)
    (zim_dir / "readme.txt").write_text("not a zim")

    files = scan_zim_files(tmp_path)
    assert len(files) == 2
    names = {f.name for f in files}
    assert names == {"wiki.zim", "ifixit.zim"}


def test_scan_zim_files_empty(tmp_path):
    """Should return empty list if no zim/ directory."""
    assert scan_zim_files(tmp_path) == []


def test_estimate_index_new_files(tmp_path):
    """Estimate should identify all files as new when DB is empty."""
    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    (zim_dir / "a.zim").write_bytes(b"x" * 1000)
    (zim_dir / "b.zim").write_bytes(b"y" * 2000)

    db = SearchDB(tmp_path / "search.db")
    plan = estimate_index(tmp_path, db, strategy="fast")
    db.close()

    assert plan.total_zims == 2
    assert plan.new_zims == 2
    assert plan.already_indexed == 0
    assert plan.strategy == "fast"
    assert plan.estimated_db_bytes > 0


def test_estimate_index_incremental(tmp_path):
    """Already-indexed ZIMs with same checksum should be skipped."""
    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    zim_file = zim_dir / "a.zim"
    zim_file.write_bytes(b"x" * 1000)

    db = SearchDB(tmp_path / "search.db")
    # Simulate a previously indexed file
    checksum = f"{zim_file.stat().st_size}:{zim_file.stat().st_mtime}"
    db.upsert_source("a.zim", size_bytes=1000, checksum=checksum)
    plan = estimate_index(tmp_path, db, strategy="fast")
    db.close()

    assert plan.total_zims == 1
    assert plan.new_zims == 0
    assert plan.already_indexed == 1


@patch("svalbard.indexer.extract_articles")
@patch("svalbard.indexer.zim_article_count")
def test_run_index_fast(mock_count, mock_extract, tmp_path):
    """run_index should populate DB from ZIM files."""
    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    zim_file = zim_dir / "test.zim"
    zim_file.write_bytes(b"x" * 100)

    mock_count.return_value = 2
    mock_extract.return_value = iter([
        ("article/Foo", "Foo", "Foo is great."),
        ("article/Bar", "Bar", "Bar is also great."),
    ])

    db = SearchDB(tmp_path / "search.db")
    run_index(tmp_path, db, strategy="fast")

    assert db.article_count() == 2
    assert db.source_count() == 1
    assert db.get_meta("tier") == "fast"

    results = db.search_fts("Foo", limit=5)
    assert len(results) == 1
    assert results[0]["title"] == "Foo"
    db.close()


@patch("svalbard.indexer.extract_articles")
@patch("svalbard.indexer.zim_article_count")
def test_run_index_incremental(mock_count, mock_extract, tmp_path):
    """Second run should skip already-indexed ZIMs."""
    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    zim_file = zim_dir / "test.zim"
    zim_file.write_bytes(b"x" * 100)

    mock_count.return_value = 1
    mock_extract.return_value = iter([
        ("article/A", "A", "Article A."),
    ])

    db = SearchDB(tmp_path / "search.db")
    run_index(tmp_path, db, strategy="fast")
    assert db.article_count() == 1

    # Second run — same file, should skip
    mock_extract.return_value = iter([
        ("article/B", "B", "Should not be indexed."),
    ])
    run_index(tmp_path, db, strategy="fast")
    assert db.article_count() == 1  # unchanged
    db.close()
```

**Step 2: Run tests to verify they fail**

Run: `cd . && uv run pytest tests/test_indexer.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'svalbard.indexer'`

**Step 3: Implement indexer.py**

```python
# src/svalbard/indexer.py
"""Index ZIM files into the search database."""

from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

from rich.console import Console
from rich.progress import Progress

from svalbard.search_db import SearchDB
from svalbard.zim_extract import (
    extract_articles as _extract_articles,
    article_count as _article_count,
)

console = Console()

# Re-export for mocking in tests
extract_articles = _extract_articles
zim_article_count = _article_count

BATCH_SIZE = 10_000

# Estimation: bytes per article by tier
BYTES_PER_ARTICLE = {"fast": 500, "standard": 2000, "semantic": 3000}


@dataclass
class IndexPlan:
    total_zims: int
    new_zims: int
    already_indexed: int
    changed_zims: int = 0
    missing_zims: int = 0
    strategy: str = "fast"
    estimated_articles: int = 0
    estimated_db_bytes: int = 0
    files_to_index: list[Path] | None = None


def _file_checksum(path: Path) -> str:
    """Quick checksum: size + mtime. Not cryptographic, but fast."""
    stat = path.stat()
    return f"{stat.st_size}:{stat.st_mtime}"


def scan_zim_files(drive_path: Path) -> list[Path]:
    """Find all .zim files in the drive's zim/ directory."""
    zim_dir = drive_path / "zim"
    if not zim_dir.exists():
        return []
    return sorted(zim_dir.glob("*.zim"))


def estimate_index(
    drive_path: Path, db: SearchDB, strategy: str = "fast"
) -> IndexPlan:
    """Scan ZIMs and estimate indexing work."""
    zim_files = scan_zim_files(drive_path)
    indexed = db.indexed_filenames()

    new_files = []
    changed_files = []
    already = 0

    for zim_file in zim_files:
        if zim_file.name in indexed:
            # Check if file changed
            row = db.conn.execute(
                "SELECT checksum FROM sources WHERE filename = ?",
                (zim_file.name,),
            ).fetchone()
            current_checksum = _file_checksum(zim_file)
            if row and row[0] == current_checksum:
                already += 1
            else:
                changed_files.append(zim_file)
        else:
            new_files.append(zim_file)

    # Check for ZIMs removed from disk
    disk_names = {f.name for f in zim_files}
    missing = len(indexed - disk_names)

    files_to_index = new_files + changed_files
    total_size = sum(f.stat().st_size for f in files_to_index)
    # Rough estimate: 1 article per 5KB of ZIM data
    est_articles = int(total_size / 5000) if files_to_index else 0
    est_db = est_articles * BYTES_PER_ARTICLE.get(strategy, 500)

    return IndexPlan(
        total_zims=len(zim_files),
        new_zims=len(new_files),
        already_indexed=already,
        changed_zims=len(changed_files),
        missing_zims=missing,
        strategy=strategy,
        estimated_articles=est_articles,
        estimated_db_bytes=est_db,
        files_to_index=files_to_index,
    )


def run_index(
    drive_path: Path,
    db: SearchDB,
    strategy: str = "fast",
    on_progress: callable | None = None,
) -> None:
    """Index ZIM files into the search database.

    Incremental: skips ZIMs already indexed with matching checksum.
    """
    plan = estimate_index(drive_path, db, strategy)
    if not plan.files_to_index:
        return

    max_body = 500 if strategy == "fast" else 0  # 0 = no truncation

    # Set pragmas for bulk insert performance
    db.conn.execute("PRAGMA synchronous=OFF")

    for zim_file in plan.files_to_index:
        checksum = _file_checksum(zim_file)
        source_id = db.upsert_source(
            zim_file.name,
            size_bytes=zim_file.stat().st_size,
            checksum=checksum,
        )

        # Delete old articles if re-indexing a changed file
        db.delete_source_articles(source_id)

        batch = []
        count = 0
        for path, title, body in extract_articles(zim_file, max_body_chars=max_body):
            batch.append((source_id, path, title, body))
            count += 1
            if len(batch) >= BATCH_SIZE:
                db.insert_articles_batch(batch)
                batch = []
                if on_progress:
                    on_progress(zim_file.name, count)

        if batch:
            db.insert_articles_batch(batch)

        # Update source metadata
        db.conn.execute(
            "UPDATE sources SET article_count = ?, indexed_at = ? WHERE id = ?",
            (count, datetime.now(timezone.utc).isoformat(), source_id),
        )
        db.conn.commit()

    # Restore safe pragma
    db.conn.execute("PRAGMA synchronous=FULL")

    db.set_meta("tier", strategy)
    db.set_meta("indexed_at", datetime.now(timezone.utc).isoformat())
```

Note: this needs a `delete_source_articles` method on `SearchDB`. Add it:

In `src/svalbard/search_db.py`, add method to `SearchDB`:

```python
def delete_source_articles(self, source_id: int):
    """Delete all articles for a source (but keep the source row)."""
    self.conn.execute("DELETE FROM articles WHERE source_id = ?", (source_id,))
    self.conn.commit()
```

**Step 4: Run tests to verify they pass**

Run: `cd . && uv run pytest tests/test_indexer.py -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add src/svalbard/indexer.py src/svalbard/search_db.py tests/test_indexer.py
git commit -m "feat(search): add indexer core with incremental ZIM scanning"
```

---

### Task 4: CLI Command — `svalbard index`

Wire up the indexer to the Click CLI with estimation, confirmation, and progress display.

**Files:**
- Modify: `src/svalbard/cli.py` (add `index` command)
- Test: `tests/test_cli_index.py`

**Step 1: Write the failing test**

```python
# tests/test_cli_index.py
from pathlib import Path
from unittest.mock import patch
from click.testing import CliRunner

from svalbard.cli import main


@patch("svalbard.indexer.extract_articles")
@patch("svalbard.indexer.zim_article_count")
def test_index_command_creates_db(mock_count, mock_extract, tmp_path):
    """svalbard index should create search.db in data/."""
    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    (zim_dir / "test.zim").write_bytes(b"x" * 100)

    mock_count.return_value = 1
    mock_extract.return_value = iter([("a/1", "Test", "Test body.")])

    runner = CliRunner()
    result = runner.invoke(main, ["index", str(tmp_path), "--yes"])
    assert result.exit_code == 0
    assert (tmp_path / "data" / "search.db").exists()


def test_index_command_no_zims(tmp_path):
    """Should handle drive with no ZIM files gracefully."""
    runner = CliRunner()
    result = runner.invoke(main, ["index", str(tmp_path), "--yes"])
    assert result.exit_code == 0
    assert "No ZIM files" in result.output
```

**Step 2: Run test to verify it fails**

Run: `cd . && uv run pytest tests/test_cli_index.py -v`
Expected: FAIL — no `index` command registered

**Step 3: Add index command to cli.py**

Add to `src/svalbard/cli.py`:

```python
@main.command()
@click.argument("path", default=".")
@click.option(
    "--strategy",
    type=click.Choice(["fast", "standard", "semantic"]),
    default="fast",
    help="Indexing strategy tier",
)
@click.option("--yes", "-y", is_flag=True, help="Skip confirmation prompt")
def index(path: str, strategy: str, yes: bool) -> None:
    """Build search index over ZIM files on a drive."""
    from pathlib import Path as P

    from svalbard.indexer import estimate_index, run_index, scan_zim_files
    from svalbard.search_db import SearchDB

    drive_path = P(path)
    zim_files = scan_zim_files(drive_path)
    if not zim_files:
        console.print("[dim]No ZIM files found in zim/[/dim]")
        return

    data_dir = drive_path / "data"
    data_dir.mkdir(parents=True, exist_ok=True)
    db = SearchDB(data_dir / "search.db")

    try:
        plan = estimate_index(drive_path, db, strategy)

        console.print(f"\n[bold]Found {plan.total_zims} ZIM files[/bold]")
        if plan.already_indexed:
            console.print(f"  {plan.already_indexed} already indexed")
        if plan.new_zims:
            console.print(f"  {plan.new_zims} new")
        if plan.changed_zims:
            console.print(f"  {plan.changed_zims} changed")
        if plan.missing_zims:
            console.print(f"  [yellow]{plan.missing_zims} removed from disk[/yellow]")

        if not plan.files_to_index:
            console.print("\n[green]Index is up to date.[/green]")
            return

        est_mb = plan.estimated_db_bytes / 1e6
        console.print(f"\n  Strategy: {strategy}")
        console.print(f"  Estimated index size: ~{est_mb:.0f} MB")

        if not yes:
            from rich.prompt import Confirm

            if not Confirm.ask("\n  Proceed?", default=True):
                return

        from rich.progress import Progress

        with Progress(console=console) as progress:
            task = progress.add_task("Indexing...", total=len(plan.files_to_index))

            def on_file_done(filename, count):
                pass  # progress updated per-file below

            for i, zim_file in enumerate(plan.files_to_index):
                progress.update(task, description=f"Indexing {zim_file.name}...")
                # Index one file at a time via run_index with a single-file plan
                from svalbard.indexer import _file_checksum, BATCH_SIZE
                from svalbard.zim_extract import extract_articles as do_extract
                from datetime import datetime, timezone

                checksum = _file_checksum(zim_file)
                source_id = db.upsert_source(
                    zim_file.name,
                    size_bytes=zim_file.stat().st_size,
                    checksum=checksum,
                )
                db.delete_source_articles(source_id)
                max_body = 500 if strategy == "fast" else 0
                batch = []
                count = 0
                for path_str, title, body in do_extract(zim_file, max_body_chars=max_body):
                    batch.append((source_id, path_str, title, body))
                    count += 1
                    if len(batch) >= BATCH_SIZE:
                        db.insert_articles_batch(batch)
                        batch = []
                if batch:
                    db.insert_articles_batch(batch)
                db.conn.execute(
                    "UPDATE sources SET article_count = ?, indexed_at = ? WHERE id = ?",
                    (count, datetime.now(timezone.utc).isoformat(), source_id),
                )
                db.conn.commit()
                progress.advance(task)
                console.print(f"  [green]OK[/green] {zim_file.name} — {count} articles")

        db.set_meta("tier", strategy)
        db.set_meta("indexed_at", datetime.now(timezone.utc).isoformat())

        stats = db.stats()
        console.print(
            f"\n[bold green]Done.[/bold green] "
            f"{stats['sources']} sources, {stats['articles']} articles indexed."
        )
    finally:
        db.close()
```

**Step 4: Run tests to verify they pass**

Run: `cd . && uv run pytest tests/test_cli_index.py -v`
Expected: all PASS

**Step 5: Run all tests to verify nothing is broken**

Run: `cd . && uv run pytest -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add src/svalbard/cli.py tests/test_cli_index.py
git commit -m "feat(search): add 'svalbard index' CLI command"
```

---

### Task 5: Drive-Side CLI Search — `search.sh`

Shell script that queries the FTS5 index via bundled sqlite3 and optionally opens results in kiwix-serve.

**Files:**
- Create: `recipes/actions/search.sh`
- Test: manual — run against a test search.db

**Step 1: Write search.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"

DB="$DRIVE_ROOT/data/search.db"
if [ ! -f "$DB" ]; then
    ui_error "Search index not found. Run 'svalbard index' to build it."
    exit 1
fi

SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -z "$SQLITE_BIN" ]; then
    ui_error "sqlite3 not found."
    exit 1
fi

# Get query from argument or prompt
query="${1:-}"
if [ -z "$query" ]; then
    read -rp "Search: " query
    if [ -z "$query" ]; then
        exit 0
    fi
fi

# Escape single quotes in query for SQL
safe_query="${query//\'/\'\'}"
# Convert spaces to FTS5 implicit AND
fts_query="$safe_query"

# Get index stats
stats=$("$SQLITE_BIN" -separator $'\t' "$DB" "
    SELECT
        (SELECT COUNT(*) FROM sources),
        (SELECT COUNT(*) FROM articles);
")
IFS=$'\t' read -r src_count art_count <<< "$stats"
echo ""
echo "${DIM}Searching $src_count sources, $art_count articles...${NC}"
echo ""

# Run FTS5 query
results=$("$SQLITE_BIN" -separator $'\t' "$DB" "
    SELECT a.id, s.filename, a.path, a.title,
           snippet(articles_fts, 1, '>', '<', '...', 12)
    FROM articles_fts
    JOIN articles a ON a.id = articles_fts.rowid
    JOIN sources  s ON s.id = a.source_id
    WHERE articles_fts MATCH '${fts_query}'
    ORDER BY rank
    LIMIT 20;
")

if [ -z "$results" ]; then
    echo "  No results found."
    exit 0
fi

# Parse and display results
declare -a IDS FILENAMES PATHS TITLES SNIPPETS
i=0
while IFS=$'\t' read -r id filename path title snippet; do
    IDS+=("$id")
    FILENAMES+=("$filename")
    PATHS+=("$path")
    TITLES+=("$title")
    SNIPPETS+=("$snippet")
    # Extract short source label from filename (e.g. "wikipedia-en" from "wikipedia_en_all.zim")
    label="${filename%.zim}"
    label="${label%%_*}"
    i=$((i + 1))
    printf "  ${CYAN}%2d${NC}) [${DIM}%s${NC}] ${BOLD}%s${NC}\n" "$i" "$label" "$title"
    if [ -n "$snippet" ]; then
        printf "      %s\n" "$snippet"
    fi
done <<< "$results"

echo ""
read -rp "  Open result [1-$i, q to quit]: " choice
case "$choice" in
    q|Q|"") exit 0 ;;
    *[!0-9]*) exit 0 ;;
esac

if [ "$choice" -ge 1 ] 2>/dev/null && [ "$choice" -le "$i" ]; then
    idx=$((choice - 1))
    filename="${FILENAMES[$idx]}"
    article_path="${PATHS[$idx]}"
    # Derive kiwix-serve content name from filename
    zim_name="${filename%.zim}"

    # Check if kiwix-serve is running
    kiwix_port=""
    for port in 8080 8081 8082 8083 8084; do
        if (echo >/dev/tcp/localhost/"$port") 2>/dev/null; then
            kiwix_port="$port"
            break
        fi
    done

    if [ -n "$kiwix_port" ]; then
        url="http://localhost:${kiwix_port}/${zim_name}/${article_path}"
        echo "  → ${url}"
        open_browser "$url"
    else
        echo "  ${YELLOW}kiwix-serve not running. Start it with browse or serve-all.${NC}"
        echo "  Article: ${zim_name} / ${article_path}"
    fi
fi
```

**Step 2: Verify the script is syntactically correct**

Run: `bash -n recipes/actions/search.sh`
Expected: no output (no syntax errors)

**Step 3: Commit**

```bash
git add recipes/actions/search.sh
git commit -m "feat(search): add drive-side CLI search script"
```

---

### Task 6: Drive-Side REST API — `search-server.sh`

Minimal HTTP server using socat or bash /dev/tcp that serves JSON search results.

**Files:**
- Create: `recipes/actions/search-server.sh`
- Create: `recipes/actions/lib/search-cgi.sh` (CGI handler)

**Step 1: Write search-server.sh and search-cgi.sh**

`recipes/actions/search-server.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"
source "$DRIVE_ROOT/.svalbard/lib/binaries.sh"
source "$DRIVE_ROOT/.svalbard/lib/process.sh"

DB="$DRIVE_ROOT/data/search.db"
if [ ! -f "$DB" ]; then
    ui_error "Search index not found. Run 'svalbard index' to build it."
    exit 1
fi

SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -z "$SQLITE_BIN" ]; then
    ui_error "sqlite3 not found."
    exit 1
fi

PORT="${1:-8084}"
BIND="${2:-127.0.0.1}"
KIWIX_PORT="${3:-8080}"

export DB SQLITE_BIN DRIVE_ROOT KIWIX_PORT

trap_cleanup

ui_status "Search API: http://$BIND:$PORT"
ui_status "  GET /search?q=query"
ui_status "  GET /article/{id}"
ui_status "  GET /health"

# Use socat if available, otherwise fall back to bash loop
SOCAT_BIN="$(command -v socat 2>/dev/null || true)"

if [ -n "$SOCAT_BIN" ]; then
    "$SOCAT_BIN" "TCP-LISTEN:${PORT},bind=${BIND},reuseaddr,fork" \
        EXEC:"$DRIVE_ROOT/.svalbard/lib/search-cgi.sh" &
    SVALBARD_PIDS+=($!)
else
    # Fallback: bash-only HTTP server (single-threaded)
    while true; do
        {
            read -r request_line
            method=$(echo "$request_line" | cut -d' ' -f1)
            path=$(echo "$request_line" | cut -d' ' -f2)
            # Read headers until empty line
            while IFS= read -r header && [ -n "$(echo "$header" | tr -d '\r\n')" ]; do :; done
            export REQUEST_METHOD="$method" REQUEST_URI="$path"
            response=$("$DRIVE_ROOT/.svalbard/lib/search-cgi.sh" 2>/dev/null)
            echo -ne "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nAccess-Control-Allow-Origin: *\r\nConnection: close\r\n\r\n${response}"
        } < /dev/tcp/"$BIND"/"$PORT" || sleep 0.1
    done &
    SVALBARD_PIDS+=($!)
fi

wait_for_services
```

`recipes/actions/lib/search-cgi.sh`:

```bash
#!/usr/bin/env bash
# CGI handler for search API. Called by socat or bash HTTP loop.
# Expects: DB, SQLITE_BIN, KIWIX_PORT environment variables.

# Read HTTP request
read -r request_line 2>/dev/null || true
method=$(echo "$request_line" | cut -d' ' -f1)
uri=$(echo "$request_line" | cut -d' ' -f2 | tr -d '\r')

# Consume headers
while IFS= read -r header && [ -n "$(echo "$header" | tr -d '\r\n')" ]; do :; done

# Use REQUEST_URI if set (from parent), otherwise parse from request
path="${REQUEST_URI:-$uri}"

send_json() {
    local body="$1"
    local status="${2:-200 OK}"
    printf "HTTP/1.1 %s\r\n" "$status"
    printf "Content-Type: application/json\r\n"
    printf "Access-Control-Allow-Origin: *\r\n"
    printf "Connection: close\r\n"
    printf "\r\n"
    printf "%s" "$body"
}

# URL decode
urldecode() {
    local data="${1//+/ }"
    printf '%b' "${data//%/\\x}"
}

# Route: /health
if [[ "$path" == "/health" ]]; then
    stats=$("$SQLITE_BIN" -json "$DB" "
        SELECT
            (SELECT value FROM meta WHERE key='tier') as tier,
            (SELECT COUNT(*) FROM sources) as sources,
            (SELECT COUNT(*) FROM articles) as articles;
    " 2>/dev/null)
    send_json "${stats:-[{}]}"
    exit 0
fi

# Route: /search?q=...
if [[ "$path" == /search* ]]; then
    query_string="${path#*\?}"
    q=""
    IFS='&' read -ra params <<< "$query_string"
    limit=20
    for param in "${params[@]}"; do
        key="${param%%=*}"
        val="${param#*=}"
        case "$key" in
            q) q="$(urldecode "$val")" ;;
            limit) limit="$val" ;;
        esac
    done

    if [ -z "$q" ]; then
        send_json '{"error":"missing q parameter"}' "400 Bad Request"
        exit 0
    fi

    safe_q="${q//\'/\'\'}"
    results=$("$SQLITE_BIN" -json "$DB" "
        SELECT a.id, s.filename as source, a.path, a.title,
               snippet(articles_fts, 1, '', '', '...', 12) as snippet
        FROM articles_fts
        JOIN articles a ON a.id = articles_fts.rowid
        JOIN sources  s ON s.id = a.source_id
        WHERE articles_fts MATCH '${safe_q}'
        ORDER BY rank
        LIMIT ${limit};
    " 2>/dev/null)
    send_json "${results:-[]}"
    exit 0
fi

# Route: /article/{id}
if [[ "$path" =~ ^/article/([0-9]+)$ ]]; then
    article_id="${BASH_REMATCH[1]}"
    row=$("$SQLITE_BIN" -separator $'\t' "$DB" "
        SELECT s.filename, a.path FROM articles a
        JOIN sources s ON s.id = a.source_id
        WHERE a.id = ${article_id};
    " 2>/dev/null)
    if [ -z "$row" ]; then
        send_json '{"error":"not found"}' "404 Not Found"
        exit 0
    fi
    IFS=$'\t' read -r filename article_path <<< "$row"
    zim_name="${filename%.zim}"
    url="http://localhost:${KIWIX_PORT:-8080}/${zim_name}/${article_path}"
    printf "HTTP/1.1 302 Found\r\n"
    printf "Location: %s\r\n" "$url"
    printf "Connection: close\r\n"
    printf "\r\n"
    exit 0
fi

send_json '{"error":"not found"}' "404 Not Found"
```

**Step 2: Verify syntax**

Run: `bash -n recipes/actions/search-server.sh && bash -n recipes/actions/lib/search-cgi.sh`
Expected: no errors

**Step 3: Commit**

```bash
git add recipes/actions/search-server.sh recipes/actions/lib/search-cgi.sh
git commit -m "feat(search): add drive-side REST API server"
```

---

### Task 7: Toolkit Integration

Wire search into entries.tab and serve-all.sh so it appears in the drive menu and starts automatically.

**Files:**
- Modify: `src/svalbard/toolkit_generator.py` (add search section to entries.tab)
- Modify: `recipes/actions/serve-all.sh` (start search server)
- Test: `tests/test_toolkit_generator.py` (add test)

**Step 1: Write failing test**

Add to `tests/test_toolkit_generator.py`:

```python
def test_entries_tab_includes_search_when_db_exists(tmp_path):
    """Search entry should appear when search.db exists."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()
    (tmp_path / "data").mkdir()
    (tmp_path / "data" / "search.db").touch()

    generate_toolkit(tmp_path, "default-32")

    entries = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[search]" in entries
    assert "search.sh" in entries
```

**Step 2: Run test to verify it fails**

Run: `cd . && uv run pytest tests/test_toolkit_generator.py::test_entries_tab_includes_search_when_db_exists -v`
Expected: FAIL — no `[search]` in entries

**Step 3: Add search section to toolkit_generator.py**

In `src/svalbard/toolkit_generator.py`, in `_build_entries()`, add after the `[browse]` section:

```python
    # ── Search ──────────────────────────────────────────────────────────
    search_db = drive_path / "data" / "search.db"
    if search_db.exists():
        lines.append("[search]")
        lines.append(
            f"Search all content"
            f"\t.svalbard/actions/search.sh"
        )
        lines.append("")
```

**Step 4: Add search server to serve-all.sh**

Append to `recipes/actions/serve-all.sh` before `wait_for_services`:

```bash
SQLITE_BIN="$(find_binary sqlite3 2>/dev/null || true)"
if [ -n "$SQLITE_BIN" ] && [ -f "$DRIVE_ROOT/data/search.db" ]; then
    port="$(find_free_port 8084)"
    export DB="$DRIVE_ROOT/data/search.db" SQLITE_BIN KIWIX_PORT="${kiwix_port:-8080}"
    "$DRIVE_ROOT/.svalbard/actions/search-server.sh" "$port" "$BIND" &
    SVALBARD_PIDS+=($!)
    ui_status "Search: http://$BIND:$port"
fi
```

Note: capture `kiwix_port` from the kiwix-serve block above it (rename the local `port` variable to `kiwix_port` after kiwix starts).

**Step 5: Run tests**

Run: `cd . && uv run pytest tests/test_toolkit_generator.py -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add src/svalbard/toolkit_generator.py recipes/actions/serve-all.sh tests/test_toolkit_generator.py
git commit -m "feat(search): integrate search into toolkit menu and serve-all"
```

---

### Task 8: Semantic Tier — Embedding Index and Rerank

Add the semantic tier: embed articles via llama-server, store vectors, rerank at query time.

**Files:**
- Modify: `src/svalbard/search_db.py` (add embeddings table + methods)
- Create: `src/svalbard/embedder.py` (llama-server client)
- Modify: `src/svalbard/indexer.py` (semantic tier flow)
- Test: `tests/test_embedder.py`
- Test: `tests/test_search_db.py` (add embedding tests)

**Step 1: Write failing tests for embeddings in search_db**

Add to `tests/test_search_db.py`:

```python
import struct

def _make_vector(dims: int, value: float = 0.5) -> bytes:
    """Create a float16 vector blob for testing."""
    import struct
    import array
    # Use float32 packed as bytes (simpler for test)
    return struct.pack(f"{dims}f", *([value] * dims))


def test_create_embeddings_table(tmp_path):
    db = SearchDB(tmp_path / "search.db")
    db.ensure_embeddings_table()
    tables = {r[0] for r in db.conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table'"
    ).fetchall()}
    assert "embeddings" in tables
    db.close()


def test_insert_and_query_embeddings(tmp_path):
    db = SearchDB(tmp_path / "search.db")
    db.ensure_embeddings_table()
    sid = db.upsert_source("w.zim", size_bytes=100, checksum="a")
    db.insert_articles_batch([
        (sid, "a/1", "Audi repair", "How to fix your Audi."),
        (sid, "a/2", "Cooking pasta", "Boil water and add pasta."),
    ])
    vec1 = _make_vector(4, 0.9)
    vec2 = _make_vector(4, 0.1)
    db.insert_embeddings_batch([(1, vec1), (2, vec2)])

    vecs = db.get_embeddings([1, 2])
    assert len(vecs) == 2
    assert 1 in vecs
    db.close()


def test_embed_resume_point(tmp_path):
    db = SearchDB(tmp_path / "search.db")
    db.ensure_embeddings_table()
    sid = db.upsert_source("w.zim", size_bytes=100, checksum="a")
    db.insert_articles_batch([
        (sid, "a/1", "One", "First"),
        (sid, "a/2", "Two", "Second"),
    ])
    vec = _make_vector(4)
    db.insert_embeddings_batch([(1, vec)])
    assert db.embed_resume_point() == 1
    db.close()
```

**Step 2: Implement embedding methods in SearchDB**

Add to `src/svalbard/search_db.py`:

```python
def ensure_embeddings_table(self):
    self.conn.execute("""
        CREATE TABLE IF NOT EXISTS embeddings (
            article_id INTEGER PRIMARY KEY REFERENCES articles(id),
            vector     BLOB NOT NULL
        )
    """)
    self.conn.commit()

def insert_embeddings_batch(self, embeddings: list[tuple]):
    """Insert (article_id, vector_blob) tuples."""
    self.conn.executemany(
        "INSERT OR REPLACE INTO embeddings(article_id, vector) VALUES (?, ?)",
        embeddings,
    )
    self.conn.commit()

def get_embeddings(self, article_ids: list[int]) -> dict[int, bytes]:
    """Fetch embedding vectors for given article IDs."""
    placeholders = ",".join("?" * len(article_ids))
    rows = self.conn.execute(
        f"SELECT article_id, vector FROM embeddings WHERE article_id IN ({placeholders})",
        article_ids,
    ).fetchall()
    return {r[0]: r[1] for r in rows}

def embed_resume_point(self) -> int:
    """Return the last embedded article_id, or 0."""
    row = self.conn.execute(
        "SELECT MAX(article_id) FROM embeddings"
    ).fetchone()
    return row[0] or 0 if row else 0

def unembedded_articles(self, after_id: int = 0, limit: int = 1000) -> list[tuple]:
    """Return (id, title, body) for articles not yet embedded."""
    return self.conn.execute(
        """SELECT a.id, a.title, a.body FROM articles a
           LEFT JOIN embeddings e ON e.article_id = a.id
           WHERE e.article_id IS NULL AND a.id > ?
           ORDER BY a.id LIMIT ?""",
        (after_id, limit),
    ).fetchall()
```

**Step 3: Write the embedder module**

```python
# src/svalbard/embedder.py
"""Embed text via llama-server HTTP API."""

import struct
import subprocess
import time
from pathlib import Path

import httpx


def start_embedding_server(
    model_path: Path, port: int = 8085, host: str = "127.0.0.1"
) -> subprocess.Popen:
    """Start llama-server in embedding mode. Returns the process."""
    proc = subprocess.Popen(
        [
            "llama-server",
            "-m", str(model_path),
            "--embeddings",
            "--port", str(port),
            "--host", host,
            "-ngl", "0",  # CPU only for portability
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    # Wait for server to be ready
    for _ in range(30):
        try:
            r = httpx.get(f"http://{host}:{port}/health", timeout=2)
            if r.status_code == 200:
                return proc
        except httpx.ConnectError:
            pass
        time.sleep(1)
    proc.kill()
    raise RuntimeError("llama-server failed to start within 30s")


def embed_batch(
    texts: list[str], port: int = 8085, host: str = "127.0.0.1"
) -> list[list[float]]:
    """Embed a batch of texts via llama-server /embedding endpoint."""
    r = httpx.post(
        f"http://{host}:{port}/embedding",
        json={"input": texts},
        timeout=120,
    )
    r.raise_for_status()
    data = r.json()
    # llama-server returns list of {"embedding": [...], "index": N}
    results = sorted(data, key=lambda x: x["index"])
    return [item["embedding"] for item in results]


def vectors_to_blob(vectors: list[list[float]]) -> list[bytes]:
    """Pack float vectors as float32 blobs."""
    blobs = []
    for vec in vectors:
        blobs.append(struct.pack(f"{len(vec)}f", *vec))
    return blobs
```

**Step 4: Write failing test for embedder**

```python
# tests/test_embedder.py
import struct

from svalbard.embedder import vectors_to_blob


def test_vectors_to_blob():
    vecs = [[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]]
    blobs = vectors_to_blob(vecs)
    assert len(blobs) == 2
    unpacked = struct.unpack("3f", blobs[0])
    assert unpacked == (1.0, 2.0, 3.0)
```

**Step 5: Run all tests**

Run: `cd . && uv run pytest tests/test_search_db.py tests/test_embedder.py -v`
Expected: all PASS

**Step 6: Commit**

```bash
git add src/svalbard/search_db.py src/svalbard/embedder.py tests/test_search_db.py tests/test_embedder.py
git commit -m "feat(search): add semantic tier — embeddings storage and llama-server client"
```

---

### Task 9: Add sqlite3 Binary Recipe

Add a recipe for bundling sqlite3 on the drive, following the existing binary recipe pattern.

**Files:**
- Create: `recipes/tools/sqlite3.yaml`

**Step 1: Check existing binary recipe for reference**

Look at an existing binary recipe (e.g. kiwix-serve or fzf) for the format.

**Step 2: Create sqlite3 recipe**

```yaml
# recipes/tools/sqlite3.yaml
id: sqlite3
type: binary
group: tools
description: SQLite CLI — required for cross-ZIM search
tags: [search, database]
size_gb: 0.002
platforms:
  darwin-arm64: https://www.sqlite.org/2025/sqlite-tools-osx-arm64-3490200.zip
  darwin-x86_64: https://www.sqlite.org/2025/sqlite-tools-osx-x64-3490200.zip
  linux-x86_64: https://www.sqlite.org/2025/sqlite-tools-linux-x64-3490200.zip
```

Note: the exact URLs need to be verified against sqlite.org/download.html at implementation time. The implementor should check the current latest version.

**Step 3: Commit**

```bash
git add recipes/tools/sqlite3.yaml
git commit -m "feat(search): add sqlite3 binary recipe for drive bundling"
```

---

### Task 10: Add Embedding Model Recipe

Add the nomic-embed-text-v1.5 GGUF recipe for the semantic tier.

**Files:**
- Create: `recipes/models/nomic-embed-text-v1.5.yaml`

**Step 1: Create recipe**

```yaml
# recipes/models/nomic-embed-text-v1.5.yaml
id: nomic-embed-text-v1.5
type: gguf
group: models
description: Nomic Embed Text v1.5 — small embedding model for semantic search
tags: [embedding, search]
size_gb: 0.14
url: https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q8_0.gguf
license:
  id: Apache-2.0
  url: https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF
```

**Step 2: Commit**

```bash
git add recipes/models/nomic-embed-text-v1.5.yaml
git commit -m "feat(search): add nomic-embed-text-v1.5 embedding model recipe"
```

---

## Execution Order Summary

| Task | What | Depends on |
|------|------|------------|
| 1 | SearchDB module (schema, queries) | — |
| 2 | ZIM text extractor | — |
| 3 | Indexer core (scan, estimate, build) | 1, 2 |
| 4 | CLI command `svalbard index` | 3 |
| 5 | Drive-side `search.sh` | — (shell only) |
| 6 | Drive-side `search-server.sh` | — (shell only) |
| 7 | Toolkit integration | 5, 6 |
| 8 | Semantic tier (embeddings) | 1, 3 |
| 9 | sqlite3 binary recipe | — |
| 10 | Embedding model recipe | — |

Tasks 1+2, 5+6, 9+10 can be done in parallel.

Tasks 5, 6, 9, 10 are independent of the Python tasks and can be dispatched to parallel agents.
