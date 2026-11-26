# Skill vault stage 2: lean skills that need no JS — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A `skills-core` recipe installs Svalbard-authored lean skills, filtered by what the vault has installed, with a `py-docs` Python recipe for document files, an offline static check in `make verify`, and a scripted offline run check per skill; this stage ships the seven skills that need no JS libraries.

**Architecture:** A new `skills` recipe type maps to `skills/`. `skills-core` is realized by a new `embedded` build family that creates its directory; `toolkit.Generate` then writes the embedded skill folders whose `requires:` recipes are realized (the `apppages` pattern) and removes those no longer satisfied. Stage 1's index picks them up. `py-docs` is a `python-package` recipe with a launcher wrapper that runs the drive's Python with its package tree on `PYTHONPATH`.

**Tech Stack:** Go 1.26 (host-cli), the drive's uv-managed Python 3.12 (`svalbard-python`), markdown skills in the Agent Skills layout.

**Spec:** [2026-11-26-skill-vault-and-js-libraries-design.md](2026-11-26-skill-vault-and-js-libraries-design.md) (sections Recipe types and layout, Skill shape for small models, Offline verification of skills, Catalog content, Staging 2).

**Depends on:** stage 1 merged ([plan](2026-11-26-skill-vault-stage1-search-orientation-plan.md)).

## Global Constraints

- Skills are Svalbard-authored. The four document skills are written from the use case alone, without reading or adapting Anthropic's `docx`/`pdf`/`pptx`/`xlsx` skill text or scripts (their license forbids derivative works).
- SKILL.md frontmatter: `name`, `description`, `requires:` (may be empty). Each entry is a recipe id, or `type:<recipe type>` meaning "any realized recipe of that type" (e.g. `type:pmtiles`). First section is the numbered procedure. Target 300–800 tokens for SKILL.md; details in `references/`.
- A shipped skill never instructs `npm`, `pnpm`, `npx`, `pip install`, `uv add`, `curl`, `wget`, `git clone`, or loading an `http(s)://` resource, outside a fenced block whose info string is `not-for-this-stick`.
- Only skills whose `requires:` are all satisfied by realized recipes are present on a vault.
- A missing index root is never a prune (stage 1 / document-collections contract); removing skills therefore deletes skill folders but leaves the `skills/` root in place so the index prunes them.
- Agents reach `py-docs` because stage 1 puts `<drive>/bin/<platform>` on every harness's PATH.
- Saved `desired.items` stays authoritative; preset names are provenance (presets checkpoint).
- Recipes are edited under `recipes/` and `presets/`; `scripts/sync-catalog.sh` mirrors them into `host-cli/internal/catalog/embedded/`; `make verify` fails when they differ.
- Keep Aven refs out of commits and committed docs.

## Review Focus

1. Removing `platformio-core` from a vault and applying → `skills/skills-core/esp32-firmware/` disappears and its sources are pruned from the index on the same apply.
2. A user edits a file inside `skills/skills-core/<skill>/` → the next apply overwrites it (the directory is Svalbard-owned) and the apply output says so once, not silently.
3. `py-docs` on a vault whose platforms include Linux arm64 and macOS → the launcher works on each platform's interpreter (lxml and Pillow wheels for each).
4. A skill whose `requires:` names a recipe not in the catalog (typo) → `make verify` fails naming the skill and the id.
5. `skills-core` deselected in the wizard while harnesses stay → `skills/skills-core/` removed (the empty `skills/` root may stay), the index holds no skill source after that apply, AGENTS.md has no skills paragraph, and the skill tools are not offered.

Each has a test in the task that owns the code (marked RF1–RF5).

---

### Task 1: `skills` type, `embedded` build family and the skill writer

**Files:**
- Modify: `host-cli/internal/toolkit/toolkit.go:52-66` (`TypeDirs`: `"skills": "skills"`), `:165-200` (`Generate` calls `writeSkills`)
- Modify: `host-cli/internal/builder/builder.go:43-78` (`Dispatch`: family `embedded`)
- Create: `host-cli/internal/builder/embedded.go`
- Create: `host-cli/internal/toolkit/skills.go`, `host-cli/internal/toolkit/skills/` (embedded tree, one folder per skill; this task adds one fixture skill `hello` under `testdata` only)
- Test: `host-cli/internal/toolkit/skills_test.go`, `host-cli/internal/builder/embedded_test.go`

