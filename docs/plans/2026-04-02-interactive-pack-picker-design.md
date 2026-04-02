# Interactive Pack Picker — Design

**Status:** Design
**Date:** 2026-04-02

## Goal

Replace the wizard's flat preset picker with an interactive TUI that shows
all available content organized by display group and pack. Users can toggle
individual items on/off, expand/collapse sections, and see a live size total
against their drive's available space.

## Approach

Build a custom picker using `rich.live.Live` + `readchar`, following the
same pattern as hawk-hooks' toggle.py. Three-tier collapsible hierarchy:
display group → pack → individual recipe. Live size footer. Vi-style
keyboard navigation.

### New dependency

`readchar>=4.0.0` — raw keyboard input. Already used by hawk-hooks, small
and well-maintained.

## Data Model Changes

### Rename `group` → `display_group`

The `group` field on recipes currently serves as a display label and nothing
else. Rename it to `display_group` to clarify intent.

- **On recipes:** `display_group` is the fallback section for orphan recipes
  (those not in any pack).
- **On packs:** `display_group` determines which top-level section the pack
  appears under in the picker.

```yaml
# Recipe — display_group as fallback
id: wikipedia-en-nopic
type: zim
display_group: Reference
...

# Pack — display_group determines picker section
name: fi-maps
kind: pack
display_group: Maps & Geodata
description: Finnish maps and geodata overlays
sources:
  - osm-finland
  - luonnonsuojelualueet
```

### Hierarchy

```
display_group (top-level section)       ← from pack or recipe
  └── pack (curated bundle)             ← collapsible, toggleable
       └── recipe (individual item)     ← toggleable checkbox
```

For recipes in a pack: the pack's `display_group` determines placement.
For orphan recipes: the recipe's own `display_group` is used.

### Deduplication

A recipe may appear in multiple packs. When composing the selection:

- First pack wins — the recipe shows under the first checked pack that
  contains it.
- If a recipe is toggled off in one pack but exists in another checked pack,
  it's still included (shown as "included via [other pack]").
- The final source list is deduplicated by `source.id` before sync.

## Wizard Flow

### Current flow

```
Target → Region → Preset (flat list) → Local sources → Review → Sync
```

### New flow

```
Target → Mode (preset / custom) → Pack Picker → Review → Sync
```

**Mode selection:**

```
How would you like to configure this drive?

  1) Use a preset (recommended)
     Pre-configured for your drive size and region

  2) Customize
     Browse all available content and pick what you want
```

**If preset:** show preset list (same as today), then open the picker with
that preset's items pre-checked. User can adjust.

**If custom:** open the picker with nothing checked. User browses and
selects.

Either way, the picker is the central experience.

## Picker UI

### Layout

```
Svalbard — Pack Picker                    128 GB free
────────────────────────────────────────────────────────

▾ Reference
    ▾ Core Reference                  ☑   48.0 GB
        ☑ Wikipedia English (nopic)       42.0 GB
        ☑ Wikibooks                        3.0 GB
        ☐ Wiktionary English               5.5 GB
    ▾ Finnish Reference               ☑    2.5 GB
        ☑ Wikipedia Finnish                2.0 GB
        ☑ Wiktionary Finnish               0.5 GB

▸ Survival & Medical                  ☑    1.8 GB

▾ Maps & Geodata
    ▾ Finnish Maps                    ☑    1.5 GB
        ☑ OSM Finland                      1.2 GB
        ☐ Foraging habitats                0.2 GB

▸ Tools & Dev                         ☑    0.5 GB

▾ AI Models
    ☑ Qwen 0.8B                            0.6 GB
    ☐ Qwen 9B                              5.9 GB

  English Wikipedia — full with all pictures
────────────────────────────────────────────────────────
  Total: 54.2 / 128 GB                        ✓ fits
  ↑↓/jk navigate  SPACE toggle  ENTER expand  q done
```

### Row types

| Type | Rendering | Interaction |
|------|-----------|-------------|
| Display group | `▾ Reference` or `▸ Reference` | ENTER expands/collapses |
| Pack header | `▾ Core Reference  ☑  48.0 GB` | ENTER expands/collapses, SPACE toggles all items |
| Recipe item | `☑ Wikipedia English  42.0 GB` | SPACE toggles on/off |

