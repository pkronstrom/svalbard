from pathlib import Path
from unittest.mock import patch

from click.testing import CliRunner
import pytest

from svalbard.cli import main
from svalbard.crawler import load_crawl_config, register_generated_zim


def test_load_crawl_config_accepts_documented_schema(tmp_path):
    config_path = tmp_path / "crawl.yaml"
    config_path.write_text(
        """name: Example Docs
sites:
  - url: https://example.com/docs
    scope: prefix
    page_limit: 50
defaults:
  size_limit_mb: 128
  timeout_minutes: 30
"""
    )

    config = load_crawl_config(config_path)
    assert config.output == "generated/example-docs.zim"
    assert config.sites[0].max_pages == 50
    assert config.max_size_mb == 128
    assert config.time_limit_minutes == 30


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


def test_run_config_crawl_rejects_multi_site_configs(tmp_path):
    from svalbard.crawler import run_config_crawl

    config_path = tmp_path / "crawl.yaml"
    config_path.write_text(
        """name: Example Bundle
sites:
  - url: https://example.com/docs
  - url: https://example.com/help
"""
    )

    with patch("svalbard.crawler.run_url_crawl") as run_url_mock:
        with patch("svalbard.crawler.register_generated_zim"):
            with pytest.raises(ValueError, match="single-site"):
                run_config_crawl(config_path, tmp_path)

    run_url_mock.assert_not_called()


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
