# Interactive Pack Picker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the wizard's flat preset picker with an interactive TUI where users browse content by display_group → pack → recipe, toggle items on/off, and see a live size total.

**Architecture:** Rename recipe `group` → `display_group`. Add `display_group` to pack YAMLs. Build a picker TUI using `rich.live.Live` + `readchar` (same pattern as hawk-hooks). Wire into wizard and init commands.

**Tech Stack:** Python, rich, readchar, PyYAML

---

### Task 1: Rename `group` → `display_group` in models and presets

**Files:**
- Modify: `src/svalbard/models.py`
- Modify: `src/svalbard/presets.py`
- Modify: `src/svalbard/audit.py`
- Modify: `src/svalbard/taxonomy.py`
- Modify: `src/svalbard/commands.py`
- Modify: `src/tests/test_presets.py`
- Test: `uv run pytest src/tests/ -v`

**Step 1: Update the Source dataclass**

In `src/svalbard/models.py`, change line 18:

```python
# Before:
group: str = ""  # reference, practical, education, maps, regional, models, tools

# After:
display_group: str = ""  # UI section label: Reference, Practical, Maps, etc.
```

**Step 2: Update presets.py to parse both `group` and `display_group`**

In `src/svalbard/presets.py`, the `_source_from_recipe` function builds a
Source from a recipe dict. Recipe YAMLs will be migrated from `group:` to
`display_group:` but we need backwards compatibility during migration.

Add a migration shim in `_source_from_recipe`:

```python
def _source_from_recipe(recipe: dict) -> Source:
    """Build a Source from a recipe dict, converting nested structures."""
    kwargs = {k: v for k, v in recipe.items() if k in Source.__dataclass_fields__}
    # Migration: accept old 'group' field as display_group
    if "display_group" not in kwargs and "group" in recipe:
        kwargs["display_group"] = recipe["group"]
    if "license" in kwargs and isinstance(kwargs["license"], dict):
        kwargs["license"] = License(**kwargs["license"])
    return Source(**kwargs)
```

**Step 3: Update Preset dataclass to include display_group**

In `src/svalbard/models.py`, add `display_group` to Preset:

```python
@dataclass
class Preset:
    name: str
    description: str
    target_size_gb: float
    region: str
    kind: str = "preset"
    display_group: str = ""  # UI section for packs
    sources: list[Source] = field(default_factory=list)
```

**Step 4: Parse display_group from pack YAMLs**

In `src/svalbard/presets.py`, in `parse_preset`, pass `display_group`:

```python
return Preset(
    name=preset_name,
    description=data.get("description", ""),
    target_size_gb=data.get("target_size_gb", 0),
    region=data.get("region", ""),
    kind=data.get("kind", "preset"),
    display_group=data.get("display_group", data.get("group", "")),
    sources=sources,
)
```

**Step 5: Update taxonomy.py DomainCoverage**

The `DomainCoverage.group` field in taxonomy.py is a DIFFERENT concept
(taxonomy groups like "survival", "medical"). Rename it to `taxonomy_group`
to avoid confusion:

In `src/svalbard/taxonomy.py`:

```python
@dataclass
class DomainCoverage:
    domain: str
    taxonomy_group: str  # renamed from group
    score: int
    sources: list[str]
    depth_breakdown: dict[str, int]
```

Update `compute_coverage` and `audit.py` references accordingly:
- `taxonomy.py:50`: `group=group` → `taxonomy_group=group`
- `audit.py:117`: `c.group` → `c.taxonomy_group`

**Step 6: Update tests**

In `src/tests/test_presets.py`:
- Line 37: `assert tool.group == "tools"` → `assert tool.display_group == "tools"`
- Line 72: `source.group != "regional"` → `source.display_group != "regional"`
- Line 150: `source.group` → `source.display_group`

**Step 7: Run full test suite**

```bash
uv run pytest src/tests/ -v --tb=short
```

