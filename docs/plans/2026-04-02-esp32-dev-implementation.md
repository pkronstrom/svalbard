# ESP32 Dev Pack Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Provision a Svalbard USB stick with ESP32 PlatformIO toolchains, frameworks, and libraries so that `pio run` / `pio run -t upload` / `pio device monitor` work fully offline.

**Architecture:** Recipe YAMLs for each toolchain component, downloaded via existing `type: binary` infrastructure. A new `pio-setup.sh` drive action assembles the correct PlatformIO directory layout on first use by extracting archives to a host-side cache (fast I/O, structure-preserving). The stick stores compressed archives; the host gets a working PlatformIO environment.

**Tech Stack:** Python (svalbard CLI), Bash (drive-side scripts), PlatformIO registry API

---

## Drive layout

```
$DRIVE/
  bin/{platform}/
    toolchain-xtensa-esp-elf-*.tar.gz     ← downloaded by existing infra
    toolchain-esp32ulp-*.tar.gz
    tool-openocd-esp32-*.tar.gz
  tools/
    platformio/
      shared/
        framework-espidf-*.tar.gz          ← cross-platform archives
        tool-esptoolpy-*.tar.gz
      lib/                                  ← pre-cached libraries
        FastLED/
        Adafruit_BME280/
        ...
      examples/                             ← starter projects
        esp32-wifi-ap/
        esp32-blink/
  .svalbard/
    actions/
      pio-setup.sh                          ← environment setup script
```

On first use, `pio-setup.sh` extracts platform-specific archives to a host cache:

```
/tmp/svalbard-pio/                          ← on host filesystem (fast)
  packages/
    toolchain-xtensa-esp-elf/              ← extracted, structure preserved
    toolchain-esp32ulp/
    tool-openocd-esp32/
    framework-espidf/                       ← extracted from shared/
    tool-esptoolpy/
  lib/                                      ← symlinked to stick
```

`PLATFORMIO_CORE_DIR=/tmp/svalbard-pio` makes everything transparent to `pio`.

---

### Task 1: Create ESP32 toolchain recipe YAMLs

**Files:**
- Create: `recipes/tools/toolchain-xtensa-esp-elf.yaml`
- Create: `recipes/tools/framework-espidf.yaml`
- Create: `recipes/tools/tool-esptoolpy.yaml`
- Create: `recipes/tools/toolchain-esp32ulp.yaml`
- Create: `recipes/tools/tool-openocd-esp32.yaml`

**Step 1: Write toolchain-xtensa-esp-elf recipe**

```yaml
# recipes/tools/toolchain-xtensa-esp-elf.yaml
id: toolchain-xtensa-esp-elf
type: binary
group: tools
tags:
- embedded
- esp32
depth: reference-only
size_gb: 0.32
platforms:
  linux-x86_64: https://dl.registry.platformio.org/download/platformio/tool/toolchain-xtensa-esp-elf/14.2.0+20251107/toolchain-xtensa-esp-elf-linux_x86_64-14.2.0+20251107.tar.gz
  linux-arm64: https://dl.registry.platformio.org/download/platformio/tool/toolchain-xtensa-esp-elf/14.2.0+20251107/toolchain-xtensa-esp-elf-linux_aarch64-14.2.0+20251107.tar.gz
  macos-arm64: https://dl.registry.platformio.org/download/platformio/tool/toolchain-xtensa-esp-elf/14.2.0+20251107/toolchain-xtensa-esp-elf-darwin_arm64-14.2.0+20251107.tar.gz
description: Xtensa GCC cross-compiler for ESP32, ESP32-S2, ESP32-S3
license:
  id: GPL-3.0-or-later
  attribution: Espressif Systems
```

**Step 2: Write framework-espidf recipe**

The ESP-IDF framework is cross-platform (source code). It needs a new field
to signal "don't put this in bin/{platform}/, put it in tools/platformio/shared/".
For now, use `type: binary` without `platforms:` and a direct `url:`.

