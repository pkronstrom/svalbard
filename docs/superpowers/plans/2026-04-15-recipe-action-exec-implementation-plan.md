# Recipe Action Exec Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add barebones recipe-defined `action.type: exec` support, rename `.svalbard/runtime.json` to `.svalbard/actions.json`, and teach the Go launcher to execute both built-in and declarative actions.

**Architecture:** Keep built-in inferred actions, but serialize them into the same item-level `action` object shape as explicit recipe-defined actions. The Python provisioner becomes responsible for emitting `.svalbard/actions.json`, while the Go launcher loads an action manifest and resolves either built-in native actions or generic `exec` actions.

**Tech Stack:** Python 3.12, pytest, Go 1.25, Bubble Tea

---

### Task 1: Rename the generated manifest and add failing tests

**Files:**
- Modify: `src/tests/test_toolkit_generator.py`
- Modify: `src/tests/test_commands.py`
- Modify: `drive-runtime/internal/config/config_test.go`

- [ ] Update Python tests to read `.svalbard/actions.json` and expect item-level `action` objects.
- [ ] Add one Python test that writes an on-drive snapshot recipe with explicit `action.type: exec` and verifies it appears in the generated manifest.
- [ ] Update Go config tests to load `actions.json`-style item actions.
- [ ] Run:
  - `uv run pytest -q src/tests/test_toolkit_generator.py src/tests/test_commands.py -k "action or runtime or init_drive"`
  - `GOCACHE=$(pwd)/.gocache go test ./internal/config`

### Task 2: Emit `.svalbard/actions.json` from Python

**Files:**
- Modify: `src/svalbard/models.py`
- Modify: `src/svalbard/toolkit_generator.py`

- [ ] Add recipe-side `action` metadata to `Source`.
- [ ] Rename generator output from `runtime.json` to `actions.json`.
- [ ] Emit item-level `action` objects:
  - built-ins as `type: builtin`
  - explicit recipe actions as `type: exec`
- [ ] Keep explicit recipe actions overriding inferred built-ins for that item.
- [ ] Re-run the focused Python tests until green.

### Task 3: Load and execute action objects in Go

**Files:**
- Modify: `drive-runtime/internal/config/config.go`
- Modify: `drive-runtime/internal/menu/model.go`
- Modify: `drive-runtime/internal/actions/actions.go`
- Modify: `drive-runtime/cmd/svalbard-drive/main.go`

- [ ] Replace `MenuItem.Action + Args` with an `ActionSpec`.
- [ ] Load `.svalbard/actions.json` in the launcher.
- [ ] Teach the menu to dispatch `ActionSpec` instead of raw action string/args.
- [ ] Extend the action resolver with:
  - built-in native actions from the manifest
  - generic `exec` action execution with `resolve_from`, `args`, `env`, `cwd`, `mode`
- [ ] Keep `interactive`, `capture`, and `service` modes working.
- [ ] Re-run focused Go tests:
  - `GOCACHE=$(pwd)/.gocache go test ./internal/actions ./internal/config ./internal/menu`

### Task 4: Full verification

**Files:**
- Modify only if verification exposes breakage

- [ ] Run:
  - `uv run pytest -q src/tests/test_toolkit_generator.py src/tests/test_commands.py`
  - `GOCACHE=$(pwd)/.gocache go test ./...`
- [ ] Fix any regressions without expanding scope beyond barebones `exec`.
