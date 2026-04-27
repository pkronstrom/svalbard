<p align="center">
  <img src="docs/svalbard_banner.png" alt="Svalbard" width="100%">
</p>

# Svalbard

Seed vault for human knowledge — civilization on a stick.

> **Alpha (v0.2.0)** — Under active development. Presets, datasets, and curation will change significantly. Expect rough edges.
>
> **Heads up:** the codebase was recently rewritten from Python to Go. Not every flow has been re-tested end-to-end on the new code path yet, so expect breakages — please file an issue if you hit one.

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

## Install

### Pre-built binary (recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/pkronstrom/svalbard/releases). The binary is self-contained — no Go toolchain or runtime dependencies required.

```bash
# macOS (Apple Silicon)
tar xzf svalbard_darwin_arm64.tar.gz
sudo mv svalbard /usr/local/bin/

# Linux (x86_64)
tar xzf svalbard_linux_amd64.tar.gz
sudo mv svalbard /usr/local/bin/
```

The binary handles all download-based recipes (ZIMs, maps, models, PDFs, binaries) with zero dependencies. For build recipes (site scraping, geodata processing) and media import (YouTube, Yle), you also need Docker:

```bash
docker pull ghcr.io/pkronstrom/svalbard-tools
```

The `svalbard-tools` image includes warc2zim, tippecanoe, zim-tools, GDAL, ffmpeg, yt-dlp, yle-dl, and other build tools. Svalbard falls back to it automatically when a build step needs a tool that isn't available locally.

### Build from source

Requires Go 1.25+.

```bash
git clone https://github.com/pkronstrom/svalbard.git
cd svalbard

# Full build — embeds drive-runtime binaries for all platforms (~71 MB)
make build

# Dev build — smaller binary, compiles drive-runtime on-demand at apply-time
make build-dev
```

The built binary is at `bin/svalbard`.

| Make target | What it does |
|-------------|-------------|
| `make build` | Cross-compile drive-runtime for 4 platforms, embed into svalbard binary |
| `make build-dev` | Quick build without embedding (requires Go at apply-time) |
| `make build-drive-runtime` | Only cross-compile drive-runtime binaries |
| `make test` | Run all tests |
| `make clean` | Remove build artifacts |

## Architecture

The core is Go:

- `host-cli/` — host-side CLI for vault init, desired-state changes, apply, status, import, presets, and indexing
- `host-tui/` — host-side TUI app (Bubble Tea) — interactive dashboard and init wizard
- `tui/` — shared TUI component library used by `host-tui/` and `drive-runtime/` (layout, theme, key bindings, tree picker, progress views)
- `drive-runtime/` — on-drive launcher and native runtime actions, embedded into the host binary at build time
- `recipes/` and `presets/` — catalog inputs, embedded into the host binary via `//go:embed`

Python exists only as a build-worker detail:

- `recipes/builders/*.py` — specialized builder scripts for Docker-based builds (ZIM scraping, geodata processing)
- These run inside the `svalbard-tools` container and are not needed for standard download-based provisioning

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
# 1. Launch the guided setup wizard — pick a preset and apply
svalbard init /Volumes/MyStick

# 2. Build the search index
svalbard index --vault /Volumes/MyStick

# 3. Done — unplug and go
cd /Volumes/MyStick && ./run
```

`svalbard init` opens an interactive wizard that handles preset selection, manifest creation, and the initial download/apply in one flow. Run `svalbard` with no arguments to launch the dashboard for an existing vault. The non-interactive `add`/`remove`/`plan`/`apply`/`status`/`import` subcommands are for managing a vault that already exists.

### Add your own content

```bash
# Import a local file into the vault library and desired state
svalbard import ../manual.pdf --vault /Volumes/MyStick --add --name field-manual

# Or add an existing item id, then apply again
svalbard add local:field-manual --vault /Volumes/MyStick
svalbard apply --vault /Volumes/MyStick
```

`preset copy` is available for exporting built-in preset YAML as a starting point, but local custom preset loading is not yet wired into the Go CLI on this branch.

## Commands

| Command | Description |
|---------|-------------|
| `svalbard` | Launch the interactive TUI (dashboard or wizard) |
| `svalbard init [path]` | Open the guided wizard to create a vault — preset is chosen interactively |
| `svalbard add <item...> --vault <path>` | Add catalog or local item ids to desired state |
| `svalbard remove <item...> --vault <path>` | Remove item ids from desired state |
| `svalbard plan --vault <path>` | Show the reconciliation plan |
| `svalbard apply --vault <path>` | Download, materialize, and update the realized state |
| `svalbard status --vault <path>` | Show desired, realized, and pending items |
| `svalbard import <file> --vault <path> [--add]` | Import a local file into the vault library |
| `svalbard preset list` | List built-in presets from the embedded catalog |
| `svalbard preset copy <source> <target>` | Export a built-in preset YAML file |
| `svalbard index --vault <path>` | Build the full-text search index from vault ZIM files |

Set `SVALBARD_DEBUG=1` for verbose structured logging (written to `$TMPDIR/svalbard.log`).

## Roadmap

- [x] Go host CLI for vault init, desired-state edits, apply, status, import, preset listing, and indexing
- [x] Parallel downloads with resume and checksum verification
- [x] Composable presets with inheritance and regional packs
- [x] Full-text and semantic search across all ZIM content
- [x] Drive toolkit — Go launcher, embedded runtime binaries, local map viewer, bundled tools
- [x] Interactive TUI wizard for guided vault setup
- [x] Self-contained release binary with embedded drive-runtime and catalog
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
