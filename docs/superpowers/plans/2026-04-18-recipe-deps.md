# Recipe Dependencies Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Auto-resolve recipe dependencies so selecting a GGUF model pulls in llama-server, selecting pmtiles pulls in go-pmtiles+maplibre-vendor+dufs, etc.

**Architecture:** Hybrid dep declaration (type-level defaults in YAML + recipe-level `deps:` override). Python resolves deps for CLI/headless. Go resolves deps for interactive TUI toggling. Both read the same `dep-defaults.yaml`. `catalog.Item` carries dep metadata; TUI uses `AutoDepIDs` map as single source of truth for rendering.

**Tech Stack:** Python 3.12 (presets/models), Go + Bubble Tea (TUI), YAML (dep-defaults + recipe schemas)

---

### Task 1: Create dep-defaults.yaml

**Files:**
- Create: `recipes/dep-defaults.yaml`

**Step 1: Create the defaults file**

```yaml
# Type-level dependency defaults.
# When a recipe has no explicit `deps:` field, these apply based on its `type:`.
# Recipe-level `deps:` overrides (replaces) the type default entirely.
# Explicit `deps: []` means "no deps despite my type."
gguf: [llama-server]
pmtiles: [go-pmtiles, maplibre-vendor, dufs]
zim: [kiwix-serve]
```

**Step 2: Verify all referenced IDs exist as recipes**

Run: `for id in llama-server go-pmtiles maplibre-vendor dufs kiwix-serve; do ls recipes/tools/$id.yaml 2>/dev/null || ls recipes/apps/$id.yaml 2>/dev/null || echo "MISSING: $id"; done`
Expected: all files found, no MISSING lines

**Step 3: Commit**

```bash
git add recipes/dep-defaults.yaml
git commit -m "feat: add type-level dependency defaults for recipes"
```

---

### Task 2: Add auto_dep field to Python Source model

**Files:**
- Modify: `src/svalbard/models.py:15-33` (Source dataclass)

**Step 1: Add the field**

Add only `auto_dep: bool = False` to the `Source` dataclass, after `size_bytes`. Do NOT add a `deps` field — resolution reads deps from the raw recipe dict, not from Source objects.

```python
@dataclass
class Source:
    id: str
    type: str
    display_group: str = ""
    menu: dict = field(default_factory=dict)
    action: dict = field(default_factory=dict)
    tags: list[str] = field(default_factory=list)
    depth: str = "comprehensive"
    size_gb: float = 0.0
    url: str = ""
    url_pattern: str = ""
    platforms: dict[str, str] = field(default_factory=dict)
    description: str = ""
    sha256: str = ""
    license: License | None = None
    strategy: str = "download"
    build: dict = field(default_factory=dict)
    path: str = ""
    size_bytes: int = 0
    auto_dep: bool = False

    def __post_init__(self) -> None:
        if self.size_bytes and not self.size_gb:
            self.size_gb = self.size_bytes / 1e9
```

**Step 2: Verify existing tests still pass**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && python -m pytest src/tests/ -v`
Expected: PASS (new field has default, no breakage)

**Step 3: Commit**

```bash
git add src/svalbard/models.py
git commit -m "feat: add auto_dep field to Source model"
```

---

### Task 3: Implement dep resolution in presets.py

**Files:**
- Modify: `src/svalbard/presets.py`
- Create: `src/tests/test_deps.py`

**Step 1: Write failing tests**

Create `src/tests/test_deps.py`:

```python
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
    """GGUF model with no explicit deps gets llama-server from type default."""
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
    """Recipe with explicit deps: replaces type default."""
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["custom-model"])]

    resolved = _resolve_deps(sources, recipe_index, dep_defaults)

    ids = {s.id for s in resolved}
    assert "llama-server" in ids
    assert "kiwix-serve" in ids  # custom dep