All tests must pass. The migration shim means old recipe YAMLs (still using
`group:`) work without changes.

**Step 8: Commit**

```bash
git add src/svalbard/models.py src/svalbard/presets.py src/svalbard/audit.py \
        src/svalbard/taxonomy.py src/svalbard/commands.py src/tests/
git commit -m "refactor: rename group → display_group in Source model

Adds migration shim in _source_from_recipe to accept old 'group' field.
Renames DomainCoverage.group → taxonomy_group to avoid confusion."
```

---

### Task 2: Rename `group:` → `display_group:` in all recipe YAMLs

**Files:**
- Modify: all `recipes/**/*.yaml` files (~120 files)

**Step 1: Bulk rename with sed**

```bash
find recipes -name '*.yaml' -exec sed -i '' 's/^group: /display_group: /' {} +
```

**Step 2: Verify no `group:` remains in recipes**

```bash
grep -r '^group:' recipes/
# Should return nothing
```

**Step 3: Verify display_group is present**

```bash
grep -c '^display_group:' recipes/**/*.yaml | tail -5
```

**Step 4: Run tests to confirm migration shim isn't needed anymore**

```bash
uv run pytest src/tests/ -v --tb=short
```

**Step 5: Commit**

```bash
git add recipes/
git commit -m "chore: rename group → display_group in all recipe YAMLs"
```

---

### Task 3: Add `display_group` to pack YAMLs

**Files:**
- Modify: `presets/packs/core.yaml`
- Modify: `presets/packs/fi-survival.yaml`
- Modify: `presets/packs/fi-maps.yaml`
- Modify: `presets/packs/fi-reference.yaml`
- Modify: `presets/packs/tools-base.yaml`
- Modify: `presets/packs/computing.yaml`
- Modify: `presets/packs/engineering.yaml`
- Modify: `presets/packs/sciences.yaml`
- Modify: `presets/packs/survival-medical.yaml`
- Modify: `presets/packs/communications.yaml`
- Modify: `presets/packs/homesteading.yaml`
- Modify: `presets/packs/embedded/esp32-dev.yaml`

**Step 1: Add display_group to each pack**

| Pack | display_group |
|------|--------------|
| core | Core |
| fi-survival | Survival & Medical |
| survival-medical | Survival & Medical |
| fi-maps | Maps & Geodata |
| fi-reference | Reference |
| tools-base | Tools |
| computing | Computing & Dev |
| engineering | Engineering & Making |
| sciences | Science & Education |
| communications | Communications |
| homesteading | Homesteading |
| embedded/esp32-dev | Computing & Dev |

Add `display_group: <value>` after the `kind:` line in each pack YAML.

Example for fi-survival.yaml:

```yaml
name: fi-survival
kind: pack
display_group: Survival & Medical
description: Finnish survival essentials — medical, food, water, shelter, practical skills
```

**Step 2: Run tests**

```bash
uv run pytest src/tests/ -v --tb=short
```

**Step 3: Commit**

```bash
git add presets/packs/
git commit -m "feat(packs): add display_group for picker UI sections"
```

---

### Task 4: Add readchar dependency

**Files:**
- Modify: `pyproject.toml`

**Step 1: Add readchar to dependencies**

```toml
dependencies = [
    "click>=8.1",
    "rich>=13.0",
    "pyyaml>=6.0",
    "httpx>=0.27",
    "libzim>=3.1",
    "readchar>=4.0",
]
```

**Step 2: Sync**

```bash
uv sync
```

**Step 3: Verify import works**

```bash
uv run python -c "import readchar; print(readchar.__version__)"
```

**Step 4: Commit**

```bash
git add pyproject.toml uv.lock
git commit -m "chore: add readchar dependency for interactive picker"
```

---

### Task 5: Build the pack picker tree builder

This is the data layer — builds the tree structure the UI will render.

**Files:**
- Create: `src/svalbard/picker.py`
- Create: `src/tests/test_picker.py`

**Step 1: Write tests**

