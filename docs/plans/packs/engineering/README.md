# Pack: Engineering

Technical references for electrical, mechanical, embedded systems, and digital fabrication.

## Audience

Engineers, makers, hardware hackers, and technical professionals who need offline access to circuit design references, microcontroller documentation, fabrication guides, and materials data.

## Status

Idea -- collecting sources

---

## Sub-Topics

### Electrical Engineering

Circuit analysis, power electronics, signal processing, control systems, component datasheets, wiring standards. The core discipline that supports everything from house wiring to PCB design. LibreTexts Engineering and the Electronics Stack Exchange already cover significant ground; the gap is EDA tool documentation, practical component references, and standards-adjacent material.

### Embedded Systems & IoT

Microcontroller documentation (ESP32, RP2040, STM32, AVR), RTOS references (FreeRTOS, Zephyr), communication protocol specs (I2C, SPI, UART, MQTT, Modbus), and firmware development guides. The programming-toolkit plan already sketches an embedded-dev tier with toolchains and firmware images; this pack focuses on the *reference documentation* side.

### 3D Printing & Digital Fabrication

Slicer documentation, filament/resin material guides, printer calibration, design-for-manufacturing rules, model repositories, and tool-specific docs (FreeCAD, OpenSCAD, KiCad). The RepRap wiki is the foundational open knowledge base for FDM printing. Prusa Knowledge Base is excellent but licensing is unclear for redistribution.

### Mechanical & Fabrication

Machining, welding, materials science, fastener references, tolerances, and workshop practice. Machinery's Handbook is the industry bible but is copyrighted; open alternatives are limited to OER welding/machining guides and what LibreTexts covers. The CD3WD and Practical Action libraries (already in core) include some appropriate-technology fabrication.

### CAD & EDA Tool Documentation

FreeCAD, KiCad, OpenSCAD, and LibreCAD offline documentation. These are the open-source design tools most likely to be available on a Svalbard drive. No pre-built ZIMs exist for any of them -- all would need zimit scrapes or static doc builds.

---

## Source Candidates

### Already in Svalbard Recipes

These exist as recipe YAML files and are already included in various presets.

| Source | Recipe ID | Type | Size | Notes |
|--------|-----------|------|------|-------|
| Electronics Stack Exchange | `stackexchange-electronics` | ZIM | ~2.0 GB | Circuit design, components, test equipment, PCB layout Q&A |
| Engineering Stack Exchange | `stackexchange-engineering` | ZIM | ~0.5 GB | Mechanical, civil, electrical engineering Q&A |
| DIY & Home Improvement SE | `stackexchange-diy` | ZIM | ~3.0 GB | Practical wiring, plumbing, structural -- overlaps with fabrication |
| LibreTexts Engineering | `libretexts-engineering` | ZIM | ~0.65 GB | Open textbooks: civil, mechanical, electrical, environmental |
| LibreTexts Physics | `libretexts-physics` | ZIM | ~0.53 GB | Mechanics, electromagnetism, thermo -- foundational for EE |
| LibreTexts Math | `libretexts-math` | ZIM | ~0.8 GB | Calculus, linear algebra, diff eq -- essential engineering math |
| TED 3D Printing | `ted-3d-printing` | ZIM | ~137 MB | Overview talks, not technical reference |
| iFixit | `ifixit` | ZIM | ~5.0 GB | Repair guides with teardowns; strong electronics/mechanical content |
| Low-tech Magazine | `lowtech-magazine` | ZIM | ~0.67 GB | Pre-industrial and sustainable engineering approaches |
| Energypedia | `energypedia` | ZIM | ~0.76 GB | Renewable energy systems, off-grid power |
| Practical Action | `practical-action` | ZIM | ~1.0 GB | Appropriate-tech fabrication, energy, construction |
| Khan Academy | `khan-academy` | ZIM | ~10 GB | Physics and math foundations (video-heavy) |
| Wikibooks EN | `wikibooks-en` | ZIM | ~3.0 GB | Includes electronics, engineering, and programming textbooks |
| Amateur Radio SE | `stackexchange-amateur-radio` | ZIM | ~0.5 GB | RF engineering, antenna design, radio electronics |

### New ZIM Sources (Kiwix Library)

