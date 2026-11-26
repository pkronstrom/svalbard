# Skill vault stage 1: skill search and orientation — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Markdown under `skills/` and `lib/js/*/docs/` becomes searchable through two new MCP tools and a drive CLI command, stays out of the knowledge `search`, and every shell harness on the drive gets one generated orientation file that points at it.

**Architecture:** The host document pass (PDF on `main`) gains document roots with their own kinds: `books/` keeps `.pdf`; `skills/` and `lib/js/*/docs/` index `.md` with a markdown reader that splits on `##` sections. The drive engine excludes those roots from unscoped search and hybrid search; a new `SkillCapability` searches and reads only them. `agent.go`'s existing pi context becomes a shared drive context written for pi, opencode and goose, with a skills paragraph when skills are installed.

**Tech Stack:** Go 1.26 (host-cli, drive-runtime), SQLite FTS5 via `searchdb`, mcp-go.

**Spec:** [2026-11-26-skill-vault-and-js-libraries-design.md](2026-11-26-skill-vault-and-js-libraries-design.md) (sections Search, Agent-facing tools, Orientation, Staging 1).

**Base:** branch from `main` at or after `f8acc5b`. Paths below are `main`'s.

## Global Constraints

- Document sources are named by vault path, one per file; identity is size and modification time with the SHA-256 recorded (document-collections contract, `docs/plans/2026-11-26-pdf-collections-checkpoint.md`).
- A missing or unlistable index root is never a prune unless the manifest records nothing there and the index holds no source from it.
- One OS-backed writer lock covers keyword and semantic work; atomic source replacement; exact-model reader compatibility; bounded full-detail snapshots; cancellation propagates (search reliability checkpoint).
- PDF articles stay `page/N`; markdown articles are `section/N`, 1-based.
- `search` and native search never return a source under `skills/` or `lib/` unless the caller's scope names that root.
- `skill_search` defaults to 5 results; `skill_read` never truncates without saying so.
- Generated orientation text: at most ~400 tokens. Unit tests cap it at 260 words (identifiers like `mcp__svalbard__skill_search` cost several tokens each, so a words×1.3 estimate undercounts); Task 6 measures real tokens with llama-server `/tokenize`.
- Generated files keep user edits: drive copies use `agent.writeGeneratedFile`; the host copy uses a host-side twin with the same `.svalbard-sha256` marker format.
- MCP tool names differ per harness: pi `mcp__svalbard__<tool>`, omp `mcp__svalbard_<tool>` (both from `agent.go` on `main`); opencode and goose prefixes are read from their pinned versions in Task 5, never assumed.
- Keep Aven refs out of commits and committed docs.

## Review Focus

1. A SKILL.md with no frontmatter, or frontmatter but no `##` headings → indexed as one section titled by the file name, not skipped.
2. A vault with skills but no `zim/` and no `books/` → `RunConfiguredIndex` indexes the skills instead of printing "No ZIM archives or documents to index."
3. Unscoped hybrid search on a vault with semantic vectors for skills → no skill chunk in results (the vector path, not only FTS, is filtered).
4. `skills/` deleted by hand → the index keeps the skill sources (a missing root is never a prune, whatever the manifest says), and `skill_search` still answers; one skill folder removed while `skills/` exists → only that skill's sources are pruned.
5. `skill_read` on a SKILL.md longer than one part → returns the first part with `continues`, and the next call with the offset returns the rest.

Each has a test in the task that owns the code (marked RF1–RF5).

---

### Task 1: Document roots and the markdown reader (host)

**Files:**
- Modify: `host-cli/internal/commands/index_documents.go` (documentKinds, ScanDocuments, IsDocumentSource)
- Modify: `host-cli/internal/commands/index.go:210-245` (scanSources, sourceDir)
- Modify: `host-cli/internal/commands/index_configured.go:45-75` (early return, semantic gate)
- Create: `host-cli/internal/commands/markdown_reader.go`
- Test: `host-cli/internal/commands/markdown_reader_test.go`, `host-cli/internal/commands/index_documents_test.go`

