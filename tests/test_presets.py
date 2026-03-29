from svalbard.models import Preset, Source
from svalbard.presets import list_presets, load_preset


def test_parse_nordic_128():
    preset = load_preset("nordic-128")
    assert preset.name == "nordic-128"
    assert preset.region == "nordic"
    assert preset.target_size_gb == 128
    assert len(preset.sources) > 10
    assert preset.total_size_gb > 50


def test_sources_have_required_fields():
    preset = load_preset("nordic-128")
    for source in preset.sources:
        assert source.id, f"Source missing id: {source}"
        assert source.type, f"Source missing type: {source}"
        assert len(source.tags) > 0, f"Source {source.id} has no tags"
        assert source.size_gb > 0, f"Source {source.id} has size_gb <= 0"


def test_list_presets():
    presets = list_presets()
    assert "nordic-128" in presets


def test_optional_groups():
    preset = load_preset("nordic-128")
    with_maps = preset.sources_for_options({"maps"})
    without_maps = preset.sources_for_options(set())
    assert len(with_maps) > len(without_maps)