def test_explicit_empty_deps(tmp_recipes, dep_defaults):
    """Recipe with deps: [] gets no auto-deps."""
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["standalone-model"])]

    resolved = _resolve_deps(sources, recipe_index, dep_defaults)

    ids = {s.id for s in resolved}
    assert ids == {"standalone-model"}


def test_transitive_deps(tmp_recipes, dep_defaults):
    """Deps that themselves have deps are resolved transitively."""
    (tmp_recipes / "tools" / "llama-server.yaml").write_text(yaml.dump({
        "id": "llama-server", "type": "binary", "size_gb": 0.04,
        "deps": ["dufs"],
    }))
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["test-model"])]

    resolved = _resolve_deps(sources, recipe_index, dep_defaults)

    ids = {s.id for s in resolved}
    assert "dufs" in ids  # transitive: test-model -> llama-server -> dufs


def test_cycle_detection(tmp_recipes, dep_defaults):
    """Circular deps don't cause infinite loop."""
    (tmp_recipes / "tools" / "llama-server.yaml").write_text(yaml.dump({
        "id": "llama-server", "type": "binary", "size_gb": 0.04,
        "deps": ["test-model"],  # circular!
    }))
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["test-model"])]

    resolved = _resolve_deps(sources, recipe_index, dep_defaults)
    ids = {s.id for s in resolved}
    assert "test-model" in ids
    assert "llama-server" in ids


def test_already_selected_dep_not_marked_auto(tmp_recipes, dep_defaults):
    """If user explicitly selected a dep, it stays non-auto_dep."""
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [
        _source_from_recipe(recipe_index["test-model"]),
        _source_from_recipe(recipe_index["llama-server"]),
    ]

    resolved = _resolve_deps(sources, recipe_index, dep_defaults)

    llama = next(s for s in resolved if s.id == "llama-server")
    assert llama.auto_dep is False  # user picked it


def test_zim_gets_kiwix(tmp_recipes, dep_defaults):
    """ZIM type pulls in kiwix-serve."""
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["wiki"])]

    resolved = _resolve_deps(sources, recipe_index, dep_defaults)

    ids = {s.id for s in resolved}
    assert "kiwix-serve" in ids


def test_no_deps_for_unknown_type(tmp_recipes, dep_defaults):
    """Binary type with no default and no explicit deps gets nothing."""
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["dufs"])]

    resolved = _resolve_deps(sources, recipe_index, dep_defaults)

    ids = {s.id for s in resolved}
    assert ids == {"dufs"}


def test_preserves_existing_sources(tmp_recipes, dep_defaults):
    """Existing Source objects are preserved, not rebuilt from recipe index."""
    recipe_index = _build_recipe_index([tmp_recipes])
    original = _source_from_recipe(recipe_index["test-model"])
    original.size_gb = 99.9  # simulate a preset override

    resolved = _resolve_deps([original], recipe_index, dep_defaults)

    model = next(s for s in resolved if s.id == "test-model")
    assert model.size_gb == 99.9  # override preserved
    assert model is original  # same object


def test_warns_on_missing_dep_id(tmp_recipes, tmp_path, caplog):
    """Missing dep ID produces a warning."""
    defaults = _load_dep_defaults(tmp_path / "nonexistent.yaml")
    # Override defaults with a bad ID
    defaults = {"gguf": ["nonexistent-tool"]}
    recipe_index = _build_recipe_index([tmp_recipes])
    sources = [_source_from_recipe(recipe_index["test-model"])]

    import logging
    with caplog.at_level(logging.WARNING):
        resolved = _resolve_deps(sources, recipe_index, defaults)

    assert "nonexistent-tool" in caplog.text
    # Model is still resolved, just without the missing dep
    ids = {s.id for s in resolved}
    assert "test-model" in ids
    assert "nonexistent-tool" not in ids
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && python -m pytest src/tests/test_deps.py -v`
Expected: FAIL — `_load_dep_defaults` and `_resolve_deps` don't exist yet

**Step 3: Implement _load_dep_defaults and _resolve_deps in presets.py**

Add to `presets.py`, after the existing `_build_recipe_index` function:

```python
import logging

