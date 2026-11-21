# ESP32 offline example

Open the vault's Embedded Dev Shell (`./run embedded`). It provides Python,
PlatformIO and the pinned ESP-IDF tool closure; no host Python/PIO is needed.
Copy this entire example to a new directory on your computer before editing:

```sh
cp -R "$DRIVE_ROOT/tools/platformio/examples/blink" "$HOME/my-esp32-project"
cd "$HOME/my-esp32-project"
pio run                         # ESP32
pio run -e esp32s2               # ESP32-S2
pio run -e esp32s3               # ESP32-S3
pio run -t upload               # requires a connected ESP32
pio device monitor              # Ctrl-C exits; 115200 baud
```

Choose a fresh destination for the copy; repeating `cp -R` into an existing
project nests another directory. Select your board/environment when uploading,
e.g. `pio run -e esp32s3 -t upload`. GPIO 2 is only an example: change LED_GPIO
for your board. An RGB LED needs its own driver; console output still works.

The first build compiles the framework; later builds reuse this project's
`.pio/build`. Keep projects separate. Additional libraries, managed components,
Arduino, other MCU families, and standalone `idf.py` are outside this bundled
closure. Provision required dependencies before taking such projects offline.

First shell use prepares a host cache; cancellation can be retried. Changed
bundled content or a new vault mount path prepares a new generation. Set
SVALBARD_PIO_CACHE before opening the shell to choose another cache root (on
macOS, not under /tmp: CMake rewrites those paths and ESP-IDF builds fail). Existing
ready generations are retained. An explicitly set PLATFORMIO_BUILD_DIR is your
own override; normally let each project's .pio/build remain independent.

Serial upload/monitor requires a working cable, OS permissions and USB driver.
Firmware builds do not verify physical flashing or board LED behavior.
