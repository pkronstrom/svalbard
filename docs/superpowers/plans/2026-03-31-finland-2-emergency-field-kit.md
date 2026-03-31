# Finland-2 Emergency Field Kit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a visible `finland-2` preset that implements the approved emergency field kit design and make the wizard, tests, and docs reflect it.

**Architecture:** This is a preset-only feature with light integration work. The new preset lives as one YAML file under `presets/`, preset loading stays unchanged, wizard visibility comes automatically from the preset list once tests and assumptions are updated, and docs are updated to describe the new smallest Finland tier.

**Tech Stack:** YAML presets, Python preset/wizard loaders, pytest

---

### Task 1: Add failing tests for the new preset and wizard behavior

**Files:**
- Modify: `tests/test_presets.py`
- Modify: `tests/test_wizard.py`

- [ ] **Step 1: Write the failing preset tests**

```python
def test_parse_finland_2():
    preset = load_preset("finland-2")
    ids = {source.id for source in preset.sources}

    assert preset.name == "finland-2"
    assert preset.region == "finland"
    assert preset.target_size_gb == 2
    assert "wikem" in ids
    assert "quick-guides-medicine" in ids
    assert "fimea" in ids
    assert "joukahainen" not in ids
    assert "stackexchange-survival" not in ids
    assert "wikipedia-en-nopic" not in ids


def test_finland_2_estimated_size_leaves_headroom():
    preset = load_preset("finland-2")
    total_size = sum(source.size_gb for source in preset.sources)

    assert total_size < 1.7
```

- [ ] **Step 2: Write the failing wizard tests**

```python
def test_presets_for_space_includes_finland_2():
    result = presets_for_space(10, region="finland")
    names = [name for name, _, _ in result]

    assert "finland-2" in names


def test_presets_for_space_small_disk_marks_finland_2_as_fitting():
    result = presets_for_space(3, region="finland")
    fitting = [name for name, _, fits in result if fits]

    assert "finland-2" in fitting
    assert "finland-32" not in fitting
```

- [ ] **Step 3: Run the targeted tests to verify they fail**

Run:

```bash
uv run pytest -q tests/test_presets.py tests/test_wizard.py -k "finland_2 or includes_finland_2 or small_disk_marks_finland_2"
```

Expected: FAIL because `finland-2` does not exist yet.

### Task 2: Add the new preset YAML

**Files:**
- Create: `presets/finland-2.yaml`
- Test: `tests/test_presets.py`

- [ ] **Step 1: Add the preset file**

```yaml
name: finland-2
description: Emergency field kit for Finland use on a 2GB stick or phone-copyable archive
target_size_gb: 2
region: finland
sources:
- wikem
- who-basic-emergency-care
- zimgit-medicine
- fas-military-medicine
- quick-guides-medicine
- zimgit-water
- zimgit-food-preparation
- based-cooking
- grimgrains
- usda-nutrition
- zimgit-knots
- cd3wd
- fimea
- kiwix-serve
- 7z
- sqlite3
- toybox
```

- [ ] **Step 2: Run the targeted tests to verify they pass**

Run:

```bash
uv run pytest -q tests/test_presets.py tests/test_wizard.py -k "finland_2 or includes_finland_2 or small_disk_marks_finland_2"
```

Expected: PASS

### Task 3: Update preset-family assertions and user docs

**Files:**
- Modify: `tests/test_presets.py`
- Modify: `README.md`

- [ ] **Step 1: Update preset family coverage tests**

Add `finland-2` to the Finland-family expectations:

```python
def test_list_presets_contains_finland_and_default_families():
    presets = list_presets()
    assert "finland-2" in presets
    assert "finland-32" in presets
    assert "finland-1tb" in presets
    assert "default-32" in presets
    assert "default-128" in presets
```

- [ ] **Step 2: Update README preset tables and quick description**

Document `finland-2` in the Finland preset table, describing it as the emergency field kit tier, and update any wording that implies the visible Finland ladder starts at `32 GB`.

- [ ] **Step 3: Run the broader verification set**

Run:

```bash
uv run pytest -q tests/test_presets.py tests/test_wizard.py tests/test_commands.py tests/test_manifest.py
```

Expected: PASS

Run:

```bash
uv run python -m py_compile src/svalbard/presets.py src/svalbard/wizard.py
```

Expected: PASS
