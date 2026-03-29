# Primer — Offline Knowledge Kit Provisioner

## What

A Python CLI tool that provisions USB sticks and SSDs with curated offline knowledge for survival and civilization rebuilding. Nordic/Finnish focus with extensible presets.

## Philosophy

1. **Files are the product** — ZIM, PDF, EPUB, GGUF, PMTiles, HTML. Universal formats. The drive is useful without primer installed.
2. **Self-sufficient drive** — includes precompiled static binaries (kiwix-serve, go-pmtiles, llama-server) so you can serve/browse without internet or package managers.
3. **Presets, not prescriptions** — `nordic-128`, `nordic-256`, `nordic-1tb` are YAML files. Users can fork/edit.
4. **No runtime infrastructure** — no Docker, no Python, no databases on the drive. A directory tree you can `ls`.
5. **Brewfile for laptop prep** — installs Kiwix, LM Studio, etc. while you have internet. Separate concern from drive contents.

## Tiers

Fixed size points, each independently curated for its capacity:

| Preset | Target | Use Case |
|--------|--------|----------|
| `nordic-32` | 32 GB USB | Bare essentials. Wikipedia mini + core survival. |
| `nordic-64` | 64 GB USB | Survival-focused. Wikipedia text + WikiHow + medical. |
| `nordic-128` | 128 GB USB | Full survival + education + maps. Phone + laptop. |
| `nordic-256` | 256 GB USB/SSD | + Full Wikipedia with images + all Stack Exchange sites. |
| `nordic-512` | 512 GB SSD | + Deep technical + full book collections. |
| `nordic-1tb` | 1 TB SSD | Civilization rebuild. LLMs, Linux ISO, everything. |
| `nordic-2tb` | 2 TB SSD | Everything + video content + additional languages. |

USB stick tiers (32-256) overlap with SSD tiers (256-2TB). The USB stick is your grab-and-go; the SSD is your base. If you only have the stick, you're covered. The SSD has richer versions of the same content (images, video, deeper sources).

## Drive Layout

```
PRIMER/
├── bin/                              # Static binaries (3 platforms each)
│   ├── macos-arm64/
│   │   ├── kiwix-serve               # ~20 MB — serves all ZIM content
│   │   ├── go-pmtiles                 # ~10 MB — map tile server
│   │   └── llama-server              # ~5 MB  — LLM with OpenAI-compatible API
│   ├── linux-x86_64/
│   │   └── ...
│   └── linux-arm64/
│       └── ...
├── zim/                              # Kiwix ZIM files
├── maps/                             # PMTiles (+ bundled viewer HTML/JS)
│   ├── *.pmtiles
│   ├── index.html                    # MapLibre GL + pmtiles.js viewer
│   ├── maplibre-gl.js                # ~800 KB, bundled for offline use
│   ├── maplibre-gl.css
│   └── pmtiles.js                    # ~30 KB
├── books/                            # PDFs, EPUBs
├── models/                           # GGUF files (optional)
├── apps/                             # Portable apps (CyberChef.html, etc.)
├── installers/                       # Optional: Kiwix.dmg, LMStudio.dmg (1tb+ only)
├── infra/                            # Optional: Linux ISO, apt cache (1tb+ only)
├── serve.sh                          # Self-contained bash menu
├── README.md
└── manifest.yaml                     # What's on this drive, versions, checksums
```

## serve.sh — Self-Contained Drive Menu

Pure bash, no dependencies. Auto-detects OS + arch, picks correct binaries from `bin/`. Only shows options that exist on the drive.

```
$ ./serve.sh

  Primer Drive — 1 TB Nordic Kit
  Detected: macOS arm64

  [1] Start knowledge base (kiwix-serve, 24 ZIM files)
      → http://localhost:8080
  [2] Start map viewer (go-pmtiles)
      → http://localhost:8081
  [3] Start LLM server (llama-server, OpenAI-compatible API)
      → http://localhost:8082/v1/chat/completions
  [4] Start all
  [5] Show drive contents
  [q] Quit
```

