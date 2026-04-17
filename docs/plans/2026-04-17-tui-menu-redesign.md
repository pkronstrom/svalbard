# TUI Menu Redesign

## Overview

Redesign the Svalbard TUI main menu and screen flows. The current dashboard has 7 flat items (Overview, Add Content, Remove Content, Import, Plan, Apply, Presets) with a command palette (Ctrl+K). This is replaced with a tighter menu, live preview panes, and coherent screen flows.

## Menu Structure

### Dashboard (vault found)

```
1  Status       vault health & sync state
2  Browse       explore & select content
3  Plan         preview pending changes
4  Import       local files, URLs, YouTube
5  Index        keyword & semantic search
─────────────────────────────────────────
6  New Vault    init wizard for another vault
```

### Welcome (no vault)

```
1  New Vault    full setup wizard
2  Open Vault   point to existing vault
3  Browse       explore available content & presets (read-only)
```

## Layout

Two-pane master-detail. Left pane: nav list with number, label, and short description. Right pane: live preview of the selected item. At < 80 columns, stacks vertically with a compact 1-2 line summary below the nav instead of the right pane.

Header shows app name, vault identity, and ambient status (sync state, disk usage). Footer shows context-sensitive key hints.

## Navigation & Keys

| Key | Behavior |
|-----|----------|
| Arrow keys / j/k | Move cursor |
| 1-6 | Jump directly to item |
| Enter | Enter selected screen |
| Esc / q | Back to previous screen. If unsaved changes, prompt (y/n/esc) |
| Ctrl+C | Force quit, discard everything, no prompt |
| ? | Full help overlay |

## Screens

### Status (right pane only)

No sub-screen. The right pane shows live vault data when Status is selected in the nav:

- Vault path and active preset
- Desired / realized / pending item counts
- Disk used / free
- Last apply timestamp
- Pending change summary ("+2 to download, -1 to remove")

### Browse (full screen)

Full catalog tree with checkboxes. Everything in the catalog is shown; checked items are in the desired state.

```
 Browse                                        4 selected · 42.1 GB
─────────────────────────────────────────────────────────────────────────
  ▾ Maps & Geodata                                    [✓] 2/2
      [✓] osm_nordic.pmtiles        3.1 GB   OpenStreetMap Nordic
      [✓] osm_europe.pmtiles       12.4 GB   OpenStreetMap Europe
  ▾ Reference & Knowledge                             [~] 1/3
      [✓] wikipedia_en.zim          8.2 GB   English Wikipedia
      [ ] wikipedia_es.zim          4.1 GB   Spanish Wikipedia
      [ ] wiktionary_en.zim         1.8 GB   English Wiktionary
  ▸ Literature                                        [ ] 0/5
  ▸ Education                                         [ ] 0/4
```

- Space: toggle item or entire group
- Enter: expand/collapse group
- Group state: [✓] all, [~] partial, [ ] none
- Preset picker built in (p hotkey swaps preset, updates checkboxes)
- When entering from a preset, diff indicators show: `+` new, `-` removed vs current state
- Changes accumulate in memory (not saved on toggle)
- Esc/q: if changes exist, prompt "Save changes? (y/n/esc)". y saves to manifest, n discards, esc stays in Browse.
- Browse always returns to the dashboard menu after save/discard, even when entered via Plan's `b` shortcut.

### Plan (full screen)

Shows all pending changes between desired and realized state. Reachable from main menu or as next step after Browse.

```
 Plan                                          3 changes · 11.3 GB net
──────────────────────────────────────────────────────────��──────────────
  + wikipedia_en.zim          8.2 GB   download
  + osm_nordic.pmtiles        3.1 GB   download
  - gutenberg_es.zim          1.4 GB   remove
  ↑ wiktionary_en.zim         1.2 GB   update (0.9 → 1.2 GB)

                          Download:  11.3 GB
                          Remove:     1.4 GB
                          Free after: 72.9 GB
─────────────────────────────────────────────────────────────────────────
 enter apply · b browse · esc back · q quit
```

- `+` add, `-` remove, `↑` update
- Enter: starts Apply (download/sync progress)
- b: jump to Browse to adjust selections
- Esc/q: back to menu
- "Everything in sync" message when nothing is pending

### Apply (sub-screen of Plan)

Live progress. Reachable only via Enter on Plan.

```
  ✓  gutenberg_es.zim          1.4 GB   removed
  ·  wikipedia_en.zim          8.2 GB   downloading  [████████░░░] 73%
     osm_nordic.pmtiles        3.1 GB   queued
```

Progress symbols (subtle):
- (no symbol, dimmed text) queued
- `·` in progress
- `✓` done
- `✗` failed

Removes run first (free up space), then downloads. Summary on completion. Enter to return to menu.

### Import (full screen)

Text input for path or URL. Imports and adds to desired state in one shot, no confirmation step. Stays on screen for multiple imports.

```
  Path or URL: /home/user/survival-guide.pdf

  ✓ Imported local:survival-guide (12.4 MB)
    Added to desired state.

  Path or URL: _
```

Esc to return to menu.

### Index (full screen)

Two-item list with detail pane (master-detail within the sub-screen). Shows keyword and semantic search as separate capabilities.

```
  1  Keyword search     fast exact matching              enabled
  2  Semantic search    meaning-based, heavier           disabled
```

Right pane shows details for selected index type:
- Keyword: SQLite FTS5, fast, zero dependencies. Shows last built time, source/article counts.
- Semantic: embedding-based using nomic-embed-text-v1.5, finds conceptually related content. Requires ~140 MB model download. Shows install state.

Enter on keyword: rebuilds index (progress view).
Enter on semantic (disabled): prompts to enable (downloads model, builds embeddings).
Enter on semantic (enabled): rebuilds embeddings.

### Presets (within Browse)

No separate Presets screen. Preset picker is built into Browse via `p` hotkey. Selecting a preset updates checkboxes to match, with diff indicators showing what changed vs current state. Presets sorted by size (descending), then name.

### New Vault

Launches the existing init wizard: path → preset → customize → confirm. From the dashboard, accessible as item 6 (below separator). From the welcome screen, item 1.

## Welcome Screen Sub-Screens

### Open Vault

Text input for path. Validates that the path contains a vault (manifest.yaml). On success, transitions to dashboard. On failure, inline error.

### Browse (welcome)

Same catalog tree as dashboard Browse but read-only. No checkboxes, no toggles. Explore what's available before committing to a vault.

## Changes Required

1. Delete `tui/palette.go`, `tui/palette_model.go`, and palette tests
2. Remove palette fields, Ctrl+K handling, and `buildHostPaletteEntries` from dashboard
3. Replace `hostDestinations` with new 6-item menu (5 + separator + New Vault)
4. Add separator support to `NavList` component
5. Add two-line rendering to `NavList` (title + description, description dimmed)
6. Implement live right-pane previews per menu item (replacing static descriptions)
7. Add number key handlers (1-6 dashboard, 1-3 welcome)
8. Add `?` help overlay
9. Implement narrow layout variant (compact summary below nav at < 80 cols)
10. Update welcome screen to 3 items (New Vault, Open Vault, Browse)
11. Implement Browse screen (pack tree, checkboxes, preset picker, save prompt)
12. Implement Plan screen (diff view, Enter to apply, b to browse)
13. Implement Apply progress view (progress indicators, removes first)
14. Implement Import screen (text input, one-shot import+add)
15. Implement Index screen (keyword/semantic list, enable/rebuild)
16. Implement Open Vault screen (path input, validation)