**Interfaces:**
- Produces:
  - `type documentRoot struct { dir string; kinds map[string]documentKind; within func(rel string) bool }`
  - `var documentRoots = []documentRoot{ {dir: "books", kinds: {".pdf": pdf}}, {dir: "skills", kinds: {".md": markdown}}, {dir: "lib/js", kinds: {".md": markdown}, within: isLibraryDoc} }`; `isLibraryDoc(rel)` is true only for `lib/js/<recipe>/docs/<file>.md`.
  - `ScanDocuments(root string) ([]string, error)` keeps its signature and returns paths from every root, sorted; a missing root is reported per root through `ScanDocumentRoots(root string) (docs []string, missing []string, err error)`, which `ScanDocuments` wraps. The `lib/js` root never walks `node_modules`: it lists `lib/js/*/docs/` directories directly and walks only those.
  - `IsDocumentSource(filename string) bool` true for any root prefix; `documentRootOf(filename string) string` returns `"books"`, `"skills"` or `"lib/js"`; `sourceDir` uses it.
  - `openMarkdownReader() (documentReader, error)`; `markdownReader.Read(ctx, path) (documentText, error)`.

- [ ] **Step 1: Write the failing reader tests** in `markdown_reader_test.go`:
  - `TestMarkdownReaderSplitsOnSecondLevelHeadings`: input with frontmatter `name: data-dashboard`, `description: Charts from CSV`, an intro paragraph, `## Steps`, `## Check` → `Title == "data-dashboard"`, 3 parts with paths `section/1..3`, `Label`s `"Overview"`, `"Steps"`, `"Check"`; part 1 text starts with `"Charts from CSV"`.
  - `TestMarkdownReaderWithoutFrontmatterOrHeadings` (RF1): plain text file `notes.md` → `Title == "notes"`, one part `section/1`, `Note == ""`.
  - `TestMarkdownReaderKeepsCodeFencesIntact`: a `## ` line inside a ``` fence does not start a section.
- [ ] **Step 2: Run** `cd host-cli && go test ./internal/commands -run TestMarkdownReader -count=1` — expect FAIL (undefined `openMarkdownReader`).
- [ ] **Step 3: Implement `markdown_reader.go`.** Frontmatter is the leading `---` block parsed with `yaml.v3` into `name`, `description`; title falls back to the first `# ` heading, then the file name without extension. Text before the first `##` is section 1 labelled `"Overview"`, with the description prepended. Parts over `maxDocumentPartChars` are split into consecutive sections with the same label plus ` (cont.)`.
- [ ] **Step 4: Run** the reader tests — expect PASS.
- [ ] **Step 5: Write the failing index tests** in `index_documents_test.go`:
  - `TestIndexSkillsAndLibraryDocs`: vault with `skills/skills-core/a/SKILL.md`, `skills/skills-core/a/references/r.md`, `lib/js/js-core/docs/chartjs.md`, `lib/js/js-core/node_modules/chart.js/README.md` → sources are exactly the first three paths; the README is not indexed.
  - `TestIndexSkillsOnlyVault` (RF2): no `zim/`, no `books/`, manifest v2 with a realized entry at `skills/skills-core` → `RunConfiguredIndex(ctx, root, "keyword", …)` creates `data/search.db` with the skill sources.
  - `TestIndexMissingSkillsDirectoryIsNeverAPrune` (RF4): index, delete `skills/` → re-index keeps the sources (also with the manifest entry removed, matching `TestIndexMissingBooksDirectoryIsNeverAPrune`).
  - `TestIndexRemovedSkillFolderIsPruned` (RF4): index two skills, delete one skill folder but keep `skills/` → that skill's sources are pruned, the other's kept.
  - `TestScanLibraryRootSkipsNodeModules`: a `lib/js/x/node_modules/a/docs/y.md` is not returned.
  - Existing `TestIndexMissingBooksDirectoryIsNeverAPrune` and the PDF tests stay green unchanged.
