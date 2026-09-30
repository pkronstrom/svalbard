# Svalbard — master plan

Svalbard builds portable offline vaults. Plan and verify the product through the user's paths: create and resume a vault, customize its contents, use each installed pack from the stick, update installed recipes, and add or remove items later. Recipes describe acquisition; the host manages desired/realized state; the Go drive runtime provides offline access.

The queue below is ordered around those paths following the user's 2026-09-30 direction. Existing implementations should be verified and repaired before being rebuilt. Each later phase gets its own discovery and implementation plan when reached; triage is not authorization to build every wishlist idea.

## Ordered phases

- [x] Phase 1 — Tool/harness/GGUF catalog refresh verified 2026-09-30 → [results and exceptions](docs/plans/2026-09-30-catalog-refresh-results.md), [refresh plan](docs/plans/2026-09-30-catalog-refresh.md). Includes sensible defaults and release/toolchain compatibility; deployment and pack operability remain later phases.
- [ ] Phase 1b — Fix broken pins/links, size drift, model roster, preset rebalancing, local custom presets, and new content/tool/data candidates → [catalog update audit checklist](docs/plans/2026-09-30-catalog-update-audit.md). Claude session `e58429bd` reports its approved scope complete and verification passing; remaining audit items and decisions persist. See [coordination snapshot](docs/plans/2026-09-30-claude-session-coordination.md) before duplicating work.
- [ ] Phase 2 — Initial deployment and interrupted-download recovery → [user paths](docs/plans/2026-09-30-vault-user-paths.md#initial-deployment-and-resume). Verify saved selection, cancellation/restart, safe partial-file reuse, and clear completion state.
- [ ] Phase 3 — Customize a selected preset through the TUI → [user paths](docs/plans/2026-09-30-vault-user-paths.md#preset-customization). Preview additions/removals, dependencies and space; persist vault-specific choices without editing master presets.
- [ ] Phase 4 — Search reliability and usable indexes from the stick → [audit](docs/plans/2026-09-30-runtime-backlog-audit.md). Wire indexing choices into provisioning; handle changed sources and embedding-model/dimension compatibility; validate FTS/hybrid/MCP journeys with actual packaged binaries.
- [ ] Phase 5 — Complete offline PlatformIO/ESP32 operation → [audit](docs/plans/2026-09-30-runtime-backlog-audit.md#platformio). Fix archive layout/cache freshness, provision the compatible dependency closure, and verify a clean-cache build with networking disabled; distinguish device-dependent flashing checks.
- [ ] Phase 6 — Verify the remaining pack operations end to end → [user paths](docs/plans/2026-09-30-vault-user-paths.md#pack-operations). Inventory every pack's user-visible actions, required tools and destinations; repair or clearly disclose unsupported paths, including pending map builders.
- [ ] Phase 7 — Update installed recipes/packages from the master catalog → [simple acquisition tracking](docs/plans/2026-09-30-recipe-acquisition-tracking-design.md). Compare computed acquisition hashes, preview updates, apply changed artifacts and handle required reindexing; missing hashes mean unknown.
- [ ] Phase 8 — Add/remove items on an existing vault → [user paths](docs/plans/2026-09-30-vault-user-paths.md#adding-and-removing-items). Verify TUI and CLI desired-state edits, dependency handling, removal preview, unmanaged-file preservation and regenerated menus/indexes.
- [ ] Phase 9 — Plan a minimal configuration web UI → [planning brief](docs/plans/2026-09-30-minimal-configuration-web-ui-design.md). Design browser-based preset customization and add/remove configuration using the same host operations as the TUI; scope/interface decisions precede implementation.

## Follow-up queue (order within this section not yet settled)

- [ ] Non-ZIM document indexing and local document RAG — unify PDF/HTML ingestion and search/read/answer paths; existing semantic ZIM retrieval is the starting point.
- [ ] Local multi-model routing and additional native actions through MCP — refresh pins first; model switching/restart and new MCP capabilities need separate designs.
- [ ] Online version discovery — optionally discover newer tools/apps/content where an upstream mechanism exists; distinguish discovery from catalog-approved versions and deliberate installation. Depends on Phase 7.
- [ ] Offline navigation/routing — distinct from viewing PMTiles; source/graph/engine decisions remain open.
- [ ] Content/preset curation — licensing/attribution, library and hardware documentation, regional coverage, programming pack tiers, and compact-ZIM diagram readability.
- [ ] Host/mobile provisioning — existing Brewfile and iOS/Android guides are foundations; Linux workflow, wizard integration and installer/APK scope need refinement.

## Wishlist and unresolved scope

These are preserved ideas, not numbered implementation commitments. Their queue/wishlist disposition has been asked of the user.

- Bootable media: Kali, a bootable partition and Ventoy research.
- Optional interactive apps: Kolibri, notes/wiki (merge the overlapping notes/content-creation ideas), JupyterHub and a broader web dashboard. The broad dashboard is distinct from Phase 9's minimal configuration UI.
- Map enhancements: satellite imagery and annotations/photos/GeoJSON export.
- Source clarification: identify NODE PDF links and the intended AutoDoc content/workflow.
- KiCad: already recommended in provisioning; bundled installer/documentation scope is undecided. Mobile APK bundling also awaits clarification.

## Completed backlog work

- [x] Go/Cobra host CLI and native drive services — Dodo `7d14e95a`, `626c497d`; implementation inspected and runtime tests passed 2026-09-30.
- [x] LAN archive/file serving — Dodo `71a96d38`; sharing/serve-all tests passed 2026-09-30.
- [x] iOS viewer recommendations — Dodo `30a7f508`; existing guide verified 2026-09-30 (recommendation freshness is separate).

## For the next planner

- Work resumed at the user's request on 2026-09-30; [continuation checkpoint](docs/plans/2026-09-30-continuation.md) records the completed refresh, verification, remaining work and outstanding decisions.
- [Dodo triage ledger](docs/plans/2026-09-30-dodo-triage.md) accounts for all 39 original entries and their destinations; retain IDs and partial completion evidence. Dodo points here rather than defining a second phase order.
- User paths, not presence of recipe files or menu entries, define completion. Audit existing seams before planning changes; opinionated defaults and clear, easy operation are cross-cutting acceptance criteria.
- Keep acquisition hashes separate from file checksums and builder cache fingerprints. Compute on recipe load; persist only after successful installation; use the existing manifest structure.
- [Runtime audit](docs/plans/2026-09-30-runtime-backlog-audit.md) records tested paths and concrete search/PIO gaps. The [historical Python integrity plan](docs/plans/2026-03-30-manifest-integrity-and-drift.md) is background, not the current Go contract.
- Pi plugin defaults remain a design discussion. Existing upstream built-ins should inform the minimal default set.
