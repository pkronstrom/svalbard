# Built-in ZIM search versus Svalbard SQLite

## Method

Use the same ZIM files and archive scopes on both sides. Query Kiwix's embedded Xapian index and the Svalbard MCP keyword tool for up to five results. The latter uses the same SQLite search engine as drive search. Compare the rank of a preselected acceptable article, the number of questions where it appears in the top five, and mean reciprocal rank. Keep the result titles and paths for review. A narrowly specified target can miss a useful related article.

Run each query four times, alternating call order. The warm median excludes the first call for each query and includes local HTTP or stdio overhead. The pilot measures scoped retrieval. A global search that merges archives and the local document index needs its own relevance and latency measurement.

To measure the storage floor, build a fresh SQLite database on the Mac with the existing schema and full indexed text for sources that need local search. Include the two PhET archives, standalone and embedded documents, and other non-ZIM entries already in the index. Rebuild FTS5 and run its integrity check. Embeddings are outside this comparison. Keep the index on the stick intact.

## Pilot on the local scratch vault

The pilot used 11 title and longer queries in English Wikivoyage and Gardenology, including Finnish place names and plants.

| Backend | Expected article in top 5 | MRR@5 | Warm median |
| --- | ---: | ---: | ---: |
| Kiwix embedded index | **11/11** | **0.894** | 3.45 ms |
| Svalbard SQLite | 4/11 | 0.318 | **2.48 ms** |

For `Helsinki`, Kiwix put the city article first. SQLite surfaced related entries such as Greater Helsinki and Helsinki Cathedral. Kiwix also ranked the expected article first for `Helsinki airport railway train` and `Bergen airport Bybanen`. This is a small, scoped test. It supports trying the embedded index as the default for these archives.

The scratch vault's SQLite database was 1.17 GiB. The fallback rebuild was 5.36 MiB and retained both local PDFs, totaling 1,017 indexed pages. The stick has many more embedded documents, so this ratio describes the scratch vault only.

## Field comparison

The field test waits for the current USB writer to exit and the database and write-ahead log to settle. It compares only archives with a committed SQLite source and explicitly lists any excluded archives. The cases cover English and Finnish Wikipedia, English Wiktionary, Wikivoyage, and Gardenology. The raw query results and footprint measurement are written on the Mac under `~/.cache/svalbard-scratch/native-index-comparison/field/`.

The writer was interrupted during English Wiktionary and preserved 106 committed ZIMs. English Wiktionary and English Wikipedia were excluded from the paired results because they had no committed SQLite entries. The field comparison covered 15 scoped searches across Finnish Wikipedia, English Wikivoyage, and Gardenology.

| Backend | Expected article in top 5 | MRR@5 | Warm median |
| --- | ---: | ---: | ---: |
| Kiwix embedded index | **15/15** | **0.922** | **6.46 ms** |
| Svalbard SQLite | 8/15 | 0.400 | 44.98 ms |

All four Finnish Wikipedia queries found their expected article in both systems. Kiwix ranked it first in all four. SQLite ranked it first once and second three times. The warm timings describe scoped queries on this stick, including local call overhead.

After SQLite checkpointed and released the WAL, the committed database occupied **29.09 GiB** and the stick had **46.80 GiB free**. The separately rebuilt fallback database occupied **481 MiB**, a reduction of **28.62 GiB** from the current committed database. It retained 88,984 indexed articles and pages across 3,774 sources. These include the two PhET archives and 3,772 non-ZIM sources. The FTS5 integrity check passed, and a Finnish PhET lookup returned `Tiheys` from `/density_fi.html`.

Use embedded full-text search for archives that have it and retain SQLite for the fallback sources. The remaining comparison is a shared global ranker that combines native hits and SQLite results without swamping either source of evidence.
