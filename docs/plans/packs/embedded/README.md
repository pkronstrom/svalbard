# Pack: Embedded Development

Offline microcontroller development toolchains, frameworks, libraries, and documentation.

## Audience

Makers, embedded engineers, hobbyists, and anyone who needs to program microcontrollers without internet access.

## Status

Design — see [Embedded Dev Design](../../2026-04-02-embedded-dev-design.md) for the full plan.

## Packs

| Pack | Target | Toolchain | Default framework | Status |
|------|--------|-----------|-------------------|--------|
| [esp32-dev](esp32-dev.md) | ESP32 / S2 / S3 | Xtensa GCC | ESP-IDF | Design in progress |
| esp32-riscv (add-on) | C3 / C6 / H2 | RISC-V GCC | (uses esp32-dev) | TODO |
| [avr-dev](avr-dev.md) | Arduino Uno/Nano/Mega, ATtiny | avr-gcc | Arduino | TODO |
| [stm32-dev](stm32-dev.md) | STM32 F1/F4 (Blue Pill, BlackPill) | arm-none-eabi-gcc | Arduino (STM32duino) | TODO |
| [rp2040-dev](rp2040-dev.md) | Raspberry Pi Pico / Pico W / Pico 2 | arm-none-eabi-gcc (shared) | Arduino (arduino-pico) | TODO |

## Reference content (bundled with toolchains)

Q&A, language references, and documentation to ship alongside the toolchains. These are already available as recipes — just need to be included in the pack YAML.

### Recipes ready (have recipe YAML)

| Recipe | Size | Notes |
|--------|------|-------|
| `stackexchange-arduino` | 250 MB | Arduino programming, sensor interfacing Q&A |
| `stackexchange-raspberrypi` | 285 MB | GPIO, SBCs, Linux embedded Q&A |
| `stackexchange-robotics` | 230 MB | Actuators, kinematics, ROS Q&A |
| `stackexchange-3dprinting` | 115 MB | Printer troubleshooting, materials, slicing |
| `stackexchange-electronics` | 2.0 GB | Circuit design, components, PCB layout (already in default-256+) |
| `devdocs-c` | 1 MB | C standard library reference |
| `devdocs-cpp` | 7 MB | C++ standard library reference |

### Recipes needed (zimit scrape or custom build)

| Source | Size est. | Notes |
|--------|-----------|-------|
| Arduino docs (docs.arduino.cc) | ~100-200 MB | Language ref, library API, board pinouts. CC BY-SA 4.0 |
| ESP-IDF Programming Guide | ~150-300 MB | Full framework docs. Apache 2.0 |
| Raspberry Pi documentation | ~50-100 MB | Hardware specs, GPIO, Pico SDK. CC BY-SA 4.0 |
| MicroPython docs | ~20-40 MB | Quick reference per board. MIT |
| FreeRTOS docs | ~30-50 MB | Task, queue, semaphore API. MIT |
| PlatformIO docs | ~30-50 MB | CLI reference, board configs. Apache 2.0 |
| Curated datasheet ZIM | ~100-300 MB | See [engineering pack TODO](../engineering/README.md) |

## Shared toolchains

STM32 and RP2040 share `arm-none-eabi-gcc` — installing both costs less than the sum of parts.

## Size estimates (per host platform)

| Configuration | Size |
|---------------|------|
| ESP32 Xtensa only (default) | ~1.7 GB |
| ESP32 + RISC-V add-on | ~4.1 GB |
| All packs (default, no RISC-V) | ~3.0 GB |
| All packs × 3 host platforms (modern) | ~9 GB |
| All packs + RISC-V × 3 platforms | ~16 GB |
| All packs × 1 host platform (local) | ~4 GB |
