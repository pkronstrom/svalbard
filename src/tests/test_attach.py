from click.testing import CliRunner
from unittest.mock import patch


def test_attach_adds_local_source_to_manifest_and_snapshot(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "library" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    result = CliRunner().invoke(main, ["attach", source_id, str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id in (drive / "manifest.yaml").read_text()
    assert (drive / ".svalbard" / "config" / "local" / "example.yaml").exists()


def test_detach_removes_local_source_from_manifest_and_snapshot(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "library" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path), local_sources=[source_id])

    result = CliRunner().invoke(main, ["detach", source_id, str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id not in (drive / "manifest.yaml").read_text()
    assert not (drive / ".svalbard" / "config" / "local" / "example.yaml").exists()


def test_attach_defaults_drive_path_from_cwd(tmp_path, monkeypatch):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "library" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))
    monkeypatch.chdir(drive)

    result = CliRunner().invoke(main, ["attach", source_id, "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id in (drive / "manifest.yaml").read_text()


def test_attach_browse_updates_manifest_preset_and_syncs(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import init_drive
    from svalbard.manifest import Manifest
    from svalbard.presets import load_preset

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    expected_checked_ids = {source.id for source in load_preset("default-32").sources}

    with patch("svalbard.cli.pick_sources_via_packs", return_value={"kiwix-serve", "wikem"}) as picker_mock, patch(
        "svalbard.cli.sync_drive"
    ) as sync_drive_mock:
        result = CliRunner().invoke(main, ["attach", "--browse", str(drive), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert picker_mock.call_args.kwargs["checked_ids"] == expected_checked_ids
    manifest = Manifest.load(drive / "manifest.yaml")
    assert manifest.preset.startswith("custom-")
    assert [source.id for source in load_preset(manifest.preset, workspace=tmp_path).sources] == [
        "kiwix-serve",
        "wikem",
    ]
    sync_drive_mock.assert_called_once_with(str(drive))


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
    )
