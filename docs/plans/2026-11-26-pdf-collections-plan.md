# Plan: your own PDF collections in Svalbard, downloaded, indexed and searchable on the stick

## 1. Outcome

A user writes a short YAML file that lists documents by URL or local path, including folders with subfolders. `svalbard add` and `svalbard apply` download only what is missing or changed into `books/<collection>/`, and indexing puts each PDF page into the existing `data/search.db`. On the stick, the `./run` search screen and the MCP `search` / `search_read` tools find those pages next to ZIM articles, read them back with no kiwix, open the PDF at the right page in a browser, and give agents a "title, p. N" citation. Editing the file later adds or removes documents, and only those files are downloaded and re-indexed.

**Left out on purpose:**
- OCR. Scanned files are detected and reported instead.
- Formats other than PDF in the index. They are downloaded but not indexed.
- Detecting a changed file behind an unchanged URL. The acquisition-hash design has the same limit.
- `svalbard import` and `library/`, which stay as they are.
- Any change to the drive menu, `main.go` or `openwebui`.
- A wizard UI for writing collections.

## 2. Decisions

**D1 [OWNER]: How a user defines a collection.**
- Options:
  - (a) A user recipe file in `$XDG_CONFIG_HOME/svalbard/recipes/<id>.yaml` that has a `documents:` list, loaded like user presets.
  - (b) A file inside the vault.
  - (c) The repo's `recipes/local/`.
- **Pick (a).** No user-recipe seam exists today: `recipes/local/` holds only `.gitkeep`, and nothing loads the `local/recipes` that `local/README.md` mentions. User presets already set the pattern: an XDG directory, the dev-only `local/presets`, invalid files skipped with a warning. Copy it, and also make `local/recipes` load in dev builds so the README is true. The file is a normal recipe (`type: pdf`, plus id, description, tags), so presets can include it and `svalbard add <id>` works unchanged.
- Rules:
  - User recipes must have `documents`, and they cannot shadow a built-in ID.
  - User recipes load before user presets.
  - If the file is missing on another computer, nothing breaks: desired and realized IDs stay, the installed files stay, and plan prints "definition not found here; updates paused".

**D2: Where the files land.**
- Options: (a) `books/<id>/<filename>`; (b) flat `books/`.
- **Pick (a).** `pdf` already maps to `books/`, bundles already use `<typedir>/<id>/`, and the collection becomes a filter unit.
- URL documents use `filename:` if given, otherwise the last URL path segment.
- Folder entries keep their subfolders under `books/<id>/`.
- Duplicate target names fail at load with "set filename:".
- Realized state gets one `RealizedEntry` per document with the same ID. Platform recipes already produce several entries per ID, and `replaceEntries` and removal handle that.

**D3 [OWNER]: The extractor.**
Spike done on the WHO manual (240 pages, InDesign), the Raspberry Pi 4 datasheet (13 pages, LaTeX, tables), TM 9-2320-280-10 (421 pages, iText), a synthetic image-only scan, and two AES-256 encrypted copies (one with a user password, one owner-only).

| Extractor | What I saw |
|---|---|
| `ledongthuc/pdf` (already in `host-cli/go.mod`) | Drops ligature glyphs ("IV uids", "eects"). Drops every word space in the LaTeX PDF ("RaspberryPi4ModelB"). Cannot open AES-256 files even with an empty user password. Reports 0 pages for the scan. **Unusable for search.** |
| PDFium compiled to wasm (`klippa-app/go-pdfium/webassembly`, wazero, pure Go, no cgo) | Clean text with ligatures, spaces, bullets and line breaks. Table rows stay on one line ("VIN 5V Input Voltage -0.5 6.0 V"). Owner-only encrypted file reads fine; a user password gives a clear "invalid password". Image objects can be counted per page, which detects scans. Bookmarks are readable (e.g. rpi4 outline to page). Speed: WHO 3.4 s, TM9 0.7 s, rpi4 18 ms, plus one-off module start of about 1.4 s per process (43 ms with a 21 MB compile cache; not needed). |
| `pdftotext` (poppler) | Quality comparable. Needs Docker plus a newly published image (blocked on Apple Silicon today), or a host Homebrew dependency. |

- **Pick PDFium wasm, host-side.**
- **Cost for the owner to accept:** the spike binary grew from 3.3 to 15 MB (about +12 MB to the host CLI), and it adds wazero, go-pdfium, commons-pool, uuid and x/text.
- The drive runtime gets no PDF code.

