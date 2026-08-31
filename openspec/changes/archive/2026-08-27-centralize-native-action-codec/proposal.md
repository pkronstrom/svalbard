## Why

Built-in native action arguments are encoded in `internal/actions` and independently decoded in `main`, so changes require synchronized positional switches. A concrete codec gives the protocol one owner without introducing a command framework.

## What Changes

- Define a concrete native invocation value.
- Centralize action-ID mapping, positional encoding, decoding, defaults, and validation in `internal/actions`.
- Keep `main` responsible only for executing named decoded actions.
- Preserve existing hidden subcommands and recipe action IDs.

## Capabilities

### New Capabilities
- `native-action-invocation`: Round-trippable built-in action invocation encoding and validation.

### Modified Capabilities

None.

## Impact

- Affects `drive-runtime/internal/actions` and `drive-runtime/cmd/svalbard-drive`.
- Hidden subcommands remain an internal compatibility boundary.
- No generic command registry or framework is added.
