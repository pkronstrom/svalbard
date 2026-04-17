---
globs:
- host-tui/**
rook-arch: true
scope: module
---

# Host TUI Architecture

## Purpose
Terminal user interface for managing Svalbard vaults. Provides interactive screens for browsing content catalogs, reviewing plans, importing files, managing indexes, and initializing new vaults via a wizard.

## Module Boundaries
- **Inside**: Screen models (welcome, dashboard, browse, plan, import, index, wizard, openvault), app-level routing (`launch.go`), CLI entry point (`cmd/svalbard-tui/main.go`).
- **Outside**: Shared TUI primitives live in `tui/` (ShellLayout, NavList, DetailPane, KeyMap, Theme). Business logic is injected via `DashboardDeps` callbacks — the TUI never touches storage or networking directly.

## Public Interfaces
- `RunInteractive(wizardConfig, deps)` — main entry: resolves vault from CWD, launches dashboard or welcome.
- `RunInitWizard(config)` — direct entry to the init wizard.
- `DashboardDeps` — callback struct injected by the CLI layer: `LoadStatus`, `LoadDesiredItems`, `LoadPlan`, `SaveDesiredItems`, `RunApply`, `RunImport`, `RunIndex`, `RebuildForVault`.
- `WizardConfig` — catalog data (PackGroups, Presets) plus `InitVault`/`RunApply` callbacks.

## Dependencies
- `tui/` — shared layout and widget primitives (ShellLayout, NavList, DetailPane, KeyMap, Theme).
- `github.com/charmbracelet/bubbletea` — Elm-architecture TUI framework.
- `github.com/charmbracelet/lipgloss` — terminal styling (via `tui/`).
- Internal screen packages (`internal/welcome`, `internal/dashboard`, etc.) are private to this module.

## Data Flow
1. CLI constructs `DashboardDeps`/`WizardConfig` with callbacks wired to real vault operations.
2. `appModel` (launch.go) is the top-level Bubble Tea model. It owns all screen models and routes messages.
3. Each screen emits typed messages (`SelectMsg`, `BackMsg`, `SavedMsg`, `DoneMsg`) to signal navigation.
4. `appModel.Update` intercepts these messages to switch `m.screen` and construct new screen models.
5. `appModel.View` delegates to the active screen's `View()`.
6. Screen transitions send `tea.WindowSizeMsg` via `sendSize()` so the new screen renders at correct dimensions.

## Constraints
- Screen transitions must always send `sendSize()` so the new screen receives terminal dimensions.
- Screens that can be entered from multiple parents must track `prevScreen` to return correctly.
- Zero-value screen models must never be rendered — always construct via `New()` before switching to a screen.
- All key handling depends on `tui.DefaultKeyMap()` — zero-value `KeyMap` produces dead key bindings.