```python
# src/tests/test_picker.py
from svalbard.models import Source, Preset
from svalbard.picker import build_picker_tree, PickerGroup, PickerPack


def _source(id: str, display_group: str = "", size_gb: float = 1.0, **kw):
    return Source(id=id, type="zim", display_group=display_group, size_gb=size_gb, **kw)


def test_build_tree_from_packs():
    """Packs with display_group form the tree structure."""
    packs = [
        Preset(
            name="fi-maps", description="Finnish maps", target_size_gb=2,
            region="finland", kind="pack", display_group="Maps",
            sources=[_source("osm-finland"), _source("nature-reserves")],
        ),
        Preset(
            name="tools-base", description="Core tools", target_size_gb=1,
            region="", kind="pack", display_group="Tools",
            sources=[_source("kiwix-serve", size_gb=0.2)],
        ),
    ]
    tree = build_picker_tree(packs=packs)
    assert len(tree) == 2
    assert tree[0].display_group == "Maps"
    assert len(tree[0].packs) == 1
    assert tree[0].packs[0].name == "fi-maps"
    assert len(tree[0].packs[0].sources) == 2


def test_build_tree_groups_packs_by_display_group():
    """Multiple packs with same display_group appear under one group."""
    packs = [
        Preset(name="fi-survival", description="", target_size_gb=5,
               region="finland", kind="pack", display_group="Survival",
               sources=[_source("wikimed")]),
        Preset(name="survival-medical", description="", target_size_gb=3,
               region="", kind="pack", display_group="Survival",
               sources=[_source("who-emergency")]),
    ]
    tree = build_picker_tree(packs=packs)
    assert len(tree) == 1
    assert tree[0].display_group == "Survival"
    assert len(tree[0].packs) == 2


def test_build_tree_deduplicates_sources():
    """Same source in multiple packs only counted once in total size."""
    shared = _source("wikimed", size_gb=0.8)
    packs = [
        Preset(name="p1", description="", target_size_gb=1,
               region="", kind="pack", display_group="A",
               sources=[shared, _source("other", size_gb=1.0)]),
        Preset(name="p2", description="", target_size_gb=1,
               region="", kind="pack", display_group="B",
               sources=[_source("wikimed", size_gb=0.8)]),
    ]
    tree = build_picker_tree(packs=packs)
    # Total should count wikimed only once
    all_ids = set()
    for g in tree:
        for p in g.packs:
            for s in p.sources:
                all_ids.add(s.id)
    assert len(all_ids) == 2  # wikimed + other


def test_build_tree_with_preset_prechecks():
    """Sources from a preset are pre-checked."""
    packs = [
        Preset(name="tools", description="", target_size_gb=1,
               region="", kind="pack", display_group="Tools",
               sources=[_source("kiwix"), _source("cyberchef")]),
    ]
    preset_source_ids = {"kiwix"}
    tree = build_picker_tree(packs=packs, checked_ids=preset_source_ids)
    pack = tree[0].packs[0]
    assert pack.checked_ids == {"kiwix"}
```

**Step 2: Run tests to verify they fail**

```bash
uv run pytest src/tests/test_picker.py -v
```

**Step 3: Implement the tree builder**

