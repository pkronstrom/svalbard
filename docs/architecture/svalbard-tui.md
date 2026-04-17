---
globs:
- tui/**
- host-tui/**
rook-arch: true
scope: module
---

# Svalbard TUI Architecture

## Purpose

Interactive terminal UI for provisioning and managing offline knowledge vaults. Provides a two-pane dashboard (master-detail) for vault operations: browsing content, planning changes, applying downloads, importing files, and managing search indexes.

## Module Boundaries

**Inside:**
- `tui/` — shared visual components: ShellLayout, NavList, DetailPane, Theme, KeyMap, FooterHints
- `host-tui/` — host-side TUI app: dashboard, welcome screen, wizard, screen routing (launch.go)
- `host-tui/internal/` — screen implementations: dashboard, welcome, wizard stages

**Outside:**
- `host-cli/` — CLI commands and business logic (catalog, manifest, planner, apply)
- `drive-runtime/` — separate TUI app for the drive/device side (shares `tui/` components)

## Public Interfaces

- `tui.Theme`, `tui.KeyMap`, `tui.NavList`, `tui.DetailPane`, `tui.ShellLayout` — reusable components
- `tui.NavItem` — nav list item with ID, Label, Description, Separator, Disabled flags
- `tui.KeyBinding` with `Matches(tea.KeyMsg)` — key input matching
- `hosttui.RunInteractive(*WizardConfig)` — main entry point
- `hosttui.RunInitWizard(WizardConfig)` — direct wizard entry
- `hosttui.WizardConfig`, `hosttui.WizardResult` — wizard data types
- Screen message types: `welcome.SelectMsg`, `wizard.DoneMsg`, `wizard.BackMsg`

## Dependencies

- `charmbracelet/bubbletea` — TUI framework (Model-View-Update)
- `charmbracelet/lipgloss` — styling and layout
- `host-cli/internal/` — business logic consumed via CLI commands or shared types
- `tui/` is a leaf package — no dependencies on `host-tui/` or `host-cli/`

## Data Flow

1. CLI entry (`host-cli`) resolves vault, builds WizardConfig, calls `hosttui.RunInteractive()`
2. `appModel` in launch.go routes between screens: welcome, dashboard, wizard
3. Each screen is a Bubble Tea model receiving `tea.Msg` and returning `(tea.Model, tea.Cmd)`
4. Screens emit transition messages (e.g., `welcome.SelectMsg`) consumed by `appModel`
5. Shared `tui/` components render layout — screens provide pre-rendered Left/Right pane strings

## Constraints

- All `tui/` components must be stateless renderers or pure-functional (no side effects)
- Adaptive layout: two-pane at >= 80 columns, vertical stack below
- `tui/` must not import `host-tui/` or `host-cli/` (leaf package)
- Keyboard: Esc/q = back, Ctrl+C = force quit, number keys = direct jump
- No command palette — removed by design (menu + number keys are sufficient)
