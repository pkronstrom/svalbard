# Svalbard: new portable offline tools and apps (research, 2026-11-21)

Read-only research. No repo edits. Release data comes from `gh api repos/<r>/releases/latest`, PyPI JSON,
go.dev, nodejs.org and ziglang.org indexes, checked 2026-11-21 09:40–09:50 EET. "Verified" means the exact
URL returned HTTP 200 (HEAD), or the asset appears in the GitHub release API listing.

Effort: **S** = plain binary or static-app recipe · **M** = also needs an action or menu integration
(port, data dir, model wiring) · **L** = needs builder or runtime work, or a data pipeline.

## Context checked first (to avoid duplicates)

- Existing tools and apps (`recipes/tools`, `recipes/apps`): 7z, age, angr, binwalk, caido, chisel, cyberchef,
  duckdb-wasm, dufs, fzf, go-pmtiles, gobuster, feroxbuster, ffuf, httpx, katana, kiwix-serve, ligolo-ng,
  linpeas, winpeas, llama-server, maplibre-vendor, naabu, nmap-static, nuclei, platformio + the ESP-IDF
  toolchain set, pwntools, rustscan, sqlite3, sqliteviz, sqlmap, subfinder, svalbard-python, toybox, uv,
  volatility3, zstd; apps ghidra, goose, hashcat, opencode, pi.
