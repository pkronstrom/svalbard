# Recipe-Defined Exec Actions Design

## Goal

Restore the useful property that a recipe can define its own drive-side action
without requiring a new hardcoded Go runtime package, while keeping execution
inside the Go launcher/runtime rather than returning to shell-script dispatch.

This first cut is intentionally minimal:

- rename the generated launcher manifest from `.svalbard/runtime.json` to
  `.svalbard/actions.json`
- standardize terminology on `menu` + `action`
- add a single generic recipe-defined action type: `exec`
- keep existing built-in inferred behavior when a recipe does not declare an
  explicit `action`

The runtime remains Go-native. Recipes declare actions in data.

## Why

The current Go runtime is more portable, testable, and maintainable than the
old shell action system, but it lost one useful quality: recipes no longer feel
self-contained, because executable behavior is centralized in Go packages.

This design restores recipe-local action ownership without reintroducing shell
scripts as the execution contract.

## Terminology

Use the following terms consistently:

- `menu`: presentation metadata for where and how an item appears
- `action`: execution metadata for what happens when the user launches it
- `actions.json`: generated drive-side manifest consumed by the Go launcher
- `runtime`: reserved for the Go host/runtime system itself, not for per-item
  menu item behavior

This means:

- recipe metadata uses `menu:` and `action:`
- generated drive metadata is `.svalbard/actions.json`
- internal Go config should move toward `ActionManifest`, `ActionGroup`,
  `ActionItem`, or equivalent naming over time

## Scope

Included in this design:

- recipe-level `action` metadata
- generated `actions.json` rename and schema alignment
- one supported action type: `exec`
- placeholder substitution for a very small set of variables
- direct executable invocation only

Explicitly not included:

- raw shell command strings
- pipes, redirection, or `bash -lc`
- multiple action types
- recipe-defined service readiness checks
- pack/preset-level action overrides
- arbitrary nested menu trees

## Data Model

Recipes may define:

```yaml
menu:
  group: tools
  label: My Tool
  description: Run my bundled tool

action:
  type: exec
  config:
    executable: my-tool
    resolve_from: drive-bin
    args:
      - "--input"
      - "{drive_root}/data/file.db"
    env:
      DATA_DIR: "{drive_root}/data"
    cwd: "{drive_root}"
    mode: interactive
```

### `menu`

Unchanged in spirit from the current grouped menu model:

- `group`
- `label`
- `description`
- optional `subheader`
- optional `order`

### `action`

Generic top-level shape for future expansion:

```yaml
action:
  type: exec
  config: ...
```

Only `type: exec` is valid in the first implementation.

### `action.type = exec`

`config` schema:

- `executable`: required string
- `resolve_from`: optional string
- `args`: optional string array
- `env`: optional string map
- `cwd`: optional string
- `mode`: optional string

Supported `resolve_from` values:

- `drive-bin`
- `path`
- `drive-bin-or-path`

Default:

- `drive-bin-or-path`

Supported `mode` values:

- `interactive`
- `capture`
- `service`

Defaults:

- `mode: interactive`
- `args: []`
- `env: {}`
- `cwd: "{drive_root}"`

## Placeholder Expansion

Keep placeholder support deliberately tiny in the first version.

Supported placeholders:

- `{drive_root}`
- `{platform}`

These may appear in:

- `config.executable`
- `config.args[]`
- `config.env` values
- `config.cwd`

No other placeholders are supported in the first cut.

Unsupported examples for now:

- `{free_port:8080}`
- `{selected_item}`
- `{user_home}`

## Execution Semantics

The Go launcher is responsible for:

- loading `.svalbard/actions.json`
- resolving a selected action
- expanding placeholders
- resolving the executable according to `resolve_from`
- applying `cwd`
- applying `env`
- running the process according to `mode`

Execution is always direct process invocation. No shell is involved.

Examples:

- `interactive`: inherit stdin/stdout/stderr and hand over control
- `capture`: capture stdout/stderr and show the output screen in the TUI
- `service`: run as a long-lived launched process under the existing service
  execution path

## Interaction With Existing Built-In Actions

Built-in inferred behavior remains in place when a recipe does not define an
explicit `action`.

This preserves current behavior for:

- archives
- maps
- chat models
- AI clients
- tools

If a recipe defines an explicit `action`, that explicit action replaces the
inferred action for that item.

This makes the new mechanism an escape hatch first, not a mandatory rewrite of
all built-in recipes.

## Manifest Shape

Rename:

- `.svalbard/runtime.json` -> `.svalbard/actions.json`

The generated manifest continues to contain grouped menu items, but item-level
execution payloads should use `action` terminology consistently.

Conceptually:

```json
{
  "version": 3,
  "preset": "default-32",
  "groups": [
    {
      "id": "tools",
      "label": "Tools",
      "description": "Inspect the drive and launch bundled utilities.",
      "items": [
        {
          "id": "my-tool",
          "label": "My Tool",
          "description": "Run my bundled tool",
          "action": {
            "type": "exec",
            "config": {
              "executable": "my-tool",
              "resolve_from": "drive-bin",
              "args": ["--input", "{drive_root}/data/file.db"],
              "env": {"DATA_DIR": "{drive_root}/data"},
              "cwd": "{drive_root}",
              "mode": "interactive"
            }
          }
        }
      ]
    }
  ]
}
```

Existing inferred built-in items may still be emitted into this same shape; the
important part is that the manifest contract uses `action`, not shell-era
runtime naming.

## Go Runtime Changes

### Config Layer

Change the config model to load `.svalbard/actions.json` and represent:

- groups
- items
- action payloads

The loader should support:

- inferred built-in item actions serialized into the same action schema
- explicit recipe-defined `action` blocks

### Action Resolution Layer

Extend the action resolver with:

- existing built-in/native actions
- generic `exec` actions from the manifest

The runtime should not distinguish between “built-in” and “recipe-defined” at
the menu layer. The distinction belongs in the action resolver only.

### Binary Resolution

`resolve_from` maps to existing or near-existing helpers:

- `drive-bin`: resolve only from the drive’s bundled binaries
- `path`: resolve only from host PATH
- `drive-bin-or-path`: try drive first, then PATH

This should reuse the existing binary resolution and extraction logic as much as
possible.

## Error Handling

Validation errors should be explicit and early:

- unknown `action.type`
- missing `config.executable`
- invalid `mode`
- invalid `resolve_from`

Launch-time errors should be user-facing:

- executable not found
- cwd missing
- process launch failure

For `capture` mode, errors should be shown in the captured output view.
For `interactive` and `service` mode, errors should use the existing launcher
status/error handling.

## Testing

Minimum test coverage:

- Python generation of `.svalbard/actions.json`
- inline recipe `action` parsing
- built-in inferred action fallback when `action` is absent
- explicit `action` override when present
- Go config loading of `action.type/config`
- placeholder expansion
- executable resolution by `resolve_from`
- `interactive` / `capture` / `service` mode dispatch

## Migration

Phase 1:

- add `action` support to recipe metadata
- rename generated manifest to `actions.json`
- teach Go launcher to load `actions.json`
- keep built-in inferred actions unchanged

Phase 2:

- opportunistically move selected built-in recipes onto explicit `action`
  declarations where it improves locality and clarity

This keeps the first implementation small and low-risk.

## Recommendation

Implement exactly one generic action type now:

- `action.type: exec`

Keep everything else out of scope for the first cut.

That restores recipe-defined custom commands for both built-in and user recipes,
without weakening the Go runtime architecture or reintroducing shell-script
dispatch as the main execution model.
