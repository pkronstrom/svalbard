## Context

`internal/actions` maps stable action IDs to hidden subcommands and positional arguments. `main` separately maps hidden subcommands back to action implementations and repeats positional defaults and validation.

## Goals / Non-Goals

**Goals:**
- Give the built-in native action protocol one concrete owner.
- Round-trip all built-in actions through named arguments.
- Keep existing hidden commands and action IDs compatible.

**Non-Goals:**
- A general command framework, registry, reflection-based codec, or public plugin protocol.
- Changing action implementations.

## Decisions

- Add `NativeInvocation` with stable `ActionID` and named string arguments.
- Keep a single concrete definition per built-in action that performs hidden-command mapping, positional encode/decode, defaults, and validation.
- Resolver code asks the codec for argv; it does not assemble the protocol itself.
- `main` asks the codec to decode argv, switches on stable action ID, and passes named values to implementations.
- Unknown IDs/subcommands, missing required values, and extra positional arguments are errors.

## Risks / Trade-offs

- Hidden subcommands are internal but already generated into runtime commands, so they remain stable in this cutover.
- Map-based named arguments trade compile-time field checking for a small, explicit codec; table tests defend every built-in contract.
