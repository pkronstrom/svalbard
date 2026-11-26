# Skill vault stage 2 checkpoint: lean skills that need no JS

Status: built, reviewed and verified 2026-11-27 on branch `skill-vault-stage2` (cut from `main` plus stage 1); all seven tasks of the [plan](2026-11-26-skill-vault-stage2-lean-skills-plan.md), stage 2 of the [design](2026-11-26-skill-vault-and-js-libraries-design.md). Not yet merged.

## Resulting user path

1. **Install.** `ai/harnesses` now carries `skills-core`, so `default-256`, `test-comprehensive` and any vault that adds the harnesses get it; its deps bring `py-docs` (and `svalbard-python`). `svalbard add skills-core` works alone. Deselecting `skills-core` drops both.
2. **Apply writes the skills that can run.** `skills-core` is built by the `embedded` family (a folder and a marker, no download). Generate then writes each skill embedded in host-cli whose `requires:` are installed into `skills/skills-core/<skill>/`, and deletes those whose requirements went. A file is written only when its bytes differ, so an unchanged apply re-reads nothing.
3. **Local edits.** The folder is Svalbard's. A file edited, added or deleted by hand is put back, and apply says once per skill `restored svalbard skill <name> (local edits replaced)`: on the command line, and as a "Note:" on the TUI's plan and wizard apply screens. Hidden files (`.DS_Store`, `._*` on exFAT) are left alone.
4. **Find and use.** Stage 1's `skill_search`/`skill_read` and `<drive>/run skills` find them. The four document skills run `py-docs "<folder>"/scripts/<tool>.py …`; the scripts also run directly (`#!/usr/bin/env py-docs`) and name `py-docs` when started with another Python.
5. **Check.** Every skill has `examples/check.sh`: with a file it validates what a model made, without one it self-tests in its own temporary folder. `scripts/skill-run-check.sh <vault> [skill…]` runs them with the network denied.
6. **Remove.** Removing `py-docs`, or any Python package, now also removes its launcher and entry-point wrappers from `bin/<platform>/`.

The seven skills:

| Skill | Requires | What it does |
|---|---|---|
| `vault-research` | – | search, read and cite the drive's archives; say when the drive lacks it; `scripts/mcp_call.sh` for agents without the MCP tools |
| `local-models` | `llama-server` | `scripts/models.sh`: RAM, each model's size and the largest context tier that fits; set `ai.default_model`/`ai.context`; never download |
| `esp32-firmware` | `platformio-core`, `platformio-espressif32` | copy the drive's example, write ESP-IDF C, build with `scripts/pio.sh` (the Embedded Dev Shell, non-interactive); flash only with a board and consent; tested snippets for GPIO, LEDC, ADC |
| `spreadsheet-file` | `py-docs` | `xlsx_tool.py`: CSV/JSON → xlsx with styled header, formulas, total row; read as CSV; openpyxl snippets (charts, formats) |
| `pdf-file` | `py-docs` | `pdf_tool.py`: markdown → PDF (headings, lists, tables, images, page breaks), merge, pages, split, text, form fields and fill |
| `word-file` | `py-docs` | `docx_tool.py`: markdown → docx, read as markdown, fill `{{placeholders}}` |
| `slides-file` | `py-docs` | `pptx_tool.py`: markdown or JSON outline → 16:9 deck with bullets, pictures, notes; read |

## Contracts to keep

- **Write a skill file only when its bytes differ** (stage 1's reason: size and mtime are the index's identity, and a replaced source sets the semantic state to building). Each skill folder holds `.svalbard-sha256`, the hashes of what Svalbard last wrote; a difference from it is a local edit, a difference from the embedded tree alone is an update and is not reported.
- **`skills/` is never removed** by apply; only `skills/<embedded recipe>/` and the skill folders inside it. A realized skills recipe always has its folder, even when no skill qualifies.
- **`requires:`** entries are recipe ids or `type:<type>`; `make verify` (`TestRealSkillsPassLint`) fails a skill that names an id the catalog lacks or a type no catalog recipe has, that is not in a catalog recipe's folder, whose frontmatter `name` is not its folder, or that has no description.
- **Offline static check:** any line of a skill's `.md .txt .html .css .js .mjs .ts .py .sh .json .ini .c .h` that runs a package manager or installer, downloads, clones or pulls, opens a network address from Python, or loads a script, stylesheet, image, font, module, socket or `fetch` from the network fails, outside a fence whose info string is `not-for-this-stick`. An unclosed fence fails. Plain links in prose pass.
- **`examples/check.sh` contract** (authoring README): with an argument it validates and writes nothing in the current folder; without one it self-tests in `mktemp -d` with `example-*` names; exit 77 means nothing to test (SKIP).
- **A `python_launcher` package installs on every vault platform or fails the build**, naming the platform.
- **SKILL.md commands quote `"<folder>"`.**

