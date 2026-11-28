# Kiwix/openZIM ZIMs missing from the Svalbard catalog (as of 2026-11-26)

Method: I parsed the full library.kiwix.org OPDS feed (`/catalog/v2/entries?count=-1`, 3,684 entries, fetched 2026-11-26) and the Apache listings of 22 `download.kiwix.org/zim/<dir>/` folders, then compared both with `catalog-index.txt` (227 recipes). Every path below appears in a listing, and I HEAD-checked a sample of 29 shortlisted URLs: all returned 200. Sizes come from the directory listings (Apache "G/M", about GiB/MiB). Dates are the `_YYYY-MM[a-z]` filename stamp plus the listing mtime. `https://download.kiwix.org/zim/` itself now 301s to hub.kiwix.org/downloads, but the subdirectories still list files.

Sources used throughout:
- OPDS: https://library.kiwix.org/catalog/v2/entries?count=-1
- Listings: https://download.kiwix.org/zim/{other,zimit,videos,books,community,maps,gutenberg,stack_exchange,ted,devdocs,phet,wikipedia,wikivoyage,wikiversity,wikiquote,wikisource,wikibooks,wiktionary,freecodecamp,libretexts,ifixit,mooc}/
- Requests: https://github.com/openzim/zim-requests/issues

## What is on download.kiwix.org that the catalog lacks?

### Takeaway
Two directories hold content the catalog never references: `books/` (the November 2026 Gutenberg split plus the USAID archive) and `community/` (new community uploads, including **The Last Codex**, **Learn AI Anywhere** and a 6.8 G **Gutenberg Top 2500**). A third, `maps/`, holds OSM map ZIMs. The best value per GB in the existing folders is `wikipedia_en_knots`, the BeReady UK site, targeted TED topics, three more Canadian Prepper sets, Gutenberg LCC U/V, a cheap DevDocs bundle and about 15 Stack Exchange sites.

### Cited Findings

