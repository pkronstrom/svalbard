# Skill vault and JS libraries on the stick: design

Status: draft for approval, 2026-11-26. Source brief: the owner's "Svalbard implementation brief" (not committed); this design narrows it to the skills recipe, the JS library recipes, and how the drive's agents find and use them.

Base: `main` at or after `f8acc5b` (PDF document sources, Open WebUI). The current `catalog-refresh-2026-09` branch predates them; nothing here depends on that branch.

## Intent

A local model on the stick, with no network, should be able to build a useful interactive HTML artifact (a dashboard, a CAD tool, a visualization) and hand someone a single file that opens by double-click. To do that it needs three things it lacks today:

1. **Know where it is.** The drive's agents (pi, opencode, goose, Open WebUI) get the Svalbard MCP tools but no orientation: nothing tells the shell-based harnesses they are offline, what is installed, or how to work here.
2. **Procedures.** Agent skills (SKILL.md folders from upstream repos) teach workflows a small model can't improvise. There may be hundreds; none should be loaded up front.
3. **Libraries it can actually use offline.** Pinned browser JS libraries with docs a small model can follow, and a way to turn a page plus its libraries into one self-contained file.

The owner's personal `~/.skill-vault` and its `/sv` meta-skill (search → read → notes, never register everything) is the model for the skills part, rebuilt the Svalbard way: pinned recipes, `apply`/`update`, the drive's own search.

## Goals