- **Already proposed in `docs/plans/2026-09-30-catalog-update-audit.md` §3 (not repeated here):** ripgrep,
  jq, yq, duckdb CLI, pandoc, rclone, restic, arduino-cli, meshtastic CLI, syncthing, croc, ffmpeg, yt-dlp,
  tealdeer, micro, rtl_433. The audit skipped whisper.cpp, Piper, GDAL and qsv (status of the first two has
  changed, see #1 and the rejected list).
- **Earlier deferrals** (`docs/plans/2026-03-30-deferred-sources-and-tools.md`): VeraCrypt, TestDisk/PhotoRec
  ("out of scope"), par2, QGIS, Meshtastic ("limited value without radios"), NomadNet ("python deps"),
  CHIRP/Dire Wolf/fldigi/JS8Call. Valhalla was recommended there for routing but deferred because of the
  tile pipeline.
- MASTERPLAN wishlist: Kolibri, notes/wiki (FlatNotes was named in the dodo triage), JupyterHub, offline
  routing (follow-up queue), non-ZIM document indexing (follow-up queue).
- **Platform reality:** `host-cli/internal/toolkit/toolkit.go` `supportedPlatforms` lists only macos-arm64,
  macos-x86_64, linux-arm64 and linux-x86_64. No recipe has a Windows key. The Windows column below is for
  information only.
- **Archive formats:** the Go resolver (`drive-runtime/internal/binary/runtimebinary.go`) extracts
  `.tar.gz/.tgz/.tar.bz2/.zip/.gz` but **not `.tar.xz`**. The shell helper (`recipes/actions/lib/binaries.sh`)
  does handle `.tar.xz` through the system `tar`, and `7z.yaml` already ships `.tar.xz`. Builder `extract`
  steps take only `.zip/.tar.gz`. Other formats go through `exec` steps (see `hashcat.yaml`). Candidates
  that ship only as `.tar.xz` (Typst, Helix, Zig) are marked "xz" below.

---

## Top-5 shortlist

| # | Candidate | Why it is first | Pack | Effort |
|---|---|---|---|---|
| 1 | **sherpa-onnx** 2.0.2 | One Apache-2.0 binary set, 4/4 platforms, gives offline **speech-to-text and text-to-speech**: Parakeet-TDT-0.6B-v3 (25 European languages **including Finnish**), Whisper, Piper voices (Finnish `fi_FI-harri` is available), Kokoro. It supersedes the audit's "whisper.cpp needs a build handler" and "Piper skipped". | ai (new `ai-speech`) | M |
| 2 | **SilverBullet** 2.12.1 | Single-binary Markdown notes and wiki (MIT), 4/4 platforms, about 10–12 MB. Pages are plain `.md` files in a folder on the stick, so they stay greppable and svalbard search could index them. Offline-first PWA usable over the LAN. Covers the merged notes/wiki wishlist item. | tools-base / new `apps` | M |
| 3 | **LocalSend CLI** 1.18.2 | New in 1.18 (first CLI release). 3–4 MB, Apache-2.0, 4/4 platforms. Sends and receives files from the stick to the LocalSend app that is common on phones (iOS/Android), with no browser and no internet. Complements dufs, which is browser-based. | tools-base (sharing) | S (M for a menu action) |
| 4 | **Dev toolchains: Go 1.27.4, Node.js 26.13.1 LTS, Zig 0.17.1** | Official tarballs, 4/4 platforms, permissive licenses, about 53–71 MB each. The coding harnesses that already ship (opencode, pi, goose) can write code, but on a bare host nothing can build or run it. Zig also gives a C/C++ cross-compiler (`zig cc`). | new `dev-toolchains` (computing tier) | S each (Zig is xz) |
| 5 | **Valhalla via pyvalhalla** 3.9.3 | Unblocks the "offline routing" follow-up. The PyPI wheels now **bundle the C++ `valhalla_service` and `valhalla_build_tiles` executables** for linux x64/arm64, macOS arm64 and Windows, so it fits the existing `python-package` recipe type. MIT. The work that remains is the tile build and the map-viewer hook. | fi-maps / maps | L |

Honourable mention: **#6, speech-to-text through the llama-server that already ships.** `/v1/audio/transcriptions`
was merged in llama.cpp PR #21863 (2026-04-14) and tested with gemma-4-E4B-it, which svalbard already ships
text-only. Adding the 560 MB `mmproj` file plus a `--mmproj` flag gives STT with no new binary.

---

## Full ranked list

| # | Name | Latest | Platforms (mac-a / mac-x / lin-x / lin-a / win) | License | Size per platform | Pack | Dup check | Effort | URL verified |
|---|---|---|---|---|---|---|---|---|---|
| 1 | sherpa-onnx | v2.0.2 (2026-11-19) | ✓ ✓ ✓ ✓ ✓ | Apache-2.0 | 28–44 MB + models | ai-speech | new | M | y |
| 2 | SilverBullet | 2.12.1 (2026-10-12) | ✓ ✓ ✓ ✓ ✓ | MIT | 10–12 MB | tools-base/apps | new (wishlist) | M | y |
| 3 | LocalSend CLI | v1.18.2 (2026-08-21) | ✓ ✓ ✓ ✓ ✓(GUI zip) | Apache-2.0 | 3–4 MB | tools-base | new | S/M | y |
| 4 | Go / Node / Zig | 1.27.4 / 26.13.1 / 0.17.1 | ✓ ✓ ✓ ✓ ✓ | BSD-3 / MIT / MIT | 71 / 58–63 / 53–59 MB | dev-toolchains | new | S | y |
| 5 | Valhalla (pyvalhalla) | 3.9.3 (2026-11-17) | ✓ ✗ ✓ ✓ ✓ | MIT | 24–35 MB + tiles | fi-maps | deferred 2026-03; blocker is gone | L | y (PyPI) |
| 6 | llama-server audio (Gemma-4-E4B mmproj) | llama.cpp ≥ Apr 2026 | ✓ ✓ ✓ ✓ | Apache-2.0 (model) | +560 MB (Q8_0) | ai | extends an existing model | M | y |
| 7 | Typst | v0.15.1 (2026-07-17) | ✓ ✓ ✓ ✓ ✓ | Apache-2.0 | 13–16 MB | computing | new (pandoc is in audit) | S (xz) | y |
| 8 | Meshtastic Web client | v2.7.2 (2026-08-09) | static | GPL-3.0 | 2 MB | communications | Meshtastic deferred; CLI in audit | S | y |
| 9 | draw.io webapp | v32.7.4 (2026-11-17) | static | Apache-2.0 | 48 MB | tools-base | new | S | y |
| 10 | pdf.js viewer | v6.5.438 (2026-11-01) | static | Apache-2.0 | 6 MB | tools-base | new | S/M | y |
| 11 | stable-diffusion.cpp (sd-cli, sd-server) | master-1106 (2026-11-16) | ✓ ✗ ✓ ✗ ✓ | MIT | 27–36 MB + 2–7 GB model | ai | new | M | y |
| 12 | KeePassXC | 2.8.0 (2026-10-21) | dmg dmg AppImage AppImage zip | GPL-2/3 | 45–49 MB | tools-base (security) | new | M/L | y |
| 13 | code-server | v4.146.0 (2026-11-20) | ✓ ✓ ✓ ✓ ✓ | MIT | 235–257 MB | dev-toolchains | new | M | y |
| 14 | Kolibri | v0.19.5 (2026-07-13) | wheel (py3.10–3.14) | MIT | 98 MB + channels (GBs) | wishlist / education | wishlist | L | y |
| 15 | marimo notebooks | 0.25.2 (2026-10-19) | pure-Python wheel | Apache-2.0 | small (+ deps) | computing / sciences | new (JupyterHub wishlist) | S/M | y (PyPI) |
| 16 | mitmproxy standalone | 12.3.1 (2026-10-25) | ✓ ✓ ✓ ✓ ✓ | MIT | ~102 MB | cybersec | new | S | y |
| 17 | ImHex Web | v1.38.1 (2025-12-21) | static WASM | GPL-2.0 | 9 MB | cybersec-ctf | new | S | y |
| 18 | Diagnostics set: gdu, bottom, fastfetch, lnav, trippy | various, 2025–26 | ✓ ✓ ✓ ✓ (lnav: no mac-x) | MIT / MIT / MIT / BSD-2 / Apache-2.0 | 1–40 MB each | tools-base | new | S | y |
| 19 | yazi (file manager TUI) | v26.9.1 (2026-09-01) | ✓ ✓ ✓ ✓ ✓ | MIT | 11–14 MB | tools-base | new (fzf exists) | S | y |
| 20 | glow (Markdown reader TUI) | v3.0.0 (2026-08-11) | ✓ ✓ ✓ ✓ ✓ | MIT | 5–6 MB | tools-base | new | S | y |
| 21 | Helix editor | 25.07.1 (2025-07-18) | ✓ ✓ ✓ ✓ ✓ | MPL-2.0 | 14–15 MB | dev-toolchains | micro is in audit | S (xz) | y |
| 22 | rust-analyzer (+ other LSPs) | 2026-11-16 | ✓ ✓ ✓ ✓ ✓ | MIT/Apache-2.0 | 13–14 MB `.gz` | dev-toolchains | new | S | y (API) |
| 23 | Reticulum stack: rns, lxmf, nomadnet | 1.6.1 / 1.3.1 / 1.5.1 | pure-Python | Reticulum License (custom) | < 1 MB + crypto deps | communications | NomadNet deferred 2026-03 | S/M | y (PyPI) |
| 24 | gitleaks | v8.30.1 (2026-03-21) | ✓ ✓ ✓ ✓ ✓ | MIT | 7 MB | cybersec | new | S | y |
| 25 | Datasette | 0.65.6 (2026-11-15) | pure-Python | Apache-2.0 | small | data / tools-base | partly overlaps sqliteviz | S/M | y (PyPI) |

(mac-a = macOS arm64, mac-x = macOS x86_64, lin-x = Linux x86_64, lin-a = Linux arm64.)

---

## Candidate details

### 1. sherpa-onnx (speech in and out) — ai-speech, M
- **What and why:** k2-fsa's ONNX-runtime speech toolkit. The `bin/` folder includes `sherpa-onnx-offline` (file
  ASR), `sherpa-onnx-offline-tts`, `sherpa-onnx-vad-with-offline-asr`, `sherpa-onnx-offline-websocket-server`,
  speaker diarization, punctuation and denoise. One binary covers STT and TTS. A drive action such as
  "transcribe this recording" or "read this article aloud" could run it, and an MCP tool could expose it to
  the agent harnesses.
