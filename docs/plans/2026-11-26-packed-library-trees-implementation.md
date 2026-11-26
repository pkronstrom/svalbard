# Packed library trees: implementation plan

Status: parked, 2026-11-26. Owner decision: keep library trees as loose files for now and recommend small exFAT clusters (32 KB or 4 KB) when preparing a stick. Packing and unpacking on the host may come back later only as an option the end user is asked about. The measurements below stay valid.

Status: plan, 2026-11-26. Implements chunks 1 and 2 of the [design](2026-11-26-packed-library-trees-design.md) under the owner decisions in its section 6. No repository file was written. Scratch is cleaned: `/Users/pkronstrom/.cache/svalbard-scratch/trees/` is empty, and the two measurement directories under `$TMPDIR` and `~/Library/Caches` are removed.

**Later phases, named only:** design chunk 3 (pack the interpreter and the two PlatformIO Python trees) and design chunk 4 (serve browser apps from their zip).

## 1. Measurements

Run on this machine: macOS 15.8.3 arm64, M4 Max, local SSD, load average 5–8 from other jobs, archive in the page cache. Open WebUI 0.13.5 slim for Python 3.12: 19,532 files, 553.5 MB, 209.5 MB zip without Mac metadata, unpacked by a Go program with 8 writers.

### The cost of the contained default

One start is: unpack, import 17 compiled packages (584 modules), remove. Times are seconds for starts 1 / 2 / 3.

| Unpack target | Path | Unpack | First import | Start total |
|---|---|---|---|---|
| scratch (`~/.cache`) | new random | 1.25 / 1.16 / 1.19 | 6.15 / 5.58 / 5.64 | 7.58 / 6.79 / 6.88 |
| scratch | same fixed | 1.19 / 1.25 / 1.19 | 5.64 / 5.55 / 5.63 | 6.87 / 6.84 / 6.86 |
| `$TMPDIR` | new random | 1.57 / 1.43 / 1.54 | 5.63 / 5.77 / 5.62 | 7.24 / 7.27 / 7.23 |
| `$TMPDIR` (load rose to 8) | same fixed | 1.69 / 1.92 / 2.11 | 6.78 / 7.38 / 6.46 | 8.53 / 9.37 / 8.63 |
| `~/Library/Caches` | new random | 1.16 / 1.16 / 1.15 | 5.68 / 5.58 / 5.59 | 6.89 / 6.78 / 6.79 |
| `~/Library/Caches` | same fixed | 1.22 / 1.19 / 1.12 | 5.50 / 5.73 / 5.66 | 6.77 / 6.96 / 6.82 |

- **The check repeats on every unpack, and a fixed path does not avoid it.** `XprotectService` used 5.3–5.6 CPU-seconds in every start (6.2–7.0 in the loaded series).
- **Same tree, no new unpack:** a second import took 0.17–0.19 s. Compiling with no bytecode cache took 0.72–0.82 s, so the check is about 4.8 s of each first import.
- **The check scales with the libraries loaded.** Loading all 170 compiled libraries (163 MB) took 16.0–17.8 s after each unpack and 0.15–0.20 s the second time. Open WebUI's own start falls between 5.5 s and that; it was not started.
- **Removing the tree at exit:** 0.9–1.3 s.
- **Shell tools, per invocation (unpack, run, remove):**
  - `sqlmap --version`: 0.79–1.06 s, against 0.48–0.85 s from a tree already unpacked.
  - `pwn version`: 1.70–4.00 s against 0.12–0.27 s.
  - `checksec --help`: 2.5–3.0 s against 0.19–0.21 s.
  - All three ran correctly from a moved tree, which the design had not tested for pwntools.
- **Read-only disk image of the same tree** (298 MB, APFS, compressed):
  - Attach 0.18–0.30 s, detach 0.17 s.
  - First mount: import 7.58 s with the check.
  - Second and third mount: import 1.33 s, all 170 libraries 0.46–0.66 s, zero `XprotectService` CPU. The scan result survives a remount.

### Where the temporary location should be

