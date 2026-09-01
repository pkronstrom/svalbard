## Context

Recent consolidation left wrappers and exported methods with no production callers. Tests currently preserve some of those surfaces instead of testing the narrower paths production uses.

## Goals / Non-Goals

**Goals:** delete proven-dead APIs, migrate tests to production paths, remove needless allocations and one-entry state.

**Non-Goals:** changing MCP protocols, search semantics, menu navigation behavior, or introducing shared test frameworks.

## Decisions

- Trace each symbol immediately before deletion; unexpected production callers stop that item only.
- Keep package-local tests close to internals rather than exporting introspection solely for external-package tests.
- Replace helpers with `strings.Contains` and `tui.StripAnsi`; do not create `testutil`.
- Replace `visibleEntries` with a count and the separator map with direct equality.

## Risks / Trade-offs

- Keep unrelated correctness or architecture changes out of this round.

## Safety gate

Only internal symbols with no production caller and an already-used replacement
qualify for deletion. Conditional compatibility surfaces are handled in
separate changes.
