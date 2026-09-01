## Context

`drive-runtime/internal/config.MenuItem`, `RuntimeConfig.FindItemByAlias`, and `tui.PaletteEntry` already support aliases. `catalog.MenuSpec` and toolkit's mirrored `menuItem` omit them, leaving the feature available only to hand-authored JSON.

## Goals / Non-Goals

**Goals:** carry recipe aliases unchanged into generated runtime config and prove both runtime discovery paths.

**Non-Goals:** inventing default aliases, changing stable IDs, or adding alias conflict resolution.

## Decisions

- `MenuSpec.Aliases` is optional YAML and defaults empty.
- Toolkit copies recipe aliases into the corresponding generated item.
- Built-in capabilities receive no speculative aliases.
- Existing runtime behavior resolves the first configured match in menu order.

## Risks / Trade-offs

- Duplicate aliases remain a catalog-author responsibility, matching existing runtime semantics.
