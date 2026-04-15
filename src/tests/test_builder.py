from pathlib import Path
from unittest.mock import patch, MagicMock

from svalbard.builder import (
    BuildResult,
    HANDLERS,
    _find_tool,
    check_tools,
    run_build,
)
from svalbard.models import Source


def _make_source(id: str, family: str, **build_extra) -> Source:
    build = {"family": family, **build_extra}
    return Source(id=id, type="pmtiles", display_group="maps", strategy="build", build=build)


def test_handler_registry_has_all_families():
    expected = {"vector-static", "vector-service", "osm-extract", "reference-static", "app-bundle", "raster-tms", "mml-topo"}
    assert set(HANDLERS.keys()) == expected


def test_check_tools_returns_empty_when_all_present():
    with patch("shutil.which", return_value="/usr/bin/tool"), \
         patch("svalbard.builder.has_docker", return_value=False):
        missing = check_tools(["reference-static", "app-bundle"])
    assert missing == []


def test_check_tools_detects_missing():
    def fake_which(name):
        return None if name == "tippecanoe" else f"/usr/bin/{name}"

    with patch("shutil.which", side_effect=fake_which), \
         patch("svalbard.builder.has_docker", return_value=False):
        missing = check_tools(["vector-static"])
    assert "tippecanoe" in missing


def test_check_tools_accepts_docker_fallback():
    """When Docker is available, tools with Docker images should not be missing."""
    with patch("shutil.which", return_value=None), \
         patch("svalbard.builder.has_docker", return_value=True), \
         patch("svalbard.builder.ensure_tools_image", return_value=True):
        missing = check_tools(["vector-static"])
    # ogr2ogr and tippecanoe have Docker images, so neither should be missing
    assert "ogr2ogr" not in missing
    assert "tippecanoe" not in missing


def test_check_tools_finds_binary_on_drive(tmp_path):
    """Binaries on the drive should satisfy tool requirements."""
    import platform as _platform
    os_name = "macos" if _platform.system() == "Darwin" else "linux"
    arch = "arm64" if _platform.machine() in ("aarch64", "arm64") else "x86_64"
    bin_dir = tmp_path / "bin" / f"{os_name}-{arch}"
    bin_dir.mkdir(parents=True)
    pmtiles_bin = bin_dir / "pmtiles"
    pmtiles_bin.write_bytes(b"#!/bin/sh\n")
    pmtiles_bin.chmod(0o755)

    with patch("shutil.which", return_value=None), \
         patch("svalbard.builder.has_docker", return_value=False):
        missing = check_tools(["osm-extract"], drive_path=tmp_path)
    assert "pmtiles" not in missing


def test_find_tool_prefers_drive_over_path(tmp_path):
    """_find_tool should return the drive binary even when PATH has the tool."""
    import platform as _platform
    os_name = "macos" if _platform.system() == "Darwin" else "linux"
    arch = "arm64" if _platform.machine() in ("aarch64", "arm64") else "x86_64"
    bin_dir = tmp_path / "bin" / f"{os_name}-{arch}"
    bin_dir.mkdir(parents=True)
    pmtiles_bin = bin_dir / "pmtiles"
    pmtiles_bin.write_bytes(b"#!/bin/sh\n")
    pmtiles_bin.chmod(0o755)

    with patch("shutil.which", return_value="/usr/local/bin/pmtiles"):
        result = _find_tool("pmtiles", tmp_path)
    assert result == str(pmtiles_bin)


def test_run_build_unknown_family(tmp_path):
    source = _make_source("test", "nonexistent-family")
    result = run_build(source, tmp_path, cache_dir=tmp_path / "cache")
    assert not result.success
    assert "Unknown build family" in result.error


def test_build_reference_static_creates_sqlite(tmp_path):
    source = Source(
        id="test-ref",
        type="sqlite",
        display_group="regional",
        strategy="build",
        build={
            "family": "reference-static",
            "source_url": "",
            "tables": [
                {"name": "items", "fts": True, "fts_columns": ["data"]},
            ],
        },
    )
    result = run_build(source, tmp_path, cache_dir=tmp_path / "cache")
    assert result.success
    assert result.artifact is not None
    assert result.artifact.exists()
    assert result.artifact.name == "test-ref.sqlite"

    # Verify schema
    import sqlite3
    conn = sqlite3.connect(str(result.artifact))
    tables = [r[0] for r in conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table'"
    ).fetchall()]
    conn.close()
    assert "items" in tables
    assert "items_fts" in tables
    assert "_meta" in tables


def test_build_reference_static_skips_if_exists(tmp_path):
    source = Source(
        id="test-ref",
        type="sqlite",
        display_group="regional",
        strategy="build",
        build={"family": "reference-static", "tables": []},
    )
    # Pre-create the artifact
    data_dir = tmp_path / "data"
    data_dir.mkdir()
    (data_dir / "test-ref.sqlite").write_bytes(b"existing")

    result = run_build(source, tmp_path, cache_dir=tmp_path / "cache")
    assert result.success
    # File should not have been overwritten
    assert (data_dir / "test-ref.sqlite").read_bytes() == b"existing"


def test_build_app_bundle_with_assets(tmp_path):
    source = Source(
        id="test-app",
        type="app",
        display_group="tools",
        strategy="build",
        build={
            "family": "app-bundle",
            "assets": [
                {"url": "https://example.com/lib.js", "dest": "vendor/lib.js"},
            ],
        },
    )
    # Mock the download
    with patch("svalbard.builder._download_to") as mock_dl:
        def fake_download(url, dest, **kwargs):
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_text("// js content")
            return dest
        mock_dl.side_effect = fake_download

        result = run_build(source, tmp_path, cache_dir=tmp_path / "cache")

    assert result.success
    assert (tmp_path / "apps" / "test-app" / "vendor" / "lib.js").exists()


def test_build_app_bundle_skips_if_populated(tmp_path):
    source = Source(
        id="test-app",
        type="app",
        display_group="tools",
        strategy="build",
        build={"family": "app-bundle", "source_url": "https://example.com/app.zip"},
    )
    # Pre-populate the directory
    app_dir = tmp_path / "apps" / "test-app"
    app_dir.mkdir(parents=True)
    (app_dir / "index.html").write_text("<html>existing</html>")

    result = run_build(source, tmp_path, cache_dir=tmp_path / "cache")
    assert result.success
    # Should not have attempted download
    assert (app_dir / "index.html").read_text() == "<html>existing</html>"


def test_build_result_dataclass():
    r = BuildResult("test", True, artifact=Path("/tmp/test.pmtiles"))
    assert r.source_id == "test"
    assert r.success
    assert r.error == ""