## Verification evidence

All on macOS arm64 (128 GB), a minimal scratch vault: a clone of the shared test vault (Qwen3.5 4B, pi, llama-server, 9 ZIMs and 2 PDFs, keyword index) plus, applied with this branch's CLI, `py-docs`, `skills-core`, the PlatformIO ESP32 closure (17 items, 1.5 GB, the only large download) and Qwen3.8 27B cloned from the local model cache.

- `make verify` passed on the final tree; one earlier run failed `drive-runtime/internal/openwebui` `TestRunStartsEverythingAndStopsItOnCancel` while a model check had llama-server running (no drive-runtime file changed on this branch; the package passed 3 of 3 alone and in the final run). `make build` passed.
- **Run checks**, `scripts/skill-run-check.sh <vault>` on the skills apply wrote, network denied (`sandbox-exec (deny network*)`, every socket), after the review fixes:

  | Skill | Result | What the self-test does |
  |---|---|---|
  | esp32-firmware | PASS | copies the example, builds it offline (147,600-byte image, magic `e9`), breaks `main.c` and expects the check to fail |
  | local-models | PASS | lists 2 models, recommends Qwen3.8 27B (fits Max on 128 GB), checks it is a drive model that fits |
  | pdf-file | PASS | 3-page PDF with table and picture, merge → 6 pages, text of page 5, pages 2-3, split into 3, form fields listed and filled |
  | slides-file | PASS | 4-slide deck from JSON and 3 from markdown, picture, sub-bullets, notes, no shape off the slide |
  | spreadsheet-file | PASS | CSV → xlsx, `B2 = 120`, `=SUM(B2:B4)`, read back; `Nan`, `007`, `+358…` stay text |
  | vault-research | PASS | `vault_sources`, `search`, `search_read` over the drive's MCP server by stdio |
  | word-file | PASS | docx with title, numbered and bulleted lists, table; read back; template fill |

  Fixtures: `net-probe` (curl) FAIL, `ok` PASS; `vault-research` on a vault without an index: SKIP.
- **RF1** (on a clone of the vault): removing `platformio-core` alone made Generate fail before any skill was touched (`PlatformIO Python closure missing platformio-core`, an existing guard); removing `platformio-espressif32` too and applying removed `skills/skills-core/esp32-firmware/` and pruned its two index sources, leaving 7. Unit and apply tests cover the requirement filter.
- **RF2**: editing `pdf-file/SKILL.md` and adding `scripts/extra.py`, then applying, printed `svalbard: restored svalbard skill pdf-file (local edits replaced)` once, restored the file, deleted the extra one, re-indexed only that SKILL.md ("8 documents unchanged"), and left another skill's mtime unchanged; the next apply printed nothing.
- **RF3**: `py-docs` built for all four platforms (case-sensitive APFS image, see below); `.so` files are Mach-O arm64/x86_64 and ELF aarch64/x86-64 as expected; each launcher imported openpyxl, pypdf, reportlab, docx, pptx, lxml and PIL on its own interpreter: macOS arm64 natively, macOS x86_64 under Rosetta, Linux arm64 and x86_64 in `debian:stable-slim` containers with `--network none` (the arm64 one also wrote and re-read a PDF).
- **RF4**: `TestLintUnknownRequires` and the real-tree test.
- **RF5**: `svalbard remove skills-core` + apply removed `skills/skills-core`, kept `skills/` (empty), left 0 skill sources in the index, and `.svalbard/recipes.json` held no `skills` recipe, so `tools/list` over MCP offered no `skill_*` tool. The harness context paragraph follows `recipes.json` (stage 1 tests); not launched on that vault.
- **Self-tests never touch the caller's files**: every check run without an argument in a folder holding `report.md`, `report.pdf`, `plan.docx`, `deck.md`, `values.json`, `budget.xlsx`, `models.txt` left all of them unchanged.

## Model checks (recorded, not gating)

