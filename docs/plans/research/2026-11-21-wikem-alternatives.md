# WikEM: where the ZIM went, and what replaces it (2026-11-21)

Research only; no recipe or preset was changed. Checked against `recipes/content/wikem.yaml`, the ten medical
recipes listed in the brief, `cdc-travelers-health`, `librepathology`, and the presets that name `wikem`.

How things were checked:
- **Kiwix**: the full OPDS catalog (`https://opds.library.kiwix.org/catalog/v2/entries?count=-1`, 3,682 entries;
  `library.kiwix.org/catalog/...` now 301s to `opds.library.kiwix.org`), the directory listings of all 26
  categories under `lb.download.kiwix.org/zim/`, the archive tree `archive.download.kiwix.org/zim/` (with
  `other/`, `zimit1/`, `0.8/`, `0.9/`, `aimhi/`), and nine mirrors from a current `.meta4`, plus older mirrors.
- **Verified**: an HTTP HEAD (following redirects) returned 200 and a Content-Length on 2026-11-21. Sizes are
  those byte counts (GB = 10^9). For the WikEM copies I also ranged-GET the 80-byte ZIM header (magic, entry count,
  checksum position) and the 16-byte internal MD5. Nothing larger than one 2 MB sample PDF was downloaded.
- **Effort**: S / M / L as in the global sizing table.

