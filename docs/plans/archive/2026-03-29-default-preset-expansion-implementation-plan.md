# Default Preset Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the `default-*` preset family to a full mirrored size ladder with region-neutral large tiers, including GGUF/LLM content at `default-512+`.

**Architecture:** Keep the current preset schema and command flow unchanged. Implement the feature as a catalog expansion: add the missing `default-256`, `default-512`, `default-1tb`, and `default-2tb` YAML files, derive them from the current Finland tiers while stripping localization-specific sources, and update tests and README to reflect the full region-neutral ladder.

**Tech Stack:** Python 3.12, PyYAML preset files, pytest, Click, Rich

---

## File Map

**Create:**
- `docs/plans/2026-03-29-default-preset-expansion-implementation-plan.md`
- `src/svalbard/presets/default-256.yaml`
- `src/svalbard/presets/default-512.yaml`
- `src/svalbard/presets/default-1tb.yaml`
- `src/svalbard/presets/default-2tb.yaml`

**Modify:**
- `tests/test_presets.py`
- `tests/test_wizard.py`
- `README.md`

### Task 1: Lock In the Expanded Default Ladder With Tests

**Files:**
- Modify: `tests/test_presets.py`
- Modify: `tests/test_wizard.py`

- [ ] **Step 1: Write the failing preset catalog tests**

```python
from svalbard.presets import list_presets, load_preset


def test_list_presets_contains_full_default_family():
    presets = list_presets()
    assert "default-32" in presets
    assert "default-64" in presets
    assert "default-128" in presets
    assert "default-256" in presets
    assert "default-512" in presets
    assert "default-1tb" in presets
    assert "default-2tb" in presets


def test_default_512_includes_llm_sources():
    preset = load_preset("default-512")
    ids = {source.id for source in preset.sources}
    assert "llama-3b" in ids
    assert "llama-server-binaries" in ids


def test_default_1tb_includes_large_llm_sources():
    preset = load_preset("default-1tb")
    ids = {source.id for source in preset.sources}
    assert "llama-70b" in ids
    assert "llama-server-binaries" in ids


def test_default_2tb_stays_region_neutral():
    preset = load_preset("default-2tb")
    ids = {source.id for source in preset.sources}
    assert "wikipedia-fi-all" not in ids
    assert "wiktionary-fi" not in ids
    assert "wikipedia-sv-all" not in ids
```

- [ ] **Step 2: Write the failing wizard coverage test**

```python
from svalbard.wizard import presets_for_space


def test_presets_for_space_default_region_includes_large_tiers():
    result = presets_for_space(2500, region="default")
    names = [name for name, _, _ in result]
    assert "default-256" in names
    assert "default-512" in names
    assert "default-1tb" in names
    assert "default-2tb" in names
```

- [ ] **Step 3: Run the targeted tests to verify they fail**

Run: `uv run --with pytest python -m pytest -q tests/test_presets.py tests/test_wizard.py -k "full_default_family or default_512_includes_llm_sources or default_1tb_includes_large_llm_sources or default_2tb_stays_region_neutral or default_region_includes_large_tiers"`

Expected: FAIL because the larger `default-*` files do not exist yet.

- [ ] **Step 4: Commit the test changes after implementation passes**

```bash
git add tests/test_presets.py tests/test_wizard.py
git commit -m "test: cover full default preset ladder"
```

### Task 2: Add the Missing `default-*` Preset Files

**Files:**
- Create: `src/svalbard/presets/default-256.yaml`
- Create: `src/svalbard/presets/default-512.yaml`
- Create: `src/svalbard/presets/default-1tb.yaml`
- Create: `src/svalbard/presets/default-2tb.yaml`

- [ ] **Step 1: Implement `default-256.yaml` as the region-neutral counterpart to `finland-256`**

```yaml
name: default-256
description: Region-neutral expanded reference and practical archive for a 256GB drive
target_size_gb: 256
region: default

sources:
  - id: wikipedia-en-maxi
    type: zim
    group: reference
    tags: [general-reference, medicine, agriculture, electronics, mechanical, chemistry, physics, biology, history]
    depth: comprehensive
    size_gb: 100.0
    url_pattern: "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_maxi_{date}.zim"
    description: English Wikipedia with pictures
```

Implementation notes:
- Keep English-only reference sources.
- Keep technical/practical Stack Exchanges already present in `finland-256`.
- Exclude Finnish, Swedish, Norwegian localization sources.