- **Assets (verified 200):** `https://github.com/k2-fsa/sherpa-onnx/releases/download/v{V}/sherpa-onnx-v{V}-{plat}-shared.tar.bz2`
  with plat = `linux-x64` (28 MB), `osx-arm64` (20 MB), `osx-x64`, `osx-universal2` (44 MB). Linux arm64 breaks
  the pattern: `sherpa-onnx-v{V}-linux-aarch64-shared-cpu.tar.bz2` (28 MB). The `-static` variants are 235–537 MB, so prefer
  `-shared`. `.tar.bz2` is already handled by the Go resolver.
- **Models (verified in the `asr-models` and `tts-models` release tags):** `sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2`
  464 MB (multilingual ASR incl. Finnish; model license CC-BY-4.0), `vits-piper-fi_FI-harri-medium-int8.tar.bz2`
  20 MB, `kokoro-multi-lang-v1_0.tar.bz2` 333 MB. Check each Piper voice's own license in its model card.
- **Effort:** binary recipe S. Model recipes need a new type or a generic archive type. The action/menu
  for recording/file input and output needs M work.
- **Dup check:** nothing in the catalog. Audit §3 skipped whisper.cpp and Piper; this replaces both.

### 2. SilverBullet — notes/wiki, M
- **What and why:** Markdown notes and personal wiki served from one binary (v2 is a compiled Deno server).
  Live queries, offline-first PWA. The data is a folder of `.md` files, so it can live in the vault
  (`notes/`) and be backed up, grepped or indexed. A better fit than Memos (SQLite storage) or FlatNotes
  (Python/Docker).