| | `$TMPDIR` | `~/Library/Caches` |
|---|---|---|
| Unpack, 4 interleaved rounds | 1.36–1.52 s | 1.09–1.32 s |
| Remove | 1.13–1.35 s | 1.01–1.13 s |
| Volume | same APFS data volume | same |
| Leftovers after a crash | removed by macOS when unused for 3 days | stay until Svalbard runs there again |

- The 3-day figure is read from `com.apple.bsd.dirhelper.plist` on this machine (`CLEAN_FILES_OLDER_THAN_DAYS=3`, at boot and daily). That it covers the per-user temp directory is not tested.
- **macOS: use `$TMPDIR`.** It costs 0.2–0.3 s more to unpack, and it is the only place where the computer cleans up when the stick never comes back. Resolve it to its canonical path. If it is unset or resolves under `/tmp`, use the user cache dir.
- **Linux, from reading, not run:**
  - `/tmp` is RAM-backed on Fedora, Arch and Debian 13, so the tree would take 0.55 GB of memory beside the model. `$XDG_RUNTIME_DIR` is RAM too.
  - Use `/var/tmp/svalbard-<uid>/`: on disk, and systemd removes entries unused for 30 days.
  - Fall back to the user cache dir when `/var/tmp` is not writable, is tmpfs, is mounted noexec, or the directory is not owned by the user.
- `SVALBARD_RUN_DIR` overrides both, with the same `/tmp` rejection.

**Not measured:** a physical stick (the design computes 2–7 s more to read the zip), a start of Open WebUI, Linux or Intel, whether the scan result survives a reboot, whether today's loose tree on exFAT is rescanned at each mount.

## 2. Questions for the owner

**Q1. The contained default costs more than a few seconds per start on a Mac.** Each Open WebUI start pays 1.1–1.5 s of unpack, 5–18 s of macOS scanning, and about 1 s of removal at exit, before the read from the stick. With the keep setting on it pays 0.2 s.

- A. Keep the default and accept it.
- B. Make "keep" the default.
- C. Keep the default now, and evaluate the disk image for macOS straight after chunk 2. The image had the scan once per computer, then 1.3 s starts, with nothing unpacked on the host and no RAM use.
- **Recommended: C.** Chunks 1 and 2 are needed on Linux either way, and trees are reached through one interface so an image backend can replace unpacking on macOS without touching callers.
- Open before the image could become the default: who builds it (the builder runs in a Linux container and has no `hdiutil`), its size (298 MB against 210 MB), and behaviour from an exFAT stick and after a reboot.

**Q2. Scope of the keep setting.** Settings live on the stick, so a plain on/off would also leave trees on borrowed computers.

- **Recommended: per computer.** The value is stored on the stick, keyed by a hash of the machine and user, so nothing extra is written to the computer.
- The alternative is one switch per stick, which is smaller to build.

## 3. Shape

```text
 USB stick                          This computer
┌────────────────────────────┐    ┌──────────────────────────────────────┐
│ runtime/python/<platform>/ │    │ run dir (default, removed on exit)   │
│   tools/<id>.zip ──────────┼─┐  │  macOS  $TMPDIR/svalbard-run/        │
│   tools/<id>.packages      │ │  │  Linux  /var/tmp/svalbard-<uid>/     │
│   python/   (stays loose)  │ │  │   <session>.lock  held while alive   │
│ bin/<platform>/<entry> ─┐  │ ├─►│   <session>/<id>-<key>/      tree    │
│ .svalbard/              │  │ │  │   <session>/<id>-<key>.pyc/  bytecode│
│   checksums.sha256  key │  │ │  ├──────────────────────────────────────┤
│   settings.json         │  │ │  │ kept dir (setting on, this computer) │
│   runtime/<platform>/   │  │ └─►│  <user cache>/svalbard/trees/        │
│     svalbard-drive ◄────┘  │    │   <id>-<key>/ <id>-<key>.pyc/ locks/ │
└────────────────────────────┘    └──────────────────────────────────────┘
 start: sweep sessions nobody holds → Acquire (lock, space check, unpack
 to .partial, rename) → run → Close (remove session). Crash or pulled
 stick: the next start on that computer sweeps it.
```

