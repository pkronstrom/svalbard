# Pack: ESP32 Development

**Status:** Design in progress
**Date:** 2026-04-02

## Overview

Offline ESP32 development toolkit for PlatformIO CLI. ESP-IDF as default framework, Arduino available as add-on component. Xtensa variants (ESP32, S2, S3) by default. RISC-V variants (C3, C6, H2) available as a separate add-on recipe.

## Approach

- CLI-first: `pio` commands + vim/neovim for editing
- Host assumed to have VSCode + PlatformIO extension installed (via Brewfile) for GUI users
- Stick provides: toolchains, frameworks, cached libraries, docs, example projects
- PlatformIO's `platformio_core_dir` pointed at stick's cache

## Components

### Toolchains — base (per host platform)

| Component | Size (unpacked) | Covers |
|-----------|-----------------|--------|
| Xtensa GCC (`toolchain-xtensa-esp-elf`) | ~1.2 GB | ESP32, S2, S3 |
| esptool.py | ~2 MB | Flash all variants |
| Xtensa GDB | ~89 MB | Debugging |
| OpenOCD-ESP32 | ~6 MB | On-chip debug |
| ESP32 ULP toolchain | ~38 MB | Ultra-low-power coprocessor |

### Toolchains — RISC-V add-on (separate recipe)

| Component | Size (unpacked) | Covers |
|-----------|-----------------|--------|
| RISC-V GCC (`toolchain-riscv32-esp`) | ~2.3 GB | C3, C6, H2 |
| RISC-V GDB | ~96 MB | Debugging |

The RISC-V toolchain is a separate recipe (`esp32-riscv-toolchain`) that can
be included when needed. Add it when you get a C3, C6, or H2 board.

### Frameworks

| Framework | Size | Default? |
|-----------|------|----------|
| ESP-IDF (v5.x) | ~398 MB | Yes |
| Arduino-ESP32 | ~736 MB | Optional add-on |

### Configuration knobs

```yaml
# packs/esp32-dev.yaml
esp32:
  framework: espidf          # espidf | arduino | both
  include_debuggers: true
  include_arduino_addon: false
  include_riscv: false        # add esp32-riscv-toolchain recipe
```

### Documentation to bundle

- ESP-IDF Programming Guide (offline HTML)
- ESP32/S3/C3/C6 datasheets + technical reference manuals
- Common sensor/module datasheets (BME280, SSD1306, MPU6050, etc.)
- PlatformIO ESP32 quick reference

### Libraries to pre-cache

Curated essentials (TBD — start with top 30-50 most-used ESP32 libraries):
- Networking: AsyncWebServer, PubSubClient (MQTT), ESP-NOW helpers
- Displays: TFT_eSPI, SSD1306/SH1106 drivers, LVGL
- Sensors: Adafruit Unified Sensor, DHT, BME280, MPU6050
- LEDs: FastLED, NeoPixel
- Storage: LittleFS, SPIFFS helpers
- Motor: AccelStepper, ESP32Servo
- Communication: I2C scanner, SPI helpers

### Example projects

- WiFi AP + captive portal web server
- BLE beacon scanner
- MQTT sensor node
- OTA update server
- MicroPython REPL-ready images

## Size estimate

- **Base (Xtensa only):** ESP-IDF + Xtensa toolchain + tools: ~1.7 GB per host platform
- **With RISC-V add-on:** +2.4 GB per host platform (~4.1 GB total)
- **With Arduino add-on:** +736 MB per host platform
- **Documentation + libraries + examples:** ~200-500 MB (cross-platform)

## Open questions

- Exact library list to pre-cache
- How to keep ESP-IDF version in sync with PlatformIO's expectations
- MicroPython firmware images: include pre-built .bin for each variant?