```yaml
# recipes/tools/framework-espidf.yaml
id: framework-espidf
type: binary
group: tools
tags:
- embedded
- esp32
depth: reference-only
size_gb: 0.08
url: https://dl.registry.platformio.org/download/platformio/tool/framework-espidf/3.50503.0/framework-espidf-3.50503.0.tar.gz
sha256: 8353f6fd5030dd7e662500891428fad15d46efd7e4b718cab2fe6bfb9e7f13fc
description: Espressif IoT Development Framework (ESP-IDF v5.x)
license:
  id: Apache-2.0
  attribution: Espressif Systems
```

**Step 3: Write tool-esptoolpy recipe**

```yaml
# recipes/tools/tool-esptoolpy.yaml
id: tool-esptoolpy
type: binary
group: tools
tags:
- embedded
- esp32
depth: reference-only
size_gb: 0.001
url: https://dl.registry.platformio.org/download/platformio/tool/tool-esptoolpy/2.41100.0/tool-esptoolpy-2.41100.0.tar.gz
sha256: ec80a347efa02e64706075575fbe07b32d3140f4c4aaa7002350ea7d9f71b826
description: ESP32 firmware flashing and serial tool
license:
  id: GPL-2.0-or-later
  attribution: Espressif Systems
```

**Step 4: Write toolchain-esp32ulp recipe**

```yaml
# recipes/tools/toolchain-esp32ulp.yaml
id: toolchain-esp32ulp
type: binary
group: tools
tags:
- embedded
- esp32
depth: reference-only
size_gb: 0.016
platforms:
  linux-x86_64: https://dl.registry.platformio.org/download/platformio/tool/toolchain-esp32ulp/1.23800.240113/toolchain-esp32ulp-linux_x86_64-1.23800.240113.tar.gz
  linux-arm64: https://dl.registry.platformio.org/download/platformio/tool/toolchain-esp32ulp/1.23800.240113/toolchain-esp32ulp-linux_aarch64-1.23800.240113.tar.gz
  macos-arm64: https://dl.registry.platformio.org/download/platformio/tool/toolchain-esp32ulp/1.23800.240113/toolchain-esp32ulp-darwin_arm64-1.23800.240113.tar.gz
description: ESP32 Ultra-Low-Power (ULP) coprocessor toolchain
license:
  id: GPL-3.0-or-later
  attribution: Espressif Systems
```

**Step 5: Write tool-openocd-esp32 recipe**

```yaml
# recipes/tools/tool-openocd-esp32.yaml
id: tool-openocd-esp32
type: binary
group: tools
tags:
- embedded
- esp32
depth: reference-only
size_gb: 0.003
platforms:
  linux-x86_64: https://dl.registry.platformio.org/download/platformio/tool/tool-openocd-esp32/2.1200.20230419/tool-openocd-esp32-linux_x86_64-2.1200.20230419.tar.gz
  linux-arm64: https://dl.registry.platformio.org/download/platformio/tool/tool-openocd-esp32/2.1200.20230419/tool-openocd-esp32-linux_aarch64-2.1200.20230419.tar.gz
  macos-arm64: https://dl.registry.platformio.org/download/platformio/tool/tool-openocd-esp32/2.1200.20230419/tool-openocd-esp32-darwin_arm64-2.1200.20230419.tar.gz
description: OpenOCD debug server for ESP32 (SWD/JTAG)
license:
  id: GPL-2.0-or-later
  attribution: Espressif Systems
```

**Step 6: Commit**

```bash
git add recipes/tools/toolchain-xtensa-esp-elf.yaml \
        recipes/tools/framework-espidf.yaml \
        recipes/tools/tool-esptoolpy.yaml \
        recipes/tools/toolchain-esp32ulp.yaml \
        recipes/tools/tool-openocd-esp32.yaml
git commit -m "feat(recipes): add ESP32 PlatformIO toolchain recipes"
```

---

### Task 2: Create ESP32 dev pack YAML

**Files:**
- Create: `presets/packs/embedded/esp32-dev.yaml`
- Verify: `presets/packs/` directory structure exists

**Step 1: Create packs/embedded directory**

```bash
mkdir -p presets/packs/embedded
```

**Step 2: Write esp32-dev pack**