```python
# src/svalbard/picker.py
"""Interactive pack picker for Svalbard wizard."""

from dataclasses import dataclass, field

from svalbard.models import Preset, Source


@dataclass
class PickerPack:
    name: str
    description: str
    display_group: str
    sources: list[Source]
    checked_ids: set[str] = field(default_factory=set)

    @property
    def total_size_gb(self) -> float:
        return sum(s.size_gb for s in self.sources)

    @property
    def checked_size_gb(self) -> float:
        return sum(s.size_gb for s in self.sources if s.id in self.checked_ids)


@dataclass
class PickerGroup:
    display_group: str
    packs: list[PickerPack] = field(default_factory=list)


def build_picker_tree(
    packs: list[Preset],
    checked_ids: set[str] | None = None,
) -> list[PickerGroup]:
    """Build a tree of PickerGroup → PickerPack → Source from pack presets.

    Args:
        packs: List of pack presets (kind='pack') with display_group set.
        checked_ids: Source IDs to pre-check (e.g. from a selected preset).

    Returns:
        Sorted list of PickerGroups, each containing PickerPacks.
    """
    checked_ids = checked_ids or set()
    groups: dict[str, PickerGroup] = {}

    for pack in packs:
        dg = pack.display_group or "Other"
        if dg not in groups:
            groups[dg] = PickerGroup(display_group=dg)

        picker_pack = PickerPack(
            name=pack.name,
            description=pack.description,
            display_group=dg,
            sources=list(pack.sources),
            checked_ids={s.id for s in pack.sources if s.id in checked_ids},
        )
        groups[dg].packs.append(picker_pack)

    return sorted(groups.values(), key=lambda g: g.display_group)
```

**Step 4: Run tests**

```bash
uv run pytest src/tests/test_picker.py -v
```

**Step 5: Commit**

```bash
git add src/svalbard/picker.py src/tests/test_picker.py
git commit -m "feat: add picker tree builder for interactive pack selection"
```

---

### Task 6: Build the picker TUI renderer

The interactive full-screen terminal UI using rich.live + readchar.

**Files:**
- Modify: `src/svalbard/picker.py`
- Test: manual testing (TUI is interactive, hard to unit test)

**Step 1: Add row types and rendering**

Add to `src/svalbard/picker.py`:

