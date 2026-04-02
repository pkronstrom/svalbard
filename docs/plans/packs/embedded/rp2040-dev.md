# Pack: RP2040 Development

**Status:** TODO
**Date:** 2026-04-02

## Overview

Offline RP2040/RP2350 development toolkit for PlatformIO CLI. Arduino (Earle Philhower core) as default, Pico SDK available for bare-metal C/C++. Supports Pico, Pico W (WiFi/BLE), and Pico 2 (RP2350).

## Components

### Toolchains (per host platform)

| Component | Size (unpacked) | Notes |
|-----------|-----------------|-------|
| arm-none-eabi-gcc | ~500-700 MB | **Shared with STM32 pack** |
| picotool | ~5 MB | Flash via USB BOOTSEL |
| elf2uf2 | ~1 MB | Convert ELF → UF2 for drag-and-drop |
| pioasm | ~1 MB | PIO state machine assembler |
| OpenOCD (RP2040 branch) | ~10 MB | SWD debug via debug probe |

### Frameworks

| Framework | Size | Default? |
|-----------|------|----------|
| Arduino (arduino-pico, Earle Philhower) | ~50-100 MB | Yes — WiFi/BLE support on Pico W |
| Pico SDK | ~50-100 MB | Optional — bare-metal C/C++ |

### Target boards

- Raspberry Pi Pico (RP2040)
- Raspberry Pi Pico W (RP2040 + CYW43439 WiFi/BLE)
- Raspberry Pi Pico 2 (RP2350, newer — ARM Cortex-M33)
- Adafruit Feather RP2040
- Seeed XIAO RP2040
- SparkFun Thing Plus RP2040

### Documentation to bundle

- RP2040 datasheet
- RP2350 datasheet
- Pico SDK documentation
- Pico W networking guide
- PIO (Programmable I/O) programming guide — unique RP2040 feature
- Pinout diagrams for Pico / Pico W / Pico 2

### Unique features to document

- **PIO state machines** — programmable I/O blocks, unique to RP2040/RP2350
- **Dual-core** — both cores available from Arduino and Pico SDK
- **UF2 drag-and-drop** — no special programmer needed, flash via USB mass storage

## Size estimate

- ARM toolchain: ~500-700 MB per host platform (shared with STM32)
- RP2040-specific frameworks + board support: ~100-200 MB
- Documentation + examples: ~50 MB (cross-platform)
- **Incremental cost if STM32 also installed: ~100-200 MB** (toolchain shared)

## Open questions

- RP2350 RISC-V core support via PlatformIO? (experimental — may need separate RISC-V toolchain)
- Include CircuitPython firmware images? (alternative to MicroPython)
- MicroPython firmware images for Pico / Pico W?
- Pre-built examples: PIO NeoPixel driver, WiFi AP, USB HID?
