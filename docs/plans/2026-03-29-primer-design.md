# Primer — Offline Knowledge Kit Provisioner

## What

A Python CLI tool that provisions USB sticks and SSDs with curated offline knowledge for survival and civilization rebuilding. Nordic/Finnish focus with extensible presets.

## Philosophy

1. **Files are the product** — ZIM, PDF, EPUB, GGUF, PMTiles, HTML. Universal formats. The drive is useful without primer installed.
2. **Self-sufficient drive** — includes precompiled static binaries (kiwix-serve) so you can serve/browse without internet.
3. **Presets, not prescriptions** — `nordic-128`, `nordic-256`, `nordic-1tb` are YAML files. Users can fork/edit.
4. **No runtime infrastructure** — no Docker, no databases. A directory tree you can `ls`.

## Tiers

| Preset | Target | Use Case |
|--------|--------|----------|
| `nordic-128` | 128 GB USB stick | Grab-and-go survival. Phone + laptop. Nordic focus. |
| `nordic-256` | 256 GB USB/SSD | Same + full Wikipedia with images + all Stack Exchange sites. |
| `nordic-1tb` | 1 TB SSD | Civilization rebuild. Deep technical, medical, agricultural, educational. LLMs. Linux ISO. |

## Drive Layout

```
PRIMER/
├── bin/
│   ├── macos-arm64/kiwix-serve
│   ├── linux-x86_64/kiwix-serve
│   └── linux-arm64/kiwix-serve
├── zim/                          # Kiwix ZIM files
├── maps/                         # PMTiles
├── books/                        # PDFs, EPUBs
├── models/                       # GGUF, llamafile (optional)
├── courses/                      # Kolibri DBs or standalone HTML
├── apps/                         # Portable apps (CyberChef.html)
├── installers/                   # Optional: Kiwix.dmg, LMStudio.dmg
├── infra/                        # Optional: Linux ISO, apt cache
├── serve.sh                      # One-liner: detect OS, run kiwix-serve
├── README.md
└── manifest.yaml                 # What's on this drive, versions, checksums
```

## CLI Commands

```
primer wizard              # Interactive setup (calls init + sync)
primer init <path> --preset nordic-128
primer sync                # Download/update content
primer status              # What's downloaded, what's stale
primer coverage            # Domain coverage scores
primer audit > audit.md    # LLM-ready gap analysis report
primer serve               # Run kiwix-serve from drive
```

## Wizard Flow

Sequential Rich-powered prompts (like `npm init`):

1. **Target** — auto-detect mounted volumes, show sizes/formats, or enter custom path
2. **Preset** — recommend based on drive size, show what each includes
3. **Options** — toggle optional content (LLMs, app installers, Linux ISO, maps)
4. **Review** — summary table, estimated download size, confirm
5. **Download** — progress bars per file, resume support, checksum verification

The wizard just collects answers and calls `primer init` + `primer sync`.

## Preset YAML Format

```yaml
name: Nordic 128GB
description: Grab-and-go survival kit, Nordic/Finnish focus
target_size_gb: 120

sources:
  - id: wikipedia-en-nopic
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim
    tags: [general-reference, medicine, agriculture, engineering, science]
    depth: comprehensive

  - id: wikipedia-fi-nopic
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikipedia/wikipedia_fi_all_nopic_{date}.zim
    tags: [general-reference, language]
    depth: comprehensive

  - id: osm-finland-nordics
    type: pmtiles
    url: https://build.protomaps.com/...
    tags: [maps-geo, navigation]
    depth: comprehensive
    optional_group: maps
```

Sources that exist in multiple tiers use `replaces` to handle upgrades:

```yaml
  - id: wikipedia-en-full
    type: zim
    url_pattern: https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_maxi_{date}.zim
    replaces: wikipedia-en-nopic  # In presets that include both, full wins
```

## Taxonomy & Coverage

Fixed taxonomy of ~30 knowledge domains grouped into 7 areas:

- **Survival**: water, fire-shelter, food-foraging, navigation, first-aid, self-defense
- **Medical**: medicine, dentistry, pharmacy, emergency-medicine, mental-health
- **Food**: agriculture, gardening, animal-husbandry, food-preservation, cooking
- **Engineering**: electronics, mechanical, civil-construction, metalworking, woodworking, energy-power
- **Tech**: computing, radio-comms, networking-mesh, 3d-printing, drones-robotics
- **Science**: chemistry, physics, biology, earth-science, mathematics
- **Society**: education-pedagogy, governance-law, trade-economics, history, language
- **Reference**: general-reference, maps-geo, repair