```python
import os
import textwrap

import readchar
from rich.console import Console
from rich.live import Live
from rich.text import Text

# Row types
ROW_GROUP = "group"
ROW_PACK = "pack"
ROW_ITEM = "item"


def _build_rows(
    tree: list[PickerGroup],
    collapsed_groups: dict[str, bool],
    collapsed_packs: dict[str, bool],
) -> list[dict]:
    """Build flat list of rows from tree with collapse state."""
    rows = []
    for group in tree:
        rows.append({
            "kind": ROW_GROUP,
            "display_group": group.display_group,
            "packs": group.packs,
        })
        if collapsed_groups.get(group.display_group, False):
            continue

        for pack in group.packs:
            checked_count = len(pack.checked_ids)
            total_count = len(pack.sources)
            rows.append({
                "kind": ROW_PACK,
                "pack": pack,
                "checked": checked_count,
                "total": total_count,
            })
            if collapsed_packs.get(pack.name, True):
                continue

            for source in pack.sources:
                rows.append({
                    "kind": ROW_ITEM,
                    "source": source,
                    "pack": pack,
                    "checked": source.id in pack.checked_ids,
                })
    return rows


def _is_selectable(row: dict) -> bool:
    return row["kind"] in (ROW_GROUP, ROW_PACK, ROW_ITEM)


def _move_cursor(rows: list[dict], idx: int, direction: int) -> int:
    total = len(rows)
    if total == 0:
        return 0
    for _ in range(total):
        idx = (idx + direction) % total
        if _is_selectable(rows[idx]):
            return idx
    return idx


def _compute_total(tree: list[PickerGroup]) -> float:
    """Compute total size of all checked items, deduplicating by source ID."""
    seen: set[str] = set()
    total = 0.0
    for group in tree:
        for pack in group.packs:
            for source in pack.sources:
                if source.id in pack.checked_ids and source.id not in seen:
                    seen.add(source.id)
                    total += source.size_gb
    return total


def _build_display(
    rows: list[dict],
    cursor: int,
    tree: list[PickerGroup],
    free_gb: float,
    scroll_offset: int,
    max_visible: int,
    width: int,
) -> str:
    """Render the full picker display as a rich markup string."""
    total_gb = _compute_total(tree)
    fits = total_gb <= free_gb

    lines = []
    lines.append(f"[bold]Svalbard — Pack Picker[/bold]{' ' * 20}{free_gb:.0f} GB free")
    lines.append("[dim]" + "─" * width + "[/dim]")
    lines.append("")

    visible_rows = rows[scroll_offset:scroll_offset + max_visible]
    visible_start = scroll_offset

    for i, row in enumerate(visible_rows):
        abs_idx = visible_start + i
        is_cur = abs_idx == cursor
        prefix = "▸ " if is_cur else "  "

        if row["kind"] == ROW_GROUP:
            dg = row["display_group"]
            # Check if any pack in this group has checked items
            has_checked = any(
                len(p.checked_ids) > 0 for p in row["packs"]
            )
            style = "[bold]" if is_cur else "[bold dim]" if not has_checked else "[bold]"
            end = style.replace("[", "[/")
            lines.append(f"{prefix}{style}{dg}{end}")

        elif row["kind"] == ROW_PACK:
            pack = row["pack"]
            checked = row["checked"]
            total = row["total"]
            if checked == total:
                mark = "☑"
            elif checked > 0:
                mark = "◐"
            else:
                mark = "[dim]☐[/dim]"

            size = f"{pack.checked_size_gb:.1f} GB" if pack.checked_size_gb >= 0.1 else f"{pack.checked_size_gb * 1024:.0f} MB"
            style = "[bold white]" if is_cur else "[white]" if checked > 0 else "[dim]"
            end = style.replace("[", "[/")
            lines.append(f"    {prefix}{mark} {style}{pack.name}{end}  [dim]{size}[/dim]")

        elif row["kind"] == ROW_ITEM:
            source = row["source"]
            checked = row["checked"]
            mark = "☑" if checked else "[dim]☐[/dim]"
            size_str = f"{source.size_gb:.1f} GB" if source.size_gb >= 0.1 else f"{source.size_gb * 1024:.0f} MB"
            style = "[white]" if is_cur else "" if checked else "[dim]"
            end = style.replace("[", "[/") if style else ""
            desc = source.description or source.id
            if len(desc) > width - 40:
                desc = desc[:width - 43] + "..."
            lines.append(f"        {prefix}{mark} {style}{desc}{end}  [dim]{size_str}[/dim]")

    lines.append("")

    # Description panel
    if 0 <= cursor < len(rows):
        cur_row = rows[cursor]
        desc = ""
        if cur_row["kind"] == ROW_ITEM:
            desc = cur_row["source"].description or ""
        elif cur_row["kind"] == ROW_PACK:
            desc = cur_row["pack"].description or ""
        if desc:
            wrapped = textwrap.wrap(desc, width=width - 4)[:2]
            for line in wrapped:
                lines.append(f"  [dim italic]{line}[/dim italic]")

    # Footer
    lines.append("[dim]" + "─" * width + "[/dim]")
    if fits:
        lines.append(f"  Total: {total_gb:.1f} / {free_gb:.0f} GB{' ' * 20}[green]✓ fits[/green]")
    else:
        over = total_gb - free_gb
        lines.append(f"  Total: {total_gb:.1f} / {free_gb:.0f} GB{' ' * 16}[red]✗ {over:.1f} GB over[/red]")
    lines.append("  [dim]↑↓/jk navigate  SPACE toggle  ENTER expand/collapse  q done[/dim]")

    return "\n".join(lines)


def _get_terminal_height() -> int:
    try:
        return os.get_terminal_size().lines
    except OSError:
        return 24


def _update_scroll(total: int, cursor: int, max_visible: int, offset: int) -> int:
    if cursor < offset + 2:
        offset = max(0, cursor - 2)
    elif cursor >= offset + max_visible - 2:
        offset = min(total - max_visible, cursor - max_visible + 3)
    return max(0, offset)


def run_picker(
    tree: list[PickerGroup],
    free_gb: float = 0,
) -> set[str]:
    """Run the interactive picker. Returns set of selected source IDs."""
    console = Console()
    width = min(100, console.width - 2)

    collapsed_groups: dict[str, bool] = {}
    collapsed_packs: dict[str, bool] = {
        pack.name: True
        for group in tree
        for pack in group.packs
    }
    # Auto-expand groups
    for group in tree:
        collapsed_groups[group.display_group] = False

    cursor = 0
    scroll_offset = 0

    rows = _build_rows(tree, collapsed_groups, collapsed_packs)
    if rows:
        cursor = _move_cursor(rows, -1, 1)  # first selectable

    with Live("", console=console, refresh_per_second=15, screen=True) as live:
        while True:
            rows = _build_rows(tree, collapsed_groups, collapsed_packs)
            max_visible = max(8, _get_terminal_height() - 10)
            scroll_offset = _update_scroll(len(rows), cursor, max_visible, scroll_offset)

            display = _build_display(
                rows, cursor, tree, free_gb, scroll_offset, max_visible, width,
            )
            live.update(Text.from_markup(display))

            key = readchar.readkey()

            # Navigation
            if key in (readchar.key.UP, "k"):
                cursor = _move_cursor(rows, cursor, -1)
            elif key in (readchar.key.DOWN, "j"):
                cursor = _move_cursor(rows, cursor, 1)

            # Quit
            elif key in ("q", readchar.key.ESC):
                break

            # Toggle / Expand
            elif key in (" ", readchar.key.ENTER) and 0 <= cursor < len(rows):
                row = rows[cursor]

                if row["kind"] == ROW_GROUP:
                    dg = row["display_group"]
                    collapsed_groups[dg] = not collapsed_groups.get(dg, False)

                elif row["kind"] == ROW_PACK:
                    pack = row["pack"]
                    if key == readchar.key.ENTER:
                        collapsed_packs[pack.name] = not collapsed_packs.get(pack.name, True)
                    else:  # SPACE
                        if len(pack.checked_ids) == len(pack.sources):
                            pack.checked_ids.clear()
                        else:
                            pack.checked_ids = {s.id for s in pack.sources}

                elif row["kind"] == ROW_ITEM:
                    source = row["source"]
                    pack = row["pack"]
                    if source.id in pack.checked_ids:
                        pack.checked_ids.discard(source.id)
                    else:
                        pack.checked_ids.add(source.id)

    # Collect all checked IDs
    result: set[str] = set()
    for group in tree:
        for pack in group.packs:
            result.update(pack.checked_ids)
    return result
```