**Interfaces:**
- Produces:
  - `buildEmbedded(root string, recipe catalog.Item, cat *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error)` — creates `<TypeDir>/<id>/` with a `.svalbard-embedded` marker file and returns one directory entry; no download.
  - `//go:embed skills` `var embeddedSkills embed.FS`
  - `type SkillMeta struct { Name, Description string; Requires []string }`; `ParseSkillMeta(data []byte) (SkillMeta, error)` (exported for the catalog test in Task 3).
  - `RequirementsMet(requires []string, entries []manifest.RealizedEntry) bool` — an id matches a realized entry's `ID`; `type:X` matches any realized entry with `Type == X`.
  - `writeSkills(root string, entries []manifest.RealizedEntry, skills fs.FS) (SkillsReport, error)`, `type SkillsReport struct { Written, Removed, Restored []string }` — for each realized entry of type `skills` whose id has a folder in `skills`, writes every skill folder whose requirements are met, **writing a file only when its bytes differ** (unchanged files keep their mtime, so the index doesn't re-read them), deletes skill folders on disk that weren't written, and lists in `Restored` the files whose on-disk bytes differed from the previous Svalbard write (per-folder `.svalbard-sha256` manifest).
  - `GenerateOpts.Notice func(string)` — `Generate` calls it once per restored skill: `restored svalbard skill <name> (local edits replaced)`; `apply/apply.go:317` passes a function that prints through apply's output.

- [ ] **Step 1: Write failing tests**:
  - `TestBuildEmbeddedCreatesDirectoryEntry`: entry `RelativePath == "skills/skills-core"`, marker exists.
  - `TestWriteSkillsFiltersByRequires`: fixture FS with `skills-core/a` (requires `js-core`), `skills-core/b` (requires none) and `skills-core/c` (requires `type:pmtiles`); realized `{skills-core}` → only `b`; adding any realized `pmtiles` entry → `c` too.
  - `TestWriteSkillsLeavesUnchangedFilesAlone`: second call with nothing changed → every file's mtime unchanged.
  - `TestWriteSkillsRemovesUnsatisfied` (RF1): first call with `js-core` realized writes `a`; second without → `a` removed, `removed == ["skills/skills-core/a"]`.
  - `TestWriteSkillsReportsOverwrittenEdits` (RF2): edit `b/SKILL.md`, call again → file restored, `Restored` lists it, and `Generate`'s `Notice` receives `restored svalbard skill b (local edits replaced)` once.
  - `TestGenerateWithoutSkillsRecipe` (RF5): no `skills` entry → no `skills/skills-core` dir; an existing one from a previous apply is removed, its parent `skills/` kept.
- [ ] **Step 2: Run** `cd host-cli && go test ./internal/toolkit ./internal/builder -run 'Embedded|WriteSkills|WithoutSkills' -count=1` — FAIL.
- [ ] **Step 3: Implement**; `writeSkills` runs after `writeAppPages` in `Generate`, which `apply/apply.go:317` calls before indexing — keep that order.
- [ ] **Step 4: Run** `go test ./internal/... -count=1` — PASS.
- [ ] **Step 5: Commit** `feat(catalog): skills recipe type with embedded, requirement-filtered skills`.

### Task 2: `py-docs` recipe and launcher

**Files:**
- Create: `recipes/tools/py-docs.yaml`
- Modify: `host-cli/internal/builder/pythonvenv.go:210-230` (wrapper generation), `host-cli/internal/catalog/catalog.go` (`Item.PythonLauncher string \`yaml:"python_launcher,omitempty"\``)
- Test: `host-cli/internal/builder/pythonvenv_test.go`

**Interfaces:**
- Produces: recipe

  ```yaml
  id: py-docs
  type: python-package
  venv: svalbard-python
  packages: [openpyxl==<pin>, pypdf==<pin>, reportlab==<pin>, python-docx==<pin>, python-pptx==<pin>]
  python_launcher: py-docs
  ```

  (pins: latest releases at implementation time with wheels for every vault platform). For a recipe with `python_launcher`, the existing "skipping … no compatible wheels" path (`pythonvenv.go:197-203`) becomes an error naming the platform, so a vault never has document skills on a platform without the launcher. A `python_launcher: <name>` writes `bin/<platform>/<name>`: `PYTHONPATH="$PY/tools/<id>" PYTHONNOUSERSITE=1 exec "$PY/<python>" "$@"`, using the same `DRIVE`/`PY` preamble as entry-point wrappers.
- Skills call it as `py-docs path/to/script.py args…` (`bin/<platform>` is on the drive shell's PATH).

- [ ] **Step 1: Write failing tests**: `TestPythonLauncherWrapper` (wrapper content and mode 0755 for each platform), `TestPythonLauncherWithoutEntryPoints` (a package recipe with no `entry_points` but a launcher builds), `TestPythonLauncherFailsWithoutWheels` (a platform with no compatible wheels fails the build with the platform named).
- [ ] **Step 2: Run** `go test ./internal/builder -run PythonLauncher -count=1` — FAIL.
- [ ] **Step 3: Implement**; add the recipe; `scripts/sync-catalog.sh`.
- [ ] **Step 4: Run** the tests, then an end-to-end build into a scratch vault for the host platform and `bin/<platform>/py-docs -c "import openpyxl, pypdf, reportlab, docx, pptx; print('ok')"` → `ok`. For RF3, repeat the build for `linux-arm64` and confirm each wheel's platform tag in `runtime/python/tools/py-docs` (`find … -name '*.so' | xargs file`). Record the per-platform size for the checkpoint.
- [ ] **Step 5: Commit** `feat(catalog): py-docs Python recipe with a launcher`.

### Task 3: Static offline check in `make verify`

**Files:**
- Create: `host-cli/internal/toolkit/skills_lint.go`
- Test: `host-cli/internal/toolkit/skills_lint_test.go` (runs over the real embedded tree)

**Interfaces:**
- Consumes: `ParseSkillMeta`, the embedded catalog (`catalog.LoadEmbedded` or the loader the catalog tests use).
- Produces: `LintSkills(skills fs.FS, recipeIDs map[string]bool) []SkillProblem`; `type SkillProblem struct { Skill, File string; Line int; Reason string }`.

- [ ] **Step 1: Write failing tests** with fixture skills: `TestLintTypeRequirement` (`type:pmtiles` passes; `type:nonsense` fails because no `toolkit.TypeDirs` key matches), `TestLintFlagsInstallCommands` (`npm install x` on line 7 → one problem with line 7), `TestLintAllowsMarkedBlocks` (same inside a `not-for-this-stick` fence → none), `TestLintFlagsRemoteResources` (`<script src="https://cdn…">` in `templates/a.html`), `TestLintUnknownRequires` (RF4), `TestLintRequiresFrontmatter` (missing `name` or `description`), and `TestRealSkillsPassLint` over the embedded tree with the real catalog ids.
- [ ] **Step 2: Run** `go test ./internal/toolkit -run Lint -count=1` — FAIL.
- [ ] **Step 3: Implement**; scan `.md`, `.html`, `.py`, `.sh`, `.js` files; plain `http(s)://` links in prose are allowed (attribution), resource loads (`src=`, `href=` on `link`, `import … from "http`, `fetch("http`) are not.
- [ ] **Step 4: Run** — PASS; `make verify` runs it as part of the host test suite.
- [ ] **Step 5: Commit** `test(skills): fail the build on skills that need the network or missing recipes`.

### Task 4: Offline run-check harness

**Files:**
- Create: `scripts/skill-run-check.sh`
- Create: `host-cli/internal/toolkit/skills/README.md` (authoring rules: layout, frontmatter, steps-first, `examples/check.sh` contract)

**Interfaces:**
- Produces: `scripts/skill-run-check.sh <vault> [skill…]` runs each skill's `examples/check.sh` with `DRIVE=<vault>`, `PATH=<vault>/bin/<platform>:<vault>/.svalbard/runtime/<platform>:$PATH` (the second holds `svalbard-drive`), inside a no-network sandbox (`sandbox-exec -p '(version 1)(allow default)(deny network*)'` on macOS, `unshare -rn` on Linux), and prints `PASS <skill>` or `FAIL <skill>: <last line>`; exits non-zero on any FAIL.
- Contract for `examples/check.sh [artifact]`: with no argument it produces the reference output into a temp dir and validates it; with an argument it validates that file (an artifact a model produced) by the same rules. Exits 0 only when validation passes.

- [ ] **Step 1:** Write the script and a fixture skill `testdata/skills/net-probe` whose check runs `curl -sI https://example.com` and must FAIL under the sandbox; and `testdata/skills/ok` that passes.
- [ ] **Step 2:** Run `scripts/skill-run-check.sh <scratch vault> net-probe ok` → `FAIL net-probe`, `PASS ok`, exit 1.
- [ ] **Step 3: Commit** `test(skills): offline run check for skill examples`.

### Task 5: The seven lean skills

**Files:**
- Create under `host-cli/internal/toolkit/skills/skills-core/`, each with `SKILL.md`, optional `references/`, `scripts/`, and `examples/check.sh`:
  - `vault-research/` — requires `[]`. Procedure: `vault_sources` → `search` with 2–4 words → `search_read` → cite "title (source)" or "title, p. N"; two differently worded empty searches → say the drive doesn't have it. Check: `check.sh` runs `svalbard-drive mcp` over stdio against the vault with a scripted `vault_sources` + `search` and asserts a non-empty result (skips with PASS-note when the vault has no index).
  - `local-models/` — requires `[llama-server]`. Procedure: read installed GGUFs from the vault, RAM rule of thumb (model file size + context KV estimate ≤ 75% RAM), start via the drive menu's chat/agent action, never download. Check: lists the vault's `models/*.gguf` and asserts the chosen one exists.
  - `esp32-firmware/` — requires `[platformio-core, platformio-espressif32]` (the ESP32 platform, framework and compiler closure, and the drive's blink example from `writePlatformIOExample`, come with the latter). Wraps `recipes/actions/docs/embedded-getting-started.md` steps: new project from the drive's offline example, `pio run`, upload and monitor only with hardware and user consent. Check: builds the drive's ESP32 example offline and asserts `firmware.bin` exists.
  - `spreadsheet-file/` — requires `[py-docs]`. Script `scripts/xlsx_tool.py` (create from CSV/JSON with header styling, formulas preserved, read to CSV). Check: CSV → xlsx → reopen asserts cell `B2` value and a `SUM` formula string.
  - `pdf-file/` — requires `[py-docs]`. Scripts: create (reportlab, text + table), merge, split, fill form fields, extract text (pypdf). Check: create 3-page PDF, merge with itself → 6 pages, extract text contains a known string.
  - `word-file/` — requires `[py-docs]`. Script: create .docx with headings, list, table; read paragraphs. Check: reopen asserts heading text and table cell.
  - `slides-file/` — requires `[py-docs]`. Script: create .pptx from a JSON outline (title + bullet slides + one image). Check: reopen asserts slide count and first title.

- [ ] **Step 1:** For each skill, write `examples/check.sh` first and run `scripts/skill-run-check.sh <vault> <skill>` → FAIL (no scripts yet).
- [ ] **Step 2:** Write SKILL.md (steps first, exact commands, `py-docs …` invocations, a "Check your result" last step) and scripts.
- [ ] **Step 3:** Run the run check for all seven → PASS; `go test ./internal/toolkit -run Lint` → PASS.
- [ ] **Step 4:** One commit per skill: `feat(skills): <name> lean skill`.

### Task 6: Presets and packs

**Files:**
- Create: `recipes/skills/skills-core.yaml` (`type: skills`, `strategy: build`, `build: {family: embedded}` — apply routes build recipes only when both are set, `apply.go:201,552` — `deps: [py-docs]`, `menu` hidden or a "Skills" entry that opens `skills/skills-core` in Browse, following how `apps` recipes declare `menu:`), `presets/packs/ai/harnesses.yaml` (add `skills-core` only; `py-docs` arrives through `skills-core`'s deps so deselecting `skills-core` drops it)
- Test: `host-cli/internal/catalog/catalog_test.go`

- [ ] **Step 1: Write failing tests**: `TestHarnessesPackIncludesSkills` (resolving `default-256` with deps yields `skills-core` and `py-docs`), `TestSkillsCoreDeselectable` (a user preset extending `default-256` with `-skills-core` resolves with neither `skills-core` nor `py-docs`), and an end-to-end apply of `skills-core` into a scratch vault (catches strategy routing).
- [ ] **Step 2: Run** `go test ./internal/catalog -run 'Harnesses|Deselectable' -count=1` — FAIL.
- [ ] **Step 3:** Add the recipe and pack lines; `scripts/sync-catalog.sh`; update `size_gb` from the Task 2 measurement.
- [ ] **Step 4: Run** `make verify` — PASS.
- [ ] **Step 5: Commit** `feat(presets): skills come with the AI harnesses`.

### Task 7: Model checks and checkpoint

- [ ] **Step 1:** Fresh `default-256`-based vault (models pack for the test machine's RAM), apply, network disabled. For each of the seven skills, start pi with the catalog's Qwen model and give the skill's example task as a plain request ("make a spreadsheet of these numbers with a total row"). Record: found the skill (y/n), followed it (y/n), result valid (`examples/check.sh <the model's output file>`), attempts, failure text.
- [ ] **Step 2:** Run `scripts/skill-run-check.sh` for all seven on the same vault — all PASS.
- [ ] **Step 3:** Write `docs/plans/<date>-skill-vault-stage2-checkpoint.md` with the tables, sizes, and every adaptation made (a skill rewritten around what the stick has, or left out with its reason). Commit `docs: skill vault stage 2 checkpoint`.
