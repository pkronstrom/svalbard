<p align="center">
  <img src="docs/svalbard_banner.png" alt="Svalbard" width="100%">
</p>

# Svalbard

Seed vault for human knowledge — civilization on a stick.

> **Alpha (v0.4.0)** — Under active development. Presets, datasets, and curation will change significantly. Expect rough edges.
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
- **AI models** — Portable Gemma 4 (QAT), Qwen 3.5 and Qwen 3.8 GGUFs that run locally from the drive; [RAM tiers and optional MTP](docs/plans/2026-09-30-catalog-refresh-results.md)
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

Requires Go 1.26+.

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
| `make test` / `make verify` | Check catalog parity and test all five Go modules |
| `scripts/sync-catalog.sh` | Regenerate the embedded catalog after editing `recipes/` or `presets/` |
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

| Preset | Content | What you get |
|--------|---------|-------------|
| `default-2` | ~1.5 GiB | Bugout kit — emergency medicine (WikEM, WHO, field manuals), water/food, knots, appropriate tech |
| `default-32` | ~17 GiB | Survival core — Wikipedia top 50k, WikiMed, MedlinePlus, iFixit, repair/homesteading guides, practical Q&A |
| `default-64` | ~40 GiB | Adds pictured top Wikipedia, Wikibooks, Gutenberg agriculture + medicine, trades textbooks, electronics/radio Q&A |
| `default-128` | ~91 GiB | Full-text English Wikipedia (no pictures), Wiktionary, OSM wiki, DSP/IoT Q&A |
| `default-256` | ~170 GiB | Sciences, Gutenberg technology, Make It Yourself, prepping videos, and a starter local AI (8–16 GB RAM models) |
| `default-512` | ~366 GiB | Adds Khan Academy and 24 GB-class local AI |
| `default-1tb` | ~700 GiB | Pictured English Wikipedia, Survivor Library, 64 GB-class local AI |
| `default-2tb` | ~1 TiB | Full Gutenberg, Stack Overflow, sysadmin Q&A — everything curated |

Sizes are catalog estimates in GiB; every tier leaves at least ~15% of the drive free for search indexes and your own files. Specialization packs (cybersec, computing, sciences, engineering, communications, ESP32 dev, homesteading) are opt-in and not part of the default tiers.

Finnish presets (`finland-*`) extend the same-size default tier plus the next-smaller Finnish tier: Finnish Wikipedia and Wiktionary, Finnish Gutenberg/Wikibooks/Wikisource, partiotaidot (scoutwiki), the Fimea medicine registry, and — from 32 GB up — Finnish maps and open geodata (recreation structures, nature reserves, groundwater).

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

### Custom presets

A custom preset is a YAML file in `~/.config/svalbard/presets/` (`$XDG_CONFIG_HOME/svalbard/presets/`). It composes built-in presets and packs with the same rules the built-in presets use: `extends` sources are merged in order and deduplicated by id, `sources` adds recipe ids, and `-id` drops an inherited one.

```yaml
# ~/.config/svalbard/presets/my-256.yaml
name: my-256                      # optional; defaults to the file name
description: finland-256 plus Qwen 27B and cybersec
extends: [finland-256, cybersec, esp32-dev]
sources: [qwen-27b, -gutenberg-en-lcc-t]
```

```bash
svalbard preset copy finland-256 my-256   # writes an extends stub to ~/.config/svalbard/presets/my-256.yaml
svalbard preset list                      # custom presets are marked "(user: <file>)"
svalbard init                             # the wizard lists them, marked "(custom)"
svalbard add cybersec --vault /Volumes/MyStick   # add/remove also accept preset and pack names
```

Custom presets are checked when the catalog loads, and any command fails with the file name if one is invalid: a custom preset may not reuse a built-in or another custom preset's name, and every `extends` entry and source id (including `-id` removals) must exist. A custom preset without a `region` takes the region of the first preset it extends, or `custom` if that has none. In a development checkout, the gitignored `local/presets/` is also read. `preset copy --flatten` writes the fully resolved source list instead of an `extends` stub.

## Commands

| Command | Description |
|---------|-------------|
| `svalbard` | Launch the interactive TUI (dashboard or wizard) |
| `svalbard init [path]` | Open the guided wizard to create a vault — preset is chosen interactively |
| `svalbard add <item...> --vault <path>` | Add catalog or local item ids, or every item of a preset/pack, to desired state |
| `svalbard remove <item...> --vault <path>` | Remove item ids, or every item of a preset/pack, from desired state |
| `svalbard plan --vault <path>` | Show the reconciliation plan |
| `svalbard apply --vault <path>` | Download, materialize, and update the realized state |
| `svalbard status --vault <path>` | Show desired, realized, and pending items |
| `svalbard import <file> --vault <path> [--add]` | Import a local file into the vault library |
| `svalbard preset list` | List built-in and custom presets |
| `svalbard preset copy <source> <target> [--flatten]` | Start a custom preset that extends `<source>` (a bare `<target>` name goes to `~/.config/svalbard/presets/`) |
| `svalbard index --vault <path>` | Build the full-text search index from vault ZIM files |

Set `SVALBARD_DEBUG=1` for verbose structured logging (written to `$TMPDIR/svalbard.log`).

Set `SVALBARD_CACHE_DIRS=/path/one:/path/two` to reuse already-downloaded files (e.g. a local model cache like `~/.models`) during `apply`. Each catalog file is matched by filename in those directories, size-checked against the source, and copied into the vault instead of re-downloaded.

## Roadmap

- [x] Go host CLI for vault init, desired-state edits, apply, status, import, preset listing, and indexing
- [x] Parallel downloads with resume and checksum verification
- [x] Composable presets with inheritance and regional packs
- [x] Full-text and semantic search across all ZIM content
- [x] Drive toolkit — Go launcher, embedded runtime binaries, local map viewer, bundled tools
- [x] Interactive TUI wizard for guided vault setup
- [x] Self-contained release binary with embedded drive-runtime and catalog
- [x] Custom presets composed from built-in presets and packs
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