**Step 2: Add a CLI test entry point**

Add a temporary test command to try the picker:

```python
# At end of picker.py
if __name__ == "__main__":
    from svalbard.presets import load_preset, list_presets, PRESETS_DIR

    # Load all packs
    packs_dir = PRESETS_DIR / "packs"
    all_packs = []
    for name in list_presets():
        try:
            p = load_preset(name)
            if p.kind == "pack":
                all_packs.append(p)
        except Exception:
            pass

    # Pre-check from a preset
    preset = load_preset("finland-128")
    checked = {s.id for s in preset.sources}

    tree = build_picker_tree(packs=all_packs, checked_ids=checked)
    selected = run_picker(tree, free_gb=128)
    print(f"\nSelected {len(selected)} sources:")
    for s in sorted(selected):
        print(f"  {s}")
```

**Step 3: Manual test**

```bash
uv run python -m svalbard.picker
```

Verify: picker renders, navigation works, checkboxes toggle, size updates,
q exits.

**Step 4: Commit**

```bash
git add src/svalbard/picker.py
git commit -m "feat: add interactive pack picker TUI with rich + readchar"
```

---

### Task 7: Wire picker into wizard

**Files:**
- Modify: `src/svalbard/wizard.py`
- Modify: `src/svalbard/cli.py`

**Step 1: Refactor wizard to add mode selection**

In `src/svalbard/wizard.py`, after the target selection and space check,
replace the Region + Preset steps with:

