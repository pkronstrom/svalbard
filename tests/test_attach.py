from click.testing import CliRunner


def test_attach_adds_local_source_to_manifest_and_snapshot(tmp_path):
    from svalbard.cli import main
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "generated" / "example.zim"
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

    artifact = tmp_path / "generated" / "example.zim"
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

    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))
    monkeypatch.chdir(drive)

    result = CliRunner().invoke(main, ["attach", source_id, "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert source_id in (drive / "manifest.yaml").read_text()
