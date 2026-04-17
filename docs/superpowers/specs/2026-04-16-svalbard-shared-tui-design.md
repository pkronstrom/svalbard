# Svalbard Shared TUI Design

Date: 2026-04-16
Status: Approved for planning

## Summary

Svalbard should use one shared terminal UX language across the host-side
`svalbard` CLI and the drive-side `svalbard-drive` runtime.

The two products should not use identical home screens. They have different
jobs:

- host `svalbard` manages desired vault state
- drive `svalbard-drive` uses realized vault contents

They should, however, feel like the same product through a shared shell,
interaction grammar, screen vocabulary, and visual system.

The recommended UX model is:

- plain `svalbard` opens a dashboard-first TUI for the current vault, or a
  TUI welcome state when no vault can be resolved
- `svalbard init [PATH]` opens a guided setup flow using the same shell
- plain `svalbard-drive` opens a dashboard-first TUI for using the provisioned
  vault
- a global command palette is always available as the expert fast lane

## Goals

- Make plain `svalbard` the most useful entrypoint for everyday vault work.
- Keep the host-side and drive-side TUIs recognizably related without forcing
  identical screens.
- Make the main workflows obvious:
  - host: edit desired state, review plan, apply changes
  - drive: browse, search, maps, chat, apps, share, verify
- Use Bubble Tea and the Charm stack to build a distinctive product shell, not
  a stock demo-app aesthetic.
- Preserve direct command verbs for scripting, automation, and LLM use.

## Non-Goals

- Making the host and drive TUIs use the exact same information architecture.
- Designing a mouse-first UI.
- Replacing command-line verbs with TUI-only interactions.
- Building read-only verbs like `inspect`, `show`, or `list` into the first
  host TUI milestone.

## Design Decisions

### 1. Shared UX Language, Purpose-Built Homes

The host and drive applications should share:

- navigation model
- shell layout conventions
- visual roles and color semantics
- screen types
- modal and palette behavior
- progress and review patterns

They should not share the same home screen content.

Recommended split:

- host `svalbard`: home is a vault-control dashboard
- drive `svalbard-drive`: home is a use-the-vault dashboard

This is preferable to an identical shell because the products have different
primary nouns and different “what should I do next?” questions.

### 2. Default Entry Points

The default entrypoints should be:

- `svalbard`
  - resolve the vault from `cwd` or nearest parent containing `manifest.yaml`
  - if a vault is resolved, open the current vault dashboard
  - if no vault is resolved, open a host-side welcome screen in the same shell
    language rather than falling back to a plain CLI error
- `svalbard init [PATH]`
  - open the guided setup flow
- `svalbard-drive`
  - open the drive-use dashboard

This split creates a clear product model:

- operational home for repeat usage
- guided setup flow for first-time creation

### 3. Home Screen Model

The recommended shell is a restrained two-pane operator console:

- left pane: sections and primary actions
- right pane: contextual summary, preview, warnings, and consequences
- top bar: app name, vault/drive identity, status badge
- footer: stable key hints

This is preferable to:

- a single-column layout, which becomes cramped once plan and status context
  matter
- a card-grid home, which wastes space and scales poorly for expert use
- a pure action palette home, which hides the system state users need before
  `plan` and `apply`

### 4. Host Home Screen

The host-side `svalbard` home should emphasize vault control.

The left pane should contain primary destinations, not direct destructive
actions. Every row on the home screen is a navigable destination that opens a
dedicated screen or focused flow when activated with `Enter`.

The host home destinations are:

- `Overview`
- `Add Content`
- `Remove Content`
- `Import`
- `Plan`
- `Apply`
- `Presets`

Activation semantics:

- `Overview`
  - opens a fuller vault summary view or keeps focus in the home shell
- `Add Content`
  - opens the picker/editor flow for adding desired items
- `Remove Content`
  - opens the picker/editor flow scoped to current desired items
- `Import`
  - opens the import flow
- `Plan`
  - opens the plan/review screen
- `Apply`
  - opens the plan/review screen in execution mode; it does not execute
    immediately from the home dashboard
- `Presets`
  - opens a preset-focused picker/editor flow

Recommended right-pane behavior:

- `Overview`
  - vault name and resolved path
  - desired item count
  - preset provenance
  - pending plan summary
  - unmanaged file count
  - last apply result
  - next useful actions
- `Add Content`
  - what kinds of content can be added
  - current desired count
  - recommended categories or recent additions
- `Plan`
  - compact diff summary
  - estimated size delta
  - regeneration needs for index/runtime assets
- `Apply`
  - execution readiness summary
  - warning if there is no pending work
  - warning if unmanaged files or conflicts exist

This home should answer three questions immediately:

- where am I?
- what state is this vault in?
- what should I do next?

### 5. Drive Home Screen

The drive-side `svalbard-drive` home should emphasize using the vault.

The drive home also uses left-pane destinations rather than a flat launch list.

The drive home destinations are:

- `Browse`
- `Search`
- `Maps`
- `Chat`
- `Apps`
- `Share`
- `Verify`
- `Embedded`

