# Drive Toolkit — Go Binary Rewrite

**Status:** Requirements gathering
**Date:** 2026-04-02

## Goal

Replace `run.sh` and the shell script actions in `.svalbard/` with a single
portable Go binary. One static binary per platform, no shell dependency, works
on Mac/Linux/Windows without a POSIX shell.

## Requirements

### Existing functionality (from current run.sh + actions/)

- [ ] Interactive menu (browse, search, maps, chat, apps, share, verify)
- [ ] Platform detection (OS, arch, select correct binary paths)
- [ ] Binary resolution (find bundled tools by name + platform)
- [ ] Port management (find free ports, avoid conflicts)
- [ ] Process management (track child PIDs, cleanup on exit)
- [ ] Kiwix server launch (browse ZIM content)
- [ ] Search server launch (FTS5 + semantic search)
- [ ] Map tile server launch (PMTiles via go-pmtiles)
- [ ] LLM chat launch (llama-server + model selection)
- [ ] App serving (static HTML apps like CyberChef, SQLiteViz via dufs)
- [ ] File sharing (dufs file server with upload/download)
- [ ] Drive verification (checksum validation)

### New: Embedded development support

- [ ] Set `PLATFORMIO_CORE_DIR` to `$DRIVE/tools/platformio` — point PlatformIO
      at the stick's pre-cached packages directory so that `pio run`,
      `pio run --target upload`, and `pio device monitor` all work offline
- [ ] Decompress toolchains on first use if stored compressed (zstd) — extract
      to host temp dir or user-specified location, cache the extraction path
- [ ] Serial port detection — list available `/dev/tty*` or `COM*` devices for
      upload target selection
- [ ] Submenu: "Embedded Dev" with options for build, flash, monitor, new project
- [ ] Environment setup: export `PATH` additions for bundled toolchains so that
      `pio` and `esptool.py` resolve without host installs

### Cross-cutting

- [ ] Compressed asset extraction — generic support for zstd-compressed tools
      and toolchains, with extraction caching (extract once, reuse on subsequent
      runs)
- [ ] Drive manifest awareness — read drive config for host_platforms, installed
      packs, content inventory
- [ ] Self-update check — compare binary version against what's on the stick

## Architecture notes

- Single `main.go` with subcommands (or just a TUI menu as default)
- Static binary, no CGO — cross-compile for all host platforms from CI
- Embed default config and templates via `go:embed`
- Use bubbletea or similar for the TUI menu (consistent with current rich
  terminal experience)

## Open questions

- Should the Go binary subsume `svalbard` (the Python provisioner) or stay
  separate? Likely separate — the Go binary is the drive-side runtime, Python
  CLI is the host-side provisioner.
- Ship as `svalbard` on the drive, or a different name to avoid confusion with
  the Python CLI? (`svb`? `run`? `drive`?)
- How to handle the LLM chat integration — shell out to llama-server, or embed
  a Go inference runtime?
