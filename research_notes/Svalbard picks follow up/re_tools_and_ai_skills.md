# Reverse-engineering tools and AI agent skill folders: research notes

Date: 2026-11-26. Research only; no repo files changed. Versions and asset sizes come from the GitHub
releases API (`gh release view`) on 2026-11-26. Sizes are the download size in MiB, rounded down.
`[unverified]` marks claims I could not confirm from a primary source.

Repo context checked:
- `recipes/apps/ghidra.yaml` ships Ghidra 12.1.4 as an `app-bundle`, with "requires JDK 21+ on host".
- `presets/packs/cybersec-ctf.yaml` contains ghidra, hashcat, pwntools, angr, volatility3 and binwalk.
- The recipe platforms in use are `linux-x86_64`, `linux-arm64`, `macos-arm64` and `macos-x86_64`. No
  recipe has a Windows entry, so the Windows columns below are informational.
- `docs/plans/2026-11-21-pack-operations-inventory.md` already records that Ghidra and hashcat have no
  launcher and that Ghidra needs a host JDK. Packaging more JVM or GUI tools inherits that gap.
- `drive-runtime/internal/agent/agent.go` redirects `HOME`, `XDG_CONFIG_HOME`, `PI_CODING_AGENT_DIR` and
  so on into `.svalbard/runtime/<client>/…` for opencode, goose and pi. That matters for where skills can
  be dropped (see B3).
- The drive skill notes say "pi is now launched with --no-skills". No `--no-skills` appears in the repo or its
  git history (`git log -S"no-skills"` is empty). `agent.go:151` passes only `--model`. Host skills are
  still isolated, because `HOME` and `PI_CODING_AGENT_DIR` point onto the drive.

---

## Part A: Reverse-engineering tools

### A0. Two findings that affect the existing Ghidra recipe