Available on download.kiwix.org but not yet in Svalbard recipes.

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| 3D Printing Stack Exchange | ZIM | ~115 MB | core | 3dprinting.stackexchange.com; printer troubleshooting, materials, slicing. CC BY-SA 4.0 |
| Arduino Stack Exchange | ZIM | ~130 MB | core | arduino.stackexchange.com; AVR/ESP programming, sensor interfacing. CC BY-SA 4.0 |
| Robotics Stack Exchange | ZIM | ~80 MB | extended | robotics.stackexchange.com; actuators, kinematics, ROS. CC BY-SA 4.0 |
| Raspberry Pi Stack Exchange | ZIM | ~285 MB | extended | raspberrypi.stackexchange.com; GPIO, SBCs, Linux embedded. CC BY-SA 4.0 |
| DevDocs C | ZIM | ~5 MB | core | C standard library reference (from devdocs.io). MIT-licensed scrape |
| DevDocs C++ | ZIM | ~7 MB | core | C++ standard library reference. MIT-licensed scrape |

### Candidate Sources Requiring Zimit Scrapes

These are web-based docs that could be crawled into ZIM with zimit. All need license verification for redistribution.

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| RepRap Wiki | zimit scrape | ~200-400 MB | core | reprap.org/wiki; foundational 3D printing knowledge base. GNU FDL |
| FreeCAD documentation | zimit scrape | ~220 MB | core | wiki.freecad.org; parametric CAD for mechanical/3D print design. CC BY 3.0 + LGPL |
| KiCad documentation | zimit scrape | ~50-100 MB | core | docs.kicad.org; schematic capture and PCB layout. GPL-3.0 / CC BY 3.0 |
| OpenSCAD documentation | zimit scrape | ~20-40 MB | extended | en.wikibooks.org/wiki/OpenSCAD_User_Manual + openscad.org docs. CC BY-SA 3.0 |
| Arduino docs (docs.arduino.cc) | zimit scrape | ~100-200 MB | core | Board pinouts, language reference, library API docs. CC BY-SA 4.0 |
| ESP-IDF Programming Guide | zimit scrape | ~150-300 MB | extended | docs.espressif.com; ESP32 full framework docs, API reference, examples. Apache 2.0 |
| Raspberry Pi documentation | zimit scrape | ~50-100 MB | extended | raspberrypi.com/documentation; hardware specs, GPIO, Pico SDK. CC BY-SA 4.0 |
| FreeRTOS documentation | zimit scrape | ~30-50 MB | extended | freertos.org/Documentation; task, queue, semaphore API. MIT |
| MicroPython documentation | zimit scrape | ~20-40 MB | core | docs.micropython.org; quick reference per board, library API. MIT |
| CircuitPython docs | zimit scrape | ~30-50 MB | extended | docs.circuitpython.org; Adafruit's fork with broader board support. MIT |
| LibreCAD documentation | zimit scrape | ~10-20 MB | extended | docs.librecad.org; 2D CAD for mechanical drawings. GPL |

### Candidate Sources -- PDFs and Static References

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| OpenStax University Physics (Vol 1-3) | PDF/EPUB | ~150 MB | core | Mechanics, E&M, optics, modern physics. CC BY 4.0 |
| OpenStax Chemistry | PDF/EPUB | ~80 MB | extended | Materials science foundation. CC BY 4.0 |
| NIST Engineering Statistics Handbook | zimit/PDF | ~50-100 MB | extended | itl.nist.gov/div898/handbook; statistical process control, DOE. Public domain (US Gov) |
| OER Welding Textbooks | PDF | ~20-50 MB | extended | Open Oregon / Fox Valley TC welding guides. CC BY 4.0 |
| Fastener reference tables | CSV/PDF | ~5-10 MB | core | Bolt grades, thread pitches, torque specs. Compile from public sources |
| Wire gauge / ampacity tables | CSV/PDF | ~1-2 MB | core | AWG/metric cross-ref, ampacity by insulation type. Public domain data |
| Common component datasheets | PDF bundle | ~50-200 MB | extended | BME280, SSD1306, ADS1115, NE555, LM7805, etc. Vendor-published, freely available |
| Resistor/capacitor code charts | PDF/HTML | ~1 MB | core | Color code, SMD marking, E-series values. Public domain reference data |
| Machinery's Handbook (older PD edition) | PDF | ~100 MB | reference | Pre-1929 editions are public domain via Internet Archive. Tables remain largely valid for thread data, trig, fits |

### Software Documentation (from Programming Toolkit overlap)

These are documented in the programming-toolkit plan but their reference docs belong here too.

| Source | Type | Size est. | Tier | Notes |
|--------|------|-----------|------|-------|
| PlatformIO documentation | zimit scrape | ~30-50 MB | extended | docs.platformio.org; multi-platform embedded build system. Apache 2.0 |
| Zephyr RTOS documentation | zimit scrape | ~100-200 MB | reference | docs.zephyrproject.org; modern RTOS for resource-constrained devices. Apache 2.0 |
| LVGL documentation | zimit scrape | ~30-50 MB | extended | docs.lvgl.io; embedded GUI library. MIT |
| Protocol specs (I2C, SPI, UART) | PDF | ~10-20 MB | core | NXP I2C spec is freely available; SPI/UART from various vendors |