- [ ] **Step 2: Implement `default-512.yaml` with the first region-neutral LLM tier**

```yaml
  - id: llama-3b
    type: gguf
    group: models
    tags: [computing, general-reference, education-pedagogy]
    depth: overview
    size_gb: 2.0
    url: "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf"
    description: Llama 3.2 3B Instruct (Q4_K_M) — small portable LLM

  - id: llama-server-binaries
    type: binary
    group: tools
    tags: [computing]
    depth: reference-only
    size_gb: 0.005
    url: "https://github.com/ggerganov/llama.cpp/releases/latest/download/llama-server-linux-x86_64"
    description: llama.cpp server binary for serving GGUF models
```

- [ ] **Step 3: Implement `default-1tb.yaml` with the larger LLM tier**

```yaml
  - id: llama-70b
    type: gguf
    group: models
    tags: [computing, general-reference, education-pedagogy]
    depth: comprehensive
    size_gb: 40.0
    url: "https://huggingface.co/bartowski/Meta-Llama-3.1-70B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-70B-Instruct-Q4_K_M.gguf"
    description: Llama 3.1 70B Instruct (Q4_K_M) — large capable LLM
```

- [ ] **Step 4: Implement `default-2tb.yaml` as the maximum region-neutral archive**

```yaml
  - id: llama-70b
    type: gguf
    group: models
    tags: [computing, general-reference, education-pedagogy]
    depth: comprehensive
    size_gb: 40.0
    url: "https://huggingface.co/bartowski/Meta-Llama-3.1-70B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-70B-Instruct-Q4_K_M.gguf"
    description: Llama 3.1 70B Instruct (Q4_K_M) — large capable LLM

  - id: llama-3b
    type: gguf
    group: models
    tags: [computing, general-reference]
    depth: overview
    size_gb: 2.0
    url: "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf"
    description: Llama 3.2 3B Instruct (Q4_K_M) — fast small LLM
```

- [ ] **Step 5: Run the targeted tests to verify the new ladder passes**

Run: `uv run --with pytest python -m pytest -q tests/test_presets.py tests/test_wizard.py -k "full_default_family or default_512_includes_llm_sources or default_1tb_includes_large_llm_sources or default_2tb_stays_region_neutral or default_region_includes_large_tiers"`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add src/svalbard/presets/default-256.yaml src/svalbard/presets/default-512.yaml src/svalbard/presets/default-1tb.yaml src/svalbard/presets/default-2tb.yaml tests/test_presets.py tests/test_wizard.py
git commit -m "feat: expand default preset family to large tiers"
```

### Task 3: Update README to Reflect the Full Default Family

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the README preset tables**

```markdown
| `default-256` | English Wikipedia with pictures | English Wiktionary, Project Gutenberg | Expanded practical + technical Stack Exchanges | Wikibooks, Khan Academy | CyberChef, Kiwix tools |
| `default-512` | English Wikipedia with pictures | English Wiktionary, Project Gutenberg | Expanded practical + technical Stack Exchanges | Wikibooks, Khan Academy | CyberChef, Kiwix tools, Llama 3.2 3B |
| `default-1tb` | English Wikipedia with pictures | English Wiktionary, Project Gutenberg | Broad practical + technical Stack Exchanges | Wikibooks, Khan Academy | CyberChef, Kiwix tools, Llama 3.1 70B |
| `default-2tb` | Full English-heavy reference archive | Gutenberg + broader technical references | Maximum practical + technical Stack Exchanges | Wikibooks, Khan Academy | CyberChef, Kiwix tools, Llama 70B + 3B |
```

Also add one explicit note:

```markdown
LLM downloads begin at the `512 GB` tiers in both preset families.
```

- [ ] **Step 2: Run the full test suite to make sure the catalog expansion did not regress anything**

Run: `uv run --with pytest python -m pytest -q`

Expected: PASS with all tests green.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document expanded default preset family"
```

## Self-Review

- **Spec coverage:** This plan covers the mirrored default ladder, region-neutral content rules, `default-512+` LLM policy, wizard visibility, and README documentation.
- **Placeholder scan:** No `TODO`/`TBD` placeholders remain.
- **Type consistency:** The plan only adds new preset YAML files and tests against existing preset APIs (`list_presets`, `load_preset`, `presets_for_space`), so there are no new code-interface mismatches introduced by the plan itself.
