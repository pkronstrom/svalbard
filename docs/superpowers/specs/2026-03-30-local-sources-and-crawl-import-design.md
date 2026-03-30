# Local Sources And Crawl Import Design

## Summary

Svalbard should support user-generated and user-provided content as first-class local sources without making built-in presets machine-dependent. The primary use case is crawling a website into a ZIM file, storing that generated artifact in the workspace, and later selecting it for one or more drives.

The design introduces a workspace-local source catalog under `local/`. Each local source is defined by a sidecar YAML recipe such as `local/example-docs.yaml`. That sidecar points at a canonical local file or directory path. For crawled sources, the generated artifact will normally live inside a managed workspace-generated directory. For manually added sources, the sidecar may point to any local file or directory path.

The sidecar acts as the source definition. Svalbard scans local sidecars automatically, exposes them as available sources, and lets each drive opt into a selected subset through its manifest and the setup wizard.

## Goals

- Make crawled ZIMs reusable across multiple drives from one workstation.
- Treat local artifacts and their metadata as one coherent unit.
- Keep checked-in presets static and reproducible across machines.
- Allow users to add local artifacts beyond crawled ZIMs later, including manually downloaded files and directories.
- Add wizard support for selecting local sources when space remains.

## Non-Goals

- Automatically adding all discovered local sources to every preset.
- Storing crawl outputs primarily on a target drive.
- Turning ad hoc local content into checked-in repo recipes by default.
- Designing a general remote artifact registry.
- Solving deduplication across distinct local source files beyond basic path and checksum recording.

## Current Context

Today, presets are loaded from checked-in YAML files and recipe directories. Sources currently support `download` and `build` strategies, and `sync` either downloads or builds content for a drive. The current crawl work is a thin wrapper around Dockerized Zimit, but it is config-driven and not integrated with the source model in a way that supports reusable local artifacts cleanly.

The wizard already selects a target drive, region, and preset, then shows a review step before initializing and syncing a drive. That makes it the right place to offer optional local sources after preset sizing is known.

## Terminology

- Local source: a sidecar YAML source definition under `local/` that points at a local file or directory path
- Built-in source: a checked-in recipe from `recipes/`
- Crawled source: a local source produced by `svalbard crawl`
- Drive selection: the subset of local sources a specific drive should include

## Storage Model

### Workspace Layout

Add a new workspace-local directory:

```text
local/
  example-docs.yaml
  radio-notes.yaml
```

The sidecar is the canonical entrypoint for discovery. It points at a local file or directory path, which may live inside or outside the workspace.

For crawled sources, Svalbard should write generated artifacts into a managed workspace-generated directory. One acceptable default is:

```text
generated/
  example-docs.zim
  example-docs.crawl.yaml
local/
  example-docs.yaml
```

The exact directory naming can still be adjusted, but it should describe generated durable artifacts rather than a disposable cache. Crawled outputs should remain workspace-local rather than being written to a drive first.

### Sidecar Schema

The sidecar should follow the same conventions as other source recipes. It is a normal source recipe with local-specific fields added only where needed. Minimum expected fields:

```yaml
id: example-docs
type: zim
group: practical
strategy: local
path: generated/example-docs.zim
description: Example Docs crawled from example.com
tags: [docs]
depth: comprehensive
```

Example for a manually added directory:

```yaml
id: repair-library
type: app
group: practical
strategy: local
path: /path/to/repair-library
description: Local repair manuals and static app bundle
size_bytes: 123456789
```

For crawled sources, crawl provenance should be stored separately from the recipe metadata. One acceptable structure is an adjacent crawl metadata file:

```yaml
artifact: generated/example-docs.zim
origin_url: https://example.com/docs
created: 2026-03-30T12:00:00
tool: zimit
scope: prefix
page_limit: 500
size_limit_mb: 512
time_limit_minutes: 60
checksum_sha256: ...
size_bytes: ...
```

This keeps the recipe clean and recipe-like while preserving crawl-specific provenance and execution settings. No second source registry is required: the recipe remains the local source definition, and the crawl metadata is an auxiliary record for generated sources only.

### Path Policy

`path` may refer to:

- a regular file
- a directory
- a symlink to either

Sync should copy the dereferenced contents to the drive and record the resulting drive artifact in the manifest as a normal concrete file or directory payload.

## Source Model Changes

Add a new source strategy:

- `local`: source content already exists on the local machine and should be copied to the drive

Required fields for `local` sources:

- `path`: a local file or directory path

Existing `download` and `build` behavior remains unchanged.

Local sources should be discoverable alongside built-in recipes, but they must remain logically distinct:

- built-in recipes are checked into `recipes/`
- local recipes are scanned from `local/`

Preset loading must not silently inject local sources into built-in presets. Inclusion remains explicit per drive.

## Drive Manifest Changes

Extend the drive manifest with:

- `local_sources: [source-id, ...]`

Meaning:

- these local source IDs should be included for this drive in addition to the preset's built-in sources

The manifest continues to record concrete downloaded or copied artifacts under `entries`.

## Discovery And Inclusion Rules

### Discovery

Svalbard automatically scans `local/*.yaml` and loads valid sidecars whose referenced artifact exists.

