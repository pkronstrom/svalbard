"""Tests for recipe dependency resolution."""
from pathlib import Path

import pytest
import yaml

from svalbard.presets import _build_recipe_index, _resolve_deps, _load_dep_defaults, _source_from_recipe


@pytest.fixture
def tmp_recipes(tmp_path):
    """Create a minimal recipe tree for testing."""
    recipes_dir = tmp_path / "recipes"
    tools = recipes_dir / "tools"
    models = recipes_dir / "models"
    tools.mkdir(parents=True)
    models.mkdir(parents=True)

    (tools / "llama-server.yaml").write_text(yaml.dump({
        "id": "llama-server", "type": "binary", "size_gb": 0.04,
        "description": "llama.cpp server",
    }))
    (tools / "kiwix-serve.yaml").write_text(yaml.dump({
        "id": "kiwix-serve", "type": "binary", "size_gb": 0.01,
        "description": "Kiwix server",
    }))
    (tools / "go-pmtiles.yaml").write_text(yaml.dump({
        "id": "go-pmtiles", "type": "binary", "size_gb": 0.01,
    }))
    (tools / "maplibre-vendor.yaml").write_text(yaml.dump({
        "id": "maplibre-vendor", "type": "binary", "size_gb": 0.01,
    }))
    (tools / "dufs.yaml").write_text(yaml.dump({
        "id": "dufs", "type": "binary", "size_gb": 0.01,
    }))
    (models / "test-model.yaml").write_text(yaml.dump({
        "id": "test-model", "type": "gguf", "size_gb": 5.0,
        "description": "Test GGUF model",
    }))
    (models / "custom-model.yaml").write_text(yaml.dump({
        "id": "custom-model", "type": "gguf", "size_gb": 3.0,
        "deps": ["llama-server", "kiwix-serve"],
        "description": "Custom deps model",
    }))
    (models / "standalone-model.yaml").write_text(yaml.dump({
        "id": "standalone-model", "type": "gguf", "size_gb": 1.0,
        "deps": [],
        "description": "No deps needed",
    }))
    (recipes_dir / "content").mkdir()
    (recipes_dir / "content" / "wiki.yaml").write_text(yaml.dump({
        "id": "wiki", "type": "zim", "size_gb": 2.0,
    }))

    return recipes_dir


@pytest.fixture
def dep_defaults(tmp_path):
    """Load dep defaults from a test file."""
    path = tmp_path / "dep-defaults.yaml"
    path.write_text(yaml.dump({
        "gguf": ["llama-server"],
        "pmtiles": ["go-pmtiles", "maplibre-vendor", "dufs"],
        "zim": ["kiwix-serve"],
    }))
    return _load_dep_defaults(path)


def test_load_dep_defaults(tmp_path):
    path = tmp_path / "dep-defaults.yaml"
    path.write_text(yaml.dump({
        "gguf": ["llama-server"],
        "pmtiles": ["go-pmtiles", "maplibre-vendor", "dufs"],
    }))
    defaults = _load_dep_defaults(path)
    assert defaults["gguf"] == ["llama-server"]
    assert defaults["pmtiles"] == ["go-pmtiles", "maplibre-vendor", "dufs"]


def test_load_dep_defaults_missing():
    defaults = _load_dep_defaults(Path("/nonexistent/path.yaml"))
    assert defaults == {}


def test_type_default_deps(tmp_recipes, dep_defaults):
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["test-model"])]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert "test-model" in ids
    assert "llama-server" in ids
    llama = next(s for s in resolved if s.id == "llama-server")
    assert llama.auto_dep is True
    model = next(s for s in resolved if s.id == "test-model")
    assert model.auto_dep is False


def test_recipe_level_deps_override(tmp_recipes, dep_defaults):
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["custom-model"])]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert "llama-server" in ids
    assert "kiwix-serve" in ids


def test_explicit_empty_deps(tmp_recipes, dep_defaults):
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["standalone-model"])]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert ids == {"standalone-model"}


