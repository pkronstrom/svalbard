# Documents inside ZIMs: design

Status: approved by the owner, 2026-11-26. Reviewed by Codex Sol; its three findings are folded in (long EPUB chapters, resuming an interrupted document pass, hybrid search's page cap).

## Problem

Some ZIMs are a web page wrapped around a set of files. openZIM's nautilus ZIMs (the zimgit series:
water, medicine, knots, food preparation, post-disaster; `usda-2015`, the USDA home canning guide) are a
small JavaScript app plus PDFs. The search index reads only a ZIM's `text/html` and `text/plain` entries
(`host-cli/internal/zimext/extractor.go:117`), so it skips every PDF and indexes the app's source maps
and templates instead. Agents and the search screen see nothing of this content, which is core survival
material in the 2 GB and 32 GB presets.

Measured 2026-11-26 on the local copies, CLI built from `main`:

| ZIM | Entries indexed today | PDFs inside | Pages with text | Not searchable |
|---|---|---|---|---|
| `usda-2015_en_2025-04` | 19 junk (the 2-page HTML app shell, 17 source maps and handlebars templates) | 8, 14 MB | 193 | 0 |
| `zimgit-food-preparation_en_2025-04` | the same 19 | 17, 92 MB | 1,126 | 1 scanned |
| `zimgit-post-disaster_en_2024-05` | the same 19 | 108, 639 MB | 11,466 | 5 scanned, 1 with no text; 3 partly scanned |
| `wikem_en_all_maxi_2026-04` | its HTML (fine) | 48, clinical pathways | 112 | 3 scanned; 1 partly scanned |

Extracting all 181 PDFs took 73 s with a 608 MB peak on an M-series Mac. `gutenberg_fi_all_2026-01`, read
remotely through range requests, holds 3,506 EPUBs, each beside the HTML version of the same book
(`X.73459.epub` and `X.73459`).

## Decisions

- **Extraction runs on the host only**, in `svalbard index` (which `apply` runs). The drive runtime gains no
  PDF or EPUB code and reads page text from `data/search.db`, as it does for loose documents today.
- **Any ZIM**, not only nautilus ones: every `application/pdf` and `application/epub+zip` content entry is
  a candidate.
- **A PDF or EPUB entry is skipped when the same ZIM has an HTML article at its path without the
  extension.** This drops Gutenberg's duplicate EPUBs (and any duplicate PDFs) and keeps nautilus files.
- **Junk entries are skipped** in the HTML pass: scripts, styles, JSON, SVG, wasm, source maps and
  template files. The exact rule is fixed at plan time and must leave none of the 19 in the nautilus ZIMs.
- **Loose EPUBs under `books/` are indexed too**, with the same EPUB reader.
- **Out of scope:** OCR of scanned pages (a later step in the same host pass), video ZIMs (a separate
  investigation that starts by measuring one), other file types inside ZIMs.

## Index shape

```text
search.db source                          articles
───────────────────────────────────────   ─────────────────────────
usda-2015_en_2025-04.zim                  HTML pages, junk skipped
usda-2015_en_2025-04.zim/files/GUIDE02…   page/1 … page/32
books/manuals/volvo.pdf  (unchanged)      page/1 … page/356
books/novels/x.epub      (new)            page/1 … page/N  (parts, cited "ch. K")
```

- **One document source per embedded file, named `<zim filename>/<entry path>`.** ZIM sources never contain
  `/` and loose documents start with `books/`, so the name alone tells the three kinds apart. The drive's
  document handling applies once it recognises the new form: page numbers (`engine.PageNumber`), the
  per-document cap (`pagesPerDocument`, `drive-runtime/internal/search/engine/keyword.go:26`), reading from
  the index (`readDocument`), and a scope ending in `/` that lists a collection
  (`engine.InScope`). This changes the "document sources are named by vault path" contract from the
  PDF collections checkpoint; the checkpoint's other contracts hold.
- **Granularity:** a PDF article is one physical page (`page/N`, 1-based), as for loose PDFs. An EPUB
  article is one part of a spine item (a chapter), stored as `page/N` in reading order and cited
  "ch. K" with the chapter's heading, so the drive's page reader (neighbouring pages, overview, the
  per-document cap) serves EPUBs unchanged. A spine item longer than the per-article limit
  (`maxDocumentPartChars`, which `documentArticles` applies to both the body and the readable copy,
  `host-cli/internal/commands/index_documents.go:500`) is split at line breaks into consecutive parts
  ("ch. K (cont.)"), as the markdown reader already does with `splitAtLines`, so no text is dropped.
- **Titles:** nautilus's `database.js` (`ti` and `dsc` per file in `fp`, e.g. "Part 2: Fruit and Fruit
  Products" for `GUIDE02_HomeCan_rev0715.pdf`), else the file's own title metadata, else the file name.
- **Lifecycle follows the ZIM.** A ZIM never changes in place. Its embedded sources are rebuilt when the
  ZIM's identity changes and pruned when the ZIM or the entry is gone. A ZIM that has no HTML articles
  left but holds documents is indexed, not failed (today `errNoIndexableArticles` aborts it,
  `extractor.go:110`).
