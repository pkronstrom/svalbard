"""Tests for the document bundle builder."""

import json
import zipfile
from pathlib import Path
from unittest.mock import Mock, patch

from click.testing import CliRunner

from svalbard.bundle import (
    _title_from_filename,
    _build_collection_json,
    _create_bundle_zip,
    run_bundle_build,
)
from svalbard.cli import main


def test_title_from_filename_strips_extension_and_cleans():
    assert _title_from_filename("survival-manual.pdf") == "Survival Manual"
    assert _title_from_filename("first_aid_guide.epub") == "First Aid Guide"
    assert _title_from_filename("RADIO operations.PDF") == "Radio Operations"


def test_build_collection_json_generates_entries():
    files = [Path("doc1.pdf"), Path("doc2.epub")]
    result = _build_collection_json(files)

    assert len(result) == 2
    assert result[0]["title"] == "Doc1"
    assert result[0]["files"] == ["doc1.pdf"]
    assert result[1]["title"] == "Doc2"
    assert result[1]["files"] == ["doc2.epub"]


def test_create_bundle_zip_contains_files_and_collection(tmp_path):
    staging = tmp_path / "staging"
    staging.mkdir()
    (staging / "manual.pdf").write_bytes(b"%PDF-fake")
    (staging / "guide.epub").write_bytes(b"PK-fake")
    files = [staging / "manual.pdf", staging / "guide.epub"]

    zip_path = _create_bundle_zip(files, tmp_path / "bundle.zip")

    assert zip_path.exists()
    with zipfile.ZipFile(zip_path) as zf:
        names = zf.namelist()
        assert "collection.json" in names
        assert "manual.pdf" in names
        assert "guide.epub" in names
        collection = json.loads(zf.read("collection.json"))
        assert len(collection) == 2
        for info in zf.infolist():
            assert info.compress_type == zipfile.ZIP_STORED


def test_run_bundle_build_invokes_docker_and_registers(tmp_path):
    workspace = tmp_path / "workspace"
    workspace.mkdir()
    (workspace / "generated").mkdir()

    f1 = tmp_path / "doc.pdf"
    f1.write_bytes(b"%PDF-fake-content")

    docker_result = Mock(returncode=0)

    def fake_docker_run(cmd, **kwargs):
        output_path = workspace / "generated" / "my-bundle.zim"
        output_path.write_bytes(b"ZIM-fake")
        return docker_result

    with (
        patch("svalbard.bundle.has_docker", return_value=True),
        patch("svalbard.bundle.ensure_tools_image", return_value=True),
        patch("svalbard.bundle.subprocess.run", side_effect=fake_docker_run),
        patch("svalbard.bundle.register_generated_zim", return_value="local:my-bundle") as reg_mock,
    ):
        source_id = run_bundle_build(
            files=[f1],
            name="my-bundle",
            workspace_root=workspace,
            title="My Bundle",
        )

    assert source_id == "local:my-bundle"
    reg_mock.assert_called_once()


def test_run_bundle_build_rejects_missing_files(tmp_path):
    import pytest

    with pytest.raises(FileNotFoundError):
        run_bundle_build(
            files=[Path("/nonexistent/file.pdf")],
            name="test",
            workspace_root=tmp_path,
        )


def test_bundle_cli_flag_invokes_run_bundle_build(tmp_path):
    f1 = tmp_path / "doc.pdf"
    f1.write_bytes(b"%PDF-fake")

    runner = CliRunner()
    with patch("svalbard.cli.run_bundle_build", return_value="local:test-bundle") as mock_build:
        result = runner.invoke(main, [
            "import",
            str(f1),
            "--bundle", "test-bundle",
            "--title", "Test Bundle",
            "--workspace", str(tmp_path),
        ])

    assert result.exit_code == 0
    assert "local:test-bundle" in result.output
    mock_build.assert_called_once()
    assert mock_build.call_args.kwargs["title"] == "Test Bundle"


def test_bundle_cli_requires_bundle_name_for_multiple_inputs(tmp_path):
    f1 = tmp_path / "a.pdf"
    f2 = tmp_path / "b.pdf"
    f1.write_bytes(b"a")
    f2.write_bytes(b"b")

    runner = CliRunner()
    result = runner.invoke(main, [
        "import",
        str(f1), str(f2),
        "--workspace", str(tmp_path),
    ])

    assert result.exit_code != 0


def test_bundle_cli_handles_multiple_files(tmp_path):
    f1 = tmp_path / "a.pdf"
    f2 = tmp_path / "b.epub"
    f1.write_bytes(b"a")
    f2.write_bytes(b"b")

    runner = CliRunner()
    with patch("svalbard.cli.run_bundle_build", return_value="local:my-docs") as mock_build:
        result = runner.invoke(main, [
            "import",
            str(f1), str(f2),
            "--bundle", "my-docs",
            "--workspace", str(tmp_path),
        ])

    assert result.exit_code == 0
    called_files = mock_build.call_args.kwargs["files"]
    assert len(called_files) == 2
