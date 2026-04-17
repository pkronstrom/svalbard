# Cybersecurity Pack Design

**Date:** 2026-04-16
**Status:** Draft

## Overview

Add portable penetration testing, CTF, and home network auditing tools to
Svalbard. Four composable packs provide Kali-like capabilities without requiring
the Kali distro — everything runs directly from the drive, fully offline.

## Pack Structure

| Pack | Purpose | ~Size |
|------|---------|-------|
| `cybersec` | Core recon, web, tunneling, vuln scanning | ~500 MB |
| `cybersec-ctf` | RE, exploit dev, password cracking, forensics | ~2-3 GB |
| `cybersec-reference` | Offline security docs (HackTricks, OWASP, etc.) | ~500 MB |
| `cybersec-wordlists` | SecLists, rockyou, nuclei-templates | ~1-2 GB |

Presets compose them freely:

```yaml
extends:
  - default-256
  - cybersec
  - cybersec-reference
  # add if space allows:
  # - cybersec-ctf
  # - cybersec-wordlists
```

## New Infrastructure: Python Runtime on Drive

Not cybersec-specific — benefits all future packs that need Python tools.

### Two New Recipe Types

**`python-venv`** — one per drive, declares the shared Python runtime:

```yaml
id: svalbard-python
type: python-venv
display_group: tools
tags: [computing]
size_gb: 0.03
python: ">=3.11"
description: Shared Python runtime for drive tools
license:
  id: PSF-2.0
  attribution: Python Software Foundation
```

**`python-package`** — per tool, installs into the shared venv:

```yaml
id: sqlmap
type: python-package
display_group: tools
tags: [cybersec]
size_gb: 0.02
venv: svalbard-python
packages:
  - sqlmap==1.8.4
entry_points:
  - sqlmap
description: Automatic SQL injection and database takeover tool
license:
  id: GPL-2.0
  attribution: Bernardo Damele and Miroslav Stampar
```

### uv as Build Tool

