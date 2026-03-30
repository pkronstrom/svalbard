from unittest.mock import patch

from click.testing import CliRunner

from svalbard.cli import main
from svalbard.crawler import register_generated_zim


def test_register_generated_zim_writes_recipe_and_source_metadata(tmp_path):
    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")

    source_id = register_generated_zim(
        workspace_root=tmp_path,
        artifact_path=artifact,
        origin_url="https://example.com/docs",
        kind="web",
        runner="docker",
        tool="zimit",
        source_id="local:example",
    )

    assert source_id == "local:example"
    assert (tmp_path / "local" / "example.yaml").exists()
    metadata_path = tmp_path / "generated" / "example.source.yaml"
    assert metadata_path.exists()
    assert "kind: web" in metadata_path.read_text()
    assert "tool: zimit" in metadata_path.read_text()


def test_add_command_registers_local_file(tmp_path):
    runner = CliRunner()
    artifact = tmp_path / "manual.zim"
    artifact.write_bytes(b"data")

    result = runner.invoke(main, ["add", str(artifact), "--workspace", str(tmp_path)])

    assert result.exit_code == 0
    assert (tmp_path / "local" / "manual.yaml").exists()


def test_add_command_routes_remote_urls_through_run_add(tmp_path):
    runner = CliRunner()

    with patch("svalbard.cli.run_add", return_value="local:example") as run_add_mock:
        result = runner.invoke(
            main,
            [
                "add",
                "https://example.com/docs",
                "--workspace",
                str(tmp_path),
            ],
        )

    assert result.exit_code == 0
    run_add_mock.assert_called_once()


def test_add_command_passes_quality_and_audio_only_flags(tmp_path):
    runner = CliRunner()

    with patch("svalbard.cli.run_add", return_value="local:audio") as run_add_mock:
        result = runner.invoke(
            main,
            [
                "add",
                "https://areena.yle.fi/1-12345",
                "--workspace",
                str(tmp_path),
                "--quality",
                "1080p",
                "--audio-only",
            ],
        )

    assert result.exit_code == 0
    assert run_add_mock.call_args.kwargs["quality"] == "1080p"
    assert run_add_mock.call_args.kwargs["audio_only"] is True


def test_legacy_crawl_command_is_not_available():
    result = CliRunner().invoke(main, ["crawl"])

    assert result.exit_code != 0
    assert "No such command 'crawl'" in result.output


def test_legacy_local_command_is_not_available():
    result = CliRunner().invoke(main, ["local"])

    assert result.exit_code != 0
    assert "No such command 'local'" in result.output
