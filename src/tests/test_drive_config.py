def test_init_drive_writes_config_snapshot(tmp_path):
    from svalbard.commands import init_drive

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path))

    assert (drive / ".svalbard" / "config" / "preset.yaml").exists()
    assert (drive / ".svalbard" / "config" / "recipes").exists()


def test_drive_snapshot_contains_selected_local_source_metadata(tmp_path):
    from svalbard.commands import add_local_source, init_drive

    artifact = tmp_path / "library" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")
    source_id = add_local_source(artifact, workspace_root=tmp_path, source_type="zim")

    drive = tmp_path / "drive"
    init_drive(str(drive), "default-32", workspace_root=str(tmp_path), local_sources=[source_id])

    snapshot = drive / ".svalbard" / "config" / "local" / "example.yaml"
    assert snapshot.exists()
