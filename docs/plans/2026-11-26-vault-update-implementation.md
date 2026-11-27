# Updating an installed vault: implementation plan

Status: implemented 2026-11-26, uncommitted; verified on macOS arm64 in a disposable local vault. Physical media and other operating systems are unverified. Builds on the agreed [acquisition tracking design](2026-09-30-recipe-acquisition-tracking-design.md) and the "Updating an existing vault" path in [vault user paths](2026-09-30-vault-user-paths.md).

## User paths

1. Open an old vault and see which installed items have a changed recipe, which were installed before tracking, and which are no longer in the catalog.
2. Update chosen items, or all changed ones. A failed update leaves the installed copy in place.
3. Keep a withdrawn item on the stick while adding, removing and updating others.

## Rules

- `apply` never replaces installed data. It downloads what is missing and deletes what was deselected. Updating is `svalbard update` or the TUI Updates screen.
- Every installed item has one version state:

  | State | Meaning |
  |---|---|
  | `current` | recorded hash equals the recipe's hash |
  | `update` | recorded hash differs: the recipe changed |
  | `unknown` | installed before tracking; no recorded hash |
  | `withdrawn` | the id is not in the catalog; the installed copy is kept |
  | `local` | imported by the user |

- `update --mark-current` records `unknown` items as current without downloading, except one whose recipe pins a checksum the installed file does not have.
- `update --all` takes `update` items only. `update <id>` reacquires that item whatever its state, which is also how a `url_pattern` recipe picks up a newer upstream file and how an `unknown` item becomes tracked.
- A withdrawn item cannot be updated. Only `remove` deletes it, and `remove` says it cannot be downloaded again.
- An update swaps: the installed files are moved aside, the item is acquired again, and the old files are deleted only after it succeeds. On failure they are moved back.
- A swap needs room for both copies. `update` refuses when the estimate does not fit and names `--replace`, which deletes first; a failed `--replace` leaves the item pending.
- No per-item "hold". Not updating an item is the way to keep it.

## Findings from the existing code

- The planner and status compare ids only; `RealizedEntry` has no acquisition identity.
- Step pipelines skip the build when their output exists (`builder/pipeline.go`), so an in-place update would do nothing and record a new hash. Moving the output aside first makes them rebuild; unchanged procedures still restore from the block cache.
- Downloads reuse a final file whose sidecar matches URL and pin (`downloader.reusableFinal`). Moving the file aside avoids reusing the old artifact.
- Python packages are installed by their venv's aggregate job, so the job id differs from the item id. Moving `runtime/python` aside would reinstall every tool, so the venv builder updates them in place instead: an item being updated is passed in `builder.Options.Rebuild`, its tree is reinstalled through `installTree` (unpacked into `<tree>.partial`, which replaces the tree only when that succeeds), and a failure keeps the installed tree and fails the update, unlike a failed older-layout repair, which only warns. Package stamps record the interpreter version before the layout line, so a venv version change installs the new interpreter beside the old one, rebuilds the trees, and then removes the earlier interpreter.
- A `content-archive` update is a full rebuild. That is correct but slow; its own targeted refetch stays the tool for partial refreshes.
- A desired item that is neither installed nor in the catalog used to fail every `apply`. Plan and apply now leave it out and say so, and status names it `unavailable`. The saved selection is not changed, so the preset-customization contract holds.

## Chunks

1. **Hash.** `catalog.AcquisitionHash(item)`: SHA-256 over deterministic JSON of the acquisition fields, formatted `acq-v1:sha256:<hex>`. Tests for what does and does not change it.
2. **Record.** `acquisition_hash` on `RealizedEntry`, stamped in `apply` when a job's entries are checkpointed. Old manifests load unchanged.
3. **Report.** Version state in `status` (text and JSON) and an updates section in `plan`.
4. **Update.** `planner.Plan.ToUpdate`, the swap in `apply`, recovery of files left aside by an interrupted run, `svalbard update [ids…] [--all] [--replace]`, reindex afterwards.
5. **TUI.** An Updates row on the dashboard that opens the existing plan screen with the changed items, and counts in the Status pane.

## Not covered

- Remote freshness behind a stable URL (a separate, queued question).
