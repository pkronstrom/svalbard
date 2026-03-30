# Programming Toolkit Preset

**Status:** Idea / future work
**Date:** 2026-03-30

## Concept

Bundled offline programming toolchains, compilers, documentation, and embedded
development tools. These would be defined as composable preset tiers that can be
included from file-size presets (e.g. `default-512` could include
`programming-essentials` as a nested preset).

The goal: not just consuming knowledge offline, but being able to **build and
deploy software** — from desktop apps to microcontroller firmware.

## Preset Tiers

### programming-essentials (~200 MB)

Minimum viable offline development environment.

| Component | Size | Purpose |
|-----------|------|---------|
| Zig | ~40 MB | C/C++/Zig compiler, bundles musl libc, cross-compiles to any target |
| Go | ~150 MB | Fully self-contained toolchain + stdlib, no package manager needed |
| TCC | ~2 MB | Tiny C Compiler for quick C scripts, near-instant compilation |
| Lua | ~1 MB | Embeddable scripting language, tiny runtime |

**Documentation to bundle:**
- Go stdlib reference (offline godoc)
- Zig language reference
- C standard library reference (e.g. cppreference subset)
- POSIX programming manual

### programming-full (~1-1.5 GB)

Broader language support with vendored package ecosystems.

Everything in essentials, plus:

| Component | Size | Purpose |
|-----------|------|---------|
| Rust (rustc + cargo) | ~400-500 MB | Systems programming, vendored core crates |
| CPython + pip wheels | ~30-50 MB | Python interpreter + bundled wheels for essentials (requests, numpy, etc.) |
| MicroPython firmware | ~5 MB | Pre-built firmware images for common boards |
| Node.js (optional) | ~30 MB | JavaScript runtime for tooling |

**Vendored packages:**
- Rust: serde, tokio, clap, reqwest, embedded-hal — top ~50 no-std crates
- Python: pip wheels for stdlib-adjacent packages
- Go: no vendoring needed (stdlib is self-sufficient)

**Documentation to bundle:**
- Rust Book + stdlib reference
- Python docs
- MicroPython docs

### embedded-dev (~200-500 MB addon)

For microcontroller and hardware development. Can combine with either tier above.

| Component | Size | Purpose |
|-----------|------|---------|
| ESP-IDF toolchain | ~200 MB | ESP32/ESP32-S2/S3/C3 development |
| MicroPython firmware | ~5 MB | Pre-built .bin for ESP32, RP2040, STM32 |
| CircuitPython firmware | ~5 MB | Pre-built .uf2 for RP2040, SAMD, nRF |
| Arduino CLI + cores | ~100-300 MB | Arduino framework for AVR, ESP32, RP2040 |
| esptool.py | ~1 MB | Flash firmware to ESP32 devices |
| picotool | ~1 MB | Flash firmware to RP2040 |
| OpenOCD | ~10 MB | On-chip debugger for ARM/RISC-V |

**Documentation to bundle:**
- ESP-IDF programming guide
- RP2040 datasheet + SDK docs
- MicroPython quick reference per board
- Arduino language reference
- Common sensor/module datasheets (BME280, SSD1306, etc.)

**Pre-built example projects:**
- ESP32 WiFi AP + web server (instant local network)
- RP2040 USB serial terminal
- MicroPython REPL-ready images

## Composable Preset Inclusion

File-size presets could include programming tiers as nested presets:

```yaml
# presets/default-512.yaml
name: default-512
sources:
  - ...existing sources...
includes:
  - programming-essentials    # pulls in the whole tier
```

```yaml
# presets/default-1tb.yaml
name: default-1tb
sources:
  - ...existing sources...
includes:
  - programming-full
  - embedded-dev
```

This requires a small change to the preset loader to support an `includes:` field
that merges sources from another preset definition.

## Platform Builds Required

Each compiler/tool needs per-platform builds:

- `linux-x86_64`
- `linux-arm64`
- `macos-arm64`
- `macos-x86_64` (maybe — dropping?)
- `windows-x86_64` (if Windows support is added)

For Go and Zig this is trivial — both distribute official tarballs per platform.
Rust is also straightforward via `rustup` standalone installers. ESP-IDF is the
most complex (large toolchain with cross-compiler for Xtensa/RISC-V).

## Open Questions

- Should vendored crate/package mirrors be a build-time step in svalbard, or
  pre-assembled archives?
- How to handle Go modules offline — vendor a GOPROXY mirror, or just rely on
  stdlib?
- Which ESP32 variants to target (original, S2, S3, C3, C6)?
- Should we include pre-built binaries for common tools people might want to
  study/modify (e.g. BusyBox source + build scripts)?
- Cross-compilation targets: should Zig/Go be set up to cross-compile for
  ARM embedded Linux (Raspberry Pi, routers)?
- Include RISC-V toolchain for future-proofing?