**D4: The unit indexed.**
- Options: page, chunk across pages, whole document.
- **Pick one `Article` per PDF page that has text.** Pages are what people cite and what `#page=N` opens. They average about 1.9k characters here. Longer pages are already split into 500-word chunks by the semantic chunker.
- Mapping:
  - `Source.Filename` is the vault-relative path, e.g. `books/car-manuals/volvo-240.pdf`. ZIM names never contain `/`, so this is unique and easy to tell apart.
  - `Source.Title` is the document title: the `title:` from the YAML, written to `.svalbard/documents.json` at apply. Otherwise the PDF Title metadata (empty in all three test files), otherwise the filename.
  - `Article.Path` is `page/N` (1-based physical page).
  - `Article.Title` is `"<doc title>, p. N"`, plus `" · <bookmark section>"` when the PDF has an outline.
  - `Body` is the page text with whitespace normalised, capped at 16,000 characters.
  - `Sections` is `[{"heading": Article.Title, "body": text}]`, so semantic chunking works with no changes.

**D5: Reading a page back on the stick.**
- **Pick: straight from `search.db`.**
- `search_read` sees a source containing `/` and never starts kiwix.
  - With `path: page/N` it returns that page's body plus `links` to the previous and next indexed pages (reusing `PageLink`).
  - With no path it returns an overview: title, pages indexed, note, and up to 100 page links with their titles.
- Fix `engine.ReadArticle`: try the exact filename first, then `+".zim"`. Today it always appends `.zim`.

**D6 [OWNER]: How a human opens a hit at the page.**
- Options:
  - (a) `file:///…pdf#page=N`. On macOS `open` would hand this to Preview and probably lose the page (UNVERIFIED).
  - (b) A loopback HTTP file server, started once per search session and limited to `books/`, opening `http://127.0.0.1:P/car-manuals/volvo-240.pdf#page=N` in the default browser.
- **Pick (b).** Chrome, Firefox and Edge viewers honour `#page=`; Safari is UNVERIFIED.
- It goes in `Session.OpenResult`, so the line-mode screen and the menu TUI both get it without touching `internal/menu`.
- MCP results also carry `file: "books/…pdf#page=N"` and `page: N`.

**D7: Change detection.**
- Downloads:
  - A new `commands.BuildPlan(m, cat)` wraps `planner.Build`, which compares IDs only. It adds every collection whose expected set of relative paths differs from its realized entries.
  - It also adds collections where a `sha256:` pin differs from the recorded `checksum_sha256`, or a local `path:` file's size differs from `size_bytes`.
  - All of this is computed with no network access.
  - The collection job then fetches only missing or mismatched files and deletes dropped ones.
- Index:
  - Source identity is `UUID = "sha256:<hex>"` of the file, plus size and nanosecond mtime.
  - If size and mtime match the stored row, skip without hashing, so unchanged sticks are not re-read.
  - Otherwise hash and re-extract.
  - Replacement uses the existing `ReplaceSource` (atomic, under the writer lock, extract before replacing, previous source kept on failure). `PruneSources` gets the union of ZIM names and document paths.
- This is cache invalidation only. It stays separate from file checksums and from the future acquisition hash.

**D8: Scanned, encrypted and broken PDFs.**
- These are per-document outcomes, never index-aborting errors.
- Each is recorded as a source, possibly with zero articles, plus a new `sources.note` column, added through the existing column-migration list. Unchanged files are then not retried.

| Case | How it is detected | Note stored |
|---|---|---|
| Scanned page | A page with fewer than 20 non-space characters and at least one image object is "image-only" | — |
| Fully scanned document | All pages are image-only | `scanned: no text layer, needs OCR` |
| Partly scanned document | At least 25% of pages are image-only (threshold UNVERIFIED, tune on real files) | `N of M pages are scanned images without text` |
| Encrypted with a user password | PDFium reports a password error | `password-protected` |
| Broken file | Cannot be opened | `unreadable PDF: <err>` |

- The user sees these lines in `apply` / `index` output, and agents see them in `vault_sources` (`note`, `pages_indexed`).

**D9: Semantic search.**
- **Pick: covered now.** The semantic indexer already walks every source's articles and chunks `Sections`.
- The only change is in `RunConfiguredIndex`: run the semantic stage when ZIMs *or* documents exist, not only when ZIMs exist.