- [ ] **Step 6: Run** `go test ./internal/commands -run 'TestIndex' -count=1` — expect the three new tests to FAIL.
- [ ] **Step 7: Implement the roots.** `ScanDocumentRoots` walks each root with the existing hidden-entry and symlink-root rules; `scanSources` builds `missing[root]` per root with the existing `recorded(dir)` rule; `RunConfiguredIndex`'s early return counts realized entries under any document root and documents from all roots. Keep the comment "A second kind is added here and nowhere else" true by keeping kinds only in `documentRoots`.
- [ ] **Step 8: Run** `go test ./internal/commands/... -count=1` — expect PASS, including all pre-existing document and index tests.
- [ ] **Step 9: Commit** `feat(index): index skills and library docs as markdown document sources`.

### Task 2: Keep skills out of knowledge search (drive engine)

**Files:**
- Modify: `drive-runtime/internal/search/engine/documents.go` (roots, `InScope`)
- Modify: `drive-runtime/internal/search/engine/keyword.go:56-80` (`scopeClause`)
- Modify: `drive-runtime/internal/search/engine/engine.go:53-200` (`KeywordContext`, `HybridContext`, `vectorSearch*` gain a scope)
- Modify: `drive-runtime/internal/search/semantic.go:302+` (`HybridSnapshot`, `HybridSnapshotFull` pass their scope through)
- Modify: `drive-runtime/internal/mcp/read.go:205-271` (`resolveScope`, `knownSources`), `drive-runtime/internal/mcp/documents.go:140-185` (`documentFolder`), `drive-runtime/internal/mcp/vault.go` (`vault_sources` listing)
- Test: `drive-runtime/internal/search/engine/skills_scope_test.go`, `drive-runtime/internal/mcp/search_skills_scope_test.go`

**Interfaces:**
- Produces:
  - `const SkillsDir = "skills"`, `const LibraryDocsDir = "lib/js"`; `IsSkillSource(filename string) bool` — prefix `skills/` or `lib/js/`.
  - `IsDocumentSource` stays books-only on the drive (PDF page semantics depend on it).
  - Scope semantics, one rule used everywhere: the empty scope means "every knowledge source" (ZIMs and `books/`), no longer every source; a scope that names a skill root or a source below it (`skills/`, `lib/js/`, `skills/skills-core/`) includes those; `SkillScope = "\x00skills"` means both skill roots and nothing else. `InScope` and `scopeClause` implement it; update `InScope`'s doc comment and its existing tests, and grep every `InScope(`/`scopeClause(` caller to confirm none needs the old "everything" meaning.
  - `KeywordContext(ctx, query, limit, scope string)`, `HybridContext(ctx, query, vec, limit, scope string)`, and the vector query filter by the same scope, so hybrid search, both its FTS candidates and its vector scan, never mixes the two worlds.
  - `SourceFilenames` keeps returning every source, because `knownSources` validates explicit `source` arguments against it; only display listings (`ListSources`, `vault_sources`, the empty-search "what the drive holds" text) drop skill sources.
  - `documentFolder` accepts folder scopes under `skills/` and `lib/js/` as well as `books/`, so `search` with `source: "skills/"` resolves.

- [ ] **Step 1: Write failing tests** in `skills_scope_test.go` with an in-memory searchdb holding one ZIM article, one `books/x.pdf` page and one `skills/skills-core/a/SKILL.md` section, all containing `chart`, each with a vector:
  - `TestKeywordSearchExcludesSkillsByDefault`: empty scope → ZIM and PDF only.
  - `TestKeywordSearchScopedToSkills`: `SkillScope` → only the skill; `"skills/"` → only the skill.
  - `TestHybridSearchExcludesSkillVectors` (RF3): the skill's vector equals the query vector → `HybridContext(..., "")` returns no skill; `HybridContext(..., SkillScope)` returns only the skill.
  - `TestInScopeEmptyExcludesSkills`.
  And in `search_skills_scope_test.go`: `TestSearchToolScopedToSkillsFolder` (`search` with `source: "skills/"` returns the skill, no "unknown source" error), `TestVaultSourcesHidesSkills`.
- [ ] **Step 2: Run** `cd drive-runtime && go test ./internal/search/... ./internal/mcp -run 'Skill|InScope|VaultSources' -count=1` — expect FAIL.
- [ ] **Step 3: Implement** the scope plumbing above; existing callers pass `""`.
- [ ] **Step 4: Run** `go test ./internal/search/... ./internal/mcp/... -count=1` — expect PASS with existing tests unchanged apart from `InScope`'s documented meaning.
- [ ] **Step 5: Commit** `feat(search): keep skill and library docs out of knowledge search`.

