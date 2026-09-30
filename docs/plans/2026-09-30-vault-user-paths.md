# Vault user paths

Status: user priorities recorded 2026-09-30; discovery and implementation planning required per masterplan phase.

## Initial deployment and resume

Path: start init → select target disk/platforms/preset → save configuration → acquire content → cancel/stop → reopen the same vault → resume remaining work → see installed/failed/pending state.

Verify cancellation propagation, manifest persistence, which successful items survive failures, partial downloads, source identity on resume, server behavior when Range is unsupported or unsatisfiable, disk-full errors and restart recovery. Existing HTTP Range support is a starting point, not proof of the whole journey. Partial files must not appear as installed/usable content. User settings must survive interruption.

## Preset customization

Path: choose a preset → inspect its contents → remove/add items through TUI → inspect required dependencies and size → save vault-specific desired state → preview/apply.

Separate master preset definitions from this vault's changes. Reopening the vault must preserve overrides. Define how explicit removals interact with required dependencies, catalog changes and future preset refreshes. Verify the existing picker and commands before introducing new configuration syntax. Browser configuration is a separate planned interface over the same operations.

## Pack operations

For every offered pack, write down: how the user installs it, where files land, what actions appear, which binaries/configs those actions need, the first useful operation, and how failures/recovery behave. Read through that path even where hardware/platform testing is unavailable; record the limitation instead of treating inspection as a live test.

Cover ZIM browsing and keyword/semantic search, local LLM chat and harness/MCP usage, maps/layers, structured SQLite data, PDFs/EPUBs, offline apps, utilities, and embedded development. In particular verify PIO cache/setup/build/upload/monitor and whether host dependencies are actually supplied. A pack menu appearing is not operational verification. For missing native capability or executable routing, repair the path or clearly identify unsupported behavior.

## Updating an existing vault

Path: open existing vault → compare installed acquisition hashes with current master recipes → preview changed items and unknown older entries → choose/apply updates → regenerate runtime assets and affected indexes → verify installed state.

Use the agreed simple acquisition-hash design. A recipe hash detects catalog changes, not remotely changed bytes behind a stable URL. Optional online discovery of newer binaries/apps/content is separately queued; define how it becomes a deliberate selected version without silently modifying master recipes or installing whatever upstream reports.

## Adding and removing items

Path: inspect existing vault → search/browse catalog → add/remove desired items → preview additions/dependencies/removals/space → apply → confirm files, menus, services and indexes agree.

Verify current TUI and CLI paths, dependency closure, changes cancelled before apply, removal of managed outputs only, stale search entries and generated assets, multi-platform/multi-file recipes and unknown catalog items. Preserve unrelated user files. Keep operation preview and execution consistent.

## Completion evidence

Each phase records the path read-through, concrete gaps and fixes, relevant automated tests and feasible live checks. Record untested platforms, hardware requirements and remaining blockers explicitly. Do not write a large combined implementation plan for all journeys.
