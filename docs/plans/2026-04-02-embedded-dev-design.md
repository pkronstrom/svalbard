# Embedded Development Toolkit — Design

**Status:** Design
**Date:** 2026-04-02

## Goal

Offline embedded development from a Svalbard USB stick. Plug in, connect a
microcontroller, flash firmware. No internet required after provisioning.

The stick carries PlatformIO toolchains, frameworks, libraries, documentation,
datasheets, and example projects. Users edit with vim/neovim on the stick or
VSCode on the host (assumed installed via Brewfile). All `pio` CLI commands work
offline.

## Approach

**CLI-first.** PlatformIO CLI (`pio`) drives all builds, uploads, and library
management. No IDE dependency on the stick — the host may have VSCode +
PlatformIO extension installed (covered by the Brewfile), but the stick works
standalone with any editor.

**ESP-IDF as default framework for ESP32.** Arduino available as an ESP-IDF
component for library compatibility. Arduino is the default framework for AVR,
STM32, and RP2040 where it's the natural fit.

**Xtensa ESP32 variants by default, RISC-V as add-on.** The base ESP32 pack
covers ESP32, S2, and S3 (Xtensa). RISC-V variants (C3, C6, H2) are a separate
recipe — add when you have the hardware. Saves 2.3 GB per host platform.

## Architecture

### Pack structure

Four standalone packs, composed into one umbrella tier:

```
packs/
  embedded/
    esp32-dev.yaml      # ESP-IDF + Xtensa toolchain (base)
    esp32-riscv.yaml    # RISC-V toolchain add-on (C3/C6/H2)
    avr-dev.yaml        # Arduino AVR + avr-gcc
    stm32-dev.yaml      # STM32duino / STM32Cube + arm-none-eabi-gcc
    rp2040-dev.yaml     # Arduino-pico / Pico SDK + arm-none-eabi-gcc
  embedded-dev.yaml     # Umbrella — includes all four
```

The umbrella preset:

```yaml
name: embedded-dev
kind: pack
description: Offline embedded development — ESP32, AVR, STM32, RP2040
includes:
  - embedded/esp32-dev
  - embedded/avr-dev
  - embedded/stm32-dev
  - embedded/rp2040-dev
```

File-size presets include packs by tier:

```yaml
# presets/default-512.yaml (example)
includes:
  - tools-base
  - embedded/esp32-dev    # just ESP32 at 512 GB

# presets/default-1tb.yaml (example)
includes:
  - tools-base
  - embedded-dev          # all four platforms at 1 TB+
```

### Shared toolchains

STM32 and RP2040 both use `arm-none-eabi-gcc`. The resolver should detect this
and download the toolchain once, not twice.

| Toolchain | Used by | Size per host platform | Default? |
|-----------|---------|----------------------|----------|
| xtensa-esp-elf-gcc | ESP32, S2, S3 | ~1.2 GB | Yes (esp32-dev) |
| riscv32-esp-gcc | ESP32-C3, C6, H2 | ~2.3 GB | No (esp32-riscv add-on) |
| arm-none-eabi-gcc | STM32, RP2040 | ~500-700 MB | Yes (shared) |
| avr-gcc | AVR/ATmega | ~200-250 MB | Yes (avr-dev) |

Default toolchain footprint (without RISC-V): **~1.9-2.2 GB per host platform**.
With RISC-V add-on: **~4.2-4.5 GB per host platform**.

### PlatformIO integration

The stick carries a pre-populated PlatformIO packages directory. A setup script
(or `run.sh` menu entry) configures PlatformIO to use the stick's cache:

```bash
# Point PlatformIO at the stick's packages
export PLATFORMIO_CORE_DIR="$DRIVE/tools/platformio"
# Or via platformio.ini override:
# platformio_core_dir = /Volumes/svalbard/tools/platformio
```

This directory contains:
- `packages/` — toolchains, frameworks, tools (the big stuff)
- `platforms/` — board definitions and platform metadata
- `lib/` — pre-cached libraries

PlatformIO checks this directory before attempting network downloads, so offline
use is transparent.

## Host platform presets

Toolchains are native binaries — each host OS/arch needs its own copy. Platform
presets control which host architectures to provision.

This is a **drive-level setting**, not per-pack. It affects all tool recipes
(kiwix-serve, llama-server, etc.), not just embedded toolchains.

### Defined presets

```yaml
modern:
  description: >
    Current hardware — Apple Silicon Macs, standard PCs,
    and Raspberry Pi 3+
  platforms:
    - darwin-arm64
    - linux-x64
    - linux-arm64

universal:
  description: >
    All supported hardware — includes older Intel Macs,
    Raspberry Pi 2 and Zero, and Windows PCs
  platforms:
    - darwin-arm64
    - darwin-x64
    - linux-x64
    - linux-arm64
    - linux-armv7l
    - windows-x64

local:
  description: >
    This computer only — smallest possible drive
  platforms:
    - <auto-detected at provision time>

minimal-embedded:
  description: >
    Headless Linux boards only — for dedicated workshop
    or field machines
  platforms:
    - linux-x64
    - linux-arm64
```

### Drive config

```yaml
# Written by wizard, editable by user
host_platforms: modern          # preset name

# Or explicit override:
# host_platforms:
#   - darwin-arm64
#   - linux-armv7l
```