### Task 3: `skill_search` and `skill_read` (MCP)

**Files:**
- Create: `drive-runtime/internal/mcp/skills.go`
- Modify: `drive-runtime/internal/mcp/server.go:107-118` (`DriveCapabilities`), `:27-33` (server instructions)
- Test: `drive-runtime/internal/mcp/skills_test.go`

**Interfaces:**
- Consumes: `engine.SkillScope`, `engine.IsSkillSource`, the scoped `HybridSnapshot`/`KeywordContext` from Task 2, the search DB opener and semantic backend that `SearchCapability` uses (`openSearchDB`, `search.NewSemanticBackend`).
- Produces:
  - `NewSkillCapability(driveRoot string, meta DriveMetadata) *SkillCapability`; `Tool() == "skill"`; actions `search`, `read` → tool names `skill_search`, `skill_read`.
  - `skill_search` params: `query` (required), `kind` (`skill` | `library`), `limit` (default 5, 1–20). Result JSON array of `skillResult{ID, Kind, Title, Description, HasScripts bool, License string, Version string, Portability string, Requires []string}`; one per skill folder or library doc (best-ranked section wins).
  - `skill_read` params: `id` (required), `file`, `section`, `offset`. Returns the entry file's sections from `search.db` in order, in parts of `readChars` (6000, `mcp/read.go:24`) with the same string `Continues` field `search_read` uses; plus `files: []string` listing the skill folder's other files (from disk, relative paths; empty when the folder is missing).
  - `ID` is the skill folder path (`skills/skills-core/data-dashboard`) or the doc path (`lib/js/js-core/docs/chartjs.md`).
  - Result metadata (`Version`, `License`, `Portability`, `Requires`, `HasScripts`) is read at query time from the file's frontmatter and folder on disk, for at most `limit` results; a file missing on disk leaves those fields empty and the indexed text still answers. The index stores only text.
  - Registration is gated like the wiki tools (`server.go:127-129`): `DriveCapabilities` adds `NewSkillCapability` only when `.svalbard/recipes.json` lists a realized recipe of type `skills` or `js-lib` (`skillsInstalled(meta)`), so tools-only vaults pay no schema tokens. `DriveToolNames` follows automatically.
  - Server instructions: new `NewDriveServer(driveRoot string) *Server` builds the capabilities and, when skills are installed, passes `mcpserver.WithInstructions("Procedures for building things on this drive (apps, dashboards, maps, documents, firmware) are found with skill_search and read with skill_read. Read the skill before starting.")`. `NewServer(caps...)` stays for tests. Replace the four `mcp.NewServer(mcp.DriveCapabilities(driveRoot)...)` call sites (`cmd/svalbard-drive/main.go:309,329,343`, `internal/openwebui/openwebui.go:268`) with `NewDriveServer`.

- [ ] **Step 1: Write failing tests** in `skills_test.go` using a fixture drive with two skills (one with `scripts/run.py`) and one library doc, indexed through the host-free test helper the existing MCP tests use:
  - `TestSkillCapabilityToolNames`: actions render as `skill_search`, `skill_read`.
  - `TestDriveCapabilitiesGateSkills`: no `skills`/`js-lib` entry in recipes.json → no skill tools and no instructions; with one → both.
  - `TestSkillSearchGroupsBySkill`: a query matching three sections of one SKILL.md returns that skill once, `HasScripts == true` for the one with `scripts/`.
  - `TestSkillSearchKindFilter`: `kind: library` returns only the doc.
  - `TestSkillSearchWithoutIndex`: no `data/search.db` → text result naming `skills/` and `lib/js/` and saying the vault has no search index; no error.
  - `TestSkillReadInParts` (RF5): a SKILL.md longer than one part → first call has `continues`, second call with its offset ends with the last section's text.
  - `TestSkillReadListsFiles`: `files` contains `scripts/run.py` and `references/r.md`.
  - `TestSkillReadRejectsPathsOutsideRoots`: `id: "zim/../books/x"` → error.