pi 1.6.1 from the drive's menu action (`run __native-agent pi <model>`), context Balanced (32K), each run in an empty work folder, `HOME`/`XDG_*` in scratch, `sandbox-exec` denying every network connection except localhost and Unix sockets, a no-op `open` first on PATH. One plain request per skill, no hint about skills. "Found" means `skill_search` then `skill_read` of that skill; "valid" is the skill's `check.sh` on the model's file. The first round ran on the skills before the review fixes; "4B (after)" reran the document tasks on the final skills.

| Skill | Request | Qwen3.5 4B | 4B (after) | Qwen3.8 27B |
|---|---|---|---|---|
| spreadsheet-file | "Make a spreadsheet file sales.xlsx … with a total row" | not found; wrote a markdown table into `sales.xlsx`; invalid ("not a zip file") | same; invalid | found, followed (CSV, `create --total`, `read`, ran the check itself); valid, `=SUM(B2:B4)` |
| pdf-file | two-page `report.pdf`, table, checklist | found, followed after 4 failed tries (script directly, `python`, `python3`, then `py-docs` found with `find`); valid, 2 pages | found, followed; ran the script directly (new shebang) first time; valid | found, followed; valid |
| word-file | `notice.docx` with heading, paragraph, table | not found; wrote markdown into `notice.docx`; invalid | not found; wrote raw WordprocessingML into it; invalid | found, followed, ran the check; valid (1 heading, 1 table) |
| slides-file | `deck.pptx`, title + 3 slides | found, followed after `python`/`python3`; valid, 4 slides | found; guessed `<folder>/py-docs`, then found the launcher; valid | found, followed; valid |
| vault-research | boiling water, answer with sources to `answer.md` | not searched for (went straight to `search`, which the drive context already teaches): 4 searches, 3 reads; valid, cites Outdoors Stack Exchange | – | not searched for; `vault_sources`, 5 searches, 2 reads; valid |
| local-models | which model for a coding agent → `choice.txt` | not found; read pi's own docs and `models/`; chose the 4B; valid by the check (fits), a weak choice | – | not found; read `hw.memsize` and `models/`; chose the 27B; valid |
| esp32-firmware | blink GPIO 2 once a second, print a counter, build | found, followed (copied example, edited `main.c`, `pio.sh … run`); builds; toggles every 1 s (0.5 Hz blink) | – | found, followed, ran the check; builds; toggles every 500 ms (1 Hz) |

