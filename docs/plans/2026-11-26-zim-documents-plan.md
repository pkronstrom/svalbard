# Documents inside ZIMs: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Index the PDFs and EPUBs inside ZIMs (and loose EPUBs under `books/`) as page-level document sources, and stop indexing ZIM junk entries, so agents and the search screen find, read, cite and open them.

**Architecture:** The host's ZIM pass gains a junk filter and a document pass that hands each embedded PDF or EPUB, read from the ZIM into memory, to the existing document readers; each becomes a source named `<zim filename>/<entry path>`. A reader version recorded per source, published last for a ZIM, makes upgrades and interrupted runs resume. The drive treats those names as documents read from `search.db`, and opens PDF hits through kiwix-serve at `#page=N`.

**Tech Stack:** Go 1.24 (host-cli, drive-runtime modules), gozim v0.1.0, go-pdfium in wazero (`host-cli/internal/pdfext`), SQLite FTS5 (`search.db`), `archive/zip` and `encoding/xml` for EPUB.

**Spec:** `docs/plans/2026-11-26-zim-documents-design.md`

## Global Constraints

- PDF and EPUB code runs on the host only; the drive runtime reads documents from `data/search.db`.
- Embedded source name: `<zim filename>/<entry path>`, e.g. `usda-2015_en_2025-04.zim/files/GUIDE02_HomeCan_rev0715.pdf`. ZIM sources never contain `/`; loose documents start with `books/`; skills with `skills/` or `lib/js/`.
- Articles: PDF `page/N` (1-based physical page, label `p. N`); EPUB `page/N` in reading order (label `ch. K`, `ch. K (cont.)` for later parts of a long chapter).
- A PDF or EPUB entry is skipped when the same ZIM has an HTML entry at its path minus the extension (`X.73459.epub` beside `X.73459`).
- `maxDocumentPartChars` = 16000 (existing); `maxEmbeddedDocumentBytes` = 512 MiB (new; largest seen 52 MB).
- `zimReaderVersion` = 2. A legacy row reads as 0. The reader version is an index cache key only, never a checksum or acquisition hash.
- Per-document outcomes never abort an index run; only cancellation and database errors do.
- Keyword search returns at most 3 pages of one document (`pagesPerDocument`); hybrid search keeps its own diversity step.
- No Aven references in commits or committed docs. End every commit message with
  `Claude-Session: https://claude.ai/code/session_01Ld6WLTPKcpPwP3kdAbMcnZ`.
- Tests that start anything run with an explicit local model endpoint and isolated HOME/XDG; nothing opens the owner's default browser; delete only literal absolute paths.

## Review Focus

1. **Entry paths with spaces, parentheses or non-ASCII** (`files/Emergency Preparedness (10).pdf`): the kiwix URL the search screen opens must be escaped per segment, or Chrome lands on a 404. Test in Task 7.
2. **A nautilus ZIM whose `database.js` is missing, renamed or not the expected quasi-Python list**: titles fall back to the PDF's own title, then the file name, with no error. Test in Task 2.
3. **A newer dated ZIM replacing an old one** (`zimgit-post-disaster_en_2024-05.zim` → `_2025-04.zim`): the old ZIM's embedded sources are pruned with it, not left as orphans that still show up in search. Test in Task 5.
4. **A model naming an embedded document loosely** (`usda-2015/files/GUIDE02_HomeCan_rev0715.pdf`, `GUIDE02_HomeCan_rev0715`): `search_read` resolves it as it does a loose document's short forms. Test in Task 7.
5. **An EPUB that is broken, DRM-encrypted or has no OPF**: it gets a note and the run continues. Test in Task 4.

---

### Task 1: Reader version and article heads in `search.db`

**Files:**
- Modify: `host-cli/internal/searchdb/db.go` (migration list at `:117`, `Source` at `:235`, `IndexedSources` `:248`, `ReplaceSource` `:274`)
- Test: `host-cli/internal/searchdb/db_test.go`