- [ ] **Step 2: Run** `go test ./internal/mcp -run 'Skill' -count=1` — expect FAIL.
- [ ] **Step 3: Implement `skills.go`** and `NewDriveServer`; register in `DriveCapabilities` after `NewSearchCapability`, behind the gate.
- [ ] **Step 4: Run** `go test ./internal/mcp/... -count=1` — expect PASS.
- [ ] **Step 5: Commit** `feat(mcp): skill_search and skill_read for skills and library docs`.

### Task 4: `svalbard-drive skills find|read`

**Files:**
- Create: `drive-runtime/cmd/svalbard-drive/skills.go`
- Modify: `drive-runtime/cmd/svalbard-drive/main.go:85-90` (fast path beside `settings`)
- Test: `drive-runtime/cmd/svalbard-drive/skills_test.go`

**Interfaces:**
- Consumes: `mcp.NewSkillCapability(...).Handle(ctx, "search"|"read", params)`.
- Produces: `runSkills(stdout, stderr io.Writer, driveRoot string, args []string) error`. `skills find <words…> [--kind skill|library] [--limit N]` prints one line per result: `<id>  <title> — <description>`; `skills read <id> [--file F] [--offset N]` prints the text and, at the end, `continues: svalbard-drive skills read <id> --offset N` when there is more.

- [ ] **Step 1: Write failing tests**: `TestSkillsFindPrintsOneLinePerSkill`, `TestSkillsReadPrintsContinuation`, `TestSkillsUnknownSubcommandShowsUsage` (exit error, usage on stderr).
- [ ] **Step 2: Run** `go test ./cmd/svalbard-drive -run Skills -count=1` — FAIL.
- [ ] **Step 3: Implement**; the subcommand works without `actions.json` like `settings`.
- [ ] **Step 4: Run** the tests — PASS.
- [ ] **Step 5: Commit** `feat(drive): skills find and read on the command line`.

### Task 5: One drive context and PATH for every shell harness

**Files:**
- Create: `drive-runtime/internal/agent/context.go` (context text and builder, moved from `agent.go`)
- Modify: `drive-runtime/internal/agent/agent.go` (`piDriveContext`, `piContext`, `ompPrompt`, the opencode, goose, pi and omp cases, launch env)
- Create: `host-cli/internal/toolkit/agentsmd.go`; modify `host-cli/internal/toolkit/toolkit.go:165-200` (`Generate`)
- Test: `drive-runtime/internal/agent/context_test.go`, `host-cli/internal/toolkit/agentsmd_test.go`