- **A reader version is recorded** with each ZIM source and each embedded source, beside the existing
  identity (UUID, size, modification time). It is an index cache key only, separate from file checksums
  and acquisition hashes. It is a new `sources` column added by `searchdb.Open`'s existing additive
  migration (`host-cli/internal/searchdb/db.go:117`).
- **The new index needs the matching drive runtime.** An older runtime would treat
  `<zim>/<entry>` sources as archives. `apply` installs the runtime with the index; a stick indexed by a
  newer CLI without `apply` is not supported.

## Host pass

```text
svalbard index (and apply)
  for each ZIM
    identity + reader version current? ── yes ─► skip
    HTML pass   text/html (junk filter) ─► ZIM source           (only if needed)
    document pass  pdf / epub entries, minus same-name HTML siblings
                   entry bytes ─► pdfext / EPUB reader ─► one source per file
    publish the ZIM source's reader version   (last: marks the ZIM complete)
  books/  documentKinds: .pdf (today), .epub (new)
```

- **Upgrading an existing index:** when the reader version is older, the document pass runs for ZIMs
  whose header MIME list (`zim.Archive.MIMETypes`) names PDF or EPUB, and the HTML pass re-runs only for
  a ZIM whose indexed paths include entries the junk filter now drops. Wikipedia and Gutenberg HTML are not
  re-extracted. Junk in ZIMs without documents (one entry in the Outdoors Stack Exchange ZIM) clears when
  that ZIM is next updated.
- **Reading an entry:** its bytes go to the existing extractor through a seekable reader
  (`requests.OpenDocument` takes a `FileReader`, `host-cli/internal/pdfext/pdfext.go:182`); nothing is
  written to disk. Entries above a size limit, set at plan time well above the 52 MB largest seen, are
  reported and skipped.
- **EPUB reader:** the zip's OPF gives the spine order; each XHTML item goes through the existing HTML
  section and strip functions. It serves both ZIM entries and loose files, through `documentKinds`
  (`host-cli/internal/commands/index_documents.go:55`), the one place that maps an extension to a reader.
- **Failure:** per-document outcomes never abort a run; only cancellation and database errors do.
  Scanned, encrypted, empty and broken files get a note. Each embedded source is written with the ZIM's
  identity and the reader version. The ZIM source's own reader version is written last, after the
  document pass has reconciled every entry, because a matching ZIM identity is what makes the indexer
  skip a ZIM (`host-cli/internal/commands/index.go:133`). An interrupted run therefore leaves the ZIM
  incomplete; the next run visits it again, keeps the embedded sources already current and reads only
  the rest.
- **Output:** one summary line per ZIM ("zimgit-post-disaster: 108 PDFs, 11,466 pages; 6 not searchable
  (5 scanned, 1 no text): …"), not one line per file.
- **Semantic indexing** embeds the new articles like any other, since it works over every source in
  `search.db` (`host-cli/internal/commands/index_semantic.go:131`).

## On the drive

| Action | Behaviour |
|---|---|
| `search` with no source | Ranks ZIM HTML, embedded documents and `books/` together. Keyword search returns at most 3 pages of one document; hybrid search keeps its own diversity step, as the PDF collections checkpoint already records |
| `search` with source `usda-2015` | Covers the ZIM's HTML and its embedded documents |
| `search_read` of a ZIM, no path | Its main page as today, plus a note naming the documents it holds with titles and page counts ("also holds 8 documents, read from the index: …"), the first 20 and a count of the rest |
| `search_read` of an embedded PDF, `page/12` | That page's text from the index; kiwix-serve is not needed |
| `search_read` of an EPUB, `page/3` | That part's text, with links to the neighbouring parts |
| `vault_sources` | A ZIM's description plus how many documents it holds |
| Search screen, open a PDF hit | kiwix-serve's copy in the browser at `#page=N` (`/content/<book>/<entry>`, as `drive-runtime/internal/search/session.go:211` builds for HTML) |
| Search screen, open an EPUB hit | The file, without a position: browsers do not render EPUB |

Hits cite "title, p. N" for PDF pages and "title, ch. K" for EPUB parts, using the titles above.

## Testing

- **Unit:** a fixture ZIM built by a committed script, holding an HTML page, a PDF, an EPUB, an EPUB with
  a same-name HTML sibling, a scanned PDF, an oversized entry and the nautilus junk set. A loose EPUB
  fixture under `books/`.
- **Real files:** the four ZIMs above. Page counts and the not-searchable lists must match the measurement
  table, and no junk entry may remain. Gutenberg duplicate skipping on `gutenberg_fi_all` (a 1.4 GB
  download; ask the owner first).
- **User paths**, in a scratch vault with isolated HOME and XDG directories:
  - create and apply a vault with the nautilus ZIMs; MCP over stdio: `search` across sources, `search`
    limited to one ZIM, `search_read` of a PDF page, of an EPUB part and of a ZIM without a path,
    `vault_sources`;
  - open a PDF hit from the search screen in an isolated Chrome and land on the page;
  - upgrade: index with the current `main` CLI, then the new one. Only ZIMs that hold documents or junk are
    read again, and a second run reads nothing;
  - interrupt the document pass, run again, and check it resumes: the documents already indexed are not
    read again, the rest are, and the ZIM is then skipped;
  - an EPUB with a chapter longer than `maxDocumentPartChars`: every part is searchable and readable in
    order.
