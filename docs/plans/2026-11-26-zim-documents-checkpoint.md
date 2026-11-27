# Documents inside ZIMs checkpoint

Status: built and verified 2026-11-26 on branch `zim-documents`, all eight tasks of the [plan](2026-11-26-zim-documents-plan.md), from the [design](2026-11-26-zim-documents-design.md). Based on `skill-vault-stage1` (`cf6c00c`): merge that branch first. Not yet merged.

## Resulting user path

1. **Index.** `svalbard index` (and `apply`) reads every PDF and EPUB stored inside a ZIM into a source of its own, `<zim>/<entry path>`, with one article per page; EPUBs in parts of a chapter. The archive's web-app files (scripts, styles, source maps, templates) are no longer indexed. EPUBs under `books/` are indexed too.
2. **Report.** One summary per archive that holds documents, then one line for each document that is not searchable (scanned, encrypted, empty, too large).
3. **Upgrade.** An index from an older `svalbard` is upgraded at the next run: archives that hold documents get them, archives that held web-app files are extracted again, and every other archive is left as it is.
4. **Use on the drive.** `search` ranks document pages from archives next to everything else, at most three pages of one document per keyword search; a search limited to an archive covers its documents; `search_read` reads a page from the index with no kiwix-serve, and `search_read` of an archive names its documents; `vault_sources` counts the documents of each archive; the search screen opens a PDF hit through kiwix-serve at its page.

The user-facing description is in [docs/usage.md](../usage.md), under `svalbard index` and "On the drive".

## Contracts to keep

- **A document inside an archive is a source named `<zim filename>/<entry path>`.** This adds a third kind to "document sources are named by vault path" from the [PDF collections checkpoint](2026-11-26-pdf-collections-checkpoint.md): `books/...` documents, ZIMs (no `/`), and documents inside a ZIM. `engine.EmbeddedArchive` is the one place on the drive that tells them apart.
- **An archive's scope holds its documents.** `InScope` and `scopeClause` agree; a scope naming a `.zim` covers the documents below it.
- **The reader version is a cache key only** (`sources.reader_version`, `zimReaderVersion` = 2), separate from file checksums and acquisition hashes. A ZIM source's version is written last, after its document pass: until then the archive is visited again, and documents already current are skipped.
- **The document pass runs only for archives whose header MIME list names PDF or EPUB** (`Archive.HoldsDocuments`). A Wikipedia ZIM is never walked for documents.
- **A PDF or EPUB beside an HTML entry of the same path without the extension is not indexed**: its text is indexed as that page.
- **The new index needs the matching drive runtime.** An older runtime would treat the new sources as archives. `apply` installs both.
- **Per-document outcomes never abort an index run** (kept from the PDF collections contract).

## Verification evidence

- `make verify`: the catalog sync check and 61 tested packages across the Go modules, after Task 7, and again after the Task 8 fixes.
- Fixture ZIM and EPUBs, built by `host-cli/internal/zimext/testdata/make_fixture.py` with python-libzim: unit tests for the junk filter, the document listing, duplicate skipping, nautilus titles, the size limit, the EPUB reader (spine order, a 49,000-character chapter in four parts, DRM, broken files), first index, a second run that reads nothing, upgrade, resuming after cancellation, pruning with a renamed archive, an archive with nothing to index, and on the drive: names, scopes, caps, reads, holdings, `vault_sources` and the open URL.
- Real ZIMs, CLI built from the branch, isolated HOME and XDG folders, copies by clonefile in the session's scratch folder:

| ZIM | Documents | Pages | Not searchable | Archive articles after |
|---|---|---|---|---|
| `usda-2015_en_2025-04` | 8 | 193 | 0 | 1 (`home`), was 19 |
| `zimgit-food-preparation_en_2025-04` | 17 | 1,126 | 1 scanned | 1, was 19 |
| `zimgit-post-disaster_en_2024-05` | 108 | 11,466 | 5 scanned, 1 without text | 1, was 19 |
| `wikem_en_all_maxi_2026-04` | 48 | 112 | 3 scanned | 4,505, unchanged |

  The counts match the extraction of the same PDFs as loose files before the change. A query for web-app paths and template titles in the archive sources returns 0 rows.