**New directories**
- `community/` lists the following community-contributed ZIMs, none of them in the catalog ([listing](https://download.kiwix.org/zim/community/)):
  - `lastcodex.org_en_all_2026-10.zim`: 264M, 2026-11-02. Also available in de, es, fr, it and pt at about 268M each, dated 2026-11.
  - `learnaianywhere_en_all_2026-11.zim`: 96M. Also in ar, es, fr, hi, pt, sw-ke, sw-tz and ur.
  - `gutenberg_en_top_2026-11.zim`: 6.8G.
  - `worldwide-equipment-guide_en_all_all_2026-11.zim`: 123M.
  - `zimi_mul_almanac_mini_2026-11.zim` (11M) and `zimi_mul_almanac_maxi_2026-11a.zim` (4.5G).
  - `hitchwiki.org_en_all_2026-09a.zim` (20M), `trashwiki.org_en_all_2026-09a.zim` (3.8M) and `nethackwiki_en_all_full_2026-10.zim` (27M).
  - Portuguese-only: `plantas-medicinais_pt_all_2026-11.zim` (28M), `chuvas-tempestades_pt_all_2026-11.zim` and `lugares-seguros_pt_all_2026-11a.zim`.
  - About 20 `sermonindex.net_*` files.
- The Last Codex describes itself as an "Illustrated codex of human inventions and how to rebuild them". It is licensed CC BY-NC-ND 4.0, one ZIM per language, and the site owner submitted the request ([zim-requests #1950](https://github.com/openzim/zim-requests/issues/1950), closed 2026-11-02).
- Learn AI Anywhere is an AI-literacy course, licensed CC BY-NC-SA 4.0 and uploaded by its authors. It went to production on 2026-11-16 ([#1969](https://github.com/openzim/zim-requests/issues/1969)).
- `gutenberg_en_top` contains "the top 2500 most downloaded books from Project Gutenberg" at about 7 GB, compared with more than 250 GB for the full set. It sits in the community collection, and the reviewer noticed a few French and Spanish titles mixed in ([#1985](https://github.com/openzim/zim-requests/issues/1985)).
- The US Army Worldwide Equipment Guide ZIM is a public-domain TRADOC G-2 reference to military equipment, contributed on 2026-11-24. A maintainer still called the in-page search text "pretty ugly" ([#2001](https://github.com/openzim/zim-requests/issues/2001)).
- Zimi Almanac bundles NASA imagery, the Yale Bright Star Catalogue, the ESO Milky Way panorama and satellite.js under GPL-3.0 and mixed data licences. It comes in mini and maxi flavours ([#1988](https://github.com/openzim/zim-requests/issues/1988)).
- `books/` holds the November 2026 Gutenberg rebuild. Files carry a letter suffix: `gutenberg_en_lcc-{a,b,cdef,g,h,j,k,l,m,n,p,pa…pz,q,r,s,t,u,v,z}_2026-11a.zim`, `gutenberg_fi_all_2026-11a.zim` (1.4G) and `gutenberg_mul_all_2026-11c.zim` (184G) ([listing](https://download.kiwix.org/zim/books/)). The relevant new classes are:
  - `lcc-u` (Military Science): 864M
  - `lcc-v` (Naval Science): 774M
  - `lcc-q` (Science): 12G
  - `lcc-g` (Geography, Anthropology, Recreation): 5.8G
  - For comparison with existing recipes: `lcc-r` 1.4G, `lcc-s` 3.2G, `lcc-t` 8.9G.
- The old `gutenberg/` folder still exists. Its newest English LCC files are dated 2026-03 (for example `gutenberg_en_lcc-r_2026-03.zim`, 1.9G), and its newest Finnish file is `gutenberg_fi_all_2026-01.zim` ([listing](https://download.kiwix.org/zim/gutenberg/)). The OPDS feed lists the `books/` copies.
- `books/usaid-archive.aiddata.org_en_all_2026-11a.zim` is 8.0G, dated 2026-11-20. Its OPDS summary reads "AidData's archive of USAID's Development Experience Clearinghouse evaluations" ([listing](https://download.kiwix.org/zim/books/); [#1897](https://github.com/openzim/zim-requests/issues/1897), closed 2026-11-23).
- `maps/` has 193 OSM map ZIMs (`maps_en_<country|region>_2026-0x.zim`, plus a 77.7 GB world file). There is **no Finland-only file**. `maps_en_nordics_2026-08a.zim` is 5.3G and its OPDS summary reads "Regional map for Finland, Sweden, Norway, Denmark, Faroe and Iceland." Single-country neighbours are `maps_en_estonia_2026-06` (260M), `maps_en_sweden_2026-06` (2.0G) and `maps_en_norway_2026-06` (3.9G) ([listing](https://download.kiwix.org/zim/maps/)). The regional files came from [#1891](https://github.com/openzim/zim-requests/issues/1891) and moved to production on 2026-08-24. On 2026-06-23 maintainers warned that "you might have issues with kiwix-desktop … and old versions of kiwix-android" ([#1309](https://github.com/openzim/zim-requests/issues/1309)).

**other/ and zimit/ (ZIMs built from websites)**
- `wikipedia/wikipedia_en_knots_maxi_2026-10.zim`: 13M. Knots by Wikipedia ([listing](https://download.kiwix.org/zim/wikipedia/)).
- `zimit/beready.campaign.gov.uk_en_all_2026-11.zim`: 158M, 2026-11-25. UK government preparedness guidance ("Be ready UK") ([listing](https://download.kiwix.org/zim/zimit/)).
- `zimit/armypubs_en_all_2024-12.zim`: 7.7G. Army Publishing Directorate, the US Army's central publications site. This is an old build ([listing](https://download.kiwix.org/zim/zimit/)).
- `zimit/ready.gov_en_all_2026-10.zim`: 1.9G, 2026-10-15. This is a **renamed** series. The catalog's `www.ready.gov_en_{date}` pattern stops at `www.ready.gov_en_2024-12.zim` ([listing](https://download.kiwix.org/zim/zimit/)).
- `other/theworldfactbook_en_all_2026-02.zim`: 410M. The OPDS title is "CIA World Factbook - 2020" ("now-discontinued almanac") ([OPDS](https://library.kiwix.org/catalog/v2/entries?count=-1)).
- `other/postmarketos_en_all_maxi_2026-08.zim`: 29M. postmarketOS wiki ([listing](https://download.kiwix.org/zim/other/); [#1611](https://github.com/openzim/zim-requests/issues/1611)).
- `other/skin-of-color-society_en_all_2026-11.zim`: 1.41 GB per OPDS. Dermatology for skin of colour ([OPDS](https://library.kiwix.org/catalog/v2/entries?count=-1)).
- `other/mdwiki_en_all_2025-11.zim`: 10.75 GB, the full (non-maxi) WikiMed flavour. The catalog uses the 2.30 GB maxi ([OPDS](https://library.kiwix.org/catalog/v2/entries?count=-1)).
- Computing sites under zimit, each a distinct file:
  - `developer.mozilla.org_en_all_2026-11.zim`: 560M (MDN)
  - `docs.python.org_en_all_2026-11.zim`: 3.6G
  - `lua.org_en_all_2026-11.zim`: 362M
  - `gobyexample.com_en_all_2026-10.zim`: 323K
  - `opendatastructures.org_en_all_2026-05.zim`: 111M
  - `jeffe.cs.illinois.edu_en_all_2026-09.zim`: 493M (Algorithms textbook)
  - `htdp.org_en_all_2026-11.zim`: 1.9M
  - `noted.lol_en_all_2026-10.zim`: 2.1G (self-hosting and open-source publication)
  - `cloudflare.com_en_learning-center_2025-12.zim`: 182M ("Resources on cyber security and how the Internet works")
  - `incognitocat.me_en_all_2025-09.zim`: 8.9M ("Tools and techniques to safeguard your digital well-being")
  - Source: [listing](https://download.kiwix.org/zim/zimit/)
- Cooking and food:
  - `zimit/foss.cooking_en_all_2026-11.zim`: 29M
  - `zimit/publicdomainrecipes.com_en_all_2026-11.zim`: 16M
  - Source: [listing](https://download.kiwix.org/zim/zimit/)
- General and open-textbook collections:
  - `zimit/milneopentextbooks.org_en_all_2026-11.zim`: 1.5G
  - `zimit/onemorelibrary.com_en_all_2026-11.zim`: 3.7G (public-domain books)
  - `zimit/encyclopedie-environnement.org_en_all_2026-11.zim`: 4.0G
  - `other/wikispecies_en_all_nopic_2026-10.zim`: 1.67 GB
  - Source: [listing](https://download.kiwix.org/zim/zimit/); [OPDS](https://library.kiwix.org/catalog/v2/entries?count=-1)

**videos/**
- Three more Canadian Prepper sets (the catalog has only `winterprepping`):
  - `canadian-prepper_en_bugoutconcepts_2026-11.zim`: 2.7G
  - `canadian-prepper_en_bugoutroll_2026-10.zim`: 838M
  - `canadian-prepper_en_preppingfood_2026-10.zim`: 2.0G
  - Source: [listing](https://download.kiwix.org/zim/videos/)
- Corey Schafer tutorials:
  - `coreyms_en_python-tutorials_2026-09.zim`: 2.4G
  - `coreyms_en_git-tutorials_2026-09.zim`: 57M
  - `coreyms_en_sql-tutorials_2026-09.zim`: 22M
  - Also Django, Flask, pandas and matplotlib sets of 104–271M each
  - Source: [listing](https://download.kiwix.org/zim/videos/)
- `videos/kris-occhipinti_en_all_2026-10.zim`: 175M ([listing](https://download.kiwix.org/zim/videos/)).

**ted/ (all `ted_mul_<topic>_2026-09.zim`; the catalog has 3d-printing, electricity and farming)**
- Medical and survival: water 832M, natural-disaster 683M, medicine 3.4G, public-health 2.2G, vaccines 658M, virus 798M, pandemic 1.0G, mental-health 2.2G, sleep 337M ([listing](https://download.kiwix.org/zim/ted/)).
- Homesteading: gardening 311M, plants 633M, agriculture 1.2G, food 2.2G, insects 859M, weather 368M.
- Engineering: solar-energy 515M, renewable-energy 1.0G, wind-energy 166M, nuclear-energy 177M, energy 1.8G, engineering 2.7G.
- Cybersecurity: cyber-security 195M, online-privacy 492M. AI: machine-learning, 1.46 GB per OPDS.

**devdocs/ (all `devdocs_en_<x>_2026-1x.zim`, mostly under 3 MB each; the catalog has c, cpp, go, javascript, python and rust)**
- Available ([listing](https://download.kiwix.org/zim/devdocs/)):
  - Shells and tools: bash 559K, fish 581K, zsh 763K, git 1.5M, jq 377K, gcc 1.6M, cmake 2.8M, powershell 1.9M
  - Infrastructure: docker 1.7M, nginx 820K, kubernetes 603K, nix 391K, ansible 27M, terraform 30M
  - Databases: postgresql 2.5M, sqlite 3.3M, redis 851K, mariadb 9.3M
  - Languages: lua 418K, zig 479K, typescript 971K, node 1.4M (2026-08), openjdk 14M, kotlin 7.8M, haskell 3.9M, erlang 3.7M, perl 5.1M, php 7.2M, ruby 2.0M
  - Web: html 1.6M, css 2.6M, dom 11M, http 1.9M, markdown 325K
  - Python data and ML: numpy 5.0M, pandas 4.7M, matplotlib 32M, scikit-learn 56M, pytorch 6.1M, tensorflow 6.0M
  - Other: man 28M, qt 48M, latex 766K, octave 1.9M, r 4.7M

**stack_exchange/ (all `<site>_en_all_2026-10.zim` unless noted)**
- Not in the catalog. I checked these by hand against the truncated index lines, because networkengineering, reverseengineering and softwareengineering are already in the catalog. Source: [listing](https://download.kiwix.org/zim/stack_exchange/).
  - Computing and security: android 393M, apple 1.1G, retrocomputing 120M, tor 30M, vi 87M, emacs 146M, hardwarerecs 40M, softwarerecs 212M, opensource 29M, webapps 276M, webmasters 216M
  - Computer science and AI: cs 264M, ai 100M, genai 11M, datascience 260M, stats 1.5G, scicomp 94M, bioinformatics 55M
  - Science: astronomy 188M, space 381M, aviation 487M, mathoverflow.net 821M, tex 4.2G (2026-08)
  - Everyday life: lifehacks 47M, pets 83M, parenting 57M, fitness 76M, law 179M, money 242M, travel 441M, expatriates 41M
  - Food and drink: coffee 17M, alcohol 13M, vegetarianism 9.3M
  - Hobbies: drones 12M, sound 73M, photo 430M
  - Cryptocurrency: bitcoin 149M, monero 21M

**phet/, freecodecamp/, Wikimedia projects**
- `phet/phet_en_all_2026-11.zim` (110M) and `phet/phet_mul_all_2026-11.zim` (148M). The catalog has only `phet_fi` ([listing](https://download.kiwix.org/zim/phet/)).
- `freecodecamp/freecodecamp_en_all_2026-11.zim`: 11M ([listing](https://download.kiwix.org/zim/freecodecamp/)).
- English Wikipedia subject subsets dated 2026-09 to 2026-11 that the catalog lacks. Sizes are maxi / nopic / mini ([listing](https://download.kiwix.org/zim/wikipedia/)):

  | Subset | maxi | nopic | mini |
  |---|---|---|---|
  | chemistry | 366M | 132M | 22M |
  | physics | 870M | 306M | 51M |
  | mathematics | 950M | 349M | 56M (2026-06) |
  | molcell | 695M | 339M | none |
  | computer | 868M | 493M | none |
  | climate-change | 182M | 86M | 12M |
  | astronomy | 1.2G | 557M | none |
  | geography | 1.1G | 586M | none |
  | history | 1.9G | 736M | none |

- English Wikipedia size ladder: the catalog lacks the mini flavours and the top-1M selection ([listing](https://download.kiwix.org/zim/wikipedia/); [#1631](https://github.com/openzim/zim-requests/issues/1631)):
  - `wikipedia_en_top_mini_2026-09.zim`: 283M
  - `wikipedia_en_all_mini_2026-09.zim`: 13G
  - `wikipedia_en_top1m_mini_2026-11.zim`: 4.9G
  - `wikipedia_en_top1m_nopic_2026-11.zim`: 22G
  - `wikipedia_en_top1m_maxi_2026-04.zim`: 46G
- `wikivoyage/wikivoyage_en_all_nopic_2026-09.zim` (260M; maxi 1.0G) and `wikivoyage_en_europe_nopic_2026-09.zim` (80M; maxi 367M) ([listing](https://download.kiwix.org/zim/wikivoyage/)).
- `wikiversity/wikiversity_en_all_nopic_2026-11.zim`: 1.5G; maxi 2.0G ([listing](https://download.kiwix.org/zim/wikiversity/)).
- `wikisource/wikisource_en_all_nopic_2026-09.zim`: 3.6G ([listing](https://download.kiwix.org/zim/wikisource/)).

**Problems in existing recipes found during the diff**
- `usda-nutrition.yaml` describes "USDA National Nutrient Database", but the OPDS title of `usda-2015_en` (2025-04) is **"USDA Guide to Home Canning"** ([OPDS](https://library.kiwix.org/catalog/v2/entries?count=-1)).
- `ready-gov.yaml` uses `https://download.kiwix.org/zim/zimit/www.ready.gov_en_{date}.zim`, whose newest file is 2024-12. The current series is `ready.gov_en_all_2026-10.zim` ([listing](https://download.kiwix.org/zim/zimit/)).
- No `wikem*` file appears in `other/` or in the OPDS feed. [#1964](https://github.com/openzim/zim-requests/issues/1964) (2026-10-23) explains the article-count drop: on 2026-07-31 a TranslationBot deleted 29,928 stale machine-translated pages on wikem.org. A maintainer then wrote "Also moved ZIMs to production", but no file is listed. This matches the catalog's pinned mirror snapshot.
- Survivor Library: the latest file is `survivorlibrary.com_en_all_2026-09.zim` at 205G. Its size has swung between 235G and 2.7G ([#1690](https://github.com/openzim/zim-requests/issues/1690)).
- No newer builds exist for `medlineplus.gov_en_all` (2025-01), `wwwnc.cdc.gov_en_all` (2024-11) or `solar.lowtechmagazine.com_mul_all` (2025-01) ([listing](https://download.kiwix.org/zim/zimit/)).

### Inferences
- **Top additions by value per GB:**
  1. The Last Codex (en, 264M): homesteading or engineering, and a natural companion to Wikiciv.
  2. `wikipedia_en_knots` (13M): survival-medical.
  3. BeReady UK (158M): survival-medical.
  4. TED water (832M) and natural-disaster (683M): survival-medical.
  5. TED gardening (311M) and plants (633M): homesteading.
  6. TED solar-energy (515M): engineering.
  7. TED cyber-security (195M) and online-privacy (492M): cybersec.
  8. Gutenberg LCC U (864M) and V (774M): survival and engineering history.
  9. A DevDocs bundle of bash, git, docker, nginx, postgresql, sqlite, lua, man, typescript, node, html/css/http, gcc and cmake: computing, under 60 MB combined.
  10. postmarketOS wiki (29M): computing or embedded.
  11. Learn AI Anywhere (96M): ai pack.
  12. PhET en (110M): sciences.
  13. Wikipedia chemistry, physics, mathematics and molcell nopic (132–349M each): sciences.
  14. Wikivoyage en nopic (260M): core.
  15. The CIA World Factbook (410M): core.
- **Large but strong** (opt-in or XL presets):
  - `armypubs` (7.7G): field manuals for survival-medical. It is a 2024 build.
  - Canadian Prepper bugoutconcepts and preppingfood (4.7G combined): survival-medical.
  - Gutenberg LCC Q (12G): sciences.
  - `gutenberg_en_top` (6.8G): core reading.
  - USAID DEC archive (8G): development and agriculture evaluations; moderate relevance.
  - `wikipedia_en_top1m_mini` (4.9G): a middle rung between top-50k and all.
- **Stack Exchange picks** that fit existing packs:
  - computing: android, apple, retrocomputing, vi, emacs, hardwarerecs, softwarerecs, cs
  - cybersec: tor
  - homesteading: pets, lifehacks, coffee, vegetarianism, alcohol
  - sciences: astronomy, space, stats
  - ai: ai, datascience, genai
  - Skip: religion, gaming and fandom sites, CMS vendor sites.
- `worldwide-equipment-guide` (123M) mainly suits an OSINT or intelligence niche next to S2 Underground. It is low priority, and its search-text quality was still under review on 2026-11-24.
- Maps ZIMs are an alternative to the protomaps recipes rather than a gap. The Nordics file is 5.3G and there is no Finland-only build. Reader compatibility is caveated, so the existing `maps/osm-finland` approach is likely still better for fi-maps.
- Gutenberg recipes that point at `gutenberg/` will trail the `books/` copies by about 8 months. The `books/` files use letter suffixes (`2026-11a`), and the date matcher must accept those. Both points are worth checking in the update path.

### Gaps
- I did not test whether bundled kiwix-serve 3.8.2 renders the `maps/` ZIMs. The only evidence is the compatibility caveat in #1309.
- No Hesperian ("Where There Is No Doctor"), WHO or MSF ZIM exists in the OPDS feed or any listing (I searched for hesperian, who.int, msf, doctor, first aid and redcross). These would need custom recipes, which is outside this scope.
- I found no new emergency-medicine ZIM that could replace WikEM. The nearest medical additions are TED medicine, Skin of Color (dermatology, 1.4 GB) and the full mdwiki flavour.
- I did not open `armypubs` or `usaid-archive` to measure how much of their content is relevant (for example which FMs are included). Their value per GB is an estimate.
- I could not identify the content of `project-fuel_en_all` (182M) or `rabbitwaves.ca` (78M, "Useful knowledge as comics") from metadata alone.

## Which ZIM collections were added in 2025–2026?

### Takeaway
Activity in 2025–2026 was mostly community uploads (the `community/` folder) and small zimit sites, plus three structural changes: OSM maps ZIMs (June–August 2026), the Gutenberg split into `books/` (November 2026) and quarterly update cadence. The additions relevant to preparedness are The Last Codex, CD3WD, Wikiciv, Permacomputing, Restarters, Libre Pathology, WikiVet, BeReady UK, Learn AI Anywhere and postmarketOS.

### Cited Findings
- Closed zim-requests issues with the date each request or upload was completed:
  - cd3wdproject.org: 2025-11-13 ([#1543](https://github.com/openzim/zim-requests/issues/1543))
  - Wikiciv: 2025-11-10 ([#1395](https://github.com/openzim/zim-requests/issues/1395))
  - Restarters wiki: 2025-09-15 ([#1357](https://github.com/openzim/zim-requests/issues/1357))
  - Libre Pathology: 2025-09-15 ([#1523](https://github.com/openzim/zim-requests/issues/1523))
  - WikiVet: 2026-07-22 ([#1558](https://github.com/openzim/zim-requests/issues/1558))
  - Permacomputing: 2026-03-25 ([#1412](https://github.com/openzim/zim-requests/issues/1412))
  - postmarketOS: 2026-02-16 ([#1611](https://github.com/openzim/zim-requests/issues/1611))
  - incognitocat.me: 2025-09-30 ([#1548](https://github.com/openzim/zim-requests/issues/1548))
  - foss.cooking: 2025-11-07 ([#1485](https://github.com/openzim/zim-requests/issues/1485))
  - Corey MS: 2025-10-06 ([#1511](https://github.com/openzim/zim-requests/issues/1511))
  - UnrealIRCd docs: 2026-02-11 ([#1632](https://github.com/openzim/zim-requests/issues/1632))
  - ncase.me: 2026-02-17 ([#1545](https://github.com/openzim/zim-requests/issues/1545))
  - top1m nopic and mini: 2026-03-23 ([#1631](https://github.com/openzim/zim-requests/issues/1631))
  - The Last Codex: 2026-11-02 ([#1950](https://github.com/openzim/zim-requests/issues/1950))
  - USAID DEC: 2026-11-23 ([#1897](https://github.com/openzim/zim-requests/issues/1897))
  - Learn AI Anywhere: 2026-11-16 ([#1969](https://github.com/openzim/zim-requests/issues/1969))
  - Gutenberg top: 2026-11-12 ([#1985](https://github.com/openzim/zim-requests/issues/1985))
  - WEG: 2026-11-24 ([#2001](https://github.com/openzim/zim-requests/issues/2001))
  - Zimi Almanac: 2026-11-14 ([#1988](https://github.com/openzim/zim-requests/issues/1988))
- OSM maps ZIMs were "released to production" on 2026-06-23 ([#1309](https://github.com/openzim/zim-requests/issues/1309)). Regional maps, including Nordics, followed on 2026-08-24 ([#1891](https://github.com/openzim/zim-requests/issues/1891)).
- Kiwix decided to "Move to quarterly updates" on 2025-11-17 ([#1586](https://github.com/openzim/zim-requests/issues/1586)). It also chose to "Split gutenberg recipe and stop creating mul version" on 2025-10-28 ([#1421](https://github.com/openzim/zim-requests/issues/1421)) and to "Create smaller selections for Gutenberg English ZIMs" on 2025-12-15 ([#1576](https://github.com/openzim/zim-requests/issues/1576)).
- Removals and deprecations:
  - Wikinews recipes: 2026-06-23 ([#1707](https://github.com/openzim/zim-requests/issues/1707))
  - hitchwiki: 2026-09-28 ([#1934](https://github.com/openzim/zim-requests/issues/1934)); a copy now lives in `community/`
  - GCFGlobal: archived 2026-04-22 ([#1724](https://github.com/openzim/zim-requests/issues/1724))
  - Wikipedia 0.8 selection: 2026-09-17 ([#1887](https://github.com/openzim/zim-requests/issues/1887))

### Inferences
- Under quarterly cadence, most series refresh about every 3 months. Many `_2026-09`/`_2026-10` files are therefore current until about 2027-01, so a stale-looking date is not necessarily broken.
- `community/` files are reviewed but labelled community, and some use non-commercial licences (Last Codex is NC-ND, Learn AI Anywhere is NC-SA). Recipe licence metadata should record this.

### Gaps
- I did not read the Kiwix blog or the zimfarm recipe list directly. The OPDS feed, directory listings and zim-requests together covered additions, but announcements of planned collections may be missing.

## Finnish-language ZIMs

### Takeaway
The catalog already covers most Finnish ZIMs. The remaining gaps are small: Wikiversity fi, Wikiquote fi, One More Library fi, and the `mini` flavours of Finnish Wikipedia. The Finnish Gutenberg has a newer build in `books/`, and there is no Finland-only maps ZIM.

### Cited Findings
- Finnish files that exist but are not in the catalog:
  - `wikiversity/wikiversity_fi_all_maxi_2026-10.zim`: 20M (nopic 11M)
  - `wikiquote/wikiquote_fi_all_maxi_2026-10.zim`: 19M (nopic 8.7M)
  - `other/onemorelibrary.com_fi_all_2026-11.zim`: 27M
  - `wikipedia/wikipedia_fi_all_mini_2026-09.zim`: 1.0G
  - `wikipedia/wikipedia_fi_top_mini_2026-10.zim`: 112M
  - Source: [wikiversity](https://download.kiwix.org/zim/wikiversity/), [wikiquote](https://download.kiwix.org/zim/wikiquote/), [other](https://download.kiwix.org/zim/other/), [wikipedia](https://download.kiwix.org/zim/wikipedia/)
- Current builds of Finnish titles already in the catalog:
  - `wikipedia_fi_all_maxi_2026-09` (7.4G), `_nopic_2026-09` (2.6G), `wikipedia_fi_top_maxi_2026-10` (1.4G), `_nopic` (461M)
  - `wiktionary_fi_all_nopic_2026-10` (502M)
  - `wikibooks_fi_all_maxi_2026-10` (63M; nopic 21M)
  - `wikisource_fi_all_nopic_2026-10` (53M)
  - `wikivoyage_fi_all_maxi_2026-09` (20M)
  - `phet_fi_all_2026-11` (80M)
  - `scoutwiki.org_fi_all_maxi_2026-11` (23M)
  - Source: same listings
- Finnish Gutenberg: `books/gutenberg_fi_all_2026-11a.zim` (1.4G, 2026-11-16), compared with `gutenberg/gutenberg_fi_all_2026-01.zim` (1.5G) ([books](https://download.kiwix.org/zim/books/)).
- No Finnish `ifixit`, `vikidia`, `freecodecamp`, `devdocs`, `stack_exchange` or `videos` ZIMs exist. The only Finnish matches in those folders were the ones listed above.
- `maps_en_nordics_2026-08a.zim` (5.3G) is the only ZIM covering Finland's map ([maps](https://download.kiwix.org/zim/maps/)).
- `wiktionary_fi` gained the "Liite" (appendix) namespace in 2026-05 ([#1774](https://github.com/openzim/zim-requests/issues/1774)).

### Inferences
- For fi-reference, the cheap wins are `wikipedia_fi_top_mini` (112M) as a phone-sized tier, `wikiversity_fi` and `wikiquote_fi`. They add about 150 MB in total.
- `onemorelibrary.com_fi` (27M) probably overlaps with Gutenberg fi. It is low priority.

### Gaps
- I did not check whether the 27M One More Library Finnish collection overlaps with `gutenberg_fi`.

## High value-per-GB items by theme

### Takeaway
For medicine, repair, radio, agriculture and water, the new value is in TED topic packs, The Last Codex, BeReady UK, the Wikipedia knots subset, more Canadian Prepper videos and Army publications. The requested Stack Exchange sites (diy, electronics, ham, outdoors, gardening, cooking, security, unix) are **already in the catalog**. No WHO, MSF or Hesperian ZIMs exist on Kiwix.

### Cited Findings
- **survival-medical:**
  - `ted_mul_water_2026-09` (832M), `ted_mul_natural-disaster_2026-09` (683M), `ted_mul_medicine_2026-09` (3.4G), `ted_mul_public-health_2026-09` (2.2G) ([ted](https://download.kiwix.org/zim/ted/))
  - `wikipedia_en_knots_maxi_2026-10` (13M) ([wikipedia](https://download.kiwix.org/zim/wikipedia/))
  - `beready.campaign.gov.uk_en_all_2026-11` (158M) and `armypubs_en_all_2024-12` (7.7G) ([zimit](https://download.kiwix.org/zim/zimit/))
  - Canadian Prepper bugoutconcepts, bugoutroll and preppingfood (2.7G, 838M, 2.0G) ([videos](https://download.kiwix.org/zim/videos/))
  - `books/gutenberg_en_lcc-u_2026-11a` (864M, Military Science) ([books](https://download.kiwix.org/zim/books/))
  - `skin-of-color-society_en_all_2026-11` (1.41 GB) ([OPDS](https://library.kiwix.org/catalog/v2/entries?count=-1))
- **homesteading:**
  - `community/lastcodex.org_en_all_2026-10` (264M) ([community](https://download.kiwix.org/zim/community/))
  - `ted_mul_gardening` (311M), `plants` (633M), `agriculture` (1.2G), `food` (2.2G), `weather` (368M) ([ted](https://download.kiwix.org/zim/ted/))
  - `foss.cooking` (29M) and `publicdomainrecipes.com` (16M) ([zimit](https://download.kiwix.org/zim/zimit/))
  - `books/usaid-archive.aiddata.org_en_all_2026-11a` (8.0G) ([books](https://download.kiwix.org/zim/books/))
  - Stack Exchange pets (83M) and lifehacks (47M) ([stack_exchange](https://download.kiwix.org/zim/stack_exchange/))
- **engineering:**
  - `ted_mul_solar-energy` (515M), `renewable-energy` (1.0G), `wind-energy` (166M), `nuclear-energy` (177M) ([ted](https://download.kiwix.org/zim/ted/))
  - `books/gutenberg_en_lcc-v_2026-11a` (774M) ([books](https://download.kiwix.org/zim/books/))
- **sciences:**
  - `phet_en_all_2026-11` (110M) ([phet](https://download.kiwix.org/zim/phet/))
  - Wikipedia chemistry, physics, mathematics and molcell subsets ([wikipedia](https://download.kiwix.org/zim/wikipedia/))
  - `books/gutenberg_en_lcc-q_2026-11a` (12G) ([books](https://download.kiwix.org/zim/books/))
  - `community/zimi_mul_almanac_mini_2026-11` (11M) ([community](https://download.kiwix.org/zim/community/))
  - Stack Exchange astronomy (188M), space (381M), stats (1.5G) ([stack_exchange](https://download.kiwix.org/zim/stack_exchange/))
- **computing:**
  - The DevDocs bundle described above ([devdocs](https://download.kiwix.org/zim/devdocs/))
  - `developer.mozilla.org_en_all_2026-11` (560M), `lua.org` (362M), `gobyexample` (323K), `opendatastructures` (111M), `jeffe` (493M), `noted.lol` (2.1G) ([zimit](https://download.kiwix.org/zim/zimit/))
  - `postmarketos_en_all_maxi_2026-08` (29M) ([other](https://download.kiwix.org/zim/other/))
  - `freecodecamp_en_all_2026-11` (11M) ([freecodecamp](https://download.kiwix.org/zim/freecodecamp/))
  - Corey MS videos ([videos](https://download.kiwix.org/zim/videos/))
  - Stack Exchange android, apple, retrocomputing, vi, emacs, hardwarerecs, softwarerecs ([stack_exchange](https://download.kiwix.org/zim/stack_exchange/))
- **cybersec:**
  - `ted_mul_cyber-security` (195M) and `online-privacy` (492M) ([ted](https://download.kiwix.org/zim/ted/))
  - `cloudflare.com_en_learning-center_2025-12` (182M) and `incognitocat.me_en_all_2025-09` (8.9M) ([zimit](https://download.kiwix.org/zim/zimit/))
  - `tor.stackexchange.com_en_all_2026-10` (30M) ([stack_exchange](https://download.kiwix.org/zim/stack_exchange/))
- **communications:** no new radio ZIMs beyond those already in the catalog (ham and dsp Stack Exchange, S2 Underground). `unrealircd_en_all_maxi_2026-11` is 1.4M of IRC server docs ([zimit](https://download.kiwix.org/zim/zimit/)).
- **ai:**
  - `community/learnaianywhere_en_all_2026-11` (96M) ([community](https://download.kiwix.org/zim/community/))
  - `ted_mul_machine-learning_2026-09` (1.46 GB per [OPDS](https://library.kiwix.org/catalog/v2/entries?count=-1))
  - Stack Exchange ai (100M), datascience (260M), genai (11M)
- **core:**
  - `theworldfactbook_en_all_2026-02` (410M) ([other](https://download.kiwix.org/zim/other/))
  - `wikivoyage_en_all_nopic_2026-09` (260M) ([wikivoyage](https://download.kiwix.org/zim/wikivoyage/))
  - `wikipedia_en_top_mini_2026-09` (283M) and `wikipedia_en_top1m_mini_2026-11` (4.9G) ([wikipedia](https://download.kiwix.org/zim/wikipedia/))
  - `community/gutenberg_en_top_2026-11` (6.8G) ([community](https://download.kiwix.org/zim/community/))

### Inferences
- A ranked shortlist ("must add") of about 1.9 GB:
  1. lastcodex en
  2. wikipedia_en_knots
  3. beready UK
  4. TED water and natural-disaster
  5. TED gardening
  6. TED solar-energy
  7. TED cyber-security
  8. DevDocs bundle
  9. postmarketOS
  10. Learn AI Anywhere en
  11. PhET en
  12. wikivoyage en nopic
  13. wikipedia_fi_top_mini
  14. wikiversity_fi
  15. Fix the ready.gov URL pattern
  16. Fix the usda-2015 description
- No candidate WikEM replacement exists among Kiwix ZIMs. The emergency-medicine gap needs a non-Kiwix source.

### Gaps
- Hesperian, WHO and MSF field guides are not on Kiwix in any form I could find.
- No new electronics or radio reference ZIM (for example All About Circuits or ARRL) exists in the OPDS feed or the listings.
