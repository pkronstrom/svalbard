from collections import namedtuple
from pathlib import Path

import svalbard.wizard as wizard
from svalbard.wizard import detect_volumes, local_sources_for_space, presets_for_space


def test_detect_volumes_returns_list():
    """detect_volumes should return a list (may be empty in CI)."""
    result = detect_volumes()
    assert isinstance(result, list)
    # Every volume must have the network classification field
    for v in result:
        assert "network" in v
        assert isinstance(v["network"], bool)


def test_detect_volumes_sorted_local_first():
    """Local volumes should appear before network volumes."""
    result = detect_volumes()
    saw_network = False
    for v in result:
        if v["network"]:
            saw_network = True
        elif saw_network:
            assert False, "Local volume appeared after network volume"


def test_detect_volumes_skips_time_machine_mount_names(monkeypatch):
    """Time Machine mount names should not be suggested as targets."""
    usage = namedtuple("usage", "total used free")
    volumes_root = Path("/Volumes")
    candidates = [
        volumes_root / ".timemachine",
        volumes_root / ".MobileBackups",
        volumes_root / "KINGSTON",
    ]

    monkeypatch.setattr(wizard.platform, "system", lambda: "Darwin")
    monkeypatch.setattr(wizard, "_parse_mount_types", lambda: {})
    monkeypatch.setattr(
        wizard.Path,
        "exists",
        lambda self: self == volumes_root,
    )
    monkeypatch.setattr(
        wizard.Path,
        "iterdir",
        lambda self: iter(candidates) if self == volumes_root else iter(()),
    )
    monkeypatch.setattr(wizard.shutil, "disk_usage", lambda _: usage(128 * 10**9, 0, 64 * 10**9))

    result = detect_volumes()

    assert [volume["name"] for volume in result] == ["KINGSTON"]


def test_presets_for_space_122gb():
    """122 GB free (typical 128 GB stick) should include finland-128 as fitting."""
    result = presets_for_space(122, region="finland")
    names = [name for name, _, _ in result]
    fitting = [name for name, _, fits in result if fits]
    assert "finland-128" in fitting
    assert "finland-32" in fitting
    assert "finland-64" in fitting
    # Larger presets should be present but not fitting
    assert "finland-256" in names
    assert "finland-256" not in fitting


def test_presets_for_space_includes_finland_2():
    result = presets_for_space(10, region="finland")
    names = [name for name, _, _ in result]

    assert "finland-2" in names


def test_presets_for_space_small_disk_marks_finland_2_as_fitting():
    result = presets_for_space(3, region="finland")
    fitting = [name for name, _, fits in result if fits]

    assert "finland-2" in fitting
    assert "finland-32" not in fitting


def test_presets_for_space_filters_by_region_family():
    result = presets_for_space(122, region="default")
    names = [name for name, _, _ in result]
    assert "default-64" in names
    assert "finland-64" not in names


def test_presets_for_space_default_region_includes_large_tiers():
    result = presets_for_space(2500, region="default")
    names = [name for name, _, _ in result]
    assert "default-256" in names
    assert "default-512" in names
    assert "default-1tb" in names
    assert "default-2tb" in names


def test_presets_for_space_sorted_by_size():
    """Presets should be sorted smallest first."""
    result = presets_for_space(500, region="finland")
    sizes = [size for _, size, _ in result]
    assert sizes == sorted(sizes)


def test_presets_for_space_too_small():
    """10 GB free should fit finland-2 but still reject larger Finland tiers."""
    result = presets_for_space(10, region="finland")
    names = [name for name, _, _ in result]
    fitting = [name for name, _, fits in result if fits]

    assert len(result) > 0
    assert "finland-2" in names
    assert "finland-2" in fitting
    assert "finland-32" not in fitting


def test_presets_for_space_defaults_to_finland_family():
    result = presets_for_space(122)
    names = [name for name, _, _ in result]
    assert "finland-64" in names
    assert "default-64" not in names


def test_local_sources_for_space_marks_fit_and_overflow(tmp_path):
    generated = tmp_path / "generated"
    local = tmp_path / "local"
    generated.mkdir()
    local.mkdir()
    (generated / "small.zim").write_bytes(b"x" * 100)
    (generated / "large.zim").write_bytes(b"x" * 200)
    (local / "small.yaml").write_text(
        """id: local:small
type: zim
group: practical
strategy: local
path: generated/small.zim
size_bytes: 100000000
"""
    )
    (local / "large.yaml").write_text(
        """id: local:large
type: zim
group: practical
strategy: local
path: generated/large.zim
size_bytes: 3000000000
"""
    )

    result = local_sources_for_space(1.0, root=tmp_path)
    by_id = {source.id: fits for source, _, fits in result}
    assert by_id["local:small"] is True
    assert by_id["local:large"] is False