- A **generated `AGENTS.md`** that the shell-based drive agents see: what Svalbard is, the offline rules, what this vault contains, and which tools to use for procedures and libraries.
- **A first set of lean, Svalbard-authored skills** (`skills-core`): short SKILL.md procedures written for small local models and for what this stick actually has, covering the use cases of upstream skills (Anthropic's first) rather than shipping those skills. Upstream skill repos pinned as recipes come later.
- **JS library recipes**: a few themed recipes (`js-core`, `js-3d-cad`, `js-maps`, `js-media`), each pinning npm tarballs plus their dependencies, with a Svalbard-written quickstart per library, and one Python recipe (`py-docs`) for document files.
- **Search over skills and library quickstarts** through the existing document sources in `data/search.db`, reached through two new MCP tools and a drive CLI command.
- **`svalbard-drive html pack`**: bundles an agent-written page and the installed libraries into one HTML file, and refuses output that references the network.

## Definition of done

Two tests on a fresh vault from the stick, network disabled, pi with a local Qwen model from the catalog:

1. "Make a world map of population by country, with charts, from the vault's data": the data comes from a small data recipe already in the vault, Natural Earth (public domain, `recipes/maps/natural-earth.yaml`), not from a file the tester supplies. The agent finds the data with the drive's tools (`vault_databases`, `query_describe`, `query_sql`), finds the `geodata` and `data-dashboard` skills, exports the countries' shapes and attributes to GeoJSON with the `geodata` skill's script, draws a choropleth with D3 plus a chart or table from `js-core`, and packs it. The resulting single file opens by double-click (file://) on another machine with the map and charts, data inside the file. A "load another file" button that reads a CSV or GeoJSON is part of the `data-dashboard` skill, not of the test.
2. "Make a parametric spur gear I can export as STL": the agent uses JSCAD and Three.js from `js-3d-cad`, packs one file, and the STL export downloads a valid mesh.

Run each with and without `AGENTS.md` in the harness config. A three-sentence system prompt was already tried in Open WebUI with Qwen3.5 4B and changed nothing measured on lookup tasks (`docs/plans/2026-11-26-open-webui-implementation.md` on `main`); this test checks whether orientation matters for artifact building, where the stakes (CDN links, `npm install`) are different. Record model, attempts and failures in the checkpoint. A pass on the larger model and an honest failure report on the small one is an acceptable outcome; a hypothetical success rate is not.

## Non-goals

- Importing the owner's ~4,300-skill `~/.skill-vault` or its `NOTES.md` (it holds personal and work notes).
- Shipping upstream skill repos in v1. The `skills` type and search are built so pinned upstream repos can be added later as recipes, but the first set is Svalbard-authored only.
- Adapting the text or scripts of Anthropic's `docx`, `pdf`, `pptx` and `xlsx` skills: their license forbids copies outside Anthropic's services and derivative works. Lean skills for those use cases are written from the use case alone, without working from that text. Anthropic's other skills are Apache-2.0 and may be adapted with attribution.
- Registering skills individually with any harness, or one MCP tool per skill or library.
- The brief's derived "compact" (L1) skill rewrites. Originals only, with section-level reads; compaction waits for measured evidence that it helps.
- The full brief JS catalog, the electronics/engineering pack, native toolchains, Node or npm on the stick.
- A browser on the stick for automated artifact testing.
- `html pack` and `AGENTS.md` in Open WebUI. It gets the search tools through the shared MCP server and nothing else in v1: it has no shell to pack with and reads no context file.

## Anti-goals (failure even if it technically works)

- An artifact that silently loads from a CDN, or an agent told to `npm install`, by `AGENTS.md` or an indexed skill.
- `AGENTS.md` grows until small models at the 8K context tier lose the task. Budget: ~400 tokens, enforced by a test.
- Skill text treated as authority, or skill scripts run automatically because a skill was read.
- A skill or doc truncated without saying so.
- Skill and library text drowning out knowledge results in the existing `search` tool.
- Breaking the search contracts ([search reliability](2026-10-06-search-reliability-checkpoint.md)) or the document-collection contracts (`docs/plans/2026-11-26-pdf-collections-checkpoint.md` on `main`).

## Approaches considered

For how agents search skills and library docs:

1. **Index markdown into the existing search index** (chosen). On `main` this means a second entry in `commands.documentKinds`, beside PDF: markdown files become document sources named by vault path, with the existing per-file reconcile (size and mtime identity, recorded SHA-256, no prune on a missing directory), keyword and semantic indexing, and reads from `search.db`. Loosely phrased requests from small models still find the right skill.
2. **A separate word-match catalog like `vault.sh find`.** Small and independent of the search code, but word-only: "make a CSV dashboard" misses a skill described as "interactive charts from tabular data".
3. **Pack skills as a ZIM** so today's search works unchanged. Dropped: it turns originals into HTML, and packing and scripts need the real files.

The owner picked 1.

For the orientation: an always-loaded generated `AGENTS.md` (chosen by the owner) rather than a `svalbard` skill, because small local models often don't load skills unprompted. The detailed how-tos are the lean skills, found by search, so they cost nothing until needed. This replaces the brief's "single gateway skill": the gateway is `AGENTS.md` plus the search tools.

For packaging: a few themed recipes rather than one per library (owner's call), so the catalog gains about five entries, not thirty.

For library sources: pinned npm tarballs (owner agreed). They carry minified and unminified builds, ESM and often UMD formats, `.d.ts` types, README and license. GitHub release archives only for projects not on npm.

## Decided shape

```text
 recipes (YAML, pinned)         host apply + index                   stick
 ──────────────────────         ────────────────────────────────     ─────────────────────────
 skills-core (embedded) ──────▶ skills/skills-core/<skill>/  ──────┐ only skills whose
 js-core, js-3d-cad,  ─bundle─▶ lib/js/<recipe>/node_modules/      │ requires: are installed
 js-maps, js-media    (trimmed) lib/js/<recipe>/docs/*.md ─────────┤
 py-docs ─────python-package──▶ tools/py-docs/ + launcher          │ markdown document sources
                                data/search.db ◀───────────────────┘ (beside books/*.pdf)
                                AGENTS.md, harness configs
                                                                     agent ─ skill_search / skill_read
                                                                           └ svalbard-drive html pack
```

### Recipe types and layout

- **`skills` type** maps to `skills/` in `toolkit.TypeDirs`. `skills-core` installs Svalbard-authored skill folders embedded in host-cli (the `toolkit/apppages` mechanism), written on every apply into `skills/skills-core/<skill>/`. Each follows the Agent Skills layout: SKILL.md with `name`, `description` and `requires:` frontmatter, plus optional `references/`, `templates/`, `examples/` and `scripts/`. **Apply writes only the skills whose `requires:` recipes are installed on this vault**, so `skill_search` never offers a skill the vault can't run (a small model handed an unusable skill reaches for a CDN). One `skills-core` recipe serves every vault this way; there are no per-theme skill recipes in v1. Later recipes of the same type can pin upstream repos as GitHub archive tarballs at a commit through `bundle` (which already extracts `github.com/<o>/<r>/archive/<sha>.tar.gz`); per-repo `license`/`review` metadata and an `exclude:` list arrive with that stage.
- **`requires:` versus recipe `deps:`.** Recipe-level `deps:` (`catalog/deps.go`) pulls the baseline in: `skills-core` depends on `py-docs` from stage 2 and on `js-core` from stage 3. A `requires:` entry is a recipe id or `type:<recipe type>` (any realized recipe of that type, e.g. `type:pmtiles` for `offline-map`). A skill's `requires:` only filters what is written. Removing `js-3d-cad` removes `parametric-cad` from the vault on the next apply.
- **`js-lib` type** maps to `lib/js/`. Each npm tarball is extracted **trimmed**, not whole: `package.json`, the browser builds the quickstart names, README, LICENSE and `.d.ts`. Lucide alone is 4,274 files and big sticks use exFAT with 128 KB clusters, so whole packages would waste gigabytes and copy slowly. This needs the bundle `files:` filter (exact names today, `host-cli/internal/apply/bundle.go`) to accept prefixes or globs. A self-contained build (Mermaid, Turf, `d3.min.js`) is pinned as the single UMD file with no dependency closure.
- **Dependency trees are per recipe**, in npm lockfile layout (nested `node_modules` where versions differ), which esbuild resolves natively. Mermaid's own closure needs two `d3-array` versions, so a catalog-wide one-version rule can't hold. The only catalog rule: the same top-level package may not appear in two recipes. Pins (URL and sha256 per tarball) are generated from a lockfile by a development tool, not written by hand; Node-only packages that closures drag in (commander, iconv-lite) are dropped by the trim.
- A `libraries:` block per top-level package records version, license, a one-line capability description, browser entry (ESM import path or UMD global), measured portability, and its quickstart file.
- **Portability is measured per library, with three levels:** `single-file` (opens by double-click), `served` (runs under the drive's server, needed for PMTiles range reads and module workers) and `directory`. WASM doesn't rule out single-file: esbuild's `binary` loader can inline it. Every result a skill produces is labelled with its level; single-file is the default wherever possible (owner's call: both kinds are allowed, labelled).
- **Quickstarts are Svalbard-authored markdown** embedded in host-cli and written on apply to `lib/js/<recipe>/docs/<package>.md`: the exact import, one tested minimal example, the portability level, and pointers to the package's README and `.d.ts`. They correct for what small models get wrong (Three.js needs `three/addons/…` imports, not the old `THREE.OrbitControls` global; p5 stays on 1.11 because 2.x removed `preload()`). A user `js-lib` recipe without an embedded quickstart gets its package README indexed in its place.
- **`py-docs`** is one `python-package` recipe with openpyxl, pypdf, reportlab, python-docx and python-pptx, so lxml and Pillow resolve once. `builder/pythonvenv.go` installs a package recipe into its own `tools/<id>` tree and puts it on PYTHONPATH only for entry-point wrappers, so the recipe ships a launcher (`py-docs <script>`) that runs the stick's Python with that path. That launcher is what the document skills call. About 30–40 MB per platform.
- **User recipes** (`~/.config/svalbard/recipes`) may use both types; nothing in them depends on host-cli embedded files.
- The recipe-level `license:` stays a summary; per-item licenses live in `libraries:` (and later `repos:`).

### Search

- The document pass indexes markdown (`.md`) as a second kind in `commands.documentKinds`, under these roots in addition to `books/`: `skills/` (SKILL.md and the markdown inside each skill folder) and `lib/js/*/docs/`. Sources are named by vault path, one per file, under the existing per-file identity and reconcile.
- Articles: one per `##`-level section (path `section/N`, title from the heading; the file's frontmatter `name` and `description` go into the first section). A SKILL.md's title in results is its frontmatter `name`.
- **Scopes keep the two worlds apart.** The existing `search` tool and native search exclude `skills/` and `lib/` unless its `source` names one of them. `skill_search` covers only those two roots. `books/` PDFs stay in `search` exactly as on `main`.
- **No index, no skill search.** Indexing runs when any indexed root has files, not only when `zim/` does. With strategy `none`, `skill_search` returns a message that this vault has no search index and names the folders, and `AGENTS.md` says the same.
- **Skill identity** for agents is the skill folder's vault path (for example `skills/skills-core/data-dashboard`).

### Agent-facing tools

Two MCP tools on the existing server (kept to two for the small-context tiers):

- `skill_search(query, kind?, limit=5)`: results grouped to one per skill or library, with id, kind, title, description, `has_scripts`, `license`, and for libraries version and portability. `kind` is `skill` or `library`.
- `skill_read(id, file?, section?, offset?)`: returns the entry file (SKILL.md or quickstart) from `search.db`, whole or by section, continued in parts like `search_read`. It lists the skill folder's other files by path. Markdown files among them can be read with `file`, and templates, scripts or assets are read from disk by agents that have a shell. Nothing cuts silently.

The same operations as `svalbard-drive skills find|read` for agents with only a shell (Claude Code or Codex on a laptop with the stick mounted). Nothing in either path executes skill content.

### Skill shape for small models

- **Steps first.** The first section of each SKILL.md is the numbered procedure; background and alternatives come after it or in `references/`.
- **Templates over prose.** HTML skills ship tested `templates/*.html` that pack as they are, with marked regions the model edits (data, parameters, labels). A gear starts from tested geometry, not from geometry the model invents.
- **The budget is the whole context.** At the 8K tier, what has to fit is the harness system prompt, tool schemas, `AGENTS.md`, the skill, a quickstart and the page being written. The 300–800-token SKILL.md target stays a target, and the run check measures the whole stack with llama-server's `/tokenize`.

### Offline verification of skills

Every skill that ships is verified to work offline, and a skill that doesn't is adapted to use what the vault already has (its tools, Python packages, JS libraries, datasets, maps, search) rather than shipped as is or dropped by default.

- **Static check, in `make verify`:** a catalog test fails a shipped skill that tells the agent to install or fetch (`npm`, `pnpm`, `npx`, `pip install`, `uv add`, `curl`, `wget`, `git clone`, `http(s)://` resources to load) outside a block marked as not for this stick, or whose `requires:` names a recipe the catalog lacks.
- **Run check, per skill:** each skill has an `examples/` task with a known-good result and a script that performs its steps with the stick's runtime (`py-docs`, `svalbard-drive html pack`, the drive's query tools) and validates the output: an xlsx that reopens with the expected cells, a PDF with the expected pages, a GeoJSON with the expected feature count, a packed HTML that loads in Chromium and Firefox with networking disabled, with its exports (STL, downloads) exercised. A static URL scan alone doesn't prove offline behaviour. Run with the network disabled; results go in the checkpoint.
- **Model check, per skill:** a local model on the stick does the skill's example task once from the prompt alone; outcome and failures are recorded, not gated.
- **Adapting:** when a use case or a later upstream skill needs something missing (a network service, an uninstalled tool), the first choice is rewriting its steps around what Svalbard ships; the second is adding a small recipe or Python package the stage owns; only then is it left out, with the reason in the checkpoint. The same rule applies when upstream skill repos arrive later.

### Packing

- `svalbard-drive html pack <page.html> [-o out.html] [--dir]` embeds **esbuild through its Go API** in `svalbard-drive` (no separate binary recipe; works on every runtime target). It bundles the page's module scripts with bare imports resolved against the installed `lib/js/*/node_modules` trees, inlines JS, CSS, imported data files (CSV, JSON, GeoJSON) and WASM where the library allows it, and copies license notices of bundled packages into a trailing comment.
- After building it **checks the output**: any `http(s)://` or protocol-relative resource reference, a missing local asset, a relative `fetch` (which file:// can't serve), or a library measured `served` or `directory` fails single-file mode with a message naming the cause. `--dir` emits a portable directory instead. It warns when a single file passes about 10 MB.
- Browser targets: current Chromium and Firefox; Safari best effort. Hardware APIs (Web Serial, WebUSB) are Chromium-only and say so.

### Orientation

- `toolkit.Generate` writes `AGENTS.md` at the drive root from installed recipes: offline rules, a one-line inventory per installed area (search sources, maps, databases, apps, skills, JS libraries), and the tool and command names to use. Sections for absent areas are omitted.
- `agent.PrepareClientLaunchConfig` places the same text where each shell harness reads a global context file in its isolated config: pi and opencode read `AGENTS.md`; goose needs either `CONTEXT_FILE_NAMES` or a `.goosehints` copy (verify against the pinned goose). Tool names in the text use each harness's own MCP prefix (pi `mcp__svalbard__`, omp `mcp__svalbard_`, opencode and goose as their pinned versions name them); omp gets the skills line in its lean system prompt.
- It also puts `<drive>/bin/<platform>` and the directory holding `svalbard-drive` on every harness's PATH, so `py-docs` and `svalbard-drive html pack` resolve from an agent's shell. Today agents inherit PATH unchanged.
- The skill tools are offered only on vaults with a `skills` or `js-lib` recipe, like the wiki tools, so other vaults pay no schema tokens.
- The MCP server sets its `instructions` to a one-line pointer to `skill_search`. Open WebUI gets that and the two tools, nothing more.
- Separate from the host-side `svalbard skill install` skill, which stays as is. The open drive-skill tasks are folded into this work.

### Catalog content

**Recipes**

| Recipe | Contents | Size (unmeasured estimate, all platforms) |
|---|---|---|
| `skills-core` | all lean skills below; apply writes those whose `requires:` are installed | under 5 MB |
| `js-core` | Alpine, Pico CSS, Lucide (trimmed), Chart.js, D3, Papa Parse, DOMPurify, MiniSearch, fflate, Marked, Mermaid (UMD), SheetJS CE (official distribution), pdf-lib, Math.js, one or two `@fontsource` faces | ~20–40 MB trimmed |
| `js-3d-cad` | Three.js with `examples/jsm` addons, `@jscad/modeling` and its STL serializer | ~15–25 MB |
| `js-maps` | MapLibre GL, PMTiles, Turf, proj4, topojson-client | ~10–35 MB |
| `js-media` | p5 1.11, Tone.js, Matter.js | ~12 MB |
| `py-docs` | openpyxl, pypdf, reportlab, python-docx, python-pptx, launcher | ~30–40 MB per platform |

Cut from the brief's shortlist, with reasons in the checkpoint: ECharts (duplicates Chart.js; 60 MB in 1,347 files), PDF.js (browsers render PDFs; `py-docs` extracts), Vue, Plotly, ELK, Replicad, Manifold (until tested), DuckDB-Wasm (already the `duckdb-wasm` app recipe). `js-docs` is dropped; its useful parts are in `js-core`. `recipes/tools/maplibre-vendor.yaml`, which today fetches unpinned files from unpkg, keeps its id and paths (map recipes depend on it through `dep-defaults.yaml`, and saved selections name it) but is re-sourced from the same pinned npm tarballs as `js-maps`, at equal versions; both stay on maplibre-gl 5.x because 6.x is ESM-only and breaks the viewer.

**Lean skills in `skills-core`**

| Skill | Use case (upstream source) | Requires |
|---|---|---|
| `html-app` | single-file offline app, distinctive design, themes (`web-artifacts-builder`, `frontend-design`, `theme-factory` themes adapted with Apache-2.0 attribution; fonts are the ones the stick ships) | `js-core` |
| `data-dashboard` | charts and tables from CSV/JSON or vault databases, data inlined, load-another-file | `js-core` |
| `geodata` | features from a vault GeoPackage to GeoJSON (stdlib-only script), drawn with D3 in one file | `js-core`, `py-docs` (its launcher runs the script) |
| `offline-map` | interactive maps over the stick's PMTiles with MapLibre; `served` | `js-maps`, `type:pmtiles` |
| `diagram` | flowcharts and diagrams in Mermaid, packed into one file | `js-core` |
| `parametric-cad` | parametric parts in mm from a tested template, 3D preview, STL export; printability checks | `js-3d-cad` |
| `generative-art` | seeded sketches with parameter controls (`algorithmic-art`, its CDN template rewritten) | `js-media` |
| `spreadsheet-file`, `pdf-file`, `word-file`, `slides-file` | .xlsx, .pdf, .docx, .pptx (Anthropic's use cases only, written without their text) | `py-docs` |
| `vault-research` | search, read and cite the stick's archives and PDFs; say when the vault doesn't have it | (none) |
| `esp32-firmware` | wraps the offline PlatformIO path (`recipes/actions/docs/embedded-getting-started.md`); upload and monitor depend on hardware | `platformio-core`, `platformio-espressif32` |
| `local-models` | choose and start a local model by RAM and context on this stick | `llama-server` |

Skill tiers beyond v1, recorded for later stages:

- **Upstream as-is (later `skills-software`):** mattpocock `tdd` and `diagnosing-bugs` (MIT), `animation-vocabulary` (MIT).
- **Adapt later:** `threejs-skills` (loads Three from a CDN; ~3,000 words each; 27B+ tier), `doc-coauthoring` (Apache-2.0; drop its subagent steps).
- **Left out:** Anthropic `docx`/`pdf`/`pptx`/`xlsx` (license), `webapp-testing` (no browser automation on the stick), `remotion` (Node and Chrome; not an OSI license), `manim` (manim, ffmpeg, LaTeX), `strudel` (GPL; REPL not on the stick), impeccable, taste-skill and ui-ux-pro-max (React, Tailwind, CDN fonts; `html-app` covers them), skill-creator, mcp-builder and context-engineering (for agent builders), the Claude- or company-specific Anthropic skills (`brand-guidelines`, `internal-comms`, `slack-gif-creator`, `claude-api`, `academy-guide`, `discernment-nudge`), and aggregator repos whose license covers only their scaffolding (pin originals instead).

**Presets (owner's call).** `skills-core` and the JS and Python recipes come with the AI harnesses: the `ai/harnesses` pack (today pulled in by `default-256`) gains `skills-core`, `js-core` and `py-docs`; a new `ai/artifacts` pack holds `js-3d-cad`, `js-maps` and `js-media` and is also extended by `default-256`. Any of them can be deselected in the wizard like other items (saved `desired.items` stays authoritative), and a vault that adds a harness gets them by default.

## Staging

Overall size: **XL**. Three L-sized pieces, each shippable on its own, starting from `main`:

1. **Docs search and orientation**: the markdown document kind and roots, scope separation in `search`, `skill_search`/`skill_read` and the CLI, `AGENTS.md` generation and harness wiring. Risk that raises it: regressions in the search and document-collection contracts; both checkpoints' verification steps must be rerun.
2. **Lean skills that need no JS**: the `skills` type with `requires:` filtering, `py-docs` and its launcher, the static offline check, and `vault-research`, `local-models`, `esp32-firmware` and the four document skills, each with its run and model check; preset and pack changes.
3. **JS libraries and packing**: the `js-lib` type with trimmed extraction and per-recipe trees, the pin generator, the four JS recipes with quickstarts, `html pack`, `maplibre-vendor` moved onto `js-maps`, the HTML skills (`html-app`, `data-dashboard`, `geodata`, `offline-map`, `diagram`, `parametric-cad`, `generative-art`) with templates, and the two definition-of-done tests. Prerequisite for test 1, checked first because it may already be broken: on `main`, `gpkg` has no entry in `toolkit.TypeDirs` and `inspect.Databases` lists only `data/*.sqlite`, so it is unverified where Natural Earth's `.gpkg.zip` lands and the drive's query tools don't see it. The stage makes an installed GeoPackage land unzipped under `data/` and appear in `vault_databases` and `query_sql`.

Later, not planned here: upstream skill repos as pinned recipes, with per-repo license, review flags and `exclude:`.

## Left to the builder

Section splitting details, tool descriptions and result JSON, the goose context-file mechanism, the precise library pins and trim lists, the pin generator's shape, skill and quickstart wording, esbuild version, the part size for `skill_read`, and which fonts ship.

## Calls made without the owner (check these)

- Trimmed npm packages on the stick, not whole packages or the original `.tgz`; the recipe's URL and sha256 pins allow a byte-identical refetch on a networked host.
- Per-recipe dependency trees; only a top-level package in two recipes is a conflict.
- esbuild embedded in `svalbard-drive` instead of a binary recipe.
- Two tools (`skill_search`, `skill_read`) cover skills and library quickstarts; `search` excludes them unless scoped.
- `requires:` filters which skills apply writes; recipe `deps:` pulls in the baseline.
- File-producing skills use `py-docs` through a launcher; HTML skills use the JS libraries.
- Offline verification as three layers (static in `make verify`, scripted run per skill including a real browser load, one recorded model run per skill); only the first two gate.
- The recipe contents, cuts and skill tiers above (merged from the Codex Sol and Opus layout reviews).
- Browser targets: Chromium and Firefox; Safari best effort.
- Test 1 uses Natural Earth from the vault (owner's suggestion); making GeoPackages visible to the drive's query tools joins stage 3.
- Open WebUI gets the tools only.
- With strategy `none`, skill search is off and says so.