_log = logging.getLogger(__name__)


def _load_dep_defaults(path: Path | None = None) -> dict[str, list[str]]:
    """Load type-level dependency defaults from YAML."""
    if path is None:
        path = _PROJECT_ROOT / "recipes" / "dep-defaults.yaml"
    if not path.exists():
        return {}
    with open(path) as f:
        data = yaml.safe_load(f)
    return data or {}


def _get_deps_for_recipe(recipe: dict, defaults: dict[str, list[str]]) -> list[str]:
    """Return deps for a recipe: explicit deps field > type default > empty."""
    if "deps" in recipe:
        return recipe["deps"]
    return defaults.get(recipe.get("type", ""), [])


def _resolve_deps(
    sources: list[Source],
    recipe_index: dict[str, dict],
    defaults: dict[str, list[str]],
) -> list[Source]:
    """Resolve deps transitively, preserving existing Source objects.

    Sources already in the list are kept as-is (preserving any overrides).
    Only auto-dep additions are constructed from the recipe index.
    Missing dep IDs produce a warning.
    """
    existing = {s.id: s for s in sources}
    resolved_ids: set[str] = set()
    result: list[Source] = []

    def _visit(source_id: str, is_auto: bool) -> None:
        if source_id in resolved_ids:
            return
        resolved_ids.add(source_id)

        if source_id in existing:
            # Preserve the already-built Source (may have preset overrides)
            src = existing[source_id]
            src.auto_dep = False  # user-selected
            result.append(src)
        elif is_auto:
            recipe = recipe_index.get(source_id)
            if recipe is None:
                _log.warning("Dep '%s' not found in recipe index — skipping", source_id)
                return
            src = _source_from_recipe(recipe)
            src.auto_dep = True
            result.append(src)
        else:
            return  # not in sources and not auto — skip

        recipe = recipe_index.get(source_id, {})
        for dep_id in _get_deps_for_recipe(recipe, defaults):
            _visit(dep_id, is_auto=True)

    for src in sources:
        _visit(src.id, is_auto=False)

    return result
```

Key differences from original plan:
- Takes `list[Source]` not `list[str]` — preserves existing Source objects with overrides
- Warns on missing dep IDs instead of silently skipping
- Existing sources are kept as-is, only new auto-deps are constructed from recipe index

**Step 4: Run tests to verify they pass**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && python -m pytest src/tests/test_deps.py -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add src/svalbard/presets.py src/tests/test_deps.py
git commit -m "feat: implement recipe dep resolution with transitive support"
```

---

### Task 4: Integrate dep resolution into parse_preset (top-level only)

**Files:**
- Modify: `src/svalbard/presets.py:73-158` (parse_preset function)

**Step 1: Write failing test**

Add to `src/tests/test_deps.py`:

```python
from svalbard.presets import parse_preset


def test_parse_preset_resolves_deps(tmp_path, tmp_recipes):
    """parse_preset should auto-add deps at the top level."""
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
    """Dep resolution should not destroy preset-level overrides."""
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
    assert model.size_gb == 99.9  # override preserved through dep resolution


def test_extends_removal_honored_before_deps(tmp_path, tmp_recipes):
    """Removing a source via -source should not leave orphan deps."""
    dep_file = tmp_path / "dep-defaults.yaml"
    dep_file.write_text(yaml.dump({"gguf": ["llama-server"]}))
    defaults = _load_dep_defaults(dep_file)
    recipe_index = _build_recipe_index([tmp_recipes])

    base_file = tmp_path / "base.yaml"
    base_file.write_text(yaml.dump({
        "name": "base",
        "description": "base",
        "target_size_gb": 10,
        "sources": ["test-model", "wiki"],
    }))

    child_file = tmp_path / "child.yaml"
    child_file.write_text(yaml.dump({
        "name": "child",
        "description": "child",
        "extends": str(base_file),
        "target_size_gb": 10,
        "sources": ["- test-model"],  # remove the model
    }))

    # Need to make resolve_preset_path work for this test; use parse_preset directly
    preset = parse_preset(child_file, recipe_index=recipe_index, dep_defaults=defaults)

    ids = {s.id for s in preset.sources}
    assert "test-model" not in ids
    # llama-server should not be present since nothing needs it
    assert "llama-server" not in ids
    # wiki should still be present with its dep
    assert "wiki" in ids
    assert "kiwix-serve" in ids
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && python -m pytest src/tests/test_deps.py::test_parse_preset_resolves_deps -v`
Expected: FAIL — parse_preset doesn't accept dep_defaults yet

