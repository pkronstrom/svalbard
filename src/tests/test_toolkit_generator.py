import json
import yaml
from pathlib import Path

from svalbard.toolkit_generator import generate_toolkit


def _write_manifest(drive_path: Path, data: dict) -> None:
    (drive_path / "manifest.yaml").write_text(yaml.dump(data))


def _read_actions_config(drive_path: Path) -> dict:
    return json.loads((drive_path / ".svalbard" / "actions.json").read_text())


def _group(runtime_config: dict, group_id: str) -> dict | None:
    return next((group for group in runtime_config["groups"] if group["id"] == group_id), None)


def _group_items(runtime_config: dict, group_id: str) -> list[dict]:
    group = _group(runtime_config, group_id)
    return [] if group is None else group["items"]


def test_generate_toolkit_creates_run(tmp_path):
    """run should be created at the drive root."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    assert (tmp_path / "run").exists()
    assert (tmp_path / ".svalbard" / "actions.json").exists()
    assert not (tmp_path / ".svalbard" / "actions").exists()
    assert not (tmp_path / ".svalbard" / "lib").exists()


def test_run_is_executable(tmp_path):
    """run should have executable permission."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")

    import os
    assert os.access(tmp_path / "run", os.X_OK)


def test_generate_toolkit_copies_platform_runtime_launchers(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")

    for platform in ("macos-arm64", "macos-x86_64", "linux-arm64", "linux-x86_64"):
        assert (tmp_path / ".svalbard" / "runtime" / platform / "svalbard-drive").exists()


def test_run_execs_platform_runtime_binary(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")

    text = (tmp_path / "run").read_text()
    assert ".svalbard/runtime/" in text
    assert 'uname -s' in text
    assert 'exec "$DRIVE_ROOT/.svalbard/runtime/$platform/svalbard-drive"' in text


def test_runtime_config_omits_sections_without_content(tmp_path):
    """Groups for missing content should not appear."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    runtime = _read_actions_config(tmp_path)
    groups = [group["id"] for group in runtime["groups"]]
    assert runtime["version"] == 2
    assert groups == ["library", "tools"]


def test_runtime_config_groups_top_level_menu(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-512",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
            {"id": "gemma-4-e2b-it", "type": "gguf",
             "filename": "gemma-4-e2b-it-Q4_0.gguf",
             "size_bytes": 3_040_000_000, "tags": ["tool-calling"], "depth": "overview"},
            {"id": "llama-server", "type": "binary",
             "filename": "llama-b8586-bin-macos-arm64.tar.gz",
             "size_bytes": 40_000_000, "tags": [], "depth": "reference-only"},
            {"id": "opencode", "type": "binary",
             "filename": "opencode-darwin-arm64.zip",
             "size_bytes": 40_000_000, "tags": [], "depth": "reference-only"},
            {"id": "osm-finland", "type": "pmtiles",
             "filename": "osm-finland.pmtiles",
             "size_bytes": 500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()
    (tmp_path / "models").mkdir()
    (tmp_path / "models" / "gemma-4-e2b-it-Q4_0.gguf").touch()
    (tmp_path / "maps").mkdir()
    (tmp_path / "maps" / "osm-finland.pmtiles").touch()
    (tmp_path / "data").mkdir()
    (tmp_path / "data" / "search.db").touch()

    generate_toolkit(tmp_path, "default-512")

    runtime = _read_actions_config(tmp_path)
    assert [group["id"] for group in runtime["groups"]] == ["search", "library", "maps", "local-ai", "tools"]


def test_library_group_contains_format_subheaders(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    library = _group(_read_actions_config(tmp_path), "library")
    assert library is not None
    assert library["description"]
    assert any(item.get("subheader") == "Archives" for item in library["items"])
    assert all(item["description"] for item in library["items"])


def test_runtime_config_includes_maps_when_present(tmp_path):
    """Maps group should appear when pmtiles exist."""
    _write_manifest(tmp_path, {
        "preset": "finland-128",
        "region": "finland",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "osm-finland", "type": "pmtiles",
             "filename": "osm-finland.pmtiles",
             "size_bytes": 500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "maps").mkdir()
    (tmp_path / "maps" / "osm-finland.pmtiles").touch()

    generate_toolkit(tmp_path, "finland-128")

    runtime = _read_actions_config(tmp_path)
    items = _group_items(runtime, "maps")
    assert len(items) == 1
    assert items[0]["action"] == {
        "type": "builtin",
        "config": {
            "name": "maps",
            "args": {},
        },
    }


def test_runtime_config_includes_serve_when_content_exists(tmp_path):
    """Network tools should appear when there are servable files."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    labels = {item["label"] for item in _group_items(_read_actions_config(tmp_path), "tools")}
    assert "Serve everything" in labels
    assert "Share on local network" in labels


def test_runtime_config_always_has_info(tmp_path):
    """Drive tools should always be present."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")

    labels = {item["label"] for item in _group_items(_read_actions_config(tmp_path), "tools")}
    assert "List drive contents" in labels
    assert "Verify checksums" not in labels  # no entries = no checksums


def test_regeneration_cleans_legacy_shell_runtime_artifacts(tmp_path):
    """Calling generate_toolkit should remove old shell-runtime artifacts from earlier drives."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")
    (tmp_path / ".svalbard" / "actions").mkdir(parents=True)
    (tmp_path / ".svalbard" / "lib").mkdir(parents=True)
    (tmp_path / ".svalbard" / "actions" / "stale.sh").write_text("old")
    (tmp_path / ".svalbard" / "lib" / "ui.sh").write_text("old")
    (tmp_path / ".svalbard" / "entries.tab").write_text("old")

    generate_toolkit(tmp_path, "default-32")

    assert not (tmp_path / ".svalbard" / "actions").exists()
    assert not (tmp_path / ".svalbard" / "lib").exists()
    assert not (tmp_path / ".svalbard" / "entries.tab").exists()


def test_runtime_config_includes_search_when_db_exists(tmp_path):
    """Search group should appear when search.db exists."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()
    (tmp_path / "data").mkdir()
    (tmp_path / "data" / "search.db").touch()

    generate_toolkit(tmp_path, "default-32")

    items = _group_items(_read_actions_config(tmp_path), "search")
    assert len(items) == 1
    assert items[0]["action"] == {
        "type": "builtin",
        "config": {
            "name": "search",
            "args": {},
        },
    }


def test_runtime_config_omits_search_when_no_db(tmp_path):
    """Search group should NOT appear when search.db does not exist."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    assert not _group_items(_read_actions_config(tmp_path), "search")


def test_runtime_config_includes_embedded_when_toolchains_present(tmp_path):
    """Embedded dev tools should appear when toolchain entries exist."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "toolchain-xtensa-esp-elf", "type": "toolchain",
             "filename": "toolchain-xtensa-esp-elf-linux_x86_64-14.2.0.tar.gz",
             "size_bytes": 200_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })

    generate_toolkit(tmp_path, "default-32")

    actions = _group_items(_read_actions_config(tmp_path), "tools")
    embedded = [
        item for item in actions
        if item["action"] == {
            "type": "builtin",
            "config": {
                "name": "embedded-shell",
                "args": {},
            },
        }
    ]
    assert len(embedded) == 1
    assert embedded[0]["label"] == "Open embedded dev shell"


def test_runtime_config_omits_embedded_when_no_toolchains(tmp_path):
    """Embedded dev tool should NOT appear when no toolchain entries exist."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "wikipedia-en-nopic", "type": "zim",
             "filename": "wikipedia-en-nopic.zim",
             "size_bytes": 4_500_000_000, "tags": [], "depth": "comprehensive"},
        ],
    })
    (tmp_path / "zim").mkdir()
    (tmp_path / "zim" / "wikipedia-en-nopic.zim").touch()

    generate_toolkit(tmp_path, "default-32")

    assert "Open embedded dev shell" not in {
        item["label"] for item in _group_items(_read_actions_config(tmp_path), "tools")
    }


