# Drive Runtime Go Port Design

Date: 2026-04-15
Status: Approved for planning

## Summary

Svalbard should replace the USB stick's shell-based drive runtime with a Go
runtime in two phases.

Phase 1 establishes the new Go runtime boundary:

- a keyboard-first Go TUI replaces `run.sh` on macOS and Linux
- the menu launches stable Go action adapters instead of raw shell paths
- those adapters may still delegate to the existing `.svalbard/actions/*.sh`
  implementations internally
- newly provisioned drives ship launcher binaries for all supported host
  platforms by default unless `--platform` narrows the output

Phase 2 ports the drive runtime implementations behind those adapters from
shell to native Go. The action surface from phase 1 remains stable so the
menu and provisioning contracts do not need another rewrite.

## Goals

- Replace the current `run.sh` numeric menu with a better keyboard-driven Go
  TUI.
- Stop exposing shell script paths as the runtime contract.
- Create a clean action boundary that supports incremental runtime migration.
- Keep the Python CLI as the host-side provisioner and keep the Go binary as
  the drive-side runtime.
- Design phase 1 so Windows can be added later without changing the TUI or
  action model.

## Non-Goals

- Full Windows support in phase 1.
- Reimplementing external payload tools such as `kiwix-serve`,
  `llama-server`, or PlatformIO toolchains in Go.
- Preserving backwards compatibility with already-provisioned drives that only
  contain `run.sh` and `.svalbard/entries.tab`.

## Current Runtime Boundary

Today the drive-side runtime consists of:

- `run.sh` as the user entrypoint
- `.svalbard/entries.tab` as the generated menu definition
- `.svalbard/lib/*.sh` for shared runtime helpers
- `.svalbard/actions/*.sh` for user-facing actions
- bundled content and payload tools on the stick such as `kiwix-serve`,
  `llama-server`, `dufs`, `sqlite3`, `pmtiles`, and PlatformIO assets

The Python code in `src/svalbard/` is primarily the provisioner that writes
this runtime to the drive, not the runtime itself.

## Design Decisions

### 1. Two-Phase Migration

The migration should be explicitly split:

#### Phase 1: Go launcher plus Go action adapters

- Introduce a Go launcher binary as the primary entrypoint on macOS/Linux.
- Introduce stable Go action IDs and adapter implementations for all runtime
  actions.
- Generate a new runtime config file at provision time. This replaces
  `entries.tab` as the canonical menu source.
- Keep the legacy shell implementations only as adapter backends during the
  transition.

#### Phase 2: Native Go runtime implementations

- Preserve the phase-1 action IDs and menu/runtime contracts.
- Port helper concerns now living in shell into Go.
- Replace each adapter's shell-backed implementation with native Go code.
- Remove `.svalbard/actions` and most of `.svalbard/lib` after parity is
  reached.

This split keeps the architecture stable while allowing the runtime logic to
move in controlled slices.

### 2. Runtime Responsibility Split

The responsibility split should be:

- Python provisioner: build-time inspection, drive assembly, artifact download,
  and generation of runtime metadata and launcher binaries
- Go drive runtime: runtime UX, action dispatch, runtime orchestration, host
  integration, and later native implementations of runtime behaviors

The Go runtime should not subsume the Python provisioner in this project.

### 3. Runtime Config as the New Canonical Menu Contract

Phase 1 should stop treating shell script paths as the menu contract. Instead,
the provisioner should generate a runtime config file that the Go launcher
reads directly.

The config should include:

- sections/groups
- display labels
- action IDs
- action arguments
- optional descriptions or status text
- visibility based on actual drive contents

Example conceptual shape:

```json
{
  "version": 1,
  "actions": [
    {
      "section": "browse",
      "label": "Browse encyclopedias",
      "action": "browse",
      "args": {
        "mode": "all"
      }
    }
  ]
}
```

The exact wire format may be JSON, YAML, or TOML. JSON is preferred for the
first cut because it is simple to emit from Python and straightforward to
consume from Go.

There should be one canonical runtime config source during migration to avoid
drift. `entries.tab` should not remain co-equal once the Go runtime ships.

### 4. Action Model

The runtime should define stable action IDs, for example:

- `browse`
- `search`
- `maps`
- `chat`
- `apps`
- `share`
- `serve-all`
- `verify`
- `inspect`
- `embedded-shell`

The TUI should know only:

- what actions exist
- how they are labeled
- what arguments they receive

The TUI should not know whether an action is implemented in shell or Go.

The runtime should provide an internal action execution boundary equivalent to:

- `ListActions(config)`
- `RunAction(actionID, args, driveRoot)`

Phase 1 binds this interface to shell-backed adapters. Phase 2 swaps those
adapters to native Go implementations without changing the menu layer.

### 5. TUI Interaction Model

Phase 1 should use a keyboard-only TUI. Bubble Tea is the preferred library
because it supports a polished custom terminal UI now and leaves room for
future mouse support without forcing a framework change.

Required interaction model:

