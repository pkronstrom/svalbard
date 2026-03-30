from svalbard.presets import list_presets, load_preset


def test_parse_finland_128():
    preset = load_preset("finland-128")
    assert preset.name == "finland-128"
    assert preset.region == "finland"
    assert preset.target_size_gb == 128
    assert len(preset.sources) > 10


def test_parse_finland_128_group_and_platforms():
    preset = load_preset("finland-128")
    tool = next(source for source in preset.sources if source.id == "kiwix-serve")
    assert tool.group == "tools"
    assert "linux-x86_64" in tool.platforms
    assert tool.platforms["linux-x86_64"].startswith("https://")


def test_list_presets_only_returns_canonical_names():
    presets = list_presets()
    assert "finland-128" in presets
    assert "default-64" in presets
    assert "nordic-128" not in presets


def test_list_presets_contains_finland_and_default_families():
    presets = list_presets()
    assert "finland-32" in presets
    assert "finland-1tb" in presets
    assert "default-32" in presets
    assert "default-128" in presets


def test_list_presets_contains_full_default_family():
    presets = list_presets()
    assert "default-32" in presets
    assert "default-64" in presets
    assert "default-128" in presets
    assert "default-256" in presets
    assert "default-512" in presets
    assert "default-1tb" in presets
    assert "default-2tb" in presets


def test_default_64_is_region_neutral():
    preset = load_preset("default-64")
    assert preset.region == "default"
    assert all(source.group != "regional" for source in preset.sources)


def test_default_512_includes_portable_qwen_model():
    preset = load_preset("default-512")
    ids = {source.id for source in preset.sources}
    assert "qwen-9b" in ids
    assert "llama-server" in ids
    assert "llama-3b" not in ids


def test_default_1tb_includes_large_qwen_model_under_24gb():
    preset = load_preset("default-1tb")
    large_model = next(source for source in preset.sources if source.id == "qwen-35b-a3b")
    assert large_model.size_gb < 24.0
    ids = {source.id for source in preset.sources}
    assert "qwen-35b-a3b" in ids
    assert "llama-server" in ids
    assert "llama-70b" not in ids


def test_default_2tb_uses_qwen_models_only():
    preset = load_preset("default-2tb")
    ids = {source.id for source in preset.sources}
    assert "qwen-35b-a3b" in ids
    assert "qwen-9b" in ids
    assert "llama-70b" not in ids
    assert "llama-3b" not in ids


def test_default_2tb_stays_region_neutral():
    preset = load_preset("default-2tb")
    ids = {source.id for source in preset.sources}
    assert "wikipedia-fi-all" not in ids
    assert "wiktionary-fi" not in ids
    assert "wikipedia-sv-all" not in ids


def test_finland_128_uses_standalone_sources_only():
    preset = load_preset("finland-128")
    ids = {source.id for source in preset.sources}
    assert "wikipedia-en-nopic" in ids
    assert "kiwix-serve" in ids
    assert all(not source.platforms or source.type == "binary" for source in preset.sources)


def test_finland_family_uses_canonical_metadata():
    for preset_name in [name for name in list_presets() if name.startswith("finland-")]:
        preset = load_preset(preset_name)
        assert preset.name == preset_name
        assert preset.region == "finland"


def test_canonical_presets_do_not_use_legacy_source_fields():
    for preset_name in list_presets():
        preset = load_preset(preset_name)
        for source in preset.sources:
            assert not hasattr(source, "optional_group")
            assert not hasattr(source, "replaces")


def test_all_preset_sources_resolve_from_recipes():
    """Every source ID in every preset must resolve to a recipe."""
    for preset_name in list_presets():
        preset = load_preset(preset_name)
        assert len(preset.sources) > 0, f"{preset_name} has no sources"
        for source in preset.sources:
            assert source.id, f"Source in {preset_name} has no id"
            assert source.type, f"Source {source.id} in {preset_name} has no type"


def test_recipes_have_consistent_group_field():
    """Every resolved source should have a group assigned."""
    for preset_name in list_presets():
        preset = load_preset(preset_name)
        for source in preset.sources:
            assert source.group, f"Source {source.id} in {preset_name} has no group"
