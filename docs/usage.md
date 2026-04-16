# Svalbard Usage Guide

This guide describes the current Go host CLI in `host-cli/`.

The active model on this branch is:

- A vault is a plain directory with `manifest.yaml`, downloaded artifacts, generated toolkit files, and the on-drive runtime.
- The host CLI reconciles desired state against realized state.
- Python remains only for specialized builder scripts under `recipes/builders/`, intended to run inside the `svalbard-tools` container.

Examples below assume you are running from `host-cli/`:

```bash
cd host-cli
```

## Core Flow

1. Initialize a vault from an embedded preset.
2. Inspect the reconciliation plan.
3. Apply the plan to download and materialize content.
4. Build the search index.
5. Run the generated `./run` launcher from the vault root.

Example:

```bash
go run ./cmd/svalbard init /Volumes/MyDrive --preset default-32
go run ./cmd/svalbard plan --vault /Volumes/MyDrive
go run ./cmd/svalbard apply --vault /Volumes/MyDrive
go run ./cmd/svalbard index --vault /Volumes/MyDrive
cd /Volumes/MyDrive && ./run
```

## Catalog And Presets

The Go CLI loads the built-in catalog from embedded copies of `recipes/` and `presets/`.

- `svalbard preset list` shows the embedded preset names.
- `svalbard preset copy <source> <target>` exports one preset as YAML.
- `svalbard init --preset <name>` currently resolves only catalog presets known to the binary.

That means `preset copy` is useful for inspection and editing, but local custom preset loading is not yet part of the current CLI flow.

## Vault Layout

After `apply`, a vault typically contains:

- `manifest.yaml`: desired and realized state
- `library/`: imported local files and library metadata
- `zim/`, `maps/`, `models/`, `tools/`: realized content, depending on selected items
- `.svalbard/`: runtime metadata, embedded runtime binaries, and generated action config
- `run` and `activate`: launcher scripts for the on-drive runtime
- `apps/map/index.html`: offline PMTiles viewer when map content is present

## Commands

### `svalbard init [path] --preset <name>`

Create a new vault directory and seed its desired state from a preset.

Example:

```bash
go run ./cmd/svalbard init /Volumes/MyDrive --preset finland-128
```

### `svalbard add <item...> --vault <path>`

Append one or more item ids to the vault's desired state.

These ids can come from:

- built-in catalog entries bundled with the binary
- local imported items such as `local:manual`

### `svalbard remove <item...> --vault <path>`

Remove one or more item ids from desired state.

### `svalbard plan --vault <path>`

Show what `apply` would download, keep, or remove.

Use this before `apply` when you want to inspect the reconciliation result.

### `svalbard apply --vault <path>`

Execute the reconciliation plan:

- resolve item URLs
- download artifacts
- update realized entries in `manifest.yaml`
- regenerate runtime metadata and launch scripts
- generate or remove the offline map viewer based on realized PMTiles

### `svalbard status --vault <path>`

Print the vault name, preset, desired count, realized count, pending count, and a per-item table.

### `svalbard import <file> --vault <path> [--add] [--name <name>]`

Import a local file into the vault library.

- `--add` also appends the new `local:*` id to desired state
- `--name` overrides the derived output name

Example:

```bash
go run ./cmd/svalbard import ../manual.pdf --vault /Volumes/MyDrive --add --name field-manual
go run ./cmd/svalbard apply --vault /Volumes/MyDrive
```

The current Go implementation imports local files only. Remote web/media ingestion from the old Python flow is not part of this CLI contract.

### `svalbard preset list`

List embedded preset names.

### `svalbard preset copy <source> <target>`

Write a preset YAML file to the target path.

Example:

```bash
go run ./cmd/svalbard preset copy default-128 ../tmp/default-128.yaml
```

### `svalbard index --vault <path>`

Build the SQLite full-text search index from realized ZIM files under the vault.

The generated database is written under `data/search.db`.

## Builder Scripts

Python builder scripts are still valid, but they are not the host CLI anymore.

- Keep them under `recipes/builders/`.
- Run them through `svalbard-tools`.
- Treat them as containerized worker implementation, not host prerequisites.

The remaining Python tests in `src/tests/` should track those builder scripts directly rather than the retired Python control plane.