**Interfaces:**
- Produces:
  - `DriveContext(opts ContextOptions) string`, `type ContextOptions struct { DriveRoot string; ToolPrefix string; Wiki, Skills, Libraries bool }`. Replaces `piContext`. Text = today's `piDriveContext` with tool names built as `ToolPrefix + name`, plus, when `Skills`: "For building something (an app, dashboard, map, document, firmware), first find a procedure with `<prefix>skill_search` and read it with `<prefix>skill_read`, then follow it. Use only the libraries and tools the skill names; they are on this drive." and, when `Libraries`: "Pack HTML with `svalbard-drive html pack`; never link a CDN." An empty `DriveRoot` drops the path sentence ("You run from a Svalbard USB drive.").
  - Tool prefixes: pi `mcp__svalbard__`, omp `mcp__svalbard_`, opencode and goose: read how each pinned version (`recipes/apps/opencode.yaml`, `recipes/apps/goose.yaml`) names MCP tools for a server called `svalbard` (start the harness against the drive's MCP server and list its tools) and record the prefix in a code comment with the version checked.
  - omp: `ompPrompt` gains the same skills sentence with omp's prefix (omp runs `--no-rules`, so its system prompt is its only orientation).
  - `skillsInstalled(meta DriveMetadata) bool` / `librariesInstalled` — any realized recipe of type `skills` / `js-lib` in `.svalbard/recipes.json`; shared with Task 3's gate.
  - Placement: pi `configRoot/AGENTS.md` (unchanged); opencode `configRoot/opencode/AGENTS.md`; goose `configRoot/goose/.goosehints`. Confirm both paths against the pinned versions in the same check as the prefixes; if goose reads `AGENTS.md` via `CONTEXT_FILE_NAMES`, set that instead and say so in the commit message.
  - **PATH for every client:** `PrepareClientLaunchConfig` sets `PATH = <drive>/bin/<platform> : <dir of os.Executable()> : inherited PATH` for pi, omp, opencode and goose, so `py-docs` (stage 2) and `svalbard-drive` (stage 3) resolve from an agent's shell. Platform names come from the same mapping the activate script uses (`macos-arm64`, `macos-x86_64`, `linux-arm64`, `linux-x86_64`).
  - Host: `agentsMDText(skills, libraries bool) string` in `agentsmd.go` produces the drive-root `AGENTS.md`: the same text with an empty drive root and bare tool names (`skill_search`) plus "Your harness may prefix tool names with the server name `svalbard`." `Generate` writes it when any harness or `skills` recipe is realized, through `writeGeneratedFileHost` (same marker format as the drive's). A drive test writes `DriveContext(ContextOptions{Skills: true, Libraries: true})` (empty root, empty prefix) to `drive-runtime/internal/agent/testdata/drive_context.golden`; the host test reads that file by repo-relative path and asserts `agentsMDText(true, true)` equals it plus the prefix sentence, so the two copies cannot drift.
- Owner preference recorded in the design: Open WebUI gets no context file.

- [ ] **Step 1: Write failing tests**:
  - `TestDriveContextOmitsAbsentAreas`: no skills, no wiki → no `skill_search` and no `wiki_` text.
  - `TestDriveContextUsesHarnessPrefix`: prefix `mcp__svalbard_` → `mcp__svalbard_skill_search` appears and `mcp__svalbard__` does not.
  - `TestDriveContextGolden` and `TestDriveContextBudget` (all options on, ≤ 260 words).
  - `TestPrepareOpencodeWritesContext`, `TestPrepareGooseWritesHints`, `TestOmpPromptMentionsSkills`.
  - `TestPrepareClientPutsDriveBinariesOnPath`: for each client, `cfg.Env["PATH"]` starts with `<drive>/bin/<platform>` followed by the executable's directory.
  - Host: `TestGenerateWritesRootAgentsMD` (realized `skills-core` → root `AGENTS.md` matching the golden plus prefix sentence; a user-edited file is kept).
- [ ] **Step 2: Run** `go test ./internal/agent/... -count=1` (drive) and `go test ./internal/toolkit -run AgentsMD -count=1` (host) — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** both — PASS; `make verify` from the repo root — PASS.
- [ ] **Step 5: Commit** `feat(agent): one generated drive context and drive PATH for every shell harness`.

### Task 6: Stage verification and checkpoint

**Files:**
- Create: `docs/plans/2026-11-2x-skill-vault-stage1-checkpoint.md` (date of completion)

- [ ] **Step 1:** Rerun the verification steps of both contract checkpoints on a disposable vault built from this branch: PDF collection add/apply/edit/index, unchanged-source skip, touched-file reindex, missing `books/` not a prune, semantic resume after SIGINT (search reliability checkpoint, "Verification evidence"). Record each result.
- [ ] **Step 2:** On the same vault, hand-place two fixture skills under `skills/skills-core/` and one doc under `lib/js/js-core/docs/` (stage 2 and 3 ship the real ones), and add matching `skills`/`js-lib` entries to `.svalbard/recipes.json` by hand so the gates open (stage 2 makes this real). Index, then exercise over MCP stdio: `skill_search`, `skill_read` with continuation, `search` with no scope (no skills), `search` scoped `skills/` (skills).
- [ ] **Step 3:** Launch pi, omp, opencode and goose from the drive menu; in each, ask "what tools do you have for building things?" and run `echo $PATH` through its shell tool; record the answers. Measure the context file with llama-server `/tokenize` (≤ 400 tokens).
- [ ] **Step 4:** `make verify` and `make build` — PASS; write the checkpoint with commands and outcomes; commit `docs: skill vault stage 1 checkpoint`.