### Service binaries

| Binary | Size | Purpose | Format served | Included when |
|--------|------|---------|---------------|---------------|
| `kiwix-serve` | ~20 MB | All ZIM content (Wikipedia, WikiHow, Khan Academy videos, TED, Stack Exchange, books) | ZIM → browser | always |
| `go-pmtiles` | ~10 MB | Offline map viewer | PMTiles → browser via MapLibre GL | maps present |
| `llama-server` | ~5 MB | LLM inference, OpenAI-compatible API | GGUF → JSON API | models present |

Total binary overhead: ~35 MB per platform. All cross-platform (macOS-arm64, linux-x86_64, linux-arm64).

### Video playback

ZIM videos (TED, Khan Academy) are WebM/VP9 — plays natively in Chrome, Firefox, Safari 14.1+. No separate video player needed. kiwix-serve streams them inline in the browser.

### LLM approach

- **Drive ships GGUF files** — universal format, works with llama-server on any platform
- **llama-server provides OpenAI-compatible API** at `/v1/chat/completions` — scripts can hit it with curl
- **MLX models are NOT on the drive** — they require a Python runtime, are Mac-only, and the user's provisioned laptop (LM Studio) handles MLX natively
- GGUF via llama.cpp Metal backend is ~10-30% slower than MLX on M4 but works everywhere

## CLI Tool: `primer`

### Entry point behavior

- **First run (no manifest found):** wizard launches automatically
- **Subsequent runs:** quick status + interactive menu
- **`primer help`:** full CLI reference for scripting

### Subsequent run menu

```
$ primer

  Primer — /Volumes/PRIMER (128 GB, nordic)
  Last synced: 2026-03-15 | 24 sources | 110 GB used

  [s] Sync (check for updates)
  [a] Audit report
  [w] Wizard (reconfigure)
  [q] Quit
```

### CLI commands (for scripting / automation)

```
primer wizard                          # Interactive setup
primer init <path> --preset nordic-128 # Non-interactive init
primer sync                            # Download/update content
primer status                          # What's downloaded, what's stale
primer audit > audit.md                # LLM-ready gap analysis report
primer help                            # Full CLI reference
```

### Wizard flow

Sequential Rich-powered prompts (informative guide style, like `npm init`):

1. **Target** — auto-detect mounted volumes + show sizes/formats. Also offer "Custom directory..." for provisioning to a local folder. If pointed to a volume, default budget to 90% of free space.
2. **Budget** — "How much space?" with detected drive size as default. Tool auto-selects the best preset that fits.
3. **Region** — Nordic (default), US, Global. Only Nordic implemented in v1, others show "coming soon."
4. **Options** — toggle optional content: LLMs, app installers, Linux ISO, maps. Defaults based on tier.
5. **Review** — summary table with all sources, estimated download size, estimated time.
6. **Download** — progress bars per file, resume support, checksum verification.

The wizard collects answers and calls `primer init` + `primer sync`.

## Preset YAML Format

