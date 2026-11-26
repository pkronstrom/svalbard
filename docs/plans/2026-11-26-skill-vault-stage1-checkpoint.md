# Skill vault stage 1 checkpoint: skill search and orientation

Status: built and verified 2026-11-26 on branch `skill-vault-stage1`; all six tasks of the [plan](2026-11-26-skill-vault-stage1-search-orientation-plan.md), stage 1 of the [design](2026-11-26-skill-vault-and-js-libraries-design.md). Not yet merged. Two decisions are open, listed at the end.

No recipe ships skills or JS libraries yet: stages 2 and 3 add them. Everything below was checked with two hand-placed fixture skills and one fixture quickstart.

## Resulting user path

1. **Index.** Markdown under `skills/` and `lib/js/<recipe>/docs/` is indexed with the vault's other documents: one source per file, named by its vault path, one article per `##` section. A vault that holds nothing but skills gets an index. `node_modules` is never walked.
2. **Knowledge search stays knowledge.** `search`, `search_read` and the search screen cover the archives and `books/` as before. Skills are found only when the search names their folder (`source: "skills/"`).
3. **Find and read a skill.** On a drive with a `skills` or `js-lib` recipe the MCP server offers `skill_search` (one result per skill or library, five by default) and `skill_read` (the SKILL.md or quickstart from the index, whole or one section, in parts, with the skill's other files and the folder they are in). `./run skills find <words>` and `./run skills read <id>` print the same for an agent that only has a shell; so does the binary run by its path on a mounted stick, from any folder.
4. **Orientation.** pi, opencode and goose each get one generated context file that names the tools the way that harness lists them; oh-my-pi has the same lines in its lean prompt. With skills installed the text sends a model that is asked to build something to `skill_search` first, or, on a vault applied without a search index, to the skills' folder. `apply` writes the same text with bare tool names to `AGENTS.md` at the drive's root.
5. **PATH.** Every harness starts with `<drive>/bin/<platform>` and the folder of `svalbard-drive` first on PATH and with `DRIVE_ROOT` set, so `svalbard-drive skills find chart` works from an agent's shell.

## Contracts to keep

- **Skill files are document sources.** Named by vault path, identity is size and modification time with the SHA-256 recorded, replaced atomically, embedded like any article. Articles are `section/N`, 1-based; the text before the first `##` is section 1 ("Overview") and opens with the frontmatter description; a section longer than 16,000 characters continues in the next article under the same heading plus " (cont.)". A file without frontmatter or headings is one section.
- **One table decides what is indexed** (`commands.documentKinds`, by extension) and one says where (`commands.documentRoots`). `lib/js` lists `lib/js/*/docs/` and takes only the `.md` files directly inside.
- **A missing `skills/` or `lib/js/` is never a prune**, as for `books/`: missing counts as empty only when the manifest records nothing there and the index holds no source from it. A skill folder removed while `skills/` exists is pruned.
- **The empty search scope means every knowledge source**, not every source. `engine.InScope` and `scopeClause` are the one rule, used by keyword search, by both halves of hybrid search (the keyword candidates and the full vector scan) and by the missing-word check. `engine.SkillScope` is both skill roots and nothing else. `SourceFilenames` still returns every source; lists shown to a model or a person leave the skills out.
- **A search over several sources returns at most three sections of one skill file** (`sectionsPerSkillFile`), as it returns three pages of a document: every section's title carries the file's title, and skill search groups what it is given, so without the bound one long skill is all that arrives. It holds in keyword search, in hybrid search's keyword candidates and in its full vector scan. A search limited to the file returns every section.
- **A loosely named source never resolves to a skill file.** A skill source matches only by its whole path. A document collection named `skills` keeps its short name while the drive has no skill files.
- **The skill tools are offered only when `.svalbard/recipes.json` holds a recipe of type `skills` or `js-lib`**, like the wiki tools. They read text from `search.db` and metadata (description, license, version, portability, requires, scripts) from the file on the drive at query time; a file that is gone leaves those empty and the index still answers.
- **A skill's id is its folder's vault path**; a library's is the path of its quickstart. `skill_read` also takes the SKILL.md path, another letter case, or the bare name when only one skill has it, and refuses any path with `..`.
- **Nothing truncates silently.** `skill_read` parts carry `continues` with the next offset; a file that needs more than one part names its sections; a section indexed in several pieces is returned whole, whichever way it is asked for; the file list puts markdown first and says when it is cut.
- **The command line searches by keyword**, also on an index with vectors, and finds the drive from `DRIVE_ROOT`, then a working folder that is a drive, then the binary's own place.
- **Nothing executes skill content.** Both tools and the command line only read.
- **The drive context is one text** (`agent.DriveContext`), built per harness with that harness's tool prefix, at most 260 words with every area on (a test). The host's copy for the root `AGENTS.md` is held to it by six golden files in `drive-runtime/internal/agent/testdata`.
- **Generated context files keep a user's edits**: replaced only while they match the recorded hash. The root file's record is `.svalbard/AGENTS.md.svalbard-sha256`.
- **Tool prefixes and context-file places are measured, not assumed**: pi 1.6.1 `mcp__svalbard__`, oh-my-pi 19.2.1 `mcp__svalbard_`, opencode 1.19.6 `svalbard_` with `$XDG_CONFIG_HOME/opencode/AGENTS.md`, goose 1.61.1 `svalbard__` with `$XDG_CONFIG_HOME/goose/.goosehints`. A pin bump needs the check repeated.

## Verification evidence

All on macOS arm64, a scratch vault built by the branch's CLI through normal recipes (WikEM, NHS medicines, Qwen3.5 4B Q4_K_M, Nomic embed v1.5 Q8_0, llama-server b12359, kiwix-tools, pi, oh-my-pi, opencode, goose), `XDG_CONFIG_HOME` and every harness's HOME inside scratch, a no-op `open` first on PATH.

- `make verify` and `make build` passed on the final commit.
- An independent review of tasks 1 to 5 found no broken contract and 13 defects, none above medium. They are fixed, each with a test: a long skill crowding others out of the results, a split section cut short when asked for by number, headings cut at a comma in the title, lost indentation at a split, a leading rule read as frontmatter, two fence cases and UTF-16 files, an unreadable recipe folder read as absent, the file list hiding markdown files, letter case in a file name, a collection named `skills`, the command line run without `DRIVE_ROOT` and its embedding server, and the orientation on a drive without an index. Two parts are not: the "at least N more" wording for a skill folder too large to list has no test, and the context still names `html pack`, which stage 3 builds, behind the `js-lib` gate.
- **Document-collection contract, rerun.** A collection of three local PDFs (one in a folder two levels deep) added, planned (`new field-notes: 3 documents`) and applied; edited to add one and drop one (`update field-notes: +1 document, -1 document`), after which apply indexed only the new file and deleted the dropped one. An unchanged run read nothing. Touching one PDF and one SKILL.md re-read exactly those two.
- **Missing roots.** With `books/` moved away, index refused (`the manifest records content installed there`). With `skills/` moved away, index and apply refused (`the index holds sources from skills/, which is missing; not pruning them`), the same for `lib/js/`, and the nine sources were unchanged afterwards. Removing one skill folder pruned that skill's source and kept the other two; putting it back indexed it again.
- **Search-reliability contract, rerun.** `svalbard index --type semantic` interrupted with SIGINT after 384 vectors (71 articles, state `building`); the next ordinary run resumed to 26,608 vectors of 256 dimensions over all 5,872 articles, state `complete`, and all 384 earlier vectors were byte-identical. Replacing one SKILL.md and running `--type full` re-embedded only it.
- **Skill tools over MCP stdio** (`./run mcp`, 10 tools, the instructions line present):
  - `skill_search "turn my spreadsheet numbers into a picture"` (no word in common with the skill; hybrid) returned `data-dashboard` first, then the Chart.js quickstart, each once though the skill has 10 indexed sections and a reference file with 4.
  - `skill_search "chart" kind=library` returned the quickstart alone, with version 4.5.0, license MIT, portability single-file.
  - `skill_read skills/skills-core/data-dashboard`: 5,993 characters, `continues` naming offset 5994, the ten section headings, three other files and their folder; the call with offset 5994 returned the remaining 1,704 characters ending with the skill's last line.
  - `skill_read` by bare name and section, of a reference file's section, of `zim/../books/x` (refused) and of a script (refused, with the file's path).
  - `search "chart csv dashboard"` and a sentence about charts in a web page, unscoped and hybrid: WikEM only. `search "chart" source=skills/`: the skill's sections. `search_read` of a skill section. `vault_sources`: no skill.
