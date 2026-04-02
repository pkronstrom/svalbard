# Pack: AVR Development

**Status:** TODO
**Date:** 2026-04-02

## Overview

Offline AVR/ATmega development toolkit for PlatformIO CLI. Arduino framework as default. Covers classic Arduino boards (Uno, Nano, Mega) and bare ATtiny chips.

## Components

### Toolchains (per host platform)

| Component | Size (unpacked) | Notes |
|-----------|-----------------|-------|
| avr-gcc (`toolchain-atmelavr`) | ~200-250 MB | AVR-specific, not shared with ARM |
| avrdude | ~5 MB | Flash via serial/ISP/USBasp |
| micronucleus | ~1 MB | Digispark/ATtiny85 USB bootloader |

### Frameworks

| Framework | Size | Default? |
|-----------|------|----------|
| Arduino AVR | ~30 MB | Yes |
| Bare metal (avr-libc) | included in toolchain | Available, no framework line |

### Documentation to bundle

- Arduino language reference
- AVR instruction set reference
- ATmega328P / ATmega2560 datasheets
- ATtiny85 datasheet
- Common shields/modules pinouts

### Target boards

- Arduino Uno (ATmega328P)
- Arduino Nano (ATmega328P)
- Arduino Mega 2560 (ATmega2560)
- Arduino Pro Mini 3.3V/5V (ATmega328P)
- Digispark (ATtiny85)
- Generic ATtiny85

## Size estimate

- Total per host platform: ~400-500 MB
- Documentation + examples: ~50 MB (cross-platform)

## Open questions

- Include ATtiny series beyond ATtiny85? (ATtiny84, ATtiny1614 etc.)
- Include MegaCoreX or other extended AVR cores?
- Pre-cache any Arduino libraries specifically for AVR constraints?
