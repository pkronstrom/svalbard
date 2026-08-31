## Context

`StripAnsi` is exported by the standalone `tui` module and referenced only by repository tests. Repository-local dead-code evidence cannot establish that external consumers do not use it.

## Goals / Non-Goals

**Goals:**
- Determine the module's compatibility expectations and known downstream use.
- Prefer retaining the export when safety is uncertain.
- Remove or unexport only with evidence.

**Non-Goals:**
- Rewriting ANSI parsing or shell rendering.
- Making this optional cleanup block the main refactors.

## Decisions

- Check module/repository publication and known downstream references before editing.
- Treat possible external use as sufficient reason to retain `StripAnsi`.
- If the module is private or an API break is explicitly acceptable, move assertion-only behavior into tests and remove the unused production export.
- Record a no-code decision as a valid completion when compatibility risk remains.

## Risks / Trade-offs

- Keeping one tiny helper costs little and avoids a breaking change.
- Public code search cannot prove absence across private downstream repositories.
