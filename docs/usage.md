# Svalbard Usage Guide

This guide describes the Go host CLI shipped as the `svalbard` binary.

The model:

- A vault is a plain directory with `manifest.yaml`, downloaded artifacts, generated toolkit files, and the on-drive runtime.
- The host CLI reconciles desired state against realized state.
- Python is used only by specialized builder scripts under `recipes/builders/`, which run inside the `svalbard-tools` container during build-from-source recipes. The host CLI itself has no Python dependency.

Commands below assume `svalbard` is on your `PATH` (see [README — Install](../README.md#install)). When developing from source you can substitute `go run ./host-cli/cmd/svalbard` for `svalbard`.

## Core Flow

1. Run the guided wizard to create a vault and select a preset.
2. Wait for the wizard's apply step to download and materialize content.
3. Build the search index.
4. Run the generated `./run` launcher from the vault root.

Example:

```bash
svalbard init /Volumes/MyDrive   # opens interactive wizard — pick preset, apply
svalbard index --vault /Volumes/MyDrive
cd /Volumes/MyDrive && ./run
```

For an existing vault you can iterate with the non-interactive subcommands:

```bash
svalbard plan --vault /Volumes/MyDrive
svalbard apply --vault /Volumes/MyDrive
svalbard status --vault /Volumes/MyDrive
```

## Catalog And Presets

The CLI loads the built-in catalog from embedded copies of `recipes/` and `presets/`.

- `svalbard preset list` shows the embedded preset names.
- `svalbard preset copy <source> <target>` exports one preset as YAML.
- The init wizard only offers presets that are known to the binary's embedded catalog.

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

### `svalbard init [path]`

Open the guided setup wizard. The wizard handles preset selection, vault directory creation, manifest seeding, and the initial download/apply in one flow. The optional `path` argument pre-fills the destination so you don't have to type it in the wizard.

Non-interactive vault creation (a `--preset` flag) is not currently supported — `init` always launches the wizard.

Example:

```bash
svalbard init /Volumes/MyDrive
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
svalbard import ../manual.pdf --vault /Volumes/MyDrive --add --name field-manual
svalbard apply --vault /Volumes/MyDrive
```

The current implementation imports local files only — remote web/media ingestion is not part of the host CLI.

### `svalbard preset list`

List embedded preset names.

### `svalbard preset copy <source> <target>`

Write a preset YAML file to the target path.

Example:

```bash
svalbard preset copy default-128 ../tmp/default-128.yaml
```

### `svalbard index --vault <path>`

Build the SQLite full-text and semantic search index from realized ZIM files under the vault.

The generated database is written under `data/search.db`.

## Builder Scripts

Some recipes build their artifacts from source rather than downloading prebuilt outputs (e.g. ZIM scraping, geodata processing). These steps are implemented as Python scripts under `recipes/builders/` and run inside the `svalbard-tools` container — the host CLI invokes them through Docker on demand.

You only need Docker and the `svalbard-tools` image when working with build recipes; standard download-based provisioning has no Python or Docker dependency.
