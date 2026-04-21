---
globs:
- tui/**
- host-tui/**
rook-arch: true
scope: system
---

# Svalbard TUI Architecture

## System Modules

- **tui/** — Shared TUI component library (Go). Framework-agnostic layout primitives, navigation, tree picker, progress views, styling, keyboard bindings. No business logic.
- **host-tui/** — Host-side TUI application (Go, Bubble Tea). Screen orchestration, screen models (dashboard, wizard, browse, plan, import, index, welcome, openvault). Imports from tui/ only.
- **host-cli/** — CLI backend with business logic. Vault operations, builders, pipelines. Never imported by host-tui/ directly — decoupled via DashboardDeps callbacks.

## Module Boundaries

- `host-tui/` imports `tui/` (shared components)
- `host-tui/` does NOT import `host-cli/` — decoupled via `DashboardDeps` struct (callbacks + data structs)
- `tui/` has no internal dependencies on host-tui or host-cli
- Screen packages under `host-tui/internal/` are independent of each other — coordination happens in `launch.go`

## Cross-Cutting Concerns

- **Styling**: Centralized `tui.Theme` with semantic roles (Base, Focus, Success, Warning, Danger, Muted, Status, Error)
- **Keyboard**: `tui.KeyMap` provides standard bindings; screens customize footer hints via `tui.FooterHints()`
- **Layout**: `tui.ShellLayout` handles responsive 2-pane layout for all screens
- **Navigation**: Message-based screen transitions via Bubble Tea message dispatch in `launch.go`

## Data Flow

1. User input → Bubble Tea KeyMsg → active screen's Update()
2. Screen emits navigation messages (BackMsg, SelectMsg, DoneMsg) → appModel routes to next screen
3. Business logic accessed via DashboardDeps callbacks (async operations report progress via channels)
4. Screen's View() → ShellLayout.Render() → terminal output

## Extensibility Strategy

- New screens: add package under `host-tui/internal/`, wire in `launch.go`
- New shared components: add to `tui/` package
- New business logic: add to `host-cli/`, expose via DashboardDeps callbacks

## Constraints

- All TUI rendering must work at 80+ chars wide and degrade gracefully below 80 chars
- No direct host-cli imports from host-tui — callback-only boundary
- Bubble Tea message-driven architecture — no shared mutable state between screens
