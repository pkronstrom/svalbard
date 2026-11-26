# PDF collections checkpoint

Status: built and verified 2026-11-26 on branch `pdf-sources`; all four chunks of the [plan](2026-11-26-pdf-collections-plan.md). Not yet merged.

## Resulting user path

1. **Define.** A YAML file in `~/.config/svalbard/recipes/<id>.yaml` lists documents by `url:`, local file or local folder. It loads as a recipe, so `svalbard add <id>` and presets take it.
2. **Apply.** The files land in `books/<id>/`, one realized entry per document, and the vault keeps a copy of the recipe in `.svalbard/recipes/<id>.yaml`.
3. **Index.** Every PDF under `books/` becomes a source in `data/search.db` with one article per page that has text. Scanned, partly scanned, password-protected and unreadable files are recorded with a note and reported.
4. **Change.** Editing the file and applying fetches, deletes and re-indexes only the difference.
5. **Use on the drive.** The search screen and the MCP `search` / `search_read` tools find pages, read them from the index with no kiwix-serve, cite "title, p. N", and the screen opens the PDF in a browser at the page.
6. **Another computer.** Without the definition, plan and apply use the vault's copy; documents that came from a local path are kept and never refreshed from it.

The user-facing description is "Your own documents" in [docs/usage.md](../usage.md). The differences from the plan are in its section 10.

## Contracts to keep

- **Realized entries are a collection's complete file set.** One entry per document under `books/<id>/`. A failed document fails the item, and the documents that arrived are still recorded (`downloadResult.partial`).
- **A local source that cannot be read keeps its files.** A missing file, a missing or empty folder, and every `path:` entry of a vault's recipe copy are `catalog.UnreadableSource`: nothing installed from them is deleted or refreshed.
- **A vault's recipe copy never reads `path:` entries**, also when the path exists on this computer. A vault that is handed over must not be able to make apply copy host files onto it.
- **Collection ids are unique without regard to letter case**, and file names must be storable on exFAT.
- **Document sources are named by vault path** (`books/...`), ZIM sources never contain `/`. Articles are `page/N`, 1-based physical pages. `Body` is one line; the page's own lines are in `Sections[0].body` with an empty heading.
- **Identity is size and modification time**, the SHA-256 is recorded, and the title a collection gave is recorded in the `document_titles` meta key. These are index cache keys only, separate from file checksums and acquisition hashes.
- **Per-document outcomes never abort an index run.** Only cancellation and database errors do.
- **A missing or unlistable `books/` or `zim/` is never a prune.** Missing counts as empty only when the manifest records nothing there and the index holds no source from it.
- **PDFium runs on the host only**, in wazero with no file system mounted, one instance per document. The drive runtime has no PDF code and reads documents from `search.db`.
- **One place decides what is indexed:** `commands.documentKinds` in `host-cli/internal/commands/index_documents.go` maps an extension to its reader.
- **A search returns at most three pages of one document** unless it is limited to that document (`pagesPerDocument` in `drive-runtime/internal/search/engine/keyword.go`).

## Verification evidence

- `make verify`: catalog sync check and 59 tested packages across the five Go modules, after the final change.
- Scratch vault (`XDG_CONFIG_HOME` in scratch, hand-written v2 manifest), CLI and runtime built from the branch:
  - add, plan, apply of a collection with two URLs and a folder three levels deep; edit (+1, -1), plan, apply fetched only the new file and deleted the dropped one;
  - a real HTTP 403 and a web page behind a `.pdf` URL each failed the item, named the URL, and kept the documents that arrived;
  - index, touch one file (only it was read again), scanned and partly scanned files reported, sqlite rows checked;
  - the recipe copy was byte-identical to the config file; with an empty config folder plan and apply used it, listed each kept document and changed no file;
  - an emptied source folder kept its five installed documents.