```yaml
name: Nordic 128GB
description: Grab-and-go survival kit, Nordic/Finnish focus
target_size_gb: 120
region: nordic

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

### Key fields

- **`url_pattern` with `{date}`** — primer scrapes the download page to find the latest date, enabling freshness checks and auto-updates.
- **`replaces`** — handles tier upgrades (e.g. `wikipedia-en-full` replaces `wikipedia-en-nopic` in 256+ tiers).
- **`optional_group`** — groups togglable in the wizard (maps, models, installers, infra).
- **`tags`** — list of taxonomy domains for coverage scoring.
- **`depth`** — `comprehensive`, `overview`, or `reference-only`.

## Taxonomy & Coverage

### Domains (~30)

| Group | Domains |
|-------|---------|
| **Survival** | `water`, `fire-shelter`, `food-foraging`, `navigation`, `first-aid`, `self-defense` |
| **Medical** | `medicine`, `dentistry`, `pharmacy`, `emergency-medicine`, `mental-health` |
| **Food** | `agriculture`, `gardening`, `animal-husbandry`, `food-preservation`, `cooking` |
| **Engineering** | `electronics`, `mechanical`, `civil-construction`, `metalworking`, `woodworking`, `energy-power` |
| **Tech** | `computing`, `radio-comms`, `networking-mesh`, `3d-printing`, `drones-robotics` |
| **Science** | `chemistry`, `physics`, `biology`, `earth-science`, `mathematics` |
| **Society** | `education-pedagogy`, `governance-law`, `trade-economics`, `history`, `language` |
| **Reference** | `general-reference`, `maps-geo`, `repair` |

### Coverage scoring

Each domain scored based on downloaded sources:

- `comprehensive` = 30 points per source
- `overview` = 15 points per source
- `reference-only` = 10 points per source
- Cap at 100. Thresholds: <30% = gap, 30-60% = weak, >60% = covered.

### Gap analysis

The taxonomy file defines **known sources** per domain (including sources not yet downloaded), so primer can suggest what's missing:

```yaml
domains:
  radio-comms:
    name: Radio & Communications
    known_sources:
      - id: stackexchange-amateur-radio
        essential: true
      - id: meshtastic-docs
        url: https://meshtastic.org/docs
        note: "Use zimit to archive"
        essential: true
```

## Audit Report

`primer audit` generates a markdown file designed to be pasted into any LLM for gap analysis:

```markdown
# Primer Audit Report
Generated: 2026-03-29
Preset: nordic-1tb
Drive: /Volumes/PRIMER (931GB total, 620GB used, 311GB free)

## System Prompt for AI Analysis

You are analyzing an offline knowledge kit designed for survival and
civilization rebuilding scenarios, with a Nordic/Finnish focus.
The kit must be usable with:
- MacBook (M4, macOS, 128GB RAM)
- iPhone/iPad (iOS, Kiwix reader)
- Android phone (Kiwix, OsmAnd)
- Any x86/ARM Linux machine

