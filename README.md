# Svalbard

Seed vault for human knowledge — civilization on a stick.

Assemble offline knowledge drives — Wikipedia, maps, books, and AI models on a single USB stick.

## Install

```bash
# With uv (recommended)
uv run svalbard --help          # Run directly, no install needed
uv run svb --help               # Short alias
uv sync                        # Or install into a venv first

# With pip
pip install -e .
```

## Quick Start

```bash
svalbard wizard
```

The wizard walks you through choosing a region, selecting a preset tier, and initializing a drive.

## Presets

Default presets are region-neutral and English-first.

| Preset | Wikipedia | Reference | Practical | Education | Tools |
| ------ | --------- | --------- | --------- | --------- | ----- |
| `default-32` | English Wikipedia | WikiMed | Practical Action, Outdoors & Survival Stack Exchange | - | CyberChef, Kiwix tools |
| `default-64` | English Wikipedia | English Wiktionary | WikiMed, iFixit, Outdoors & Survival Stack Exchange, DIY Stack Exchange, Practical Action | Wikibooks | CyberChef, Kiwix tools |
| `default-128` | English Wikipedia | English Wiktionary, Project Gutenberg | WikiMed, WikiHow, iFixit, Outdoors & Survival, DIY, Gardening, Cooking Stack Exchange, Practical Action | Wikibooks, Khan Academy | CyberChef, Kiwix tools |
| `default-256` | English Wikipedia with pictures | English Wiktionary, Project Gutenberg | WikiHow, iFixit, Outdoors & Survival, DIY, Gardening, Cooking, Amateur Radio, Electronics, Physics Stack Exchange, Practical Action | Wikibooks, Khan Academy | CyberChef, Kiwix tools |
| `default-512` | English Wikipedia with pictures | English Wiktionary, Project Gutenberg | `default-256` plus Math Stack Exchange | Wikibooks, Khan Academy | CyberChef, Kiwix tools, Qwen3.5 9B |
| `default-1tb` | English Wikipedia with pictures | English Wiktionary, Project Gutenberg | `default-512` plus Chemistry, Biology, Engineering Stack Exchange | Wikibooks, Khan Academy | CyberChef, Kiwix tools, Qwen3.5 35B-A3B |
| `default-2tb` | Full English Wikipedia | English Wiktionary, Project Gutenberg | `default-1tb` plus Server Fault and Super User | Wikibooks, Khan Academy | CyberChef, Kiwix tools, Qwen3.5 35B-A3B + Qwen3.5 9B |

LLM downloads begin at the `512 GB` tiers in both preset families. Bundled models are capped to portable quants that fit roughly within a `24 GB` RAM budget; anything larger should become an explicit optional add-on later.

Finland presets add Finnish-language and Finland-focused content on top of that baseline.

| Preset       | Size   | Focus                                               |
| ------------ | ------ | --------------------------------------------------- |
| `finland-128`  | 128 GB | Finnish + English reference and practical guides    |
| `finland-1tb`  | 1 TB   | Full Finnish-first archive with larger models/tools |

For quick manual end-to-end checks, `test-1gb` is a visible smoke-test preset with a tiny Wikipedia ZIM, a compact local model, a small Uusimaa basemap, and a few lightweight support sources. It is meant for validation, not as a recommended archival tier.

## Source Catalog

These are the recurring data sources and tools that appear across the preset bundles.

| Source | What it is | Why it is included |
| ------ | ---------- | ------------------ |
| English Wikipedia (`wikipedia-en`, `wikipedia-en-maxi`, `wikipedia-en-all`) | Offline encyclopedia in compact, pictured, or full variants | Broadest general reference base for science, medicine, engineering, history, and basic lookup |
| Finnish Wikipedia (`wikipedia-fi`, `wikipedia-fi-all`) | Finnish-language encyclopedia | Local-language reference for Finland-focused presets |
| Swedish / Norwegian / Danish / German / Russian / Estonian Wikipedia variants | Additional language encyclopedias in larger Finland tiers | Regional and multilingual context where the Finland family expands beyond English/Finnish |
| English Wiktionary (`wiktionary-en`) | Offline dictionary and word reference | Language lookup, definitions, and translation support |
| Finnish Wiktionary (`wiktionary-fi`) | Finnish dictionary and word reference | Finnish-language support in Finland presets |
| WikiMed (`wikimed`) | Medical encyclopedia | High-value offline medical reference for first aid, medicine, and emergency use |
| WikiHow (`wikihow-en`) | Practical step-by-step guides | Procedure-oriented survival and repair knowledge, not just reference articles |
| iFixit (`ifixit`) | Repair manuals and teardown guides | Practical repair coverage for devices, tools, and household equipment |
| Practical Action (`practical-action`) | Appropriate technology and field guides | Low-resource engineering, water, agriculture, and infrastructure knowledge |
| Stack Exchange bundles | Topical Q&A archives such as survival, DIY, gardening, cooking, radio, electronics, physics, math, chemistry, biology, engineering, Server Fault, and Super User | Practical troubleshooting and niche technical knowledge that complements encyclopedia content |
| Wikibooks (`wikibooks-en`) | Open textbooks | Structured learning material for science, math, computing, and technical topics |
| Khan Academy (`khan-academy`, `khan-academy-lite`) | Educational course material | Offline teaching and self-study for core subjects |
| Project Gutenberg (`gutenberg`, `gutenberg-subset`) | Public-domain book collection | Long-form reference, historical texts, and deeper background reading |
| CyberChef (`cyberchef`) | Browser-based data transformation tool | Handy offline utility for encoding, decoding, text, and binary manipulation |
| Kiwix tools (`kiwix-serve`) | Local server/runtime for ZIM archives | Makes the content actually browsable from the drive on supported desktops |
| Qwen3.5 9B (`qwen-9b`) | Portable local GGUF model | First bundled offline LLM tier with a better current quality/size tradeoff than the old tiny defaults |
| Qwen3.5 35B-A3B (`qwen-35b-a3b`) | Strong local GGUF model in a sub-24 GB quant | Higher-capability offline LLM that still fits sane laptop memory limits |
| `llama-server-binaries` | Runtime for serving GGUF models locally | Lets the GGUF models be used directly from the drive on supported systems |

## Commands

| Command                                | Description                          |
| -------------------------------------- | ------------------------------------ |
| `svalbard wizard`                      | Interactive setup                    |
| `svalbard init <path> --preset <name>` | Initialize a drive with a preset     |
| `svalbard sync <path>`                 | Download/update content              |
| `svalbard status <path>`               | Show drive contents and sync status  |
| `svalbard audit <path>`                | Coverage report against taxonomy     |
| `svalbard add <path-or-url>`           | Add a local artifact, website, or media URL as a reusable source |
| `svalbard add <url> --quality 480p`    | Import remote media or websites with explicit ingest settings |
| `svalbard add <url> --audio-only`      | Create an audio-first offline archive from a media source |

## Design

See [docs/plans/](docs/plans/) for the design document.
