# Svalbard — master plan

Archived 2026-11-21 from `MASTERPLAN.md`; links adjusted for this location. Open work moved to the task tracker (Aven project `svalbard`).

Svalbard builds portable offline vaults. Plan and verify the product through the user's paths: create and resume a vault, customize its contents, use each installed pack from the stick, update installed recipes, and add or remove items later. Recipes describe acquisition; the host manages desired/realized state; the Go drive runtime provides offline access.

The queue below is ordered around those paths following the user's 2026-09-30 direction. Existing implementations should be verified and repaired before being rebuilt. Each later phase gets its own discovery and implementation plan when reached; triage is not authorization to build every wishlist idea.

## Ordered phases

- [x] Phase 1 — Tool/harness/GGUF catalog refresh verified 2026-09-30 → [results and exceptions](../2026-09-30-catalog-refresh-results.md), [refresh plan](../2026-09-30-catalog-refresh.md). Includes sensible defaults and release/toolchain compatibility; deployment and pack operability remain later phases.
- [ ] Phase 1b — Fix broken pins/links, size drift, model roster, preset rebalancing, local custom presets, and new content/tool/data candidates → [catalog update audit checklist](../2026-09-30-catalog-update-audit.md). Claude session `e58429bd` reports its approved scope complete and verification passing; remaining audit items and decisions persist. See [coordination snapshot](../2026-09-30-claude-session-coordination.md) before duplicating work.
- [x] Phase 2 — Initial deployment and interrupted-download recovery verified 2026-09-30 → [checkpoint and verification](../2026-09-30-deployment-resume-checkpoint.md), [design](../2026-09-30-deployment-resume-design.md), [implementation](../2026-09-30-deployment-resume-implementation.md). Saved selection, cancellation/restart, safe partial-file reuse and truthful completion state implemented and tested; physical-stick/power-loss checks remain unverified.
- [x] Phase 3 — Customize a selected preset through the TUI verified 2026-10-05 → [checkpoint and verification](../2026-10-05-preset-customization-checkpoint.md), [implementation plan](../2026-09-30-preset-customization-implementation.md), [design](../2026-09-30-preset-customization-design.md). Existing wizard retains edits and dependency classification, validates missing items and saves the reviewed selection for reopen/resume.
- [x] Phase 4 — Search reliability and usable indexes from the stick verified 2026-10-07 → [checkpoint](../2026-10-06-search-reliability-checkpoint.md), [implementation](../2026-10-06-search-reliability-implementation.md), [design](../2026-10-06-search-reliability-design.md). Apply indexing, source reconciliation, semantic resume and exact-model native/MCP search verified with packaged macOS arm64 binaries; other platforms/physical media remain unexecuted.
- [x] Phase 5 — Complete offline PlatformIO/ESP32 operation verified 2026-11-21 → [checkpoint and verification](../2026-11-21-offline-platformio-checkpoint.md), [implementation](../2026-10-07-offline-platformio-implementation.md), [design](../2026-10-07-offline-platformio-design.md). Normal apply of `esp32-dev`, fresh-cache network-denied ESP32/S2/S3 builds with host Python excluded, reuse, relocation and cancel/retry verified on macOS arm64; other hosts, physical sticks and flashing remain unverified.
- [ ] Phase 6 — Verify the remaining pack operations end to end → [user paths](../2026-09-30-vault-user-paths.md#pack-operations), [discovery inventory 2026-11-21](../2026-11-21-pack-operations-inventory.md) (2 verified, 16 plausible, 6 broken; 4-chunk repair plan, map viewer vendor path already fixed). Inventory every pack's user-visible actions, required tools and destinations; repair or clearly disclose unsupported paths, including pending map builders.
- [ ] Phase 7 — Update installed recipes/packages from the master catalog → [simple acquisition tracking](../2026-09-30-recipe-acquisition-tracking-design.md). Compare computed acquisition hashes, preview updates, apply changed artifacts and handle required reindexing; missing hashes mean unknown.
- [ ] Phase 8 — Add/remove items on an existing vault → [user paths](../2026-09-30-vault-user-paths.md#adding-and-removing-items). Verify TUI and CLI desired-state edits, dependency handling, removal preview, unmanaged-file preservation and regenerated menus/indexes.
- [ ] Phase 9 — Plan a minimal configuration web UI → [planning brief](../2026-09-30-minimal-configuration-web-ui-design.md). Design browser-based preset customization and add/remove configuration using the same host operations as the TUI; scope/interface decisions precede implementation.

## Follow-up queue (order within this section not yet settled)

- [ ] Semantic recall/performance tuning — evaluate the existing keyword candidate restriction on realistic vaults before changing full-vector scanning or adding ANN retrieval; deferred from the reliability repair.
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

- Work resumed at the user's request on 2026-09-30; [continuation checkpoint](../2026-09-30-continuation.md) records the completed refresh, verification, remaining work and outstanding decisions.
- [Dodo triage ledger](../2026-09-30-dodo-triage.md) accounts for all 39 original entries and their destinations; retain IDs and partial completion evidence. Dodo points here rather than defining a second phase order.
- User paths, not presence of recipe files or menu entries, define completion. Audit existing seams before planning changes; opinionated defaults and clear, easy operation are cross-cutting acceptance criteria.
- Keep acquisition hashes separate from file checksums and builder cache fingerprints. Compute on recipe load; persist only after successful installation; use the existing manifest structure.
- [Runtime audit](../2026-09-30-runtime-backlog-audit.md) records tested paths and concrete search/PIO gaps. The [historical Python integrity plan](../2026-03-30-manifest-integrity-and-drift.md) is background, not the current Go contract.
- Pi plugin defaults remain a design discussion. Existing upstream built-ins should inform the minimal default set.
- Phase 3 [checkpoint](../2026-10-05-preset-customization-checkpoint.md): saved `desired.items` is authoritative; preset names are provenance. Keep explicit/automatic selections distinct during wizard edits and preserve Phase 2 cancellation. Phase 4 [checkpoint](../2026-10-06-search-reliability-checkpoint.md): keyword indexing is automatic by default, `none` skips it, and semantic/full resumes matching contracts. Preserve writer locking, atomic source replacement/publication, exact-model reader compatibility, bounded full-detail snapshots and cancellation. Phase 5 [checkpoint](../2026-11-21-offline-platformio-checkpoint.md): ready generations are immutable and keyed by vault path and archive content; keep the cache canonical (espressif32 7.1.3 breaks on symlinked or macOS `/tmp` cache paths) and keep upload/monitor claims hardware-dependent.
