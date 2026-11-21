# Preset customization checkpoint

Status: done 2026-10-05; implementation, final review and verification complete.

Scope: MASTERPLAN Phase 3, following the reviewed [design](2026-09-30-preset-customization-design.md) and [implementation plan](2026-09-30-preset-customization-implementation.md). The existing init flow and manifest format are retained.

## Resulting user path

1. Choose a built-in or local preset; its sources seed the existing Pack Picker. Customize starts empty.
2. Add/remove recipe selections and inspect the dependency-expanded selection and estimated catalog size. Required dependencies stay automatic while needed; removing their last consumer releases them unless explicitly selected.
3. Missing explicit preset sources appear as removable unavailable rows. Missing required dependencies stay visible and block confirmation until the selection is repaired. Unknown sizes are shown as unknown and excluded from the known-size subtotal.
4. Continue to Review. Going back preserves explicit choices and dependency classification; picker cancel and same-preset/Customize reselection retain edits. A different preset starts from its sources.
5. Confirm validates and saves exactly the reviewed, sorted effective IDs, preset provenance, region and platforms before the existing cancellable Apply begins. Init errors remain on Review with selections intact. Empty Customize is valid.
6. Reopen/resume uses saved `desired.items`. Editing the base/local preset afterward does not restore removed items or add newly listed ones automatically.

Validation/init errors take priority in the final shell viewport, including an 80×8 terminal. Their text wraps to the actual pane width; rendered height is reserved before list rows and the live picker uses that budget.

## Verification

- `GOCACHE=/private/tmp/svalbard-go-cache GOFLAGS=-count=1 make verify` — passed after final layout fixes: 53 tested packages across all five Go modules and catalog synchronization.
- `go test -race ./... -count=1` in `tui` and `host-tui` — passed; full host TUI race suite rerun after final fixes.
- `go test -race ./internal/catalog ./internal/cli -count=1` in `host-cli` — passed.
- `make build-dev` — passed after final fixes; host CLI built.
- `git diff --check` — passed.
- Final correctness/design review finding fixed; targeted fix review — zero remaining findings.

Actual key/command tests cover preset → remove/add → Review → back → last-consumer removal/re-addition → picker cancel → same preset → Review → confirm. They assert exact review/init/result agreement, automatic classification and unchanged base preset. Additional key paths repair an unavailable preset and confirm empty Customize after cancel/reselection.

The real host fixture loads a temporary local preset, customizes it with dependencies, saves through the actual init callback and reloads the manifest. After deliberately changing the preset, fresh configuration sees that change while reopened desired state, status and pending plan retain only the saved selection. No public download occurs.

Review findings reproduced and fixed: asynchronous done/cancel commands previously read mutable selections too late; nil resolver previously bypassed known unavailable-ID validation; final shell clipping hid errors on short terminals. Regression fixtures also exposed incorrect counting of newlines/wide characters and a picker budget changed only on a View copy; actual renderer-based counting and live budgets repair both.

## Scope and limits

This phase does not implement reusable preset export, existing-vault add/remove, versioned catalog updates, exact platform sizing, browser UI or search/index reliability. Those remain in later masterplan phases. Acceptance was verified through headless key/viewport tests and local filesystem fixtures on the current macOS host; no physical-stick or power-loss test was performed.

All pre-existing uncommitted work is retained. No staging, commits or Git metadata edits were performed. Phase 2 cancellation/checkpoint behavior remains covered by repository verification.
