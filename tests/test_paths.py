from pathlib import Path


def test_default_workspace_root_falls_back_to_user_data_dir(tmp_path, monkeypatch):
    from svalbard.paths import workspace_root

    monkeypatch.setenv("HOME", str(tmp_path))

    root = workspace_root(None, cwd=tmp_path / "not-a-workspace")

    assert root == tmp_path / ".local" / "share" / "svalbard"


def test_workspace_root_prefers_explicit_workspace(tmp_path):
    from svalbard.paths import workspace_root

    explicit = tmp_path / "my-workspace"
    assert workspace_root(explicit) == explicit.resolve()


def test_workspace_root_prefers_repo_style_workspace_when_present(tmp_path):
    from svalbard.paths import workspace_root

    (tmp_path / "presets").mkdir()
    (tmp_path / "recipes").mkdir()

    assert workspace_root(None, cwd=tmp_path) == tmp_path.resolve()