Activation semantics:

- `Browse`, `Search`, `Maps`, `Chat`, `Apps`, and `Embedded`
  - open focused subviews or pickers inside the TUI first
- `Share` and `Verify`
  - open readiness/review-oriented screens before long-running work starts

Recommended right-pane behavior:

- `Browse`
  - archive count
  - featured sources
  - short explanation of the browsing flow
- `Search`
  - indexed source/article counts
  - whether semantic search is available
  - quick explanation of search behavior
- `Maps`
  - layer count
  - region coverage summary
- `Chat`
  - available models
  - available clients
  - runtime readiness
- `Apps`
  - bundled tool count
  - a few featured tools
- `Verify`
  - integrity summary
  - last verification status

This home should feel like a launch-and-use surface, not a configuration UI.

#### Drive section visibility rules

Drive sections fall into two categories:

- core sections that are always present
- capability sections that are present only when the drive meaningfully ships
  that capability

Always-present core sections:

- `Browse`
- `Search`
- `Verify`

Capability sections:

- `Maps`
- `Chat`
- `Apps`
- `Share`
- `Embedded`

Visibility policy:

- a section is `visible but disabled` when it represents a core capability the
  user should understand exists, but the current drive is missing the required
  data or preparation
- a section is `hidden entirely` when the capability is not part of the drive's
  intended product surface and showing it would add noise

Examples:

- `Search` remains visible but disabled when no search index exists yet
- `Browse` remains visible but disabled when no browseable archives are present
- `Maps` is hidden entirely when the drive ships no map data or map-opening
  capability
- `Chat` is hidden entirely when the drive ships no local AI capability
- `Apps` is hidden entirely when there are no bundled runtime apps
- `Share` is hidden entirely when the drive does not ship file-sharing support
- `Embedded` is hidden entirely when the drive does not ship embedded/toolchain
  capability

### 6. Shared Screen Vocabulary

Both applications should reuse the same small set of screen types:

#### Home dashboard

The everyday landing screen with two-pane navigation and summary context.

#### Picker / editor

Used for:

- host: add/remove content, choose presets, select imported items
- drive: choose sources, apps, models, clients, or maps

The picker should have:

- grouped rows in the navigation pane
- a right-side detail preview for the focused row
- optional local filtering
- optional multi-select summary

#### Plan / review

Used for host reconciliation review and, where useful, drive-side review flows
such as verification or service launch confirmation.

The review screen should group changes such as:

- downloads
- removals
- updates
- unmanaged files
- runtime/index regeneration work

#### Progress / run

Used for:

- host `apply`
- imports
- indexing
- drive-side runtime actions with meaningful duration

The progress screen should emphasize:

- current phase
- completed steps
- active step
- errors
- optional expandable logs

Logs should never be the first or only thing the user sees.

#### Result / detail

A focused detail view for one selected source, change, warning, or result.

#### Command palette

A global fast lane for experts. It complements the dashboard rather than
replacing it.

### 7. Wizard Model For `svalbard init`

The setup flow should use the same shell language, not a separate visual mode.

Recommended step sequence:

- `Vault Path`
- `Choose Preset`
- `Adjust Contents`
- `Review Plan`
- `Apply`

`PATH` handling must be explicit:

- when `svalbard init` is run without a path argument, the `Vault Path` step is
  active and editable
- when `svalbard init /some/path` is run with a path argument, the `Vault Path`
  step is prefilled with that path and remains editable by default
- the step is not skipped in milestone 1 because the user still needs to review
  path consequences such as existing files, odd targets, or removable media
  choices
- if the prefilled path is already valid, the user can advance immediately

Recommended layout:

- left pane: ordered steps
- right pane: explanation, preview, and consequences of the current step

This makes `init` feel like a constrained guided workflow within the same
product shell, instead of a disconnected wizard app.

### 8. Interaction Grammar

The keyboard model should be stable across the entire product.

Core bindings:

- `j` / `k` and arrow keys: move selection
- `Enter`: open, confirm, or launch the focused action
- `Esc`: back out one layer, close a modal, or clear a transient mode
- `/`: start local filtering in the current screen
- `Ctrl+K`: open the global command palette
- `Space`: toggle selection in multi-select pickers
- `Tab`: switch active region when a screen has multiple focusable panes
- `q`: quit only from top-level non-destructive views

Behavior rules:

- `Esc` unwinds one layer only
- `Enter` must not perform destructive work without a review step
- expensive work should flow through review before execution unless the user
  explicitly requested direct execution via a CLI verb
- filtering should be lightweight and easy to abandon
- long-running actions should show whether they can be canceled safely

### 9. Visual Language

The recommended tone is a calm field console:

- dark-first, but not neon
- compact, high-signal rows
- low-chroma base colors
- one primary accent color for focus
- semantic colors reserved for real status meaning
- whitespace and hierarchy preferred over heavy panel nesting

Visual role guidance:

