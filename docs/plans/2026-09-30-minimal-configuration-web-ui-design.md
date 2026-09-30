# Minimal configuration web UI — planning brief

Status: requested feature, queued for planning 2026-09-30; not designed or implemented.

## User goal

Optionally configure a vault in a browser: choose a preset, customize its selected items, and add/remove items later. This is a minimal configuration interface, distinct from the old broad runtime-dashboard wishlist.

## Proposed boundary to explore

Reuse host-side catalog, desired-state, dependency resolution and plan/apply operations used by the TUI. Avoid separate browser-specific recipe semantics or duplicated reconciliation. Propose a local browser interface, with lifecycle and access boundaries settled in discovery.

## Decisions to settle with the user

- Host-side setup only, or also launchable from an existing stick? Do not assume the read-only runtime should own host-side mutation.
- Configure and preview only, or also start/cancel/resume apply jobs and show progress?
- Screens and interactions: preset selection, item selection/removal, dependencies/space summary, saved overrides and operation preview.
- Local access, lifetime, concurrent TUI/browser edits and persistence behavior.

## Planning acceptance criteria

Produce a small screen/user-flow sketch, agreed scope, and an implementation plan of at most four chunks. The resulting implementation must share the established vault configuration operations and tests for saved configuration/preview consistency. Do not build an expansive dashboard, account system or remote management service as part of this feature.