**Interfaces:**
- Produces: `Source.ReaderVersion int`; `func (d *DB) SetSourceReaderVersion(ctx context.Context, filename string, version int) error`; `type ArticleHead struct{ Path, Title string }`; `func (d *DB) ArticleHeads(filename string) ([]ArticleHead, error)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestReaderVersionRoundTripsAndDefaultsToZero(t *testing.T)
// ReplaceSource(Source{Filename: "a.zim", UUID: "u", Size: 1, ModTimeNS: 2, ReaderVersion: 2}, nil)
// then IndexedSources() has ReaderVersion == 2; a row inserted by ReplaceSource with ReaderVersion 0
// reads back 0; SetSourceReaderVersion(ctx, "a.zim", 3) → 3; on an unknown filename it returns an error.

func TestOpenAddsReaderVersionToAnOldDatabase(t *testing.T)
// create the sources table without reader_version (copy the pre-change schema), Open, IndexedSources
// returns ReaderVersion 0 for the existing row.

func TestArticleHeadsListsPathsAndTitles(t *testing.T)
// two articles {Path:"/home", Title:"Home"}, {Path:"/home.html", Title:"{{ title }}"} → both, in path order.
```

- [ ] **Step 2: Run them, expect FAIL** — `cd host-cli && go test ./internal/searchdb/ -run 'ReaderVersion|ArticleHeads'` → compile errors.
- [ ] **Step 3: Implement.** Add `{"reader_version", "INTEGER"}` to the additive migration list; read it with `COALESCE(reader_version, 0)`; write it in `ReplaceSource`'s insert and upsert; `SetSourceReaderVersion` is one `UPDATE … WHERE filename = ?` that errors when no row changed.
- [ ] **Step 4: Run, expect PASS**, plus `go test ./internal/searchdb/`.
- [ ] **Step 5: Commit** — `feat(index): record a reader version per indexed source`.

### Task 2: ZIM content filter and the archive's embedded documents

**Files:**
- Modify: `host-cli/internal/zimext/extractor.go`
- Create: `host-cli/internal/zimext/archive.go`, `host-cli/internal/zimext/nautilus.go`
- Create: `host-cli/internal/zimext/testdata/make_fixture.py`, `host-cli/internal/zimext/testdata/documents.zim`, `host-cli/internal/zimext/testdata/book.epub`, `host-cli/internal/zimext/testdata/drm.epub`
- Test: `host-cli/internal/zimext/archive_test.go`, `host-cli/internal/zimext/extractor_test.go`

**Fixture** (`make_fixture.py`, run as `uv run --with 'libzim>=3,<4' python make_fixture.py .`; the docstring lists every entry like `pdfext/testdata/make_fixtures.py` does). `documents.zim`, title "Fixture Library", main page `home`:

| Path | MIME | Purpose |
|---|---|---|
| `home` | text/html | rendered page, `<title>Fixture Library</title>`, one paragraph |
| `home.html` | text/html | unrendered template, `<title>{{ title }}</title>` |
| `about` | text/html | a normal article |
| `vendors/x.css.map`, `display_rows.handlebars` | text/plain | junk |
| `vendors/app.js` | application/javascript | junk |
| `database.js` | text/javascript | nautilus list: `'ti': 'Part 2', 'dsc': 'Fruit and Fruit Products', 'fp': ['guide.pdf']` and `'ti': 'Water purification', 'dsc': 'Quick Guides', 'fp': ['water.pdf']` |
| `files/guide.pdf` | application/pdf | copy of `pdfext/testdata/text.pdf` |
| `files/water.pdf` | application/pdf | copy of `text.pdf` |
| `files/scan (1).pdf` | application/pdf | copy of `pdfext/testdata/scan.pdf`; a space and parentheses in the path |
| `files/book.epub` | application/epub+zip | `book.epub` |
| `Novel.123` / `Novel.123.epub` | text/html / application/epub+zip | Gutenberg-style duplicate pair |

