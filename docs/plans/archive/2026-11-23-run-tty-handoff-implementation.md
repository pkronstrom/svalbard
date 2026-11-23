# `./run` terminal handoff: implementation

Status: done 2026-11-23

Goal: an interactive action started from the command line (`./run search`, `./run embedded`, any alias) can read the keyboard. Today it stops on its first read.

## Design

### Decisions
- `runActionProcess` keeps starting the action in its own process group, so cancellation still reaps the whole group.
- When the action is interactive (`ModeExecProcess`) and stdin is a terminal, that group becomes the terminal's foreground group at start (`SysProcAttr.Foreground` with `Ctty` set to stdin). When the action ends, the runtime takes the foreground back, ignoring SIGTTOU while it does.
- Whether an action takes the terminal is decided from its own stdin: a terminal whose foreground group is the runtime's. Captured actions and runs without a terminal on stdin behave exactly as before.
- Ctrl+C in the terminal reaches the action's group only. The runtime returns when the action exits, then ends anything left in that group. SIGTERM to the runtime still cancels and reaps.
- Ctrl+Z does nothing to an action started from the command line: a suspended action is continued at once, because the runtime could not hand a stopped action's terminal back to the calling shell. Job control inside a shell the action opens is unaffected.
- SIGTTOU stays ignored in the runtime after it takes the terminal back; the runtime runs one action and exits.
- Test dependency: `github.com/creack/pty`, test-only, to give a helper process a real controlling terminal.

### Invariants
- A cancelled action leaves no process from its group (`TestRunResolvedActionContextReapsChildProcessGroup` stays green).
- Piped stdin keeps working: the Phase 5 acceptance harness pipes commands into `run embedded`.
- After the action ends, the calling shell owns the terminal again.

### Non-goals
- `--help`, more aliases, passing arguments after an alias (Phase 2).
- A Windows build of the runtime (Phase 5).
- The menu path, which uses `tea.ExecProcess` and already works.

### Relevant existing architecture
- `drive-runtime/cmd/svalbard-drive/main.go`: `run` → `runResolvedAction` → `runResolvedActionContext` → `runActionProcess`.
- `drive-runtime/internal/actions/actions.go`: `ResolvedAction{Mode, Cmd}`; builtins re-exec the runtime as `__native-<id>`.
- Evidence: issue 6 in `docs/plans/research/2026-11-21-drive-menu-ux.md`; reproduced in tmux on 2026-11-23 (child group in state `T`, terminal foreground still the runtime).

## Chunk 1 — Foreground handoff for interactive actions

### Goal
Interactive command-line actions own the terminal while they run and give it back after.

### Files
- `drive-runtime/cmd/svalbard-drive/main.go`
- `drive-runtime/cmd/svalbard-drive/main_test.go`
- `drive-runtime/go.mod`, `drive-runtime/go.sum`

### Depends on
None.

### Follow this existing pattern
`runActionProcess` itself, and `TestRunResolvedActionContextReapsChildProcessGroup` for test style (shell-script fixtures, deadlines, cleanup).

### Implementation notes
- `runActionProcess` learns whether the action is interactive from its caller; only `ModeExecProcess` with a terminal on `os.Stdin` gets `Foreground`.
- Reclaim the foreground on every exit path, including cancellation, before returning.
- The terminal test uses the helper-process pattern: the test re-executes its own binary under a pty; the helper runs `runResolvedActionContext` on `sh -c 'read line; echo got:$line'`; the test writes a line to the pty and expects `got:<line>` before a deadline, then a clean exit.

### Edge cases
- stdin is not a terminal: no foreground change, no ioctl.
- The action exits non-zero: the foreground is still reclaimed and the error returned.
- Cancellation while the action holds the terminal.

### Acceptance
- New pty test fails before the change and passes after; the reap test still passes.
- `cd drive-runtime && go test -race ./cmd/svalbard-drive` and `go vet ./...` pass; `make verify` passes.
- In tmux: an alias for `activate-shell` gives a usable shell; a command typed there runs; Ctrl+C interrupts a `sleep` inside it without ending the shell; `exit` returns to a usable outer shell.
- `printf 'echo piped-ok\nexit\n' | svalbard-drive <alias>` prints `piped-ok` and exits 0.
- Final gate: `./run embedded` typed in a real terminal on a freshly applied `esp32-dev` vault opens the shell and `pio --version` runs.

### Do not
- Change `actions`, `embedded`, `shell` or the menu.
- Remove the process group or the reap logic.
