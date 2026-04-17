# Menu: direct-action groups and cleanup

## Problem

Search should be a direct action at the top level, not a group you drill into. The current single-item auto-activate hack is fragile. Also, `MenuMeta` is a lossy duplicate of `catalog.MenuSpec` with a translation bug (`recipe.Description` copied into `meta.Label`).

## Design

Minimal changes, no new abstractions.

### 1. Direct-action groups via `auto_activate`

`MenuGroup` gains one field: `AutoActivate bool`. When true, entering the group immediately executes its first item instead of showing the submenu. The group's label/description are what the user sees at the top level.

This is explicit in the JSON config and narrow in scope. No action-on-group, no kind inference, no union types.

```json
{
  "id": "search",
  "label": "Search",
  "description": "Search across indexed archives and documents.",
  "order": 100,
  "auto_activate": true,
  "items": [{"id": "search-all", "action": {"type": "builtin", "config": {"name": "search"}}}]
}
```

### 2. Delete `MenuMeta`, pass `catalog.MenuSpec` directly

`MenuMeta` in toolkit.go is a copy of `catalog.MenuSpec` with fields mapped wrong. Delete it. Pass `map[string]catalog.MenuSpec` into `GenerateOpts` and use it directly in `writeActionsConfig`.

This requires toolkit.go to import the catalog package. If that's a problem due to circular deps, pass `map[string]*catalog.MenuSpec` — but it shouldn't be since toolkit already imports manifest from the same module.

### 3. Keep existing inference rules

- `binary + local-ai group -> agent` action stays as a local rule in toolkit.go. Two recipes don't justify a new cross-layer field.
- Recipe `menu:` spec overrides type defaults. No change to this precedence.
- Built-in capabilities append runtime items. No change.

## Changes

### drive-runtime/internal/config/config.go

Add `AutoActivate bool \`json:"auto_activate,omitempty"\`` to `MenuGroup`.

### drive-runtime/internal/menu/model.go

On Enter at top level: if `group.AutoActivate && len(group.Items) > 0`, call `activateItem(group.Items[0])` instead of drilling in. Remove the single-item auto-activate hack.

### host-cli/internal/toolkit/toolkit.go

- Add `AutoActivate bool` to `menuGroup`
- Delete `MenuMeta` type
- Change `GenerateOpts.Menus` from `map[string]MenuMeta` to `map[string]catalog.MenuSpec`
- Update `writeActionsConfig` to read `catalog.MenuSpec` fields directly
- Search builtin capability sets `AutoActivate: true` on its group

### host-cli/internal/apply/apply.go

- Pass `recipe.Menu` (`*catalog.MenuSpec`) directly instead of building `MenuMeta`
- Fix the bug where `recipe.Description` was copied into label

### Cleanup: builtins reference group IDs only

`builtinCapabilities` entries should not duplicate group label/description. They reference a group ID from `groupRegistry` and only define item-level fields (label, description, order, action). The search builtin is special — it defines the group itself with `AutoActivate: true` rather than adding an item.

### Cleanup: remove dead single-item hack

The `activateItem` extraction and single-item auto-activate code in `model.go` gets replaced by the `AutoActivate` check. Clean up any leftover branching.

## Not doing

- `MenuGroup.Action` field — too much surface area for one case
- `MenuSpec.Action` field — the binary+local-ai inference works fine
- `MenuGroup.Kind` field — auto_activate is simpler and more explicit
- Any recipe YAML changes — not needed

## Verification

1. `go build ./...` and `go test ./...` in both host-cli and drive-runtime
2. `svalbard --vault /tmp/svalbard-test apply`
3. Check `actions.json` — search group has `auto_activate: true`
4. Run drive TUI — Enter on Search opens search directly, Enter on Library drills into submenu