**Step 3: Modify parse_preset**

Add `dep_defaults` parameter (data, not a path). Run resolution only at the top level (when `_seen` is not provided by caller):

```python
def parse_preset(
    path: Path,
    recipe_index: dict[str, dict] | None = None,
    _seen: set[str] | None = None,
    dep_defaults: dict[str, list[str]] | None = None,
) -> Preset:
```

At the end, just before `return Preset(...)`, add dep resolution only if this is the top-level call:

```python
    # ── Resolve dependencies (top-level only) ───────────────────────────
    if dep_defaults is not None:
        sources = _resolve_deps(sources, recipe_index, dep_defaults)
```

Do NOT pass `dep_defaults` down to recursive `parse_preset` calls in the extends chain. This ensures deps are resolved once, after all extends/merges/removals are complete.

**Step 4: Run tests**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && python -m pytest src/tests/test_deps.py -v`
Expected: all PASS

**Step 5: Run full test suite**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && python -m pytest src/tests/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add src/svalbard/presets.py src/tests/test_deps.py
git commit -m "feat: integrate dep resolution into parse_preset (top-level only)"
```

---

### Task 5: Add dep metadata to catalog.Item and dep resolution to catalog package

**Files:**
- Modify: `host-cli/internal/catalog/catalog.go` (Item struct + ResolveDeps method)
- Create: `host-cli/internal/catalog/deps.go`
- Create: `host-cli/internal/catalog/deps_test.go`

**Note:** Dep resolution logic lives in `catalog/` (business logic), not `tui/` (presentation). The `catalog` package already has the recipe index and YAML parsing.

**Step 1: Add Deps and HasExplicitDeps to catalog.Item**

Find the `Item` struct in `catalog.go` and add:

```go
type Item struct {
    // ... existing fields ...
    Deps            []string `yaml:"deps"`
    HasExplicitDeps bool     `yaml:"-"` // set during YAML unmarshaling
}
```

In the YAML unmarshaling code, after unmarshaling, check if the `deps` key was present in the raw YAML to set `HasExplicitDeps`. Use `yaml.Node` or a custom unmarshaler.

**Step 2: Create deps.go with DepDefaults and ResolveDeps**

Create `host-cli/internal/catalog/deps.go`:

```go
package catalog

import (
    "log/slog"
    "os"

    "gopkg.in/yaml.v3"
)

// DepDefaults maps recipe type to default dep IDs.
type DepDefaults map[string][]string

// LoadDepDefaults reads dep-defaults.yaml from the given path.
func LoadDepDefaults(path string) (DepDefaults, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return DepDefaults{}, nil
        }
        return nil, err
    }
    var defaults DepDefaults
    if err := yaml.Unmarshal(data, &defaults); err != nil {
        return nil, err
    }
    return defaults, nil
}

// depsForItem returns the dep IDs for an item: explicit deps > type default > nil.
func depsForItem(item Item, defaults DepDefaults) []string {
    if item.HasExplicitDeps {
        return item.Deps
    }
    return defaults[item.Type]
}

// ResolveDeps takes user-selected IDs and returns auto-dep IDs.
// The catalog's recipe index is used for transitive lookup.
func (c *Catalog) ResolveDeps(selectedIDs map[string]bool, defaults DepDefaults) map[string]bool {
    autoDeps := make(map[string]bool)
    visited := make(map[string]bool)

    var visit func(id string, isAuto bool)
    visit = func(id string, isAuto bool) {
        if visited[id] {
            return
        }
        item, ok := c.recipes[id]
        if !ok {
            if isAuto {
                slog.Warn("dep not found in recipe index", "dep", id)
            }
            return
        }
        visited[id] = true
        if isAuto && !selectedIDs[id] {
            autoDeps[id] = true
        }
        for _, depID := range depsForItem(item, defaults) {
            visit(depID, true)
        }
    }

    for id := range selectedIDs {
        visit(id, false)
    }

    return autoDeps
}
```