- arrow keys and `j`/`k` for navigation
- `Enter` to launch
- `/` to start filtering
- `Esc` to clear the filter or back out of filter mode
- `q` to quit

The menu should visibly group actions by section and present a cleaner layout
than the current numeric shell menu.

### 6. Execution Model

When a user launches an action:

- the TUI suspends
- the selected Go action adapter runs with inherited stdio in the current
  terminal
- any browser-opening or long-running service behavior remains the
  responsibility of the action implementation
- when the action exits, the TUI resumes

This preserves the current flow while letting the launcher become much nicer.

### 7. Platform Scope

Phase 1 target platforms:

- macOS
- Linux

Windows is a future target, but phase 1 does not need to execute there.
However, the architecture must support adding Windows later without changing
the TUI/action contracts.

Implications:

- the phase-1 runtime can still rely on shell-backed adapters internally
- the public runtime contract must already be Go action IDs rather than shell
  script paths

### 8. Provisioning Output

Newly provisioned drives should include launcher binaries for all supported
host platforms by default unless the existing `--platform` switch narrows the
build.

Provision-time output should include:

- the Go launcher binaries
- the canonical runtime config
- legacy shell actions and helpers during phase 1 only
- bundled payload tools and content as today

The drive root entrypoint can be a platform shim, a small host-aware launcher
wrapper, or another mechanism that selects the correct binary. The exact
selection mechanism is an implementation detail for planning; the important
point is that the runtime contract is now the Go launcher.

### 9. Phase 1 Adapter Strategy

Phase 1 should port the launcher and action boundary, not the behavior.

Each action gets a Go adapter. In phase 1 those adapters may:

- validate the action arguments
- resolve the drive root and host platform
- shell out to the current `.svalbard/actions/*.sh` implementations on
  macOS/Linux
- normalize command invocation and error reporting

This provides immediate benefits:

- the TUI is no longer coupled to shell paths
- the runtime surface is prepared for Windows later
- phase 2 becomes incremental implementation replacement instead of another
  architecture change

### 10. Phase 2 Port Scope

Phase 2 ports helper concerns and action behaviors to native Go.

Helper concerns to port:

- platform detection
- binary lookup
- on-demand extraction/caching
- port selection
- process tracking and cleanup
- browser launching
- checksum verification

Suggested action port order:

1. `inspect`
2. `verify`
3. `share`
4. `maps`
5. `apps`
6. `browse`
7. `serve-all`
8. `embedded-shell`
9. `search`

This order starts with the simplest and most isolated actions and leaves the
highest-risk ports for later.

### 11. Search Is the Hardest Runtime Port

`search` deserves explicit treatment because it currently mixes:

- SQLite queries
- FTS query construction
- optional semantic reranking
- `llama-server` embedding mode
- `curl`
- an inline Python reranker
- Kiwix coordination and browser launching

It should be treated as the highest-risk phase-2 action and should not block
shipping the earlier phase-2 slices.

### 12. Embedded Shell Remains Shell-Adjacent

Even after phase 2, `embedded-shell` will likely remain partly host-shell
oriented because its purpose is to launch an interactive development shell with
prepared environment variables and toolchain paths.

The Go runtime should own:

- environment preparation
- toolchain extraction/caching
- shell command selection
- host-specific path logic

But the final handoff into an interactive shell is still expected.

## Package Shape

The Go runtime should use a structure along these lines:

- `cmd/svalbard-drive/main.go`
- `internal/menu`
- `internal/runtime`
- `internal/actions`
- `internal/config`
- `internal/platform`
- `internal/process`
- `internal/ports`
- `internal/browser`

The exact package tree can be refined during planning, but phase 1 should keep
the launcher, config loading, action registry, and action execution boundary
separate from the TUI rendering code.

## Risks

- The menu/config migration can drift if both `entries.tab` and a new runtime
  config remain canonical.
- A launcher-only rewrite without adapters would not materially reduce the
  shell dependency surface.
- Search parity may take noticeably longer than the other phase-2 ports.
- Windows support can be delayed safely, but only if action IDs and config stay
  shell-agnostic from phase 1 onward.

## Open Questions for Planning

- What exact runtime config path and format should the provisioner emit?
- How should the drive choose the correct launcher binary at the root level?
- Should the drive-side binary be named `svalbard-drive`, `svb-drive`, or
  something shorter for on-stick ergonomics?
- Should phase 1 keep a tiny compatibility `run.sh` shim on macOS/Linux during
  transition, or retire it immediately on newly provisioned drives?

## Acceptance Criteria

Phase 1 is complete when:

- a newly provisioned macOS/Linux drive launches into the Go TUI instead of the
  current shell menu
- the menu is driven by the new canonical runtime config
- all current top-level actions are launched through Go action adapters
- the menu no longer dispatches directly to raw shell script paths

Phase 2 is complete when:

- runtime helper behavior formerly in `.svalbard/lib/*.sh` is implemented in Go
- the current user-facing actions are implemented in Go behind unchanged action
  IDs
- the shell action layer is removable without changing the menu/runtime
  contracts
