from click.testing import CliRunner
from unittest.mock import patch


def test_compute_builtin_selection_review_classifies_changes():
    from svalbard.cli import _compute_builtin_selection_review
    from svalbard.manifest import Manifest, ManifestEntry
    from svalbard.models import Source

    manifest = Manifest(
        preset="default-32",
        region="default",
        target_path="/tmp/drive",
        entries=[
            ManifestEntry(id="kiwix-serve", type="binary", filename="kiwix.tar.gz", size_bytes=10),
            ManifestEntry(id="wikem", type="zim", filename="wikem.zim", size_bytes=10),
        ],
    )
    source_lookup = {
        "kiwix-serve": Source(id="kiwix-serve", type="binary", size_gb=0.2),
        "wikem": Source(id="wikem", type="zim", size_gb=1.0),
        "qwen-9b": Source(id="qwen-9b", type="gguf", size_gb=5.2),
    }

    review = _compute_builtin_selection_review(
        manifest=manifest,
        current_ids={"kiwix-serve", "wikem"},
        selected_ids={"kiwix-serve", "qwen-9b"},
        source_lookup=source_lookup,
    )

    assert review.added_ids == ["qwen-9b"]
    assert review.removed_ids == ["wikem"]
    assert review.will_download_ids == ["qwen-9b"]
    assert review.will_remove_ids == ["wikem"]
    assert review.current_total_gb == 1.2
    assert review.selected_total_gb == 5.4
    assert review.size_delta_gb == 4.2


def test_add_accepts_bare_local_source_name(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "library" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    result = CliRunner().invoke(main, ["add", "example", str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id in (drive / "manifest.yaml").read_text()
    assert (drive / ".svalbard" / "config" / "local" / "example.yaml").exists()


def test_add_builtin_source_updates_manifest_preset_and_syncs(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import init_drive
    from svalbard.manifest import Manifest
    from svalbard.presets import load_preset

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    with patch("svalbard.cli.sync_drive") as sync_drive_mock:
        result = CliRunner().invoke(main, ["add", "qwen-9b", str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    manifest = Manifest.load(drive / "manifest.yaml")
    assert manifest.preset.startswith("custom-")
    source_ids = [source.id for source in load_preset(manifest.preset, workspace=tmp_path).sources]
    assert "qwen-9b" in source_ids
    sync_drive_mock.assert_called_once_with(str(drive))


def test_remove_removes_local_source_from_manifest_and_snapshot(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "library" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path), local_sources=[source_id])

    result = CliRunner().invoke(main, ["remove", "example", str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id not in (drive / "manifest.yaml").read_text()
    assert not (drive / ".svalbard" / "config" / "local" / "example.yaml").exists()


def test_add_defaults_drive_path_from_cwd(tmp_path, monkeypatch):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "library" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))
    monkeypatch.chdir(drive)

    result = CliRunner().invoke(main, ["add", "example", "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id in (drive / "manifest.yaml").read_text()


def test_add_browse_updates_manifest_preset_and_syncs(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import init_drive
    from svalbard.manifest import Manifest
    from svalbard.presets import load_preset

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    expected_checked_ids = {source.id for source in load_preset("default-32").sources}

    with patch("svalbard.cli.pick_sources_via_packs", return_value={"kiwix-serve", "wikem"}) as picker_mock, patch(
        "svalbard.cli._review_builtin_selection", return_value="a"
    ), patch("svalbard.cli.sync_drive") as sync_drive_mock:
        result = CliRunner().invoke(main, ["add", "--browse", str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert picker_mock.call_args.kwargs["checked_ids"] == expected_checked_ids
    manifest = Manifest.load(drive / "manifest.yaml")
    assert manifest.preset.startswith("custom-")
    assert [source.id for source in load_preset(manifest.preset, workspace=tmp_path).sources] == [
        "kiwix-serve",
        "wikem",
    ]
    sync_drive_mock.assert_called_once_with(str(drive))


def test_add_browse_back_returns_to_picker_before_applying(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import init_drive

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    picker_returns = [{"kiwix-serve"}, {"kiwix-serve", "wikem"}]
    review_actions = ["b", "a"]

    with patch("svalbard.cli.pick_sources_via_packs", side_effect=picker_returns) as picker_mock, patch(
        "svalbard.cli._review_builtin_selection", side_effect=review_actions
    ) as review_mock, patch("svalbard.cli.sync_drive") as sync_drive_mock:
        result = CliRunner().invoke(main, ["add", "--browse", str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert picker_mock.call_count == 2
    assert review_mock.call_count == 2
    sync_drive_mock.assert_called_once_with(str(drive))


def test_add_browse_quit_cancels_without_applying(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import init_drive
    from svalbard.manifest import Manifest

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))
    original_manifest = Manifest.load(drive / "manifest.yaml")

    with patch("svalbard.cli.pick_sources_via_packs", return_value=None), patch(
        "svalbard.cli.sync_drive"
    ) as sync_drive_mock:
        result = CliRunner().invoke(main, ["add", "--browse", str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    manifest = Manifest.load(drive / "manifest.yaml")
    assert manifest.preset == original_manifest.preset
    sync_drive_mock.assert_not_called()


def test_attach_command_is_not_available():
    from svalbard.cli import main

    result = CliRunner().invoke(main, ["attach"])

    assert result.exit_code != 0
    assert "No such command 'attach'" in result.output


def test_detach_command_is_not_available():
    from svalbard.cli import main

    result = CliRunner().invoke(main, ["detach"])

    assert result.exit_code != 0
    assert "No such command 'detach'" in result.output


def test_init_command_routes_through_picker_flow(tmp_path):
    from svalbard.cli import main

    drive = tmp_path / "drive"
    with patch("svalbard.cli.run_wizard") as run_wizard_mock:
        result = CliRunner().invoke(main, ["init", str(drive), "--preset", "finland-32"])

    assert result.exit_code == 0
    run_wizard_mock.assert_called_once_with(
        target_path=str(drive),
        preset_name="finland-32",
        browse_only=True,
        platform=None,
    )


def test_init_command_forwards_platform_filter(tmp_path):
    from svalbard.cli import main

    drive = tmp_path / "drive"
    with patch("svalbard.cli.run_wizard") as run_wizard_mock:
        result = CliRunner().invoke(main, ["init", str(drive), "--preset", "finland-32", "--platform", "host"])

    assert result.exit_code == 0
    run_wizard_mock.assert_called_once_with(
        target_path=str(drive),
        preset_name="finland-32",
        browse_only=True,
        platform="host",
    )
