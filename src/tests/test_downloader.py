from pathlib import Path

from rich.progress import Progress

from svalbard.downloader import DownloadResult, download_file, download_sources, find_downloader


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


def test_download_file_resumes_partial_without_checksum(tmp_path, monkeypatch):
    dest_dir = tmp_path / "downloads"
    dest_dir.mkdir()
    partial = dest_dir / "model.gguf"
    partial.write_bytes(b"partial")
    resumed = {}

    def fake_httpx(url, dest_path, progress, task_id):
        resumed["called"] = True
        assert dest_path == partial
        dest_path.write_bytes(b"complete")
        return dest_path

    monkeypatch.setattr("svalbard.downloader.download_file_httpx", fake_httpx)

    with Progress() as progress:
        task_id = progress.add_task("dl", filename="model.gguf", total=None)
        path, sha = download_file(
            "https://example.com/model.gguf",
            dest_dir,
            progress=progress,
            task_id=task_id,
        )

    assert resumed["called"] is True
    assert path == partial
    assert path.read_bytes() == b"complete"
    assert sha == ""


def test_download_sources_retries_partial_without_checksum(tmp_path, monkeypatch):
    dest_dir = tmp_path / "downloads"
    dest_dir.mkdir()
    partial = dest_dir / "model.gguf"
    partial.write_bytes(b"partial")

    def fake_download_file(url, dest_dir_arg, expected_sha256="", progress=None, task_id=None, use_cli=None):
        assert dest_dir_arg == dest_dir
        partial.write_bytes(b"complete")
        return partial, ""

    monkeypatch.setattr("svalbard.downloader.download_file", fake_download_file)

    results = download_sources([("gemma", "https://example.com/model.gguf", dest_dir)])

    assert len(results) == 1
    assert results[0].success is True
    assert results[0].filepath == partial
    assert partial.read_bytes() == b"complete"