```yaml
# presets/packs/embedded/esp32-dev.yaml
name: esp32-dev
kind: pack
description: ESP32 offline development — Xtensa toolchain + ESP-IDF framework
target_size_gb: 2
sources:
- toolchain-xtensa-esp-elf
- framework-espidf
- tool-esptoolpy
- toolchain-esp32ulp
- tool-openocd-esp32
```

**Step 3: Update preset path resolution for nested packs**

The current `resolve_preset_path` in `presets.py:50` checks:
1. `local/presets/{name}.yaml`
2. `presets/{name}.yaml`
3. `presets/packs/{name}.yaml`

It does NOT support `presets/packs/embedded/esp32-dev.yaml` — nested pack
directories. We need to add support for this.

**Test:** `src/tests/test_presets.py`

```python
def test_resolve_nested_pack(tmp_path):
    """Nested packs like packs/embedded/esp32-dev resolve correctly."""
    packs_dir = tmp_path / "presets" / "packs" / "embedded"
    packs_dir.mkdir(parents=True)
    pack_file = packs_dir / "esp32-dev.yaml"
    pack_file.write_text("name: esp32-dev\nkind: pack\nsources: []\n")

    # Patch PRESETS_DIR to use tmp_path
    from svalbard import presets
    original = presets.PRESETS_DIR
    presets.PRESETS_DIR = tmp_path / "presets"
    try:
        path = presets.resolve_preset_path("embedded/esp32-dev")
        assert path == pack_file
    finally:
        presets.PRESETS_DIR = original
```

Run: `uv run pytest src/tests/test_presets.py::test_resolve_nested_pack -v`
Expected: FAIL — nested path not supported yet.

**Step 4: Add nested pack resolution**

Modify `resolve_preset_path` in `src/svalbard/presets.py`:

```python
def resolve_preset_path(name: str, workspace: Path | str | None = None) -> Path:
    """Resolve a preset path: local → built-in → packs (including nested)."""
    local_path = local_presets_dir(workspace) / f"{name}.yaml"
    builtin_path = PRESETS_DIR / f"{name}.yaml"
    packs_path = PRESETS_DIR / "packs" / f"{name}.yaml"
    if local_path.exists() and (builtin_path.exists() or packs_path.exists()):
        raise ValueError(f"Local preset '{name}' collides with built-in preset")
    if local_path.exists():
        return local_path
    if builtin_path.exists():
        return builtin_path
    if packs_path.exists():
        return packs_path
    raise FileNotFoundError(f"Preset not found: {name}")
```

The key insight: `name` can be `"embedded/esp32-dev"`, so
`PRESETS_DIR / "packs" / "embedded/esp32-dev.yaml"` already resolves correctly
via Path's `/` operator. **No code change needed** — the existing code handles
it because `Path("packs") / "embedded/esp32-dev.yaml"` creates
`packs/embedded/esp32-dev.yaml`.

Run: `uv run pytest src/tests/test_presets.py::test_resolve_nested_pack -v`
Expected: PASS (verify this — if it fails, add explicit nested dir scanning).

**Step 5: Also update list_presets to find nested packs**

Current `list_presets` uses `packs_dir.glob("*.yaml")` which doesn't recurse.
Change to `rglob`:

In `src/svalbard/presets.py`, function `list_presets`:

```python
# Before:
names.update(p.stem for p in packs_dir.glob("*.yaml"))

# After:
for p in packs_dir.rglob("*.yaml"):
    rel = p.relative_to(packs_dir).with_suffix("")
    names.add(str(rel))
```

This makes `list_presets()` return `"embedded/esp32-dev"` instead of just
`"esp32-dev"`, avoiding collisions with future top-level packs of the same name.

**Step 6: Run full test suite**

```bash
uv run pytest src/tests/test_presets.py -v
```

**Step 7: Commit**

```bash
git add presets/packs/embedded/esp32-dev.yaml src/svalbard/presets.py src/tests/test_presets.py
git commit -m "feat(presets): add esp32-dev pack with nested pack resolution"
```

---

