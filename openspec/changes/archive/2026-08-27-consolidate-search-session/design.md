## Context

`Run` and `Session` independently open the search database, detect capabilities, choose engines, launch embedding/Kiwix processes, perform fallback, open results, and clean up. Menu search already uses `Session`; terminal search maintains a second backend.

## Goals / Non-Goals

**Goals:**
- Make `Session` the sole owner of database, engine, child-process, fallback, and result-opening behavior.
- Keep `Run` limited to terminal parsing and rendering.
- Preserve all existing terminal commands and entry points.

**Non-Goals:**
- Changing search ranking, modes, action IDs, menu routing, or result presentation.
- Introducing a public search interface solely for tests.
- Consolidating MCP launchers in this change.

## Decisions

- `Run` constructs one `Session`, defers `Close`, and obtains counts/default mode from `Info`.
- Terminal searches call `Session.Search`; fallback messages render `SearchResponse.Status`.
- Numbered selections call `Session.OpenResult`.
- If the terminal loop needs isolation for tests, use an unexported function accepting an existing `*Session` and injected input/output; keep backend APIs concrete.
- Behavioral tests use a temporary SQLite search database where practical.

## Risks / Trade-offs

- `Session.Search` must preserve terminal-specific mode aliases and fallback output.
- Cleanup regressions can leak child processes; tests and real-path smokes must cover normal and error exits.
- A thin unexported loop seam is acceptable; a public interface would add unjustified abstraction.
