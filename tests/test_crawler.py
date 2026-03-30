from pathlib import Path
from unittest.mock import patch

from click.testing import CliRunner
import pytest

from svalbard.cli import main
from svalbard.crawler import load_crawl_config, register_crawled_zim


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


def test_register_crawled_zim_writes_recipe_and_crawl_metadata(tmp_path):
    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir()
    artifact.write_bytes(b"data")

    source_id = register_crawled_zim(
        workspace_root=tmp_path,
        artifact_path=artifact,
        origin_url="https://example.com/docs",
        source_id="local:example",
    )

    assert source_id == "local:example"
    assert (tmp_path / "local" / "example.yaml").exists()
    assert (tmp_path / "generated" / "example.crawl.yaml").exists()


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
        with patch("svalbard.crawler.register_crawled_zim"):
            with pytest.raises(ValueError, match="single-site"):
                run_config_crawl(config_path, tmp_path)

    run_url_mock.assert_not_called()


def test_crawl_url_cli_requires_subcommand_and_registers_output(tmp_path):
    artifact = tmp_path / "generated" / "example.zim"
    artifact.parent.mkdir(parents=True, exist_ok=True)
    artifact.write_bytes(b"data")
    runner = CliRunner()

    with patch("svalbard.cli.check_docker", return_value=True), \
         patch("svalbard.cli.ensure_zimit_image", return_value=True), \
         patch("svalbard.cli.run_url_crawl", return_value=artifact):
        result = runner.invoke(
            main,
            [
                "crawl",
                "url",
                "https://example.com/docs",
                "-o",
                "example.zim",
                "--workspace",
                str(tmp_path),
            ],
        )

    assert result.exit_code == 0
    assert (tmp_path / "local" / "example.yaml").exists()
