# Apps, viewers, Factbook, Python docs, SDR, speech: follow-up research

Researched 2026-11-26. Versions, dates and asset sizes come from the GitHub releases API (`gh api repos/<r>/releases/latest`), HTTP HEAD `content-length`, or the vendor page linked next to each row. Sizes are MiB (1 MiB = 1,048,576 bytes), rounded down. **[unverified]** marks claims I could not confirm from a primary source.

## 0. How the drive runtime serves content (repo audit)

- `drive-runtime/internal/apps/apps.go` serves the **whole drive root** with Go `http.FileServer` on `127.0.0.1:8083` (or the next free port) and opens `http://localhost:<port>/apps/<name>/` in the host browser. Any static web app copied into `apps/<name>/` therefore runs over HTTP, so `file://` restrictions never apply. `maps.go` and `serveall.go` do the same for `apps/map/` and for the drive root.
- ZIMs go through `kiwix-serve` (`internal/serveall`, `internal/browse`, `internal/search/server`).
- PDFs are catalogued as `type: pdf` (for example `recipes/content/who-basic-emergency-care.yaml`). `internal/inspect/structured.go` groups `.pdf` and `.epub` as "books". No PDF viewer is bundled: the PDF is served raw, and the host browser's built-in viewer renders it.
- Bundled static apps today: `cyberchef`, `sqliteviz`, `duckdb-wasm` and `maplibre-vendor` (`recipes/tools/*.yaml`, `build.family: app-bundle`). Desktop apps (`recipes/apps/ghidra.yaml`) are zip bundles.
- **Platform scope:** `internal/platform/platform.go` detects only `macos-arm64`, `macos-x86_64`, `linux-arm64` and `linux-x86_64`. The runtime does not launch anything on Windows, so Windows portable zips matter only if the stick is used by hand. Native per-platform downloads go under `bin/<platform>/` (`internal/binary/runtimebinary.go`, using the recipe `platforms:` map as in `kiwix-serve.yaml`).

---

## 1. Desktop CAD and making apps

