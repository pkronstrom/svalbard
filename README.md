<p align="center">
  <img src="docs/svalbard_banner.png" alt="Svalbard" width="100%">
</p>

# Svalbard

Seed vault for human knowledge — civilization on a stick.

> **Alpha (v0.1.0)** — Under active development. Presets, datasets, and curation will change significantly. Expect rough edges.

## Why

The information we take for granted today — encyclopedias, repair manuals, medical references, maps, how-to guides — might not always be a search away. Whether you're heading off-grid, preparing for disruptions, or just want a self-contained reference library that doesn't need a connection, having critical knowledge on a physical drive is surprisingly useful.

Svalbard is a one-shot provisioner for pre-built offline knowledge archives. A bugout stick. An off-grid reference library. A civilization reboot archive. Whatever you need it to be.

The project ships with sanely curated presets built from open recipes, but it's designed to be extended and modified with your own data — and that's encouraged.

## What's on a drive

- **Encyclopedias** — Wikipedia, Wiktionary, WikiMed
- **Practical knowledge** — iFixit repair guides, Stack Exchange Q&A, Practical Action field guides
- **Books and courses** — Project Gutenberg, Wikibooks, Khan Academy
- **Maps** — OpenStreetMap regional extracts, geodata overlays
- **AI models** — Portable Gemma 4 and Qwen 3.5 GGUFs that run locally from the drive
- **Search** — Full-text and semantic search across all content — find answers, not just keywords
- **Tools** — CyberChef, Kiwix server, everything self-contained

Deploy a 2 GB emergency kit on your phone with a few apps and be offgrid-certified. Or build a complete over-the-top 2 TB doomsday vault if you feel like it.

## How it works

A svalbard drive is not a bootable image or a compressed archive — it's a plain directory of standard open formats (ZIM, PMTiles, GGUF, HTML). No extraction, no installation. The drive includes its own binaries and tools for viewing and accessing data.

**On a computer** — plug in the drive, open a terminal, run `./run`. The drive launcher opens a keyboard-driven terminal UI with grouped menus for `Search`, `Library`, `Maps`, `Local AI`, and `Tools`, plus any pack-defined extras on the drive. You can jump straight into archives, view maps, chat with local models, launch bundled AI clients like OpenCode, Crush, and Goose, inspect the drive, and share it over the local network. Works on Mac and Linux with nothing to install on the host. Windows support is planned.

**On a phone or tablet** — carry a USB-C stick, plug it into your phone, and open the files directly with apps like Kiwix (encyclopedias), OsmAnd (maps), or any PDF/EPUB reader. Or just copy the directory (or a zip of it) to your phone's filesystem — the files are standard formats that any compatible app can open. See `provisioning/` for recommended apps on iOS and Android.

## Current Architecture

The current core is Go:

- `host-cli/` contains the host-side CLI for vault init, desired-state changes, apply, status, import, presets, and indexing.
- `drive-runtime/` contains the on-drive launcher and native runtime actions.
- `recipes/` and `presets/` remain the catalog inputs consumed by the Go host binary.

Python still exists, but only as build-worker implementation detail:

- `recipes/builders/*.py` contains specialized builder scripts.
- Those scripts are intended to run inside the `svalbard-tools` container.
- The old Python host CLI and provisioner are not the active control plane on this branch.

## Presets

Presets scale from pocket-sized emergency kits to full archives:

| Preset | Size | What you get |
|--------|------|-------------|
| `default-2` | 2 GB | Universal bugout kit — medical, survival, food/water, practical references |
| `finland-2` | 2 GB | Finnish emergency field kit — extends default-2 with Finnish pharma registry |
| `default-32` | 32 GB | Core reference — Wikipedia, WikiMed, survival guides, repair manuals |
| `default-128` | 128 GB | Broad reference — adds dictionaries, books, Khan Academy, maps |
| `default-512` | 512 GB | Deep archive — adds full-picture Wikipedia plus a conservative local AI pair (`gemma-4-e2b-it`, `qwen-9b`) |
| `default-1tb` | 1 TB | Mainstream AI archive — adds stronger Gemma and Qwen options for 16-24 GB RAM hosts |
| `default-2tb` | 2 TB | Everything — full Wikipedia plus the full curated Gemma/Qwen ladder |