**Step 3: Write tests**

Create `host-cli/internal/catalog/deps_test.go`:

```go
package catalog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadDepDefaults(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "dep-defaults.yaml")
    os.WriteFile(path, []byte("gguf: [llama-server]\nzim: [kiwix-serve]\n"), 0644)

    defaults, err := LoadDepDefaults(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(defaults["gguf"]) != 1 || defaults["gguf"][0] != "llama-server" {
        t.Errorf("expected [llama-server], got %v", defaults["gguf"])
    }
}

func TestLoadDepDefaultsMissing(t *testing.T) {
    defaults, err := LoadDepDefaults("/nonexistent/path.yaml")
    if err != nil {
        t.Fatal(err)
    }
    if len(defaults) != 0 {
        t.Errorf("expected empty, got %v", defaults)
    }
}

func TestResolveDepsTypeDefault(t *testing.T) {
    c := &Catalog{recipes: map[string]Item{
        "my-model":     {Type: "gguf"},
        "llama-server": {Type: "binary"},
    }}
    defaults := DepDefaults{"gguf": {"llama-server"}}
    selected := map[string]bool{"my-model": true}

    auto := c.ResolveDeps(selected, defaults)

    if !auto["llama-server"] {
        t.Error("llama-server should be auto-dep")
    }
    if auto["my-model"] {
        t.Error("my-model should not be auto-dep")
    }
}

func TestResolveDepsExplicitOverride(t *testing.T) {
    c := &Catalog{recipes: map[string]Item{
        "custom-model": {Type: "gguf", Deps: []string{"other-tool"}, HasExplicitDeps: true},
        "llama-server": {Type: "binary"},
        "other-tool":   {Type: "binary"},
    }}
    defaults := DepDefaults{"gguf": {"llama-server"}}
    selected := map[string]bool{"custom-model": true}

    auto := c.ResolveDeps(selected, defaults)

    if auto["llama-server"] {
        t.Error("llama-server should NOT be pulled in")
    }
    if !auto["other-tool"] {
        t.Error("other-tool should be auto-dep")
    }
}

func TestResolveDepsExplicitEmpty(t *testing.T) {
    c := &Catalog{recipes: map[string]Item{
        "standalone": {Type: "gguf", Deps: []string{}, HasExplicitDeps: true},
    }}
    defaults := DepDefaults{"gguf": {"llama-server"}}
    selected := map[string]bool{"standalone": true}

    auto := c.ResolveDeps(selected, defaults)

    if len(auto) != 0 {
        t.Errorf("expected no auto-deps, got %v", auto)
    }
}

func TestResolveDepsTransitive(t *testing.T) {
    c := &Catalog{recipes: map[string]Item{
        "my-model":     {Type: "gguf"},
        "llama-server": {Type: "binary", Deps: []string{"zstd"}, HasExplicitDeps: true},
        "zstd":         {Type: "binary"},
    }}
    defaults := DepDefaults{"gguf": {"llama-server"}}
    selected := map[string]bool{"my-model": true}

    auto := c.ResolveDeps(selected, defaults)

    if !auto["llama-server"] || !auto["zstd"] {
        t.Errorf("expected transitive deps, got %v", auto)
    }
}

func TestResolveDepsCycle(t *testing.T) {
    c := &Catalog{recipes: map[string]Item{
        "my-model":     {Type: "gguf"},
        "llama-server": {Type: "binary", Deps: []string{"my-model"}, HasExplicitDeps: true},
    }}
    defaults := DepDefaults{"gguf": {"llama-server"}}
    selected := map[string]bool{"my-model": true}

    auto := c.ResolveDeps(selected, defaults)
    if !auto["llama-server"] {
        t.Error("llama-server should be auto-dep despite cycle")
    }
}

func TestResolveDepsUserSelectedNotAuto(t *testing.T) {
    c := &Catalog{recipes: map[string]Item{
        "my-model":     {Type: "gguf"},
        "llama-server": {Type: "binary"},
    }}
    defaults := DepDefaults{"gguf": {"llama-server"}}
    selected := map[string]bool{"my-model": true, "llama-server": true}

    auto := c.ResolveDeps(selected, defaults)
    if auto["llama-server"] {
        t.Error("user-selected should not be auto-dep")
    }
}
```

