from pathlib import Path
from unittest.mock import patch

import pytest

from svalbard.commands import expand_source_downloads, init_drive, show_status, sync_drive
from svalbard.manifest import Manifest, ManifestEntry
from svalbard.models import Preset
from svalbard.presets import load_preset


def test_init_drive_creates_files(tmp_path):
    """init_drive should create manifest.yaml, run.sh, and README.md."""
    init_drive(str(tmp_path), "finland-128")
    assert (tmp_path / "manifest.yaml").exists()
    assert (tmp_path / "run.sh").exists()
    assert (tmp_path / "README.md").exists()
    assert (tmp_path / ".svalbard" / "entries.tab").exists()

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
    preset = load_preset("finland-128")
    source = next(source for source in preset.sources if source.type == "zim")

    # Fake a downloaded entry
    from svalbard.manifest import ManifestEntry

    zim_dir = tmp_path / "zim"
    zim_dir.mkdir()
    fake_file = zim_dir / "test.zim"
    fake_file.write_bytes(b"fake")

    manifest.entries.append(ManifestEntry(
        id=source.id,
        type="zim",
        filename="test.zim",
        size_bytes=4,
        tags=["general-reference"],
        depth="comprehensive",
        downloaded="2026-01-01T00:00:00",
        url="https://example.com/test.zim",
    ))
    manifest.save(tmp_path / "manifest.yaml")

    minimal_preset = Preset(
        name="finland-128",
        description="test",
        target_size_gb=128,
        region="finland",
        sources=[source],
    )

    # Sync with mocked preset/downloader to avoid network calls
    with patch("svalbard.commands.load_preset", return_value=minimal_preset), \
         patch("svalbard.commands.resolve_url", return_value="https://example.com/test.zim"), \
         patch("svalbard.commands.fetch_sha256_sidecar", return_value=""), \
         patch("svalbard.commands.download_sources", return_value=[]):
        sync_drive(str(tmp_path))

    # Should still have the entry
    reloaded = Manifest.load(tmp_path / "manifest.yaml")
    assert reloaded.entry_by_id(source.id) is not None


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


def test_expand_source_downloads_filters_exact_platform(tmp_path):
    preset = load_preset("finland-128")
    source = next(source for source in preset.sources if source.id == "kiwix-serve")

    jobs = expand_source_downloads(source, tmp_path, platform_filter="macos-arm64")

    assert [job.platform for job in jobs] == ["macos-arm64"]


def test_expand_source_downloads_filters_arch_family(tmp_path):
    preset = load_preset("finland-128")
    source = next(source for source in preset.sources if source.id == "kiwix-serve")

    jobs = expand_source_downloads(source, tmp_path, platform_filter="arm64")

    assert [job.platform for job in jobs] == ["linux-arm64", "macos-arm64"]


def test_expand_source_downloads_resolves_host_platform_alias(tmp_path, monkeypatch):
    preset = load_preset("finland-128")
    source = next(source for source in preset.sources if source.id == "kiwix-serve")
    monkeypatch.setattr("svalbard.commands._detect_host_platform", lambda: "macos-arm64")

    jobs = expand_source_downloads(source, tmp_path, platform_filter="host")

    assert [job.platform for job in jobs] == ["macos-arm64"]


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


def test_add_local_file_writes_sidecar(tmp_path):
    from svalbard.commands import add_local_source

    artifact = tmp_path / "manual.zim"
    artifact.write_bytes(b"data")

    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")
    sidecar = tmp_path / "local" / "recipes" / "manual.yaml"

    assert source_id == "local:manual"
    assert sidecar.exists()
    text = sidecar.read_text()
    assert "id: local:manual" in text
    assert "path:" in text


def test_sync_copies_selected_local_source(tmp_path):
    generated = tmp_path / "library"
    local = tmp_path / "local" / "recipes"
    drive = tmp_path / "drive"
    generated.mkdir()
    local.mkdir(parents=True)
    drive.mkdir()

    (generated / "example.zim").write_bytes(b"data")
    (local / "example.yaml").write_text(
        """id: local:example
type: zim
display_group: practical
strategy: local
path: library/example.zim
size_bytes: 4
"""
    )

    manifest = Manifest(
        preset="default-32",
        region="default",
        target_path=str(drive),
        workspace_root=str(tmp_path),
        local_sources=["local:example"],
    )
    manifest.save(drive / "manifest.yaml")

    empty_preset = Preset(
        name="default-32",
        description="test",
        target_size_gb=32,
        region="default",
        sources=[],
    )
    with patch("svalbard.commands.load_preset", return_value=empty_preset):
        sync_drive(str(drive))

    assert (drive / "zim" / "local-example.zim").exists()
    reloaded = Manifest.load(drive / "manifest.yaml")
    entry = reloaded.entry_by_id("local:example")
    assert entry is not None
    assert entry.filename == "local-example.zim"
    assert entry.relative_path == "zim/local-example.zim"
    assert entry.source_strategy == "local"


