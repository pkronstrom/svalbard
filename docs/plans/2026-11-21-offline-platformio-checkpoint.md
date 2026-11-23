# Offline PlatformIO checkpoint

Status: done 2026-11-21; real offline acceptance, final review and verification complete.

Scope: MASTERPLAN Phase 5, following the [design](2026-10-07-offline-platformio-design.md) and [implementation](2026-10-07-offline-platformio-implementation.md). Chunks 1–3 were verified 2026-10-08 (`.work/handoffs/pio-*.md`); this checkpoint records Chunk 4.

## Resulting user path

1. `svalbard add esp32-dev` then `svalbard apply` acquires the pinned closure through normal recipes: PlatformIO Core 6.2.0, espressif32 7.1.3, framework-espidf 4.60100.0 (ESP-IDF 6.1.0), Xtensa and RISC-V toolchains 15.2.0+20251204, esp32ulp 2.38.0, tool-esptoolpy 2.41100.260830 (esptool 4.11.0), CMake 3.30.2, Ninja 1.13.2, SCons, ROM ELFs, GDBs and the bundled Python trees. 17 items, 1.5 GB for macOS arm64.
2. `./run embedded` prepares an immutable host-cache generation (5.3 GB) from the vault's archives without network or host Python, then opens bash/zsh/fish with the drive environment applied after user startup files. The documented example copies out and builds for ESP32, ESP32-S2 and ESP32-S3.
3. Reopening reuses the ready generation. A moved vault prepares a new generation and keeps the old one. Cancellation removes the incomplete generation, publishes no ready marker and leaves no processes; retry prepares normally.
4. An unsupported `$SHELL` is rejected before preparation. The cache root is canonicalized, and a cache under macOS `/tmp` is rejected with advice (see limits).

## Verification evidence

- Fresh disposable macOS arm64 vault provisioned by `svalbard add esp32-dev` + `svalbard apply` built from the working tree: 17/17 realized, no manual layout.
- Packaged `run embedded` with a fresh `SVALBARD_PIO_CACHE`, fresh `HOME`, `env -i`, a PATH with every host `python*`, `pip*`, `pio`, `platformio` and `uv` removed, under `sandbox-exec` denying network (an in-shell `curl` to the registry failed; `python3` was not on PATH): preparation installed Core, platform and every package from local `file://` archives; `pio run`, `-e esp32s2`, `-e esp32s3` succeeded (46 s, 39 s, 45 s); nonempty `firmware.elf`/`firmware.bin` for all three (147,600 / 142,416 / 174,208-byte bins); `pio pkg exec -p tool-esptoolpy -- esptool.py version` → 4.11.0. Preparation plus three builds: 3 min 18 s.
- Rerun on the same vault and cache selected the same generation and rebuilt ESP32 in 3 s (11 s including shell start). Relocating the vault selected a new generation; the ESP32 build passed; both generations stayed ready.
- SIGINT to the bootstrap during preparation: run exited non-zero, the incomplete generation was removed, no `.ready`, no surviving processes. Retry on the same cache published exactly one ready generation. SIGINT to the outer runtime during preparation hashing likewise left no generation or process. Outer-runner reaping of a TERM-ignoring child during bootstrap remains covered by `TestRunResolvedActionContextReapsChildProcessGroup`.
- After the final source fixes, the vault was re-applied with the rebuilt CLI; the packaged runtime contained the fixes, rejected `SHELL=/bin/sh` in 0.05 s, and rebuilt ESP32-S3 offline successfully.
- Final whole-feature spec and correctness review: no blocking finding. Two low findings fixed (shell check before preparation; absolute drive root in `Run`); others recorded below.
- After the final source changes: `GOFLAGS="-count=1 -timeout=4m" make verify` passed (catalog sync check, 53 tested packages across all five Go modules); `go test -race ./internal/embedded ./internal/shell ./cmd/svalbard-drive` in drive-runtime passed; `make build` passed (host CLI plus macOS/Linux arm64/x86_64 runtimes); scoped `git diff --check` passed. The tree also contains another session's in-progress builder edits, which compiled and passed at that moment.