| App | Licence | Latest (date) | macOS | Linux | Windows |
|---|---|---|---|---|---|
| **OrcaSlicer** (FDM slicer) | AGPL-3.0 ([repo](https://github.com/SoftFever/OrcaSlicer)) | v2.5.2 (2026-11-21) [release](https://github.com/OrcaSlicer/OrcaSlicer/releases/tag/v2.5.2) | `OrcaSlicer_Mac_universal_V2.5.2.dmg` 343 MiB | `OrcaSlicer_Linux_AppImage_Ubuntu2404_V2.5.2.AppImage` 191 MiB; `..._aarch64_V2.5.2.AppImage` 187 MiB | `OrcaSlicer_Windows_V2.5.2_x64_portable.zip` 192 MiB (arm64 175 MiB) |
| **OpenSCAD** stable | GPL-2.0 ([COPYING](https://github.com/openscad/openscad/blob/master/COPYING)) | **2021.01** (2021-02-07) [release](https://github.com/openscad/openscad/releases/tag/openscad-2021.01) | `OpenSCAD-2021.01.dmg` 27 MiB (Intel only) | `OpenSCAD-2021.01-x86_64.AppImage` 38 MiB | `OpenSCAD-2021.01-x86-64.zip` 20 MiB |
| **OpenSCAD** dev snapshot | GPL-2.0 | 2026.11.26 (nightly) [files.openscad.org/snapshots](https://files.openscad.org/snapshots/) | `OpenSCAD-2026.11.26.dmg` 70 MiB (Universal, macOS 12+) | `OpenSCAD-2026.11.26-x86_64.AppImage` 80 MiB | `OpenSCAD-2026.11.26-x86-64.zip` 67 MiB |
| **KiCad** | GPL-3.0 ([repo](https://github.com/KiCad/kicad-source-mirror)) | 10.0.8 (2026-11-06) [release](https://github.com/KiCad/kicad-source-mirror/releases/tag/10.0.8) | `kicad-unified-universal-10.0.8.dmg` 1340 MiB | `kicad-10.0.8-x86_64.AppImage.tar` 459 MiB, `-lite` (no 3D models) 301 MiB ([kicad.org/download/linux](https://www.kicad.org/download/linux/), [mirror](https://mirrors.mit.edu/kicad/appimage/stable/)); x86_64 only | installer exe only: `kicad-10.0.8-x86_64.exe` 924 MiB (no portable zip) |
| **FreeCAD** | LGPL-2.1 ([repo](https://github.com/FreeCAD/FreeCAD)) | 1.1.4 (2026-09-28) [release](https://github.com/FreeCAD/FreeCAD/releases/tag/1.1.4) | `FreeCAD_1.1.4-macOS-arm64-py311.dmg` 619 MiB; x86_64 686 MiB | `FreeCAD_1.1.4-Linux-x86_64-py311.AppImage` 782 MiB; aarch64 720 MiB | `FreeCAD_1.1.4-Windows-x86_64-py311.7z` 398 MiB (portable) |
| **LibreCAD** (2D DXF) | GPL-2.0 ([LICENSE](https://github.com/LibreCAD/LibreCAD/blob/master/LICENSE)) | v2.2.1.6 (2026-10-17) [release](https://github.com/LibreCAD/LibreCAD/releases/tag/v2.2.1.6) | `LibreCAD-v2.2.1.6-arm64.dmg` 94 MiB; Intel `.dmg` 88 MiB | `LibreCAD-v2.2.1.6-x86_64.AppImage` 79 MiB; aarch64 112 MiB | `LibreCAD-v2.2.1.6.zip` 66 MiB [unverified: that this zip is the Windows portable build] |
| **SolveSpace** (parametric, tiny) | GPL-3.0 ([repo](https://github.com/solvespace/solvespace)) | v3.3 (2026-10-21) [release](https://github.com/solvespace/solvespace/releases/tag/v3.3) | `SolveSpace.dmg` 12 MiB | none in the release (source tarball only; snap and flatpak elsewhere [unverified]) | `solvespace_x64.exe` 9 MiB (single exe) |
| **PrusaSlicer** | AGPL-3.0 ([repo](https://github.com/prusa3d/PrusaSlicer)) | 2.9.7 (2026-11-02) [release](https://github.com/prusa3d/PrusaSlicer/releases/tag/version_2.9.7) | `PrusaSlicer-2.9.7.dmg` 135 MiB | **no Linux binary** in 2.9.5 to 2.9.7 releases (setup.exe, dmg and zip only) | `PrusaSlicer-2.9.7.zip` 101 MiB |
| **Inkscape** (SVG, laser and vinyl prep) | GPL-2.0+ ([release page](https://inkscape.org/release/)) | 1.4.4 (2026-05-06) [release](https://inkscape.org/release/) | `Inkscape-1.4.4_arm64.dmg` 149 MiB ([dl](https://inkscape.org/gallery/item/59498/Inkscape-1.4.4_arm64.dmg)); Intel dmg also offered | `Inkscape-1.4.4.AppImage` 114 MiB ([dl](https://inkscape.org/gallery/item/59506/Inkscape-1.4.4.AppImage)) | `inkscape-1.4.4_2026-05-05_dcaf3e7-x64.7z` 105 MiB ([dl](https://inkscape.org/gallery/item/59503/inkscape-1.4.4_2026-05-05_dcaf3e7-x64_mHK170m.7z)) |
| **MeshLab** (mesh repair) | GPL-3.0 ([repo](https://github.com/cnr-isti-vclab/meshlab)) | 2025.07 (2025-07-22) [release](https://github.com/cnr-isti-vclab/meshlab/releases/tag/MeshLab-2025.07) | `MeshLab2025.07-macos_arm64.dmg` 38 MiB; x86_64 30 MiB | `MeshLab2025.07-linux_x86_64.AppImage` 82 MiB; aarch64 54 MiB | `MeshLab2025.07-windows_x86_64.zip` 75 MiB |
| **Candle** (GRBL CNC and laser sender) | GPL-3.0 ([repo](https://github.com/Denvi/Candle)) | v11.11 (2026-11-21) [release](https://github.com/Denvi/Candle/releases/tag/v11.11) | `candle-11.11-macos-arm64.dmg` 27 MiB; x64 28 MiB | `candle-11.11.deb` 8 MiB only (not portable; needs system Qt [unverified]) | `candle-11.11-portable.zip` 21 MiB |
| **Universal G-code Sender** | GPL-3.0 ([repo](https://github.com/winder/Universal-G-Code-Sender)) | v2.1.30 (2026-11-09) [release](https://github.com/winder/Universal-G-Code-Sender/releases/tag/v2.1.30) | `macosx-aarch64-ugs-platform-app-2.1.30.dmg` 429 MiB (bundled JRE) | `linux-x64-ugs-platform-app-2.1.30.tar.gz` 243 MiB; aarch64 241 MiB | `win64-ugs-platform-app-2.1.30.zip` 240 MiB |
| **LaserGRBL** (LightBurn alternative) | GPL-3.0+ ([LICENSE.md](https://github.com/arkypita/LaserGRBL/blob/master/LICENSE.md)) | v7.14.1 (2025-03-01) [release](https://github.com/arkypita/LaserGRBL/releases/tag/v7.14.1) | none | none | `install.exe` 7 MiB (Windows-only installer) |

Notes:
- **OpenSCAD has no newer stable than 2021.01.** The download page offers nightly "development snapshots" for macOS (Universal dmg), Windows (exe and zip) and Linux (x86_64 and arm64 AppImage), and says to enable the Manifold engine for "a huge performance boost" ([openscad.org/downloads](https://openscad.org/downloads.html)). The 2021.01 macOS dmg is Intel-only, so Apple Silicon runs it under Rosetta [unverified for 2021.01 specifically]. **Snapshots are kept about one year:** the oldest `.dmg` in the listing is `OpenSCAD-2025.11.20.dmg` ([listing](https://files.openscad.org/snapshots/)), so a pinned snapshot URL rots after about 12 months. A recipe would need re-pinning, or a mirrored copy. The listing also has a 3.2 MiB `OpenSCAD-2026.11.26-WebAssembly-web.zip`.
- The OrcaSlicer repo has moved to `github.com/OrcaSlicer/OrcaSlicer`. The old `SoftFever/OrcaSlicer` path redirects ([release](https://github.com/OrcaSlicer/OrcaSlicer/releases/tag/v2.5.2)).
- Kiri:Moto desktop (Electron) builds exist: `KiriMoto-mac-arm64.dmg` 177 MiB, `KiriMoto-linux-x86_64.AppImage` 184 MiB, Windows exe 119 MiB, release 4.7.3 (2026-08-16) ([release](https://github.com/GridSpace/grid-apps/releases/tag/4.7.3)). The README says the Windows and Mac binaries are not signed ([readme](https://github.com/GridSpace/grid-apps)).

### Recommended essential set (macOS and Linux, since the runtime targets those)

1. **OrcaSlicer**: one FDM slicer that covers Bambu, Prusa, Voron and Creality profiles. It ships a Linux AppImage, which PrusaSlicer no longer does.
2. **OpenSCAD dev snapshot** (pinned date, re-pinned yearly). Small, and it pairs with the MIY archive's `.scad` files.
3. **FreeCAD**: general parametric solid CAD and STEP files.
4. **KiCad** (use the Linux `-lite` AppImage at 301 MiB to save space; the macOS dmg is 1.3 GiB with no lite option).
5. **Inkscape**: SVG and DXF prep for laser, vinyl and plotter work.
6. **Candle**: small GRBL sender for CNC and diode lasers (macOS dmg and Windows zip). On Linux, use **UGS** (bundled JRE, about 240 MiB).
7. Optional: **LibreCAD** (2D DXF), **MeshLab** (STL repair) and **SolveSpace** (12 MiB, no Linux binary).

Rough per-platform total for items 1 to 5 plus Candle or UGS: macOS-arm64 is about 343+70+619+1340+149+27 ≈ **2.5 GiB**; linux-x86_64 is about 191+80+782+301+114+243 ≈ **1.7 GiB**.

---

## 2. Offline browser apps (static folder under `apps/<name>/`)

The runtime serves over HTTP, so "needs an HTTP server" is satisfied for every row below. `file://` matters only if a user double-clicks `index.html`.

| App | Licence | Static build available? | Size | file:// |
|---|---|---|---|---|
| **three.js editor** | MIT ([repo](https://github.com/mrdoob/three.js)) | Yes, no build step. `editor/` imports `../build/three.module.js` and `../examples/jsm/` through an importmap and registers `sw.js` ([editor/index.html @ r187](https://github.com/mrdoob/three.js/blob/r187/editor/index.html)). Copy `editor/`, `build/` and `examples/jsm/` from tag r187 (2026-10-30) ([release](https://github.com/mrdoob/three.js/releases/tag/r187)). | about 1.7 + 6.6 + 8.9 ≈ 17 MiB (git tree sizes) | No: ES modules and the service worker need http (standard browser rule [unverified per-browser]) |
| **JSCAD web** (openjscad.xyz) | MIT ([repo](https://github.com/jscad/OpenJSCAD.org)) | Yes, prebuilt: `packages/web` ships `dist/` and the README's "Use within a Web Site (pre built files)" says to copy `packages/web` plus examples and start a web server ([web README](https://github.com/jscad/OpenJSCAD.org/tree/master/packages/web)). npm `@jscad/web` 2.6.13 ([npm](https://www.npmjs.com/package/@jscad/web)). | about 2.6 MiB (`packages/web`) plus examples | Needs http per README |
| **OpenSCAD playground** (WASM) | GPL-2.0+, deployed as GPL-3.0 ([LICENSE](https://github.com/openscad/openscad-playground/blob/main/LICENSE.md)) | Build only (`npm run build:all`, webpack). It is a PWA that "should not require any internet connectivity once cached". Libraries (BOSL2 and others) are fetched on demand from the bundle ([README](https://github.com/openscad/openscad-playground)). No GitHub releases. | [unverified]; the WASM core alone is 3.2 MiB ([snapshot](https://files.openscad.org/snapshots/)) | Needs http |
| **CascadeStudio** (OpenCascade.js) | MIT ([repo](https://github.com/zalo/CascadeStudio)) | Build only (`npm run build` gives `packages/cascade-studio/dist/`, served with `npx http-server`). Installable PWA "for offline use" ([README](https://github.com/zalo/CascadeStudio)). | [unverified]; OCCT WASM is tens of MiB | Needs http |
| **replicad** studio | MIT ([repo](https://github.com/sgenoud/replicad)) | Monorepo `packages/studio`, build only. npm `replicad` 1.2.1 is 6.4 MiB unpacked ([npm](https://www.npmjs.com/package/replicad)). | [unverified] | Needs http |
| **manifold-3d** / ManifoldCAD.org | Apache-2.0 ([repo](https://github.com/elalish/manifold)) | The `gh-pages` branch holds the built ManifoldCAD editor (`index.html`, `service-worker.js`, `assets/`), but the branch is 368 MiB with docs and dashboards. The editor subset needs trimming. npm `manifold-3d` 3.5.5 is 2.6 MiB ([npm](https://www.npmjs.com/package/manifold-3d)). v3.5.5 (2026-11-13) ([release](https://github.com/elalish/manifold/releases/tag/v3.5.5)). | editor subset [unverified] | Needs http |
| **Kiri:Moto** (browser slicer, CNC, laser) | MIT ([repo](https://github.com/GridSpace/grid-apps)) | Self-hosting runs through the Node `gs-app-server` (`npm run prod`) or Docker ([package.json](https://github.com/GridSpace/grid-apps/blob/master/package.json)). A plain static folder is **not documented** [unverified]. Use the Electron desktop builds (section 1) instead. | 120 to 184 MiB desktop | n/a |
| **Excalidraw** | MIT ([repo](https://github.com/excalidraw/excalidraw)) | Build only. The official Dockerfile runs `yarn build:app:docker` and copies `excalidraw-app/build` into nginx, so the output is a pure static site ([Dockerfile](https://github.com/excalidraw/excalidraw/blob/master/Dockerfile)). Collaboration and share links need backends. Package release v0.18.1 (2026-04-21) ([release](https://github.com/excalidraw/excalidraw/releases/tag/v0.18.1)). | [unverified] | Needs http |
| **draw.io** | Apache-2.0 ([repo](https://github.com/jgraph/drawio)) | Yes: `draw.war` 50 MiB in v32.7.7 (2026-11-25) is a zip of the static webapp ([release](https://github.com/jgraph/drawio/releases/tag/v32.7.7)). Unzip it and open with `?offline=1&stealth=1` (`stealth` "disables all features that require external web services", `offline` disables remote storage) ([URL params](https://www.drawio.com/doc/faq/supported-url-parameters)). Desktop app also available (GPL-3.0, [drawio-desktop v32.7.4](https://github.com/jgraph/drawio-desktop/releases/tag/v32.7.4): `draw.io-universal-32.7.4.dmg` 248 MiB). | 50 MiB zipped | Needs http [unverified] |
| **miniPaint** (Photopea alternative) | MIT ([MIT-LICENSE.txt](https://github.com/viliusle/miniPaint/blob/master/MIT-LICENSE.txt)) | Yes, prebuilt: `dist/bundle.js` (1.2 MiB) is committed and loaded by `index.html` as a classic script. "Nothing will be sent to any server" ([README](https://github.com/viliusle/miniPaint)). v4.14.3 (2026-04-20) ([release](https://github.com/viliusle/miniPaint/releases/tag/v4.14.3)). | about 2 to 5 MiB [unverified] | Probably works from file:// (classic script) [unverified] |

**Pick:** miniPaint, draw.io (`draw.war`), JSCAD web and the three.js editor need no build and can use the existing `app-bundle` family today. OpenSCAD playground, Excalidraw and CascadeStudio need a Node build step in the builder, so they are an M each. They are worth it for the OpenSCAD playground (it overlaps with the desktop OpenSCAD) only if a browser-only CAD path is wanted.

---

## 3. PDF viewers, and whether to ship a browser

### PDF viewers

| Option | Licence | Version (date) | Size | Notes |
|---|---|---|---|---|
| **pdf.js generic viewer** | Apache-2.0 ([repo](https://github.com/mozilla/pdf.js)) | v6.5.438 (2026-11-01) ([release](https://github.com/mozilla/pdf.js/releases/tag/v6.5.438)) | `pdfjs-6.5.438-dist.zip` 6.2 MiB; `-legacy-dist.zip` 6.5 MiB | Unzip to `apps/pdfjs/` and open `web/viewer.html?file=/books/x.pdf` (same-origin). The README says "you need to start a local web server as some browsers do not allow opening PDF files using a file:// URL" ([README](https://github.com/mozilla/pdf.js/blob/master/README.md)). Cross-origin loads are blocked ([FAQ](https://github.com/mozilla/pdf.js/wiki/Frequently-Asked-Questions)), which is fine here because the runtime serves the drive root. The legacy build supports Safari 18+ "mostly" (FAQ). |
| **SumatraPDF** (Windows) | GPL-3.0 ([repo](https://github.com/sumatrapdfreader/sumatrapdf)) | 3.6.1 (2026-04-06) ([release](https://github.com/sumatrapdfreader/sumatrapdf/releases/tag/3.6.1rel)) | `SumatraPDF-3.6.1-64.zip` 9.6 MiB (HEAD 10,031,830 B); arm64 8.9 MiB ([download page](https://www.sumatrapdfreader.org/download-free-pdf-viewer)) | Portable: "single .exe for USB drives; no registry writes" (download page). Windows-only, which the runtime doesn't target. |
| **MuPDF** (mupdf-gl) | AGPL-3.0 or commercial ([mupdf.com/releases](https://mupdf.com/releases)) | Upstream 1.28.6 per Homebrew `mupdf-tools` ([formula](https://formulae.brew.sh/formula/mupdf-tools)) | `mupdf-1.28.5-windows.zip` 89 MiB (HEAD; [archive](https://mupdf.com/downloads/archive/mupdf-1.28.5-windows.zip)); no 1.28.6 Windows zip found | Official binaries are Windows-only. macOS and Linux get MuPDF from package managers, so there is no portable build. |
| **Zathura** | zlib [unverified] | n/a | n/a | Linux GTK app with plugins; no portable build. Skip. |

**Recommendation:** ship **pdf.js** only if the menu needs a uniform in-vault reader (search, outline, the same UI everywhere). Chrome, Edge, Firefox and Safari all render served PDFs natively, so it is optional. It is an S: 6.2 MiB through `app-bundle`, plus a menu link builder for `viewer.html?file=`. Skip SumatraPDF, MuPDF and Zathura (Windows-only or no portable build).

### Should we ship a portable browser?

| Browser | Licence | Version (date) | Portable size |
|---|---|---|---|
| ungoogled-chromium Linux | BSD-3-Clause ([repo](https://github.com/ungoogled-software/ungoogled-chromium-portablelinux)) | 156.0.8078.79-1 (2026-10-31) ([release](https://github.com/ungoogled-software/ungoogled-chromium-portablelinux/releases/tag/156.0.8078.79-1)) | x86_64 AppImage 199 MiB, tar.xz 150 MiB; arm64 AppImage 189 MiB |
| ungoogled-chromium macOS | BSD-3-Clause | 157.0.8103.78-1.1 (2026-11-16) ([release](https://github.com/ungoogled-software/ungoogled-chromium-macos/releases/tag/157.0.8103.78-1.1)) | arm64 dmg 145 MiB, x86_64 154 MiB; notarized "at least till ... October 14th 2027" ([README](https://github.com/ungoogled-software/ungoogled-chromium-macos)) |
| ungoogled-chromium Windows | BSD-3-Clause | 158.0.8127.36-1.1 (2026-11-22) ([release](https://github.com/ungoogled-software/ungoogled-chromium-windows/releases/tag/158.0.8127.36-1.1)) | x64 zip 194 MiB |
| Firefox | MPL-2.0 | 161.0 ([product-details](https://product-details.mozilla.org/1.0/firefox_versions.json)) | Linux `firefox-161.0.tar.xz` 87 MiB ([ftp](https://ftp.mozilla.org/pub/firefox/releases/161.0/linux-x86_64/en-US/)); macOS dmg 160 MiB. Firefox Portable (PortableApps) is Windows-only; the page returned 403 [unverified] |
| Ladybird | BSD-2-Clause ([repo](https://github.com/LadybirdBrowser/ladybird)) | pre-alpha; "first Alpha for macOS and Linux is coming later this year" ([ladybird.org](https://ladybird.org/)); downloadable app planned for the 2027 beta [secondary: [Wikipedia](https://en.wikipedia.org/wiki/Ladybird_(web_browser))] | no binaries |
| NetSurf | GPL-2.0 [unverified] | 3.11 (2023-12-28) ([Wikipedia](https://en.wikipedia.org/wiki/NetSurf)) | No JavaScript [unverified], so it would break every JS app above |
| Lynx / w3m | GPL-2.0-only / w3m licence | lynx 2.9.3, w3m 0.5.6 ([brew lynx](https://formulae.brew.sh/formula/lynx), [brew w3m](https://formulae.brew.sh/formula/w3m)) | Text only; can read kiwix-serve HTML but no apps |
| Browsh | LGPL-2.1 | v1.8.3 (2024-01-29) ([release](https://github.com/browsh-org/browsh/releases/tag/v1.8.3)) | Needs a host Firefox. Stale. |

**Recommendation: don't ship a browser by default.** Every macOS and Linux desktop the runtime supports already has a modern browser, and every served asset (kiwix-serve, apps, PDFs) works in it. A portable Chromium costs 150 to 200 MiB per platform and goes stale in weeks (security). At most, offer ungoogled-chromium as an **opt-in "kiosk/rescue" item** for Linux boxes without a GUI browser. Text browsers are irrelevant because the apps need JS.

---

## 4. CIA World Factbook

- **Discontinued.** The CIA "discontinued ... The World Factbook, as of February 4, 2026" with a notice that it "has sunset" and no reason given ([NBC/AP](https://www.nbcnews.com/politics/politics-news/cia-discontinues-publication-world-factbook-reference-tool-rcna257515), [Engadget](https://www.engadget.com/big-tech/the-cia-stops-publishing-the-world-factbook-184419024.html), [Washington Times](https://m.washingtontimes.com/news/2026/feb/5/end-era-cia-discontinues-popular-world-factbook-reference-site/)). The takedown also removed the historical archives ([Data Rescue Project](https://www.datarescueproject.org/guest-post-the-world-factbook-is-gone/)). **There is no new official edition.**
- **Kiwix ZIM:** `theworldfactbook_en_all_2026-02.zim`, 388 MiB, built 2026-02-06 ([download.kiwix.org/zim/other](https://download.kiwix.org/zim/other/)). Its catalog title is **"CIA World Factbook - 2020"**, summary "Now-discontinued almanac-style information about countries" ([Kiwix OPDS](https://library.kiwix.org/catalog/v2/entries?q=factbook)). The **content is the 2020 edition**, the last year the CIA published full zip archives ([Data Rescue Project](https://www.datarescueproject.org/guest-post-the-world-factbook-is-gone/)). Licence: US-government public domain (17 U.S.C. §105).
- **factbook/factbook.json** (CC0-1.0, [repo](https://github.com/factbook/factbook.json)): weekly auto-updates scraped the live site. The last data commit was "auto-update week 4" on **2026-01-22** (for example `europe/fi.json`). The update workflow has failed since 2026-02-05 ([actions](https://github.com/factbook/factbook/actions)). **It is the most current machine-readable copy (Jan 2026) but frozen.** The README notes the CIA "wiped out completely the World Factbook in Feb 2026."
- **iancoleman/cia_world_factbook_api** (MIT code, [repo](https://github.com/iancoleman/cia_world_factbook_api)): last push 2022-01-01. Stale.
- **simonw/cia-world-factbook-2020**: a browsable 2020 mirror ([site](https://simonw.github.io/cia-world-factbook-2020), [repo](https://github.com/simonw/cia-world-factbook-2020/)).
- **Successors:**
  - **OpenFactBook** ([openfactbook.org](https://openfactbook.org/world-factbook/)) calls itself "community-maintained", built from CIA Factbook, World Bank and REST Countries data, "Public domain under 17 U.S.C. § 105", made by DownSouth (an agency). Its "Data Source" link is factbook.json, so its Factbook core is also frozen at Jan 2026. No bulk download is documented. [Fast Company](https://www.fastcompany.com/91517984/open-factbook-cia-world-factbook)
  - **worldfactbook.io**: launched about 52 days after the shutdown with "Live AI Data" ([Citizen Tribune, paywalled](https://www.citizentribune.com/?p=113632)); search summary says weekly refreshes from World Bank, IMF and UN. Licence and dump: [unverified].
  - The Wayback Machine has 28,000+ snapshots ([Data Rescue Project](https://www.datarescueproject.org/guest-post-the-world-factbook-is-gone/)).

**Recommendation:** add the Kiwix ZIM (388 MiB) labelled "2020 edition, discontinued", or better, a small builder that renders **factbook.json @ the 2026-01-22 commit** (CC0, about 67 MiB repo) into the vault. That is the newest real edition. Don't promise updates: nothing upstream will change. A catalog recipe should pin the commit SHA.

---

## 5. Smaller Python docs

| Source | Size | Date | Notes |
|---|---|---|---|
| `devdocs_en_python_2026-10.zim` (catalog `devdocs-python`) | **4.6 MiB** | 2026-10-31 | [download.kiwix.org/zim/devdocs](https://download.kiwix.org/zim/devdocs/). Current Python 3 reference via DevDocs; searchable in kiwix-serve. |
| `docs.python.org_en_all_2026-11.zim` (zimit) | **3.6 GiB** | 2026-11-18 | [download.kiwix.org/zim/zimit](https://download.kiwix.org/zim/zimit/), [OPDS](https://library.kiwix.org/catalog/v2/entries?q=python). A web crawl, apparently of every version and translation [unverified why it is so large]. |
| `peps.python_en_all_2026-11.zim` | 8.9 MiB | 2026-11-11 | [zimit dir](https://download.kiwix.org/zim/zimit/). |
| Official `python-3.15-docs-html.zip` | 13.7 MiB (tar.bz2 9.3 MiB) | 2026-11-26 | [archives](https://docs.python.org/3.15/archives/), [download page](https://docs.python.org/3/download.html). Full tutorial, HOWTOs and FAQ for one version; static HTML would need an `app-bundle` style serve. No PDFs are offered any more. |
| Official `python-3.15-docs-text.zip` | 4.7 MiB | 2026-11-19 | Same page. Plain text, good for RAG and search indexing. |
| `python-3.15-docs.epub` | 11.3 MiB | 2026-11-19 | Same page. |
| Python cheat-sheet ZIM | none | | The only match is `cheatography.com_en_all_2025-07.zim` at **11 GiB** (all subjects) ([zimit dir](https://download.kiwix.org/zim/zimit/)). No Python-only cheat-sheet ZIM exists. |

**Recommendation:** `devdocs-python` is enough for the reference. The one worthwhile addition is **`peps.python_en_all`** (9 MiB) if PEPs matter. If tutorials and HOWTOs are wanted, the official **HTML zip (14 MiB)** gives full single-version docs at 1/250th the size of the zimit ZIM. Skip `docs.python.org_en_all` (3.6 GiB) and cheatography (11 GiB).

---

## 6. SDR apps

| App | Licence | Latest (date) | macOS | Linux | Windows |
|---|---|---|---|---|---|
| **SDR++** | GPL-3.0 ([repo](https://github.com/AlexandreRouma/SDRPlusPlus)) | Only stable is 1.0.4 (2021-10-17). **Rolling `nightly` release**, assets rebuilt 2026-11-18 by the "Build Binaries" CI (success) ([nightly](https://github.com/AlexandreRouma/SDRPlusPlus/releases/tag/nightly)) | `sdrpp_macos_arm.zip` 4 MiB, `sdrpp_macos_intel.zip` 7 MiB | **debs only** per distro (Debian bookworm, trixie and sid; Ubuntu focal, jammy, noble and resolute; amd64 and aarch64, about 2 MiB). No AppImage. | `sdrpp_windows_x64.zip` 8 MiB |
| **SatDump** | GPL-3.0 ([repo](https://github.com/SatDump/SatDump)) | Stable **1.2.2 (2024-11-29)**, nothing newer ([release](https://github.com/SatDump/SatDump/releases/tag/1.2.2)). `nightly` assets were rebuilt **2026-09-30**; the last commit was 2026-10-22 ([nightly](https://github.com/SatDump/SatDump/releases/tag/nightly)) | 1.2.2: `SatDump-macOS-Silicon.dmg` 49 MiB; nightly 120 MiB | 1.2.2: Ubuntu debs only (about 35 MiB); nightly: `SatDump.AppImage` 401 MiB | 1.2.2: `SatDump-Windows_x64_Portable.zip` 46 MiB; nightly 105 MiB |
| **SDRangel** | GPL-3.0 ([repo](https://github.com/f4exb/sdrangel)) | v8.2.0 (2026-11-13) ([release](https://github.com/f4exb/sdrangel/releases/tag/v8.2.0)) | `sdrangel-8.2.0_mac-15.8.2_arm64.dmg` 512 MiB | flatpak 310 MiB, snap, Ubuntu debs 163 MiB. No AppImage. | `sdrangel-8.2.0-win64.exe` 401 MiB |
| **gqrx** | GPL-3.0 ([repo](https://github.com/gqrx-sdr/gqrx)) | v2.17.7 (2025-05-27) ([release](https://github.com/gqrx-sdr/gqrx/releases/tag/v2.17.7)) | `Gqrx-2.17.7-arm64.dmg` 49 MiB; x86_64 55 MiB | `Gqrx-2.17.7-x86_64.AppImage` 62 MiB | `Gqrx-2.17.7-Windows.zip` 43 MiB |
| **CubicSDR** | GPL-2.0 ([repo](https://github.com/cjcliffe/CubicSDR)) | Last release 0.2.7 (2022-02-05), no assets attached via the API; commits still land (2026-11-25) | | | Stale releases. Skip. |
| **rtl-sdr tools** (rtl_fm, rtl_tcp, rtl_adsb, rtl_power) | GPL-2.0+ ([osmocom](https://github.com/osmocom/rtl-sdr), [brew](https://formulae.brew.sh/formula/librtlsdr)) | osmocom v2.0.3 tag; weekly Windows builds `rtl-sdr-64bit-20261018.zip` 384 KiB ([ftp.osmocom.org](https://ftp.osmocom.org/binaries/windows/rtl-sdr/)). RTL-SDR Blog fork V1.4.0 (2026-08-10), adds V4 and V4L support ([release](https://github.com/rtlsdrblog/rtl-sdr-blog/releases/tag/V1.4.0)) | no official binary (Homebrew `librtlsdr` 2.0.3) | distro packages; build from source for V4 | `Release.zip` 1.2 MiB (blog fork) |
| **readsb** / **dump1090-fa** (ADS-B) | GPL-3.0+ ([readsb](https://github.com/wiedehopf/readsb/blob/dev/LICENSE)) / GPL-2.0+ ([dump1090](https://github.com/flightaware/dump1090/blob/master/LICENSE)) | readsb tag v3.16.21; dump1090 tag v11.1; **no GitHub release binaries** | none | build or apt packages | none |
| **multimon-ng** (POCSAG, APRS, DTMF) | GPL-2.0 ([repo](https://github.com/EliasOenal/multimon-ng)) | 1.6.1 (2026-09-21) ([release](https://github.com/EliasOenal/multimon-ng/releases/tag/1.6.1)) | Homebrew [unverified] | build | `multimon-ng-win64.zip` 0.4 MiB |
| **AIS-catcher** | GPL-3.0 ([repo](https://github.com/jvde-github/AIS-catcher)) | v0.71 (2026-11-13) ([release](https://github.com/jvde-github/AIS-catcher/releases/tag/v0.71)) | build | install script; Fedora RPMs and Docker per release notes | `AIS-catcher.x64.zip` 5.7 MiB (SDRplay support dropped from Windows builds after v0.70) |
| **WSJT-X** | GPL-3.0 ([repo](https://github.com/WSJTX/wsjtx)) | **3.0.2 (2026-06-19)**. Development moved to GitHub ([home](https://wsjtx.github.io/wsjtx/index.html), [release](https://github.com/WSJTX/wsjtx/releases/tag/v3.0.2)) | `wsjtx-3.0.2-arm64-macOS.pkg` 52 MiB (pkg installer, not portable) | `wsjtx-3.0.2-linux-x86_64.AppImage` 83 MiB; aarch64 80 MiB | `wsjtx-3.0.2-win64.exe` 51 MiB |
| **JS8Call** | GPL-3.0 ([repo](https://github.com/js8call/js8call)) | v2.3.1 (2025-06-28) ([release](https://github.com/js8call/js8call/releases/tag/v2.3.1)) | `JS8Call_2.3.1.dmg` 62 MiB | `js8call_2.3.1-x86_64.AppImage` 137 MiB | `js8call-2.3.1-win64.exe` 20 MiB |

**Essential SDR set:**
- **SDR++ nightly** for macOS and Windows (tiny). On Linux, use **gqrx AppImage** as the portable receiver, since SDR++ ships per-distro debs only.
- **rtl-sdr tools** (rtl_fm, rtl_tcp, and rtl_adsb as the ADS-B fallback).
- **SatDump**: nightly AppImage (401 MiB) on Linux, 1.2.2 dmg on Mac (it is two years old, but nightly is the only current build).
- **AIS-catcher** and **multimon-ng**: Windows binaries only; elsewhere they need a build.
- **WSJT-X 3.0.2 AppImage** and **JS8Call AppImage** for HF digital modes.

Skip SDRangel (0.5 GiB, no portable Linux), CubicSDR (stale) and dump1090/readsb (Linux build only). The SDR++ and SatDump nightlies are moving targets: pin by asset hash, not URL.

---

## 7. Offline speech and translation

| Project | Licence | Latest (date) | Prebuilt | Models (en, fi) | Verdict |
|---|---|---|---|---|---|
| **whisper.cpp** | MIT ([repo](https://github.com/ggml-org/whisper.cpp)) | v1.9.6 (2026-11-23); binaries live in the `bNNNN` build releases, latest b5907 (2026-11-20) ([release](https://github.com/ggml-org/whisper.cpp/releases/tag/b5907)) | `whisper-bin-ubuntu-x64.tar.gz` 9.9 MiB, `-ubuntu-arm64` 4.4 MiB, `whisper-bin-x64.zip` 8.7 MiB; macOS only as `xcframework` 64 MiB, so the CLI needs a build | Multilingual (Finnish included): `ggml-base-q5_1.bin` 56 MiB, `ggml-small-q5_1.bin` 181 MiB, `ggml-large-v3-turbo-q8_0.bin` 833 MiB ([HF](https://huggingface.co/ggerganov/whisper.cpp/tree/main)) | **Current. Ship.** Finnish accuracy for small and base is [unverified]. |
| **sherpa-onnx** | Apache-2.0 ([repo](https://github.com/k2-fsa/sherpa-onnx)) | v2.0.3 (2026-11-22) ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/v2.0.3)) | `osx-universal2-shared.tar.bz2` 44.7 MiB, `linux-x64-shared` 28.5 MiB, `linux-aarch64-shared-cpu` 28.1 MiB, Windows MinSizeRel builds | Finnish TTS: `vits-piper-fi_FI-harri-medium-int8.tar.bz2` 20 MiB ([tts-models](https://github.com/k2-fsa/sherpa-onnx/releases/tag/tts-models)); ASR can also run Whisper ONNX | **Current. Ship** as the single TTS engine (no Python), and optionally ASR. |
| **Piper TTS** | MIT, archived ([rhasspy/piper](https://github.com/rhasspy/piper)) → GPL-3.0 ([OHF-Voice/piper1-gpl](https://github.com/OHF-Voice/piper1-gpl)) | Old binaries 2023.11.14-2 (archived repo); piper1-gpl v1.8.0 (2026-09-04), **Python wheels only** (32 MiB) | Old repo: per-platform tarballs, about 18 to 25 MiB | `fi_FI-harri-medium.onnx` 60 MiB, dataset CC0, "Requires Piper 1.5 or higher" ([model card](https://huggingface.co/rhasspy/piper-voices/blob/main/fi/fi_FI/harri/medium/MODEL_CARD)) | Use the voices through sherpa-onnx. Don't ship the stale 2023 binaries. Note the GPL switch. |
| **Vosk** | Apache-2.0 ([repo](https://github.com/alphacep/vosk-api)) | **v0.3.50 (2024-04-22)**, no release since | Python and other bindings | `vosk-model-small-en-us-0.15` 39 MiB (Apache-2.0); **no Finnish model** (languages listed in [model-list.json](https://alphacephei.com/vosk/models/model-list.json); [models page](https://alphacephei.com/vosk/models)) | **Stale, no fi. Skip.** |
| **Argos Translate** | MIT ([repo](https://github.com/argosopentech/argos-translate)) | PyPI 1.11.0 (2026-02-02) ([PyPI](https://pypi.org/project/argostranslate/)); GH release page last 2021 | Python only. Dependencies include `ctranslate2`, `spacy` and `stanza==1.10.1` (stanza pulls in PyTorch [unverified size]) | `translate-en_fi-1_9.argosmodel` 66 MiB and `fi_en-1_9` 66 MiB ([index](https://github.com/argosopentech/argospm-index), index last updated 2026-06-27). Index licence is MIT or CC0. Per-model data licences [unverified]. | **Current. Good en↔fi.** Heavy Python dependency tree. |
| **LibreTranslate** | AGPL-3.0 ([repo](https://github.com/LibreTranslate/LibreTranslate)) | v1.10.0 (2026-11-24) ([release](https://github.com/LibreTranslate/LibreTranslate/releases/tag/v1.10.0)) | Python (Flask) server on top of Argos | Same Argos models | Current. Adds a web UI and API over Argos. Ship only if a browser translation UI is wanted. |
| **Bergamot / Firefox Translations** | MPL-2.0 ([bergamot-translator](https://github.com/browsermt/bergamot-translator), [models repo, archived](https://github.com/mozilla/firefox-translations-models)) | bergamot-translator v0.4.5 (**2022-06-21**), last push 2024; models moved to [mozilla/translations](https://github.com/mozilla/translations) and GCS | WASM worker 4.9 MiB, Linux and macOS Python wheels (2022) | **en-fi and fi-en `base-memory` released**, 31.5 MB uncompressed, 22.6 MB gz ([registry JSON](https://storage.googleapis.com/moz-fx-translations-data--303e-prod-translations-data/db/models.json), generated 2026-11-26) | Models are current and small, but the standalone runtime is stale. An M to L effort to wire a WASM page. Revisit later. |

**Speech and translation recommendation:** ship **whisper.cpp** (ASR, multilingual), **sherpa-onnx + Piper fi_FI-harri and an English voice** (TTS), and **Argos Translate en↔fi** (about 132 MiB of models) through the existing `svalbard-python` and `uv` tooling. LibreTranslate is an optional UI on top. Drop Vosk (stale, no Finnish) and the archived rhasspy/piper binaries. Watch the Bergamot models as a lighter future browser-side translator.

---

## 8. Stale tools verdicts (OpenSCAD, SatDump, SDR++, Vosk, Argos Translate)

Verdict key: **(a)** a maintained fork or successor with current binaries; **(b)** stale but stable, the last release still runs; **(c)** use nightly or CI builds with a pinnable URL.

macOS signatures were checked locally on 2026-11-26 by downloading each artifact and running `codesign -dv`, `spctl -a -vv -t exec` and `lipo -archs` on the `.app`. The artifacts were deleted afterwards.

Note for every row below: Gatekeeper's quarantine check applies to files that carry the `com.apple.quarantine` xattr, which browsers set on download. Whether the host CLI's Go HTTP download sets it, so that ad-hoc-signed apps launch without prompting, is **[unverified: needs an in-vault launch test]**.

### OpenSCAD → **(c) dev snapshots**, with **(a) PythonSCAD** as an optional fork

- Stable is still 2021.01 (2021-02-07). Its macOS dmg predates Apple Silicon builds ([release](https://github.com/openscad/openscad/releases/tag/openscad-2021.01)). Upstream steers users to nightly snapshots and the Manifold engine ([downloads](https://openscad.org/downloads.html)).
- **Snapshot check:** `OpenSCAD-2026.11.26.dmg` (70 MiB) is **notarized Developer ID** (`Marius Kintel (28U8KJ6T2P)`, `spctl`: "source=Notarized Developer ID") and **Universal (x86_64 + arm64)**. Other snapshot assets: `-x86_64.AppImage` 80 MiB and `-x86-64.zip` 67 MiB. A Linux arm64 AppImage is offered only "for Raspberry PI OS 64-bit" per [downloads](https://openscad.org/downloads.html). The newest aarch64 AppImage in the listing is dated 2023-09-11 [unverified whether a current arm64 build lives elsewhere].
- **URL pattern:** `https://files.openscad.org/snapshots/OpenSCAD-YYYY.MM.DD.dmg`, `...-x86_64.AppImage` and `...-x86-64.zip`, each with `.sha256` and `.sha512` sidecars ([listing](https://files.openscad.org/snapshots/)). The URLs are immutable per date, but **retention is about 1 year** (oldest dmg 2025-11-20), so pin a date and re-pin yearly, or mirror.
- **PythonSCAD (fork)**, GPL-2.0 (inherits `COPYING`; [repo](https://github.com/pythonscad/pythonscad), forked from openscad/openscad). It is "a fork of OpenSCAD that stays closely synced with upstream … Everything OpenSCAD can do, PythonSCAD can do too" ([pythonscad.org](https://www.pythonscad.org)). Latest stable **v1.2.0 (2026-11-18)**; v1.2.1 (2026-11-25) is a prerelease ([releases](https://github.com/pythonscad/pythonscad/releases/tag/v1.2.0)). Cadence: v1.0.0 2026-06-02, v1.1.0 to v1.1.2 in July, v1.2.0 in November. Assets:
  - `PythonSCAD-v1.2.0.dmg` 146 MiB, notarized (`Michael Postmann (39F2MHZ52W)`), **arm64 only** (checked)
  - `PythonSCAD-1.2.0-qt6-x86_64-x86_64.AppImage` 130 MiB
  - `PythonSCAD-1.2.0-windows-x86-64.zip` 101 MiB
  - `pythonscad-v1.2.0-wasm.zip` 12 MiB
  - Debian, Ubuntu and Fedora packages, x86_64 only
  - **No Linux arm64 build and no Intel Mac build.**
- **Verdict:** ship the **OpenSCAD dev snapshot** (notarized Universal dmg, x86_64 AppImage, Windows zip), pinned by date plus sha256. PythonSCAD is real and active but narrower on platforms, so offer it only as an optional extra.

### SatDump → **(b) 1.2.2 on macOS and Windows; (c) nightly for portable Linux**

- Stable **1.2.2 (2024-11-29)** is the last release. The only other tags are `-winxp` variants and `nightly` ([tags](https://github.com/SatDump/SatDump/tags), [1.2.2](https://github.com/SatDump/SatDump/releases/tag/1.2.2)). Commits continue (last 2026-10-22).
- **1.2.2 macOS check:** `SatDump-macOS-Silicon.dmg` (49 MiB) is **notarized Developer ID** (`James Vital (B46G593P4D)`) and **arm64**. A separate `SatDump-macOS-Intel.dmg` (52 MiB) exists. Windows has `SatDump-Windows_x64_Portable.zip` 46 MiB and an arm64 portable zip at 36 MiB. **Linux has Ubuntu 22.04/24.04 amd64 and arm64 `.deb` only, no AppImage**, so it is not portable.
- **Nightly:** `https://github.com/SatDump/SatDump/releases/download/nightly/<asset>`. The release body says it is "automatically updated to allow non-logged-in users to download the latest CI builds" ([nightly](https://github.com/SatDump/SatDump/releases/tag/nightly)). The assets were last rebuilt **2026-09-30** and include `SatDump.AppImage` 401 MiB plus macOS, Windows and deb builds. **The URL is mutable**, so pin by sha256 and re-mirror. Nightly macOS signing is [unverified; not checked].
- **Verdict:** use 1.2.2 for macOS (arm64 and Intel, notarized) and Windows. On Linux, use the nightly AppImage (x86_64 only) pinned by hash, because 1.2.2 has no portable Linux build. No fork exists.

### SDR++ → **(c) nightly**

- The only stable release is 1.0.4 (2021-10-17). The README says nightlies "are usually just stable as normal releases" and tells macOS users to "Download the app bundle from the latest nightly build" ([README](https://github.com/AlexandreRouma/SDRPlusPlus)).
- The `nightly` release assets are rebuilt by the "Build Binaries" CI; the last success was **2026-11-18** ([nightly](https://github.com/AlexandreRouma/SDRPlusPlus/releases/tag/nightly)).
- **URL pattern:** `https://github.com/AlexandreRouma/SDRPlusPlus/releases/download/nightly/{sdrpp_macos_arm.zip, sdrpp_macos_intel.zip, sdrpp_windows_x64.zip, sdrpp_<distro>_<codename>_<amd64|aarch64>.deb}`. The URL is mutable, so pin by sha256 and mirror.
- **macOS check:** `sdrpp_macos_arm.zip` holds an `SDR++.app` that is **arm64, ad-hoc signed only** (`Signature=adhoc`, no TeamIdentifier), and `spctl` **rejected** it. If the copy carries the quarantine xattr, the user must clear it (`xattr -dr com.apple.quarantine SDR++.app`) or right-click → Open. See the note at the top of this section.
- **Linux:** per-distro `.deb` packages only (Debian bookworm, trixie and sid; Ubuntu focal, jammy, noble and resolute; amd64 and aarch64). These link against system libraries, so there is **no portable Linux build** [unverified whether `dpkg -x` plus bundled libs is workable].
- **Verdict:** nightly for macOS (arm64 and Intel zips) and Windows. On Linux, fall back to **gqrx 2.17.7 AppImage** (x86_64 only) as the portable receiver.

### Vosk → **(b) stale; prefer the successors sherpa-onnx and whisper.cpp**

- The GitHub `v0.3.50` tag (2024-04-22) has **no binaries** ([release](https://github.com/alphacep/vosk-api/releases/tag/v0.3.50)). The last binary release is **v0.3.45 (2022-12-14)**: `vosk-linux-x86_64-0.3.45.zip` 6.8 MiB, `vosk-linux-aarch64-0.3.45.zip` 2.2 MiB, `vosk-win64-0.3.45.zip` 14.1 MiB ([release](https://github.com/alphacep/vosk-api/releases/tag/v0.3.45)). Commits still land (2026-10-15), but nothing has been released since.
- **PyPI:** latest is `vosk 0.3.45` with `py3-none` wheels for Linux x86_64, aarch64 and armv7l, and Windows amd64. It has **no macOS wheel**; the last macOS wheel is **0.3.44** `py3-none-macosx_10_6_universal2` (2022-09-25) ([PyPI](https://pypi.org/project/vosk/)). It has `requires_python >=3` and depends on `cffi`, `requests`, `tqdm`, `srt` and `websockets`. Because the wheels are ABI-agnostic (`py3-none`, cffi loads a bundled `.so`), Python 3.12 (the bundled `svalbard-python`) and 3.13 should install them [unverified by running].
- There is **no Finnish model** ([model-list.json](https://alphacephei.com/vosk/models/model-list.json)). English small is 39 MiB (Apache-2.0).
- **Verdict:** it works, but it is frozen, macOS is pinned to 0.3.44 and Finnish is missing. If the user insists on Vosk, pin `vosk==0.3.45` (Linux and Windows) and `vosk==0.3.44` (macOS universal2) plus the English small model. The better path is **sherpa-onnx v2.0.3** (2026-11-22), the active project from the k2-fsa (next-gen Kaldi) team, with per-platform CLIs, and/or **whisper.cpp** (section 7).

### Argos Translate → **active on PyPI; ignore the GitHub "latest release"**

- The GitHub "latest release" is **v1.4.0 (2021-08-24)**, an old macOS `.app` zip ([release](https://github.com/argosopentech/argos-translate/releases/tag/v1.4.0)). That is misleading. Tags continue to **v1.11.0**, and PyPI has **argostranslate 1.11.0 (2026-02-02)**, `py3-none-any`, `requires_python >=3.5` ([PyPI](https://pypi.org/project/argostranslate/)). The repo was last pushed 2026-11-17, and **LibreTranslate v1.10.0 (2026-11-24)** builds on it ([release](https://github.com/LibreTranslate/LibreTranslate/releases/tag/v1.10.0)).
- **Dependencies:** `ctranslate2<5,>=4.0`, `sentencepiece`, `spacy`, `stanza==1.10.1`, `sacremoses` and `minisbd`. `ctranslate2 4.9.1` has wheels for cp313 and cp314 on macOS arm64 and x86_64, manylinux x86_64 and aarch64, and Windows amd64 ([PyPI](https://pypi.org/project/ctranslate2/)). `sentencepiece 0.2.3` has cp313 and cp314 wheels. **`stanza 1.10.1` requires `torch>=1.3.0`** ([PyPI](https://pypi.org/project/stanza/1.10.1/)), which makes an offline wheelhouse large: a CPU torch wheel per platform is hundreds of MiB [unverified exact size].
- **Model index:** [argospm-index](https://github.com/argosopentech/argospm-index) (`index.json`, 100 packages, last commit 2026-06-27). It has `translate-en_fi` and `translate-fi_en` **v1.9** (`argos_version 1.9.0`) at `https://argos-net.com/v1/translate-{en_fi,fi_en}-1_9.argosmodel`, 66 MiB each (HEAD-checked). Licence: index MIT or CC0. Per-model data licences are [unverified].
- **Verdict:** active and current. Ship through `svalbard-python` (3.12) with a pinned offline wheelhouse per platform plus the two `.argosmodel` files, pinned by sha256. Size it as M because of the torch wheelhouse; check whether `stanza` can be skipped, since sentence splitting has moved to `minisbd` [unverified].