- `base`: slate or graphite background
- `focus`: one signature accent, likely a cold blue, ice cyan, or muted green
- `success`: completed or healthy state only
- `warning`: pending changes, drift, or review-required state
- `danger`: failures and destructive review only
- `muted`: metadata, paths, and hints

Typography and layout guidance:

- bold for titles and focused selections only
- dim descriptions and metadata
- sparse borders
- avoid nested panel soup

The current drive runtime in `drive-runtime/internal/menu/` is the right
stylistic foundation, but it should evolve from a functional launcher into a
stronger product shell.

### 10. Charm Stack Usage

The recommended implementation strategy is:

- custom shell and layout
- selective use of Bubbles
- Lip Gloss for product-specific layout and style roles

Use Bubbles for:

- text input
- viewport
- spinner and progress
- other focused primitives where the default behavior is already strong

Do not make the main product shell depend on stock list widgets for its core
identity. The dashboard, picker rows, review lists, and palette result layout
should feel like Svalbard components rather than generic library demos.

`gum` is useful for one-off shell interactions, but it should not define the
product language of the in-app TUI.

### 11. Command Palette Role

The command palette should be global and always available, but it should not
be the home screen.

Milestone 1 palette scope is intentionally limited. It should index:

- host top-level destinations and actions
- drive top-level destinations and actions
- host presets
- host desired items and imported local items
- drive browseable sources, runtime apps, and available local AI clients/models

Milestone 1 should not attempt freeform natural-language parsing. Matching
should be label-and-alias based, with optional lightweight verb prefixes such
as `add`, `remove`, `apply`, `browse`, or `open`.

Milestone 1 may support a small whitelist of verb + freeform trailing argument
forms, but only when the receiving flow already accepts a single raw input and
can safely route into the normal reviewed UI path.

Approved milestone-1 examples:

- `import /path/to/manual.pdf`
- `import https://example.com`
- `import https://youtube.com/...`

These palette entries should open the normal import flow with the input
prefilled. They should not bypass the ordinary review, destination selection,
or optional `--add` decisions.

Non-import freeform parsing is out of scope for milestone 1.

Recommended use:

- jump to actions
- jump to content or sources
- trigger direct flows that still route through the same review or picker
  screens as ordinary navigation when review is required
- later support more forgiving command-like phrases after the index and routing
  model are stable

Examples:

- `apply`
- `add wikipedia`
- `remove ifixit`
- `import manual.pdf`
- `browse wikipedia`
- `open maps`

The palette is the expert fast lane. The dashboard remains the trust-building
and context-bearing default surface.

### 12. Empty States And Zero-State Behavior

The shared language should include deliberate empty states.

Host-side examples:

- no vault found
  - show a simple host-side welcome screen in the normal shell
  - include `Init Vault` and `Choose Preset` as primary destinations
  - do not fall back to a plain CLI error for interactive `svalbard`
  - `Choose Preset` opens preset browsing inside the welcome shell and then
    enters the normal `init` flow with the chosen preset preselected while
    keeping the `Vault Path` step active
- empty vault
  - emphasize `Add Content`, `Import`, and starter presets
- clean vault with no pending changes
  - make `Apply` visually quiet

Drive-side examples:

- no search index
  - explain what is unavailable and what still works
- no maps or no local models
  - keep the section visible only when that absence helps user understanding;
    otherwise hide unavailable sections entirely

### 13. Shared Trust Model

The TUI should make expensive and destructive actions legible.

Rules:

- host `add` and `remove` are state edits, not execution
- host `plan` previews reconciliation
- host `apply` is the only broad execution verb
- drive in-TUI navigation transitions are immediate
- drive actions that start background services, bind ports, perform verification,
  or consume meaningful resources must go through a readiness/review screen
- opening an external browser or app is not, by itself, confirmation-worthy if
  it is the explicit expected final step of a focused flow such as selecting a
  source in `Browse`
- when a drive action will start a service and then open a browser, the review
  boundary belongs before the service start rather than before the browser open
- drive runtime actions may launch services or browsers, but the screen before
  launch must make the consequence legible whenever execution leaves the TUI or
  creates background state

The UI should prefer explicit review, visible status, and stable summaries over
“magic” background behavior.

## Consequences

This design means:

- the existing drive runtime should be treated as the first reference
  implementation of the shared UX language
- the future host-side Go TUI should not be a command menu clone; it should be
  a vault dashboard built from the same interaction system
- both products need a small internal design-system layer for shell, panes,
  banners, filters, review lists, and progress views
- the design system should be shared conceptually first; code sharing can be
  deferred

## Implementation Notes For Planning

The first implementation plan should focus on:

1. extracting the shared shell grammar and reusable component vocabulary
2. redesigning the drive runtime home into the stronger dashboard model
3. designing the host-side `svalbard` home, `init` wizard shell, and
   `plan/apply` review/progress flows
4. defining how the command palette indexes actions and objects in both apps

The first implementation plan should not attempt:

- full code sharing between host and drive modules
- full visual polish before the interaction grammar is stable
- speculative read-only verbs that are not yet part of the agreed host CLI
