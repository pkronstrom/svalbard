from pathlib import Path
from primer.downloader import DownloadResult, find_downloader


def test_find_downloader():
    """Verify find_downloader returns one of the expected tools or None."""
    result = find_downloader()
    assert result in ("aria2c", "wget", "curl", None)


def test_download_result():
    """Create a DownloadResult and verify its fields."""
    r = DownloadResult(source_id="test-source", success=True, filepath=Path("/tmp/file.txt"))
    assert r.source_id == "test-source"
    assert r.success is True
    assert r.filepath == Path("/tmp/file.txt")
    assert r.error == ""


def test_download_result_failure():
    """Create a failed DownloadResult and verify fields."""
    r = DownloadResult(source_id="bad-source", success=False, error="connection refused")
    assert r.source_id == "bad-source"
    assert r.success is False
    assert r.filepath is None
    assert r.error == "connection refused"