- **Fresh index** of the four: 76 s, 887 MB peak resident memory. A second run: 0.01 s.
- **Upgrade**: indexed with the CLI from `cf6c00c` (19 junk articles per nautilus ZIM), then with the branch: 71 s, 836 MB peak. The three nautilus ZIMs were extracted again; the WikEM and permacomputing HTML were not (`indexed_at` unchanged) and every ZIM records reader version 2. A second run: 0.01 s.
- **MCP over stdio** with the branch's drive runtime and no kiwix-serve on PATH: `search "pressure canning altitude"` returns pages from the USDA guide and from the two zimgit food ZIMs, cited "USDA Guide to Home Canning, Part 1: Principles of Home Canning, p. 23"; the same search limited to `usda-2015` stays inside it; `search_read` of `usda-2015/files/GUIDE04_HomeCan_rev0715.pdf` page 5 returns the asparagus page with its table lines; `search_read` of a loose EPUB part; `vault_sources` lists the document counts. With kiwix-serve: `search_read` of `usda-2015` and of `zimgit-post-disaster` return the main page with a note naming the documents and their page counts. An empty search names each archive with its document count.
- **Opening at the page**: `files/Water (6).pdf` from kiwix-serve 3.8.2 at the escaped URL `…/files/Water%20%286%29.pdf#page=7`, loaded in an isolated Chrome through chrome-devtools-mcp, showed page 7 of 31, the page of the search hit ("In South America, people purify water ponds with copper sulphate").

## Changes from the plan

- `OpenArchive` reads without gozim's cluster cache and memory mapping: with them the real run peaked at 1.6 GB, without them at 0.9 GB, with the same output and time.
- Nautilus titles: a title that names a section every book has ("Introduction", "Appendix B") is prefixed with the archive's title like "Part 2"; a slug title becomes words; a title without letters is dropped. "PowerPoint Presentation" is no longer taken as a PDF's title.
- A document inside an archive matches a loosely given name only when the name holds a `/`, so the archive's own name still finds the archive.
- EPUB DRM is `rights.xml`, or `encryption.xml` encrypting a content file; font obfuscation alone is not DRM.
- After the final review: an entry is junk only when it is not a page (an HTML article is never junk by its name: Wikipedia keeps "Node.js" at the path `Node.js`), and a page is a template only when its title is nothing but a placeholder (`{{ title }}`). The upgrade check streams an archive's articles, stops at the first junk entry, and runs only for archives that hold documents. `search_read` of an archive at a document's path (`files/GUIDE02_HomeCan_rev0715.pdf`, or `…/page/2`) reads the document from the index rather than the PDF's bytes from kiwix-serve. The real-ZIM index gave the same counts afterwards: 79 s, 858 MB peak.

## Scope and limits

- **No OCR.** Ten of the 181 real PDFs are scanned or partly scanned and are reported, not read.
- **Gutenberg duplicate skipping is unit-tested only.** The check on `gutenberg_fi_all_2026-01.zim` (1.4 GB) waits for the owner's go-ahead; the remote listing of that file showed every one of its 3,506 EPUBs beside an HTML book of the same name.
- **Video ZIMs** are not covered.
- **Collection files take PDFs only**; an EPUB is indexed when it is under `books/` by other means.
- **`search_read` of an archive without kiwix-serve** fails as before, so the list of its documents is only shown when kiwix-serve runs; the documents themselves read without it.
- **Hybrid search** keeps its own diversity step and is not limited to three pages per document.
- **From the final review, not fixed:** an unreadable entry size or a failed walk of an archive's entries stops the run (as an unreadable HTML entry already does); the search screen says "on the drive at …" for a document inside an archive it cannot open; a document's overview names its source as `file`; a bare document file name without a `/` is not resolved; two entries with the same file name in different folders of one archive share a catalogue title; two comments still describe two kinds of source.
- **Not checked:** semantic indexing of the new pages with a real model, Safari, Firefox and Linux opening, Linux hosts, physical sticks.