## Correction 2026-11-23

The acceptance above piped its commands into `run embedded`, so it never read from a terminal. Typed into a real terminal, the same command printed "Embedded dev shell ready" and then accepted no input: the command-line path ran the action in a background process group, which the terminal stops on its first read. The menu entry was not affected. Fixed in `drive-runtime/cmd/svalbard-drive/main.go` ([plan](archive/2026-11-23-run-tty-handoff-implementation.md)) and re-checked on a freshly applied `esp32-dev` vault in tmux: the shell opens, `pio --version` reports 6.2.0, and `exit` returns to the calling shell.

## Found during acceptance

espressif32 7.1.3's `builder/frameworks/espidf.py` (`compile_source_files`) tests framework membership with resolved paths but computes object paths from the unresolved source path. With a cache under a symlinked path, or under macOS `/private/tmp` (CMake always rewrites it to `/tmp`), app and bootloader objects collide ("Two environments with different actions were specified for the same target"). The runtime now resolves the cache root and refuses macOS `/tmp` with a clear message; the default user cache is unaffected.

## Scope and limits

Execution acceptance covers macOS arm64 on local disk. Linux and macOS x86_64 provisioning is strict but runtime execution there, physical sticks, power loss, upload/monitor and board behavior are unverified. The registry mirror fallback was exercised against real upstream on 2026-10-07 and not repeated; this apply downloaded from canonical URLs. Review notes not fixed: mirror fallback covers connection failures and 5xx but not a mid-body reset (rerun resumes); the ready marker is not fsynced (power loss); every shell open re-hashes the archive closure (slow on USB 2 sticks); each vault mount path costs a 5.3 GB host generation that is never collected. Arduino, other MCU families, managed components and standalone `idf.py` stay outside the closure.

All pre-existing uncommitted work is retained. No staging, commits or Git metadata edits were performed.

## Appendix: acceptance harness

Run with `S=<scratch dir>`, and `V`, `CACHE` and `H` pointing at directories outside `/tmp`. `CMDS` overrides the commands piped into the shell.

```bash
# Phase 5 acceptance: fresh cache, host Python/PIO excluded from PATH, OS networking denied.
set -u
S=${S:?}; V=${V:-$S/p5-vault}; CACHE=${CACHE:-$S/p5-cache}; H=${H:-$S/p5-home}
HB=$S/hostbin; rm -rf "$HB"; mkdir -p "$HB" "$H"
for d in /bin /usr/bin /usr/sbin /sbin; do for f in "$d"/*; do n=$(basename "$f")
  case $n in python*|pip*|pio|platformio|pydoc*|2to3*|idle*|uv|uvx) continue;; esac
  [ -e "$HB/$n" ] || ln -s "$f" "$HB/$n"; done; done
PROFILE='(version 1)(allow default)(deny network*)(allow network* (local unix))'
CMDS=${CMDS:-'cp -R "$DRIVE_ROOT/tools/platformio/examples/blink" "$HOME/blink-$$" && cd "$HOME/blink-$$" && echo "python3=$(command -v python3) pio=$(command -v pio)" && (curl -sS -m 5 https://registry.platformio.org >/dev/null 2>&1 && echo NETWORK_OPEN || echo NETWORK_DENIED) && pio run && pio run -e esp32s2 && pio run -e esp32s3 && ls -la .pio/build/*/firmware.elf .pio/build/*/firmware.bin && pio pkg exec -p tool-esptoolpy -- esptool.py version; echo "BUILD_EXIT=$?"'}
printf '%s\nexit\n' "$CMDS" | env -i HOME="$H" USER="$USER" TERM=dumb LANG=en_US.UTF-8 SHELL=/bin/bash PATH="$HB" \
  SVALBARD_PIO_CACHE="$CACHE" sandbox-exec -p "$PROFILE" "$V/run" embedded
echo "RUN_EXIT=$?"
```