Analyze the inventory below and identify:
1. Critical knowledge gaps for survival (Nordic climate, -30°C winters)
2. Knowledge gaps for rebuilding (agriculture, manufacturing, governance)
3. Missing practical formats (theory but no step-by-step guides)
4. Accessibility gaps (content that can't be opened without specific software)
5. Redundancies worth eliminating to free space
6. Specific freely-available resources that would fill the top 10 gaps
7. Regional blind spots (Nordic flora, fauna, building codes, law)

## Inventory
[full table of all sources with id, file, size, tags, depth, date, format]

## Coverage Matrix
[domain scores with gap analysis]

## Format Accessibility Matrix
| Format | macOS | iOS | Android | Linux | Viewer on drive? |
|--------|-------|-----|---------|-------|-------------------|
| ZIM    | ✓     | ✓   | ✓       | ✓     | ✓ kiwix-serve     |
| PMTiles| ✓     | ✓   | ✓       | ✓     | ✓ go-pmtiles      |
| PDF    | ✓     | ✓   | ✓       | ✓     | ✗ OS built-in     |
| EPUB   | ✓     | ✓   | ✓       | ~     | ✗ OS built-in     |
| GGUF   | ✓     | ✗   | ✗       | ✓     | ✓ llama-server    |
| HTML   | ✓     | ✓   | ✓       | ✓     | ✓ native          |
| WebM   | ✓     | ✓   | ✓       | ✓     | ✓ via kiwix-serve |
```

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
| Maps | OSM Finland + Nordics (PMTiles) | 8 GB |
| Library | Project Gutenberg (text subset) | 10 GB |
| Apps | CyberChef.html | 0.1 GB |
| Binaries | kiwix-serve + go-pmtiles (3 platforms) | 0.1 GB |

## Content: nordic-256 (~215 GB)

Everything in nordic-128, plus:

| Change | Detail | Delta |
|--------|--------|-------|
| Wikipedia EN | Upgraded to full with images | +85 GB |
| Stack Exchange | All 20 sites (+ Electronics, Motor Vehicle, Woodworking, The Great Outdoors, Homebrewing, Arduino, Raspberry Pi, Sustainable Living, Chemistry, Medical Sciences, Biology, Earth Science, Engineering, 3D Printing, Physics) | +20 GB |

## Content: nordic-1tb (~620 GB)

Everything in nordic-256, plus:

| Category | Content | ~Size |
|----------|---------|-------|
| Reference | Wikipedia SV, NO, DA (no images) | 15 GB |
| Reference | Wikiversity, Wikivoyage | 3 GB |
| Medical | MedlinePlus, LibreTexts Medicine, NHS, CDC, LibrePathology, WikEM | 5 GB |
| Skills | Low-tech Magazine, Appropriate Technology Library | 6 GB |
| Education | Khan Academy full (with compressed video) | 50 GB |
| Education | LibreTexts (all subjects) | 12 GB |
| Education | OpenStax Textbooks | 3 GB |
| Education | TED-Ed (with video) | 6 GB |
| Education | freeCodeCamp, DevDocs | 0.2 GB |
| Library | Project Gutenberg (full) | 60 GB |
| Library | Survivor Library (1800s tech) | 100 GB |
| Maps | OSM full Europe (PMTiles) | 40 GB |
| Models | GGUF 8B general purpose (Q4_K_M) | 5 GB |
| Models | GGUF 8B medical/science (Q4_K_M) | 5 GB |
| Models | GGUF 70B general purpose (Q4) | 40 GB |
| Apps | CyberChef.html | 0.1 GB |
| Binaries | kiwix-serve + go-pmtiles + llama-server (3 platforms) | 0.1 GB |
| Installers | Kiwix.dmg, LMStudio.dmg, etc. (optional) | 5 GB |
| Infra | Bootable Linux ISO (Debian/Ubuntu LTS) | 5 GB |
| Infra | Offline apt package cache (gcc, python, build-essential) | 10 GB |

Remaining ~380 GB available for: additional LLMs, more languages, video content, 3D model libraries, Meshtastic/ESP32 firmware + docs, global maps.

## Tech Stack

- Python 3.11+
- Rich (CLI output, colors, prompts, progress bars, tables)
- PyYAML (config)
- aria2c for parallel/resumable downloads (falls back to wget/curl)
- No other runtime dependencies

## Brewfile (separate concern)

For provisioning the laptop while online:

```ruby
brew "kiwix-tools"
brew "aria2"
brew "python@3.11"
cask "kiwix"
cask "lm-studio"
```

## References & Inspiration

- [Project NOMAD](https://github.com/Crosstalk-Solutions/project-nomad) — Docker-based offline knowledge server (19k stars). US-centric, Debian-only. Good content curation reference but too heavy/inflexible for our use case.
- [Internet-in-a-Box](https://github.com/iiab/iiab) — 10+ years, field-tested in 100+ countries. Raspberry Pi focused.
- [Survivor Library](https://www.survivorlibrary.com/) — 1800s-era technical manuals for rebuilding without supply chains.
- [awesome-survival](https://github.com/alx-xlx/awesome-survival) — Curated list of survival resources.
- [Llamafile](https://github.com/mozilla-ai/llamafile) — Cross-platform single-binary LLM runtime.
- [Kiwix ZIM Library](https://library.kiwix.org/) — Canonical catalog of available ZIM files.
- [Protomaps](https://protomaps.com/) — PMTiles format for offline maps.
- [Zimit](https://github.com/openzim/zimit) — Turn any website into a ZIM file.

## Future Work (not in v1)

- Regional presets: `us-128`, `us-256`, `us-1tb`, `global-*`
- `max` preset — fill whatever drive you have
- Custom preset builder — pick domains and budget, primer selects optimal sources
- Zimit integration — archive custom websites into ZIM format
- Meshtastic/ESP32 firmware + documentation bundles
- 3D printing model libraries (Printables/Thingiverse dumps)
- Nordic-specific content: Finnish Red Cross guides, MPK material, local flora/fauna