def test_transitive_deps(tmp_recipes, dep_defaults):
    (tmp_recipes / "tools" / "llama-server.yaml").write_text(yaml.dump({
        "id": "llama-server", "type": "binary", "size_gb": 0.04,
        "deps": ["dufs"],
    }))
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["test-model"])]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert "dufs" in ids


def test_cycle_detection(tmp_recipes, dep_defaults):
    (tmp_recipes / "tools" / "llama-server.yaml").write_text(yaml.dump({
        "id": "llama-server", "type": "binary", "size_gb": 0.04,
        "deps": ["test-model"],
    }))
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["test-model"])]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert "test-model" in ids
    assert "llama-server" in ids


def test_already_selected_dep_not_marked_auto(tmp_recipes, dep_defaults):
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [
        _source_from_recipe(recipe_index["test-model"]),
        _source_from_recipe(recipe_index["llama-server"]),
    ]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    llama = next(s for s in resolved if s.id == "llama-server")
    assert llama.auto_dep is False


def test_zim_gets_kiwix(tmp_recipes, dep_defaults):
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["wiki"])]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert "kiwix-serve" in ids


def test_no_deps_for_unknown_type(tmp_recipes, dep_defaults):
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["dufs"])]
    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert ids == {"dufs"}


def test_preserves_existing_sources(tmp_recipes, dep_defaults):
    recipe_index = _build_recipe_index([tmp_recipes])
    original = _source_from_recipe(recipe_index["test-model"])
    original.size_gb = 99.9
    resolved = _resolve_deps([original], recipe_index, dep_defaults)
    model = next(s for s in resolved if s.id == "test-model")
    assert model.size_gb == 99.9
    assert model is original


def test_warns_on_missing_dep_id(tmp_recipes, tmp_path, caplog):
    defaults = {"gguf": ["nonexistent-tool"]}
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["test-model"])]
    import logging
    with caplog.at_level(logging.WARNING):
        resolved = _resolve_deps(sources, recipe_index, defaults)
    assert "nonexistent-tool" in caplog.text
    ids = {s.id for s in resolved}
    assert "test-model" in ids
    assert "nonexistent-tool" not in ids


# ── Task 4: parse_preset integration tests ─────────────────────────────────

from svalbard.presets import parse_preset


def test_parse_preset_resolves_deps(tmp_path, tmp_recipes):
    dep_file = tmp_path / "dep-defaults.yaml"
    dep_file.write_text(yaml.dump({"gguf": ["llama-server"]}))
    defaults = _load_dep_defaults(dep_file)
    preset_file = tmp_path / "test-preset.yaml"
    preset_file.write_text(yaml.dump({
        "name": "test",
        "description": "test preset",
        "target_size_gb": 10,
        "sources": ["test-model"],
    }))
    preset = parse_preset(
        preset_file,
        recipe_index=_build_recipe_index([tmp_recipes]),
        dep_defaults=defaults,
    )
    ids = {s.id for s in preset.sources}
    assert "test-model" in ids
    assert "llama-server" in ids
    llama = next(s for s in preset.sources if s.id == "llama-server")
    assert llama.auto_dep is True


def test_parse_preset_preserves_overrides(tmp_path, tmp_recipes):
    dep_file = tmp_path / "dep-defaults.yaml"
    dep_file.write_text(yaml.dump({"gguf": ["llama-server"]}))
    defaults = _load_dep_defaults(dep_file)
    preset_file = tmp_path / "test-preset.yaml"
    preset_file.write_text(yaml.dump({
        "name": "test",
        "description": "test preset",
        "target_size_gb": 10,
        "sources": [{"id": "test-model", "override": {"size_gb": 99.9}}],
    }))
    preset = parse_preset(
        preset_file,
        recipe_index=_build_recipe_index([tmp_recipes]),
        dep_defaults=defaults,
    )
    model = next(s for s in preset.sources if s.id == "test-model")
    assert model.size_gb == 99.9