- **A session, not a command, owns an unpacked tree.** Per-command unpacking costs 1.7–4 s for pwntools, so the menu action, shell or harness that starts first owns the session and exports `SVALBARD_TREES_SESSION`. A wrapper started inside it reuses the tree; a wrapper started alone owns a session for that one command.
- **The crash sweep is in chunk 1, not chunk 3.** The session lock that makes it possible is built there, and the default is not contained without it. Chunk 3 makes it visible.
- **Bytecode goes beside the tree** in both modes and leaves the stick.
- **Key:** platform, a format version, and the zip's sha256 from `.svalbard/checksums.sha256`. If the line is missing (an interrupted apply), the runtime hashes the zip for that run.

## 4. Contracts

- **Hash separation, unchanged in structure.**
  - Acquisition hash: untouched (`apply.go:258`).
  - File checksum: new for packed entries, the zip's sha256 in `ChecksumSHA256`.
  - Builder fingerprint: the `.packages` stamp, same content and place.
  - The runtime key is derived at run time and never stored. Manifest stays version 2.
- **Changed:**
  - A packed python-package entry points at `tools/<id>.zip`, with the zip's size and checksum.
  - `verify` and `checksums.sha256` start covering Python tools.
  - Wrappers call the drive runtime instead of the interpreter.
  - `svalbard update <python tool>` is allowed.
  - Removal also deletes the stamp and wrappers.
  - The uv cache and Open WebUI's bytecode leave the stick.
- **PlatformIO, untouched.** `platformio-core` and `espidf-python` stay loose at the same paths. `validatePlatformIOPythonClosure`, `validateClosure`, the fingerprint, format `1` and the cache root do not change. Generations stay immutable and keyed by vault path and archive content. The only contact is that `embedded.Run` opens a session and (chunk 3) notes its generation path there.
- **Apply checkpoints.** Packed entries come back from the venv job and are checkpointed once per job as today. Conversion is idempotent: the zip is published before the loose tree is removed, and a rerun with a matching stamp and an existing zip only returns the entries.
- **Cancellation.** Packing and unpacking honour the context and remove their `.partial`.
- Presets, search and document-collection contracts are not touched.

## Chunk 1: run a packed tree from the drive runtime (M)

**Goal:** the runtime can run a tool from `tools/<id>.zip`, contained, with nothing left after exit or after the next start following a crash. Nothing changes for users yet, because the builder still writes loose trees.

**Files:**
- Create `drive-runtime/internal/trees/` (`trees.go`, `location.go`, `unpack.go`, `session.go`, tests).
- Modify `drive-runtime/cmd/svalbard-drive/main.go` (a `tool` fast path beside `settings`, before `config.Load`).

**Interfaces:**
- `type Tree struct{ Dir, Python, PycDir string }` and `func (t Tree) Env() []string` (`PYTHONPATH`, `PYTHONNOUSERSITE=1`, `PYTHONPYCACHEPREFIX`).
- `Open(ctx, stdout, driveRoot) (*Session, error)`: sweeps, then joins the session named in `SVALBARD_TREES_SESSION` if its lock is held, else creates one and sets the variable.
- `(*Session).Acquire(ctx, id) (Tree, error)`:
  - Zip present: free-space check from the zip's listed sizes, parallel unpack to `.partial`, rename, under a per-tree lock.
  - Only a loose `tools/<id>/`: return it as it is. This covers the two PlatformIO trees and vaults not yet converted.
- `(*Session).Close() error`: the owner removes the session; a joiner does nothing.
- `Sweep() (bytes int64, err error)`: removes every session whose lock can be taken.
- `RunTool(ctx, driveRoot, id, entry string, python bool, args []string) error`: a joiner replaces itself with the tool; an owner runs it as a child, passes signals and the exit code, then closes.
- `Interpreter(driveRoot, platform) (string, error)`: the same glob as `embedded.bundledPython`, which stays where it is.
- Command: `svalbard-drive tool <id> <entry> [args…]` and `svalbard-drive tool <id> --python [args…]`.