### Checkbox rendering

```
☑  enabled (filled)
☐  disabled (dim)
◐  partially enabled (some items in pack unchecked)
```

### Description panel

Bottom area shows a 1-3 line description of the focused item:

- Recipe focused → recipe description
- Pack focused → pack description
- Display group focused → no description (or count summary)

Descriptions are loaded from recipe/pack YAML, cached after first access.

### Size tracking

Footer shows live total:

```
  Total: 54.2 / 128 GB                        ✓ fits
```

Updates on every toggle. If total exceeds available space:

```
  Total: 142.3 / 128 GB                   ✗ 14.3 GB over
```

Items that individually exceed remaining space are dimmed but still
selectable (user decides priority).

### Keyboard navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `SPACE` | Toggle item/pack on/off |
| `ENTER` | Expand/collapse group or pack |
| `t` | Toggle all items in focused group/pack |
| `q` / `ESC` | Done — proceed to review |
| `?` | Show help |

Cursor skips separator rows. Scroll viewport follows cursor with smart
offset (same as hawk pattern).

## CLI Integration

### Wizard

```bash
svalbard wizard
# Target → Mode → Picker → Review → Sync
```

### Init with picker

```bash
svalbard init /Volumes/MyStick
# Opens picker (no preset pre-selected)

svalbard init /Volumes/MyStick --preset finland-128
# Opens picker with finland-128 items pre-checked
```

### Add packs to existing drive

```bash
svalbard attach --browse /Volumes/MyStick
# Opens picker showing what's already on the drive (checked)
# and what's available to add (unchecked)
# User checks new items → svalbard sync downloads them
```

This replaces the current `attach <source> <drive>` for interactive use.
The explicit `attach <source> <drive>` still works for scripting.

## Implementation Scope

### Source data for the picker

The picker needs to build a tree of:

```python
@dataclass
class PickerGroup:
    display_group: str
    packs: list[PickerPack]
    orphan_recipes: list[Source]   # recipes not in any pack

@dataclass
class PickerPack:
    name: str
    description: str
    display_group: str
    sources: list[Source]
    checked: bool                  # from preset pre-selection
```

Built by:
1. Load all pack YAMLs (they list their sources and display_group)
2. Load preset (if any) to determine pre-checked state
3. Find orphan recipes (in preset but not in any pack)
4. Group packs by display_group
5. Sort groups and packs by a sensible order

### Output of the picker

The picker returns a list of selected source IDs. This feeds into the
existing `init_drive` / `sync_drive` flow — no changes needed downstream.

The wizard can compose a synthetic preset name or use a convention like
`custom-<timestamp>` and write a preset YAML to `local/presets/` so the
drive remembers what was selected.

### Files to create/modify

| File | Change |
|------|--------|
| `src/svalbard/picker.py` | New — picker TUI implementation |
| `src/svalbard/wizard.py` | Modify — replace preset step with picker |
| `src/svalbard/cli.py` | Modify — add `--browse` to attach |
| `src/svalbard/models.py` | Modify — rename group → display_group |
| `src/svalbard/presets.py` | Modify — parse display_group from packs |
| `recipes/**/*.yaml` | Modify — rename group → display_group |
| `presets/packs/**/*.yaml` | Modify — add display_group field |
| `pyproject.toml` | Modify — add readchar dependency |

### Phases

**Phase 1: Data model** — Rename group → display_group, add display_group
to packs. No UI changes.

**Phase 2: Picker core** — Build picker.py with rich.live + readchar.
Standalone function that takes a tree and returns selected IDs.

**Phase 3: Wizard integration** — Wire picker into wizard flow. Add
preset/custom mode selection.

**Phase 4: Attach integration** — Add `--browse` to attach command for
adding packs to existing drives.

## Open Questions

- Should display_group values be a controlled vocabulary or free-form
  strings on each pack? (Leaning free-form for now, normalize later.)
- What order should display groups appear in? Alphabetical, or a fixed
  priority (Reference first, Tools last)?
- Should the picker show platform-specific size estimates (darwin-arm64
  only vs. universal) or always show per-platform size?
- How to handle the region concept? Currently presets are region-specific
  (finland-128). Should region filter which packs are visible, or show all
  packs with regional ones labeled?
