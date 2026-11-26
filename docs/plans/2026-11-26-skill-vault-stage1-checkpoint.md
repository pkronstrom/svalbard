# Skill vault stage 1 checkpoint: skill search and orientation

Status: built, reviewed and verified 2026-11-26 on branch `skill-vault-stage1`; all six tasks of the [plan](2026-11-26-skill-vault-stage1-search-orientation-plan.md), stage 1 of the [design](2026-11-26-skill-vault-and-js-libraries-design.md). Not yet merged.

No recipe ships skills or JS libraries yet: stages 2 and 3 add them. Everything below was checked with two hand-placed fixture skills and one fixture quickstart.

## Resulting user path

1. **Index.** Markdown under `skills/` and `lib/js/<recipe>/docs/` is indexed with the vault's other documents: one source per file, named by its vault path, one article per `##` section. A vault that holds nothing but skills gets an index. `node_modules` is never walked.
2. **Knowledge search stays knowledge.** `search`, `search_read` and the search screen cover the archives and `books/` as before. Skills are found only when the search names their folder (`source: "skills/"`).
3. **Find and read a skill.** On a drive with a `skills` or `js-lib` recipe the MCP server offers `skill_search` (one result per skill or library, five by default) and `skill_read` (the SKILL.md or quickstart from the index, whole or one section, in parts, with the skill's other files and the folder they are in).
4. **Without an MCP client.** `<drive>/run skills find <words>` and `<drive>/run skills read <id>` print the same, from any folder. `<drive>/run skills` alone says in a few lines what the drive is and how to find a procedure: that, and the server's `instructions` line, are what an agent gets that was not started from the drive's menu.
5. **Orientation from the menu.** pi, opencode and goose each get one generated context file in their own config folder that names the tools the way that harness lists them; oh-my-pi has the same lines in its lean prompt. With skills installed the text sends a model that is asked to build something to `skill_search` first, or, on a vault without a search index, to the skills' folder. Nothing is written to the drive's root.
6. **PATH.** Every harness starts with `<drive>/bin/<platform>` and the folder of `svalbard-drive` first on PATH and with `DRIVE_ROOT` set, so `svalbard-drive skills find chart` works from an agent's shell.
7. **pi on the 8K tier answers again**, also on a drive with the wiki and skills.

## Contracts to keep

- **Skill files are document sources.** Named by vault path, identity is size and modification time with the SHA-256 recorded, replaced atomically, embedded like any article. Articles are `section/N`, 1-based; the text before the first `##` is section 1 ("Overview") and opens with the frontmatter description. A section longer than 16,000 characters continues in the next article under the same heading plus " (cont.)", cut at a line break with nothing else dropped, so the pieces joined by line breaks are the section. A file without frontmatter or headings is one section; a file with NUL bytes is recorded as "not UTF-8 text".
- **One table decides what is indexed** (`commands.documentKinds`, by extension) and one says where (`commands.documentRoots`). `lib/js` lists `lib/js/*/docs/` and takes only the `.md` files directly inside.
- **A missing `skills/` or `lib/js/` is never a prune**, as for `books/`: missing counts as empty only when the manifest records nothing there and the index holds no source from it. A skill folder removed while `skills/` exists is pruned. A folder that cannot be read is an error, not an absence.
- **The empty search scope means every knowledge source**, not every source. `engine.InScope` and `scopeClause` are the one rule, used by keyword search, by both halves of hybrid search (the keyword candidates and the full vector scan) and by the missing-word check. `engine.SkillScope` is both skill roots and nothing else. `SourceFilenames` still returns every source; lists shown to a model or a person leave the skills out.
- **A search over several skills returns at most three sections of one skill** (`sectionsPerSkill`), counted over its SKILL.md and every file below its folder, as a search returns three pages of a document. Every section's title carries the skill's name and skill search groups what it is given, so without the bound one long skill is all that arrives. It holds in keyword search, in hybrid search's keyword candidates and in its full vector scan. A search limited to the skill, or to a folder or file inside it, returns everything. `engine.SkillGroup` is the one rule for which skill a file belongs to.
- **A loosely named source never resolves to a skill file.** A skill source matches only by its whole path. A document collection named `skills` keeps its short name while the drive has no skill files.
- **The skill tools are offered only when `.svalbard/recipes.json` holds a recipe of type `skills` or `js-lib`**, like the wiki tools. They read text from `search.db` and metadata (description, license, version, portability, requires, scripts) from the file on the drive at query time; a file that is gone leaves those empty and the index still answers.
- **A skill's id is its folder's vault path**; a library's is the path of its quickstart. `skill_read` also takes the SKILL.md path, another letter case, or the bare name when only one skill has it, and refuses any path with `..` before it looks anything up.
- **Nothing truncates silently.** `skill_read` parts carry `continues` with the next offset; a file that needs more than one part names its sections; a section indexed in several pieces is returned whole, by number, by heading and through `search_read`; the file list puts markdown first and says when it is cut.
- **The command line searches by keyword**, also on an index with vectors, and finds the drive from `DRIVE_ROOT`, then a working folder that is a drive, then the binary's own place.
- **Nothing executes skill content.** Both tools and the command line only read.
- **The drive context is one text** (`agent.DriveContext`), built per harness with that harness's tool prefix, at most 260 words with every area on (a test). It is the single builder whichever route delivers the text; today the route is a file in each harness's own config folder.
- **No `AGENTS.md` at the drive's root.** The root is the user's, and a harness started there reads such a file on top of its own (see "Measured").
- **Generated context files keep a user's edits**: replaced only while they match the recorded hash.
- **Tool prefixes and context-file places are measured, not assumed**: pi 1.6.1 `mcp__svalbard__`, oh-my-pi 19.2.1 `mcp__svalbard_`, opencode 1.19.6 `svalbard_` with `$XDG_CONFIG_HOME/opencode/AGENTS.md`, goose 1.61.1 `svalbard__` with `$XDG_CONFIG_HOME/goose/.goosehints`. A pin bump needs the check repeated.
- **pi is told a larger window than llama-server has below the Balanced tier** (`piContextWindow`: real + 4,096 − real/16), because pi 1.6.1 keeps a fixed 4,096 tokens of whatever it is told free of every request. A pi pin bump needs `clampMaxTokensToContext` checked again.

## Verification evidence

All on macOS arm64, a scratch vault built by the branch's CLI through normal recipes (WikEM, NHS medicines, Qwen3.5 4B Q4_K_M, Nomic embed v1.5 Q8_0, llama-server b12359, kiwix-tools, pi, oh-my-pi, opencode, goose), `XDG_CONFIG_HOME` and every harness's HOME inside scratch, a no-op `open` first on PATH.

- `make verify` and `make build` passed on the final commit.
- **Independent review** of tasks 1 to 5: no broken search or document-collection contract, RF1 to RF5 hold, 13 defects, none above medium. All are fixed, each with a test that fails without the fix: a long skill crowding others out of the results (bounded per skill, in keyword and hybrid), a split section cut short when asked for by number, headings cut at a comma in the title, lost indentation at a split, a leading rule read as frontmatter, two fence cases and UTF-16 files, an unreadable recipe folder read as absent, the file list hiding markdown files, letter case in a file name, a collection named `skills`, the command line run without `DRIVE_ROOT` and its embedding server, the orientation on a drive without an index, and the `html pack` sentence, which is gone until stage 3 ships the command. One part has no test: the "at least N more" wording for a skill folder too large to list.
- **Document-collection contract, rerun.** A collection of three local PDFs (one in a folder two levels deep) added, planned (`new field-notes: 3 documents`) and applied; edited to add one and drop one (`update field-notes: +1 document, -1 document`), after which apply indexed only the new file and deleted the dropped one. An unchanged run read nothing. Touching one PDF and one SKILL.md re-read exactly those two.
- **Missing roots.** With `books/` moved away, index refused (`the manifest records content installed there`). With `skills/` moved away, index and apply refused (`the index holds sources from skills/, which is missing; not pruning them`), the same for `lib/js/`, and the nine sources were unchanged afterwards. Removing one skill folder pruned that skill's source and kept the other two; putting it back indexed it again.
- **Search-reliability contract, rerun.** `svalbard index --type semantic` interrupted with SIGINT after 384 vectors (71 articles, state `building`); the next ordinary run resumed to 26,608 vectors of 256 dimensions over all 5,872 articles, state `complete`, and all 384 earlier vectors were byte-identical. Replacing one SKILL.md and running `--type full` re-embedded only it.
- **Skill tools over MCP stdio** (`./run mcp`, 10 tools, the instructions line present), on the final binaries:
  - `skill_search "turn my spreadsheet numbers into a picture"` (no word in common with the skill; hybrid) returned `data-dashboard` first, then the Chart.js quickstart, each once though the skill has 10 indexed sections and a reference file with 4.
  - `skill_search "chart" kind=library` returned the quickstart alone, with version 4.5.0, license MIT, portability single-file.
  - `skill_read skills/skills-core/data-dashboard`: 5,993 characters, `continues` naming offset 5994, the ten section headings, three other files and their folder; the call with offset 5994 returned the remaining 1,704 characters ending with the skill's last line.
  - `skill_read` by bare name and section, of a reference file named in another letter case, of `zim/../books/x` (refused) and of a script (refused, with the file's path).
  - `search "chart csv dashboard"` and a sentence about charts in a web page, unscoped and hybrid: WikEM only. `search "chart" source=skills/`: three sections of the one skill that matches. `search "chart" source=skills/skills-core/data-dashboard`: all 14. `search_read` of a skill section. `vault_sources`: no skill.
- **Command line.** `./run skills find make a dashboard from a csv file`, `skills read data-dashboard --section Check`, and usage on an unknown subcommand with exit 1. Run by the binary's path from another folder with no `DRIVE_ROOT`: `skills` alone printed the ten-line orientation naming `<vault>/run skills …`, `skills find chart csv` found the skill, and a long read ended in `continues: <vault>/run skills read skills/skills-core/data-dashboard --offset 5994`.
- **apply writes no `AGENTS.md`** to the vault's root.
- **Harnesses launched from the drive menu** (`./run`, Local AI), Qwen3.5 4B, asked "What tools do you have for building things?" and to run `echo $PATH`:

  | Harness | Tier | Answer | PATH starts with |
  |---|---|---|---|
  | pi 1.6.1 | Fast 8K | named `mcp__svalbard__skill_search` and `mcp__svalbard__skill_read` first, then the search and data tools | pi's own `config/bin`, then `<vault>/bin/macos-arm64`, `<vault>/.svalbard/runtime/macos-arm64` |
  | oh-my-pi 19.2.1 | Balanced 32K | named `mcp__svalbard_skill_search` and `mcp__svalbard_skill_read` first | `<vault>/bin/macos-arm64`, `<vault>/.svalbard/runtime/macos-arm64` |
  | opencode 1.19.6 | Balanced 32K | called `svalbard_skill_search` three times and listed the two skills and the library | the same |
  | goose 1.61.1 | Balanced 32K | listed every tool; named `svalbard__skill_search` and `svalbard__skill_read` as the ones for building | the same |

  These four were recorded before the review fixes; the context text, tool names and PATH they show did not change after.
- **pi at the Fast tier with wiki and skills**, from the drive menu on the final binaries (llama-server `n_ctx_slot = 8192`, first prompt 4,823 tokens): "What is the usual adult dose of amoxicillin? Look it up on the drive and name the source." It searched, read `/Amoxicillin` from WikEM, compacted once (from 7,259 tokens) and answered "500-875 mg PO q12h" with the source. Peak use 7,895 of 8,192 tokens, nothing truncated. Before the fix the same drive got `max_completion_tokens: 1` and "Response was truncated".
- **Model checks, recorded and not gating** (pi, Qwen3.5 4B, 32K, one run each):
  - Dose and side effects of amoxicillin: searched with four words, read two NHS pages, answered and named the source.
  - "Read the whole WikEM article on diabetic ketoacidosis": followed `continues` four times through 24,000 characters. Both lookups ran after the two sentences on query wording and continuation were taken out of the context.
  - "Make me a dashboard page for sales.csv": `skill_search`, `skill_read`, read the template from the skill's folder, wrote a page with the data inlined, the bare `chart.js/auto` import and no network address. Before `skill_read` returned the folder, the same request ended with the model running `find` over the user's home folder for `dashboard.html`. It still fumbled the paths of the skill's check script; that is the fixture skill's wording ("run from this skill's folder"), which stage 2's skills should avoid.

## Measured

Tokens are Qwen3.5 4B's, from llama-server `/tokenize` and, for whole requests, from what the harness sent through a logging proxy.

**Context file**, drive root `/Volumes/SVALBARD` in the text:

| Harness | plain | + skills | wiki | wiki + skills |
|---|---|---|---|---|
| pi, before this branch | 276 | – | 400 | – |
| pi, the plan's text added verbatim | 276 | 343 | 400 | 467 |
| pi | 254 | 319 | 378 | 443 |
| opencode | 240 | 297 | 352 | 409 |
| goose | 242 | 301 | 354 | 413 |
| oh-my-pi (lean prompt) | 96 | 159 | 126 | 189 |

**The budget is what this measures, not the plan's ~400 tokens**: 258 words and 409 to 443 tokens with the wiki and skills, under the 260-word test. The plan sized its number before the wiki's two paragraphs (124 tokens) were in the text. To hold the word cap, two sentences that the search tools' own descriptions carry word for word (how to word a query, how a long article continues) left the text, and one rule joined it: work only in the starting folder and on the drive. A drive without skills ends 22 tokens lighter than before. The results measured on `main` with Open WebUI, which has no context file, came from the tool descriptions alone.

**Tool definitions**: `skill_search` 183 and `skill_read` 208 tokens in pi's request, 391 together. For comparison `search` is 299, `search_read` 221, the eight wiki tools 1,224.

**Whole first request of pi**, one short question, and what pi lets the model write at the Fast tier (8,192):

| Drive has | Prompt tokens | Output allowed, before | Output allowed, after |
|---|---|---|---|
| neither | 2,993 | 1,059 | 4,096 |
| skills | 3,451 | 602 | 4,096 |
| wiki (the default presets, on `main` since v0.6.0) | 4,349 | 1 | 3,316 |
| wiki and skills | 4,807 | 1 | 2,860 |

The "before" column was taken when the prompt was 12 tokens shorter (without the new rule). pi 1.6.1 sends `max_completion_tokens = min(maxTokens, contextWindow − estimated prompt − 4096)`. The 4,096 is a constant in pi (`CONTEXT_SAFETY_TOKENS` in `clampMaxTokensToContext`); changing `reserveTokens` or `maxTokens` separately did not move it. Told the real 8,192 it leaves 4,096 minus the prompt. Told 11,776 it leaves the real window minus the prompt minus 512, and the compaction reserve (4,608, counted from the same number) starts compaction when 1,024 of the real window are left. pi's footer shows "/12k". Skills add 458 tokens to every pi prompt.

First requests from the menu launches at 32K (skills, no wiki), as llama-server counted them: oh-my-pi 5,259, goose 6,457, opencode 9,848 tokens.

**Why there is no root `AGENTS.md`.** As first built, apply wrote the text to the drive's root as well. A harness started there read both: pi's system prompt went from 982 to 1,295 tokens (+313), goose's from 1,294 to 1,502 (+208), oh-my-pi's from 405 to 774 (+369; `--no-rules` does not stop it reading `AGENTS.md` in the working folder). `cd <drive> && ./run` is the usual launch.

**Could the MCP server deliver the orientation?** A shim in front of the drive's server set a marker as `instructions` and offered one resource, `svalbard://orientation`:

| Harness | `instructions` in the prompt | Resources | Same text by file / by `instructions` |
|---|---|---|---|
| pi 1.6.1 | no | neither lists nor reads; a server that declares resources makes pi give the model three more tools, +421 tokens | file only |
| oh-my-pi 19.2.1 | yes, system prompt, "## MCP Server Instructions" | lists resources and templates at connect; nothing reaches the prompt | no context file to compare |
| opencode 1.19.6 | yes, system prompt, `<mcp_instructions>` after its AGENTS.md | does not list; three more tools, +328 tokens | +308 / +355 |
| goose 1.61.1 | yes, system prompt, under the extension's name | does not list; its two resource tools are there either way | +312 / +315 |
| Open WebUI | not checked | | |

So `instructions` cost the same as a file where they arrive, and pi does not show them to the model; a resource is read by nobody unasked. goose and oh-my-pi take the instructions from the `server/discover` handshake (protocol 2026-07-28), pi and opencode from `initialize`.

## Scope and limits

- **Fixtures only.** No shipped skill or library exists yet; frontmatter fields, `requires:` and trimmed packages are stage 2 and 3.
- **Keyword-only vaults** (the default strategy) find skills by shared words. `skill_search` then also returns skills that have only some of the words, and a search with none lists the skills by name; a request with no word in common needs the semantic index. The command line is always keyword-only.
- **A vault without an index** (strategy `none`) still offers the two tools, which answer that there is no index at that vault; the orientation sends the model to `skills/` instead. Tests only, not run on a real vault.
- **The frontmatter read on the drive is not a YAML parser**: one-line values, folded or literal blocks, inline or itemized lists, and `version`/`license` under `metadata:`.
- **Harness shells are not confined.** HOME and the config folders are the drive's, but a harness's shell tool can read and write wherever the user can. The orientation's rule is a request, not a barrier.
- **pi at the Fast tier is tight even so**: a drive with the wiki and skills has 3,400 tokens for the whole conversation, and the lookup above ended 297 tokens under the window. pi estimates the prompt from its characters; if it underestimates by more than the 512 kept back, llama-server stops the answer at the window's end.
- **Open WebUI** gets the two tools and the server's instructions line, nothing else, by design. Not launched in this stage's checks.
- **Not checked:** Linux and physical sticks at run time; oh-my-pi, opencode and goose below the Balanced tier.

## Differences from the plan

- `documentRoot` names extensions and the kinds stay in `documentKinds`: the existing test seam replaces that table, and one table of kinds was the comment's promise.
- A SKILL.md that names nothing is titled by its folder; the plan's "file name" would call every such skill "SKILL".
- Scoped hybrid search is scoped inside the engine, so the MCP `search` no longer fetches an unscoped top 200 to filter.
- `search_read` reads skill sections, so a `search` scoped to the skills does not return paths nothing can open.
- `KeywordOptions.AnyWord`, used only by `skill_search`: the nearest skill instead of "nothing found" when few words match.
- A bound of three sections per skill in searches over several skills, in the engine, so grouping by skill cannot be starved.
- `skill_read` returns `folder`, an absolute path, and a note to save work elsewhere (see the model check).
- `skill_search` and `skill_read` share the search tool's embedding backend, so one embedding model is loaded.
- Two sentences left the shared context text and one rule joined it; `LivePages` and `NoIndex` joined `ContextOptions`; `Libraries` and the `html pack` sentence wait for stage 3.
- No root `AGENTS.md`, so no host copy of the text and no golden files between the modules (owner's decision, on the measurement above).
- The harnesses also get `DRIVE_ROOT`.
- `svalbard-drive` finds its drive from its own place when `DRIVE_ROOT` is unset and the working folder is not a drive. This applies to every subcommand that used the working folder before.
- pi's context window and compaction reserve below the Balanced tier (not in the plan; found by measuring the prompt).

## For stage 2

- **Write a skill file only when its bytes differ.** Identity is size and modification time, and replacing a source sets the semantic state to "building". Skills written unconditionally at every apply, as the app pages are, would be read again each time and would switch hybrid search off for the whole drive until the semantic stage runs again; on a keyword-strategy vault that is until the user reruns it. Stage 2's Task 1 requires it; this is why.
- **A realized `skills` recipe whose folder does not exist is an index error**, as for `books/`. An apply that writes no skill (none of their `requires:` installed) must still create the folder.
- **Skill wording.** "Run it from this skill's folder" sent a 4B model copying its page into the skill. Give commands with the path `skill_read` returns.

## Found on the way, not part of this stage

- After oh-my-pi is closed, `omp __omp_worker_daemon` and the `svalbard-drive mcp` it started keep running, which would hold a stick busy. The daemon also keeps the MCP connection of the earlier run: a second launch showed the first one's server instructions.
- opencode cannot work below the Balanced tier: its first request is 9,848 tokens, more than the 8K window, and at a 16K window it looped in its own compaction, answering nothing (about 9,300 tokens of prompt and tools; its configured output limit of 8,192 does not fit beside them, which is the likely cause). This branch adds about 450 of those tokens, so `main` is in the same place.
- opencode installs `@opencode-ai/plugin` and its dependencies into its config folder at first launch; here it reached npm. What it does on a computer without a network was not checked.
- `TestWikiAppendsSentTogetherAllArrive` in `drive-runtime/internal/mcp` fails about once in a hundred runs, on `main` at 482fd4e as well ("path Log.md leaves the wiki through a link").
