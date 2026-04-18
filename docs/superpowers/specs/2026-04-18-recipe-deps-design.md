# Recipe Dependencies

## Problem

When a user cherry-picks individual recipes in the tree picker (e.g., a GGUF model without llama-server, or pmtiles without go-pmtiles), the resulting vault is broken. Packs paper over this by manually listing all needed tools, but there's no automatic safety net.

## Design

### Dependency Declaration (Hybrid)

**Type-level defaults** live in `recipes/dep-defaults.yaml` — single source of truth read by both Python and Go:

```yaml
gguf: [llama-server]
pmtiles: [go-pmtiles, maplibre-vendor, dufs]
zim: [kiwix-serve]
```

**Recipe-level override** — optional `deps:` field in any recipe YAML. When present, it **replaces** the type default entirely (not merges). An explicit `deps: []` means "no deps despite my type."

```yaml
id: gemma-4-e4b-it
type: gguf
deps: [llama-server, some-other-thing]  # overrides type default
```

If no `deps:` field is present, the type default applies.

**Validation:** on load, every dep ID referenced in `dep-defaults.yaml` or a recipe's `deps:` field must exist in the recipe index. Missing IDs produce a warning (not a silent skip) — the whole point of this feature is preventing broken vaults from missing tools.

### Resolution Logic

**Two resolution contexts:**

1. **Python (CLI/headless):** runs once at the end of `parse_preset()`, after all extends/merges/removals are complete. Only the top-level call resolves deps — recursive `extends` calls skip it. This ensures removals (`-source`) are honored before dep resolution.

2. **Go (interactive TUI):** runs in the pack picker after each toggle. This is needed because the user is dynamically changing selections.

Both read the same `dep-defaults.yaml` and apply the same algorithm. An integration test verifies parity.

**Algorithm:**

1. Collect all selected source IDs.
2. For each, look up deps: recipe-level `deps:` field > type-level default > none.
3. Dep IDs not already in the selection are added with `auto_dep=True`.
4. Resolve transitively with cycle detection — deps can have deps.
5. When a source is removed, re-run resolution; deps with no remaining dependents are removed.

**Python-specific:** resolution accepts already-built `Source` objects and only constructs new `Source` objects for auto-dep additions. This preserves any preset-level overrides (e.g., `override: {size_gb: 2.0}`).

### Data Model Changes

**Python** — `Source` gets one field:

```python
auto_dep: bool = False  # True if pulled in by dependency resolution
```

**Go** — `catalog.Item` gets dep metadata (since it already reads recipe YAML):

```go
Deps            []string // from recipe `deps:` field
HasExplicitDeps bool     // true if recipe YAML had `deps:` key at all
```

The TUI layer (`TreePicker`) tracks auto-dep state via a single `AutoDepIDs map[string]bool` — no redundant `AutoDep` bool on `PackSource`. Rendering checks `AutoDepIDs[src.ID]`.

### TUI Behavior

**Visual:** auto-deps render with a dimmed `[✓]` (muted theme color). Description suffix in dim text shows origin, e.g., `← dep`. When the cursor is on a dimmed dep, the footer shows `"auto-included — needed by [source]"`.

**Interaction:**
- Checking a source auto-checks its deps (dimmed) in their natural tree position.
- Unchecking a source removes its deps if nothing else needs them.
- User cannot directly uncheck a dep while a dependent exists (space is a no-op or brief flash message).
- **User intent tracking:** the picker maintains a `UserCheckedIDs` set (items the user explicitly toggled on). If the user manually checked a dep before it became auto-included, it stays as a normal (non-dimmed) check. Unchecking the dependent does not remove it — user intent wins. `recalcDeps` only removes IDs that are in `AutoDepIDs` but NOT in `UserCheckedIDs`.

**Pack-level toggle:** toggling an entire pack triggers dep resolution for all its sources. Pack size indicators account for deps.

**Orphan deps:** if a dep recipe isn't in any currently-visible pack, it appears in a synthetic "Dependencies" group at the bottom of the tree. The existing "Other" group pattern in `root.go` (for loose recipes) can be extended for this.

**Performance:** `recalcDeps` only runs after actual toggle events, not on every keypress (navigation, expand/collapse). `TreePicker.Update()` returns a signal indicating whether a toggle occurred.

### Feature-Level Prerequisites (Separate Mechanism)

The indexing feature needs an embedding model, but this is not a recipe dep — it's a feature prerequisite. Handled as a dedicated check in the index flow:

- Before indexing, check for an embedding model (e.g., `nomic-embed-text-v1.5` or `all-minilm-l6-v2`) in the resolved source list.
- If missing: inline prompt — *"Semantic search needs an embedding model. Add nomic-embed-text-v1.5 (140 MB)? [Y/n]"*
- Yes: add to vault config, quick download, proceed with indexing.
- No: skip semantic indexing, run full-text only.

This pattern applies to other feature-level checks in the future (e.g., chat requiring a chat model + llama-server).

## Files Affected

- `recipes/dep-defaults.yaml` — new file, type-level dep defaults
- `src/svalbard/models.py` — `auto_dep` field on `Source`
- `src/svalbard/presets.py` — dep resolution pass in `parse_preset()` (top-level only), accepts already-built Sources
- `host-cli/internal/catalog/catalog.go` — `Deps`/`HasExplicitDeps` on `Item`, dep resolution method
- `tui/treepicker.go` — `AutoDepIDs` map, `UserCheckedIDs` map, dimmed rendering, toggle guards, toggle-only recalc signal
- `host-tui/internal/wizard/packpicker.go` — dep resolution on toggle (via catalog), config struct for constructor
- `host-tui/internal/index/indexmodel.go` — embedding model prerequisite check
