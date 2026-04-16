# Embedded Development — Getting Started

This drive contains offline toolchains for programming ESP32 microcontrollers
using PlatformIO. Everything works without an internet connection.

## Prerequisites

You need PlatformIO CLI installed on your computer:

    brew install platformio        # macOS (via Homebrew)
    pip install platformio         # any platform

If you have VSCode with the PlatformIO extension, that works too — the dev
shell configures the environment for both CLI and IDE use.

## Quick start

### 1. Open the dev shell

From the drive menu (`./run`), select **Open embedded dev shell**.

This extracts the toolchains to a fast location on your computer and drops
you into a terminal with everything configured. You'll see:

    Embedded dev shell ready.
      Toolchains: /tmp/svalbard-pio/packages
      Build dir:  /tmp/svalbard-pio-build

### 2. Create a project

Create your project on your computer's filesystem (not on the stick — your
computer's drive is much faster for compilation):

    mkdir ~/my-esp32-project && cd ~/my-esp32-project
    pio init --board esp32dev --project-option "framework=espidf"

This creates a PlatformIO project with ESP-IDF framework for ESP32.

### 3. Write your code

    vim src/main.c

Or open the folder in VSCode — the PlatformIO extension will detect the
project automatically.

Minimal blink example for ESP-IDF:

    #include "driver/gpio.h"
    #include "freertos/FreeRTOS.h"
    #include "freertos/task.h"

    #define LED_PIN 2

    void app_main(void) {
        gpio_reset_pin(LED_PIN);
        gpio_set_direction(LED_PIN, GPIO_MODE_OUTPUT);
        while (1) {
            gpio_set_level(LED_PIN, 1);
            vTaskDelay(500 / portTICK_PERIOD_MS);
            gpio_set_level(LED_PIN, 0);
            vTaskDelay(500 / portTICK_PERIOD_MS);
        }
    }

### 4. Build

    pio run

First build takes a few minutes (compiles ESP-IDF from source). Subsequent
builds are fast — only your changes get recompiled.

### 5. Flash

Plug in your ESP32 via USB, then:

    pio run -t upload

PlatformIO auto-detects the serial port. If you have multiple devices,
specify the port:

    pio run -t upload --upload-port /dev/tty.usbserial-0001

### 6. Monitor serial output

    pio device monitor

Press Ctrl+C to exit the monitor. To build, flash, and monitor in one go:

    pio run -t upload && pio device monitor

## Supported boards

The Xtensa toolchain on this drive supports these ESP32 variants:

| Board flag         | Chip      | Notes                        |
|--------------------|-----------|------------------------------|
| esp32dev           | ESP32     | Original, most common        |
| esp32-s2-saola-1   | ESP32-S2  | Single-core, native USB      |
| esp32-s3-devkitc-1 | ESP32-S3  | Dual-core, USB, AI accel.    |

To target a specific board:

    pio init --board esp32-s3-devkitc-1 --project-option "framework=espidf"

List all available boards:

    pio boards --filter espressif32

## Using Arduino libraries with ESP-IDF

You can use Arduino as a component within ESP-IDF projects. In your
`platformio.ini`:

    [env:esp32dev]
    platform = espressif32
    board = esp32dev
    framework = espidf
    lib_deps =
        espressif/arduino-esp32

This lets you use Arduino-compatible libraries while keeping ESP-IDF's
full RTOS and hardware control.

## Pre-cached libraries

If this drive includes pre-cached libraries (check `tools/platformio/lib/`),
they're available automatically — just `#include` the header and PlatformIO
finds them.

To see what's available:

    ls $(printenv PLATFORMIO_CORE_DIR)/lib/

## Project structure

A PlatformIO project looks like this:

    my-project/
      platformio.ini        <- board, framework, library config
      src/
        main.c              <- your code
      include/
        config.h            <- your headers
      lib/
        MyLib/              <- project-local libraries
      test/
        test_main.c         <- unit tests (optional)

## Common commands

    pio run                       # build
    pio run -t upload             # flash
    pio run -t clean              # clean build artifacts
    pio device monitor            # serial monitor (115200 baud default)
    pio device monitor -b 9600    # serial monitor at 9600 baud
    pio device list               # list connected devices
    pio boards --filter espressif # list supported boards
    pio check                     # static analysis (cppcheck)
    pio test                      # run unit tests

## How it works

The dev shell sets two environment variables:

- `PLATFORMIO_CORE_DIR` — points at the extracted toolchains from this drive
- `PLATFORMIO_BUILD_DIR` — points at a temp directory on your computer

PlatformIO checks `PLATFORMIO_CORE_DIR` for toolchains before trying to
download them. Since the drive provides everything, no internet is needed.

Build artifacts go to your computer's filesystem (not the drive) because
compilation generates thousands of small files — your computer's SSD is
much faster than a USB stick for this.

## Troubleshooting

**"toolchain not found" or "platform not installed"**
Make sure you're in the dev shell (opened from the drive menu). Check
that `echo $PLATFORMIO_CORE_DIR` points to the extracted packages.

**Upload fails — "no serial port detected"**
Check that your ESP32 is connected via USB and your OS recognizes it.
On macOS, you may need the CP2102 or CH340 USB-serial driver:

    ls /dev/tty.usb*        # should show your device

**Build is very slow**
First build compiles the entire ESP-IDF framework (~5 min). Subsequent
builds only recompile your changes. If every build is slow, check that
`PLATFORMIO_BUILD_DIR` points to your local filesystem, not the USB stick.

**Permission denied on serial port**
On Linux, add yourself to the dialout group:

    sudo usermod -a -G dialout $USER
    # Log out and back in for it to take effect