- **Command line.** `./run skills find make a dashboard from a csv file`, `skills read data-dashboard --section Check`, a long read ending in `continues: svalbard-drive skills read ... --offset 5994`, and usage on a missing subcommand with exit 1. Run by the binary's path from another folder with no `DRIVE_ROOT`, `skills find chart csv` found the skill and the continuation line carried that path.
- The MCP calls above were repeated on the final binaries after the review fixes; `search "chart" source=skills/` then returned three sections of each of the two files.
- **Harnesses launched from the drive menu** (`./run`, Local AI), Qwen3.5 4B, asked "What tools do you have for building things?" and to run `echo $PATH`:

  | Harness | Tier | Answer | PATH starts with |
  |---|---|---|---|
  | pi 1.6.1 | Fast 8K | named `mcp__svalbard__skill_search` and `mcp__svalbard__skill_read` first, then the search and data tools | pi's own `config/bin`, then `<vault>/bin/macos-arm64`, `<vault>/.svalbard/runtime/macos-arm64` |
  | oh-my-pi 19.2.1 | Balanced 32K | named `mcp__svalbard_skill_search` and `mcp__svalbard_skill_read` first | `<vault>/bin/macos-arm64`, `<vault>/.svalbard/runtime/macos-arm64` |
  | opencode 1.19.6 | Balanced 32K | called `svalbard_skill_search` three times and listed the two skills and the library | the same |
  | goose 1.61.1 | Balanced 32K | listed every tool; named `svalbard__skill_search` and `svalbard__skill_read` as the ones for building | the same |