`book.epub`: title "Fixture Book", spine cover (image only), chapter 1 "Beginnings" (short), chapter 2 "The Long Road" (40,000 characters of numbered lines). `drm.epub`: a valid EPUB plus `META-INF/encryption.xml`.

**Interfaces:**
- Produces:
  - `var ErrNoIndexableArticles` (renamed from `errNoIndexableArticles`; `ExtractArticlesContext` still returns it when no HTML article remains).
  - `func IsJunkPath(path string) bool`: true for the extensions `.map .js .mjs .css .json .svg .wasm .handlebars .hbs .mustache .tmpl .tpl .ejs`.
  - `func IsTemplateTitle(title string) bool`: true when the title holds `{{`.
  - `type Embedded struct { Path, MIME, Title string; Size int64 }` (`Title` is "" when no catalogue names it).
  - `type Archive struct` with `func OpenArchive(path string) (*Archive, error)`, `Close() error`, `UUID() string`, `Title() string`, `HoldsDocuments() bool` (header MIME list names `application/pdf` or `application/epub+zip`), `Documents(ctx context.Context) ([]Embedded, error)`, `ReadEntry(path string, limit int64) ([]byte, error)`.
  - `var ErrEntryTooLarge = errors.New("zimext: entry too large")`.

- [ ] **Step 1: Write the failing tests** (fixture path `testdata/documents.zim`)

```go
func TestExtractArticlesSkipsJunkAndTemplates(t *testing.T)
// paths == ["/about", "/home"]; none of home.html, *.map, *.handlebars, app.js.

func TestDocumentsListsPDFsAndEPUBsWithoutDuplicates(t *testing.T)
// paths == ["files/book.epub", "files/guide.pdf", "files/scan (1).pdf", "files/water.pdf"]
// ("Novel.123.epub" is absent); each Size > 0; MIME as in the fixture.

func TestDocumentsTakeNautilusTitles(t *testing.T)
// guide.pdf → "Fixture Library, Part 2: Fruit and Fruit Products"  (generic "Part N" gets the archive title and dsc)
// water.pdf → "Water purification"                                 (a real title stands alone)
// book.epub, "scan (1).pdf" → ""

func TestDocumentsWithoutCatalogueOrWithAMalformedOne(t *testing.T)
// parseNautilusTitles("var DATABASE = [ not a list") and parseNautilusTitles("") → empty map, no panic.

func TestReadEntryEnforcesLimit(t *testing.T)
// ReadEntry("files/guide.pdf", 1<<20) == bytes of pdfext/testdata/text.pdf;
// ReadEntry("files/guide.pdf", 10) → errors.Is(err, ErrEntryTooLarge), checked with ContentSize before reading.

func TestHoldsDocuments(t *testing.T)
// documents.zim → true; gozim's small.zim (testSmallZIMPath) → false.
```

- [ ] **Step 2: Run them, expect FAIL** — `cd host-cli && go test ./internal/zimext/`.
- [ ] **Step 3: Write and run the fixture script**, commit its outputs with it (`documents.zim` stays under 200 KB).
- [ ] **Step 4: Implement.** In `ExtractArticlesContext`, skip an entry when `IsJunkPath(entry.Path())`, and an HTML entry whose `<title>` satisfies `IsTemplateTitle`. `Documents` walks the content namespace once: collect HTML paths into a set, then keep `application/pdf` and `application/epub+zip` entries whose path minus extension is not in the set; size from `Entry.ContentSize`. `nautilus.go`: `parseNautilusTitles(js string) map[string]string` keyed by `fp` base name, using per-record regexes for `'ti'`, `'dsc'`, `'fp'` (the list is a Python repr, not JSON). It appends `: dsc` and prefixes `Title() + ", "` only when `ti` matches `(?i)^(part|chapter|vol(ume)?|section|book)\s*[\dIVX.]+$`. `Documents` matches catalogue entries by the entry path's base name.
- [ ] **Step 5: Run, expect PASS** — `go test ./internal/zimext/`.
- [ ] **Step 6: Commit** — `feat(zimext): skip junk entries and list the documents inside a ZIM`.