Which presets use it: `default-2`, `default-32` and `packs/fi-survival` list `wikem` as a source.
`packs/survival-medical` names it only in a comment ("wikimed, wikem, … are already in the default preset
chain"), which goes stale once WikEM leaves `default-2`.

---

## 1. What happened to the ZIM

**It was not moved or renamed. Kiwix withdrew it.** No catalog entry's name, title, summary, tags or URL
contains "wikem" (`q=wikem` returns `totalResults 0`, as does `name=wikem_en_all`), and no file in any
`download.kiwix.org` category or in the archive tree contains it.

| Date | Event | Source |
|---|---|---|
| 2017-05 → 2021-02 | Regular builds (`wikem_en_all[_maxi\|_nopic\|_novid]_YYYY-MM`). The last of this run, `maxi_2021-02`, is 44.1 MB. The recipe was then paused. | Wayback CDX for `download.kiwix.org/zim/other/wikem*`; openzim/mwoffliner#71 |
| 2025-06-30 | openZIM revives the paused recipe (zim-requests#1460). Builds fail on WikEM's custom skin, PHP errors from the `PrivatePageProtection` extension, and 403s from wikem.org. | openzim/zim-requests#1460 |
| 2025-08-01 | WikEM rewrites its Terms of Use and adds an AI/ML clause and an automated-access ban (see §2). | `WikEM:Terms_of_use` revision history |
| 2026-04-25 | New build: `wikem_en_all_maxi_2026-04.zim` (48.7 MB, 13,875 entries) and `nopic_2026-04` (20.9 MB). These are English-only. | triplebit mirror listing and ZIM headers (below) |
| 2026-06-02 | WikEM puts machine translations into 15 languages on the web. | blog.wikem.org "15 Languages now live on WikEM" |
| 2026-07 | `wikem_en_all_maxi_2026-07.zim` was reported at **375 MB**: 101,394 HTML entries, 4.4 % of them English, each translation at `<Title>/<lang>`. Svalbard's `size_gb` went from 0.042 to **0.349** in commit 635c75d (2026-09-30), which matches this bloated build. | Crosstalk-Solutions/project-nomad#1453 (not verifiable now; no copy is left) |
| 2026-07-31 | WikEM's TranslationBot deletes 29,928 stale translations. | zim-requests#1964 |
| 2026-10-23 | A smaller rebuild (71,799 pages) moves to production. | zim-requests#1964 |
| 2026-10-29 | **"WikEM is all but unusable"**: the site's landing page now captures only links to an external blog. Kiwix unpromotes every WikEM book, flags the title `unstable` in its CMS, pauses the recipe, and marks the books for deletion. | **openzim/zim-requests#1970** (open, label Bug) |
| by 2026-11-21 | No WikEM file on download.kiwix.org, the archive tree, or any current mirror. | this check |

So the cause is upstream site changes (a custom skin, machine-translation bloat, and a landing page that now
points at the blog) that broke Kiwix's capture. WikEM's licence did not change. No Kiwix replacement title
exists.

### Copies that still exist

| Host | File | Size (B) | Date | Status |
|---|---|---|---|---|
| mirror.triplebit.org | `https://mirror.triplebit.org/download.kiwix.org/zim/other/wikem_en_all_maxi_2026-04.zim` | 48,676,248 | 2026-04-25 | **200**. Valid ZIM v6.3, 13,875 entries, internal MD5 `935ea43f392a5a70a9eaf3343b52d191` |
| mirror.triplebit.org | `…/wikem_en_all_nopic_2026-04.zim` | 20,915,021 | 2026-04-25 | **200**. ZIM v6.3, 11,157 entries |
| mirror.triplebit.org | `…/wikem_en_all_maxi_2021-02.zim` (also `2021-01`, `nopic_2021-0{1,2}`) | 44,098,910 | 2021-02-28 | 200. Same length as the archived Kiwix torrent. The archived Kiwix sha256 (`d7af2f87…abd7296`) can verify it |
| archive.org (`zimarchive` collection) | `https://archive.org/download/wikem_en_all_maxi_2020-06.zim/wikem_en_all_maxi_2020-06.zim` | 62,896,704 | 2020-06 | **200**. md5 `20ff8bb641f0bccba86ce28af00e592a`. Older items go back to 2018-03 |
| archive.org (WikiTeam) | `wiki-wikem.org_w-20231207`: `history.xml.zst` 25.7 MB plus `images.7z` 898 MB | — | 2023-12-07 | A MediaWiki XML dump, not a ZIM. Turning it into one needs a MediaWiki import plus mwoffliner (L) |
| Kiwix main + current mirrors | nluug, accum.se, dotsrc, fau.de, driftle.ss, mblibrary fr/in, isoc.il, mirror.download.kiwix.org, archive.download.kiwix.org | — | — | 404 everywhere (hacktegic: TLS handshake failure) |
| Kiwix sidecars | `.meta4`, `.torrent`, `.sha256` on download.kiwix.org | — | — | 404. The Wayback copy of the 2021-02 `.torrent` (infohash `25017e6ed663ed8816bd1b547037c27344d1f42e`) has a single webseed, nluug, which is now 404. No live source found |
| Wayback Machine | `download.kiwix.org/zim/other/wikem_*` | — | — | Only the 302 redirects were captured, never the file bodies. There are no captures of the 2026 builds |

About triplebit: in Nov 2025, download.kiwix.org's mirrorbrain redirected WikEM requests there (Wayback
capture 2025-11-19 → `mirror.triplebit.org/download.kiwix.org/...`), so it was an official Kiwix mirror. It is
**not** in the current `.meta4` mirror list. It still syncs new files (its newest are dated 2026-11-20) but
keeps files Kiwix deleted, so the WikEM copies there are leftovers that a cleanup could remove at any time.
Its directory hrefs are `./name.zim`. Svalbard's resolver anchors its regex (`^name_{date}.zim$`), so a
`url_pattern` on that host would not match. A pinned `url:` would work.

## 2. Upstream: wikem.org

- **Licence: still CC BY-SA, but the site gives two versions.** `WikEM:Copyrights` and `WikEM:General_disclaimer`
  link CC BY-SA **4.0**. `WikEM:Terms_of_use` says CC BY-SA **3.0** "unless otherwise noted". The MediaWiki
  `siteinfo` `rightsinfo` is empty. Either way the content is redistributable with attribution and ShareAlike.
  The recipe's `CC-BY-SA-4.0` matches the Copyrights page.
- **Terms added 2025-08-01** (by Rossdonaldson1, the editor-in-chief):
  - "any use of content … for the purpose of training, fine-tuning, or evaluating machine learning models or AI
    systems … is prohibited without prior written permission."
  - "**Any automated access (e.g., bots, crawlers, or spiders)**, especially for … large-scale data harvesting,
    **is prohibited without our express written consent**."
  - "This section does not waive any rights granted under the Creative Commons license."

  The WikEM name and logo are trademarks and may not imply endorsement.
- **robots.txt** (MediaWiki rules dated 2026-06-10): `Disallow: /w/` for all agents (this covers `api.php`,
  which mwoffliner uses), `Disallow: /wiki/Special:` (this covers `Special:Export`), `Crawl-delay: 5`, plus a
  Cloudflare content signal `search=yes, ai-train=no, use=reference`.
- **Dump/export**: there is no official dump. The only bulk copies are third-party: the WikiTeam 2023-12 dump
  and the archive.org ZIMs above.
- **Can Svalbard build its own (zimit/mwoffliner)?** **No, not without written consent from the OpenEM
  Foundation.** The Terms ban automated access and robots.txt blocks the API. Redistributing an
  already-built CC BY-SA copy is a different matter: the licence allows it, and the Terms say they do not
  restrict it. A local RAG index of a ZIM copy is not "training", but anyone wiring WikEM into Svalbard's
  embedding search should know the clause exists. A fresh build is openZIM's call (zim-requests#1970).

## 3. Alternatives

### WikEM's role, and how much the catalog already covers

WikEM is a concise, physician-level ED reference with one searchable page per presentation: differential,
workup, management and dosing. The existing catalog splits that role:

| Recipe | Size | Covers | Gap vs WikEM |
|---|---|---|---|
| `who-basic-emergency-care` | 4 MB | ABCDE triage and first-contact emergency care, low-resource settings | One PDF, not a searchable topic index; no dosing reference |
| `fas-military-medicine` | 65 MB | About 27 PDFs: Special Forces Medical Handbook (3 parts), Corpsman manual, first aid, surgery handbook, childbirth, malaria, dental | Field and medic level, manual-shaped PDFs |
| `zimgit-medicine` | 70 MB | "Books pertaining to field and emergency medicine" (PDF bundle) | Same: PDFs, little full-text search |
| `nhs-medicines` | 15 MB | Drug monographs and dosing (consumer level) | No acute or ED dosing |
| `wikimed` (MDWiki maxi) | **2.30 GB** actual (recipe says 2.1) | Broad encyclopedic medicine, well curated | Too big for `default-2`; encyclopedic rather than decision-support |
| `medlineplus`, `quick-guides-medicine`, `stackexchange-medicalsciences`, `libretexts-medicine`, `wikipedia-en-medicine-compact` | 0.06–1.9 GB | Consumer health, video, Q&A, textbooks, encyclopedia | None is ED decision support |

**Verdict:** for a 2 GB bugout preset, BEC + FAS + zimgit + NHS cover austere emergency care well enough. What
goes missing is the fast "look up presentation X, get the workup and the dose" page. For 8–32 GB presets,
`wikimed` already covers the topics, though in encyclopedic form.

### Candidates (all URLs HEAD-verified 2026-11-21 unless noted)

| # | Candidate | Exact URL | Size (B) | Licence | Updated | Fit for WikEM's role | Effort |
|---|---|---|---|---|---|---|---|
| 1 | **WikEM 2026-04 snapshot** | `https://mirror.triplebit.org/download.kiwix.org/zim/other/wikem_en_all_maxi_2026-04.zim` | 48,676,248 | CC BY-SA 4.0 | 2026-04-25 | **Exact** (it is WikEM, English-only, pre-translation). Host is fragile (§1) | S |
| 2 | WikEM 2020-06 (archive.org) | `https://archive.org/download/wikem_en_all_maxi_2020-06.zim/wikem_en_all_maxi_2020-06.zim` | 62,896,704 | CC BY-SA | 2020-06 | Exact, but six years old. Stable host | S |
| 3 | **Wikipedia Medicine "mini"** (Kiwix) | `https://download.kiwix.org/zim/wikipedia/wikipedia_en_medicine_mini_2026-10.zim` | 174,350,854 | CC BY-SA 4.0 | 2026-10-12 | Breadth, not depth: lead section and infobox of every WikiProject Medicine article. Weak on workup and dosing. Official, refreshed about every 6 months | S |
| 4 | Wikipedia Medicine "nopic" (Kiwix) | `https://download.kiwix.org/zim/wikipedia/wikipedia_en_medicine_nopic_2026-10.zim` | 1,047,708,415 | CC BY-SA 4.0 | 2026-10-10 | Full articles, no images. Overlaps `wikimed` and `wikipedia-en-medicine-compact`. Too big for `default-2` (1.13 + 1.05 = 2.18 GB) | S |
| 5 | **Joint Trauma System CPGs** (US DoD) | `https://jts.health.mil/assets/docs/cpgs/Zip_of_Current_JTS_CPGs_A-H.zip` and `…/Zip_of_Current_JTS_CPGs_I-Z.zip` | 74,399,528 + 87,530,606 = 161.9 MB | US Government work, public domain (17 USC 105). Some figures may be third-party | 2026-11-13 | Strong for trauma, prehospital, prolonged casualty care, burns, drowning, altitude, CBRN, airway, crush and envenomation: concise algorithmic guidelines. Military framing, no general internal medicine. Stable filenames, updated in place | **M**: Svalbard has no zip or PDF-bundle type. An S if the user accepts a few individual `type: pdf` recipes, but those filenames carry dates |
| 6 | StatPearls (NCBI Bookshelf open-access bulk) | `https://ftp.ncbi.nlm.nih.gov/pub/litarch/3d/12/statpearls_NBK430685.tar.gz` | 2,069,085,570 | CC BY-NC-ND 4.0 | 2026-11-20 | The best topical match (point-of-care review articles, heavy EM coverage). The tarball is JATS XML plus images, not a ZIM. Format conversion is allowed under CC 4.0 §2(a)(4) even with ND. zim-requests#1984 (open, 2026-11-07) asks openZIM for a ZIM | **L** (JATS→HTML→ZIM builder). An S if #1984 ships |
| 7 | Queensland Primary Clinical Care Manual, 11th ed. | `health.qld.gov.au/__data/assets/pdf_file/0016/1215043/PCCM_11th_ed.pdf` | **not verified** (Cloudflare managed challenge, 403 to curl) | CC BY-NC-SA 4.0 (per search results; not verified) | 2022/2024 updates. 12th ed. (2025) appears to be print-only | Very good: rural and remote clinical decision support with drug protocols | S as a `pdf` recipe, but **blocked**: getting past Cloudflare needs the user's go-ahead |
| 8 | LITFL (ECG, toxicology and critical-care libraries) | none (no ZIM) | — | CC BY-NC-SA 4.0 (site footer) | live | Very good match, but litfl.com returns 403 to non-browser clients. Do not self-scrape; it would need an openZIM request or the owner's consent | — |

Already proposed elsewhere: the OpenStax nursing PDFs (`2026-11-21-content-candidates.md` #4, CC BY-NC-SA
4.0) add nursing procedures and pharmacology to `survival-medical`, but they do not do WikEM's ED job.

**Excluded** (not openly licensed, or no redistribution right found): MSF Clinical Guidelines ("all rights
reserved … no reproduction without prior permission"), IBCC/EMCrit (free to read, no open licence),
NASEMSO National Model EMS Clinical Guidelines (offered "to any EMS entity", but no reuse licence; the
2014 edition says "All Rights Reserved"), Open Critical Care (CC BY-NC 4.0 on one page, "unauthorized
alteration … prohibited" in its Terms, no bulk download), Radiopaedia, Merck and UpToDate. The Kiwix catalog has
no other EM or clinical-decision ZIM. Its medical entries are MDWiki, Wikipedia Medicine (mini/nopic/maxi),
NHS, FAS, zimgit, LibrePathology, CDC Travelers, MedlinePlus, LibreTexts, Medical Sciences SE, Quick Guides
and TED.

---

## Recommendations

### (a) Mirror to point the recipe at

**One legitimate copy exists, on a fragile host:**
`https://mirror.triplebit.org/download.kiwix.org/zim/other/wikem_en_all_maxi_2026-04.zim` (200, 48,676,248 B,
2026-04-25). It is an unmodified Kiwix build of CC BY-SA content, served by a host that was a Kiwix mirror as of
2025-11. It is no longer in Kiwix's mirror list, and Kiwix withdrew the title, so expect it to disappear.

If you repoint:
- use a pinned `url:` (not `url_pattern`, because the `./` hrefs break the resolver);
- run `zimcheck -C` on the first fetch (internal MD5 `935ea43f…d191`), then pin that `sha256:`;
- set `size_gb: 0.049`. The current 0.349 describes the bloated multilingual 2026-07 build.

Once fetched, Svalbard's download cache keeps the copy even if the mirror drops it. The durable but older
fallback is archive.org `wikem_en_all_maxi_2020-06.zim` (62,896,704 B, md5 `20ff8bb6…592a`).

### (b) Replacement per preset

| Preset | Target | Today without `wikem` | Change | After |
|---|---|---|---|---|
| `default-2` | 2 GB | 1.13 GB | Replace `wikem` with a new recipe **`wikipedia-en-medicine-mini`** (`url_pattern: https://download.kiwix.org/zim/wikipedia/wikipedia_en_medicine_mini_{date}.zim`, 0.174 GB, CC BY-SA 4.0). It is official and stable. The existing BEC/FAS/zimgit/NHS items keep covering emergency procedures | **1.30 GB** |
| `default-32` | 32 GB | 17.62 GB | Drop `wikem`. `wikimed`, `medlineplus`, `quick-guides-medicine` and BEC already cover the topics. To fill the decision-support gap, add **JTS CPGs** (0.162 GB, public domain) once a zip/PDF-bundle type exists (M). Longer term, StatPearls via zim-requests#1984 | 17.6–17.8 GB |
| `fi-survival` | 8 GB | **21.95 GB (already about 14 GB over target)** | Drop `wikem` and add nothing. `wikimed`, BEC, FAS, zimgit and quick-guides already cover medicine, and the pack needs trimming anyway (s2-underground 4.4, ifixit 3.3, ready-gov 2.3, urban-prepper 2.1 GB) | 21.95 GB |

Also update the stale comment in `packs/survival-medical.yaml` (that pack is itself at 5.22 GB against a 2 GB
target).

The repointed `wikem` (49 MB) can be added to any of these as an opt-in extra. Keep it out of the built-in
defaults while it depends on a mirror Kiwix no longer lists.

### (c) The `wikem` recipe itself

**Repoint, don't delete (S).** Pin it to the triplebit 2026-04 URL with `sha256`, set `size_gb: 0.049`, and
change the description to "frozen 2026-04 snapshot; Kiwix withdrew the title 2026-10 (zim-requests#1970)".
Remove it from the three presets as in (b). It is the only copy of the best open EM reference, and it is small.

Watch zim-requests#1970. If Kiwix republishes, switch back to the
`download.kiwix.org/zim/other/wikem_en_all_maxi_{date}.zim` pattern, but check the size first: a build
full of translations is about 8× larger. If the triplebit file is gone before the first fetch, either point at
the archive.org 2020-06 copy or delete the recipe.

Do **not** build a fresh copy with zimit or mwoffliner. WikEM's Terms ban automated access without written
consent, and robots.txt blocks `/w/`.

---

## Sources
- openzim/zim-requests [#1460](https://github.com/openzim/zim-requests/issues/1460), [#1964](https://github.com/openzim/zim-requests/issues/1964), [#1970](https://github.com/openzim/zim-requests/issues/1970), [#1984](https://github.com/openzim/zim-requests/issues/1984); openzim/mwoffliner [#71](https://github.com/openzim/mwoffliner/issues/71)
- Crosstalk-Solutions/project-nomad [#1453](https://github.com/Crosstalk-Solutions/project-nomad/issues/1453) (size and makeup of the 2026-07 build)
- Kiwix OPDS `https://opds.library.kiwix.org/catalog/v2/entries`; listings `https://lb.download.kiwix.org/zim/<category>/`, `https://archive.download.kiwix.org/zim/`
- Wayback CDX `https://web.archive.org/cdx/search/cdx?url=download.kiwix.org/zim/other/wikem&matchType=prefix`
- archive.org items `wikem_en_all_maxi_2020-06.zim`, `wiki-wikem.org_w-20231207`
- wikem.org: `WikEM:Copyrights`, `WikEM:Terms_of_use` (and its revision history), `robots.txt`, `api.php?meta=siteinfo`; blog.wikem.org "15 Languages now live on WikEM" (2026-06-02)
- JTS CPG index `https://jts.health.mil/index.cfm/PI_CPGs/cpgs`
- NCBI Bookshelf open-access list `https://ftp.ncbi.nlm.nih.gov/pub/litarch/file_list.csv`
- MSF `https://medicalguidelines.msf.org/en`; Queensland Health PCCM pages (challenge-blocked); litfl.com (403)