Two more 4B runs with the context sentence changed to name "spreadsheet, Word, PDF or slides file" (edited in pi's generated context file, then restored): spreadsheet and Word still written directly, invalid. The 4B model looked for a skill when asked for a PDF or slides (2 of 2 runs each) or firmware (1 of 1), and never when asked for a spreadsheet or a Word file (0 of 3 each); the drive context's wording is not the lever. The 27B model found and followed the skill for every file or firmware it was asked to make, 5 of 5 valid on the first try. Neither model looked for a skill on the two questions (research, model choice), and both answers were still valid.

## Measured

- **SKILL.md** (Qwen3.5 4B tokens, `/tokenize`, final text): esp32-firmware 758, vault-research 658, pdf-file 654, local-models 618, word-file 611, spreadsheet-file 604, slides-file 589. All inside the 300–800 target.
- **skills-core** on the vault: 40 files, 180 KB (all seven written).
- **py-docs** tree per platform: macOS arm64 45 MB (53 MB after `__pycache__` from runs), macOS x86_64 38 MB, Linux arm64 46 MB, Linux x86_64 45 MB; the shared interpreter (`svalbard-python`) 67 / 67 / 86 / 103 MB. Pins: openpyxl 3.1.5, pypdf 7.2.0, reportlab 5.1.2, python-docx 1.2.0, python-pptx 1.0.2 (lxml 6.1.3, Pillow 13.0.0). Recipe `size_gb: 0.046`.
- **ESP32**: the closure is 1.5 GB; the first build on a computer prepares a 5.3 GB PlatformIO cache (about 40 s here) and a build takes about 30 s, 3 s incremental.

## Review

An independent review of tasks 1–6 found no break of the write-only-when-different contract and ten problems, all fixed in commits on top, each with a test or a check run that fails without it where one could be written:

1. High: a check run without an argument overwrote the user's work (`report.pdf`, `plan.docx`, `deck.md`, `values.json`) and printed ok. Self-tests now run in their own temporary folder with `example-*` names.
2. The local-edits notice was lost in the TUI: the plan screen and the wizard now show it (host-tui tests).
3. Hidden files counted as local edits and were deleted (test).
4. The esp32 check passed on a stale `firmware.bin`: it now builds the project first; the self-test breaks the source and expects a failure.
5. Lint gaps (Python network calls, other installers, `srcset`, CSS `@import`, WebSocket, unscanned file types, unclosed fences, `type:` checked against directories instead of catalog types) (fixtures).
6. The run-check sandbox allowed Unix sockets (DNS, Docker): now denied; vault-research passed with nothing to test: now SKIP (exit 77); its citation check accepted any description word: now an id or source name in a citation.
7. The note that `default-256` newly needs Docker was wrong: `makeityourself` already needed it. What changes: a custom preset or `test-comprehensive` that picks `ai/harnesses` now needs Docker, and a `default-256` user who removed `makeityourself` to avoid Docker must also remove `skills-core`. No preflight message says so.
8. Removing a Python package left its launcher and entry points on PATH (apply test).
9. `<folder>` was unquoted in commands (a second stick mounts as `/Volumes/SVALBARD 1`).
10. `xlsx_tool` turned `Nan`, `inf`, `+358…`, `007`, `1_000` into numbers (self-test row).

Also taken from the review's unverified list: reportlab's refusal of mis-nested emphasis (confirmed, now set as plain text), docx `fill` on a paragraph with a hyperlink (left alone instead of duplicated or crashing), `chmod` errors on exFAT/FAT ignored for skill files, the launcher ignoring an inherited `PYTHONHOME` and `CDPATH`.

## Differences from the plan

- **`skills-core.yaml` landed before the skills**, in its own commit, not in Task 6: the lint test requires every skill folder to be a catalog recipe.
- **No menu row for skills** (the plan's "hidden" option): the menu guard lists type `skills` as support-only ("procedures for the AI clients, which find them with skill_search") and `py-docs` as a support-only recipe.
- **The notice travels as a progress event** with id `svalbard`, `Status: done` and the text in `Message`; the CLI prints `Message` for done events.
- **Task 7 used a minimal vault, not `default-256`** (team-lead instruction: a 200 GB preset is out). The models were the catalog's `qwen-4b` and `qwen-27b`, both, at 32K.
- **RF3 was built on a case-sensitive APFS image**: on the default case-insensitive volume, the Linux interpreter does not install (below).
- **`check.sh` takes options after the file** (`--need-sum`, `--pages`, `--slides`, `-e`) so the same checks serve the self-test, the run check and model output; vault-research and local-models validate a text answer and a model name.
- **Example inputs are `.txt`**, not `.md`, so the self-tests' sample documents are not indexed as skill sections.
- **The scripts carry `#!/usr/bin/env py-docs`** (not in the plan), after the 4B model's runs.

## Scope and limits

- **Linux targets from a case-insensitive volume** (default APFS, exFAT): building any Python package for `linux-*` fails in uv's extraction of the Linux CPython (`share/terminfo` holds names that differ only in case). Pre-existing, not caused here; filed in the tracker. With `skills-core` in `ai/harnesses`, a `default-256` vault for Linux on an exFAT stick hits it.
- **Small models and documents:** see the model checks. The skills work when found; Qwen3.5 4B does not look for one when asked for a spreadsheet or a Word file.
- **Not verified:** Linux and Intel Mac at run time apart from the launcher imports; physical sticks; flashing and serial monitor (no board); the skills under oh-my-pi, opencode, goose and Open WebUI; semantic search over the skills (the test vault is keyword-only); the TUI notice in a real terminal (unit tests only).
- **The run check's sandbox** catches what a skill's scripts do; on macOS a URL handed to another process over XPC is not stopped, and on Linux file-system sockets stay reachable.
- **python-docx numbering:** two numbered lists in one document share one numbering, so Word continues the second from the first's count. `read` prints each list from 1. Not fixed.
- **pdf form checkboxes** need the field's "on" value (often `/Yes`); the skill does not say so.
- **`svalbard update skills-core`**, should its recipe ever change, rebuilds the folder empty and rewrites every skill (a full re-index of the skills).

## For stage 3

- Run checks for HTML skills need a browser load; the contract (exit 77, self-test in a temp folder, `example-*` names) and the lint's file types already cover `templates/*.html`, `.css`, `.mjs`.
- `requires: [js-core]` and `type:js-lib` pass the lint as soon as the catalog has them.
- Give commands with `"<folder>"` and keep SKILL.md under about 800 tokens; the 27B model followed every skill as written, and the 4B model's failures were in finding a skill, not in following one.