### Task 3: Document readers take bytes as well as files

**Files:**
- Modify: `host-cli/internal/pdfext/pdfext.go:141` (`Document`), `host-cli/internal/commands/index_documents.go` (`documentReader` `:126`, `pdfReader.Read` `:581`, `indexDocuments` `:378`), `host-cli/internal/commands/markdown_reader.go:32`
- Test: `host-cli/internal/pdfext/pdfext_test.go`, `host-cli/internal/commands/index_documents_test.go`, `host-cli/internal/commands/markdown_reader_test.go`

**Interfaces:**
- Produces:
  - `func (e *Extractor) DocumentFrom(ctx context.Context, r io.ReadSeeker, size int64) (Doc, error)`; `Document(ctx, path)` opens the file and calls it.
  - `type documentSource struct { Name string; Data readSeekerAt; Size int64 }` with `type readSeekerAt interface { io.ReadSeeker; io.ReaderAt }`. `Name` is the vault path or `<zim>/<entry>` and is used for fallback titles.
  - `documentReader.Read(ctx context.Context, src documentSource) (documentText, error)`.
  - `func fileSource(file, rel string) (documentSource, func() error, error)` opens a vault file.

- [ ] **Step 1: Write the failing tests**

```go
func TestDocumentFromBytesMatchesDocument(t *testing.T)            // pdfext: text.pdf via bytes.Reader == via path (Title, page count, page texts)
func TestPDFReaderReadsAnInMemorySource(t *testing.T)               // commands: documentSource{Name: "x.zim/files/a.pdf", Data: bytes.NewReader(textPDF)} → 3 parts "page/1".."page/3"
func TestMarkdownReaderTitleFallsBackToSourceName(t *testing.T)     // no heading or frontmatter → fileTitle(src.Name)
```

- [ ] **Step 2: Run them, expect FAIL.**
- [ ] **Step 3: Implement** the signatures above. `indexDocuments` opens each file with `fileSource`, closes it after `Read`, and keeps its stat-before/after check unchanged.
- [ ] **Step 4: Run, expect PASS** — `cd host-cli && go test ./internal/pdfext/ ./internal/commands/`: every existing document and markdown test still passes unchanged.
- [ ] **Step 5: Commit** — `refactor(index): document readers read from bytes as well as files`.

### Task 4: EPUB reader

**Files:**
- Create: `host-cli/internal/commands/epub_reader.go`
- Modify: `host-cli/internal/commands/index_documents.go` (`documentKinds` `:57`, `documentRoots` `:79`)
- Test: `host-cli/internal/commands/epub_reader_test.go`

**Interfaces:**
- Consumes: `documentSource` (Task 3); `zimext.StripHTML`; `splitAtLines` and `maxDocumentPartChars` (existing).
- Produces: `func openEPUBReader() (documentReader, error)`; `documentKinds[".epub"] = {unit: "part", open: openEPUBReader}`; `books` root extensions `{".pdf", ".epub"}`.

