# Awesome-list gap scan: offline tools and datasets missing from Svalbard's catalog

Method: I read the catalog index (228 recipes) and the tools-base, computing, communications, engineering and core packs. I built candidates from the categories in awesome-selfhosted, awesome-cli-apps, awesome-go/rust apps, awesome-ham-radio/sdr, awesome-security/ctf, awesome-datahoarding and awesome-public-datasets. Then I checked each one directly against upstream, which is more reliable than the list entries: the GitHub API (`repos/<r>` for license, archived flag and stars; `releases/latest` for tag, date and asset names and sizes), plus HTTP HEAD requests for size and Last-Modified on dataset URLs. All checks ran on 2026-11-26. "Maintained" means a release on or after about 2025-05. Sizes are the release asset sizes (compressed). Anything already in the catalog is left out: kiwix, sqlite3, sqliteviz, duckdb-wasm, cyberchef, age, 7z, fzf, toybox, dufs, uv, go-pmtiles, llama-server, ghidra, hashcat, nmap, nuclei, ffuf and so on.

Platform shorthand: L64 = linux x86_64, LA = linux arm64, M64 = macOS x86_64, MA = macOS arm64.

## (a) Tools: which offline-useful portable apps are missing?

### Takeaway
About 40 maintained tools with prebuilt linux/macOS releases fill clear gaps. The biggest are:
- **Document and media handling:** pandoc, pdf.js, a PDF CLI, ffmpeg, an ebook reader.
- **Local-first knowledge:** TiddlyWiki, SilverBullet.
- **LAN-without-internet:** LocalSend, Syncthing, rclone, Nebula.
- **Backup and secrets:** restic, KeePassXC.
- **Editors and calculators:** neovim/micro, Numbat.
- **Off-grid comms:** Pat Winlink, JS8Call, FreeDV, rtl_433, Reticulum MeshChat, Meshtastic web.
- **Offline speech:** sherpa-onnx.
- **Light reverse engineering and offline vulnerability scanning:** Cutter, yara-x, trivy/grype.

### Cited Findings

**Tier 1: small, single-binary, all four platforms, broad value (recommended for tools-base and core)**

