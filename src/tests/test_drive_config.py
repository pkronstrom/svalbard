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


def test_load_snapshot_preset_ignores_macos_appledouble_sidecars(tmp_path):
    from svalbard.drive_config import load_snapshot_preset

    config_root = tmp_path / ".svalbard" / "config"
    recipes_dir = config_root / "recipes"
    recipes_dir.mkdir(parents=True)

    (config_root / "preset.yaml").write_text(
        """name: finland-2
description: test
target_size_gb: 2
region: finland
sources:
- wikem
"""
    )
    (recipes_dir / "wikem.yaml").write_text(
        """id: wikem
type: zim
group: practical
size_gb: 0.042
description: WikEM
"""
    )
    (recipes_dir / "._wikem.yaml").write_bytes(b"\x00\x05\x16\x07binary-sidecar")

    preset = load_snapshot_preset(tmp_path)

    assert preset is not None
    assert preset.name == "finland-2"
    assert [source.id for source in preset.sources] == ["wikem"]