- [ ] **Step 1: Write the failing tests** (fixtures from Task 2's `zimext/testdata`)

```go
func TestEPUBReaderReadsSpineInOrder(t *testing.T)
// book.epub: Title "Fixture Book"; the cover yields no part; parts[0] = {Path "page/1", Label "ch. 1", Section "Beginnings"};
// chapter 2 yields 3 parts with Labels "ch. 2", "ch. 2 (cont.)", "ch. 2 (cont.)", consecutive paths page/2..page/4,
// each Text ≤ 16000 runes; the parts joined by "\n" hold every numbered line of chapter 2 exactly once.

func TestEPUBReaderNotesProblemsWithoutFailing(t *testing.T)
// drm.epub → Note "DRM-protected", no parts, err == nil; random bytes → Note starting "unreadable EPUB:";
// a zip without META-INF/container.xml → Note "unreadable EPUB: no package file".

func TestLooseEPUBsUnderBooksAreScanned(t *testing.T)
// ScanDocumentRoots finds books/novels/book.epub; indexVault on a scratch vault indexes it with 4 articles,
// first title "Fixture Book, ch. 1 · Beginnings".
```

- [ ] **Step 2: Run them, expect FAIL.**
- [ ] **Step 3: Implement.** `archive/zip.NewReader(src.Data, src.Size)`. `META-INF/encryption.xml` present → DRM note. `container.xml` rootfile → OPF: `dc:title`, manifest id→href (resolved relative to the OPF's folder), spine itemrefs. For each item: text = `zimext.StripHTML`, and heading = first `h1`/`h2`, else `<title>`, else "". Items with no text are skipped and don't count toward K. Split with `splitAtLines(text, maxDocumentPartChars)`. Part paths are numbered across the whole book.
- [ ] **Step 4: Run, expect PASS**, plus `go test ./internal/commands/`.
- [ ] **Step 5: Commit** — `feat(index): read EPUBs, loose under books/ and for archives`.

### Task 5: The ZIM pass indexes embedded documents

**Files:**
- Create: `host-cli/internal/commands/index_zim.go` (the per-ZIM pass, moved out of `indexVaultUnlocked`)
- Modify: `host-cli/internal/commands/index.go:116-178` (the ZIM loop now calls it; `present` gains embedded names)
- Test: `host-cli/internal/commands/index_zim_test.go`

**Interfaces:**
- Consumes: Tasks 1–4.
- Produces:
  - `const zimReaderVersion = 2`; `var maxEmbeddedDocumentBytes int64 = 512 << 20` (a var so tests can lower it).
  - `func embeddedSourceName(zimFilename, entryPath string) string` returns `zimFilename + "/" + entryPath`.
  - `func indexZIM(ctx context.Context, db *searchdb.DB, root, zf string, force bool, indexed map[string]searchdb.Source, readers *readerSet, w io.Writer, notify func(IndexProgress)) (present []string, err error)`.
  - `type readerSet` opens each `documentKinds` reader once per run and is shared with `indexDocuments`.

The per-ZIM algorithm is not determined by the tests alone, so here it is:

```text
identity := archiveSourceIdentity(...)            // UUID, size, mtime (existing)
stored   := indexed[zf]
if !force && stored.IdentityCurrent(identity) && stored.ReaderVersion >= zimReaderVersion:
    skip; present = zf + every indexed name with prefix zf+"/"
htmlStale := force || !stored.IdentityCurrent(identity) || any ArticleHead of zf has IsJunkPath(path) or IsTemplateTitle(title)
if htmlStale:  ExtractArticlesContext → ReplaceSource(identity with ReaderVersion 0)
               ErrNoIndexableArticles is fatal only when archive.HoldsDocuments() is false;
               otherwise the source is written with no articles
if htmlStale || archive.HoldsDocuments():
    for each Embedded e:  name := embeddedSourceName(zf, e.Path); present += name
        child current (UUID "zim:"+uuid, same size/mtime, ReaderVersion == zimReaderVersion) && !force → skip
        e.Size > maxEmbeddedDocumentBytes → note "too large to index (N MB)", no articles
        else ReadEntry → reader for e.MIME (".pdf"/".epub") → documentArticles(title, parts)
             title = documentTitle(e.Title, text.Title, e.Path)
        ReplaceSource(Source{Filename: name, UUID: "zim:"+uuid, Size/ModTimeNS: the ZIM's, ReaderVersion: zimReaderVersion, Note})
SetSourceReaderVersion(zf, zimReaderVersion)       // last: marks the ZIM complete
summary line, e.g. "  usda-2015: 8 documents, 193 pages"; then one indented line per not-searchable document
```

- [ ] **Step 1: Write the failing tests** (scratch vault in `t.TempDir()` with `zim/documents.zim` copied from the fixture)

```go
func TestIndexZIMIndexesEmbeddedDocuments(t *testing.T)
// sources: "documents.zim" (articles /about, /home) and "documents.zim/files/guide.pdf" (page/1..3, first title
// "Fixture Library, Part 2: Fruit and Fruit Products, p. 1"), ".../water.pdf", ".../scan (1).pdf" (Note
// "scanned: no text layer, needs OCR", 0 articles), ".../book.epub" (4 articles); no "Novel.123.epub" source;
// stored ReaderVersion of all == 2; output holds "documents: 4 documents" and a line naming scan (1).pdf.

func TestIndexZIMSecondRunReadsNothing(t *testing.T)
// a second indexVault reports the ZIM "skip"; a readerSet counting Read calls sees 0.

func TestIndexZIMUpgradesAnOldIndex(t *testing.T)
// seed: index with ExtractArticles behaving as before (insert "documents.zim" with ReaderVersion 0 and
// the junk articles /home.html and /vendors/x.css.map); run indexVault: junk gone, embedded sources present,
// ZIM ReaderVersion 2. Seed an index of gozim's small.zim with ReaderVersion 0: its articles are not
// re-extracted (ReplaceSource not called; check indexed_at unchanged) and its version becomes 2.

func TestIndexZIMResumesAfterInterruption(t *testing.T)
// cancel the context from notify after the first embedded document is "done"; the ZIM's ReaderVersion stays 0;
// the next run reads only the remaining documents (count Read calls == total-1) and then sets the version to 2.

func TestIndexZIMPrunesWithTheArchive(t *testing.T)
// index, then rename zim/documents.zim to zim/documents_2.zim and index: no source with prefix
// "documents.zim/" remains; "documents_2.zim/files/guide.pdf" exists.

func TestIndexZIMSkipsOversizedEntries(t *testing.T)
// maxEmbeddedDocumentBytes = 1000: guide.pdf has Note starting "too large to index", 0 articles; the run succeeds.

func TestIndexZIMWithNothingIndexableStillFails(t *testing.T)
// testdata/empty.zim (only vendors/app.js) → indexVault returns an error naming empty.zim.
```

This task adds `empty.zim` to the Task 2 fixture script.

- [ ] **Step 2: Run them, expect FAIL.**
- [ ] **Step 3: Implement** `index_zim.go` per the algorithm. Then grep the host for code that assumes every non-document source is a file in `zim/` (`sourceDir`, `svalbard status`, the semantic progress view, `index_configured.go`); embedded names must count as `zim` and be shown under their ZIM.
- [ ] **Step 4: Run, expect PASS** — `cd host-cli && go test ./internal/commands/ ./internal/searchdb/ ./internal/zimext/`.
- [ ] **Step 5: Commit** — `feat(index): index the PDFs and EPUBs inside ZIMs`.

### Task 6: The drive engine knows embedded documents

**Files:**
- Modify: `drive-runtime/internal/search/engine/documents.go` (`IsDocumentSource` `:39`, `InScope`, `Documents` `:112`), `drive-runtime/internal/search/engine/keyword.go:70` (`scopeClause`)
- Test: `drive-runtime/internal/search/engine/documents_test.go`, `drive-runtime/internal/search/engine/keyword_test.go`

**Interfaces:**
- Produces: `func IsEmbeddedSource(filename string) bool` (not a document root or skill path, and contains `".zim/"`); `func EmbeddedArchive(filename string) (archive, entry string, ok bool)`; `IsDocumentSource` returns true for embedded sources; `Documents` includes them.

- [ ] **Step 1: Write the failing tests**

```go
func TestEmbeddedSourceNames(t *testing.T)
// "usda.zim/files/a b.pdf" → embedded, archive "usda.zim", entry "files/a b.pdf", IsDocumentSource true;
// "usda.zim", "books/x.pdf", "skills/a/SKILL.md" → not embedded.

func TestInScopeArchiveIncludesItsDocuments(t *testing.T)
// InScope("usda.zim/files/a.pdf", "usda.zim") true; InScope("usda.zim2/files/a.pdf", "usda.zim") false;
// InScope("usda.zim/files/a.pdf", "") true; scopeClause gives the same answers over a test DB
// (one test table-driven over both).

func TestKeywordSearchCapsPagesPerEmbeddedDocument(t *testing.T)
// one embedded PDF with 6 matching pages and one ZIM article: unscoped search returns 3 pages + the article;
// scoped to the embedded source returns all 6.

func TestDocumentsListsEmbedded(t *testing.T)
// Documents() returns books/ and embedded documents with page counts and notes.
```

- [ ] **Step 2: Run them, expect FAIL** — `cd drive-runtime && go test ./internal/search/engine/`.
- [ ] **Step 3: Implement.** Archive scope in `InScope`: `filename == scope || strings.HasSuffix(scope, ".zim") && strings.HasPrefix(filename, scope+"/")`. `scopeClause` gets the matching SQL, using `substr` for the prefix as the folder case does. `Documents` adds `OR src.filename LIKE '%.zim/%'`.
- [ ] **Step 4: Run, expect PASS**, plus `go test ./internal/search/...`.
- [ ] **Step 5: Commit** — `feat(search): treat documents inside archives as documents`.

### Task 7: Agents and the search screen

**Files:**
- Modify: `drive-runtime/internal/mcp/read.go` (no-path archive read, `resolveScope` names), `drive-runtime/internal/mcp/documents.go` (`documentNames` `:127`, `sourceNames` `:183`), `drive-runtime/internal/mcp/search.go` (`holdings` `:300`, tool description `:51`, result items `:244`), `drive-runtime/internal/mcp/vault.go:69`, `drive-runtime/internal/inspect/structured.go:14`, `drive-runtime/internal/search/documents.go` (`SourceLabel` `:29`, `DocumentFile` `:39`), `drive-runtime/internal/search/session.go:187` (`OpenResult`)
- Test: `drive-runtime/internal/mcp/documents_test.go`, `drive-runtime/internal/mcp/read_test.go`, `drive-runtime/internal/mcp/vault_test.go`, `drive-runtime/internal/search/session_test.go`

**Interfaces:**
- Consumes: Task 6.
- Produces: `SourceInfo.Documents *int` (`json:"documents,omitempty"`) on a ZIM entry; `DocumentFile` returns "" for an embedded source; `func embeddedURL(base, filename string, page int) string` in `search`.

- [ ] **Step 1: Write the failing tests** (an index DB built in the test with the engine fixtures' helpers, one ZIM source with two embedded PDFs)

```go
func TestSearchReadOfArchiveNamesItsDocuments(t *testing.T)
// stub kiwix main page; result Note contains "also holds 2 documents" and both document titles with page counts.

func TestSearchReadOfEmbeddedPage(t *testing.T)
// search_read source "usda-2015_en_2025-04.zim/files/GUIDE02_HomeCan_rev0715.pdf" path "page/2": body from the
// index, no kiwix request (the stub fails the test if called), File == "", links to pages 1 and 3.

func TestResolveEmbeddedDocumentByLooseNames(t *testing.T)
// "usda-2015/files/GUIDE02_HomeCan_rev0715.pdf", "usda-2015_en_2025-04/files/GUIDE02_HomeCan_rev0715",
// and the exact name all resolve to the embedded source; "usda-2015" still resolves to the archive.

func TestEmptySearchFoldsEmbeddedDocumentsIntoTheirArchive(t *testing.T)
// holdings lists "usda-2015_en_2025-04 (… 2 documents)" and no embedded source as its own holding.

func TestVaultSourcesCountsDocumentsInArchives(t *testing.T)
// the ZIM's SourceInfo.Documents == 2.

func TestOpenEmbeddedHitUsesKiwixAtPage(t *testing.T)
// opener records "http://localhost:<port>/content/zimgit-post-disaster_en_2024-05/files/Emergency%20Preparedness%20%2810%29.pdf#page=7";
// an embedded EPUB hit has no fragment; a books/ EPUB hit opens the document server URL without "#page=".
```

- [ ] **Step 2: Run them, expect FAIL** — `cd drive-runtime && go test ./internal/mcp/ ./internal/search/ ./internal/inspect/`.
- [ ] **Step 3: Implement.**
  - `documentNames` adds the forms without `.zim` and without the archive's `_<lang>_<date>` suffix (`sourceName` of the archive part).
  - `sourceNames` and `holdings` count embedded sources under their archive.
  - The no-path archive read appends the note (first 20 documents, then "and N more; search with this source to find a page").
  - `OpenResult` checks `IsEmbeddedSource` before the `books/` branch and builds `embeddedURL`, escaping each path segment with `url.PathEscape` and adding `#page=N` only for `.pdf`.
  - The search tool description reads "A hit in a document (a PDF or EPUB, on its own or inside an archive) is one page…".
- [ ] **Step 4: Run, expect PASS**, plus `cd drive-runtime && go test ./...`.
- [ ] **Step 5: Commit** — `feat(drive): find, read and open documents inside archives`.

### Task 8: Real files, user paths, checkpoint

**Files:**
- Create: `docs/plans/2026-11-26-zim-documents-checkpoint.md`
- Modify: `docs/usage.md` (the "Your own documents" section gains EPUB; the search section notes PDFs inside archives)

- [ ] **Step 1: `make verify`** from the worktree root → all modules pass.
- [ ] **Step 2: Real ZIMs.** Build the CLI (`make build`). Copy `usda-2015_en_2025-04.zim`, `zimgit-food-preparation_en_2025-04.zim`, `zimgit-post-disaster_en_2024-05.zim` (from `~/.cache/svalbard-scratch/shared-vault/zim/`) and `wikem_en_all_maxi_2026-04.zim` (from `~/.cache/svalbard-scratch/skills/vault/zim/`) into a scratch vault under the session scratchpad, by clonefile copy (`cp -c`), and run `svalbard index`. Expected: document and page counts match the spec's measurement table (8/193, 17/1,126, 108/11,466, 48/112) and the not-searchable lists match; `sqlite3 data/search.db "select count(*) from articles a join sources s on s.id=a.source_id where s.filename not like '%/%' and (a.path like '%.map' or a.path like '%.handlebars' or a.title like '%{{%')"` → 0. A second run reads nothing.
- [ ] **Step 3: Upgrade path.** Index the same vault with a CLI built from `cf6c00c` (the branch base), then with the new one. Only the four ZIMs above are read again, and a second run reads nothing. Record timings and the peak memory from `/usr/bin/time -l`.
- [ ] **Step 4: Gutenberg duplicates.** Ask the owner before downloading `gutenberg_fi_all_2026-01.zim` (1.4 GB). If approved: index it and confirm 0 EPUB sources and unchanged HTML article counts.
- [ ] **Step 5: Drive user paths** with the built runtime, isolated `HOME`/`XDG_*` and no kiwix-serve on the host. MCP over stdio:
  - `search "pressure canning altitude"` (canning pages from more than one source, at most 3 per document);
  - `search` with source `usda-2015`;
  - `search_read` of a PDF page, of an EPUB part (a loose `books/` EPUB) and of `usda-2015` without a path;
  - `vault_sources`.

  Then the search screen with a stand-in opener that records the URL, and that URL loaded in an isolated Chrome through chrome-devtools-mcp: it lands on the page of the hit.
- [ ] **Step 6: Write the checkpoint** in the repo's checkpoint format: resulting user path, contracts kept and changed (the embedded naming contract, the reader version), measurements, scope and limits (OCR, video ZIMs, Safari/Firefox/Linux opening not checked). Update `docs/usage.md`.
- [ ] **Step 7: Commit** — `docs: documents inside ZIMs checkpoint`.
