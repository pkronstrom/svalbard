"""Tests for svalbard.indexer — cross-ZIM indexing engine."""

from pathlib import Path
from unittest.mock import patch

import pytest

from svalbard.indexer import (
    IndexPlan,
    estimate_index,
    run_index,
    scan_zim_files,
)
from svalbard.search_db import SearchDB


# ── Fixtures ─────────────────────────────────────────────────────────


@pytest.fixture
def db(tmp_path: Path) -> SearchDB:
    """Return an open SearchDB backed by a temp file."""
    return SearchDB(tmp_path / "search.db")


@pytest.fixture
def drive(tmp_path: Path) -> Path:
    """Create a mock drive layout with a zim/ directory and dummy ZIM files."""
    zim_dir = tmp_path / "drive" / "zim"
    zim_dir.mkdir(parents=True)
    (zim_dir / "wiki.zim").write_bytes(b"\x00" * 128)
    (zim_dir / "books.zim").write_bytes(b"\x00" * 256)
    return tmp_path / "drive"


def _fake_extract_articles(zim_path, max_body_chars=500):
    """Yield a handful of fake articles for any ZIM path."""
    name = Path(zim_path).stem
    for i in range(3):
        yield f"/{name}/art{i}", f"{name} Article {i}", f"Body text {i} for {name}"


def _fake_zim_article_count(zim_path):
    """Return a fixed article count for any ZIM path."""
    return 3


# ── scan_zim_files ───────────────────────────────────────────────────


class TestScanZimFiles:
    def test_finds_zim_files(self, drive: Path):
        """scan_zim_files should find .zim files in drive/zim/."""
        result = scan_zim_files(drive)
        names = [p.name for p in result]
        assert "wiki.zim" in names
        assert "books.zim" in names
        assert len(result) == 2

    def test_ignores_non_zim(self, drive: Path):
        """scan_zim_files should ignore non-.zim files."""
        (drive / "zim" / "readme.txt").write_text("hello")
        (drive / "zim" / "data.zimaa").write_bytes(b"\x00")
        result = scan_zim_files(drive)
        names = [p.name for p in result]
        assert "readme.txt" not in names
        assert "data.zimaa" not in names
        assert len(result) == 2

    def test_returns_empty_for_missing_dir(self, tmp_path: Path):
        """scan_zim_files returns empty list when zim/ does not exist."""
        assert scan_zim_files(tmp_path / "nonexistent") == []

    def test_returns_sorted(self, drive: Path):
        """scan_zim_files returns files sorted by name."""
        result = scan_zim_files(drive)
        names = [p.name for p in result]
        assert names == sorted(names)


# ── estimate_index ───────────────────────────────────────────────────


class TestEstimateIndex:
    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    def test_all_new_when_db_empty(self, _mock_count, drive: Path, db: SearchDB):
        """All ZIMs are new when the database is empty."""
        plan = estimate_index(drive, db)
        assert plan.total_zims == 2
        assert plan.new_zims == 2
        assert plan.already_indexed == 0
        assert plan.changed_zims == 0
        assert len(plan.files_to_index) == 2

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_skips_already_indexed(
        self, _mock_extract, _mock_count, drive: Path, db: SearchDB
    ):
        """Files indexed with matching checksum are skipped."""
        # First run: index everything
        run_index(drive, db)

        # Second estimate: everything should be already_indexed
        plan = estimate_index(drive, db)
        assert plan.already_indexed == 2
        assert plan.new_zims == 0
        assert plan.changed_zims == 0
        assert len(plan.files_to_index) == 0

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_detects_changed_file(
        self, _mock_extract, _mock_count, drive: Path, db: SearchDB
    ):
        """A file with a different size/mtime is detected as changed."""
        run_index(drive, db)

        # Modify one file to change its checksum
        wiki_path = drive / "zim" / "wiki.zim"
        wiki_path.write_bytes(b"\x00" * 999)

        plan = estimate_index(drive, db)
        assert plan.changed_zims == 1
        assert plan.already_indexed == 1
        assert len(plan.files_to_index) == 1

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    def test_detects_missing_files(self, _mock_count, tmp_path: Path, db: SearchDB):
        """Files in DB but not on disk are counted as missing."""
        # Add a source that doesn't exist on disk
        db.upsert_source("gone.zim")

        drive = tmp_path / "drive"
        (drive / "zim").mkdir(parents=True)
        (drive / "zim" / "present.zim").write_bytes(b"\x00" * 64)

        plan = estimate_index(drive, db)
        assert plan.missing_zims == 1
        assert plan.new_zims == 1

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    def test_estimated_articles(self, _mock_count, drive: Path, db: SearchDB):
        """estimated_articles is populated from zim_article_count."""
        plan = estimate_index(drive, db)
        # 2 files * 3 articles each
        assert plan.estimated_articles == 6
        assert plan.estimated_db_bytes > 0


# ── run_index ────────────────────────────────────────────────────────


class TestRunIndex:
    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_populates_db(self, _mock_extract, _mock_count, drive: Path, db: SearchDB):
        """run_index should insert sources and articles into the DB."""
        plan = run_index(drive, db)

        assert db.source_count() == 2
        # 2 files * 3 articles each
        assert db.article_count() == 6
        assert plan.new_zims == 2

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_incremental_skips_unchanged(
        self, mock_extract, _mock_count, drive: Path, db: SearchDB
    ):
        """A second run_index with unchanged files should not re-extract."""
        run_index(drive, db)
        mock_extract.reset_mock()

        plan = run_index(drive, db)

        # Nothing to index on second run
        assert plan.files_to_index == []
        mock_extract.assert_not_called()
        # DB unchanged
        assert db.article_count() == 6

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_sets_meta(self, _mock_extract, _mock_count, drive: Path, db: SearchDB):
        """run_index should set tier and indexed_at in meta."""
        run_index(drive, db, strategy="fast")

        assert db.get_meta("tier") == "fast"
        assert db.get_meta("indexed_at") is not None

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_reindexes_changed_file(
        self, _mock_extract, _mock_count, drive: Path, db: SearchDB
    ):
        """Changed files should be re-indexed with fresh articles."""
        run_index(drive, db)
        assert db.article_count() == 6

        # Modify wiki.zim to trigger re-index
        wiki_path = drive / "zim" / "wiki.zim"
        wiki_path.write_bytes(b"\x00" * 999)

        run_index(drive, db)
        # Should still be 6 articles (3 old removed + 3 re-inserted for wiki,
        # plus 3 unchanged for books)
        assert db.article_count() == 6

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_on_progress_called(
        self, _mock_extract, _mock_count, drive: Path, db: SearchDB
    ):
        """on_progress callback is invoked during indexing."""
        calls = []
        run_index(drive, db, on_progress=lambda f, n, t: calls.append((f, n, t)))
        assert len(calls) >= 2  # at least one call per file

    @patch("svalbard.indexer.zim_article_count", side_effect=_fake_zim_article_count)
    @patch("svalbard.indexer.extract_articles", side_effect=_fake_extract_articles)
    def test_returns_plan(
        self, _mock_extract, _mock_count, drive: Path, db: SearchDB
    ):
        """run_index returns the IndexPlan that was executed."""
        plan = run_index(drive, db)
        assert isinstance(plan, IndexPlan)
        assert plan.total_zims == 2