---

## Tiering Notes

- **32 GB** (~1.5-2 GB for this pack): Electronics SE + Engineering SE + LibreTexts Engineering + 3D Printing SE + Arduino SE + DevDocs C/C++ + MicroPython docs + Arduino docs + wire/resistor/fastener quick-ref tables. Focused on the most bytes-efficient Q&A and language references. Total ~3.5 GB but some sources (Electronics SE, LibreTexts Eng) are already in core/default presets at this tier.
- **128 GB** (~5-8 GB for this pack): Add RepRap Wiki, FreeCAD docs, KiCad docs, OpenStax Physics, ESP-IDF docs, Raspberry Pi SE + docs, Robotics SE, FreeRTOS docs, common datasheets bundle, OER welding guides. This is the maker/engineer sweet spot.
- **512+ GB** (~15-25 GB for this pack): Add iFixit (if not already in core), Khan Academy physics/math (if not in core), Zephyr RTOS docs, PlatformIO docs, NIST statistics handbook, CircuitPython docs, expanded datasheet library, Machinery's Handbook PD edition, full component reference PDFs. Completionist tier.

---

## Relationship to Existing Content

- **Core pack overlap**: Electronics SE, Engineering SE, LibreTexts Engineering, iFixit, Practical Action, and Khan Academy are already in default presets. This pack adds focused engineering supplements rather than duplicating them. Recipe inclusion should use the existing recipe IDs.
- **Communications pack overlap**: Amateur Radio SE and some RF/electronics content from Electronics SE are shared with the Communications pack. That pack owns radio-specific content; this pack owns general EE and embedded systems.
- **Programming toolkit overlap**: The programming-toolkit plan covers *toolchains and compilers* (Zig, Go, Rust, ESP-IDF SDK, Arduino CLI). This pack covers *reference documentation and knowledge* -- the things you read, not the things you run. There is natural overlap in embedded-dev documentation (ESP-IDF, MicroPython, Arduino) where both packs would want the same docs bundled.
- **Sciences pack**: LibreTexts Physics and Math are foundational to engineering but likely belong in a sciences pack. This pack should reference them as dependencies rather than claiming ownership.

---

## Build Complexity Notes

Most of the new-recipe work here falls into three categories:

1. **Trivial** -- new Stack Exchange ZIMs (3D Printing, Arduino, Robotics, Raspberry Pi) follow the exact same recipe pattern as existing SE recipes. Just add YAML files.
2. **Moderate** -- zimit scrapes of documentation sites (FreeCAD, KiCad, Arduino docs, ESP-IDF, RepRap Wiki). These need scope tuning to avoid crawling forums, issue trackers, and unrelated pages. FreeCAD docs at ~220 MB and ESP-IDF at ~300 MB are the largest.
3. **Manual curation** -- PDF bundles (datasheets, OpenStax textbooks, reference tables, welding OER). These need hand-selection, license verification, and packaging into a browsable format (HTML index or small ZIM).

---

## Open Questions

- How much of LibreTexts Engineering is actually EE vs. civil/environmental? If it is mostly non-EE, the pack may need more targeted electrical engineering textbook content.
- Should common component datasheets be bundled as a flat PDF collection, or converted into a searchable HTML/SQLite reference? A curated "top 50 components" datasheet pack would be more useful than a raw dump.
- Is the RepRap Wiki still actively maintained, or has the community moved to other resources? The wiki is large and some content may be outdated (pre-2020 printer designs). A focused subset might be better than a full crawl.
- Should KiCad symbol/footprint libraries be bundled alongside docs? They are CC BY-SA 4.0 and essential for actual PCB design, but add significant size (~1-2 GB with 3D models).
- How to handle the Prusa Knowledge Base -- it is one of the best 3D printing troubleshooting resources, but its license for offline redistribution is unclear (not standard CC, not explicitly restricted either).
- Should OpenSCAD docs come from Wikibooks (already included in `wikibooks-en`) or as a separate targeted scrape? The Wikibooks version may already be sufficient.
- What is the right boundary between this pack and a potential "sciences" pack for physics and math content? Should LibreTexts Physics/Math be listed as dependencies or included directly?
- Would an offline parametric component database (resistor values, capacitor specs, IC pinouts) be worth building as a custom SQLite reference? Nothing like this exists as open data in a structured format.