def test_runtime_config_includes_ai_clients_when_models_and_binaries_exist(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-512",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "gemma-4-e2b-it", "type": "gguf",
             "filename": "gemma-4-e2b-it-Q4_0.gguf",
             "size_bytes": 3_040_000_000, "tags": ["tool-calling"], "depth": "overview"},
            {"id": "qwen-9b", "type": "gguf",
             "filename": "Qwen3.5-9B-Q4_K_M.gguf",
             "size_bytes": 5_900_000_000, "tags": ["coding"], "depth": "overview"},
            {"id": "llama-server", "type": "binary",
             "filename": "llama-b8586-bin-macos-arm64.tar.gz",
             "size_bytes": 40_000_000, "tags": [], "depth": "reference-only"},
            {"id": "opencode", "type": "binary",
             "filename": "opencode-darwin-arm64.zip",
             "size_bytes": 40_000_000, "tags": [], "depth": "reference-only"},
            {"id": "crush", "type": "binary",
             "filename": "crush-darwin-arm64.tar.gz",
             "size_bytes": 30_000_000, "tags": [], "depth": "reference-only"},
            {"id": "goose", "type": "binary",
             "filename": "goose-aarch64-apple-darwin.tar.bz2",
             "size_bytes": 40_000_000, "tags": [], "depth": "reference-only"},
        ],
    })

    generate_toolkit(tmp_path, "default-512")

    actions = _group_items(_read_actions_config(tmp_path), "local-ai")
    labels = {action["label"] for action in actions}
    assert any("Chat with Gemma 4 E2B IT" in label for label in labels)
    assert any("Chat with Qwen3.5 9B Instruct" in label for label in labels)
    assert "OpenCode with local model" in labels
    assert "Crush with local model" in labels
    assert "Goose with local model" in labels
    agent_clients = {
        action["action"]["config"]["args"]["client"]
        for action in actions
        if action["action"]["config"]["name"] == "agent"
    }
    assert agent_clients == {"opencode", "crush", "goose"}