Invalid local sources should be skipped with clear diagnostics, for example:

- sidecar exists but artifact is missing
- duplicate local source ID
- unsupported type or invalid strategy

### Inclusion

Local sources are not included automatically just because they were discovered.

Inclusion happens only when a drive manifest lists specific local source IDs. This keeps built-in presets reproducible and avoids surprising content drift on future drives.

## Crawl Behavior

The default crawl UX should become direct-url based:

```bash
svalbard crawl https://example.com/docs -o example-docs.zim
```

Behavior:

1. Run Zimit against the requested URL.
2. Write the resulting artifact into `generated/`.
3. Write crawl provenance metadata alongside it, for example `generated/<id>.crawl.yaml`.
4. Register the output as a local source by generating a matching recipe in `local/`.
5. Report the created local source ID and how to include it in a drive.

Advanced mode may still support YAML crawl configs for reusable multi-site jobs, but those configs are secondary to the direct-url flow and should produce the same generated artifact plus local recipe outputs.

## Local Add Behavior

Add a local-source management command namespace:

- `svalbard local add <path>`

Behavior:

1. Accept a file or directory path.
2. Infer or require a source ID and type.
3. Compute size:
   - file size directly for files
   - recursive total size for directories
4. Write a sidecar recipe to `local/<id>.yaml`.
5. Record the original local path in the recipe.

This is the primary way to register existing local content. It is simpler than requiring symlinks and works for both files and directories.

`svalbard crawl` should reuse the same registration path after generating its ZIM output. In other words, crawl generates a ready-made artifact and then registers it locally in the same way that `svalbard local add` registers an existing file or directory.

Future commands such as `svalbard local list` or `svalbard local validate` are reasonable follow-ups but not required for the first implementation.

## Sync Behavior

During sync:

1. Load preset sources as today.
2. Load local source recipes from `local/`.
3. Read `manifest.local_sources`.
4. Resolve selected local IDs into source objects.
5. Copy each selected local source path to the correct drive directory based on source type.
6. Record copied artifacts in manifest entries just like downloaded sources.

Copying local sources should reuse the same destination directory rules already used for built-in sources:

- `zim` -> `zim/`
- `pdf` -> `books/`
- `pmtiles` -> `maps/`
- and so on

If a copied local source already exists on the drive with the same filename and checksum or size, sync may skip it.

For directory-backed local sources, sync should copy the directory tree into the target type directory in a deterministic location derived from source ID.

## Wizard Behavior

After preset selection and size estimation, the wizard should detect available local sources and offer them as optional additions if free space remains.

Wizard flow addition:

1. User selects target, region, and preset.
2. Wizard computes preset size and remaining free space.
3. Wizard scans `local/` for valid local sources.
4. If any local sources exist, wizard shows them as optional extras with size and description.
5. User selects zero or more local source IDs to include.
6. Wizard writes those IDs into `manifest.local_sources`.

If free space cannot be determined, the wizard may still offer local sources with a warning. If no local sources are found, the step is skipped.

The wizard should not auto-select local sources by default.

## CLI Surface

### Initial Scope

Planned near-term CLI changes:

- `svalbard crawl <url> -o <filename.zim>`
- `svalbard local add <path>`
- existing config-driven crawl mode remains available or is adapted later

Potential follow-up commands, not required in the first implementation:

- `svalbard local list`
- `svalbard local validate`

The first implementation should stay narrow: direct-url crawl, local add, sync integration, and wizard integration.

## Error Handling

Key failure modes:

- local sidecar missing referenced artifact
- duplicate local source IDs
- crawl output filename collision
- crawl succeeds but sidecar generation fails
- manifest references unknown local source ID
- `svalbard local add` path does not exist
- local add cannot infer a valid source type

Desired behavior:

- fail loudly for the direct action being performed
- keep unrelated built-in preset behavior intact
- never auto-delete user-local artifacts

## Testing Strategy

Add tests for:

- local source sidecar parsing and discovery
- duplicate ID handling
- manifest support for `local_sources`
- sync copying selected local sources onto the drive
- wizard behavior when local sources are present
- crawl generating artifact path, separate crawl metadata, and local recipe registration
- local add for file paths
- local add for directory paths with recursive size calculation

Keep tests isolated from Docker by mocking crawl execution where appropriate.

## Open Questions Resolved In This Design

- Should local sources be cached in the workspace or written to the drive first?
  - Workspace first.
- Is the sidecar the recipe?
  - Yes.
- Should local sources be auto-scanned?
  - Yes.
- Should built-in presets auto-include discovered local sources?
  - No.
- Where should selection happen?
  - Per drive manifest, with wizard support.

## Recommended Implementation Sequence

1. Add source-model and manifest support for `local` sources and `local_sources`.
2. Add local sidecar discovery and validation.
3. Teach sync to copy selected local sources to the drive.
4. Add wizard selection of local sources.
5. Add `svalbard local add <path>` for file and directory-backed sources.
6. Refactor `crawl` to emit generated artifacts, separate crawl metadata, and local recipe registration.
7. Optionally preserve or rework advanced YAML crawl configs on top of the same model.