- **Assets (verified):** `https://github.com/silverbulletmd/silverbullet/releases/download/{V}/silverbullet-server-{darwin-aarch64|darwin-x86_64|linux-x86_64|linux-aarch64|windows-x86_64}.zip`.
  The `sb-*.zip` assets (2–5 MB) appear to be a separate CLI client.
- **Effort:** recipe S. Menu entry "Notes", port allocation, space dir on the stick and an optional auth flag
  for LAN mode make it M.
- **Alternatives:** LeafWiki v0.14.2 (MIT, Go, raw binaries 43–46 MB for all 4 platforms, Markdown on disk;
  newer and smaller community); Memos 26.11.1 (MIT, 18–20 MB, all platforms; SQLite storage); TiddlyWiki
  5.4.1 single HTML (`https://tiddlywiki.com/archive/empty/Empty-TiddlyWiki-5.4.1.html`, verified 200; BSD-3).
  TiddlyWiki can save through a PUT/WebDAV saver, which could pair with dufs `--allow-upload`.

### 3. LocalSend CLI — sharing, S/M
- **What and why:** the 1.18.0 release notes say "first release of the CLI. It uses the same Rust library as the
  Flutter app." It interoperates with LocalSend apps on phones and desktops over the LAN (multicast discovery,
  HTTPS, checksums). For a vault this means pushing a ZIM, APK, map or document to a phone, or receiving
  photos and notes back, without a browser upload flow.
- **Assets (verified):** `https://github.com/localsend/localsend/releases/download/v{V}/LocalSend-CLI-{V}-{linux-x86-64|linux-arm-64|macos-arm-64|macos-x86-64}.tar.gz`
  (3–4 MB). There is no Windows CLI asset; the Windows GUI zip exists.
- **Effort:** S as a binary. Add M for a "Send to phone" or "Receive" menu action. Subcommand syntax was not
  verified.

### 4. Go, Node.js, Zig toolchains — dev-toolchains, S each
- **Go 1.27.4** `https://go.dev/dl/go{V}.{darwin|linux}-{amd64|arm64}.tar.gz` (verified, 71 MB). BSD-3.
- **Node.js 26.13.1 LTS ("Lithium")** `https://nodejs.org/dist/v{V}/node-v{V}-{darwin|linux}-{arm64|x64}.tar.gz` (verified, 58–63 MB). MIT.
- **Zig 0.17.1** `https://ziglang.org/download/{V}/zig-{x86_64|aarch64}-{linux|macos}-{V}.tar.xz` (verified, 53–59 MB). MIT.
  Upstream moved off GitHub; use the ziglang.org index (`/download/index.json`). xz only.
- **Why:** pairs with the coding harnesses and llama-server ("write me a tool, then build it here"). These are
  `toolchain`-type recipes like the ESP ones. Offline module and package caches (GOMODCACHE, npm cache) are a
  separate, later problem.

### 5. Valhalla via pyvalhalla — routing, L
- **What and why:** the PyPI `pyvalhalla` 3.9.3 wheels (`cp312-abi3`, so they match svalbard-python 3.12) ship the
  native executables. `python -m valhalla valhalla_build_tiles -c valhalla.json finland-latest.osm.pbf` and
  `valhalla_service` both work from the wheel. That removes the "no binaries" half of the 2026-03 deferral.
- **Platforms:** macOS arm64 (macosx_15), Linux x86_64 and aarch64 (manylinux_2_28), Windows. **No macOS
  x86_64 wheel.**
- **Remaining L work:** a host-side builder that turns Geofabrik Finland into tiles (likely 1–2 GB, not
  measured), a drive action that starts `valhalla_service` on a port, and a routing UI hook in the MapLibre
  viewer. This is the design decision listed in the MASTERPLAN follow-up.