**D10 [OWNER]: A document that fails to download.**
- **Pick: keep today's apply semantics.**
  - The collection item fails and the error names each failing URL with its HTTP status.
  - Successfully downloaded documents are still recorded (partial entries are checkpointed).
  - Since apply failed, the index step is skipped, as for any failed item today.
- Next `plan` shows "1 document missing" and the next `apply` retries only that one.
- `svalbard index` still works by hand.
- Alternative: treat it as a warning so indexing proceeds. That needs a warning channel through apply, the CLI and the TUI, which is a separate change.

## 3. The user's view

`~/.config/svalbard/recipes/car-manuals.yaml`:
```yaml
id: car-manuals
type: pdf
description: Owner and workshop manuals for our vehicles
tags: [vehicles, repair]
documents:
  - title: Volvo 240 Owner's Manual (1990)
    url: https://example.org/manuals/volvo-240-1990.pdf
  - title: HMMWV Operator's Manual TM 9-2320-280-10
    url: https://www.liberatedmanuals.com/TM-9-2320-280-10.pdf
    filename: hmmwv-operator.pdf          # optional
    sha256: 3f1c…                         # optional pin
  - title: Trailer manual (scanned)
    path: ~/Documents/manuals/trailer.pdf
  - path: ~/Documents/manuals/workshop/   # every PDF below, subfolders kept
```

Commands:
```text
$ svalbard add car-manuals --vault /Volumes/STICK
$ svalbard apply --vault /Volumes/STICK
Indexing documents:
  car-manuals/volvo-240-1990.pdf     312 pages
  car-manuals/hmmwv-operator.pdf     418 pages
  car-manuals/trailer.pdf            not searchable: scanned, no text (OCR)
  car-manuals/workshop/brakes.pdf    44 pages
Apply complete
# later: edit the file, add one URL, delete one entry
$ svalbard plan --vault /Volumes/STICK
update car-manuals: +1 document, -1 document
$ svalbard apply --vault /Volumes/STICK      # fetches/indexes only those
```

Search screen (`./run`, then Search):
```text
Search (5 sources, 1,212 articles, keyword)
────────────────────────────────
  [keyword] Search (/fts /hybrid q): coolant capacity
────────────────────────────────
  1. [car-manuals/volvo-240-1990.pdf] Volvo 240 Owner's Manual (1990),
     p. 112 · Cooling system
     …Coolant capacity incl. heater: 9.5 litres. Use only Volvo type C…
  2. [wikipedia_en_100_mini] Antifreeze
     …
  Open # (or new search, q to quit): 1
  Opening result
```
Choosing 1 opens `http://127.0.0.1:8091/car-manuals/volvo-240-1990.pdf#page=112`. The menu TUI shows the source as `books/car-manuals/volvo-240-1990.pdf` (menu not edited).

Agent, `search {"query":"coolant capacity","source":"books/car-manuals"}`:
```json
[{"source":"books/car-manuals/volvo-240-1990.pdf","path":"page/112",
  "title":"Volvo 240 Owner's Manual (1990), p. 112 · Cooling system",
  "page":112,"file":"books/car-manuals/volvo-240-1990.pdf#page=112",
  "read_hint":"Use search_read with this exact source and path",
  "snippet":"…Coolant capacity incl. heater: 9.5 litres…"}]
```

`search_read {"source":"books/car-manuals/volvo-240-1990.pdf","path":"page/112"}`:
```json
{"source":"books/car-manuals/volvo-240-1990.pdf","path":"page/112",
 "title":"Volvo 240 Owner's Manual (1990), p. 112 · Cooling system",
 "body":"…full page text…",
 "links":[{"path":"page/111","label":"previous page"},
          {"path":"page/113","label":"next page"}]}
```

## 4. Diagram

```text
 ~/.config/svalbard/recipes/car-manuals.yaml   (documents: url | path)
        │ catalog.LoadUserRecipes (before user presets)
        ▼
 svalbard add car-manuals ──► manifest desired.items
        │
 svalbard apply ── BuildPlan: new item, or document set/pin/size changed
        │ assembleDocuments: fetch/copy only missing or changed files
        ▼
 books/car-manuals/**.pdf  +  .svalbard/documents.json (titles, urls)
        │ RunConfiguredIndex (writer lock)
        │ pdfext (PDFium wasm): text per page, scanned/password notes
        ▼
 data/search.db   source=books/car-manuals/x.pdf   article=page/N
        │                    (semantic stage chunks the same pages)
   ┌────┴─────────────────────┐
   ▼                          ▼
 ./run search screen       svalbard-drive mcp
 open: loopback http       search ─► search_read(source, page/N)
 x.pdf#page=N in browser   ─► agent answer cites "title, p. N"
```

