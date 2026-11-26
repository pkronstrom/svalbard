---
name: svalbard
description: Create and manage Svalbard offline vaults with the svalbard CLI. Use when the user wants to put offline knowledge, maps, tools or local AI models on a drive or folder, or to check, extend, update or trim an existing vault.
---

# Svalbard vaults

A vault is a folder (usually a USB drive) holding offline content and a launcher, `./run`. Its `manifest.yaml` lists what the user selected and what is installed. `svalbard apply` makes the installed content match the selection.

Use the commands below, never the interactive screens: `svalbard` with no arguments and `svalbard init` without `--preset` or `--add` open a full-screen interface you cannot drive. Do not edit `manifest.yaml` by hand.

Commands that read or change a vault take `--vault <path>`, or find the vault when run inside it. Listing commands take `--json`; prefer it over reading tables.

## Before you download or delete

`apply` and `update` download, and can delete, many gigabytes. Before running either one, tell the user the destination, the download size and what will be removed, and wait for a yes. Get those numbers from `svalbard plan --json` (`download_gb`, `remove`, `remove_gb`) or from the preset size.

## Create a vault

```bash
svalbard preset list --json
svalbard preset show default-64 --json
svalbard catalog wikipedia --json
svalbard init /Volumes/MyDrive --preset default-64 --add ai/harnesses --remove cd3wd --platform all --apply
```

- Pick the preset whose `size_gb` fits the drive. A preset name ends in its target size in GB. `preset list` has each preset's total; `preset show` lists its items with their sizes.
- `--preset` is optional: `--add` alone creates a vault with only those items and what they depend on.
- `--add` and `--remove` take item ids, pack names and preset names. Dependencies are included for you.
- `--platform` names the computers the drive will be used on: `macos-arm64`, `macos-x86_64`, `linux-x86_64`, `linux-arm64`, or `all`. The default is this computer only, so ask where the drive will be used.
- The parent folder must exist. Without `--apply` the vault is created empty; run `svalbard apply` later.

## Look at a vault

```bash
svalbard status --vault /Volumes/MyDrive --json
svalbard plan --vault /Volumes/MyDrive --json
```

`status` gives each item a `status` (`realized`, `pending`, `unavailable`, or `other platform` for an item that has no download for this vault's platforms and is skipped) and, when installed, a `version`:

| version | meaning |
|---|---|
| `current` | installed from the recipe as it is now |
| `update` | the recipe changed; an update is available |
| `unknown` | installed before versions were recorded |
| `withdrawn` | no longer in the catalog; the installed copy is kept |
| `local` | a file the user imported |
| `documents` | a document collection; `apply`, not `update`, fetches its new and changed documents |

## Add or remove content

```bash
svalbard catalog knots --json
svalbard add zimgit-knots --vault /Volumes/MyDrive
svalbard remove cd3wd --vault /Volumes/MyDrive
svalbard plan --vault /Volumes/MyDrive --json
svalbard apply --vault /Volumes/MyDrive
```

`catalog` matches its words against ids, descriptions, tags and pack names; use `type` and `description` to choose between hits. `add` and `remove` only change the selection; nothing is downloaded or deleted until `apply`. `apply` never replaces content that is already installed.

A dependency that `add` brought in stays selected when the item that needed it is removed. Remove it too if nothing else needs it; `remove` warns when something still does.

## Update installed content

```bash
svalbard update --vault /Volumes/MyDrive --json
svalbard update --all --vault /Volumes/MyDrive
svalbard update zimgit-knots --vault /Volumes/MyDrive
```

- Without arguments, `update` only lists what has an update.
- `--all` updates those items. Naming ids downloads those items again whatever their state.
- The installed copy is kept until the new one is in place, so an update needs room for both. If it is refused for space, offer `--replace`, which deletes first, and say that a failed download then leaves the item missing until the next `apply`.

## Withdrawn items

A `withdrawn` item cannot be downloaded again. It stays on the vault through `apply` and `update`. Never remove one unless the user asks for that item by name, and tell them it is not recoverable. If it was removed by mistake and `apply` has not run, `svalbard add <id>` takes the removal back.

## Import a file

```bash
svalbard import ./manual.pdf --vault /Volumes/MyDrive --add --name field-manual
svalbard apply --vault /Volumes/MyDrive
```

## Search index

`apply` builds the search index. To rebuild it: `svalbard index --vault /Volumes/MyDrive --type keyword` (or `semantic`, `full`).

## When something fails

- `apply` prints a line as each item starts and finishes, and exits non-zero if any failed. Running it again continues with what is missing.
- An `unavailable` item is selected, not installed and gone from the catalog; `apply` fails until you `svalbard remove` it.
- Build recipes need Docker. `plan --json` sets `tools_required` when a pending item is one.
