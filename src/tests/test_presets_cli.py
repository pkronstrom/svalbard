from click.testing import CliRunner


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