## 5. Chunks

### Chunk 1: Define and download (host)

**Goal:** a collection file loads, can be added and applied, lands in `books/<id>/`, and later edits fetch or delete only the difference.

**Files:**
- Add:
  - `host-cli/internal/catalog/userrecipes.go`: `UserRecipeDirs`, `LoadUserRecipes`, `SkippedUserRecipes`, validation.
  - `host-cli/internal/catalog/documents.go`: `DocumentSource{Title, URL, Path, Filename, SHA256}`, and `ExpectedDocuments(item)`, which resolves relative paths and walks local folders.
  - `host-cli/internal/apply/documents.go`: `assembleDocuments`, which:
    - stages in `.staging/documents/<id>/`, reusing `fetchFile` (resume and cache);
    - checks `%PDF-` magic for `.pdf` targets, so an HTML login page is rejected;
    - copies local files;
    - deletes previously realized paths that are no longer expected, and empty directories;
    - returns entries for the successful documents together with an error.
  - `host-cli/internal/toolkit/documents_metadata.go`: `.svalbard/documents.json`, keyed by relative path, with title, collection and url.
  - `host-cli/internal/commands/plan_documents.go`: `BuildPlan`, plus a per-collection +/- summary for `WritePlan`.
- Change:
  - `catalog/catalog.go`: one `Documents` field on `Item`, one `skippedRecipes` field on `Catalog`. **Conflicts with the settings branch's schema edits.**
  - `catalog/embed.go`: `LoadCatalog` loads user recipes first.
  - `apply/apply.go`:
    - a job-partition condition and an `executeJob` case;
    - record `res.entries` even when `res.err` is set;
    - remove `books/<id>/` on collection removal;
    - call `WriteDocumentsMetadata` after `toolkit.Generate`.
  - `commands/apply.go` and `cli/root.go`: use `BuildPlan` at the three `planner.Build` sites; `loadCatalog` also prints skipped user recipes. **`root.go` has uncommitted edits in the main checkout.**

**Tests:**
- Load and validation:
  - A valid collection loads, and a user preset can list it.
  - Rejected with a stated reason: built-in shadowing, url and path both set, a non-http URL, duplicate filenames, and missing `documents`.
- Apply against an `httptest` server:
  - Two documents land.
  - Adding a third makes exactly one request.
  - Removing one deletes its file.
  - An HTML response fails that document only.
  - A partial failure keeps the good entries and names the failing URL.
  - A folder with subfolders is mirrored.
  - A changed pin or local size triggers a refetch.
  - `svalbard remove car-manuals` followed by apply leaves no `books/car-manuals`.
- `BuildPlan`:
  - drift → `ToDownload`;
  - no drift → empty;
  - missing definition → no error, a warning.

**Verify on a scratch vault:**
- Setup:
  - `XDG_CONFIG_HOME=<scratch>/config`.
  - A hand-written v2 `manifest.yaml`. Init is wizard-only on `main`.
  - The collection file holds the WHO and RPi4 URLs (the RPi URL goes through a 301 then a 302) and a local folder with a subfolder.
- Run: `svalbard add`, `apply`, then inspect the tree and the manifest.
- Then edit the file (add TM9, drop RPi4) and check that `plan` shows +1 −1, and that `apply` downloads only TM9 and deletes the RPi4 file.

### Chunk 2: Extract and index (host)

**Goal:** documents in `books/**` become per-page articles. Unchanged files are skipped, scanned and password-protected files are reported, and semantic search covers them.

**Files:**
- Add:
  - `host-cli/internal/pdfext/pdfext.go`:
    - `Open()` gives one wasm instance per index run.
    - `Document(ctx, path) (Doc{Title, Pages[]{Number, Section, Text, Images}}, error)`; it checks `ctx` between pages and returns `ErrPassword`.
    - `Classify(doc)` produces the note.
    - Tests use small committed fixtures: a text page, an image-only page, an AES user-password file and an owner-only file.
  - `host-cli/internal/commands/index_documents.go`: scan `books/` recursively for `.pdf`, ignoring `._*`; identity; read titles from `documents.json`; build `[]searchdb.Article`; write report lines to `w`.