`uv` (Astral's Rust-based Python package manager) is bundled as a standard
`type: binary` recipe. It handles Python installation, venv creation, and
package installation at sync time.

```yaml
id: uv
type: binary
display_group: tools
tags: [computing]
size_gb: 0.03
platforms:
  linux-x86_64: https://github.com/astral-sh/uv/releases/...
  linux-arm64: https://github.com/astral-sh/uv/releases/...
  macos-arm64: https://github.com/astral-sh/uv/releases/...
  macos-x86_64: https://github.com/astral-sh/uv/releases/...
description: Fast Python package manager for building drive venvs
license:
  id: Apache-2.0
  attribution: Astral
```

### Build Flow at `svalbard sync`

1. Download `uv` binary (standard `type: binary` handling)
2. For each `python-venv` recipe: run `uv python install <version>` into
   `runtime/python/<platform>/`
3. Create the venv: `uv venv runtime/python/<platform>/`
4. Collect all `python-package` recipes referencing this venv from the resolved
   preset
5. Merge their `packages:` lists, run `uv pip install <all packages>` into the
   venv
6. For each declared `entry_points:`, generate a wrapper script in
   `bin/<platform>/`

### Entry Point Wrapper Scripts

Generated in `bin/<platform>/` so Python tools are discoverable alongside native
binaries. The Go runtime does not need to know the difference.

```sh
#!/bin/sh
DRIVE="$(cd "$(dirname "$0")/../.." && pwd)"
exec "$DRIVE/runtime/python/macos-arm64/bin/sqlmap" "$@"
```

## Drive Filesystem Layout

```
drive/
  bin/
    macos-arm64/
      nmap                    # static binary
      rustscan                # Rust binary
      nuclei                  # Go binary
      gobuster                # Go binary
      ffuf                    # Go binary
      httpx                   # Go binary
      chisel                  # Go binary
      sqlmap                  # wrapper -> runtime/python/.../bin/sqlmap
      vol                     # wrapper -> runtime/python/.../bin/vol
      uv                      # for on-drive maintenance
      ...
    linux-x86_64/
      ...
  runtime/
    python/
      macos-arm64/
        bin/                  # python, sqlmap, vol, pwntools entry points
        lib/                  # site-packages
      linux-x86_64/
        ...
  apps/
    ghidra/                   # extracted ZIP + portable JDK
  data/
    seclists/                 # wordlist directory tree
    nuclei-templates/         # YAML vulnerability templates
    rockyou.txt
  zim/
    hacktricks.zim
    gtfobins.zim
    owasp-wstg.zim
    payloads-all-the-things.zim
```

The `runtime/` directory is designed for future language runtimes (Rust, Go, C++)
following the same `runtime/<language>/<platform>/` pattern.

## Pack Contents

### `cybersec` — Core (~500 MB)

| Recipe | Type | Size | What |
|--------|------|------|------|
| nmap-static | binary | ~6 MB | Network scanner (static Linux only — no macOS static builds exist) |
| rustscan | binary | ~5 MB | Fast port scanner |
| naabu | binary | ~15 MB | Port scanner (ProjectDiscovery) |
| nuclei | binary | ~50 MB | Template-based vuln scanner |
| gobuster | binary | ~10 MB | Dir/DNS brute-forcer |
| feroxbuster | binary | ~15 MB | Recursive content discovery |
| ffuf | binary | ~8 MB | Web fuzzer |
| httpx | binary | ~25 MB | HTTP probe toolkit |
| subfinder | binary | ~20 MB | Subdomain enumeration |
| katana | binary | ~20 MB | Web crawler |
| chisel | binary | ~10 MB | TCP/UDP tunnel over HTTP (bare .gz — needs gunzip support) |
| ligolo-ng-proxy | binary | ~15 MB | Network pivoting proxy (run on attacker) |
| ligolo-ng-agent | binary | ~15 MB | Network pivoting agent (deploy on target) |
| caido | binary | ~80 MB | Web proxy (Burp alternative) |
| linpeas | binary | ~5 MB | Linux priv esc enumeration |
| winpeas | binary | ~5 MB | Windows priv esc enumeration |
| sqlmap | python-package | ~15 MB | SQL injection automation |

### `cybersec-ctf` — CTF Additions (~2-3 GB)

| Recipe | Type | Size | What |
|--------|------|------|------|
| ghidra | app | ~700 MB | RE framework (NSA), requires JDK 17+ on host |
| radare2 | binary | ~10 MB | RE framework (CLI), built from source via custom builder |
| hashcat | app | ~300 MB | GPU password cracking + kernels (.7z — needs custom extractor) |
| pwntools | python-package | ~50 MB | CTF exploit dev library |
| angr | python-package | ~200 MB | Binary analysis + symbolic execution |
| volatility3 | python-package | ~30 MB | Memory forensics |
| binwalk | python-package | ~10 MB | Firmware analysis |

### `cybersec-reference` — Offline Docs (~500 MB)

| Recipe | Type | Size | What |
|--------|------|------|------|
| hacktricks | zim | ~200 MB | Pentest knowledge base |
| gtfobins | zim | ~5 MB | Unix binary exploitation ref |
| lolbas | zim | ~5 MB | Windows living-off-the-land |
| payloads-all-the-things | zim | ~50 MB | Web attack payload reference |
| owasp-wstg | zim | ~10 MB | Web security testing guide |
| owasp-cheat-sheets | zim | ~5 MB | OWASP cheat sheet series |

Note: OWASP content was already planned as TODOs in `computing.yaml`. Those
recipes serve both packs.

### `cybersec-wordlists` — Datasets (~1-2 GB)

| Recipe | Type | Size | What |
|--------|------|------|------|
| seclists | dataset | ~500 MB | Curated fuzzing/password lists |
| rockyou | dataset | ~133 MB | Classic password wordlist |
| nuclei-templates | dataset | ~200 MB | Nuclei vuln templates |

## Implementation Scope

### Part 1 — Infrastructure (python-venv pipeline)

- Add `python-venv` and `python-package` to `Source.type` handling
- Add `uv` binary recipe
- Add `svalbard-python` venv recipe to `tools-base`
- New builder that runs uv to create venvs and install packages at sync time
- Wrapper script generation for `bin/<platform>/`
- Add `runtime/python` to `TYPE_DIRS` mapping
- Add `dataset` to `TYPE_DIRS` (maps to `data/`)
- Add bare `.gz` decompression to runtime binary resolver
- Update `tools-base.yaml` to include `uv` and `svalbard-python`
- Custom builder scripts: `radare2-static.py`, `extract-7z.py`, `zimit-scrape.py`,
  `github-wiki-zim.py`

### Part 2 — Recipes and packs (cybersec content)

- ~25 individual recipe YAML files across `recipes/tools/`, `recipes/apps/`,
  `recipes/datasets/`, `recipes/content/`
- 4 pack YAML files in `presets/packs/`
- ZIM build configs for reference content (HackTricks, GTFOBins, etc.)

Parts are independent: Part 2 YAML can be authored now and will be valid. Sync
just won't handle `python-package` type until Part 1 lands.