def test_add_local_directory_rejects_nested_symlinks(tmp_path):
    from svalbard.commands import add_local_source

    root = tmp_path / "appdir"
    root.mkdir()
    (root / "real.txt").write_text("data")
    (root / "link.txt").symlink_to(root / "real.txt")

    with pytest.raises(ValueError, match="nested symlinks"):
        add_local_source(root, workspace_root=tmp_path, source_type="app")


def test_expand_toolchain_source_with_platforms(tmp_path):
    """Toolchain sources with platforms go to tools/platformio/packages/{platform}/."""
    from svalbard.models import Source
    from svalbard.commands import expand_source_downloads

    source = Source(
        id="toolchain-xtensa-esp-elf",
        type="toolchain",
        platforms={
            "linux-x86_64": "https://example.com/linux.tar.gz",
            "macos-arm64": "https://example.com/macos.tar.gz",
        },
    )
    jobs = expand_source_downloads(source, tmp_path)
    assert len(jobs) == 2
    assert jobs[0].dest_dir == tmp_path / "tools" / "platformio" / "packages" / "linux-x86_64"
    assert jobs[1].dest_dir == tmp_path / "tools" / "platformio" / "packages" / "macos-arm64"


def test_expand_toolchain_source_no_platforms(tmp_path):
    """Cross-platform toolchain sources go to tools/platformio/packages/."""
    from svalbard.models import Source
    from svalbard.commands import expand_source_downloads

    source = Source(
        id="framework-espidf",
        type="toolchain",
        url="https://example.com/espidf.tar.gz",
    )
    jobs = expand_source_downloads(source, tmp_path)
    assert len(jobs) == 1
    assert jobs[0].dest_dir == tmp_path / "tools" / "platformio" / "packages"


def test_expand_binary_source_still_goes_to_bin(tmp_path):
    """Regular binary sources still go to bin/{platform}/ (regression check)."""
    from svalbard.models import Source
    from svalbard.commands import expand_source_downloads

    source = Source(
        id="kiwix-serve",
        type="binary",
        platforms={
            "linux-x86_64": "https://example.com/linux.tar.gz",
            "macos-arm64": "https://example.com/macos.tar.gz",
        },
    )
    jobs = expand_source_downloads(source, tmp_path)
    assert len(jobs) == 2
    assert jobs[0].dest_dir == tmp_path / "bin" / "linux-x86_64"
    assert jobs[1].dest_dir == tmp_path / "bin" / "macos-arm64"


def test_run_import_routes_youtube_urls_to_media_backend(tmp_path):
    from svalbard.importer import run_import

    artifact = tmp_path / "library" / "youtube-video-abc123.zim"
    artifact.parent.mkdir(parents=True, exist_ok=True)
    artifact.write_bytes(b"data")

    with patch("svalbard.importer.run_media_ingest", return_value=artifact) as media_mock:
        source_id = run_import("https://www.youtube.com/watch?v=abc123", workspace_root=tmp_path)

    assert source_id == "local:youtube-video-abc123"
    media_mock.assert_called_once()


def test_run_import_uses_youtube_playlist_id_for_default_slug(tmp_path):
    from svalbard.importer import run_import

    artifact = tmp_path / "library" / "youtube-playlist-pl987.zim"
    artifact.parent.mkdir(parents=True, exist_ok=True)
    artifact.write_bytes(b"data")

    with patch("svalbard.importer.run_media_ingest", return_value=artifact):
        source_id = run_import("https://www.youtube.com/playlist?list=PL987", workspace_root=tmp_path)

    assert source_id == "local:youtube-playlist-pl987"


def test_run_import_writes_media_provenance(tmp_path):
    from svalbard.importer import run_import

    artifact = tmp_path / "library" / "lecture.zim"
    artifact.parent.mkdir(parents=True, exist_ok=True)
    artifact.write_bytes(b"data")

    with patch("svalbard.importer.run_media_ingest", return_value=artifact):
        run_import("https://areena.yle.fi/1-12345", workspace_root=tmp_path, audio_only=True)

    metadata = (tmp_path / "library" / "lecture.source.yaml").read_text()
    assert "kind: media" in metadata
    assert "audio_only: true" in metadata