- Change:
  - `commands/index.go` (`indexVaultUnlocked`):
    - a missing `zim/` or `books/` counts as empty only when the manifest records no realized entries of that type; otherwise it is an error, never a prune, which keeps the checkpoint's mount guard;
    - run the document pass;
    - the present set is the union of both.
  - `commands/index_configured.go`: the early-return guard and the semantic gating consider documents.
  - `searchdb/db.go`: `Source.Note` plus the migration.
  - `toolkit/toolkit.go`: one line, `case "zim", "pdf": state.HasLibrary = true`, so a documents-only stick shows Search. **Conflicts with the settings branch.**
  - `host-cli/go.mod` / `go.sum`.
  - The `index` Short text in `root.go`.

**Tests:**
- An unchanged document is skipped without an extractor call (use an injected fake).
- A touched document is re-extracted.
- A removed document is pruned.
- A ZIM-only change keeps documents, and the reverse.
- A documents-only vault with no `zim/` indexes.
- A missing `books/` with realized `pdf` entries is an error.
- A scanned document gives 0 articles plus a note and is not retried.
- A password-protected document gets a note.
- Cancelling mid-document keeps the previous source.
- Semantic runs on a documents-only vault (fake embedder, as in `index_semantic_test.go`).

**Verify:**
- Run `svalbard index` on the chunk-1 vault and check the output lines and the `sources` / `articles` rows with sqlite.
- `touch` one PDF; only it is re-extracted.
- Add the scanned fixture to the local folder, then apply; the report says "no text".

### Chunk 3: The stick (runtime)

**Goal:** the search screen and MCP find, read, cite and open document pages with no kiwix.

**Files:**
- Add:
  - `drive-runtime/internal/search/documents.go`: `IsDocumentSource`, page parsing, and the loopback `books/` file server for the session, bound to 127.0.0.1 and refusing paths outside `books/`.
  - `drive-runtime/internal/mcp/documents.go`: page read, overview, previous/next links, and per-document info from `search.db` (works with old databases that lack the `note` column).
- Change:
  - `search/engine/engine.go`: `ReadArticle`.
  - `search/session.go`: the `OpenResult` branch; `Close` stops the server.
  - `search/search.go`: `RenderResults` drops the `books/` prefix; the header becomes "Search".
  - `mcp/search.go`:
    - action descriptions mention documents;
    - a `handleRead` branch before kiwix;
    - `page` and `file` result fields;
    - the `source` filter also matches a `books/<collection>` prefix.
  - `mcp/vault.go` and `inspect/structured.go`: additive `path`, `title`, `pages_indexed` and `note` on book sources. **The Open WebUI file-tools branch also works in `internal/mcp`; these are edits to existing files, no new capability registration.**

**Tests:**
- `ReadArticle` for a document source.
- A search hit carries `page` and `file`.
- `search_read` for `page/N` returns the body and links; with no path it returns the overview.
- The document path never calls the kiwix starter (inject one that fails).
- The collection-prefix filter works.
- `vault_sources` shows the note.
- `OpenResult` calls the fake opener with `http://127.0.0.1:P/…#page=N`, and a GET returns the PDF bytes; `../` is refused.

**Verify:**
- Run `make build`, then apply to the scratch vault so the runtime is embedded.
- In `./run`, search "naloxone", open it, and check that the browser lands on the WHO page.
- Run `svalbard-drive mcp` over stdio, call `search` then `search_read`, with kiwix-serve absent from PATH.

### Chunk 4: Docs and real-document check

**Goal:** a non-developer can do this from the docs, and it holds on real files.

