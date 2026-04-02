from pathlib import Path

import pytest

from svalbard.local_sources import active_sources_for_manifest, load_local_sources, workspace_root
from svalbard.manifest import Manifest
from svalbard.models import Preset, Source


def test_workspace_root_is_repo_root():
    root = workspace_root()
    assert (root / "pyproject.toml").exists()


def test_load_local_sources_discovers_sidecars_and_derives_size_gb(tmp_path):
    local_dir = tmp_path / "local" / "recipes"
    generated_dir = tmp_path / "library"
    local_dir.mkdir(parents=True)
    generated_dir.mkdir()
    artifact = generated_dir / "example.zim"
    artifact.write_bytes(b"x" * 100)
    (local_dir / "example.yaml").write_text(
        """id: local:example
type: zim
group: practical
strategy: local
path: library/example.zim
size_bytes: 100
"""
    )

    sources = load_local_sources(tmp_path)
    assert [s.id for s in sources] == ["local:example"]
    assert sources[0].path == "library/example.zim"
    assert sources[0].size_gb > 0


def test_load_local_sources_rejects_builtin_id_collision(tmp_path):
    local_dir = tmp_path / "local" / "recipes"
    generated_dir = tmp_path / "library"
    local_dir.mkdir(parents=True)
    generated_dir.mkdir()
    (generated_dir / "example.zim").write_bytes(b"x")
    (local_dir / "example.yaml").write_text(
        """id: wikipedia-en-nopic
type: zim
group: practical
strategy: local
path: library/example.zim
size_bytes: 1
"""
    )

    with pytest.raises(ValueError, match="collides"):
        load_local_sources(tmp_path)


def test_active_sources_for_manifest_merges_preset_and_selected_local_sources(tmp_path):
    local_dir = tmp_path / "local" / "recipes"
    generated_dir = tmp_path / "library"
    local_dir.mkdir(parents=True)
    generated_dir.mkdir()
    (generated_dir / "example.zim").write_bytes(b"x")
    (local_dir / "example.yaml").write_text(
        """id: local:example
type: zim
group: practical
strategy: local
path: library/example.zim
size_bytes: 1
"""
    )

    preset = Preset(
        name="default-32",
        description="test",
        target_size_gb=32,
        region="default",
        sources=[Source(id="wiki", type="zim", group="reference")],
    )
    manifest = Manifest(
        preset="default-32",
        region="default",
        target_path="/tmp/drive",
        workspace_root=str(tmp_path),
        local_sources=["local:example"],
    )

    sources = active_sources_for_manifest(manifest, preset)
    assert [source.id for source in sources] == ["wiki", "local:example"]


def test_active_sources_for_manifest_keeps_missing_selected_local_source_visible(tmp_path):
    preset = Preset(
        name="default-32",
        description="test",
        target_size_gb=32,
        region="default",
        sources=[Source(id="wiki", type="zim", group="reference")],
    )
    manifest = Manifest(
        preset="default-32",
        region="default",
        target_path="/tmp/drive",
        workspace_root=str(tmp_path),
        local_sources=["local:missing"],
    )

    sources = active_sources_for_manifest(manifest, preset)
    ids = [source.id for source in sources]
    assert ids == ["wiki", "local:missing"]
    missing = sources[-1]
    assert missing.strategy == "local"
    assert missing.description.startswith("Missing local source")