- MCP over stdio with no kiwix-serve on the drive or the host: `search`, `search_read` of a page, of a document, of a collection, of a scanned document and of a page past the end; `vault_sources`; an empty search listing the collections with their descriptions.
- Search screen run from the applied vault with a stand-in for `open`: it opened `http://127.0.0.1:<port>/field-manuals/who-basic-emergency-care.pdf#page=205`. Loaded in an isolated Chrome through DevTools, the viewer showed page 205 of 240, the page of the hit.
- `pdfext` tests pass as linux/arm64 and linux/amd64 binaries in a local container (amd64 under emulation).

## Real-document matrix (plan section 7)

| # | Check | Result |
|---|---|---|
| 1 | Car owner's manual, 300+ pages, multi-column, tables | Volvo V60 2012 (356 pages) and Honda Civic 2001 (324 pages): all pages indexed, columns read in order, table rows stay on one line ("Coolant 3.0T T5 9.4 US qts. (8.9 liters)"). Neither is flagged as scanned; 18 and 3 of their pages hold a picture and almost no text. |
| 2 | Scanned manual from archive.org without OCR | Railroad Field Manual 1915, Google scan, 441 pages: "1 page; 440 of 441 pages are scanned images without text" (the one text page is the scanner's notice). |
| 3 | Partly scanned file | A datasheet merged with scans: "13 pages; 6 of 19 pages are scanned images without text". |
| 4 | Password-protected and owner-only | User password: "not searchable: password-protected". Owner-only: reads normally. |
| 5 | File of 100 MB or more | Machinery's Handbook 1924, 151 MB, 1,610 pages: 15 s to extract, 650 to 780 MB peak memory for the run, of which about 320 MB is compiling PDFium once. SIGINT during it stopped in 36 ms and kept the indexed pages. |
| 6 | Finnish text | Perusopetuksen opetussuunnitelman perusteet 2014, 472 pages: `äidinkieli` 112 hits, snippets and outline sections ("14.4.1 Äidinkieli ja kirjallisuus") intact. |
| 7 | URL that returns a web page | Rejected: "not a PDF: the source returned a web page (a login or error page?)". |
| 8 | Redirect chain | The Raspberry Pi datasheet URL, 301 then 302: fetched. |
| 9 | Local folder of 3+ levels | Mirrored with its subfolders. |
| 10 | Documents-only vault | The drive menu offers Search; MCP search and read work. |
| 11 | Semantic indexing with a real embedding model | Not run. Covered by a test with a stand-in embedder only. |
| 12 | Opening a page from the drive | Chrome on macOS over the loopback address: lands on the page. Safari, Firefox and Linux `xdg-open` not checked. |

The 25% threshold for "partly scanned" was not changed: no real file showed it wrong.

## Measured

- Host CLI: 34.6 MB on `main`, 48.5 MB on the branch (+13.9 MB), dev builds without embedded drive runtimes. The drive runtime gained no PDF library.
- Extraction: 240 pages in 1.4 to 3.4 s and 421 pages in 1.1 to 2.5 s depending on machine load, plus 1.4 to 3.4 s once per run to compile PDFium.

## Scope and limits

- **No OCR.** Scanned files are reported, not read.
- **Only PDFs are indexed.** Other files a collection downloads are stored and not searchable.
- **A changed document behind an unchanged URL** is not fetched again unless its `sha256:` pin or its `filename:` changes.
- **A PDF deleted from the vault by hand** is not noticed by `plan`, which compares the manifest; the collection's next real change refetches it.
- **`svalbard status`** shows the size of one document for a collection, not the sum.
- **A compound broken at a hyphen on a line end** keeps its hyphen only when the document shows that it is one; see plan section 10.
- **The hybrid search path** is not limited to three pages per document; its own diversity step applies.
- **Not checked:** semantic indexing with a real model, Safari and Firefox page opening, Linux and physical sticks at run time, the libraries inside go-pdfium's WebAssembly build other than libjpeg-turbo ([THIRD-PARTY-NOTICES.md](../../THIRD-PARTY-NOTICES.md)).