**Files:**
- `docs/usage.md`: a "Your own documents" section with the example file and the scanned / password messages.
- Fix the stale `library/README.md` (it mentions `import --bundle` and `attach`, which don't exist).
- A checkpoint in `docs/plans/`.

**Verify:** run the matrix in section 7 on a scratch vault and record what each case showed.

The work fits in four chunks. The first phase ends after chunk 3; chunk 4 closes it.

## 6. Sizes

| Chunk | Size | Risk that could change it |
|---|---|---|
| 1 Define and download | **M** | Becomes L if the apply scheduler or checkpoint mishandles an ID that is both realized and in `ToDownload`, or partial entries plus an error. Needs checking in `jobLevels` and `plannedIDsCoveredBy`. |
| 2 Extract and index | **M** | Becomes L if PDFium wasm hangs or runs out of memory on large or odd real PDFs, forcing per-document timeouts or a subprocess. |
| 3 Stick | **M** | Becomes L if browsers ignore `#page=` from loopback HTTP and a bundled viewer (pdf.js) is needed. |
| 4 Docs and real check | **S** | Real documents may force threshold or format tweaks back in chunk 2. |
| **Overall** | **L** | It changes stored data (per-document realized entries, `documents.json`, `sources.note`) and plan semantics. It becomes XL only if the extractor has to move to Docker / poppler. |

## 7. Risks and unknowns

- **PDFium wasm on large files (UNVERIFIED).** Opening by `FilePath` in wasm mode may load the whole file into wasm memory, and a 300 MB scanned manual could be slow or fail. Also: cancellation only between pages, a pathological page could block, and the wazero compiler on linux-x86_64 builds is untested.
- **Page opening (UNVERIFIED).** `#page=` behaviour in Safari, Chrome, Firefox and Linux `xdg-open`. Also whether `open file://…#page` really drops the page on macOS, which is the reason for D6.
- **Accuracy unknowns.**
  - The 25% image-only threshold may flag manuals full of diagrams as "partly scanned".
  - Section labels from bookmarks may be wrong for PDFs with bad outlines.
  - Default-ranking quality: every page's title repeats the document title, which boosts all pages of a matching document.
- **Not detected.** Content changing behind an unchanged URL. Local files edited without a size change.
- **Merge conflicts** with in-flight branches: `catalog.go` (Item and Catalog fields), `toolkit.go` (one line), `root.go` (plan sites), and `mcp/search.go` / `vault.go` against the file-tools branch.
- **Not yet checked:** licences of go-pdfium (it ships a LICENSE file I didn't read) and of PDFium. `hosttui` showing user collections in the wizard's customise list (it should, via `AllRecipes`).
- **Real documents to test before calling it done:**
  1. A real 300+ page car owner's manual with multi-column layout and tables.
  2. A real scanned manual from archive.org without OCR.
  3. A partly scanned file.
  4. An AES password-protected file and an owner-only one.
  5. A file of 100 MB or more (memory and time).
  6. Finnish text with ä/ö (FTS tokenizer and snippets).
  7. A URL that returns an HTML login page.
  8. A redirect chain (the RPi URL does 301 then 302).
  9. A local folder of 3+ levels.
  10. A documents-only vault: menu shows Search, MCP works.
  11. Semantic indexing with a real embed model over documents plus a ZIM.
  12. `./run` opening a page on macOS and on Linux.

## 8. Questions for the owner

1. **Where should collection files live: in your config folder or on the stick?** Recommended: `~/.config/svalbard/recipes/` (D1). On another computer the installed files stay, but updates pause until the file is copied there. If you want them to travel with the stick, chunk 1 adds vault-aware catalog loading and grows to L.
2. **Is about +12 MB on the host CLI acceptable for PDFium-quality text?** Recommended: yes. The pure-Go fallback that's already a dependency drops spaces and ligatures and can't open AES files.
3. **Should one dead URL block indexing of the rest of the collection?** Recommended for phase 1: yes, as today. The item fails, downloaded documents are kept, and `svalbard index` works by hand. A warning-only path can follow as its own change.

Spike files are in `/Users/pkronstrom/.cache/svalbard-scratch/pdf/`:
- `spike/led`: the `ledongthuc/pdf` extractor
- `spike/pdfium`: the PDFium extractor
- `spike/bm`: bookmark and title reading
- `spike/enc`: makes the AES-256 test files with pdfcpu
- `docs/`: the test PDFs, including the synthetic `scanned.pdf`

The repo was not modified.

## 9. Owner decisions (2026-11-26)

1. **Collection files are recipes on the computer** (`$XDG_CONFIG_HOME/svalbard/recipes/`), and their files are staged onto the stick as for any other recipe (D1, option a).
2. **PDFium (wasm) is the extractor**; the larger host CLI is accepted (D3).
3. **A dead URL fails the item**, as any failed download does today (D10).

4. **The vault keeps a copy of each collection's recipe** (added 2026-11-26, changes D1). Apply writes the definition file byte for byte to `.svalbard/recipes/<id>.yaml`, follows it at every apply and removes it with the collection. The file in the config folder stays the one to edit and wins when both exist. A computer without the definition uses the vault's copy for plan and apply and says so; D1's "definition not found here; updates paused" now appears only when neither exists.
5. **A definition loaded from the vault's copy never reads `path:` sources from this computer**, not even when the path exists. Reason: a stick is something people hand to each other, and a prepared one must not be able to make `svalbard apply` copy host files onto it (`path: ~/.ssh/id_rsa`). The installed files of such entries stay, and each is reported as "local source, not refreshed from the vault's copy". Its `url:` documents are fetched as usual.

Sequencing: chunks 1 and 2 are host-side and start now. Chunk 3 changes the same runtime files as the search and read tool fixes on branch `search-tools`, so it starts after that branch is merged.

Kept open for later, not built here: the document pass in chunk 2 takes its file kinds from one place, so a second kind (Markdown pages from a wiki folder) can be added without reshaping it.

## 10. Changes made while building chunks 1 and 2 (2026-11-26)

These differ from the sections above. Where they conflict, this section is what the code does.

**Added by the owner after approval:** the recipe copy on the vault and its rule for `path:` entries, section 9 items 4 and 5.

**Changed for safety**

- **A local source that cannot be read keeps its installed files** (D7). A missing file, a missing or unplugged folder, and a folder with no documents in it all say nothing about what they held, so their documents on the vault are neither deleted nor refreshed, and the plan names each one. To remove them, take the entry out of the file. Such a source fails the item only when the vault holds nothing from it yet, like a dead URL.
- **Collection ids are compared without regard to letter case**, against built-in recipes too: on an exFAT stick `books/Manuals` and `books/manuals` are one folder.
- **File names a stick cannot store are rejected at load** (`<>:"/\|?*`, a trailing dot or space, `..`), for `filename:` and for names taken from a URL.
- **The index never prunes because a directory is missing** (chunk 2). On top of the manifest rule, `books/` and `zim/` may not be missing while the index still holds sources read from them. `books/` may be a symlink.
- **A file that cannot be read at all** (permission or disk error) is reported as skipped and keeps what the index held; the other documents are still indexed.

**Changed for quality**

- **Downloads identify as `svalbard`.** The plan's own example host (liberatedmanuals.com) answers Go's default agent with 403.
- **A title set, changed or removed in the collection file re-indexes the document** although the file is unchanged. The index records the title each document was given (`meta` key `document_titles`), since the indexed title alone cannot tell a given title from the PDF's own.
- **Document titles are written as soon as a collection's entries are saved**, not only by an apply that succeeds, so indexing by hand after a failed apply still cites documents by title.
- **Sections JSON has an empty heading** (D4 said the article title). The semantic chunker prefixes the article title itself; repeating it as the heading produced "Title, p. 3 > Title, p. 3".
- **`Body` is one line, `Sections` keeps the line breaks** (D4). Keyword search and snippets read the body as ZIM articles do; chunk 3 can read a page back with its table rows intact from the section.
- **Section label of a page** is the first outline entry that starts on the page, else the last one that started before it.
- **Words split across a line break follow the document's own spelling.** PDFium marks a line-end hyphen the same way whether it belongs to the word or only breaks it. The halves are joined when the document writes the word unbroken elsewhere or a half is no word of its own ("medica-tions"); the hyphen is kept when the document writes the compound with a hyphen elsewhere, the second half is capitalised, or both halves are words it never writes as one ("longer-acting"), so both stay findable. The soft hyphen and the `¬` of archive.org OCR layers always join.
- **One wasm instance per document**, not per index run (chunk 2): memory is released between documents and a file that breaks its instance cannot affect the next. PDFium reads the file through a Go reader, with no file system mounted into the module. Extraction stops in the middle of a page on cancellation and gives up on one file after 20 minutes.
- **Unchanged documents that are not searchable are listed again at every index run**, without being read again, so the problem stays visible.

**Known limits**

- A URL that is changed while the saved file name stays the same is not fetched again. Rename the file, or pin `sha256:`.
- A PDF deleted from the vault by hand is not noticed by `plan`, which compares the manifest and not the disk, as for every other recipe. The collection's next real change refetches it.
- `svalbard status` shows the size of one document for a collection, not the sum.
- Chunk 2 must not reach users without chunk 3: PDFs under `books/` (the built-in WHO manual included) now appear in search results, and the current stick runtime tries to open them through kiwix.
