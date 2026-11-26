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

- `svalbard preset list` shows the embedded preset names and your custom presets.
- `svalbard preset copy <source> <target>` starts a custom preset in `~/.config/svalbard/presets/` that extends `<source>`.
- The init wizard offers the embedded presets and every valid custom preset.

A custom preset that fails validation is skipped with a warning naming the file and the reason; see "Custom presets" in the README.

## Vault Layout

After `apply`, a vault typically contains:

- `manifest.yaml`: desired and realized state
- `library/`: imported local files and library metadata
- `zim/`, `maps/`, `models/`, `tools/`: realized content, depending on selected items
- `wiki/`: your own wiki pages, one Markdown file per page, created the first time the wiki is used
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

## Your own documents

A document collection puts your own PDFs on a vault and makes every page searchable: manuals, datasheets, guides, by URL or from your disk. You describe the collection in one small file; `svalbard apply` fetches what is missing and indexes it.

### 1. Write the collection file

Create `~/.config/svalbard/recipes/car-manuals.yaml` (the file name is the collection's id):

```yaml
description: Owner and workshop manuals for our vehicles
tags: [vehicles, repair]
documents:
  - title: Volvo 240 Owner's Manual (1990)
    url: https://example.org/manuals/volvo-240-1990.pdf
  - title: HMMWV Operator's Manual
    url: https://www.liberatedmanuals.com/TM-9-2320-280-10.pdf
    filename: hmmwv-operator.pdf          # optional: the name on the vault
    sha256: 2a2f590ff8b0f81b4570191d9b4ef0271e2771da62b3dfb53a8d77971675a1fe   # optional pin
  - title: Trailer manual
    path: ~/Documents/manuals/trailer.pdf
  - path: ~/Documents/manuals/workshop/   # every PDF below, subfolders kept
```

- Each entry is a `url:`, a local file, or a local folder. `title:` is how the document is cited in search results; without it the PDF's own title or the file name is used.
- A relative `path:` starts at the folder of the collection file; `~` is your home folder.
- A file that cannot be used is skipped with a warning that names the file and the reason, for example two documents that would be saved under the same name (`set filename:`).

### 2. Add and apply

```bash
svalbard add car-manuals --vault /Volumes/MyDrive
svalbard plan --vault /Volumes/MyDrive      # new car-manuals: 4 documents
svalbard apply --vault /Volumes/MyDrive
```

The files land in `books/car-manuals/` on the vault, and apply reports each document as it is indexed:

```text
Indexing documents:
  car-manuals/volvo-240-1990.pdf   312 pages
  car-manuals/hmmwv-operator.pdf   421 pages
  car-manuals/trailer.pdf          not searchable: scanned: no text layer, needs OCR
  car-manuals/workshop/brakes.pdf  44 pages; 12 of 56 pages are scanned images without text
```

| The line says | What it means |
|---|---|
| `312 pages` | Every page with text is searchable. |
| `not searchable: scanned: no text layer, needs OCR` | The PDF is pictures of pages. Run it through an OCR tool and apply again. |
| `44 pages; 12 of 56 pages are scanned images without text` | Part of it is searchable. |
| `not searchable: password-protected` | It needs a password to open. Save a copy without one. A PDF that is only locked against printing or editing reads normally. |
| `not searchable: unreadable PDF: ...` | The file is damaged or not a PDF. |
| `skipped: cannot read file: ...` | The file could not be read this time; the next index tries again. |

Documents that are not searchable are listed at every index run until they are fixed or removed. They are still on the vault and open normally.

### 3. Change it later

Edit the file and apply again. Only the difference is fetched, deleted and indexed:

```bash
svalbard plan --vault /Volumes/MyDrive      # update car-manuals: +1 document, -1 document
svalbard apply --vault /Volumes/MyDrive
```

- A document taken out of the file is deleted from the vault. `svalbard remove car-manuals` followed by apply removes the whole collection.
- A local file is copied again when its size changes; a `url:` document when its `sha256:` pin changes. To fetch a changed document from the same URL, give it a new `filename:` or pin it.
- If one document fails (a dead link, or a page that asks you to log in instead of the PDF), apply stops with an error that names its URL. The documents that arrived stay; fix or remove the entry and apply again. `svalbard index` makes what arrived searchable in the meantime.

### On the drive

Search finds document pages next to archive articles. A hit is one page, titled with the document, the page number and, when the PDF has an outline, the section:

```text
  1. [car-manuals/volvo-240-1990.pdf] Volvo 240 Owner's Manual (1990), p. 112 · Cooling system
     ...»Coolant« »capacity« incl. heater: 9.5 litres. Use only type C...
```

Opening it shows the PDF in your browser at that page. A search lists at most three pages of one document, so one long manual does not push everything else out.

AI tools connected to the drive get the same pages through `search` and `search_read`, with the page number to cite. They can limit a search to one document or to a whole collection (`books/car-manuals`).

### Another computer, a missing folder

- **The vault keeps a copy of the collection file** in `.svalbard/recipes/`. On a computer that does not have your file, plan and apply use that copy and say so. Your own file wins when it exists.
- **Documents from your disk are never taken from another computer.** With the vault's copy, `path:` entries are not read, even if the same path exists there; the installed files stay and are listed as `kept ...: local source, not refreshed from the vault's copy`. Their `url:` documents are fetched as usual.
- **A folder or file that cannot be read keeps its documents.** If a `path:` is missing, on an unplugged disk, or a folder is empty because it has not synced, nothing is deleted; plan lists each document as `kept`. To remove them, take the entry out of the file.

## The wiki

A drive built from a default preset has a wiki: a place for your own notes, next to the reference library. Add it to another vault with `svalbard add silverbullet` and apply.

- **Open it** with **Wiki** in the drive's menu (under Apps), or `./run wiki`. It opens in the browser on `http://localhost:8088`, with a start page and no login. Ctrl+C stops it.
- **The pages are plain Markdown files** in `wiki/` on the drive: page `Projects/Garden plan` is the file `wiki/Projects/Garden plan.md`. They can be read, edited and copied without the wiki running. A page changed on disk shows in an open browser tab, and in the wiki's search, within a second.
- **The AI clients can read and write the pages.** pi, Goose, opencode and Open WebUI get eight tools from the drive: `wiki_list`, `wiki_read`, `wiki_search`, `wiki_create`, `wiki_append`, `wiki_edit`, `wiki_move` and `wiki_delete`. Ask one to "summarise what the vault says about burns into a wiki page", and the page appears. The wiki does not have to be open for this; while it is, the answer carries a link to the page. In Open WebUI the wiki tools are part of **Svalbard vault** and switch on and off with it.
- **A page an AI client deletes is not erased.** It is moved to `.svalbard/state/wiki-trash/` on the drive, under its name and the time. Move the file back into `wiki/` to restore it. Edits, appends and moves are not versioned.
- **Page names**: no `.md`, `/` for folders. A new name cannot contain `@ # | < > $ " * : ?`, start with a dot, or end like a file type (`Version 1.2`); the tools say what to send instead. A page you named yourself keeps its name.
- **The AI clients write text, not programs.** SilverBullet runs `${...}` in a page as Lua, and scripts on pages under `Library/`. The tools do not write under `Library/` and do not add a `${` outside a code block. What you write there yourself works as SilverBullet documents it.
- **Settings**, under Wiki in the Settings screen or with `./run settings`: `silverbullet.open_browser` (on) and `silverbullet.fs_watch` (`auto`; set `poll` if pages changed on the stick do not show up in the open wiki).
- **Serve Everything starts the wiki too, on this computer only**, whatever address the other services use: the wiki has no login. The files in `wiki/` are still part of the drive that Files shares read-only.

The wiki is [SilverBullet](https://silverbullet.md) 3.0.1. It keeps nothing on the drive but the pages: its index and settings are rebuilt in a temporary folder on the computer at each start and removed when it stops.

## Builder Scripts

Some recipes build their artifacts from source rather than downloading prebuilt outputs (e.g. ZIM scraping, geodata processing). These steps are implemented as Python scripts under `recipes/builders/` and run inside the `svalbard-tools` container — the host CLI invokes them through Docker on demand.

You only need Docker and the `svalbard-tools` image when working with build recipes; standard download-based provisioning has no Python or Docker dependency.
