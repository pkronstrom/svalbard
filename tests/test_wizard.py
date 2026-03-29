from svalbard.wizard import detect_volumes, presets_for_space


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


def test_presets_for_space_filters_by_region_family():
    result = presets_for_space(122, region="default")
    names = [name for name, _, _ in result]
    assert "default-64" in names
    assert "finland-64" not in names


def test_presets_for_space_sorted_by_size():
    """Presets should be sorted smallest first."""
    result = presets_for_space(500, region="finland")
    sizes = [size for _, size, _ in result]
    assert sizes == sorted(sizes)


def test_presets_for_space_too_small():
    """10 GB free should return all presets, none fitting."""
    result = presets_for_space(10, region="finland")
    assert len(result) > 0
    assert all(not fits for _, _, fits in result)


def test_presets_for_space_defaults_to_finland_family():
    result = presets_for_space(122)
    names = [name for name, _, _ in result]
    assert "finland-64" in names
    assert "default-64" not in names