**Step 4: Run tests**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && go test ./host-cli/internal/catalog/ -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add host-cli/internal/catalog/
git commit -m "feat: add dep resolution to catalog package"
```

---

### Task 6: Add AutoDepIDs and UserCheckedIDs to TreePicker

**Files:**
- Modify: `tui/treepicker.go`

**Step 1: Add tracking maps to TreePicker**

```go
type TreePicker struct {
    // ... existing fields ...
    AutoDepIDs    map[string]bool // IDs auto-included as deps (source of truth for rendering)
    UserCheckedIDs map[string]bool // IDs the user explicitly toggled on
}
```

Initialize in `NewTreePicker`:

```go
tp := TreePicker{
    // ... existing ...
    AutoDepIDs:     make(map[string]bool),
    UserCheckedIDs: make(map[string]bool),
}

// Seed UserCheckedIDs from initial checked state
for id, v := range cfg.CheckedIDs {
    if v {
        tp.UserCheckedIDs[id] = true
    }
}
```

**Step 2: Modify ToggleAtCursor — guard auto-deps, track user intent**

In the `RowItem` case:

```go
case RowItem:
    src := row.Source
    if tp.AutoDepIDs[src.ID] && !tp.UserCheckedIDs[src.ID] {
        return // can't uncheck a pure auto-dep
    }
    if tp.CheckedIDs[src.ID] {
        delete(tp.CheckedIDs, src.ID)
        delete(tp.UserCheckedIDs, src.ID)
    } else {
        tp.CheckedIDs[src.ID] = true
        tp.UserCheckedIDs[src.ID] = true
    }
```

In the `RowPack` case, mark all toggled sources as user-checked:

```go
case RowPack:
    pack := row.Pack
    checked, total := PackCheckState(pack, tp.CheckedIDs)
    if checked == total && total > 0 {
        for _, s := range pack.Sources {
            delete(tp.CheckedIDs, s.ID)
            delete(tp.UserCheckedIDs, s.ID)
        }
    } else {
        for _, s := range pack.Sources {
            tp.CheckedIDs[s.ID] = true
            tp.UserCheckedIDs[s.ID] = true
        }
    }
```

**Step 3: Add Toggled() method to distinguish toggle from navigation**

```go
// UpdateResult indicates what happened during Update.
type UpdateResult int

const (
    UpdateNone    UpdateResult = iota // key not handled
    UpdateNav                         // navigation only
    UpdateToggled                     // selection changed
)