- **Model checks, recorded and not gating** (pi, Qwen3.5 4B, 32K, one run each):
  - Dose and side effects of amoxicillin: searched with four words, read two NHS pages, answered and named the source.
  - "Read the whole WikEM article on diabetic ketoacidosis": followed `continues` four times through 24,000 characters. Both lookups ran after the two sentences on query wording and continuation were taken out of the context.
  - "Make me a dashboard page for sales.csv": `skill_search`, `skill_read`, read the template from the skill's folder, wrote a page with the data inlined, the bare `chart.js/auto` import and no network address. Before `skill_read` returned the folder, the same request ended with the model searching the disk for `dashboard.html`. It still fumbled the paths of the skill's check script; that is the fixture skill's wording ("run from this skill's folder"), which stage 2's skills should avoid.

## Measured

Tokens are Qwen3.5 4B's, from llama-server `/tokenize` and, for whole requests, from what the harness sent through a logging proxy.

**Context file**, drive root `/Volumes/SVALBARD` in the text:

| Harness | plain | + skills | + skills, libraries | wiki | wiki + skills | everything |
|---|---|---|---|---|---|---|
| pi | 242 | 307 | 323 | 366 | 431 | 447 |
| opencode | 228 | 285 | 301 | 340 | 397 | 413 |
| goose | 230 | 289 | 305 | 342 | 401 | 417 |
| oh-my-pi (lean prompt) | 85 | 148 | 164 | 115 | 178 | 194 |

Before this branch pi's file was 276 tokens plain and 400 with the wiki. The plan's text added verbatim would have been 483 with everything; the two dropped sentences and a tighter skills paragraph bring it to 447 and 258 words. The wiki's two paragraphs are 124 of those tokens.

**Tool definitions**: `skill_search` 183 and `skill_read` 208 tokens in pi's request, 391 together. For comparison `search` is 299, `search_read` 221, the eight wiki tools 1,224.

**Whole first request of pi**, one short question:

| Drive has | Prompt tokens | Output pi allows at Fast (8K) |
|---|---|---|
| neither | 2,981 | 1,059 |
| skills | 3,439 | 602 |
| wiki (as on `main` today, 34 more there) | 4,337 | 1 |
| wiki and skills | 4,795 | 1 |
| wiki, skills, libraries | 4,811 | 1 |

pi sends `max_completion_tokens` = window − prompt − `reserveTokens`, and `ensurePiSettings` sets `reserveTokens` to 4,096 at 8K. With the wiki installed pi cannot answer at the Fast tier, before and after this branch; skills cost 458 tokens of every prompt and at Fast take 457 of the 1,059 output tokens a wiki-less drive had.