def test_runtime_config_omits_ai_clients_without_llama_server(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-512",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [
            {"id": "gemma-4-e2b-it", "type": "gguf",
             "filename": "gemma-4-e2b-it-Q4_0.gguf",
             "size_bytes": 3_040_000_000, "tags": ["tool-calling"], "depth": "overview"},
            {"id": "opencode", "type": "binary",
             "filename": "opencode-darwin-arm64.zip",
             "size_bytes": 40_000_000, "tags": [], "depth": "reference-only"},
        ],
    })

    generate_toolkit(tmp_path, "default-512")

    labels = {action["label"] for action in _group_items(_read_actions_config(tmp_path), "local-ai")}
    assert "OpenCode with local model" not in labels


def test_actions_config_preserves_recipe_defined_exec_action(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    config_root = tmp_path / ".svalbard" / "config"
    recipes_dir = config_root / "recipes"
    recipes_dir.mkdir(parents=True)
    (config_root / "preset.yaml").write_text(
        """name: default-32
description: test
target_size_gb: 32
region: default
sources:
- sql-shell
"""
    )
    (recipes_dir / "sql-shell.yaml").write_text(
        """id: sql-shell
type: app
description: SQLite shell
menu:
  group: tools
  label: SQLite shell
  description: Open the bundled SQLite shell.
action:
  type: exec
  config:
    executable: sqlite3
    resolve_from: drive-bin
    args:
      - "{drive_root}/data/search.db"
    cwd: "{drive_root}"
    mode: interactive
"""
    )
    (tmp_path / "apps" / "sql-shell").mkdir(parents=True)

    generate_toolkit(tmp_path, "default-32")

    tools = _group(_read_actions_config(tmp_path), "tools")
    assert tools is not None
    item = next(item for item in tools["items"] if item["id"] == "sql-shell")
    assert item["action"] == {
        "type": "exec",
        "config": {
            "executable": "sqlite3",
            "resolve_from": "drive-bin",
            "args": ["{drive_root}/data/search.db"],
            "cwd": "{drive_root}",
        "mode": "interactive",
        },
    }