// UpdateWithResult handles keys and returns what happened.
func (tp *TreePicker) UpdateWithResult(msg tea.KeyMsg) UpdateResult {
    switch {
    case tp.Keys.MoveUp.Matches(msg), tp.Keys.MoveDown.Matches(msg):
        // ... existing nav logic ...
        return UpdateNav
    case tp.Keys.Toggle.Matches(msg):
        if !tp.ReadOnly {
            tp.ToggleAtCursor()
        }
        return UpdateToggled
    // ... etc
    }
    return UpdateNone
}
```

Keep the existing `Update() bool` as a thin wrapper for backward compatibility.

**Step 4: Modify rendering to use AutoDepIDs instead of src.AutoDep**

In `RenderTree`, `RowItem` case:

```go
case RowItem:
    src := row.Source
    isAutoDep := tp.AutoDepIDs[src.ID]
    mark := "·"
    if tp.CheckedIDs[src.ID] {
        mark = "✓"
    }
    // ... build line ...
    if isAutoDep && !tp.UserCheckedIDs[src.ID] {
        line += "  ← dep"
    }
    if isCursor {
        if isAutoDep && !tp.UserCheckedIDs[src.ID] {
            b.WriteString(tp.Theme.SelectedMuted.Render(line))
        } else {
            b.WriteString(tp.Theme.Selected.Render(line))
        }
    } else if tp.CheckedIDs[src.ID] {
        if isAutoDep && !tp.UserCheckedIDs[src.ID] {
            b.WriteString(tp.Theme.Muted.Render(line))
        } else {
            b.WriteString(tp.Theme.Base.Render(line))
        }
    } else {
        b.WriteString(tp.Theme.Muted.Render(line))
    }
```

In `RenderDetail`, `RowItem` case, show auto-dep info:

```go
if tp.AutoDepIDs[src.ID] && !tp.UserCheckedIDs[src.ID] {
    info += "\n" + tp.Theme.Muted.Render("  auto-included — needed by another recipe")
}
```

**Step 5: Run tests**

Run: `cd /Users/pkronstrom/Projects/own/svalbard/tui && go test ./... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add tui/treepicker.go
git commit -m "feat: add AutoDepIDs/UserCheckedIDs tracking and dimmed dep rendering"
```

---

### Task 7: Wire dep resolution into pack picker

**Files:**
- Modify: `host-tui/internal/wizard/packpicker.go`
- Modify: `host-tui/internal/wizard/types.go`
- Modify: `host-tui/internal/wizard/packpicker_test.go`

**Step 1: Add dep resolver to WizardConfig and packPickerModel**

In `types.go`, add a function type:

```go
// DepResolver returns auto-dep IDs for a given set of selected IDs.
type DepResolver func(selectedIDs map[string]bool) map[string]bool

type WizardConfig struct {
    // ... existing fields ...
    ResolveDeps DepResolver // nil = no dep resolution
}
```

This avoids leaking `catalog.DepDefaults` and `catalog.Catalog` into the TUI layer. The host-cli constructs the closure.

**Step 2: Use config struct for packPickerModel**

In `packpicker.go`:

```go
type packPickerConfig struct {
    Groups     []tui.PackGroup
    CheckedIDs map[string]bool
    FreeGB     float64
    ResolveDeps func(map[string]bool) map[string]bool
}

type packPickerModel struct {
    picker      tui.TreePicker
    resolveDeps func(map[string]bool) map[string]bool
    width       int
    height      int
}

func newPackPicker(cfg packPickerConfig) packPickerModel {
    tp := tui.NewTreePicker(tui.TreePickerConfig{
        Groups:     cfg.Groups,
        CheckedIDs: cfg.CheckedIDs,
        FreeGB:     cfg.FreeGB,
        ShowAction: true,
    })

    m := packPickerModel{
        picker:      tp,
        resolveDeps: cfg.ResolveDeps,
    }
    m.recalcDeps()
    return m
}
```

**Step 3: Implement recalcDeps correctly**

```go
func (m *packPickerModel) recalcDeps() {
    if m.resolveDeps == nil {
        return
    }

    autoDeps := m.resolveDeps(m.picker.UserCheckedIDs)

    // Remove old auto-deps that are no longer needed
    // (only remove from CheckedIDs if not user-selected)
    for id := range m.picker.AutoDepIDs {
        if !autoDeps[id] && !m.picker.UserCheckedIDs[id] {
            delete(m.picker.CheckedIDs, id)
        }
    }

    // Add new auto-deps
    for id := range autoDeps {
        m.picker.CheckedIDs[id] = true
    }

    m.picker.AutoDepIDs = autoDeps
}
```

Key fix: only removes from `CheckedIDs` if NOT in `UserCheckedIDs`. User intent is preserved.

**Step 4: Call recalcDeps only after toggles**

```go
case tea.KeyMsg:
    result := m.picker.UpdateWithResult(msg)
    if result == tui.UpdateToggled {
        m.recalcDeps()
    }
    if result != tui.UpdateNone {
        return m, nil
    }
