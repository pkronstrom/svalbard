from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path
from subprocess import CompletedProcess
import sys
from types import SimpleNamespace
from unittest.mock import patch

import pytest


def _load_wikipedia_builder_module():
    module_path = Path(__file__).resolve().parent.parent.parent / "recipes" / "builders" / "wikipedia-compact-zim.py"
    spec = spec_from_file_location("wikipedia_compact_zim", module_path)
    module = module_from_spec(spec)
    assert spec is not None
    assert spec.loader is not None
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def test_rewrite_zim_uses_unique_volume_and_cleans_up_on_failure(tmp_path):
    builder = _load_wikipedia_builder_module()
    source_path = tmp_path / "input" / "source.zim"
    source_path.parent.mkdir(parents=True)
    source_path.write_bytes(b"zim")
    output_path = tmp_path / "output" / "compact.zim"

    docker_calls = []

    def fake_docker_run(cmd, mounts=None, volumes=None, capture=False):
        docker_calls.append(
            {"cmd": cmd, "mounts": mounts, "volumes": volumes, "capture": capture}
        )
        return CompletedProcess(cmd, 1, stdout="", stderr="boom")

    with patch.object(builder.uuid, "uuid4", return_value=SimpleNamespace(hex="abc123def456")), \
         patch.object(builder, "_docker_run", side_effect=fake_docker_run), \
         patch.object(builder, "_volume_create") as volume_create, \
         patch.object(builder, "_volume_remove") as volume_remove:
        with pytest.raises(RuntimeError, match="zim-compact failed"):
            builder.rewrite_zim(source_path, output_path)

    volume_name = "svalbard-zim-work-abc123def456"
    volume_create.assert_called_once_with(volume_name)
    volume_remove.assert_called_once_with(volume_name)
    assert docker_calls[0]["volumes"] == {volume_name: "/work"}


def test_rewrite_zim_invokes_zimwriterfs_without_shell(tmp_path):
    builder = _load_wikipedia_builder_module()
    source_path = tmp_path / "input" / "source.zim"
    source_path.parent.mkdir(parents=True)
    source_path.write_bytes(b"zim")
    output_path = tmp_path / "output" / "compact.zim"

    docker_calls = []
    metadata = "\n".join(
        [
            "main_page=Main_Page",
            "language=eng",
            "title=Wiki's Compact",
            "description=Desc with spaces",
            "creator=Wikipedia Contributors",
        ]
    )

    def fake_docker_run(cmd, mounts=None, volumes=None, capture=False):
        docker_calls.append(
            {"cmd": cmd, "mounts": mounts, "volumes": volumes, "capture": capture}
        )
        if len(docker_calls) == 1:
            return CompletedProcess(cmd, 0, stdout=metadata, stderr="")
        return None

    with patch.object(builder.uuid, "uuid4", return_value=SimpleNamespace(hex="abc123def456")), \
         patch.object(builder, "_docker_run", side_effect=fake_docker_run), \
         patch.object(builder, "_volume_create"), \
         patch.object(builder, "_volume_remove"):
        builder.rewrite_zim(source_path, output_path)

    phase2_cmd = docker_calls[1]["cmd"]
    assert phase2_cmd[0] == "zimwriterfs"
    assert "sh" not in phase2_cmd
    assert "--welcome=Main_Page" in phase2_cmd
    assert "--title=Wiki's Compact" in phase2_cmd
    assert "--description=Desc with spaces" in phase2_cmd
    assert "--creator=Wikipedia Contributors" in phase2_cmd
    assert "--redirects=/work/redirects.tsv" in phase2_cmd
    assert phase2_cmd[-2] == "/work/extracted"
    assert phase2_cmd[-1] == "/output/compact.zim"