### Task 3: Handle cross-platform (non-platformized) binary downloads

**Problem:** `framework-espidf` and `tool-esptoolpy` have no `platforms:` map —
they're single-URL cross-platform packages. The existing `expand_source_downloads`
in `commands.py:196` puts them in `TYPE_DIRS["binary"]` = `"bin"` root, which is
wrong for PlatformIO toolchain components.

These should go to `tools/platformio/shared/` on the drive.

**Files:**
- Modify: `src/svalbard/commands.py` (add `toolchain` to TYPE_DIRS)
- Modify: `src/svalbard/toolkit_generator.py` (add `toolchain` to TYPE_DIRS)
- Modify: `recipes/tools/framework-espidf.yaml` (change type to `toolchain`)
- Modify: `recipes/tools/tool-esptoolpy.yaml` (change type to `toolchain`)
- Modify: `recipes/tools/toolchain-xtensa-esp-elf.yaml` (change type)
- Modify: `recipes/tools/toolchain-esp32ulp.yaml` (change type)
- Modify: `recipes/tools/tool-openocd-esp32.yaml` (change type)
- Test: `src/tests/test_commands.py`

**Step 1: Add `toolchain` type to TYPE_DIRS**

In `src/svalbard/commands.py`, add to TYPE_DIRS:

```python
TYPE_DIRS = {
    "zim": "zim",
    "pmtiles": "maps",
    "pdf": "books",
    "epub": "books",
    "gguf": "models",
    "binary": "bin",
    "toolchain": "tools/platformio/packages",
    "app": "apps",
    "iso": "infra",
    "sqlite": "data",
}
```

Same change in `src/svalbard/toolkit_generator.py`.

**Step 2: Update all ESP32 recipes to use `type: toolchain`**

Change `type: binary` → `type: toolchain` in all five recipe files.

**Step 3: Update expand_source_downloads for toolchain type**

In `commands.py`, the `expand_source_downloads` function puts platform-specific
sources in `bin/{platform}/`. For `toolchain` type, we want
`tools/platformio/packages/{platform}/`:

```python
def expand_source_downloads(source: Source, drive_path: Path) -> list[DownloadJob]:
    """Expand one source into one or more concrete download jobs."""
    if source.platforms:
        jobs = []
        for platform, url in sorted(source.platforms.items()):
            if source.type == "toolchain":
                dest = drive_path / "tools" / "platformio" / "packages" / platform
            else:
                dest = drive_path / "bin" / platform
            jobs.append(
                DownloadJob(
                    source_id=source.id,
                    source_type=source.type,
                    url=url,
                    dest_dir=dest,
                    source=source,
                    platform=platform,
                )
            )
        return jobs

    return [
        DownloadJob(
            source_id=source.id,
            source_type=source.type,
            url=resolve_url(source),
            dest_dir=drive_path / TYPE_DIRS.get(source.type, "other"),
            source=source,
        )
    ]
```

Cross-platform toolchain packages (framework-espidf, tool-esptoolpy) have no
`platforms:` map, so they fall through to the else branch and land in
`tools/platformio/packages/` via TYPE_DIRS.

**Step 4: Write test**

```python
def test_expand_toolchain_source_platforms(tmp_path):
    """Toolchain sources with platforms go to tools/platformio/packages/{platform}/."""
    source = Source(
        id="toolchain-xtensa-esp-elf",
        type="toolchain",
        platforms={
            "linux-x86_64": "https://example.com/linux.tar.gz",
            "macos-arm64": "https://example.com/macos.tar.gz",
        },
    )
    jobs = expand_source_downloads(source, tmp_path)
    assert len(jobs) == 2
    assert jobs[0].dest_dir == tmp_path / "tools" / "platformio" / "packages" / "linux-x86_64"
    assert jobs[1].dest_dir == tmp_path / "tools" / "platformio" / "packages" / "macos-arm64"


def test_expand_toolchain_source_no_platforms(tmp_path):
    """Cross-platform toolchain sources go to tools/platformio/packages/."""
    source = Source(
        id="framework-espidf",
        type="toolchain",
        url="https://example.com/espidf.tar.gz",
    )
    jobs = expand_source_downloads(source, tmp_path)
    assert len(jobs) == 1
    assert jobs[0].dest_dir == tmp_path / "tools" / "platformio" / "packages"
```

