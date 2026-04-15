from click.testing import CliRunner
from unittest.mock import patch


def test_preset_list_shows_builtin_and_workspace_presets(tmp_path):
    from svalbard.cli import main

    presets_dir = tmp_path / "local" / "presets"
    presets_dir.mkdir(parents=True)
    (presets_dir / "my-pack.yaml").write_text(
        "name: my-pack\ndescription: test\ntarget_size_gb: 1\nregion: default\nsources: []\n"
    )

    result = CliRunner().invoke(main, ["preset", "list", "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert "my-pack" in result.output
    assert "default-32" in result.output


def test_preset_copy_writes_workspace_owned_preset(tmp_path):
    from svalbard.cli import main

    result = CliRunner().invoke(main, ["preset", "copy", "default-32", "my-pack", "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert (tmp_path / "local" / "presets" / "my-pack.yaml").exists()


def test_sync_command_forwards_platform_filter(tmp_path):
    from svalbard.cli import main
    from unittest.mock import patch

    with patch("svalbard.commands.sync_drive") as sync_drive_mock:
        result = CliRunner().invoke(main, ["sync", str(tmp_path), "--platform", "host"])

    assert result.exit_code == 0
    sync_drive_mock.assert_called_once_with(str(tmp_path), update=False, force=False, parallel=5, platform_filter="host")


def test_refresh_command_removes_and_resyncs_single_source(tmp_path):
    from svalbard.cli import main
    from svalbard.manifest import Manifest, ManifestEntry

    drive = tmp_path / "drive"
    tool_dir = drive / "bin" / "macos-arm64" / "llama-server"
    tool_dir.mkdir(parents=True)
    (tool_dir / "llama-server").write_bytes(b"binary")
    (tool_dir / "llama-b8799-bin-macos-arm64.tar.gz").write_bytes(b"archive")

    Manifest(
        preset="test-ai-small",
        region="default",
        target_path=str(drive),
        entries=[
            ManifestEntry(
                id="llama-server",
                type="binary",
                platform="macos-arm64",
                filename="llama-b8799-bin-macos-arm64.tar.gz",
                size_bytes=10,
                relative_path="bin/macos-arm64/llama-server/llama-b8799-bin-macos-arm64.tar.gz",
            ),
            ManifestEntry(
                id="gemma-4-e2b-it",
                type="gguf",
                filename="gemma-4-e2b-it-edited-q4_0.gguf",
                size_bytes=10,
                relative_path="models/gemma-4-e2b-it-edited-q4_0.gguf",
            ),
        ],
    ).save(drive / "manifest.yaml")

    with patch("svalbard.cli.sync_drive") as sync_drive_mock:
        result = CliRunner().invoke(main, ["refresh", "llama-server", str(drive), "--platform", "host"])

    assert result.exit_code == 0
    assert not tool_dir.exists()
    manifest = Manifest.load(drive / "manifest.yaml")
    assert manifest.entry_by_id("llama-server", "macos-arm64") is None
    assert manifest.entry_by_id("gemma-4-e2b-it") is not None
    sync_drive_mock.assert_called_once_with(
        str(drive),
        platform_filter="host",
        only_source_ids=["llama-server"],
    )