```

**Step 5: Update tests**

Update `packpicker_test.go` — change `newPackPicker` calls to use config struct:

```go
m := newPackPicker(packPickerConfig{
    Groups: samplePackGroups(),
    FreeGB: 64,
})
```

**Step 6: Run tests**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && go test ./host-tui/internal/wizard/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add host-tui/internal/wizard/packpicker.go host-tui/internal/wizard/types.go host-tui/internal/wizard/packpicker_test.go
git commit -m "feat: wire dep resolution into pack picker with user intent tracking"
```

---

### Task 8: Build DepResolver closure in host-cli

**Files:**
- Modify: the host-cli command that builds WizardConfig (find with `grep -rn "WizardConfig{" host-cli/`)

**Step 1: Find the WizardConfig construction site**

Run: `grep -rn "WizardConfig{" host-cli/`

**Step 2: Load dep-defaults and build resolver closure**

At the WizardConfig construction site:

```go
depDefaultsPath := filepath.Join(projectRoot, "recipes", "dep-defaults.yaml")
depDefaults, err := catalog.LoadDepDefaults(depDefaultsPath)
if err != nil {
    return fmt.Errorf("loading dep defaults: %w", err)
}

cfg := hosttui.WizardConfig{
    // ... existing fields ...
    ResolveDeps: func(selectedIDs map[string]bool) map[string]bool {
        return cat.ResolveDeps(selectedIDs, depDefaults)
    },
}
```

Where `cat` is the already-loaded `*catalog.Catalog` instance.

**Step 3: Run full build and test**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && go build ./... && go test ./... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add host-cli/
git commit -m "feat: build DepResolver closure from catalog and pass to wizard"
```

---

### Task 9: Orphan deps synthetic group

**Files:**
- Modify: `host-tui/internal/wizard/packpicker.go` or the pack-group builder in `host-cli/`

**Step 1: After recalcDeps, check for orphan auto-deps**

An orphan dep is an ID in `AutoDepIDs` that doesn't appear as a `PackSource` in any group. For each orphan, look up the recipe in the catalog and add it to a synthetic "Dependencies" group. This extends the existing "Other" group pattern in `root.go`.

**Step 2: Rebuild rows after adding the synthetic group**

Call `m.picker.RebuildRows()` if the group list changed.

**Step 3: Test and commit**

Run: `cd /Users/pkronstrom/Projects/own/svalbard && go test ./host-tui/... -v`

```bash
git add host-tui/ host-cli/
git commit -m "feat: synthetic Dependencies group for orphan auto-deps"
```

---

### Task 10: Embedding model prerequisite check in index flow

**Files:**
- Modify: `host-tui/internal/wizard/indexmodel.go`

**Step 1: Defer to follow-up**

This is a feature-level concern, not a recipe dep. Implement after the core dep system is stable.

**Step 2: Add TODO marker**

```go
// TODO(deps): Before indexing, check if an embedding model is present in the
// vault config. If missing, prompt user to add nomic-embed-text-v1.5 (140 MB).
// See: docs/superpowers/specs/2026-04-18-recipe-deps-design.md
```

**Step 3: Commit**

```bash
git add host-tui/internal/wizard/indexmodel.go
git commit -m "chore: add TODO for embedding model prerequisite check in index flow"
```
