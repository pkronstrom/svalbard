# Pack: STM32 Development

**Status:** TODO
**Date:** 2026-04-02

## Overview

Offline STM32 development toolkit for PlatformIO CLI. Arduino (STM32duino) for quick prototyping, STM32Cube HAL for full hardware control. Focus on the F1 and F4 families — the most common hobbyist chips.

## Components

### Toolchains (per host platform)

| Component | Size (unpacked) | Notes |
|-----------|-----------------|-------|
| arm-none-eabi-gcc | ~500-700 MB | **Shared with RP2040 pack** |
| OpenOCD | ~10 MB | SWD debug, supports ST-Link |
| STM32 programmer (stm32flash) | ~1 MB | Serial bootloader flashing |

### Frameworks

| Framework | Size | Default? |
|-----------|------|----------|
| Arduino (STM32duino) | ~30-50 MB | Yes — lowest friction |
| STM32Cube HAL/LL | ~200-400 MB | Optional — full hardware control |
| CMSIS | included | Available — register-level |

### Target boards

**Primary (F4 family — recommended starting point):**
- WeAct BlackPill STM32F411CE — modern hobbyist go-to ($3-5)
- WeAct BlackPill STM32F401CE — budget variant
- Nucleo-64 F446RE — official ST board with ST-Link

**Classic (F1 family):**
- Blue Pill STM32F103C8 — the $2 classic
- Nucleo-64 F103RB

**Extended (included but not primary focus):**
- STM32L476 (low-power)
- STM32H7 series (high-performance, future)

### Documentation to bundle

- STM32F411 reference manual + datasheet
- STM32F103 reference manual + datasheet
- STM32duino wiki / getting started
- ST-Link usage guide
- Common peripheral patterns (I2C, SPI, UART, ADC, DMA)

## Size estimate

- ARM toolchain: ~500-700 MB per host platform (shared with RP2040)
- STM32-specific frameworks + board support: ~200-500 MB
- Documentation + examples: ~100 MB (cross-platform)
- **Incremental cost if RP2040 also installed: ~200-500 MB** (toolchain shared)

## Open questions

- Which STM32 families to include board definitions for? (F1, F4 minimum — but platform supports all)
- Include STM32CubeMX-generated project templates?
- Zephyr RTOS as a framework option? (growing STM32 support)
- Pre-built example projects: blinky, USB CDC, I2C scanner?
