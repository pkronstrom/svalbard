# Go Native Builders Design

**Date:** 2026-04-17
**Status:** Draft

## Overview

Add a Go-native builder system to the host-cli, alongside the existing
Docker-based builders. Native builders handle simple, well-defined tasks
(archive extraction, Python venv provisioning) as pure Go code. Complex
pipelines (geodata, ZIM scraping) stay Docker.

## Builder Package

New package: `host-cli/internal/builder/`

```
builder/
  builder.go       # Registry type, Native map, Dispatch()
  appbundle.go     # app-bundle: download + extract zip/tar to apps/<id>/
  pythonvenv.go    # python-venv: uv cache + per-tool venvs + wrappers
```

### Registry

```go
type Func func(root string, recipe catalog.Item, cat *catalog.Catalog) (manifest.RealizedEntry, error)

var Native = map[string]Func{
    "app-bundle":  buildAppBundle,
    "python-venv": buildPythonVenv,
}

func Dispatch(family string) (Func, bool) {
    fn, ok := Native[family]
    return fn, ok
}
```

### Apply Dispatch (apply.go)

Build route changes from:

```
strategy == "build" → Docker buildItem()
```

To:

```
strategy == "build" → try builder.Dispatch(family)
                      → if found: Go-native
                      → if not found: Docker buildItem()
python-package type → skip (consumed by python-venv builder)
python-venv type    → builder.Dispatch("python-venv")
```

## Catalog Model Extensions

### New Item Fields

```go
type Item struct {
    // ... existing fields ...
    Python      string            `yaml:"python,omitempty"`
    Venv        string            `yaml:"venv,omitempty"`
    Packages    []string          `yaml:"packages,omitempty"`
    EntryPoints []string          `yaml:"entry_points,omitempty"`
    Env         map[string]string `yaml:"env,omitempty"`
}
```

### New TypeDirs

```go
var TypeDirs = map[string]string{
    // ... existing ...
    "dataset":        "data",
    "python-venv":    "runtime/python",
    "python-package": "runtime/python",
}
```

## Python Venv Builder

### Drive Layout

```
runtime/python/
  cache/                              # shared wheel cache, platform-independent
    sqlmap-1.8.4-py3-none-any.whl
    pwntools-4.13.0-cp311-...whl
    requests-2.31-py3-none-any.whl    # shared dep, stored once
  macos-arm64/
    .python/                          # standalone Python interpreter (uv-managed)
    tools/
      sqlmap/bin/sqlmap               # isolated venv
      pwntools/bin/pwn               # isolated venv
      angr/bin/angr                  # isolated venv
  linux-x86_64/
    .python/
    tools/
      sqlmap/bin/sqlmap
```

### Platform Awareness

The builder reads `manifest.Desired.Options.HostPlatforms` to know which
platforms to provision. If the manifest says `[macos-arm64, linux-x86_64]`,
the builder creates venvs for both — same as binary recipes download all
specified platform variants.

### Build Flow

1. **Find uv.** Check `bin/<host_platform>/uv` on drive, then `PATH`. Fail if missing.

2. **Collect packages.** Gather all `python-package` recipes from catalog where
   `item.Venv == recipe.ID`. Merge all `Packages` lists.

3. **Download wheels to cache (per platform).** For each target platform:
   `uv pip download --dest <root>/runtime/python/cache/ --python-version 3.11 --platform <uv_platform> <all_packages...>`.
   The cache is shared across platforms — uv tags wheels by platform
   automatically. Platform-independent wheels (pure Python) are stored once.

4. **Install Python (per platform).** For each target platform:
   `uv python install --install-dir <root>/runtime/python/<platform>/.python ">=3.11"`.
   Skip if already present (idempotent).

5. **Create per-tool venvs (per platform).** For each target platform, for each
   python-package recipe:
   - `uv venv <root>/runtime/python/<platform>/tools/<id>/`
   - `uv pip install --python .../tools/<id>/bin/python3 --no-index --find-links <root>/runtime/python/cache/ <packages...>`

6. **Generate wrappers (per platform).** For each target platform, for each
   entry point, write shell script to `bin/<platform>/<entry_point>`:
   ```sh
   #!/bin/sh
   DRIVE="$(cd "$(dirname "$0")/../.." && pwd)"
   exec "$DRIVE/runtime/python/<platform>/tools/<id>/bin/<entry_point>" "$@"
   ```

### Why Per-Tool Venvs

Shared venv risks dependency conflicts (e.g. angr's z3-solver pins vs pwntools).
Per-tool venvs via the shared wheel cache give isolation with deduplication.
Overhead is ~100-150 MB over a shared venv — negligible on a GB-scale drive.

## App-Bundle Builder

Downloads `source_url` or individual `assets`, extracts archive to
`apps/<id>/`. Handles .zip and .tar.gz. Replaces the Python app-bundle builder.

## Activate Script: Recipe Env Vars

Recipes can declare env vars via an `env` field (values are drive-relative paths):

```yaml
# recipes/datasets/seclists.yaml
env:
  SECLISTS: data/seclists

# recipes/apps/ghidra.yaml
env:
  GHIDRA_INSTALL_DIR: apps/ghidra
```

The toolkit generator collects all `env` maps from realized recipes and writes
them into the activate script:

```sh
DRIVE="$(cd "$(dirname "$0")" && pwd)"
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64)  PLATFORM=macos-arm64 ;;
  Darwin-x86_64) PLATFORM=macos-x86_64 ;;
  Linux-aarch64) PLATFORM=linux-arm64 ;;
  Linux-x86_64)  PLATFORM=linux-x86_64 ;;
esac

export PATH="$DRIVE/bin/$PLATFORM:$PATH"
export SVALBARD_DATA="$DRIVE/data"
export GHIDRA_INSTALL_DIR="$DRIVE/apps/ghidra"
export SECLISTS="$DRIVE/data/seclists"
```

All tools — native binaries, Python wrappers, PlatformIO, AI tools — available
in one shell via `source activate`.

## Go Runtime: Bare .gz Decompression

Add `extractBareGz()` to `drive-runtime/internal/binary/runtimebinary.go`.
Handles chisel-style releases where the archive is a single gzipped binary
(not a tar.gz). Gunzips to the same directory, sets executable permission.

## Not In Scope

- Docker builders (unchanged)
- Python-side builder.py (legacy on main branch)
- Custom builder scripts (zimit-scrape.py, radare2-static.py — future Docker builders)
