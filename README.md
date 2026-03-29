# Primer

Assemble offline knowledge drives — Wikipedia, maps, books, and AI models on a single USB stick.

## Install

```bash
pip install -e .
```

## Quick Start

```bash
primer wizard
```

The wizard walks you through choosing a preset, selecting content, and initializing a drive.

## Presets

| Preset       | Size   | Focus                                       |
| ------------ | ------ | ------------------------------------------- |
| nordic-128   | 128 GB | Finnish + English reference, Nordic maps     |
| nordic-256   | 256 GB | Adds pictures, more languages, local models  |
| nordic-1tb   | 1 TB   | Full coverage — video, full maps, large LLMs |

## Commands

| Command                              | Description                          |
| ------------------------------------ | ------------------------------------ |
| `primer wizard`                      | Interactive setup                    |
| `primer init <path> --preset <name>` | Initialize a drive with a preset     |
| `primer sync <path>`                 | Download/update content              |
| `primer status <path>`               | Show drive contents and sync status  |
| `primer audit <path>`                | Coverage report against taxonomy     |

## Design

See [docs/plans/](docs/plans/) for the design document.