Finnish presets (`finland-*`) add Finnish-language Wikipedia, Wiktionary, Finnish maps, open geodata (recreation structures, nature reserves), and Finnish-specific guides on top of the English baseline.

## Walkthrough: provision your own stick

<img src="docs/demo.gif" alt="svalbard wizard demo" width="100%">

```bash
# 1. Run the Go host CLI from host-cli/
cd host-cli

# 2. Create a vault from a preset
go run ./cmd/svalbard init /Volumes/MyStick --preset default-32

# 3. Review and materialize the plan
go run ./cmd/svalbard plan --vault /Volumes/MyStick
go run ./cmd/svalbard apply --vault /Volumes/MyStick

# 4. Build the search index
go run ./cmd/svalbard index --vault /Volumes/MyStick

# 5. Done — unplug and go
cd /Volumes/MyStick && ./run
```

### Add your own content

The current Go CLI imports local files into the vault library. You can optionally add the imported item to the desired state in the same step.

```bash
# Import a local file into the vault library and desired state
go run ./cmd/svalbard import ../manual.pdf --vault /Volumes/MyStick --add --name field-manual

# Or add an existing item id, then apply again
go run ./cmd/svalbard add local:field-manual --vault /Volumes/MyStick
go run ./cmd/svalbard apply --vault /Volumes/MyStick
```

`preset copy` is available for exporting built-in preset YAML as a starting point, but local custom preset loading is not yet wired into the Go CLI on this branch.

## Commands

| Command | Description |
|---------|-------------|
| `svalbard init [path] --preset <name>` | Initialize a new vault from a preset |
| `svalbard add <item...> --vault <path>` | Add catalog or local item ids to desired state |
| `svalbard remove <item...> --vault <path>` | Remove item ids from desired state |
| `svalbard plan --vault <path>` | Show the reconciliation plan |
| `svalbard apply --vault <path>` | Download, materialize, and update the realized state |
| `svalbard status --vault <path>` | Show desired, realized, and pending items |
| `svalbard import <file> --vault <path> [--add]` | Import a local file into the vault library |
| `svalbard preset list` | List built-in presets from the embedded catalog |
| `svalbard preset copy <source> <target>` | Export a built-in preset YAML file |
| `svalbard index --vault <path>` | Build the full-text search index from vault ZIM files |

## Roadmap

- [x] Go host CLI for vault init, desired-state edits, apply, status, import, preset listing, and indexing
- [x] Parallel downloads with resume and checksum verification
- [x] Composable presets with inheritance and regional packs
- [x] Full-text and semantic search across all ZIM content
- [x] Drive toolkit — Go launcher, embedded runtime binaries, local map viewer, bundled tools
- [ ] Interactive picker / wizard replacement on top of the Go CLI
- [ ] Local custom preset loading from exported YAML
- [ ] Expanded Go-side import flows beyond local-file ingestion
- [ ] Fully curated and verified presets — every source checked, sized, and tested across tiers
- [ ] Custom Wikipedia ZIM builds — compact/resized images for space-constrained tiers
- [ ] Regional geodata packs — Finnish survival layers (shelters, water, foraging) as a first regional pack
- [ ] Search across all content — index PDFs, geodata, and reference databases alongside ZIMs
- [ ] RAG — query all drive content through the bundled local LLM
- [ ] Offline routing — turn-by-turn navigation from preprocessed OSM graphs
- [x] Offline coding assistant bootstrap — bundled llama.cpp runtime, portable models, and terminal AI clients
- [ ] Mobile workflow — guides and tooling for viewing drive content on phones via USB-C
- [ ] Hardware and programming toolkit — offline compilers, embedded toolchains, EDA tools, and documentation
- [ ] Port the remaining shell-backed drive actions to portable static binaries
- [ ] Limited Windows support

## Documentation

- [Usage guide](docs/usage.md) — current Go CLI flow, vault model, and builder boundary

## License

[GPL-3.0](LICENSE) — Free to use, modify, and distribute. Derivatives must remain open source. Individual datasets on provisioned drives carry their own licenses.
