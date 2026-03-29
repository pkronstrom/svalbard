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

Finland presets add Finnish-language and Finland-focused content on top of that baseline.

| Preset       | Size   | Focus                                               |
| ------------ | ------ | --------------------------------------------------- |
| `finland-128`  | 128 GB | Finnish + English reference and practical guides    |
| `finland-1tb`  | 1 TB   | Full Finnish-first archive with larger models/tools |

## Commands

| Command                                | Description                          |
| -------------------------------------- | ------------------------------------ |
| `svalbard wizard`                      | Interactive setup                    |
| `svalbard init <path> --preset <name>` | Initialize a drive with a preset     |
| `svalbard sync <path>`                 | Download/update content              |
| `svalbard status <path>`               | Show drive contents and sync status  |
| `svalbard audit <path>`                | Coverage report against taxonomy     |

## Design

See [docs/plans/](docs/plans/) for the design document.