### 6. STT through the existing llama-server (Gemma-4-E4B mmproj) — ai, M
- PR #21863 "server: support OAI /v1/audio/transcriptions API" was merged 2026-04-14 and tested with
  gemma-4-E4B-it. `docs/multimodal.md` lists audio models (Gemma 4, Ultravox, Qwen2-Audio, Voxtral).
- svalbard's `gemma-4-e4b-it.yaml` says "text-only runtime" and no recipe references an mmproj. Add
  `mmproj-gemma-4-E4B-it-Q8_0.gguf` from `ggml-org/gemma-4-E4B-it-GGUF` (560 MB, verified) or unsloth
  `mmproj-F16.gguf` (990 MB), then pass `--mmproj`. This also enables image input (OCR, describe-a-photo).
- This is cheaper than #1 if Gemma E4B is already selected (8 GB tier), but it gives no TTS, and transcription
  quality from a 4B LLM is likely below Parakeet.

### 7. Typst — computing, S (xz)
`https://github.com/typst/typst/releases/download/v{V}/typst-{aarch64-apple-darwin|x86_64-apple-darwin|x86_64-unknown-linux-musl|aarch64-unknown-linux-musl}.tar.xz` (verified).
A modern typesetting system that works fully offline with fonts embedded. Release assets also include
`typst-documentation.pdf` (25 MB), which works as offline docs. Complements pandoc, which is already in the audit.