| # | Tool | Version / date | License | Platforms, size | Value | Source |
|---|---|---|---|---|---|---|
| 1 | pandoc | 3.14 / 2026-11-12 | GPL-2.0 | L64 34M, LA 37M, M64 26M, MA 41M (tar.gz/zip) | Convert md/docx/odt/epub/html/LaTeX offline; turns any vault doc into any format | [releases](https://github.com/jgm/pandoc/releases/latest) |
| 2 | ripgrep | 16.0.0 / 2026-11-22 | Unlicense/MIT | all 4, ~1M each | Fast full-text grep over extracted docs, code, datasets | [releases](https://github.com/BurntSushi/ripgrep/releases/latest) |
| 3 | restic | v0.20.0 / 2026-11-19 | BSD-2-Clause | all 4, bz2 single binary 10–12M | Encrypted, deduplicated backups to the stick or a LAN disk | [releases](https://github.com/restic/restic/releases/latest) |
| 4 | Syncthing | v2.1.7 / 2026-10-21 | MPL-2.0 | L64 13M, LA 12M, macOS amd64 14M / arm64 12M / universal 26M | Peer-to-peer folder sync over LAN with no internet or server | [releases](https://github.com/syncthing/syncthing/releases/latest) |
| 5 | LocalSend (CLI + GUI) | v1.19.0 / 2026-11-24 | Apache-2.0 | CLI: linux x86-64/arm-64 (+musl) ~4M, macOS arm-64 3M / x86-64 4M; GUI: AppImage 58M, dmg 50M | AirDrop-style LAN file transfer between phones and laptops with no internet; apps exist for every phone OS | [releases](https://github.com/localsend/localsend/releases/latest) |
| 6 | KeePassXC (incl. keepassxc-cli) | 2.8.0 / 2026-10-21 | GPL (API: NOASSERTION; upstream GPL-2/3) | AppImage x86_64 49M / aarch64 48M; dmg x86_64 47M / arm64 45M | Offline password vault (KDBX), TOTP, SSH agent; KDBX also opens on phones | [releases](https://github.com/keepassxreboot/keepassxc/releases/latest) |
| 7 | jq | 1.8.2 / 2026-06-20 | MIT (API: NOASSERTION) | jq-linux-amd64/arm64, jq-macos-amd64/arm64; single binary 1–2M | JSON querying for dataset dumps (OFF, CVE, ATT&CK, openFDA) | [releases](https://github.com/jqlang/jq/releases/latest) |
| 8 | pdf.js (dist) | v6.5.438 / 2026-11-01 | Apache-2.0 | platform-neutral static web app, 6M zip | Browser PDF viewer the drive runtime can serve, so Survivor Library scans, MIY.pdf and manuals open on any phone | [releases](https://github.com/mozilla/pdf.js/releases/latest) |
| 9 | pdfcpu | v0.16.4 / 2026-10-27 | Apache-2.0 | all 4, 5–6M tar.xz | PDF split/merge/encrypt/optimize/extract, pure Go | [releases](https://github.com/pdfcpu/pdfcpu/releases/latest) |
| 9b | qpdf (alternative) | v12.4.2 / 2026-09-27 | Apache-2.0 | bin zips: linux x86_64/aarch64, macOS x86_64/arm64, 2–3M | PDF repair, linearize, decrypt | [releases](https://github.com/qpdf/qpdf/releases/latest) |
| 10 | Neovim | v0.12.6 / 2026-10-09 | Apache-2.0/Vim (API: NOASSERTION) | all 4, 9–10M tar.gz | Full editor on the stick | [releases](https://github.com/neovim/neovim/releases/latest) |
| 10b | micro (beginner editor) | v2.0.15 / 2025-12-31 | MIT | linux64-static, linux-arm64, macos-arm64, osx; ~4M | Nano-like editor with mouse and ctrl-S; easier for non-vim users | [releases](https://github.com/micro-editor/micro/releases/latest) |
| 11 | Typst | v0.15.1 / 2026-07-17 | Apache-2.0 | L64/LA musl 15–16M, M64 14M, MA 13M | Offline typesetting (reports, forms, labels); single binary, no TeX tree | [releases](https://github.com/typst/typst/releases/latest) |
| 12 | Numbat | v1.24.0 / 2026-08-23 | MIT/Apache-2.0 | all 4, ~2M | Scientific calculator with physical units and constants (dosage, electrical, unit conversion) | [releases](https://github.com/sharkdp/numbat/releases/latest) |
| 13 | DuckDB CLI | v2.0.1 / 2026-11-20 | MIT | linux amd64/arm64 (+musl) 20–23M, osx amd64/arm64 18–20M, universal 39M | Native SQL over multi-GB CSV/Parquet/JSON (OFF, OSV, FDC), where duckdb-wasm hits browser memory limits | [releases](https://github.com/duckdb/duckdb/releases/latest) |
| 14 | zim-tools (zimdump, zimcheck, zimwriterfs) | 3.8.0 / 2026-08-13 | GPL-3.0 | tarballs linux x86_64/aarch64 (+musl), macOS x86_64/arm64 on download.openzim.org | Inspect and extract ZIMs and build your own ZIMs from HTML offline | [repo](https://github.com/openzim/zim-tools/releases/latest), [downloads](https://download.openzim.org/release/zim-tools/) |
| 15 | monolith | v2.12.0 / 2026-11-02 | 0BSD | CLI: linux x86_64/aarch64 7M, macOS x86_64/aarch64 5–7M | Save any web page as one self-contained HTML file to grow the vault | [releases](https://github.com/Y2Z/monolith/releases/latest) |
| 16 | rclone | v1.76.0 / 2026-11-06 | MIT | linux amd64/arm64 27–30M, osx amd64/arm64 ~34M | Copy, check and sync between disks; `rclone serve webdav/dlna/sftp` serves the stick to LAN devices | [releases](https://github.com/rclone/rclone/releases/latest) |
| 17 | TiddlyWiki | v5.4.1 / 2026-07-10 | BSD-3-Clause (API: NOASSERTION) | single HTML file, 2.55 MB (`https://tiddlywiki.com/empty.html`); no GitHub assets | Zero-install personal wiki or journal in a browser; dufs can serve and save it | [releases](https://github.com/TiddlyWiki/TiddlyWiki5/releases/latest) |
| 18 | SilverBullet | 3.0.1 / 2026-11-26 | MIT | server zips: linux x86_64/aarch64 19–20M, darwin x86_64/aarch64 17–18M; `sb` CLI 3M | Markdown notes/wiki server with offline-capable PWA, for a group knowledge base on the LAN | [releases](https://github.com/silverbulletmd/silverbullet/releases/latest) |

**Tier 2: domain packs (communications, engineering, sciences, maps, ai, cybersec)**

| # | Tool | Version / date | License | Platforms, size | Value | Source |
|---|---|---|---|---|---|---|
| 19 | Pat (Winlink client) | v1.0.1 / 2026-11-01 | MIT | linux amd64/arm64 5–6M tar.gz, darwin_amd64.pkg 11M (no macOS arm64 asset) | Email over HF/VHF radio (Winlink, peer-to-peer); key emergency-comms tool | [releases](https://github.com/la5nta/pat/releases/latest) |
| 20 | JS8Call-improved | v3.0.3 / 2026-07-13 | GPL-3.0 | AppImage x86_64 56M / aarch64 55M, dmg 27M | Weak-signal HF keyboard messaging with store-and-forward relay | [releases](https://github.com/JS8Call-improved/JS8Call-improved/releases/latest) |
| 21 | FreeDV | v2.4.1 / 2026-10-31 | LGPL-2.1 (API: NOASSERTION) | AppImage x86_64 77M / aarch64 76M, dmg 65M | HF digital voice using a laptop sound card | [releases](https://github.com/drowe67/freedv-gui/releases/latest) |
| 22 | Reticulum MeshChat | v2.4.0 / 2026-07-06 | MIT | AppImage 154M, dmg arm64/x64 151–153M | LXMF messaging over Reticulum (LoRa RNode, LAN, packet radio) with no infrastructure | [releases](https://github.com/liamcottle/reticulum-meshchat/releases/latest) |
| 23 | Meshtastic web client | v2.7.2 / 2026-08-09 | GPL-3.0 | `build.tar` static web app | Serve from the stick to configure Meshtastic nodes and chat with them over Web Serial/HTTP | [releases](https://github.com/meshtastic/web/releases/latest) |
| 23b | Meshtastic firmware (embedded pack) | v2.7.26 / 2026-06-24 | GPL-3.0 | per-arch firmware zips (esp32, esp32s3, nrf52840, rp2040…) | Flash or recover LoRa mesh radios offline | [releases](https://github.com/meshtastic/firmware/releases/latest) |
| 23c | MeshCore firmware (alt) | companion-v1.18.0 / 2026-10-15 | MIT | per-board firmware zips | Alternative LoRa mesh firmware | [releases](https://github.com/meshcore-dev/MeshCore/releases/latest) |
| 24 | rtl_433 | 25.12 / 2025-12-12 | GPL-2.0 | small zips: Linux amd64/arm64/armhf, MacOS arm64 (rtlsdr and soapysdr variants) | Decode 433/868/915 MHz sensors (weather stations, TPMS, meters) with an RTL-SDR dongle | [releases](https://github.com/merbanan/rtl_433/releases/latest) |
| 25 | gqrx | v2.17.7 / 2025-05-27 | GPL-3.0 | AppImage x86_64 62M, dmg x86_64 55M / arm64 49M | General SDR receiver GUI (scanner, weather and aviation listening); 18 months old, borderline | [releases](https://github.com/gqrx-sdr/gqrx/releases/latest) |
| 26 | Nebula | v1.12.0 / 2026-10-29 | MIT | linux amd64/arm64 15–16M, nebula-darwin.zip 32M | Encrypted overlay mesh VPN; a lighthouse on the LAN works with no internet | [releases](https://github.com/slackhq/nebula/releases/latest) |
| 27 | sherpa-onnx | v2.0.3 / 2026-11-22 | Apache-2.0 | `*-linux-x64-static`, `*-linux-aarch64-static`, `*-osx-arm64-static`, `osx-universal2` tar.bz2 | Offline speech-to-text, TTS, VAD and speaker ID from prebuilt static binaries; runs Whisper, Kokoro and Piper ONNX models | [releases](https://github.com/k2-fsa/sherpa-onnx/releases/latest) |
| 28 | Avogadro 2 | 2.1.0 / 2026-10-23 | BSD-3-Clause | AppImage x86_64 107M / aarch64 108M, dmg 91–96M | Molecular editor and visualizer (sciences) | [releases](https://github.com/OpenChemistry/avogadrolibs/releases/latest) |
| 29 | Stellarium | v26.3 / 2026-09-28 | GPL-2.0 | AppImage x86_64 551–571M (no linux arm64), macOS zip 527M | Planetarium: star ID, celestial navigation practice, almanac | [releases](https://github.com/Stellarium/stellarium/releases/latest) |
| 30 | Qucs-S | 26.1.1 / 2026-04-26 | GPL-2.0 | AppImage x86_64 41M, dmg 166M (arm64) / 145M (x86_64) | SPICE circuit simulator GUI (engineering) | [releases](https://github.com/ra3xdh/qucs_s/releases/latest) |
| 31 | LibreCAD | v2.2.1.6 / 2026-10-17 | GPL-2.0 (API: NOASSERTION) | AppImage x86_64 79M / aarch64 112M, dmg 88–94M | 2D CAD, DXF drawings | [releases](https://github.com/LibreCAD/LibreCAD/releases/latest) |
| 32 | FreeCAD | 1.1.4 / 2026-09-28 | LGPL-2.1 | AppImage x86_64 782M / aarch64 720M, dmg 619–686M | Parametric 3D CAD; large, so best as an engineering-pack option | [releases](https://github.com/FreeCAD/FreeCAD/releases/latest) |
| 33 | OrcaSlicer | v2.5.2 / 2026-11-21 | AGPL-3.0 | AppImage x86_64 191M / aarch64 187M, mac universal dmg 343M | Offline 3D-print slicer (PrusaSlicer 2.9.7 ships only a dmg on GitHub) | [releases](https://github.com/OrcaSlicer/OrcaSlicer/releases/latest) |
| 34 | BRouter | v1.7.10 / 2026-07-17 | MIT | brouter-1.7.10.zip 6M (Java, needs a JRE plus `.rd5` segment data) | Offline routing for hiking, bike and car; maps pack has no routing yet | [releases](https://github.com/abrensch/brouter/releases/latest) |
| 34b | GraphHopper (alt) | 11.1 / 2026-09-29 | Apache-2.0 | graphhopper-web-11.1.jar 45M (JRE plus OSM .pbf import) | Offline routing server over OSM extracts | [releases](https://github.com/graphhopper/graphhopper/releases/latest) |
| 35 | Cutter (rizin GUI) | v2.5.0 / 2026-06-30 | GPL-3.0 | AppImage x86_64 124–137M, dmg arm64 114M / x86_64 118M | Reverse engineering without Ghidra's host-JDK requirement | [releases](https://github.com/rizinorg/cutter/releases/latest) |
| 36 | yara-x | v1.25.0 / 2026-11-26 | BSD-3-Clause | all 4, 8–9M | Malware and pattern matching (YARA rules) for forensics | [releases](https://github.com/VirusTotal/yara-x/releases/latest) |
| 37 | trivy | v0.76.2 / 2026-11-05 | Apache-2.0 | Linux 64-bit 50M / ARM64 45M, macOS 64-bit 51M / ARM64 47M | Vulnerability, misconfig and secret scanner; runs air-gapped from a pre-fetched DB | [releases](https://github.com/aquasecurity/trivy/releases/latest) |
| 37b | grype (alt) | v0.122.1 / 2026-11-18 | Apache-2.0 | all 4, 28–31M | SBOM/CVE scanner with an offline DB import | [releases](https://github.com/anchore/grype/releases/latest) |
| 38 | gitleaks | v8.30.1 / 2026-03-21 | MIT | all 4, ~7M | Secret scanning in repos and files (trufflehog v3.100.1 is AGPL, 29–77M) | [releases](https://github.com/gitleaks/gitleaks/releases/latest) |
| 39 | ImHex | v1.38.1 / 2025-12-21 | GPL-2.0 | AppImage x86_64 180M / arm64 175M, dmg 39–45M | Hex editor with pattern language (firmware, file formats) | [releases](https://github.com/WerWolv/ImHex/releases/latest) |
| 40 | mitmproxy | 12.3.1 / 2026-10-25 | MIT | standalone tarball linux x86_64 102.5M on downloads.mitmproxy.org (not on GitHub assets) | Interactive HTTPS proxy (CTF, IoT analysis) | [repo](https://github.com/mitmproxy/mitmproxy), [download](https://downloads.mitmproxy.org/12.3.1/mitmproxy-12.3.1-linux-x86_64.tar.gz) |

**Tier 3: worthwhile, but large or overlapping (catalog as optional)**

- Thorium Reader v3.5.1 (2026-09-02), BSD-3: EPUB/PDF/audiobook reader. AppImage x86_64/arm64 134–135M, dmg 124–132M. — [releases](https://github.com/edrlab/thorium-reader/releases/latest)
- KOReader v2026.07.1 (2026-08-01), AGPL-3.0: AppImage x86_64 38M and aarch64; **no macOS build**. — [releases](https://github.com/koreader/koreader/releases/latest)
- Calibre 9.19.0 (2026-11-21), GPL-3.0: linux x86_64 txz 230.2 MB (download.calibre-ebook.com); the only GitHub asset is a 389M dmg. Ebook library, converter and viewer. — [releases](https://github.com/kovidgoyal/calibre/releases/latest), [linux tarball](https://download.calibre-ebook.com/9.19.0/calibre-9.19.0-x86_64.txz)
- Kavita v0.9.1.4 (2026-09-02), GPL-3.0: self-contained ebook, PDF and comic reading server for the LAN. linux x64/arm64 and osx x64/arm64 at 108–112M each. — [releases](https://github.com/Kareadita/Kavita/releases/latest)
- Navidrome v0.64.2 (2026-09-24), GPL-3.0: music server, ~22M on all 4 platforms. — [releases](https://github.com/navidrome/navidrome/releases/latest)
- FFmpeg static (BtbN FFmpeg-Builds daily autobuild, 2026-11-26): linux64 GPL 61M and linuxarm64. **No macOS assets.** The macOS source (e.g. evermeet.cx) was not verified. — [releases](https://github.com/BtbN/FFmpeg-Builds/releases/latest)
- ImageMagick portable 7.1.2-36 (2026-11-16): linux x64/arm64 9–12M and macOS **arm64 only** 8M. ImageMagick license (API: NOASSERTION). — [releases](https://github.com/ImageMagick/ImageMagick/releases/latest)
- qsv 29.0.0 (2026-11-15), MIT/Unlicense (API: NOASSERTION): CSV toolkit. linux x86_64 musl 56M, aarch64 20M; the macOS arm64 zip is 315M. — [releases](https://github.com/dathere/qsv/releases/latest)
- Backup alternatives to restic:
  - kopia v0.23.1 (2026-06-16), Apache-2.0, 14–16M. — [releases](https://github.com/kopia/kopia/releases/latest)
  - borg 1.4.5 (2026-07-18), standalone linux/macOS binaries 14–29M. — [releases](https://github.com/borgbackup/borg/releases/latest)
  - rustic v0.12.0 (2026-11-17), Apache-2.0. — [releases](https://github.com/rustic-rs/rustic/releases/latest)
- Note apps, larger than TiddlyWiki or SilverBullet:
  - Joplin v3.7.24 (2026-11-19, AGPL): AppImage 197M. — [releases](https://github.com/laurent22/joplin/releases/latest)
  - Trilium v0.107.0 (2026-11-08, AGPL-3.0): server tarball linux 82–84M, desktop dmg 185–195M. — [releases](https://github.com/TriliumNext/Trilium/releases/latest)
  - Logseq 2.0.2 (2026-10-07, AGPL-3.0). — [releases](https://github.com/logseq/logseq/releases/latest)
- draw.io desktop v32.7.4 (2026-11-17, GPL-3.0 per API): offline diagramming. AppImage 146–148M, dmg 151–158M. — [releases](https://github.com/jgraph/drawio-desktop/releases/latest)
- Gitea v28.4.0 (2026-11-15), MIT: LAN git forge with wiki and issues, xz 36–40M. Soft Serve is **archived**, so exclude it. — [releases](https://github.com/go-gitea/gitea/releases/latest)
- Terminal utilities, all MIT and all 4 platforms unless noted:
  - gdu v5.39.0, 7–9M. — [gdu](https://github.com/dundee/gdu/releases/latest)
  - bottom 0.15.0, ~1M. — [bottom](https://github.com/ClementTsang/bottom/releases/latest)
  - yazi v26.9.1, 13M, darwin assets confirmed. — [yazi](https://github.com/sxyazi/yazi/releases/latest)
  - glow v3.0.0, 6M; renders Markdown docs in a terminal. — [glow](https://github.com/charmbracelet/glow/releases/latest)
  - zellij v0.46.1, ~30M. — [zellij](https://github.com/zellij-org/zellij/releases/latest)
  - caligula v0.5.0 (2026-05-21), GPL-3.0, 3M; safe disk-image flashing (ISO, SD cards) on linux and macOS. — [caligula](https://github.com/ifd3f/caligula/releases/latest)
- Local AI, overlapping llama-server:
  - koboldcpp v1.125.1 (AGPL): linux-x64-nocuda 105M, mac-arm64 67M. One binary covering LLM, Whisper, SD and TTS. — [releases](https://github.com/LostRuins/koboldcpp/releases/latest)
  - llamafile 0.10.6 (2026-09-15): single APE binary. — [releases](https://github.com/mozilla-ai/llamafile/releases/latest)
  - stable-diffusion.cpp (2026-11-25, MIT): Linux x86_64 29M, macOS arm64 38M only. — [releases](https://github.com/leejet/stable-diffusion.cpp/releases/latest)
  - Piper TTS v1.8.0 (GPL-3.0): ships as Python wheels only (32M) and would install through the existing uv runtime. — [releases](https://github.com/OHF-Voice/piper1-gpl/releases/latest)
  - LibreTranslate v1.10.0 (2026-11-24, AGPL-3.0): Python via uv, uses Argos models for offline translation. — [releases](https://github.com/LibreTranslate/LibreTranslate/releases/latest)

**Checked and rejected (stale, archived, or no linux/mac binaries)**

- whisper.cpp v1.9.6 (2026-11-23): the latest release has **no** linux or macOS binary assets, so it needs a source build. Use sherpa-onnx instead. — [releases](https://github.com/ggml-org/whisper.cpp/releases/latest)
- Direwolf 1.8.1 (2025-11-13): the only asset is `direwolf-1.8.1-a231971_x86_64.zip`, which looks like the Windows build. Linux and macOS need a source build. It is the key APRS/packet TNC, so it is worth a source-build handler later. — [releases](https://github.com/wb2osz/direwolf/releases/latest)
- SDR++: the "nightly" release tag dates from 2022 and has only macOS assets, although commits are active (2026-11-18). — [repo](https://github.com/AlexandreRouma/SDRPlusPlus)
- SatDump: last release 1.2.2 (2024-11-29), macOS dmg only. — [releases](https://github.com/SatDump/SatDump/releases)
- ardopcf: 1.0.4.1.3 (2024-11-01), linux only. — [releases](https://github.com/pflarue/ardop/releases/latest)
- sioyek: v2.0.0 (2022-12-16). — [releases](https://github.com/ahrm/sioyek/releases/latest)
- pspy: v1.2.1 (2023-01-17). — [releases](https://github.com/DominicBreuker/pspy/releases/latest)
- vosk-api: v0.3.50 (2024-04-22). — [releases](https://github.com/alphacep/vosk-api/releases/latest)
- argos-translate: the GitHub "latest" is v1.4.0 (2021). — [releases](https://github.com/argosopentech/argos-translate/releases/latest)
- DB Browser for SQLite: v3.13.1 (2024-10-16). — [releases](https://github.com/sqlitebrowser/sqlitebrowser/releases/latest)
- OpenSCAD: the stable release is still 2021.01. — [releases](https://github.com/openscad/openscad/releases/latest)
- Soft Serve: archived. — [repo](https://github.com/charmbracelet/soft-serve)
- Exploit-DB GitHub mirror: archived. The live repo is on GitLab (see datasets).
- No usable linux/mac GitHub assets:
  - GoldenDict-ng v26.11.0: macOS dmg only (222–238M). — [releases](https://github.com/xiaoyifang/goldendict-ng/releases/latest)
  - GPXSee 16.20: dmg only. — [releases](https://github.com/tumic0/GPXSee/releases/latest)
  - Qalculate 5.13: linux x86_64 tar.xz, no macOS. — [releases](https://github.com/Qalculate/qalculate-gtk/releases/latest)
  - QMapShack V_1.22.0: no assets.
  - tilemaker v3.2.0: no assets.
  - kiwix-desktop 2.5.1: no assets.
  - radare2 6.2.6: windows, wasi, source and xcframework only.
  - Yggdrasil v0.5.15: macOS pkg only.
  - OpenCPN 5.14.2: macOS pkg only.
  - CHIRP: no GitHub releases.

### Inferences
- Highest return for the least work: pandoc, ripgrep, restic, Syncthing, LocalSend CLI, jq, pdfcpu, neovim/micro, Numbat, DuckDB CLI, zim-tools and monolith. All are single static binaries under 40 MB with all four platforms, so they fit the existing `tools/*.yaml` GitHub-release pattern (like age, fzf and dufs) and add under about 250 MB to tools-base.
- pdf.js and TiddlyWiki are platform-neutral static web assets, like cyberchef and sqliteviz. The drive runtime can serve them, which makes PDFs and notes usable from phones with no native app.
- For the communications pack, Pat, JS8Call-improved, FreeDV, rtl_433 and the Meshtastic web client cover the HF, LoRa and SDR gaps behind the pack's "software docs" TODO. Direwolf, the most important APRS piece, needs a source-build path.
- Several GUI apps (Stellarium, FreeCAD, OrcaSlicer, Calibre) are 200–800 MB and ship as AppImage/dmg, not single binaries. They suit optional packs, and the runtime needs an AppImage/dmg launch convention. Stellarium also has no linux arm64 build.
- trivy and grype need their vulnerability DBs pre-fetched to work offline. The DB would be a separate dataset recipe, versioned like nuclei-templates.

### Gaps
- I did not fully scrape every listed awesome list. I built candidates from their categories and verified each one against GitHub directly. Niche entries in awesome-selfhosted or awesome-sdr may still be missed.
- I did not verify:
  - a macOS static source for ffmpeg;
  - KiCad release assets (it releases outside GitHub);
  - CircuitJS/Falstad offline bundle availability (the repos have no GitHub releases);
  - WSJT-X and fldigi (SourceForge);
  - a Jellyfin macOS tarball (the linux `jellyfin_12.4-amd64.tar.gz` exists on repo.jellyfin.org).
- I did not verify whether Qucs-S bundles ngspice in its AppImage/dmg.
- Licenses marked "API: NOASSERTION" come from upstream LICENSE knowledge, not from the API response. Recheck them when writing the recipes.

## (b) Datasets: which bulk offline datasets are missing?

### Takeaway
The catalog's datasets cover only security wordlists and templates and the Fimea registry. The best additions are:
- **Geocoding:** a global gazetteer (GeoNames).
- **Language:** WordNet, Tatoeba.
- **Food and plants:** current food composition (USDA FDC, Open Food Facts), plant and medicinal-plant DBs (WCVP, Dr. Duke's).
- **Chemistry and hazmat:** ChEBI, CODATA, periodic table, NIOSH/ERG/CAMEO.
- **Astronomy and radio:** a star catalog (HYG), the US amateur license DB.
- **Offline security intelligence:** CVE List V5, Exploit-DB, ATT&CK, OSV.
- **Climate and terrain:** Köppen-Geiger, WorldClim, GEBCO.

### Cited Findings

| # | Dataset | Format, size (checked 2026-11-26) | License | Value / pack | Source |
|---|---|---|---|---|---|
| 1 | GeoNames gazetteer | `allCountries.zip` 403 MB, `cities500.zip` 13.3 MB; TSV, rebuilt daily (Last-Modified 2026-11-26) | CC BY 4.0 | Offline place-name search and geocoding for the maps packs (pairs with PMTiles) | [allCountries.zip](https://download.geonames.org/export/dump/allCountries.zip) |
| 2 | USDA FoodData Central (full) | CSV zip ~458 MB (3.1 GB unzipped), 12/2025 release | Public domain / CC0 | Current nutrient composition; supersedes the catalog's 2015-era `usda-nutrition` ZIM | [FDC downloads](https://fdc.nal.usda.gov/download-datasets) |
| 3 | Open Food Facts | `en.openfoodfacts.org.products.csv.gz` 1.47 GB (2026-11-23); full `openfoodfacts-products.jsonl.gz` 13.2 GB | ODbL (DB), DbCL (contents) | Barcode → ingredients, allergens, nutrition for global packaged foods; query with DuckDB | [CSV](https://static.openfoodfacts.org/data/en.openfoodfacts.org.products.csv.gz) |
| 4 | Open English WordNet | 2025-edition (2025-12-31): JSON/XML/TTL, 9–19 MB | CC BY 4.0 | Offline thesaurus and definitions in a tiny structured form | [releases](https://github.com/globalwordnet/english-wordnet/releases/latest) |
| 5 | Tatoeba sentences | `sentences.tar.bz2` 209.8 MB (2026-11-21) | CC BY 2.0 FR | Multilingual sentence pairs (includes Finnish); offline phrasebook and language learning | [download](https://downloads.tatoeba.org/exports/sentences.tar.bz2) |
| 6 | Dr. Duke's Phytochemical & Ethnobotanical DB | `Duke-Source-CSV.zip` plus data dictionary (Ag Data Commons, DOI 10.15482/USDA.ADC/1239279) | CC0 | Medicinal plant uses and chemistry (homesteading, survival-medical) | [catalog](https://catalog.data.gov/dataset/dr-dukes-phytochemical-and-ethnobotanical-databases) |
| 7 | WCVP (Kew World Checklist of Vascular Plants) | `wcvp.zip` 84.1 MB (Last-Modified 2026-06-04) | CC BY (per Kew; not re-verified) | Global plant names and distribution for ID and foraging cross-reference | [wcvp.zip](https://sftp.kew.org/pub/data-repositories/WCVP/wcvp.zip) |
| 8 | CVE List V5 | `2026-11-26_all_CVEs_at_midnight.zip` 643 MB (daily release) | CVE Terms of Use (permissive) | Offline CVE lookup for cybersec-reference | [releases](https://github.com/CVEProject/cvelistV5/releases/latest) |
| 9 | Exploit-DB (+ searchsploit) | GitLab repo archive (`exploitdb-main.zip`); last commit 2026-11-25; size not reported by HEAD | GPL-2.0 (repo); individual exploits vary | Offline exploit search; the GitHub mirror is archived, the GitLab repo is live | [GitLab](https://gitlab.com/exploit-database/exploitdb) |
| 10 | MITRE ATT&CK STIX | `mitre-attack/attack-stix-data` (pushed 2026-11-05; ATT&CK v20.1 tag in mitre/cti 2026-11-05) | ATT&CK Terms of Use (API: NOASSERTION) | TTP reference for blue/red team work, offline | [repo](https://github.com/mitre-attack/attack-stix-data), [cti release](https://github.com/mitre/cti/releases/latest) |
| 11 | OSV all-ecosystems | `all.zip` 2.94 GB (2026-11-26) | Mixed per source (mostly CC BY 4.0 / CC0) | Offline dependency vulnerability DB | [all.zip](https://osv-vulnerabilities.storage.googleapis.com/all.zip) |
| 12 | HYG star database | `hygdata_v41.csv` 33.9 MB (v40 gz 13.6 MB) | CC BY-SA 4.0 (API: NOASSERTION) | Star catalog (~120k stars) for celestial navigation and astronomy tools | [repo](https://github.com/astronexus/HYG-Database) |
| 13 | ChEBI | `compounds.tsv.gz` 6.7 MB (2026-11-05); fuller SDF/OWL also on the FTP | CC BY 4.0 | Chemical entities, names, formulas (sciences) | [FTP](https://ftp.ebi.ac.uk/pub/databases/chebi/flat_files/compounds.tsv.gz) |
| 14 | CODATA fundamental constants | `allascii.txt`, a few KB (NIST, 2024-05, CODATA 2022) | Public domain (US gov) | Authoritative constants table | [NIST](https://physics.nist.gov/cuu/Constants/Table/allascii.txt) |
| 15 | Periodic-Table-JSON | small JSON (repo pushed 2026-11-22) | CC BY-SA (API: NOASSERTION) | Element properties for sciences tooling | [repo](https://github.com/Bowserinator/Periodic-Table-JSON) |
| 16 | NIOSH Pocket Guide to Chemical Hazards | PDF (the current official offline form; the ZIP HTML edition existed only around 2004–2009) | Public domain | Exposure limits, PPE, first aid per chemical | [CDC downloadable](https://www.cdc.gov/niosh/npg/downloadable.html) |
| 17 | Emergency Response Guidebook 2024 | PDF (PHMSA; my curl HEAD got 403, likely bot blocking) | Public domain | Hazmat placard → response guide | [PHMSA PDF](https://www.phmsa.dot.gov/sites/phmsa.dot.gov/files/2024-04/ERG2024-Eng-Web-a.pdf) |
| 18 | CAMEO Chemicals (desktop) | Windows/Mac offline app v3.1.0 (May 2024); no linux | Public domain (NOAA/EPA) | Reactivity and hazmat DB, fully offline | [EPA](https://epa.gov/cameo/cameo-chemicals-software) |
| 19 | openFDA drug labels | 13 zipped JSON parts; part 1 = 128 MB (2026-07-10) | CC0 / public domain | Full US drug labeling (indications, dosing, interactions), complementing Fimea | [part 1](https://download.open.fda.gov/drug/label/drug-label-0001-of-0013.json.zip) |
| 20 | Köppen-Geiger 1 km maps (Beck et al. 2023) | `koppen_geiger_tif.zip` GeoTIFF on Figshare | CC BY 4.0 | Climate zones 1901–2099 for crop and homestead planning | [figshare](https://figshare.com/articles/dataset/High-resolution_1_km_K_ppen-Geiger_maps_for_1901_2099_based_on_constrained_CMIP6_projections/21789074) |
| 21 | WorldClim 2.1 bioclim | 10′: 47.6 MB; 2.5′: 627.9 MB (2019 files) | License not verified (see Gaps) | Climate normals (temperature, precipitation) for growing-season planning | [10m](https://geodata.ucdavis.edu/climate/worldclim/2_1/base/wc2.1_10m_bio.zip) |
| 22 | GEBCO global grid | GEBCO_2025: 7.5 GB NetCDF, or eight 90°×90° GeoTIFF/ASCII tiles; GEBCO_2026 also out (~4 GB compressed) | GEBCO terms (free, attribution) | Global elevation and bathymetry (terrain, coastal, sailing) | [GEBCO_2025](https://www.gebco.net/gebco2025-grid) |
| 23 | kaikki.org (wiktextract) | English JSONL 3.27 GB uncompressed; all-language raw `.jsonl.gz` 2.94 GB (2026-10-30) | CC BY-SA 4.0 / GFDL (Wiktionary) | Machine-readable Wiktionary (POS, inflections, translations) for structured lookups; overlaps the Wiktionary ZIMs | [kaikki](https://kaikki.org/dictionary/raw-wiktextract-data.jsonl.gz) |
| 24 | FCC ULS amateur licenses | `l_amat.zip` 183.9 MB (2026-11-22) | Public domain (US gov) | Offline US callsign lookup; US-centric | [FCC](https://data.fcc.gov/download/pub/uls/complete/l_amat.zip) |
| 25 | USDA PLANTS checklist | `plantlst.txt` (2026-03-06) | Public domain | US plant symbols and names; small | [plantlst.txt](https://plants.usda.gov/assets/docs/CompleteChecklist/plantlst.txt) |

Other findings:
- The FoodData Central download page lists the full CSV at about 458M zipped and 3.1G unzipped, dated 12/2025. Branded data regenerates every six months. — [FDC](https://fdc.nal.usda.gov/download-datasets)
- Standard Ebooks bulk zips (`se-ebooks-[collection]-[format].zip`) are for **Patrons Circle members only**, so they can't be fetched without a membership. — [Standard Ebooks bulk](https://standardebooks.org/bulk-downloads)
- NGA says Bowditch (Pub. 9, 2019 edition) is freely downloadable from its MSI portal. I did not confirm a stable direct URL. — [NGA news](https://www.nga.mil/news/NGA_to_update_Bowditch_seeks_public_input.html)
- The NIOSH Pocket Guide is currently offered in print, online, PDF and mobile web app forms. — [CDC](https://www.cdc.gov/niosh/npg/downloadable.html)
- CAMEO Chemicals desktop runs without internet once installed. The current version is 3.1.0. — [EPA CAMEO](https://epa.gov/cameo/cameo-chemicals-software)

### Inferences
- Top five for broad value per MB:
  1. GeoNames `cities500` (13 MB) or allCountries (403 MB) for place search.
  2. Open English WordNet (~10 MB).
  3. USDA FDC (458 MB), which replaces the 2015 ZIM.
  4. HYG (14–34 MB).
  5. CODATA/ChEBI/periodic table (under 10 MB combined).
- For cybersec-reference, CVE V5 (643 MB) plus Exploit-DB plus ATT&CK gives an offline searchsploit/CVE workflow next to nuclei-templates. They change often, so they need the same tagged/dated pinning convention as nuclei-templates and SecLists. OSV (2.9 GB) is optional.
- Open Food Facts and OSV are multi-GB. Queried with the DuckDB CLI rather than duckdb-wasm, they become usable offline.
- Hazmat references (ERG, NIOSH, CAMEO) suit survival-medical or a new hazmat subsection. The ERG and NIOSH PDFs need pdf.js (tool #8) to be phone-readable.
- Tatoeba and Wiktextract both include Finnish, which matches the catalog's Finnish focus. A Finnish-only kaikki extract would be much smaller than the 3 GB English one.

### Gaps
- Not verified:
  - total size of Exploit-DB, ATT&CK STIX, Dr. Duke's zip, the Köppen-Geiger GeoTIFF zip, and openFDA labels (I checked only part 1 of 13: 128 MB; the total is probably ~1.5 GB, inferred);
  - the WorldClim 2.1 license: I did not confirm whether redistribution is allowed;
  - the WCVP license on the current Kew page;
  - stable direct URLs for NOAA World Magnetic Model WMM2025 coefficients (my guessed NCEI URL returned 404), NGA Bowditch Pub. 9 and Pub. 229 sight-reduction tables, and the Argos Translate model packages (the index JSON at raw.githubusercontent.com/argosopentech/argospm-index is reachable; per-model sizes and licenses not checked).
- Other researchers may cover these; I did not assess them: OpenStax textbooks, Hesperian/MSF medical PDFs, ICD-10, RxNorm prescribable subset, DailyMed SPL, OpenCelliD (needs API token), Copernicus DEM.
- Excluded on license grounds:
  - PFAF (Plants For A Future): non-commercial license;
  - FooDB: CC BY-NC;
  - RepeaterBook: restrictive ToS, so there is still no free global repeater DB;
  - RadioReference: proprietary.
