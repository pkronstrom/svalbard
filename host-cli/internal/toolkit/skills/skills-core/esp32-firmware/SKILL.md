---
name: esp32-firmware
description: Write and build ESP32 firmware in C (ESP-IDF) offline with the drive's PlatformIO, starting from the drive's tested example, for ESP32, ESP32-S2 and ESP32-S3 boards; flash it only when a board is connected.
requires: [platformio-core, platformio-espressif32]
---

## Steps

`<folder>` is the skill folder that `skill_read` returned, for example
`/Volumes/SVALBARD/skills/skills-core/esp32-firmware`. `$DRIVE_ROOT` is the
drive. Work in the user's working folder, never inside `<folder>` or the
drive's `tools/`.

1. Copy the drive's example to a new folder (pick a name that does not
   exist yet):

   ```sh
   cp -R "$DRIVE_ROOT/tools/platformio/examples/blink" my-firmware
   ```

2. Write the program in `my-firmware/src/main.c`. It is ESP-IDF C with
   `void app_main(void)` as the entry point. Keep `platformio.ini` and the
   `CMakeLists.txt` files as they are. Tested snippets for logging, buttons,
   PWM and analog input are in `<folder>/references/esp-idf.md`.
3. Build (ESP32 by default; add `-e esp32s2` or `-e esp32s3` for those):

   ```sh
   sh <folder>/scripts/pio.sh my-firmware run
   ```

   The first build on a computer prepares PlatformIO from the drive first:
   a few minutes and about 5 GB of disk. Wait for it.
4. Read the errors, fix `src/main.c`, build again until it ends with
   `[SUCCESS]`.
5. Check your result. It must print `ok`:

   ```sh
   sh <folder>/examples/check.sh my-firmware
   ```

6. Flash only when the user says a board is connected by USB:
   `sh <folder>/scripts/pio.sh my-firmware run -t upload` (with `-e` for an
   S2 or S3). To watch its output the user runs `./run embedded` in the
   drive's folder, then `cd` to the project and `pio device monitor`
   (Ctrl-C quits).

## What is on the drive

- PlatformIO 6.2 with espressif32 7.1.3 and ESP-IDF 6.1, compilers for
  ESP32 (Xtensa) and the RISC-V coprocessors, and esptool.
- Boards in the example: `esp32` (esp32dev), `esp32s2` (esp32-s2-saola-1),
  `esp32s3` (esp32-s3-devkitc-1).
- Not here, and not to be added offline: Arduino, other chip families,
  `lib_deps` libraries, managed components, another platform version.
  Write what a library would do with ESP-IDF's own drivers.
- GPIO 2 is the LED on many ESP32 boards but not all; ask which pin the
  user's board uses. A build that succeeds does not prove the board works.