### 8. Meshtastic Web client — communications, S
`https://github.com/meshtastic/web/releases/download/v{V}/build.tar` (verified, 2 MB static SPA). Use a builder
`download` step, then `exec tar` (like hashcat's `7z` step). This is the tool you need once you have radios,
and it addresses the "limited value without radios" deferral at almost no size cost. Caveat: Web Serial and
Web Bluetooth need a secure context, so they work on `localhost` only. The HTTP connection to WiFi-enabled
nodes works from any LAN browser.

### 9. draw.io webapp — tools-base, S
`https://github.com/jgraph/drawio/releases/download/v{V}/draw.war` (verified, 48 MB). A `.war` is a zip:
download to `{workdir}/draw.zip` and `extract`. Diagramming fully in the browser. Run it with offline/stealth
URL flags; check that it makes no CDN calls.

### 10. pdf.js viewer — tools-base, S/M
`https://github.com/mozilla/pdf.js/releases/download/v{V}/pdfjs-{V}-dist.zip` (verified, 6 MB). The same
PDF viewer with in-document search on every LAN client (some mobile browsers do not render PDFs inline).
`viewer.html?file=` links fit the "non-ZIM document indexing" follow-up.

### 11. stable-diffusion.cpp — ai, M
`https://github.com/leejet/stable-diffusion.cpp/releases/download/master-{N}-{sha}/sd-master-{sha}-bin-{Darwin-macOS-26.7.2-arm64|Linux-Ubuntu-26.04-x86_64}.zip`
(verified). Archives contain `sd-cli`, `sd-server` and the ggml CPU-variant `.so` files. **Only macOS arm64 and
Linux x86_64** (plus a Vulkan Linux build); there is no Linux arm64 or macOS x86_64. Rolling `master-*` tags
need pinning by tag. Model recipes add 2–7 GB each.

### 12. KeePassXC — security, M/L
2.8.0: `KeePassXC-{V}-{x86_64|aarch64}.AppImage`, `KeePassXC-{V}-{arm64|x86_64}.dmg`, `KeePassXC-{V}-x64.zip`
(verified the AppImage). It is the classic portable password vault, and `keepassxc-cli` is included. Extracting
from a `.dmg` on a Linux host is the hard part (7z can usually read HFS+ dmgs; this needs testing). An AppImage
needs FUSE on the host, so it should be extracted with `--appimage-extract` at build time.

### 13. code-server — dev, M
`https://github.com/coder/code-server/releases/download/v{V}/code-server-{V}-{macos|linux}-{arm64|amd64}.tar.gz`
(verified, 235–257 MB, Node bundled). VS Code in the browser, served from the stick to LAN clients. Extensions
need VSIX pre-staging, for example from Open VSX; rust-analyzer ships `.vsix` assets.

### 14. Kolibri — education, L
v0.19.5 `kolibri-{V}-py2.py3-none-any.whl` (98 MB; requires_python <3.15; classifiers 3.10–3.14, so it fits
svalbard-python 3.12). Install as a `python-package`, then `kolibri start --port`. The L part is pre-seeding
`KOLIBRI_HOME` with channel databases and content (`kolibri manage importchannel/importcontent`) on the host,
which takes several GB. This is on the wishlist.

### 15. marimo — notebooks, S/M
`marimo==0.25.2` pure-Python wheel. Reactive notebooks served locally (`marimo edit --headless --port`); it
fits the existing `python-package` recipe type. It covers most of the JupyterHub wishlist need without a hub.
JupyterLite (`jupyterlite-core` 0.8.7) would need a build step that bundles Pyodide (M/L).

### 16. mitmproxy — cybersec, S
`https://downloads.mitmproxy.org/{V}/mitmproxy-{V}-{linux-x86_64|linux-aarch64|macos-arm64|macos-x86_64}.tar.gz`
(all four verified 200; linux-x86_64 is 102 MB). PyInstaller single-file binaries `mitmproxy`, `mitmdump` and
`mitmweb`. The current web pack has caido but no scriptable intercepting proxy.

### 17. ImHex Web — cybersec-ctf, S
`https://github.com/WerWolv/ImHex/releases/download/v{V}/imhex-{V}-Web.zip` (verified, 9 MB). The pattern-language
hex editor as a static WASM app, served like CyberChef. Desktop builds are dmg, Windows zip and Arch packages
only, so the web build is the portable route.

### 18. Diagnostics set — tools-base, S
- gdu v5.39.0, disk usage: `gdu_{darwin_arm64|darwin_amd64|linux_amd64_static|linux_arm64}.tgz` (verified).
- bottom 0.15.0, system monitor: `bottom_{aarch64|x86_64}-{apple-darwin|unknown-linux-musl}.tar.gz`.
- fastfetch 2.71.0, hardware/system info: `fastfetch-{linux-amd64|linux-aarch64}.tar.gz`, `fastfetch-macos-universal.tar.gz` (verified).
- lnav v0.14.1, log navigator: `lnav-{V}-linux-musl-{x86_64|arm64}.zip`, `lnav-{V}-aarch64-macos.zip` (no macOS x86_64).
- trippy 0.13.0, traceroute/ping TUI: `trippy-{V}-{x86_64|aarch64}-{apple-darwin|unknown-linux-musl}.tar.gz` (verified).
- btop has no macOS asset (Linux musl only), so prefer bottom.

### 19–22. Terminal quality of life — S
- **yazi** v26.9.1 `yazi-{aarch64|x86_64}-{apple-darwin|unknown-linux-musl}.zip` (verified): a fast file manager
  for browsing the vault over SSH or a TTY.
- **glow** v3.0.0 `glow_{V}_{Darwin|Linux}_{arm64|x86_64}.tar.gz` (verified): renders the Markdown docs the drive
  ships (for example `embedded-getting-started.md`).
- **Helix** 25.07.1 `helix-{V}-{aarch64|x86_64}-{macos|linux}.tar.xz` (verified): a modal editor with built-in
  LSP and tree-sitter, with no plugins to fetch. The last release is 2025-07, though the repo is active. xz only.
- **rust-analyzer** `rust-analyzer-{aarch64|x86_64}-{apple-darwin|unknown-linux-gnu}.gz`: a bare `.gz` file,
  which the Go resolver already handles. Pair it with gopls (built with Go), clangd and others when the dev
  pack exists.

### 23. Reticulum stack (rns, lxmf, nomadnet) — communications, S/M
Pure-Python wheels: `rns` 1.6.1, `lxmf` 1.3.1, `nomadnet` 1.5.1. The `python-package` recipe type now exists,
which removes the 2026-03 "python deps" deferral. Off-grid encrypted messaging over LAN, WiFi or LoRa (RNode).
The release also ships `Reticulum.Manual.pdf` (6 MB) for the reference side. **License caveat:** the
"Reticulum License" is a custom, non-OSI license with use restrictions, so it needs a review before
redistributing. Reticulum MeshChat (MIT) exists only as Electron AppImage/dmg (about 150 MB), so it is a poor fit.

### 24. gitleaks — cybersec, S
`gitleaks_{V}_{darwin_arm64|darwin_x64|linux_x64|linux_arm64}.tar.gz` (verified, 7 MB). Secret scanning for loot,
dumps and repos. trufflehog (AGPL, 29–76 MB) is the heavier alternative.

### 25. Datasette — data, S/M
`datasette==0.65.6` (Apache-2.0, pure Python). A multi-user, read-only SQLite browser plus JSON API over the
LAN for the open-data datasets. It overlaps sqliteviz (single-user, in-browser) but adds server-side faceting
and an API that agents and MCP could call.

---

## Considered and not recommended (or lower priority)

| Candidate | Reason |
|---|---|
| whisper.cpp (b5907, rolling) | Now has `whisper-bin-ubuntu-{x64,arm64}.tar.gz` with `whisper-cli`, `whisper-server` and `parakeet-cli`, but still **no macOS CLI** (xcframework only). sherpa-onnx (#1) covers all 4 platforms. |
| piper1-gpl / `piper-tts` 1.8.0 | Wheels exist for all platforms (GPL-3), but sherpa-onnx runs the same Piper voices, so it would be redundant. |
| Argos Translate 1.11 | Depends on `stanza` (pulls in torch) and `spacy`, so roughly 1 GB per platform. translateLocally has no current GitHub release. LLM translation through the existing llama-server is the pragmatic path. |
| Excalidraw | No release bundle (0.18.1 has no assets); needs a source build (L). |
| Soft Serve | Repo **archived** (2026-10). |
| Gitea 28.4.0 | Works (raw binaries 110–118 MB, all platforms, MIT), but an offline git forge is niche for the vault. |
| SDR++ | Only a 2022 "nightly" tag with macOS and Windows zips; no Linux tarball. |
| TestDisk/PhotoRec 7.2 | Linux x86_64 static tarball only (macOS 404). Deferred earlier as out of scope. |
| VeraCrypt | Needs admin and FUSE; deferred earlier; age already covers file encryption. |
| rizin 0.9.1 / radare2 6.2.6 | Linux is static x86_64 only (131 MB) or `.deb`; macOS is `.pkg`. Ghidra and angr already cover this. (radare2 does have a `wasi-browser.zip`.) |
| pspy v1.2.1 | Linux-only static, last release 2023. Fine as a 1–2 MB add-on to linpeas if wanted. |
| btop | No macOS assets; bottom (#18) replaces it. |
| mpv / VLC | No portable macOS + Linux release assets. Browsers already play media served by dufs and kiwix. |
| Meilisearch / tantivy | Would duplicate the runtime's own search engine. |
| ripgrep-all | AGPL; its adapters need poppler and ffmpeg binaries. ripgrep is already in the audit. |
| Reticulum MeshChat | About 150 MB Electron AppImage/dmg. |
| tilemaker v3.2.0 | No release assets in the API listing. Only useful as a host-side map builder (Phase 6 "pending map builders"); go-pmtiles and planetiler-style pipelines cover it. |

## Cross-cutting notes for whoever implements

1. **`.tar.xz` in the Go resolver:** `runtimebinary.go` has no xz case. `host-cli/go.mod` already pulls
   `github.com/ulikunitz/xz` indirectly. One small runtime change would unlock Typst, Helix, Zig and the rizin
   static build, and would let the Go path resolve `7z` (that recipe currently relies on the shell helper's
   `tar xJf`).
2. **New recipe shapes this list would introduce:** speech/diffusion model archives (`.tar.bz2` model dirs, not
   GGUF); an `mmproj` companion on GGUF recipes (#6); "server app with a data dir on the stick" (SilverBullet,
   Kolibri, marimo, Datasette, Valhalla), which needs one shared pattern for port, data path and menu.
3. **Windows:** most candidates publish Windows assets, but svalbard's `supportedPlatforms` excludes Windows
   today, so adding Windows keys would be wasted until that changes.

Sources: GitHub releases API for each repo named above; PyPI JSON (pyvalhalla, piper-tts, sherpa-onnx, kolibri,
marimo, datasette, rns, lxmf, nomadnet, argostranslate, ctranslate2); go.dev/dl, nodejs.org/dist/index.json,
ziglang.org/download/index.json; Hugging Face API (ggml-org/gemma-4-E4B-it-GGUF, unsloth/gemma-4-E4B-it-qat-GGUF,
ggml-org/Voxtral-Mini-3B-2507-GGUF, rhasspy/piper-voices); llama.cpp PR #21863 and `docs/multimodal.md`.
