from pathlib import Path
from unittest.mock import patch

from primer.commands import init_drive, show_status, sync_drive
from primer.manifest import Manifest


def test_init_drive_creates_files(tmp_path):
    """init_drive should create manifest.yaml, serve.sh, and README.md."""
    init_drive(str(tmp_path), "nordic-128", {"maps"})
    assert (tmp_path / "manifest.yaml").exists()
    assert (tmp_path / "serve.sh").exists()
    assert (tmp_path / "README.md").exists()

    manifest = Manifest.load(tmp_path / "manifest.yaml")
    assert manifest.preset == "nordic-128"
    assert manifest.region == "nordic"
    assert "maps" in manifest.enabled_groups


def test_init_drive_default_groups(tmp_path):
    """init_drive with no enabled_groups defaults to maps."""
    init_drive(str(tmp_path), "nordic-128")
    manifest = Manifest.load(tmp_path / "manifest.yaml")
    assert "maps" in manifest.enabled_groups


def test_sync_drive_no_manifest(tmp_path, capsys):
    """sync_drive should print error when no manifest exists."""
    sync_drive(str(tmp_path))
    # Should not crash — just prints an error


def test_sync_drive_skips_downloaded(tmp_path):
    """sync_drive should skip sources already in manifest with files on disk."""
    init_drive(str(tmp_path), "nordic-128", {"maps"})
    manifest = Manifest.load(tmp_path / "manifest.yaml")

    # Fake a downloaded entry
    from primer.manifest import ManifestEntry

    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    fake_file = zim_dir / "test.zim"
    fake_file.write_bytes(b"fake")

    manifest.entries.append(ManifestEntry(
        id="wikipedia-en-nopic",
        type="zim",
        filename="test.zim",
        size_bytes=4,
        tags=["general-reference"],
        depth="comprehensive",
        downloaded="2026-01-01T00:00:00",
        url="https://example.com/test.zim",
    ))
    manifest.save(tmp_path / "manifest.yaml")

    # Sync with mocked resolver/downloader to avoid network calls
    with patch("primer.commands.resolve_url", return_value="https://example.com/test.zim"), \
         patch("primer.commands.download_sources", return_value=[]):
        sync_drive(str(tmp_path))

    # Should still have the entry
    reloaded = Manifest.load(tmp_path / "manifest.yaml")
    assert reloaded.entry_by_id("wikipedia-en-nopic") is not None


def test_show_status_no_manifest(tmp_path):
    """show_status should not crash when no manifest exists."""
    show_status(str(tmp_path))


def test_show_status_with_manifest(tmp_path):
    """show_status should work with an initialized drive."""
    init_drive(str(tmp_path), "nordic-128", {"maps"})
    show_status(str(tmp_path))
