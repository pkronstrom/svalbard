from pathlib import Path
from unittest.mock import patch

from svalbard.commands import expand_source_downloads, init_drive, show_status, sync_drive
from svalbard.manifest import Manifest, ManifestEntry
from svalbard.presets import load_preset


def test_init_drive_creates_files(tmp_path):
    """init_drive should create manifest.yaml, serve.sh, and README.md."""
    init_drive(str(tmp_path), "finland-128")
    assert (tmp_path / "manifest.yaml").exists()
    assert (tmp_path / "serve.sh").exists()
    assert (tmp_path / "README.md").exists()

    manifest = Manifest.load(tmp_path / "manifest.yaml")
    assert manifest.preset == "finland-128"
    assert manifest.region == "finland"

def test_init_drive_records_canonical_preset_without_enabled_groups(tmp_path):
    """init_drive should use the selected preset directly with no option flags."""
    init_drive(str(tmp_path), "finland-128")
    manifest = Manifest.load(tmp_path / "manifest.yaml")
    assert manifest.preset == "finland-128"
    assert "enabled_groups" not in (tmp_path / "manifest.yaml").read_text()


def test_sync_drive_no_manifest(tmp_path, capsys):
    """sync_drive should print error when no manifest exists."""
    sync_drive(str(tmp_path))
    # Should not crash — just prints an error


def test_sync_drive_skips_downloaded(tmp_path):
    """sync_drive should skip sources already in manifest with files on disk."""
    init_drive(str(tmp_path), "finland-128")
    manifest = Manifest.load(tmp_path / "manifest.yaml")

    # Fake a downloaded entry
    from svalbard.manifest import ManifestEntry

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
    with patch("svalbard.commands.resolve_url", return_value="https://example.com/test.zim"), \
         patch("svalbard.commands.download_sources", return_value=[]):
        sync_drive(str(tmp_path))

    # Should still have the entry
    reloaded = Manifest.load(tmp_path / "manifest.yaml")
    assert reloaded.entry_by_id("wikipedia-en-nopic") is not None


def test_show_status_no_manifest(tmp_path):
    """show_status should not crash when no manifest exists."""
    show_status(str(tmp_path))


def test_show_status_with_manifest(tmp_path):
    """show_status should work with an initialized drive."""
    init_drive(str(tmp_path), "finland-128")
    show_status(str(tmp_path))


def test_expand_source_downloads_creates_one_job_per_platform(tmp_path):
    preset = load_preset("finland-128")
    source = next(source for source in preset.sources if source.id == "kiwix-serve")
    jobs = expand_source_downloads(source, tmp_path)

    assert [job.platform for job in jobs] == [
        "linux-arm64",
        "linux-x86_64",
        "macos-arm64",
        "macos-x86_64",
    ]
    assert all(job.dest_dir.parent == tmp_path / "bin" for job in jobs)


def test_show_status_handles_platformed_binary_sources(tmp_path):
    init_drive(str(tmp_path), "finland-128")
    manifest = Manifest.load(tmp_path / "manifest.yaml")
    manifest.entries.append(ManifestEntry(
        id="kiwix-serve",
        type="binary",
        platform="linux-x86_64",
        filename="kiwix-tools_linux-x86_64.tar.gz",
        size_bytes=123,
    ))
    manifest.save(tmp_path / "manifest.yaml")

    show_status(str(tmp_path))
