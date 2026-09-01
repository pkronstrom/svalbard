## Why

Action aliases already work in drive-runtime CLI lookup and the command palette, but recipe menus cannot emit them. Wiring the existing capability through catalog generation makes custom offline drives easier to operate without narrowing hand-authored configuration.

## What Changes

- Add optional aliases to recipe `MenuSpec`.
- Serialize aliases into generated `actions.json` menu items.
- Preserve runtime alias lookup and case-insensitive palette matching.
- Add generated-config and catalog parsing coverage.

## Capabilities

### New Capabilities
- `action-aliases`: Custom recipes can define friendly action names that work in generated drive CLI and palette discovery.

### Modified Capabilities

None.

## Impact

- Affects host catalog and toolkit generation only; drive-runtime already consumes aliases.
- Existing recipes and generated configurations remain valid.
- No new dependencies.