**Steps:**
1. Write failing tests:
   - `TestLocatePrefersZipAndPassesLooseTreeThrough`
   - `TestKeyUsesChecksumsFileAndFallsBackToHashing`
   - `TestRunBaseRejectsMacOSTmp`
   - `TestRunBaseLinuxSkipsTmpfsNoexecAndForeignOwner` (statfs injected)
   - `TestAcquireKeepsModesAndRejectsEscapingNames`
   - `TestAcquireNamesTheSpaceNeeded`
   - `TestAcquireCancelLeavesNothing`
   - `TestCloseRemovesTreeAndBytecode`
   - `TestSecondOpenJoinsAndUnpacksOnce`
   - `TestSweepRemovesOnlySessionsNobodyHolds` (a child holds the lock and is killed)
   - `TestRunToolReturnsExitCodeAndCleansUp`
   - `TestToolCommandNeedsNoActionsFile`
2. Run `cd drive-runtime && go test ./internal/trees ./cmd/... -count=1`. Expect FAIL.
3. Implement. The lock file is created and locked before its directory exists, so a directory without a holder is always stale.
4. Run `make verify`.

**Verify through the user's path:**
- Clone the shared vault (`cp -c -R`), adding `sqlmap` if it is missing. Zip `tools/sqlmap` by hand and remove the loose directory.
- `./run tool sqlmap sqlmap --version` prints the version, and the run dir is empty afterwards.
- Start `./run tool sqlmap --python -c "import time; time.sleep(60)"` and `kill -9` the runtime. The tree is left behind; the next `./run tool …` removes it.
- On an unconverted clone the existing wrappers still work.

**Risk:** an L if a harness clears the environment, so sessions need another way to be found.

## Chunk 2: pack the trees and cut over (L)

**Goal:** every python-package recipe except the two PlatformIO trees is one zip per platform, existing vaults convert without network, and Open WebUI, the shell tools and `py-docs` run contained.

**Files:**
- Create `host-cli/internal/builder/packtree.go`.
- Modify:
  - `host-cli/internal/builder/pythonvenv.go`
  - `host-cli/internal/apply/apply.go` (`:127-133` removal, the job partition near `:171`)
  - `host-cli/internal/commands/update.go:112`
  - `drive-runtime/internal/openwebui/openwebui.go`
  - `drive-runtime/internal/shell/shell.go`, `drive-runtime/internal/embedded/embedded.go` (`Run` only), `drive-runtime/internal/agent/`
  - `recipes/apps/open-webui.yaml` and the `recipes/tools/` Python tools (`size_gb` to the packed size per platform), then `scripts/sync-catalog.sh`

**Interfaces:**
- `packTree(ctx, srcDir, zipPath) (sha256 string, size int64, err error)`:
  - Sorted entries, deflate, fixed timestamps, modes kept.
  - Skips `._*`, `.DS_Store` and `__pycache__`; a symlink is an error.
  - Writes `.partial`, syncs, renames.
- **Builder:**
  - Staging and the uv cache move to `<user cache>/svalbard/python-build/`, passed as the tool runner's work directory. Staging is removed through the tools image, since a Linux container may leave root-owned files.
  - A matching stamp with a loose tree and no zip is packed in place and the loose tree removed. A missing zip forces a rebuild.
  - An existing `<vault>/.staging/uv-cache` is deleted.
  - PlatformIO trees keep today's code path.
- **Realized entry:** `RelativePath runtime/python/<platform>/tools/<id>.zip`, `Filename <id>.zip`, `SizeBytes` and `ChecksumSHA256` of the zip.
- **Wrappers** carry a marker line so removal finds them without the catalog:
  - Entry point: `# svalbard:tree <id>`, then `DRIVE_ROOT="$DRIVE" exec "$DRIVE/.svalbard/runtime/<platform>/svalbard-drive" tool <id> <entry> "$@"`.
  - Stage 2's `python_launcher`: the same with `tool <id> --python "$@"`.
- **Apply:**
  - A realized non-PlatformIO python-package entry that is not a zip queues its venv job.
  - `builder.RemovePythonPackage(root, entry)` deletes the zip or loose tree, the stamp, and the wrappers marked with that id.
- **Update:** the python-package refusal goes; python-venv stays refused. The builder renames the new zip over the old, so a failed install keeps the installed copy. `--replace` frees nothing early for these.
- **Open WebUI:**
  - `Run` opens a session and acquires the tree before starting llama-server.
  - It starts `<python> <tree>/bin/open-webui serve …` with `Tree.Env()` in `Environment`.
  - It deletes `<stick>/.svalbard/runtime/open-webui/pycache`.
  - It closes the session after the servers have stopped.