Each source is tagged with domains + depth (comprehensive / overview / reference-only). Coverage is scored per domain, gaps flagged.

## Audit Report

`primer audit` generates a markdown file with:

- Baked-in system prompt for AI gap analysis
- Full inventory table (all files, sizes, tags, dates)
- Coverage matrix with scores
- Format accessibility matrix (what opens on macOS / iOS / Android / Linux)
- Free space available
- Platform requirements

Designed to be pasted directly into any LLM for analysis.

## Content: nordic-128 (~110 GB)

| Category | Content | ~Size |
|----------|---------|-------|
| Reference | Wikipedia EN (no images) | 25 GB |
| Reference | Wikipedia FI (no images) | 5 GB |
| Reference | Wiktionary EN + FI | 7 GB |
| Medical | WikiMed (with images) | 2 GB |
| Medical | Where There Is No Doctor + Dentist | 0.5 GB |
| Skills | WikiHow EN | 15 GB |
| Skills | iFixit | 5 GB |
| Skills | Stack Exchange (Survival, DIY, Home Improvement, Gardening, Cooking, Amateur Radio) | 15 GB |
| Skills | Practical Action | 1 GB |
| Education | Wikibooks EN | 3 GB |
| Education | Khan Academy lite (no video) | 10 GB |
| Maps | OSM Finland + Nordics | 8 GB |
| Library | Project Gutenberg (text subset) | 10 GB |
| Apps | CyberChef.html | 0.1 GB |
| Binaries | kiwix-serve (3 platforms) | 0.1 GB |

## Content: nordic-256 (~215 GB)

Everything in nordic-128, plus:

| Change | Detail | Delta |
|--------|--------|-------|
| Wikipedia EN | Upgraded to full with images | +85 GB |
| Stack Exchange | All 20 sites (+ Electronics, Motor Vehicle, Woodworking, Outdoors, Homebrewing, Arduino, Raspberry Pi, Sustainable Living, Chemistry, Medical Sciences, Biology, Earth Science, Engineering, 3D Printing, Physics) | +20 GB |

## Content: nordic-1tb (~620 GB)

Everything in nordic-256, plus:

| Category | Content | ~Size |
|----------|---------|-------|
| Reference | Wikipedia SV, NO, DA (no images) | 15 GB |
| Reference | Wikiversity, Wikivoyage | 3 GB |
| Medical | MedlinePlus, LibreTexts Medicine, NHS, CDC, LibrePathology, WikEM | 5 GB |
| Skills | Low-tech Magazine, Appropriate Technology Library | 6 GB |
| Education | Khan Academy full (Kolibri, compressed video) | 50 GB |
| Education | LibreTexts (all subjects) | 12 GB |
| Education | OpenStax Textbooks | 3 GB |
| Education | TED-Ed | 6 GB |
| Education | freeCodeCamp, DevDocs | 0.2 GB |
| Library | Project Gutenberg (full) | 60 GB |
| Library | Survivor Library (1800s tech) | 100 GB |
| Maps | OSM full Europe | 40 GB |
| Models | Llamafile 8B general | 5 GB |
| Models | Llamafile 8B medical/science | 5 GB |
| Models | Large GGUF 70B Q4 | 40 GB |
| Apps | Kolibri portable | 0.2 GB |
| Installers | Kiwix.dmg, LMStudio.dmg, etc. (optional) | 5 GB |
| Infra | Bootable Linux ISO | 5 GB |
| Infra | Offline apt package cache | 10 GB |

## Tech Stack

- Python 3.11+
- Rich (CLI output, prompts, progress bars, tables)
- PyYAML (config)
- aria2c for downloads (falls back to wget/curl)
- No other runtime dependencies

## Brewfile (separate)

For provisioning the laptop while online:

```ruby
brew "kiwix-tools"
brew "aria2"
brew "python@3.11"
cask "kiwix"
cask "lm-studio"
```

## Future Presets (not in v1)

- `us-128`, `us-256`, `us-1tb` — US-centric content, US maps
- `max` — everything, fill whatever drive you have
- Custom — user picks domains and budget, primer selects optimal sources
