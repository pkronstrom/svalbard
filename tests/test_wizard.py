from primer.wizard import detect_volumes, find_best_preset


def test_detect_volumes_returns_list():
    """detect_volumes should return a list (may be empty in CI)."""
    result = detect_volumes()
    assert isinstance(result, list)


def test_find_best_preset_128():
    """128 GB budget should pick nordic-128."""
    result = find_best_preset(128)
    assert result == "nordic-128"


def test_find_best_preset_200():
    """200 GB budget should still pick nordic-128 (no 200 preset)."""
    result = find_best_preset(200)
    assert result == "nordic-128"


def test_find_best_preset_too_small():
    """10 GB budget should return None (below smallest preset)."""
    result = find_best_preset(10)
    assert result is None