1. **Ghidra 12.2 is out (2026-10-22) and needs JDK 25.** 12.1.4 needed JDK 21.
   - The 12.2 asset is `ghidra_12.2_PUBLIC_20261016.zip`, 543 MiB
     (https://github.com/NationalSecurityAgency/ghidra/releases/tag/Ghidra_12.2_build).
   - 12.2's GettingStarted says "Java 25 64-bit Runtime and Development Kit (JDK)", Python 3.9–3.15 for
     PyGhidra
     (https://github.com/NationalSecurityAgency/ghidra/blob/Ghidra_12.2_build/GhidraDocs/GettingStarted.md).
   - The 12.1.4 doc says JDK 21
     (https://github.com/NationalSecurityAgency/ghidra/blob/Ghidra_12.1.4_build/GhidraDocs/GettingStarted.md).
   - Ghidra extensions are built per Ghidra version. LaurieWired GhidraMCP 2.0, for example, pins 12.1.4.
     **Bump Ghidra, its JDK and any Ghidra extension together.**
2. **A portable JDK recipe would unblock Ghidra, jadx, apktool and the Ghidra MCP bridges.** Adoptium
   Temurin ships relocatable tarballs (API: `https://api.adoptium.net/v3/assets/latest/<ver>/hotspot`,
   queried 2026-11-26):
   - Temurin 21.0.13.1+2 JDK: linux x64 198, linux aarch64 196, mac x64 185, mac aarch64 191, windows
     x64 196 MiB.
   - Temurin 25.0.5.1+2 JDK: linux x64 135, linux aarch64 134, mac x64 115, mac aarch64 130, windows x64
     135 MiB.
   - The Temurin 21 JRE for linux x64 is 50 MiB, but Ghidra asks for a JDK. jadx needs Java 11+
     (https://github.com/skylot/jadx#readme). Apktool needs Java 8+ per its docs
     (https://apktool.org/docs/install); whether 3.x raised that is [unverified].
   - One JDK 21, or 25 if Ghidra moves to 12.2, serves every JVM tool. The licence is GPLv2 with
     Classpath Exception (Temurin) [licence not re-checked today].

### A1. Candidate table

Platform key: **ma** = macOS arm64, **mx** = macOS x86_64, **lx** = Linux x86_64, **la** = Linux arm64,
**w** = Windows x64. "Portable" means you unpack it and run it, with no installer.

| Tool | What it does | Licence | Latest (date) | Portable prebuilts (MiB) | Runtime deps | Stick fit |
|---|---|---|---|---|---|---|
| **Ghidra** (shipped) | SRE suite: disassembler, decompiler, scripting, headless analyzer | Apache-2.0 | 12.2 (2026-10-22); repo pins 12.1.4 | one zip for all OSes, 543 | JDK 25 (12.2) or JDK 21 (12.1.4); Python for PyGhidra | Good once a JDK is on the stick |
| **Cutter** | Qt GUI for rizin | GPL-3.0 | v2.5.0 (2026-06-30) | ma dmg 114, mx dmg 118, lx AppImage 137 (Qt5 build 124), w zip 134; **no la** | Self-contained AppImage (needs FUSE or `--appimage-extract`) | Good GUI alternative to Ghidra with no JVM. The rz-ghidra decompiler is a plugin; whether release builds bundle it is [unverified] |
| **rizin** | CLI RE framework (r2 fork) | LGPL-3.0 | v0.9.1 (2026-06-29) | lx static tar.xz 131, w static zip 110, macOS `.pkg` 10 (installer); **no la, no portable macOS** | none (static on Linux) | Linux x64 and Windows are easy; macOS needs pkg extraction or a build |
| **radare2** | CLI RE framework | Mostly LGPL-3.0, with plugins under other licences (https://github.com/radareorg/radare2/blob/master/COPYING.md) | 6.2.6 (2026-11-05) | w zip 13 (arm64 12), macOS `.pkg` 11 (arm64 and x64), Linux `.deb` 7–8 only, plus wasi/UEFI builds | libc | Windows is easy. Linux and macOS need deb/pkg extraction; relocatability is [unverified] |
| **r2ghidra** | Ghidra decompiler as an r2 plugin | LGPL-3.0-only (https://github.com/radareorg/r2ghidra/blob/master/LICENSE.md) | 6.2.6 (2026-11-05) | darwin/linux arm-64 and x86-64 zips 15–16, w64 11 | matching radare2 version | Decompiler without a JVM. Version-locked to r2 |
| **ImHex** | Hex editor with a pattern language, data inspector and disassembler | GPL-2.0 | v1.38.1 (2025-12-21) | ma dmg 45, mx dmg 39, lx AppImage 180, la AppImage 175, w portable zip 63 (arm64 40, NoGPU 77) | OpenGL (a NoGPU build exists) | Best GUI hex editor; all five platforms |
| **hexyl** | Terminal hex viewer | Apache-2.0 | v0.17.0 (2026-02-14) | ma, mx, lx musl, la gnu, w: each 0.3–0.5 | none | Trivial `binary` recipe |
| **Binary Ninja Free** | Commercial RE suite, free edition | Proprietary | n/a | n/a | n/a | **Not redistributable.** The Free licence §5 forbids duplicating it or "provide others with the Software", and limits it to non-commercial use (https://docs.binary.ninja/about/license/free.html) |
| **IDA Free** | Commercial disassembler, free edition | Proprietary EULA | 9.x | n/a | n/a | **Not redistributable and not offline.** The download needs a My Hex-Rays account (https://hex-rays.com/faqs/why-i-need-to-have-a-my-hex-rays-account-to-download-ida-free). Non-commercial only, and the decompiler is "cloud only" (https://hex-rays.com/ida-free) |
| **x64dbg** | Windows user-mode debugger | GPL-3.0 (https://github.com/x64dbg/x64dbg/blob/development/LICENSE) | snapshot 2026.05.27 | w zip 30 (+8 symbols) | Windows only | Out of scope until vaults carry Windows tools |
| **pwndbg** (bundles GDB) | GDB/LLDB plugin for exploit dev | MIT | 2026.10.28 | GDB portable tar.xz: lx 105, la 101, mx 103, ma 97. LLDB portable: lx 141, la 133, mx 136, ma 126 | **None.** I listed the lx tarball: it contains `bin/gdb`, `bin/gdbserver` and `libpython3.13` | Best debugger pick: one archive per platform with gdb + Python inside. On macOS GDB cannot debug native arm64 processes [unverified]; the LLDB build covers that |
| **GEF** | Single-file GDB plugin (`gef.py`) | MIT | 2026.01 (2026-02-02) | no assets; one Python file | GDB built with Python 3 | Redundant if pwndbg ships |
| **gdb-static** (guyush1) | Static GDB 18.1 | GPL-3.0 | v18.1-static (2026-10-02) | lx full 27 / slim 13, la full 27 / slim 15; no macOS | none | Option if pwndbg is too big. Community build, not GNU |
| **jadx** | Android DEX/APK to Java decompiler, CLI + GUI | Apache-2.0 | v1.5.7 (2026-11-18) | `jadx-1.5.7.zip` 71 (all OSes), mac dmgs 100–101, w with-JRE zip 96 | Java 11+ | Good with a stick JDK |
| **Apktool** | APK resource decode and rebuild | Apache-2.0 | v3.0.3 (2026-07-20) | `apktool_3.0.3.jar` 14 (all OSes) | Java 8+ per docs | Good with a stick JDK |
| **ILSpy** | .NET decompiler (Avalonia GUI) | MIT | v11.2 (2026-10-27) | lx zip 52, ma zip 52, w zip 19 (self-contained 52); **no mx, no la** | .NET 10 runtime, except the Windows self-contained zip ("Please make sure that you have it installed", release notes) | Weak: a host runtime is needed on Linux and macOS. The notes also mention a native-AOT `ilspycmd` for linux-x64 and win-x64, but no such asset is in the release list [unverified where it ships] |
| **dnSpyEx** | .NET debugger and decompiler | GPL-3.0 | v6.6.0 (2026-06-20) | w64 94 (self-contained), win32 89, framework-dependent 26–30 | Windows only | Windows only |
| **Detect It Easy** | File type, packer and compiler identifier, GUI + `diec` CLI | MIT | 4.0.0 (2026-11-05) | ma portable zip 51, mx portable zip 35 (Qt5) or 52 (Qt6), lx AppImage 45, w64 zip 29 (Qt6 41); la only as deb/rpm | Qt bundled in the portable builds | Good triage tool |
| **Frida** | Dynamic instrumentation | wxWindows Library Licence 3.1 (`frida`); LGPL-2.0+ with WxWindows exception (`frida-tools`) | frida 17.27.6 (2026-11-25), frida-tools 14.13.2 (2026-11-16) | PyPI wheels for macOS x64/arm64, manylinux x86_64/aarch64 and Windows (https://pypi.org/project/frida/). `frida-server`/`frida-inject` per-device binaries on GitHub (e.g. linux-x86_64 26, macos-arm64 21) | Python (svalbard-python) | Fits as a `python-package` (`frida-tools`) in the existing venv. `frida-server` for Android targets is extra |
| **capstone** | Disassembly library + `cstool` | BSD-3-Clause (repo `LICENSES/`) | PyPI 5.0.9 (2026-05-28); 6.0.0-Alpha11 on GitHub | PyPI wheels for all four stick platforms | Python | Already in the venv through pwntools (`capstone>=3.0.5rc2`, https://pypi.org/pypi/pwntools/4.15.0/json) |
| **unicorn** | CPU emulator library | GPL-2.0 (repo); PyPI says BSD | 2.1.4 (2025-09-09) | PyPI wheels, all four stick platforms | Python | Already pulled in by pwntools, which **excludes 2.1.3 and 2.1.4** (`unicorn!=2.1.3,!=2.1.4,>=2.0.1`). Don't pin 2.1.4 in the shared venv |
| **binutils / objdump** | ELF disassembly and inspection | GPL-3.0 | n/a | No well-maintained static multi-arch release found. Candidates: pkgforge bincache (HF dataset), Julia Binutils_jll | libc | Skip. rizin, radare2, LIEF or pwntools `disasm` cover it; macOS hosts have `otool`/`llvm-objdump` with the CLT |
| **LIEF** (extra) | Parse and modify ELF/PE/Mach-O from Python | Apache-2.0 | 1.0.0 (2026-07-12) | 76 PyPI wheels incl. musllinux | Python | Cheap venv addition |
| **YARA-X** (extra) | YARA rule scanner (Rust) | BSD-3-Clause | v1.25.0 (2026-11-26) | ma 8.1, mx 9.1, lx 9.9, la 8.9, w 8.7 | none | Trivial `binary` recipe; pairs with the Trail of Bits `yara-authoring` skill |
| **UPX** (extra) | Packer and unpacker | GPL-2.0+ with exceptions (GitHub reports NOASSERTION) | v5.3.1 (2026-11-26) | lx 0.6, la 0.6, w 0.6; **no macOS binary** | none | Small Linux-only addition |

Release URL pattern for every row: `https://github.com/<org>/<repo>/releases/tag/<tag>`, with these repos:
rizinorg/cutter, rizinorg/rizin, radareorg/radare2, radareorg/r2ghidra, WerWolv/ImHex, sharkdp/hexyl,
x64dbg/x64dbg, pwndbg/pwndbg, hugsy/gef, guyush1/gdb-static, skylot/jadx, iBotPeaches/Apktool,
icsharpcode/ILSpy, dnSpyEx/dnSpy, horsicq/DIE-engine, frida/frida, capstone-engine/capstone,
unicorn-engine/unicorn, VirusTotal/yara-x, upx/upx.

### A2. MCP bridges that let an LLM drive an RE tool

The drive's harnesses already speak MCP (`agent.go` wires the svalbard MCP server into all three), so an
RE bridge is mostly a config entry plus the bridge's own runtime.

| Bridge | Target | Licence | Latest | Form | Notes |
|---|---|---|---|---|---|
| LaurieWired/GhidraMCP | Ghidra GUI plugin, or headless via PyGhidra | Apache-2.0 | 2.0 (2026-10-10) | Plugin zip (<1 MiB) + Python bridge (`ghidra-mcp`) | **Pins Ghidra 12.1.4**, which matches the repo. Headless mode needs `GHIDRA_INSTALL_DIR` and Python 3.10–3.13 (https://github.com/LaurieWired/GhidraMCP#readme). Most stars (~12k) |
| bethington/ghidra-mcp | Ghidra GUI plus headless server | Apache-2.0 | v7.1.0 (2026-11-24) | `GhidraMCP-7.1.0.zip` 0.9 + wheel | "200+ MCP tools" with lazy loading. Supported Ghidra version [unverified] |
| clearbluejar/pyghidra-mcp | Headless Ghidra via PyGhidra | Apache-2.0 | v0.3.0 (2026-11-02) | Python package; release has no assets | No GUI needed, so it suits agents best. Ghidra version range [unverified] |
| cyberkaida/reverse-engineering-assistant | Ghidra extension + MCP | Apache-2.0 | v7.3.2 (2026-11-13) | Per-Ghidra-version zips (12.0, 12.1, 12.1.4, 12.2), 9.4 each | Ships a 12.1.4 and a 12.2 build, which eases a bump |
| starsong-consulting/GhydraMCP | Ghidra, multi-instance REST + MCP | Apache-2.0 | v2.2.0-rc.3 (2026-02-16) | zip 0.1 | Supports Ghidra 11.x and 12.x per its description. Quieter project |
| radareorg/radare2-mcp (`r2mcp`) | radare2 | MIT | 1.9.0 (2026-10-15) | deb, Windows zip, Codex plugin; usually `r2pm -ci r2mcp` | Needs r2 plus r2pm, which builds from source; offline packaging is [unverified] |
| mrexodia/ida-pro-mcp | IDA Pro | MIT | 1.4.0 (2025-10-06) | Python | Needs IDA Pro, so it is not stick material |

Caution: these bridges let a model execute analysis scripts. Headless runs on untrusted samples should go
in a throwaway project directory.

### A3. Recommended pack

Name: `cybersec-re`. Alternatively, extend `cybersec-ctf`; the CTF pack header already claims "Reverse
engineering".

**Essential (~1.0–1.3 GB per platform, JDK included):**

| Item | Recipe type | Why |
|---|---|---|
| `temurin-jdk` (21 now; 25 with Ghidra 12.2) | new `binary`/`toolchain` per platform, ~115–200 MiB | Unblocks Ghidra, jadx, apktool and GhidraMCP. Fixes the "JDK on host" gap |
| `ghidra` (existing) | app-bundle | Main decompiler. Needs a launcher that sets `JAVA_HOME` (inventory item 3) |
| `ghidra-mcp` (LaurieWired 2.0, or RevA for a 12.2 build) | extension zip + `python-package` | LLM-driven RE with the stick's local models. Version-locked to Ghidra |
| `pwndbg` (GDB portable) | `binary` per platform, 97–105 MiB | Debugger with gdb + Python bundled |
| `imhex` | app per platform, 39–180 MiB | Hex editor and format parser |
| `detect-it-easy` | app per platform, 35–52 MiB (no la portable) | Triage |
| `jadx` | app-bundle, 71 MiB, all OSes | Android |
| `apktool` | single jar, 14 MiB | Android |
| `frida-tools` | `python-package` in svalbard-python | Dynamic instrumentation |
| `yara-x`, `hexyl` | `binary`, ~10 MiB total | Cheap CLI staples |

**Extended (opt-in):**
- `cutter` (114–137 MiB; no linux-arm64).
- `rizin` (static lx, 131 MiB).
- radare2 + r2ghidra + r2mcp, once a portable Linux/macOS packaging path is proven.
- `lief` in the venv.
- `upx` (Linux only).
- `gdb-static` (if pwndbg is dropped).
- `ilspy` (needs a host .NET 10 runtime).
- Windows-only items (`x64dbg`, `dnSpyEx`), parked until vaults carry Windows binaries.

**Excluded:** Binary Ninja Free and IDA Free (licences forbid redistribution; IDA needs an account and a
cloud decompiler). GEF (redundant with pwndbg). Standalone binutils (no maintained static release found).

Size: **M**, and an **L** if the JDK recipe also has to deliver Ghidra/jadx launchers. The risk that
raises the size is the existing "native apps have no launcher" gap: the recipes are easy, but the user
path "open Ghidra from the stick" is not.

---

## Part B: Folders of AI agent skills as recipes

### B1. Format

All candidates use the Agent Skills layout: a directory with a `SKILL.md` that starts with YAML
frontmatter (`name`, `description`, optional `license`, `compatibility`, `metadata`, `allowed-tools`) and
may hold scripts and references (https://agentskills.io/specification, linked from pi's
https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/skills.md).

### B2. Collections

Skill counts are `SKILL.md` files in the git tree at the given SHA. Tarball sizes were measured by
downloading `codeload.github.com/<repo>/tar.gz/<sha>` on 2026-11-26.

| Collection | Licence / redistributable? | SHA (date) | Skills | tar.gz / unpacked | Network or off-stick assumptions |
|---|---|---|---|---|---|
| **mattpocock/skills** (https://github.com/mattpocock/skills) | MIT. **Yes** | `6f7f7831a625f040a8e06a0e7954826b87ce775a` (2026-11-25); release v1.3.2 (2026-10-29) | 39: 20 `engineering/` (tdd, diagnosing-bugs, grill-with-docs, to-spec, to-tickets, triage, code-review, …), 7 `productivity/`, 4 `misc/`, 8 `in-progress/` | 273 KB / 779 KB | `setup-matt-pocock-skills` configures an issue tracker. `to-tickets`, `triage` and `pr` assume GitHub/`gh` and network. Recommend excluding `in-progress/` and `misc/` (Claude-Code-specific, e.g. `git-guardrails-claude-code`) |
| **anthropics/skills** (https://github.com/anthropics/skills) | No repo licence, but each skill has an Apache-2.0 `LICENSE.txt`, and the README says the docx/pdf/pptx/xlsx skills "are also open source (Apache 2.0)". **Yes, per skill.** `doc-coauthoring` has no `LICENSE.txt`: **exclude** it or ask | `e4bb5d960d87126ff3b3a8af1a104b7db67560ba` (2026-11-16) | 20 (+template) | 4.0 MB / 11.8 MB | docx/pptx/xlsx/pdf call npm `docx`, `pandoc`, LibreOffice `soffice` and Python libraries that are not on the stick. `claude-api`, `academy-guide` and `slack-gif-creator` are online or Claude-specific. Offline-useful subset: `skill-creator`, `mcp-builder`, `frontend-design`, `webapp-testing` (needs Playwright), `algorithmic-art`, `canvas-design`, `theme-factory`, `internal-comms` |
| **obra/superpowers** (https://github.com/obra/superpowers) | MIT. **Yes** | `22f0b463e7a5e579904dd6211a520321035526f7` = v7.0.2 (2026-11-03) | 15 (brainstorming, TDD, systematic-debugging, writing-plans, executing-plans, subagent-driven-development, …) | 530 KB / 1.6 MB | Ships native glue for opencode (`.opencode/plugins/superpowers.js`) and pi (`.pi/extensions/superpowers.ts`, `pi install git:github.com/obra/superpowers`). Skills assume git and some assume subagents. Its bootstrap normally comes from a session hook or plugin, so with a plain skills folder `using-superpowers` must be invoked by hand [unverified for goose] |
| **trailofbits/skills** (https://github.com/trailofbits/skills) | **CC-BY-SA-4.0**. Yes, with attribution and share-alike on modified copies | `149d7b21f4067765b82e67029730b42d228ef4b4` (2026-11-12) | 99 across ~45 plugins. RE/security-relevant: `dwarf-expert`, `yara-authoring`, `static-analysis` (3), `firebase-apk-scanner`, `constant-time-analysis`, `c-review`, `variant-analysis`, `semgrep-rule-creator`, `testing-handbook-skills` (15) | 3.0 MB / 9.7 MB | Many call semgrep, CodeQL or `gh`, or target Claude Code plugins. `second-opinion` needs Codex. Ship a curated subset, not the whole tree |
| **trailofbits/skills-curated** (https://github.com/trailofbits/skills-curated) | CC-BY-SA-4.0. **Yes** (attribution) | `6d05be4889017b06fb15069f371afd220daffb62` (2026-07-14) | 31, including **`ghidra-headless`** and `ffuf-web-fuzzing` | 951 KB / 3.2 MB | `ghidra-headless` runs `analyzeHeadless` and finds Ghidra via **`GHIDRA_HOME`**, then `$HOME/ghidra` (`scripts/find-ghidra.sh`). Needs Java 17+, so it works offline if the launcher exports `GHIDRA_HOME` and `JAVA_HOME`. It also carries copies of pocock's `grilling`, `handoff` and `teach` (name collisions) |
| **encidxvii/claude-reverse-engineering-skill** (https://github.com/encidxvii/claude-reverse-engineering-skill) | Apache-2.0 (LICENSE file; GitHub says NOASSERTION). **Yes** | `0182b14be00d0f558eb590b9f5f05edf2f9a8a38` (2026-11-19) | 1 skill + 22 files (playbooks, scripts) | 171 KB / 406 KB | "Python 3.8+ for the bundled scripts. External RE tools (Ghidra, rizin, jadx, ILSpy, Frida, etc.) are optional and detected at runtime" (frontmatter). MCP-first. **Best general RE skill to pair with the A3 pack** |
| **SimoneAvogadro/android-reverse-engineering-skill** | Apache-2.0. **Yes** | `0c7cf89830993df7dfc178498f69952262808bea` (2026-10-12) | 1 | 835 KB / 961 KB | Uses jadx and apktool-style tooling [tool list unverified] |
| radareorg/radare2-skills (https://github.com/radareorg/radare2-skills) | **No licence file.** Not redistributable until upstream adds one | `6b7b13ea…` (2026-10-17) | 26 (r2mcp, r2xsql/*, radius2, dev/*) | n/a | Official radare org. Worth asking pancake to add a licence |
| cellebrite-labs/ghidra-rpc | **No licence.** Not redistributable | `fa386624…` | 1 | n/a | "A Ghidra agentic reverse engineering skill" |
| P4nda0s/reverse-skills | **No licence.** Not redistributable | `a2baa31c…` | 7 (IDAPython, Frida, unicorn-debug, …) | n/a | IDA-centric |
| haikow/claude-reverse-skills | **No licence.** Not redistributable | `8491b3bf…` | 5 (apk, ida, radare2, js) | n/a | Chinese-language |
| miunasu/IDA-Skill | AGPL-3.0 (redistributable, copyleft) | `4595864d…` | 1 | 19 MB repo | Needs IDA Pro, so it is not useful on the stick |
| Masriyan/Claude-Code-CyberSecurity-Skill | NOASSERTION [licence unverified] | `3aa77d84…` | 22 | 6.4 MB repo | Broad, shallow security skills |
| zhaoxuya520/reverse-skill | MIT | `b972ad0e…` | 89 | 3.2 MB repo | "On-demand toolchain bootstrapping": **assumes network** to fetch tools. Its 45k-star count looks anomalous. Skip |

**Curated "awesome" lists.** These are link indexes, not skill sources:
- VoltAgent/awesome-agent-skills: MIT, README only (https://github.com/VoltAgent/awesome-agent-skills).
- travisvn/awesome-claude-skills: no licence.
- ComposioHQ/awesome-claude-skills: **no licence** but contains 864 `SKILL.md` files, so it is **not
  redistributable**.
- hesreallyhim/awesome-claude-code: NOASSERTION.
- zhuyansen/awesome-ai-reverse-engineering: 122 RE repos "read and security-graded", NOASSERTION.
- sickn33/antigravity-awesome-skills: MIT, but holds 10,042 `SKILL.md` files in a 493 MB repo, aggregated
  from many authors, so the provenance of each skill's licence is doubtful.

Only VoltAgent's README is clearly redistributable, as an offline index document.

**Name collisions** between the recommended sets: `grilling`, `handoff` and `teach` appear in both
mattpocock/skills and trailofbits/skills-curated. pi and goose keep the first one discovered and pi warns.
Install skills-curated as a hand-picked subset (e.g. only `ghidra-headless`, `ffuf-web-fuzzing`) to avoid
this. The only `name`/directory mismatch is anthropics' `template/` (`template-skill`); drop the template.

### B3. Do the stick's harnesses load skills offline, and from where?

| Harness (pinned) | Skill locations | Extra-path hook | Rules | Where svalbard could put them |
|---|---|---|---|---|
| **opencode** v1.18.33 | `~/.config/opencode/skills`, `~/.claude/skills`, `~/.agents/skills`, plus project `.opencode/skills`, `.claude/skills`, `.agents/skills` walked up to the git root. Source: v1 docs (https://github.com/anomalyco/opencode/blob/dev/services/site/src/docs/v1/content/skills.mdx), current docs (https://opencode.ai/docs/skills/) | Config `skills.paths` ("Additional paths to skill folders") and `skills.urls` in `opencode.json`. Schema: https://github.com/anomalyco/opencode/blob/v1.18.33/packages/core/src/v1/config/skills.ts. The loader globs `**/SKILL.md` and follows symlinks (`packages/core/src/skill.ts` at v1.18.33) | `name` and `description` are required. v1 docs say `name` must match the directory and be kebab-case ≤64 characters. Permission key `skill` | Add `"skills": {"paths": ["<drive>/skills"]}` to the `opencode.json` that `agent.go` already writes. Alternatively, `XDG_CONFIG_HOME` is already `.svalbard/runtime/opencode/config`, so `<config>/opencode/skills/` also works |
| **goose** v1.52.0 | Project `.agents/skills`, `.goose/skills`, `.claude/skills`; global `$HOME/.agents/skills`, `<goose config dir>/skills`, `$HOME/.claude/skills`, `$HOME/.config/agents/skills`; plugins under `~/.agents/plugins/`. Source: `crates/goose/src/skills/mod.rs` at v1.52.0 (https://github.com/aaif-goose/goose/blob/v1.52.0/crates/goose/src/skills/mod.rs) and docs (https://goose-docs.ai/docs/guides/context-engineering/using-skills) | No config key or env var for extra paths. `GOOSE_PATH_ROOT` relocates goose's own dirs (used in its tests) | Walks each root **recursively** for `SKILL.md`. `name` is required (a skill without it is skipped), and the first name found wins. Skills come through the built-in Summon extension (docs) | The config dir follows `XDG_CONFIG_HOME` (`agent.go` already writes `<config>/goose/config.yaml`), so use `.svalbard/runtime/goose/config/goose/skills/`, or `.svalbard/runtime/goose/home/.agents/skills/`. Whether Summon is enabled under the generated config is [unverified] |
| **pi** v0.99.1 | `<agent-dir>/skills/` (`PI_CODING_AGENT_DIR`, default `~/.pi/agent`), `~/.agents/skills/`, project `.pi/skills/` and `.agents/skills/`. Sources: https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/configuration.md, `…/docs/skills.md` | `settings.json` key `skills: string[]` ("Skill files or directories", `…/docs/settings.md`). CLI `--skill <path>` (repeatable); `--no-skills` disables discovered and configured skills but keeps `--skill` paths (`…/docs/cli.md`) | Discovery is recursive. `description` is required; name ≤64 characters. Name/directory mismatch is allowed. Collisions keep the first and warn. Exposed as `/skill:name` | `agent.go` sets `PI_CODING_AGENT_DIR=.svalbard/runtime/pi/config` and writes `settings.json` through `ensurePiSettings`, so append the drive skills path to `skills`, or drop folders into `<config>/skills/` |

All three read skills from local disk with no network. Caveats: docs were read at `main`/`dev`, while the
goose and opencode code was checked at the pinned tags. pi's skill code was **not** checked at v0.99.1
[unverified].

### B4. Proposed recipe shape

One recipe per collection, pinned to a commit tarball, unpacked under the drive's `skills/<collection>/`,
plus a pack. Sketch (field names follow existing recipes; `type: skills` and `include` are **new** and
would need host-cli support):

```yaml
# recipes/content/skills-mattpocock.yaml
id: skills-mattpocock
type: skills                 # new type, or reuse an archive-extract family
display_group: ai
tags: [ai, agentic, coding]
depth: reference-only
size_gb: 0.001
source_url: https://codeload.github.com/mattpocock/skills/tar.gz/6f7f7831a625f040a8e06a0e7954826b87ce775a
strip_components: 1
include: [skills/engineering/*, skills/productivity/*]   # drop in-progress/ and misc/
install_to: skills/mattpocock/
description: Matt Pocock's engineering skills (TDD, debugging, spec and ticket workflows)
license:
  id: MIT
  attribution: Matt Pocock
```

```yaml
# presets/packs/ai/skills.yaml
name: ai/skills
kind: pack
display_group: AI
description: Agent skill folders for the drive's AI clients
target_size_gb: 0.05
sources:
- skills-mattpocock
- skills-superpowers
- skills-anthropic        # curated include list; excludes doc-coauthoring and template
- skills-re               # encidxvii reverse-engineering + trailofbits ghidra-headless (CC-BY-SA)
- skills-trailofbits-sec  # curated subset of trailofbits/skills (CC-BY-SA)
```

Wiring: at launch, `agent.go` points each client at `<drive>/skills/`:
- opencode: `skills.paths`
- pi: `settings.json` `skills`
- goose: no path key, so symlink or copy `<drive>/skills/*` into `.svalbard/runtime/goose/config/goose/skills/`

On exFAT drives symlinks fail, so goose may need a copy or a per-collection mirror [unverified for the
drive filesystem]. The RE skill needs `GHIDRA_HOME` and `JAVA_HOME` exported in the client environment.

Design notes:
- Acquisition hash: the commit SHA in the URL makes the tarball content-addressed. Keep it separate from
  file checksums, as AGENTS.md requires.
- Licence handling: each recipe carries `license.id` and attribution. CC-BY-SA collections must ship
  their LICENSE file and keep upstream attribution inside the installed folder.
- **Do not ship** repos without a licence: radareorg/radare2-skills, cellebrite-labs/ghidra-rpc,
  P4nda0s/reverse-skills, haikow/claude-reverse-skills, ComposioHQ/awesome-claude-skills,
  travisvn/awesome-claude-skills. Same for anthropics `doc-coauthoring`, which has no per-skill licence.
- Total size for the recommended five: under 20 MB unpacked.
- The svalbard drive skill would live at `skills/svalbard/` beside these, generated
  from installed actions.

Size: **M**: new recipe type, pack and launcher wiring for three clients. It rises to **L** if goose
can't read a drive-wide folder without copying, or if a real-model test shows small local models misroute
with 100+ advertised skills. Every skill adds its name and description to the prompt, and the local
models have small context windows. The curation lists above aim to keep it near 40–60 skills.
