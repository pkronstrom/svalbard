# Svalbard: agent instructions

Svalbard builds portable offline vaults. Recipes describe acquisition, the host CLI and TUI (`host-cli`, `host-tui`) manage desired and realized state, and the Go drive runtime (`drive-runtime`) provides offline access from the stick. `make verify` runs the catalog sync check and every module's tests; `make build` builds the runtimes and the CLI.

## Tasks

Task tracker: aven

Open work lives in Aven (project `svalbard`). `aven list --ready` isn't sorted by
priority, so check `aven list --ready --priority urgent`, then `high`, then the rest.
File follow-ups you discover as `--status inbox`; the user triages them.
Keep Aven refs out of commits, PRs and committed docs; tasks point at repo paths and SHAs.

To add a task, run this from the repo, which sets the project (there are no assignees):
`aven add "<verb-first title>" --status <status> --priority <priority> --label <label> --description "<problem, done-when, repo paths>"`

- **Status:** `todo` when the user asked for it and it can start, `backlog` for later, `active` once started.
- **Priority:** `urgent` for a live failure, data loss or security exposure; `high` if overdue, due within two weeks, or blocking `high` work; `medium` for planned work with a clear next step; `low` for an improvement with no driver; `none` for an unranked idea.
- **Labels:** `catalog` (recipes, presets, packs), `host-cli` (host CLI and TUI), `runtime` (drive runtime), `search`, `ai`, `maps`, `embedded`, `docs`. Other labels fail until created with `aven label create`, so add one only for a lasting area.
- **Epic:** if it serves one in `aven list --epics`, run `aven epic add <task> <epic>` afterwards. Create an epic only for three or more tasks toward one result that will finish.
- **Order and dates:** `aven dep add <waiting> <blocker>` when one must finish first; `--available-at <date>` for not before; `--due <date>` for a true deadline.

## Planning rules

- Plan and verify through the user's paths: create and resume a vault, customize its contents, use each installed pack from the stick, update installed recipes, and add or remove items later. User paths, not the presence of recipe files or menu entries, define completion.
- Verify and repair existing implementations before rebuilding them, and audit the existing seams before planning changes.
- Each epic gets its own discovery and implementation plan when it is reached. Triage is not authorization to build every wishlist idea.
- Opinionated defaults and clear, easy operation are acceptance criteria for every change.
- Keep acquisition hashes separate from file checksums and builder cache fingerprints. Compute them on recipe load, persist them only after a successful install, and use the existing manifest structure.
- Contracts from finished phases:
  - Presets ([checkpoint](docs/plans/2026-10-05-preset-customization-checkpoint.md)): saved `desired.items` is authoritative and preset names are provenance. Keep explicit and automatic selections distinct during wizard edits, and preserve deployment cancellation.
  - Search ([checkpoint](docs/plans/2026-10-06-search-reliability-checkpoint.md)): keyword indexing is automatic by default, `none` skips it, and semantic/full resumes matching contracts. Preserve writer locking, atomic source replacement and publication, exact-model reader compatibility, bounded full-detail snapshots and cancellation.
  - Offline PlatformIO ([checkpoint](docs/plans/2026-11-21-offline-platformio-checkpoint.md)): ready generations are immutable and keyed by vault path and archive content. Keep the cache canonical (espressif32 7.1.3 breaks on symlinked or macOS `/tmp` cache paths), and keep upload and monitor claims hardware-dependent.
  - Document collections ([checkpoint](docs/plans/2026-11-26-pdf-collections-checkpoint.md)): a collection's realized entries are its complete file set, one per document under `books/<id>/`. A local source that cannot be read keeps its installed files, and a vault's recipe copy never reads `path:` entries from this computer. Documents are index sources named by vault path with one article per page (`page/N`); a missing `books/` or `zim/` is never a prune. PDFium runs on the host only, and the drive reads documents from `search.db` alone.

## Background records

These are history and evidence. Nobody updates them except where noted.

- [Archived master plan](docs/plans/archive/2026-11-21-MASTERPLAN.md): the phase order through 2026-11-21, with links to each phase's design, implementation plan and checkpoint.
- [Catalog update audit](docs/plans/2026-09-30-catalog-update-audit.md): the 2026-09 audit record. Its open items moved to Aven.
- [Continuation checkpoint](docs/plans/2026-09-30-continuation.md): the 2026-09-30 refresh, verification and outstanding decisions.
- [Dodo triage ledger](docs/plans/2026-09-30-dodo-triage.md): where each of the 39 original Dodo entries went, with IDs and partial-completion evidence. Dodo itself is retired.
- [Runtime audit](docs/plans/2026-09-30-runtime-backlog-audit.md): tested runtime paths and concrete search and PlatformIO gaps. The [historical Python integrity plan](docs/plans/2026-03-30-manifest-integrity-and-drift.md) is background, not the current Go contract.
- [docs/roadmap.md](docs/roadmap.md) indexes openspec changes; each change's `tasks.md` stays openspec's step checklist, updated by the openspec workflow.