- **Sessions:** `shell.Run`, `embedded.Run` and `agent.Run` open one and pass `SVALBARD_TREES_SESSION` to what they start.

**Fit with stage 2:**
- `py-docs` and its launcher convert like every other recipe.
- `TestPythonLauncherWrapper` expects the new wrapper text. `TestPythonLauncherFailsWithoutWheels` is unchanged.
- Stage 2's wheel-tag check reads the zip listing instead of `find` on a directory.
- A harness session unpacks `py-docs` once, not once per script. Its lxml and Pillow libraries would otherwise be rescanned on each call (inferred from the measurement, not run for py-docs).

**Steps:**
1. Write failing tests:
   - Builder: `TestPackTreeIsDeterministicAndSkipsMacMetadata`, `TestPackTreeKeepsModesAndRejectsSymlinks`, `TestPackTreeCancelRemovesPartial`, `TestBuildPythonVenvWritesZipEntryWithChecksum`, `TestBuildPythonVenvPacksLooseTreeWithoutInstalling`, `TestBuildPythonVenvRebuildsWhenZipIsMissing`, `TestBuildPythonVenvLeavesPlatformIOTreesLoose`, `TestBuildPythonVenvKeepsUvCacheOffTheVault`, `TestWrapperCallsDriveRuntime`.
   - Apply and update: `TestApplyQueuesVenvForUnpackedTrees`, `TestRemovePythonPackageDeletesZipStampAndWrappers`, `TestUpdatePythonPackageKeepsOldZipWhenInstallFails`.
   - Runtime: `TestRunStartsOpenWebUIFromUnpackedTree`, `TestEnvironmentKeepsBytecodeOffTheDrive`, `TestRunRemovesTreeAfterServersStop`, `TestShellPassesTreesSession`.
2. Run both modules' tests. Expect FAIL.
3. Audit the seam first: confirm whether `svalbard apply` returns early on an empty plan. Conversion must still run and be reported as one line per recipe.
4. Implement, then run `make verify`.

**Verify through the user's path:**
- Build a 6 GB exFAT image with 131072-byte clusters under `~/.cache/svalbard-scratch/trees/`. Create a vault on it with `open-webui`, `qwen-4b` and `sqlmap`, applying with `SVALBARD_CACHE_DIRS=~/.models` (a copy from there, no multi-GB download).
- **Create:** the `open-webui` zip is under 0.25 GB on the image; no `.staging/uv-cache`; `./run verify` lists the zips as OK.
- **Run from the menu:** the unpack line shows path and size, `/health` answers, a chat persists. After Ctrl+C the run dir is empty.
- **Second start:** unpacks again and works. Record both start times in the checkpoint.
- **Remount at another path:** starts, chat history opens.
- **Ctrl+C during unpack:** nothing left, retry works.
- **Update:** bump the pin in a personal recipe copy, `svalbard update open-webui`. New zip, and the old chats open.
- **Remove:** zip, stamp and wrapper are gone, the menu entry is gone, and adding it back builds.
- **Shell:** `./run activate` then `sqlmap --version` twice unpacks once. `source ./activate; sqlmap --version` works and leaves nothing.
- **Conversion:** apply a clone of the shared vault that has loose trees with networking off. The trees become zips and the tools run.
- **Stage 2:** `bin/<platform>/py-docs -c "import openpyxl, pypdf, reportlab, docx, pptx; print('ok')"`.

**Risks:**
- An XL if a tool writes into its own tree or needs its on-stick path. angr was not run from a moved tree; pwntools and sqlmap were.
- An XL if Open WebUI's real start loads enough libraries that Q1 has to be settled before release.

## Chunk 3: show and clean what is on this computer (M)

**Goal:** the user sees what a run put on the computer and can remove everything Svalbard left there, including PlatformIO generations.

**Files:**
- Create `drive-runtime/internal/trees/usage.go`.
- Modify:
  - `drive-runtime/internal/actions/actions.go` (native action `cleanup`)
  - `drive-runtime/cmd/svalbard-drive/main.go`
  - `host-cli/internal/toolkit/toolkit.go` (menu entry `Clean this computer` beside `inspect` and `verify`)
  - `drive-runtime/internal/embedded/embedded.go` (`Run` writes its generation path into its session)

