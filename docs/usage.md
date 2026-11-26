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

Everything the wizard and the TUI screens do has a command, so scripts and agents never need the TUI:

```bash
svalbard init /Volumes/MyDrive --preset default-64 --apply   # create and download in one step
svalbard catalog wikipedia                                   # find item ids
svalbard add wikipedia-en-100 --vault /Volumes/MyDrive       # select more, with dependencies
svalbard plan --vault /Volumes/MyDrive                       # what apply would do, with sizes
svalbard apply --vault /Volumes/MyDrive
svalbard status --vault /Volumes/MyDrive
```

`plan`, `status`, `catalog`, `preset list` and `preset show` take `--json`. Commands run inside a vault directory find it without `--vault`.

## Catalog And Presets

The CLI loads the built-in catalog from embedded copies of `recipes/` and `presets/`.

- `svalbard preset list` shows the embedded preset names and your custom presets.
- `svalbard preset copy <source> <target>` starts a custom preset in `~/.config/svalbard/presets/` that extends `<source>`.
- The init wizard offers the embedded presets and every valid custom preset.

A custom preset that fails validation is skipped with a warning naming the file and the reason; see "Custom presets" in the README.

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

Without flags, open the guided setup wizard. The wizard handles preset selection, vault directory creation, manifest seeding, and the initial download/apply in one flow. The optional `path` argument pre-fills the destination so you don't have to type it in the wizard.

With `--preset` or `--add`, create the vault without the wizard and with the same result:

- `--preset <name>`: the preset to start from
- `--add <ids>` / `--remove <ids>`: recipe ids, packs or presets to add or leave out; required dependencies are included
- `--platform <names>`: platforms the vault runs on, or `all` (default: this computer's)
- `--apply`: download the content and build the search index right away

Example:

```bash
svalbard init /Volumes/MyDrive
svalbard init /Volumes/MyDrive --preset default-64 --add ai/harnesses --platform all --apply
```

### `svalbard catalog [words...] [--json]`

List the items a vault can hold: id, type, size, the packs that include it and a description. Every word given must appear in one of those fields.

### `svalbard add <item...> --vault <path>`

Add items to the vault's desired state, together with the dependencies they need (a ZIM brings `kiwix-serve`). An id the catalog does not have is an error and changes nothing.

Each argument is one of:

- a recipe id from `svalbard catalog`
- a preset or pack name, which adds every item in it
- a local imported item such as `local:manual`

### `svalbard remove <item...> --vault <path>`

Remove items (or every item of a preset or pack) from desired state. It warns when a remaining item still needs one you removed, and fails if none of the named items was selected.

### `svalbard plan --vault <path> [--json]`

Show what `apply` would download or remove, with sizes and build requirements.

Use this before `apply` when you want to inspect the reconciliation result.

### `svalbard apply --vault <path>`

Execute the reconciliation plan:

- resolve item URLs
- download artifacts
- update realized entries in `manifest.yaml`
- regenerate runtime metadata and launch scripts
- print one line per item as it starts, finishes or fails
- build the vault's configured search index
- generate or remove the offline map viewer based on realized PMTiles

### `svalbard update [item...] [--all] [--replace] [--json]`

Update installed items whose recipe changed since they were installed. `apply` never does this: it only downloads what is missing and deletes what was deselected.

- no arguments: list the items with an update available
- `--all`: update every one of them
- item ids: acquire those items again whatever their state, which also picks up a newer upstream file for a recipe that follows the latest release

Each installed copy is kept until its new one is in place, and put back if the download or build fails. That needs room for both copies; `--replace` deletes the installed copy first, and a failed `--replace` leaves the item pending.

`status` shows a version for every installed item:

- `current`: installed from the recipe as it is now
- `update available`: the recipe changed
- `unknown`: installed before versions were recorded; `svalbard update <id>` refreshes it
- `withdrawn, kept`: no longer in the catalog. It cannot be updated, stays on the vault through `apply` and `update`, and goes only if you `remove` it
- `local`: a file you imported

Python tools cannot be updated this way yet; remove them, apply, and add them back.

### `svalbard status --vault <path> [--json]`

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

### `svalbard preset list [--json]`

List preset and pack names. `--json` adds kind, region, size, item count and description.

### `svalbard preset show <name> [--json]`

List the items a preset or pack contains.

### `svalbard preset copy <source> <target>`

Write a preset YAML file to the target path.

Example:

```bash
svalbard preset copy default-128 ../tmp/default-128.yaml
```

### `svalbard index --vault <path> [--type keyword|semantic|full]`

Build the SQLite search index from realized ZIM files under the vault: the keyword index by default, `semantic` for embeddings (needs an embedding model on the vault), `full` for both.

The generated database is written under `data/search.db`.

## Builder Scripts

Some recipes build their artifacts from source rather than downloading prebuilt outputs (e.g. ZIM scraping, geodata processing). These steps are implemented as Python scripts under `recipes/builders/` and run inside the `svalbard-tools` container — the host CLI invokes them through Docker on demand.

You only need Docker and the `svalbard-tools` image when working with build recipes; standard download-based provisioning has no Python or Docker dependency.

`makeityourself` (family `content-archive`) archives the 900+ links in the Make It Yourself PDF. Printables models come from the Printables GraphQL API, Thingiverse things from the Thingiverse API when `THINGIVERSE_TOKEN` is set, YouTube links as WebM video via `yt-dlp`, and links that fail live from the Wayback Machine. A project's chapter pages on the same site (a guide's steps, a Hackaday project's instructions, files and logs, the Markdown docs a GitHub README links) are added as sections of its page, and projects with more than three files offer them as one zip, which the bundled `7z` opens. The GitHub repositories a project is, or links to (up to three), are packed as zips with their READMEs added as sections: a repository under 25 MB whole, a larger one as its docs, design files, PCBs and firmware up to 50 MB, with the rest listed in `OMITTED.txt`. Set `GITHUB_TOKEN` for this, since the GitHub API allows 60 requests an hour without one. Up to two build guides a project links on other sites are added as sections too, with their chapters and files: project pages on Adafruit Learn, Instructables, Hackaday, Printables, Thingiverse and Hackster, and links whose text names a guide, instructions or plans. GitHub wikis are included with a repository's docs. The recipe's `max_files_mb`, `video_height`, `max_video_minutes` and `video_embeds` settings bound its size.