Default: `modern`. The wizard presents presets with descriptions for
non-technical users.

### Size impact

With `modern` (3 platforms) and all four embedded packs (default, no RISC-V):

| Component | Per platform | × 3 platforms |
|-----------|-------------|---------------|
| ESP32 Xtensa toolchain + ESP-IDF | ~1.7 GB | ~5.1 GB |
| ARM toolchain (STM32 + RP2040 shared) | ~0.6 GB | ~1.8 GB |
| AVR toolchain | ~0.25 GB | ~0.75 GB |
| Frameworks + board support | ~1.0 GB | ~1.0 GB (mostly cross-platform) |
| Docs + datasheets + examples | ~0.5 GB | ~0.5 GB (cross-platform) |
| **Default total** | | **~9 GB** |
| + ESP32 RISC-V add-on | ~2.4 GB | +~7.2 GB |
| **Full total (with RISC-V)** | | **~16 GB** |

With `local` (1 platform, default): **~4 GB**. With RISC-V: **~6.5 GB**.

## Per-pack configuration

Each pack exposes configuration knobs in its YAML. Defaults are practical
(most common setup); users expand or constrain as needed.

### ESP32

```yaml
name: esp32-dev
kind: pack
description: ESP32 offline development — Xtensa variants (ESP32/S2/S3), ESP-IDF framework
size_gb: 1.7                  # per host platform, base
config:
  framework: espidf           # espidf | arduino | both
  include_debuggers: true
  include_arduino_addon: false # +736 MB
  include_riscv: false         # +2.4 GB — separate esp32-riscv recipe
```

### AVR

```yaml
name: avr-dev
kind: pack
description: Arduino and ATtiny offline development
config:
  framework: arduino        # arduino | bare
  include_attiny: true
```

### STM32

```yaml
name: stm32-dev
kind: pack
description: STM32 offline development — F1 and F4 families
config:
  framework: arduino        # arduino | stm32cube | both
  families: [F1, F4]       # F1, F4, L4, H7, G4, ...
```

### RP2040

```yaml
name: rp2040-dev
kind: pack
description: Raspberry Pi Pico and RP2040 offline development
config:
  framework: arduino        # arduino | pico-sdk | both
  include_rp2350: true
```

## Content bundled per pack

### Documentation (cross-platform, bundled once)

Each pack includes offline-readable documentation:

- **ESP32:** ESP-IDF Programming Guide, chip datasheets + TRMs, PlatformIO
  ESP32 quick reference
- **AVR:** Arduino language reference, ATmega328P/2560 datasheets, ATtiny85
  datasheet
- **STM32:** STM32F411/F103 reference manuals + datasheets, STM32duino getting
  started
- **RP2040:** RP2040/RP2350 datasheets, Pico SDK docs, PIO programming guide,
  board pinout diagrams

### Shared documentation

- Common sensor/module datasheets (BME280, SSD1306, MPU6050, DHT22, etc.)
- PlatformIO CLI reference
- Serial communication basics (UART, I2C, SPI protocols)

### Pre-cached libraries (ESP32 focus, curated)

Networking: AsyncWebServer, PubSubClient (MQTT), ESP-NOW helpers
Displays: TFT_eSPI, SSD1306 drivers, LVGL
Sensors: Adafruit Unified Sensor, DHT, BME280, MPU6050
LEDs: FastLED, NeoPixel
Storage: LittleFS
Communication: Wire (I2C), SPI
Motor: AccelStepper, ESP32Servo

### Example projects

Per-platform starter projects that compile and flash out of the box:

- **ESP32:** WiFi AP + captive portal, BLE scanner, MQTT sensor node, OTA server
- **AVR:** Blinky, serial echo, I2C scanner
- **STM32:** Blinky, USB CDC serial, I2C scanner
- **RP2040:** PIO NeoPixel driver, WiFi AP (Pico W), USB HID keyboard

## Dependencies

- **Python runtime**: Required for PlatformIO Core and esptool. Provided by the
  broader programming toolkit preset (`programming-essentials` or
  `programming-full`), not bundled per-pack.
- **tools-base pack**: Provides foundational drive tools (search, file serving,
  etc.) that the embedded packs build on.

## Relationship to existing plans

- **Programming Toolkit Preset** (`2026-03-30-programming-toolkit-preset.md`):
  This design replaces and expands the `embedded-dev` section of that plan. The
  programming toolkit still owns compilers (Zig, Go, Rust, Python) and general
  dev tools. Embedded packs own microcontroller-specific toolchains.
- **Computing Pack** (`packs/computing/README.md`): Provides DevDocs ZIMs and
  Stack Exchange archives for programming reference. Complementary — the
  computing pack gives you docs to read, embedded packs give you tools to run.
  The Electronics Stack Exchange recipe bridges both.

## Open questions

- Library pre-caching: hand-curated list vs. scrape PlatformIO registry for
  top-N by download count?
- MicroPython / CircuitPython firmware images: bundle pre-built .bin/.uf2 for
  common boards?
- How to version-lock frameworks so the cached toolchain matches what PlatformIO
  expects? Pin platform versions in pack config?
- Should `run.sh` get an "Embedded Dev" submenu (flash, monitor, build) or just
  document the `pio` commands?