First requests from the menu launches above (skills and libraries, no wiki), as llama-server counted them: pi 3,488, oh-my-pi 5,259, goose 6,457, opencode 9,848 tokens. They were taken before the tool descriptions lost their last 30 tokens.

## Scope and limits

- **Fixtures only.** No shipped skill or library exists yet; frontmatter fields, `requires:` and trimmed packages are stage 2 and 3.
- **`html pack` does not exist yet.** The context names it only when a `js-lib` recipe is installed, which nothing can do before stage 3.
- **Keyword-only vaults** (the default strategy) find skills by shared words. `skill_search` then also returns skills that have only some of the words, and a search with none lists the skills by name; a request with no word in common needs the semantic index. The command line is always keyword-only.
- **A vault without an index** (strategy `none`) still offers the two tools, which answer that there is no index; the orientation sends the model to `skills/` instead. Not run on a real vault, only in tests.
- **The frontmatter read on the drive is not a YAML parser**: one-line values, folded or literal blocks, inline or itemized lists, and `version`/`license` under `metadata:`.
- **Open WebUI** gets the two tools and the server's instructions line, nothing else, by design. Not launched in this stage's checks.
- **Not checked:** Linux and physical sticks at run time; oh-my-pi, opencode and goose below the Balanced tier (opencode's first request alone is larger than the Fast window, and this branch adds about 450 of its 9,848 tokens).

## Differences from the plan

- `documentRoot` names extensions and the kinds stay in `documentKinds`: the existing test seam replaces that table, and one table of kinds was the comment's promise.
- A SKILL.md that names nothing is titled by its folder; the plan's "file name" would call every such skill "SKILL".
- Scoped hybrid search is scoped inside the engine, so the MCP `search` no longer fetches an unscoped top 200 to filter.
- `search_read` reads skill sections, so a `search` scoped to the skills does not return paths nothing can open.
- `KeywordOptions.AnyWord`, used only by `skill_search`: the nearest skill instead of "nothing found" when few words match.
- A bound of three sections per skill file in searches over several sources, in the engine, so grouping by skill cannot be starved.
- `skill_read` returns `folder`, an absolute path, and a note to save work elsewhere (see the model check).
- `skill_search` and `skill_read` share the search tool's embedding backend, so one embedding model is loaded.
- Two sentences left the shared context text and `LivePages` joined `ContextOptions`, both to carry `main`'s wiki paragraphs within the 260-word cap.
- The harnesses also get `DRIVE_ROOT`; the root file's change record is under `.svalbard/`, not beside the file.
- `svalbard-drive` finds its drive from its own place when `DRIVE_ROOT` is unset and the working folder is not a drive. This applies to every subcommand that used the working folder before.
- `ContextOptions.NoIndex` and `GenerateOpts.NoSearchIndex`, for the design's "with strategy none the orientation says so".

## Open decisions

1. **A harness started from the drive's root reads the orientation twice**: its own context file and the root `AGENTS.md`. Measured: pi's system prompt 982 → 1,295 tokens, goose 1,294 → 1,502, oh-my-pi 405 → 774 (`--no-rules` does not stop it reading `AGENTS.md` in the working folder). `cd <drive> && ./run` is the usual launch. Options: a short root file for agents that were not started from the menu (S); write only the difference into the harness file when the launch folder is the drive (M); accept it.
2. **The context text exceeds ~400 tokens with the wiki and skills together** (pi 431 to 447). It holds the 260-word cap. Getting under 400 means cutting the wiki's or the rules' wording, which this stage did not touch.

Found on the way, not part of this stage:

- After oh-my-pi is closed, `omp __omp_worker_daemon` and the `svalbard-drive mcp` it started keep running, which would hold a stick busy.
- `TestWikiAppendsSentTogetherAllArrive` in `drive-runtime/internal/mcp` fails about once in a hundred runs, on `main` at 482fd4e as well ("path Log.md leaves the wiki through a link").
- For stage 2: a skill file rewritten on every apply with the same bytes gets a new modification time, is indexed again, and puts a semantic index back to "building" until the next semantic run. Write a skill file only when its bytes differ.
- pi at the Fast tier with the wiki installed (above).
- opencode installs `@opencode-ai/plugin` and its dependencies into its config folder at first launch; here it reached npm. What it does on a computer without a network was not checked.
