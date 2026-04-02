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
