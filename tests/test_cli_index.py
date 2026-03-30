"""Tests for the ``svalbard index`` CLI command."""

from click.testing import CliRunner
from unittest.mock import patch

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
    assert result.exit_code == 0, f"Output: {result.output}"
    assert (tmp_path / "data" / "search.db").exists()


def test_index_command_no_zims(tmp_path):
    """Should handle drive with no ZIM files gracefully."""
    runner = CliRunner()
    result = runner.invoke(main, ["index", str(tmp_path), "--yes"])
    assert result.exit_code == 0
    assert "No ZIM files" in result.output