**Step 5: Run tests**

```bash
uv run pytest src/tests/test_commands.py -v -k "toolchain"
```

**Step 6: Commit**

```bash
git add src/svalbard/commands.py src/svalbard/toolkit_generator.py \
        recipes/tools/*.yaml src/tests/test_commands.py
git commit -m "feat: add toolchain source type for PlatformIO packages"
```

---

### Task 4: Add pio-setup.sh drive action

**Files:**
- Create: `recipes/actions/pio-setup.sh`
- Modify: `src/svalbard/toolkit_generator.py` (add embedded dev entries)

**Step 1: Write pio-setup.sh**

This script:
1. Detects the host platform
2. Extracts toolchain archives (preserving structure) to a host-side cache
3. Exports PlatformIO environment variables
4. Drops the user into a configured subshell

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$DRIVE_ROOT/.svalbard/lib/ui.sh"
source "$DRIVE_ROOT/.svalbard/lib/platform.sh"

PLATFORM="$(detect_platform)"
PIO_CACHE="${SVALBARD_PIO_CACHE:-/tmp/svalbard-pio}"
PKG_DIR="$PIO_CACHE/packages"
DRIVE_PKG="$DRIVE_ROOT/tools/platformio/packages"
DRIVE_SHARED="$DRIVE_PKG"
DRIVE_LIB="$DRIVE_ROOT/tools/platformio/lib"

mkdir -p "$PKG_DIR"

# Extract platform-specific toolchain archives (skip if already extracted)
_extract_toolchain() {
    local archive="$1"
    local name="$2"
    local dest="$PKG_DIR/$name"

    if [ -d "$dest" ] && [ -f "$dest/.extracted" ]; then
        return 0
    fi

    echo "  Extracting $name..."
    rm -rf "$dest"
    mkdir -p "$dest"

    case "$archive" in
        *.tar.gz|*.tgz)  tar xzf "$archive" -C "$dest" --strip-components=1 ;;
        *.tar.xz)        tar xJf "$archive" -C "$dest" --strip-components=1 ;;
        *.zip)           unzip -qo "$archive" -d "$dest" ;;
    esac

    touch "$dest/.extracted"
}

# Extract platform-specific packages
if [ -d "$DRIVE_PKG/$PLATFORM" ]; then
    for archive in "$DRIVE_PKG/$PLATFORM"/*.tar.gz "$DRIVE_PKG/$PLATFORM"/*.tar.xz "$DRIVE_PKG/$PLATFORM"/*.zip; do
        [ -f "$archive" ] || continue
        # Derive package name from archive filename (e.g., toolchain-xtensa-esp-elf-*)
        pkg_name="${archive##*/}"
        pkg_name="${pkg_name%%-[0-9]*}"  # strip version suffix
        _extract_toolchain "$archive" "$pkg_name"
    done
fi

# Extract shared (cross-platform) packages
for archive in "$DRIVE_SHARED"/*.tar.gz "$DRIVE_SHARED"/*.tar.xz "$DRIVE_SHARED"/*.zip; do
    [ -f "$archive" ] || continue
    pkg_name="${archive##*/}"
    pkg_name="${pkg_name%%-[0-9]*}"
    _extract_toolchain "$archive" "$pkg_name"
done

# Link global libraries from stick (read-only)
if [ -d "$DRIVE_LIB" ] && [ ! -L "$PIO_CACHE/lib" ]; then
    ln -sfn "$DRIVE_LIB" "$PIO_CACHE/lib"
fi

export PLATFORMIO_CORE_DIR="$PIO_CACHE"
export PLATFORMIO_BUILD_DIR="/tmp/svalbard-pio-build"