**Interfaces:**
- `Usage() []Item` with `Kind` (run, kept, platformio), `Path`, `Bytes`, `InUse`.
- `Clean(stdout) (freed int64, err error)`: removes everything not in use.
  - A run or kept tree is in use while its lock is held.
  - A PlatformIO generation is in use while a live session names it.
- `Open` prints one line when the sweep removed something: `Removed 553 MB left by a run that did not finish.`
- `./run cleanup` lists each item with its size, then what it removed and what it skipped as in use.

**Steps:**
1. Write failing tests: `TestUsageListsRunKeptAndPlatformIO`, `TestCleanSkipsWhatIsInUse`, `TestCleanSparesPlatformIOGenerationOfAnOpenShell`, `TestSweepReportsWhatItRemoved`, `TestMenuHasCleanThisComputer`.
2. Run them. Expect FAIL. Implement, then `make verify`.

**Verify through the user's path:**
- Kill a running Open WebUI start with `kill -9`. Open the menu: the removal line appears and the run dir is empty.
- Open the embedded shell in one terminal and run `./run cleanup` in another. The generation in use is kept; after the shell closes, cleanup removes it and the next shell prepares it again.
- On a second user account: run a tool, run cleanup, confirm the directories are empty.

**Risk:** an L if a PlatformIO generation needs a real lease inside the generation, which would touch the finished phase's format.

## Chunk 4: keep on this computer (M)

**Goal:** one setting, off by default, keeps unpacked trees on a computer the owner chooses, for 0.2 s starts. Turning it off removes them.

**Files:**
- Modify:
  - `drive-runtime/internal/settings/builtin.go` and `settings.go`
  - `drive-runtime/internal/trees/`
  - `host-cli/internal/catalog/settings.go` (reserved owners stay in sync)
  - `host-cli/internal/commands/` (remove)

**Interfaces:**
- Built-in setting `drive.keep_unpacked`: bool, default off, label `Keep unpacked tools on this computer`, description naming the directory and `Clean this computer`.
- `Setting.PerHost`: the value is stored as an object keyed by host id (a hash of the machine id and user id) under the same key. `stored` and `update` read and write this computer's entry only. Under Q2's alternative this is a plain bool.
- `Acquire` with the setting on:
  - Unpacks to `<user cache>/svalbard/trees/<id>-<key>.partial`, renames, and holds a shared lock for the session.
  - Prunes other generations of the same recipe that nobody holds.
  - Prints the path, the size and how to clean, at first unpack only.
- With the setting off, `Open` removes this computer's kept trees that nobody holds.
- `svalbard remove` deletes `<user cache>/svalbard/trees/<id>-*` on the provisioning computer.
- The kept root follows the PlatformIO rule (canonical, never under macOS `/tmp`), so design chunk 3 can build on it.

**Steps:**
1. Write failing tests: `TestKeepUnpackedIsPerComputer`, `TestKeepUnpackedSurvivesOlderRuntimeReadingTheFile`, `TestAcquireKeptReusesGenerationAtAnotherMountPath`, `TestAcquireKeptPrunesOldPinNotInUse`, `TestTurningKeepOffRemovesKeptTrees`, `TestRemoveDropsKeptTreeOnThisComputer`.
2. Run them. Expect FAIL. Implement, then `make verify`.

**Verify through the user's path:**
- `./run settings drive.keep_unpacked on`, then start Open WebUI twice. The second start does not unpack; record its time.
- Remount at another path: no new generation.
- Update the pin: one generation remains after the next start.
- On a second user account the setting reads off and the start is contained.
- Turn it off: the next start removes the kept tree. `Clean this computer` shows nothing left.

**Risks:**
- An L if the machine id is unstable or unreadable on some Linux systems and a second identity source is needed.
- An L if per-computer values need a settings schema version.

## Follow-ups to file as inbox tasks

- Evaluate the read-only disk image for macOS (Q1, option C).
- Measure today's loose-tree start from a physical stick, to know what the packed default is being compared with.
- The design's three: a cluster-aware space estimate, `._` companions after apply on macOS, and recording "datasets stay zipped".
