## Context

Production palette entries use only ID, label, and aliases; navigation items are never disabled; image conversion manually copies pixels; builder dispatch has one special-map entry. Tests preserve branches that production does not exercise.

## Goals / Non-Goals

**Goals:**
- Delete unused flexibility and use direct standard-library/native constructs.
- Preserve every production path and dispatch precedence.
- Add conversion coverage for image bounds and pixels.

**Non-Goals:**
- New palette syntax, navigation features, builder families, or dependencies.
- Broader TUI or builder redesign.

## Decisions

- Palette matching retains case-insensitive ID, label, and alias matching only.
- Navigation movement becomes bounded single-step movement; `Clamp` only enforces bounds.
- Non-RGBA images are copied with `draw.Draw` using source bounds and `draw.Src`; existing RGBA values retain identity.
- Builder dispatch uses a direct `python-venv` condition after explicit pipeline steps and before app-bundle conversion.

## Risks / Trade-offs

- Removed exported struct fields are source-incompatible for downstream callers, but repository evidence shows no production use.
- Non-zero image origins must be preserved by tests.
- Dispatch order must remain explicit to avoid changing which builder wins.