echo ""
echo "Embedded dev shell ready."
echo "  Toolchains: $PKG_DIR"
echo "  Libraries:  $DRIVE_LIB"
echo "  Build dir:  $PLATFORMIO_BUILD_DIR"
echo ""
echo "  pio init --board esp32dev --project-option 'framework=espidf'"
echo "  pio run"
echo "  pio run -t upload"
echo "  pio device monitor"
echo ""

# Drop into a subshell with the configured environment
exec "$SHELL"
```

**Step 2: Add embedded dev entry to toolkit_generator.py**

In `_build_entries`, after the `[apps]` section, add:

```python
# ── Embedded Dev ───────────────────────────────────────────────
toolchain_entries = [e for e in manifest.entries if e.type == "toolchain"]
if toolchain_entries:
    lines.append("[embedded]")
    lines.append(
        "Open embedded dev shell"
        "\t.svalbard/actions/pio-setup.sh"
    )
    lines.append("")
```

**Step 3: Write test for embedded dev entry generation**

In `src/tests/test_toolkit_generator.py`, add a test that verifies the
`[embedded]` section appears when toolchain entries exist in the manifest.

**Step 4: Run tests**

```bash
uv run pytest src/tests/test_toolkit_generator.py -v
```

**Step 5: Commit**

```bash
git add recipes/actions/pio-setup.sh src/svalbard/toolkit_generator.py \
        src/tests/test_toolkit_generator.py
git commit -m "feat: add embedded dev shell action for PlatformIO"
```

---

### Task 5: Test the full flow end-to-end

**Step 1: Create a test preset with just ESP32 toolchain**

```bash
cat > /tmp/test-esp32.yaml << 'EOF'
name: test-esp32
kind: test
description: ESP32 toolchain smoke test
target_size_gb: 1
region: ""
sources:
- tool-esptoolpy
EOF
```

Start with just `tool-esptoolpy` (521 KB) to validate the download pipeline
without pulling 300+ MB toolchains.

**Step 2: Init and sync a test drive**

```bash
mkdir -p /tmp/test-drive
svalbard init /tmp/test-drive --preset test-esp32
svalbard sync /tmp/test-drive
```

Verify: `tool-esptoolpy` archive appears in
`/tmp/test-drive/tools/platformio/packages/`.

**Step 3: Test pio-setup.sh extraction**

```bash
cd /tmp/test-drive && DRIVE_ROOT=/tmp/test-drive .svalbard/actions/pio-setup.sh
```

Verify: extracted package in `/tmp/svalbard-pio/packages/tool-esptoolpy/`.

**Step 4: Full test with Xtensa toolchain (manual, larger download)**

```bash
# Add toolchain-xtensa-esp-elf to the test preset and re-sync
# This downloads ~320 MB — only do this when validating the real flow
```

**Step 5: Commit any fixes**

---

### Task 6: Pre-cache popular ESP32 libraries (future)

This task is lower priority — it requires either:

A) Running `pio pkg install --global --library <name>` during provisioning
   with `PLATFORMIO_CORE_DIR` pointed at a staging directory, then copying the
   result to `tools/platformio/lib/`.

B) Downloading library archives from PlatformIO registry API and extracting
   them manually with `library.json` preserved.

**Libraries to cache (initial set):**
- FastLED
- Adafruit BME280
- Adafruit Unified Sensor
- PubSubClient (MQTT)
- AsyncTCP + ESPAsyncWebServer
- TFT_eSPI
- LittleFS
- ESP32Servo

**Deferred:** Design this as a builder recipe (like `media-zim.py`) that
runs `pio` commands to populate the library cache during `svalbard sync`.

---

## Summary

| Task | What | Code changes | Size |
|------|------|-------------|------|
| 1 | Recipe YAMLs | 5 new YAML files | Small |
| 2 | Pack YAML + nested resolution | 1 YAML + presets.py tweak | Small |
| 3 | `toolchain` type handling | commands.py + toolkit_generator.py | Medium |
| 4 | pio-setup.sh action | New script + toolkit_generator.py | Medium |
| 5 | End-to-end test | Manual validation | Small |
| 6 | Library pre-caching | Future — builder recipe | Deferred |

Tasks 1-4 are the MVP. Task 5 validates it works. Task 6 adds library
convenience later.