```python
# Step 2: Mode selection
_clear()
console.print("\n[bold]Step 2 — Configure[/bold]")
console.print("  How would you like to set up this drive?\n")
console.print("  [bold]1[/bold]) Use a preset [dim](recommended)[/dim]")
console.print("      Pre-configured for your drive size and region")
console.print("  [bold]2[/bold]) Customize")
console.print("      Browse all content and pick what you want")

mode = Prompt.ask("\n  Select", choices=["1", "2"], default="1")
```

If mode is "1": show existing region + preset picker, then open the
interactive picker with that preset's items pre-checked.

If mode is "2": open the interactive picker with nothing checked.

**Step 2: Load all packs and build tree**

```python
from svalbard.picker import build_picker_tree, run_picker

# Load all available packs
all_packs = []
for name in list_presets(workspace=workspace):
    try:
        p = load_preset(name, workspace=workspace)
        if p.kind == "pack":
            all_packs.append(p)
    except Exception:
        pass

# Pre-check from preset (if mode 1)
checked_ids = set()
if preset_name:
    preset = load_preset(preset_name, workspace=workspace)
    checked_ids = {s.id for s in preset.sources}

tree = build_picker_tree(packs=all_packs, checked_ids=checked_ids)
selected_ids = run_picker(tree, free_gb=free_gb)
```

**Step 3: Compose a synthetic preset from selected IDs**

After the picker returns, build a preset from the selected source IDs:

```python
from svalbard.presets import _build_recipe_index, _source_from_recipe

recipe_index = _build_recipe_index()
sources = []
for sid in sorted(selected_ids):
    if sid in recipe_index:
        sources.append(_source_from_recipe(recipe_index[sid]))

# Save as a local preset
custom_name = f"custom-{datetime.now().strftime('%Y%m%d-%H%M')}"
custom_path = local_presets_dir(workspace) / f"{custom_name}.yaml"
custom_path.parent.mkdir(parents=True, exist_ok=True)
custom_path.write_text(yaml.dump({
    "name": custom_name,
    "kind": "preset",
    "description": f"Custom selection — {len(sources)} sources",
    "target_size_gb": free_gb,
    "region": "",
    "sources": list(selected_ids),
}, default_flow_style=False))
preset_name = custom_name
```

**Step 4: Continue to review + sync (existing code)**

The rest of the wizard flow (review table, confirm, init, sync) stays the
same — it just uses the custom preset name instead of the original preset.

**Step 5: Test manually**

```bash
uv run svalbard wizard
```

Verify: mode selection → picker opens → toggle items → q → review → confirm.

**Step 6: Commit**

```bash
git add src/svalbard/wizard.py src/svalbard/cli.py
git commit -m "feat: wire interactive pack picker into wizard"
```

---

### Task 8: Add `--browse` to attach command

**Files:**
- Modify: `src/svalbard/cli.py`

**Step 1: Add --browse flag to attach command**

When `--browse` is passed without a source argument, open the picker
showing what's already on the drive (checked) and what's available (unchecked).

```python
@main.command()
@click.argument("source", required=False)
@click.argument("drive", required=False)
@click.option("--browse", is_flag=True, help="Browse and select packs interactively")
def attach(source, drive, browse):
    if browse:
        # ... open picker for existing drive
    elif source and drive:
        # ... existing attach logic
```

**Step 2: Commit**

```bash
git add src/svalbard/cli.py
git commit -m "feat: add --browse flag to attach for interactive pack selection"
```

---

## Summary

| Task | What | Effort |
|------|------|--------|
| 1 | Rename group → display_group in models | ~10 min |
| 2 | Rename group → display_group in recipe YAMLs | ~5 min |
| 3 | Add display_group to pack YAMLs | ~10 min |
| 4 | Add readchar dependency | ~2 min |
| 5 | Picker tree builder (data layer) | ~15 min |
| 6 | Picker TUI renderer | ~30 min |
| 7 | Wire into wizard | ~20 min |
| 8 | Add --browse to attach | ~10 min |
