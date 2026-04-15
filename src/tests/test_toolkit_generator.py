import yaml
from pathlib import Path

from svalbard.toolkit_generator import generate_toolkit


def _write_manifest(drive_path: Path, data: dict) -> None:
    (drive_path / "manifest.yaml").write_text(yaml.dump(data))


def test_generate_toolkit_creates_run_sh(tmp_path):
    """run.sh should be created at the drive root."""
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

    assert (tmp_path / "run.sh").exists()
    assert (tmp_path / ".svalbard" / "entries.tab").exists()
    assert (tmp_path / ".svalbard" / "actions" / "browse.sh").exists()
    assert (tmp_path / ".svalbard" / "lib" / "ui.sh").exists()


def test_run_sh_is_executable(tmp_path):
    """run.sh should have executable permission."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")

    import os
    assert os.access(tmp_path / "run.sh", os.X_OK)


def test_entries_tab_omits_sections_without_content(tmp_path):
    """Sections for missing content should not appear."""
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

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[browse]" in tab_content
    assert "[ai]" not in tab_content
    assert "[maps]" not in tab_content


def test_entries_tab_includes_maps_when_present(tmp_path):
    """Maps section should appear when pmtiles exist."""
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

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[maps]" in tab_content


def test_entries_tab_includes_serve_when_content_exists(tmp_path):
    """Serve section should appear when there are servable files."""
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

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[serve]" in tab_content
    assert "Serve everything" in tab_content
    assert "Share on local network" in tab_content


def test_entries_tab_always_has_info(tmp_path):
    """Info section should always be present."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[info]" in tab_content
    assert "List drive contents" in tab_content
    # Verify checksums only appears when manifest has entries with checksums
    assert "Verify checksums" not in tab_content  # no entries = no checksums


def test_regeneration_cleans_old_svalbard_dir(tmp_path):
    """Calling generate_toolkit twice should clean managed subdirs (.svalbard/actions/, .svalbard/lib/)."""
    _write_manifest(tmp_path, {
        "preset": "default-32",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-32")
    # Add a stale file inside a managed subdir
    (tmp_path / ".svalbard" / "actions" / "stale.sh").write_text("old")

    generate_toolkit(tmp_path, "default-32")

    assert not (tmp_path / ".svalbard" / "actions" / "stale.sh").exists()


def test_entries_tab_includes_search_when_db_exists(tmp_path):
    """Search entry should appear when search.db exists."""
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

    entries = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[search]" in entries
    assert "search.sh" in entries


def test_entries_tab_omits_search_when_no_db(tmp_path):
    """Search entry should NOT appear when search.db does not exist."""
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

    entries = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[search]" not in entries


def test_entries_tab_includes_embedded_when_toolchains_present(tmp_path):
    """Embedded dev section should appear when toolchain entries exist."""
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

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[embedded]" in tab_content
    assert "Open embedded dev shell" in tab_content
    assert "pio-setup.sh" in tab_content


def test_entries_tab_omits_embedded_when_no_toolchains(tmp_path):
    """Embedded dev section should NOT appear when no toolchain entries exist."""
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

    tab_content = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[embedded]" not in tab_content


def test_generate_toolkit_copies_agent_launcher(tmp_path):
    _write_manifest(tmp_path, {
        "preset": "default-512",
        "region": "default",
        "target_path": str(tmp_path),
        "entries": [],
    })

    generate_toolkit(tmp_path, "default-512")

    assert (tmp_path / ".svalbard" / "actions" / "agent.sh").exists()


def test_entries_tab_includes_ai_clients_when_models_and_binaries_exist(tmp_path):
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

    entries = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "[ai]" in entries
    assert "Chat with Gemma 4 E2B IT" in entries
    assert "Chat with Qwen3.5 9B Instruct" in entries
    assert "OpenCode with local model" in entries
    assert "Crush with local model" in entries
    assert "Goose with local model" in entries
    assert ".svalbard/actions/agent.sh\topencode" in entries
    assert ".svalbard/actions/agent.sh\tcrush" in entries
    assert ".svalbard/actions/agent.sh\tgoose" in entries


def test_entries_tab_omits_ai_clients_without_llama_server(tmp_path):
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

    entries = (tmp_path / ".svalbard" / "entries.tab").read_text()
    assert "OpenCode with local model" not in entries
