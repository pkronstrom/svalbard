# Similar offline-knowledge projects: what they bundle, and what Svalbard's catalog lacks

Scope: projects that build curated offline knowledge/tool collections, checked on 2026-11-26. The Svalbard catalog (227 recipes, read in full from the scratchpad `catalog-index.txt`) was the baseline. The gap list below shows only items that no existing recipe covers.

Method notes: GitHub activity was read with `gh api`. Project NOMAD's content manifests (`collections/*.json`) and IIAB's module catalog were downloaded and parsed. The full Kiwix OPDS catalog for English (`library.kiwix.org/catalog/v2/entries?lang=eng`, 1,315 entries, 583 distinct book names) was downloaded on 2026-11-26 and used to confirm that each ZIM gap exists, plus its size and date. The Kolibri Studio public channel API (175 public channels) supplied the Kolibri sizes. ZIM sizes are the Kiwix catalog `length` in MiB.

## Q1. Which similar projects exist, what does each bundle, how does it package and update content, and what could Svalbard borrow?

### Takeaway
Project N.O.M.A.D. (Crosstalk Solutions) is now the dominant project of its kind: 44k GitHub stars, pushed 2026-11-24. It is the nearest analogue to Svalbard: Kiwix + Kolibri + local LLM + PMTiles maps + CyberChef, with JSON-manifest content tiers. IIAB is still active but leans on the Kiwix catalog plus a small rsync module catalog, and the old RACHEL/OER2Go catalog API is dead. Most "doomsday drive" write-ups from 2026 are built on the same Kiwix ZIMs Svalbard already uses. The distinctive extras are a small set of medical PDFs and government datasets (openFDA, DailyMed, NIOSH, WHO, MSF, Hesperian).

### Cited Findings

**Project N.O.M.A.D. (Crosstalk Solutions), active**
- Apache-2.0, 44,424 stars, last push 2026-11-24 — [GitHub API: Crosstalk-Solutions/project-nomad](https://github.com/Crosstalk-Solutions/project-nomad)
- Runs as Docker containers on Debian/Ubuntu under a web admin UI. It needs internet only for install and for later content downloads, and has no authentication — [README via search summary](https://github.com/Crosstalk-Solutions/project-nomad); [SourcePulse](https://www.sourcepulse.org/projects/26188020)
- Bundles: Kiwix (Wikipedia etc.), Kolibri (Khan Academy), Ollama + Qdrant RAG chat, ProtoMaps offline maps, CyberChef — [DEV Community overview](https://dev.to/arshkharbanda2010/project-nomad-the-open-source-offline-survival-computer-that-runs-without-the-internet-20c7)
- Content is declared in `collections/kiwix-categories.json` as categories (Medicine, Survival & Preparedness, Education, DIY & Repair, Agriculture & Food, Computing). Each category has **Essential / Standard / Comprehensive** tiers, and a higher tier includes the lower ones via `includesTier` — [collections/kiwix-categories.json](https://github.com/Crosstalk-Solutions/project-nomad/blob/main/collections/kiwix-categories.json); [CATEGORIES-TODO.md](https://github.com/Crosstalk-Solutions/project-nomad/blob/main/collections/CATEGORIES-TODO.md). The TODO says "Essential tiers should be <500MB ideally" and lists planned Children, Trades, Agriculture (Practical Plants, seed saving, animal husbandry), Legal & Civic, Communications and Religion categories.
- Updates: the repo has a `validate-collection-urls.yml` CI workflow, `collection_update_service.ts`, and a DB migration that adds `content_auto_update` fields to installed resources. Content URLs are pinned to dated ZIMs and refreshed by the maintainers — [repo tree](https://github.com/Crosstalk-Solutions/project-nomad/tree/main/.github/workflows)
- Structured non-ZIM data:
  - `conditions.json` maps symptoms to search terms.
  - `home_remedies.json` is drawn from "US government health guidance (CDC, NIH/NHLBI, MedlinePlus/NLM, FDA)", public domain.
  - `natural_remedies.json` comes from NCCIH "Herbs at a Glance", public domain.
  - An "FDA Drug Reference" (1,700 MB) is pulled from `api.fda.gov/download.json`.
  - Source: [collections/](https://github.com/Crosstalk-Solutions/project-nomad/tree/main/collections)
- "Creator Packs" are YouTube creators' videos (Crosstalk Solutions 30 videos/2.9 GB, The Modern Rogue 37/5.2 GB) under a custom, still-draft license ("DRAFT — pending legal review"). Redistribution is allowed only through the official NOMAD channel, so the packs are not reusable — [creator-pack-license.md](https://github.com/Crosstalk-Solutions/project-nomad/blob/main/collections/creator-pack-license.md)
- Maps: per-US-state topographic PMTiles (e.g. California 1,100 MB, Alaska 684 MB, 2025-12) hosted in a GitHub repo — [collections/maps.json](https://github.com/Crosstalk-Solutions/project-nomad/blob/main/collections/maps.json)
- Wikipedia is offered as a single choice: none / top-mini (331 MB) / top-nopic / … — [collections/wikipedia.json](https://github.com/Crosstalk-Solutions/project-nomad/blob/main/collections/wikipedia.json)
- **Borrow:** (a) the Essential/Standard/Comprehensive tiers inside each pack, with a size budget for the Essential tier; (b) a CI job that HEAD-checks every recipe URL; (c) a small offline symptom → remedy/drug lookup built from public-domain CDC/NCCIH/openFDA data, which could feed Svalbard's search/AI.

**Internet-in-a-Box (IIAB), active**
- `iiab/iiab` last push 2026-11-20 (2,060 stars); `iiab/iiab-admin-console` last push 2026-11-22 — [GitHub API: iiab/iiab](https://github.com/iiab/iiab)
- The admin console merges three catalogs, per its `default_vars.yml`:
  - the Kiwix ZIM catalog;
  - `https://iiab-share.github.io/iiab-content/iiab-module-cat.json`;
  - the OER2Go (RACHEL) catalog at `http://dev.worldpossible.org/cgi/json_api_v1.pl` or `http://oer2go.org/cgi/json_api_v2.pl`.
  - Each item gets a "js-menu" menu definition on the home page.
  - Source: [iiab-admin-console vars/default_vars.yml](https://github.com/iiab/iiab-admin-console/blob/master/vars/default_vars.yml)
- The script comment says "As of July, 2022 the oer2go catalog v1 url does not exist". On 2026-11-26 both OER2Go endpoints returned 404 — [get_oer2go_catalog3.py](https://github.com/iiab/iiab-admin-console/blob/master/roles/cmdsrv/templates/scripts/get_oer2go_catalog3.py)
- The IIAB module catalog has only 35 modules, all pulled over `rsync://medbox.iiab.me/modules/<id>`:
  - en-cdc (≈13 GB)
  - en-wikem
  - OSM OpenMapTiles world z0–10 (`en-osm-omt-min`, ≈1.9 GB) and Central America (≈3.6 GB)
  - HealthPhone videos (24 languages)
  - Stanford Emergency Medicine videos (≈1.0 GB)
  - GCF LearnFree 2021 (≈6.6 GB)
  - w3schools (≈1.5 GB)
  - Solar Power Education (wilearn.ch, ≈2.3 GB)
  - iLearnabout, Computer Videos
  - Scale of the Universe
  - Source: [iiab-module-cat.json](https://iiab-share.github.io/iiab-content/iiab-module-cat.json)
- The IIAB menu definitions also list Hesperian Health, Practical Action, Infonet (biovision farming), Law Library, OpenStax, CK-12, Saylor, Wikispecies, Wikiquote, Wikivoyage EN, Simple English Wikipedia/Wiktionary and others — [iiab-admin-console js-menu menu-defs](https://github.com/iiab/iiab-admin-console/tree/master/roles/js-menu/files/menu-files/menu-defs)
- **Borrow:** rsync-style incremental module updates for large non-ZIM folders, which "only downloads changes" per World Possible — [RACHEL Appropedia module page](https://rachel.worldpossible.org/viewmod/en-appropedia)

**WikiProjectMed "Medical IIAB"**
- Lists the medical content it bundles, each with a license:
  - Wikipedia medicine ZIMs in 7 languages
  - Osmosis videos (3.5 GB)
  - WikEM (1.4 GB)
  - OSM to L8 (1 GB)
  - Practical Action pack (1.1 GB, CC BY-SA, which "may not cover pictures")
  - NIH and CDC content (public domain)
  - Hesperian Health Guides (0.8 GB, CC NC)
  - WHO content ("NC ND equivalent mostly")
  - Global Health Media videos (NC ND)
  - The page notes that a memorandum of understanding is needed for the NC/ND items.
  - Source: [WikiProjectMed: Internet-in-a-Box/Content](https://mdwiki.org/wiki/WikiProjectMed:Internet-in-a-Box/Content)

**RACHEL / World Possible**
- Module pages are still served at `rachel.worldpossible.org/viewmod/<module>`, with downloads as a zip over the web, FTP (`ftp.worldpossible.org`, anonymous) or rsync — [MedlinePlus static module](https://rachel.worldpossible.org/viewmod/en-medline_plus-static)
- No complete public module list was found. `worldpossible.org/rachel-modules` and `dev.worldpossible.org/mods/` both return 404 (checked 2026-11-26).

**Kolibri / Learning Equality**
- The Studio public API lists 175 public channels. Relevant English channels, with published sizes:
  - CK-12 6.66 GB (10,643 resources)
  - Open Stax 2.08 GB
  - PhET 1.11 GB
  - TED-Ed Lessons 6.58 GB
  - Sikana (English) 15.77 GB
  - Osmosis.org 5.17 GB
  - MIT Blossoms 26.02 GB
  - Goalkicker Tech Books 0.49 GB
  - HP LIFE
  - Blockly Games
  - Source: [Kolibri Studio public channels API](https://studio.learningequality.org/api/public/v1/channels?format=json)
- Channels need the Kolibri server (Python/Django) to play back. NOMAD runs it as a container — [DEV Community](https://dev.to/arshkharbanda2010/project-nomad-the-open-source-offline-survival-computer-that-runs-without-the-internet-20c7)

**Kiwix Hotspot → Kiwix Imager / Offspot**
- `kiwix/kiwix-hotspot` is archived (last push 2024-06-04). `offspot/offspot-config` is active (push 2026-10-16), and the image builder is served at imager.kiwix.org (HTTP 200) — [kiwix-hotspot](https://github.com/kiwix/kiwix-hotspot); [offspot-config](https://github.com/offspot/offspot-config); [imager.kiwix.org](https://imager.kiwix.org/)

**Prepper Disk (commercial)**
- A Raspberry Pi 4B hotspot. The "Premium" 512 GB unit sells for $279. Sellers say the content is "licensed, open source, or public domain", including children's books, classics, and "exclusive eBooks" on livestock, woods survival and emergency comms — [Alpenglow Supply listing](https://alpenglowsupplyak.com/products/prepper-disk-premium-loaded-with-512gb-of-survival-content); [Entropy Survival guide](https://entropysurvival.com/blogs/news-views/prepper-disk-guide)
- No public table of contents exists. A buyer on r/preppers asked for one — [r/preppers thread (mirror)](https://lr.us.psf.lt/r/preppers/comments/1joqart/offline_library_prepper_disk/mm7pui7/?context=3)

**Offline Survival Manual (ligi), low activity**
- Last push 2025-09-03, last release 5.0.0 (2024-10-26), GPL-3.0 (per the GitHub license field), 1,334 stars, on F-Droid. Its README says it is "based on the public domain US Army Survival Field Manual FM 3-05.70 (FM 21-76)", and the content is curated in the repo's GitHub wiki (a cloneable git repo) — [ligi/SurvivalManual README](https://github.com/ligi/SurvivalManual); [wiki](https://github.com/ligi/SurvivalManual/wiki)
- Wiki chapters: Psychology, Power, Kits, Basic Medicine, Shelter, Water Procurement, Firecraft, Food Procurement, Survival Use of Plants, Dangerous Animals, Car Repair, Basic Mechanical Skills, Desert/Tropical/Cold/Sea, Direction Finding, Signaling, Man-Made Hazards, Ropes and Knots, Poisonous Plants and others — [wiki](https://github.com/ligi/SurvivalManual/wiki)

**Personal "doomsday archive" builds, 2026**
- "Doomsday Ready: A 170 GB Offline Knowledge Archive" (May 2026) contains:
  - Wikipedia maxi ZIMs, raw XML dumps and HF Wikipedia parquet
  - 122 OpenStax PDFs (11 GB)
  - a 622 MB medical folder: Hesperian WTND/WTNDentist, Army FMs, WHO IRIS clinical handbooks, WHO/ICRC BEC, IFRC/ICRC first aid, MSF clinical guides, ICRC War Surgery Vol. 1, DailyMed/FDA Orange Book, WHO International Medical Guide for Ships, NIOSH Pocket Guide
  - Kiwix Desktop + DuckDB
  - Updates come from `health.sh` (`wget -c`, run yearly) and `health_fixup.py` (WHO IRIS DSpace 7 API). The author suggests refreshing drug labels yearly, "since drug information goes stale fastest".
  - Source: [emresokullu.com](https://emresokullu.com/myblog/2026/05/06/doomsday-ready-170gb-offline-knowledge-archive.html)
- "What to Download Before the Internet Fails" (checked against the Kiwix mirror in Oct 2026) recommends only Kiwix ZIMs Svalbard already has, plus a macOS port of NOMAD (`aaronbailey/project-nomad-macos`) — [Living Method](https://livingmethod.com/nomad-offline-content-packs-2026/)
- A newer project, "AI-Doomsday-Toolbox", appeared in search but was not examined — [GitHub](https://github.com/ManuXD32/AI-Doomsday-Toolbox)

**Already covered by Svalbard (no gap):** Survivor Library, CD3WD, Global Village Construction Set / Open Source Ecology, Appropedia, Energypedia, Wikiciv, zimgit-* libraries, Low-tech Magazine, 100 Rabbits.

### Inferences
- Svalbard's catalog already covers NOMAD's Medicine, Survival and DIY tiers almost completely. NOMAD's real differences are packaging: tiers, a URL-validation CI job, and auto-update flags. Its structured public-domain health data (symptom→remedy, openFDA) is a capability Svalbard lacks.
- The RACHEL/OER2Go ecosystem has decayed (dead catalog APIs). Mining its modules now means going through IIAB's 35-module rsync catalog or individual `viewmod` pages, so the cost of adopting them is high.
- Kolibri channels need a Python/Django server. Svalbard's Go drive runtime would be better served by ZIM or PDF equivalents (OpenStax PDFs, PhET ZIM, TED-Ed ZIM) than by Kolibri itself.
- Kiwix's own hotspot product has turned into a hosted image builder (Imager/Offspot). That supports Svalbard's "portable stick + local runtime" niche as different from the Pi-hotspot products (Prepper Disk, NOMAD, IIAB).

### Gaps
- Not examined, for lack of tool-call budget: the Long Now "Manual for Civilization" booklist (mostly in-copyright books, so likely of low direct value), the GitHub Arctic Code Vault contents, Endless OS / Endless Key offline packs, LibraryBox/PirateBox successors, Nomad Network / Reticulum page collections, and Reddit r/DataHoarder, r/selfhosted and HN threads. Searches returned blogs rather than the threads themselves.
- solarspell.org returned 403 to automated fetches, so SolarSPELL library contents were not verified.
- Prepper Disk contents are unverifiable because no public table of contents exists.

## Q2. Which concrete resources from these projects does Svalbard lack?

### Takeaway
About 60 concrete, legally redistributable items are missing. The highest-value ones:
- **US Army publications ZIM** (7.8 GB)
- **openFDA drug labels** (≈1.8 GB zipped JSON, public domain)
- **Simple English Wikipedia** and **English Wikivoyage**
- **PhET English** (Svalbard has only Finnish)
- the **Gutenberg Military Science** ZIM
- **UK "Be ready"** ZIM
- the **remaining Canadian Prepper** ZIMs
- **TED natural-disaster/weather/plants/solar** ZIMs
- **MDN / docs.python.org / extra DevDocs** ZIMs
- a **world low-zoom OSM basemap**

Each Kiwix item below was confirmed present in the Kiwix OPDS catalog on 2026-11-26.

### Cited Findings

Unless another source is given, size and date come from the Kiwix OPDS catalog ([library.kiwix.org catalog](https://library.kiwix.org/catalog/v2/entries?lang=eng)). The URL shown is the Kiwix download path; `{date}` follows Svalbard's convention.

#### Survival, preparedness, military (packs: core, survival-medical)

| Resource | Source URL | Format | Size | License | Seen in |
|---|---|---|---|---|---|
| US Army Publishing Directorate (field manuals, ATPs) | download.kiwix.org/zim/zimit/armypubs_en_all_{date}.zim (2024-12) | ZIM (zimit) | 7,851 MiB | US government work (public domain); verify distribution statements | Kiwix catalog; Army FMs in the 170 GB archive ([blog](https://emresokullu.com/myblog/2026/05/06/doomsday-ready-170gb-offline-knowledge-archive.html)) |
| Be ready UK (UK gov preparedness) | download.kiwix.org/zim/zimit/beready.campaign.gov.uk_en_all_{date}.zim (2026-11) | ZIM | 157 MiB | Probably Open Government Licence (unverified) | Kiwix catalog |
| Canadian Prepper: Bug Out Roll | download.kiwix.org/zim/videos/canadian-prepper_en_bugoutroll_{date}.zim (2026-10) | ZIM (video) | 838 MiB | Creator content redistributed by Kiwix | NOMAD survival-essential ([manifest](https://github.com/Crosstalk-Solutions/project-nomad/blob/main/collections/kiwix-categories.json)) |
| Canadian Prepper: Bug Out Concepts | …/videos/canadian-prepper_en_bugoutconcepts_{date}.zim (2026-11) | ZIM (video) | 2,752 MiB | same | NOMAD survival-standard |
| Canadian Prepper: Prepping Food | …/videos/canadian-prepper_en_preppingfood_{date}.zim (2026-10) | ZIM (video) | 2,092 MiB | same | NOMAD survival-comprehensive |
| Project Gutenberg: Military Science (LCC U) | download.kiwix.org/zim/books/gutenberg_en_lcc-u_{date}.zim (2026-11a) | ZIM | 864 MiB | Public domain | NOMAD survival-comprehensive |
| CIA World Factbook (final, discontinued) | download.kiwix.org/zim/other/theworldfactbook_en_all_{date}.zim (2026-02) | ZIM | 388 MiB | US gov, public domain | Kiwix catalog |
| Offline Survival Manual (FM 21-76 based) | github.com/ligi/SurvivalManual/wiki (git clone of `SurvivalManual.wiki.git`); original FM 3-05.70 PDF at fas.org/irp/doddir/army/fm3-05-70.pdf | Markdown wiki / PDF | small (MB scale; not measured) | GPL-3.0 (app); FM is public domain | [ligi README](https://github.com/ligi/SurvivalManual) |
| TED: natural disaster / weather / plants / solar energy | download.kiwix.org/zim/ted/ted_mul_{natural-disaster,weather,plants,solar-energy}_{date}.zim (2026-09) | ZIM (video) | 682 / 367 / 632 / 515 MiB | TED: CC BY-NC-ND | Kiwix catalog |
| TED: engineering / chemistry / mental health | ted_mul_{engineering,chemistry,mental-health} (2026-09) | ZIM (video) | 2,781 / 613 / 2,297 MiB | CC BY-NC-ND | Kiwix catalog |
| Lifehacks SE, Pets SE, Aviation SE | download.kiwix.org/zim/stack_exchange/{lifehacks,pets,aviation}.stackexchange.com_en_all_{date}.zim (2026-10) | ZIM | 46 / 82 / 486 MiB | CC BY-SA | Kiwix catalog |

#### Medical and health (pack: survival-medical)

| Resource | Source URL | Format | Size | License | Seen in |
|---|---|---|---|---|---|
| openFDA drug labeling (SPL) bulk | https://api.fda.gov/download.json → `results.drug.label` partitions | zipped JSON (14 partitions) | 1,789.5 MB zipped; 265,067 records; export 2026-11-26 (measured) | openFDA data: public domain / CC0 | NOMAD "FDA Drug Reference" (1,700 MB) |
| DailyMed all drug labels (SPL) | https://dailymed.nlm.nih.gov/dailymed/spl-resources-all-drug-labels.cfm (HTTP 200) | zipped SPL XML | multi-GB (not measured) | NLM, public domain | 170 GB archive ([blog](https://emresokullu.com/myblog/2026/05/06/doomsday-ready-170gb-offline-knowledge-archive.html)) |
| NCCIH "Herbs at a Glance" | https://www.nccih.nih.gov/health/herbsataglance (HTTP 200) | HTML (zimit-able) | small | US gov, public domain | NOMAD `natural_remedies.json` |
| CDC/NIH home-remedy + symptom JSON | github.com/Crosstalk-Solutions/project-nomad/tree/main/collections (`conditions.json`, `home_remedies.json`, `natural_remedies.json`) | JSON | <100 KB each | Source content public domain; repo Apache-2.0 | NOMAD |
| Skin of Color Society (dermatology across skin tones) | download.kiwix.org/zim/other/skin-of-color-society_en_all_{date}.zim (2026-11) | ZIM | 1,347 MiB | not stated in catalog (verify) | Kiwix catalog |
| Hesperian: Where There Is No Doctor (2025) + WT No Dentist, Health Handbook for Women | hesperian.org/wp-content/uploads/pdf/en_wtnd_2025/ (per-chapter PDFs, e.g. `en_wtnd_2025_fm.pdf`) | PDF | ≈0.8 GB for the full Hesperian set (WikiProjectMed figure) | "Open Copyright License": copying encouraged, but permission asked for some redistribution; NC | [Hesperian 2025 FM PDF](https://hesperian.org/wp-content/uploads/pdf/en_wtnd_2025/en_wtnd_2025_fm.pdf); [WikiProjectMed](https://mdwiki.org/wiki/WikiProjectMed:Internet-in-a-Box/Content) |
| MSF Medical Guidelines (clinical, drugs, obstetrics) | https://medicalguidelines.msf.org/en (HTTP 200) | HTML/PDF | not measured | not verified | 170 GB archive |
| WHO International Medical Guide for Ships | https://iris.who.int/handle/10665/43814 (HTTP 200) | PDF | not measured | WHO (typically CC BY-NC-SA 3.0 IGO; verify) | 170 GB archive |
| ICRC War Surgery Vol. 1; IFRC First Aid Guidelines | icrc.org / ifrc.org (IFRC returned 403 to automated fetch) | PDF | not measured | free download; license not verified | 170 GB archive |
| NIOSH Pocket Guide to Chemical Hazards | https://www.cdc.gov/niosh/docs/2005-149/pdfs/2005-149.pdf (403 to curl; CDC bot-blocking) | PDF | not measured | US gov, public domain | 170 GB archive |
| Stanford Emergency Medicine videos | rsync://medbox.iiab.me/modules/en-stanford_em | HTML + video folder | ≈1.0 GB (ksize 1,050,696) | not stated (verify) | [IIAB module catalog](https://iiab-share.github.io/iiab-content/iiab-module-cat.json) |
| HealthPhone health and nutrition videos (EN + 23 Indian languages) | rsync://medbox.iiab.me/modules/en-healthphone | video folder | EN ≈0.47 GB | not stated (verify) | IIAB module catalog |
| Practical Action technical briefs pack | answers.practicalaction.org/our-resources | PDF pack | ≈1.1 GB | CC BY-SA (may exclude pictures) | [WikiProjectMed](https://mdwiki.org/wiki/WikiProjectMed:Internet-in-a-Box/Content) |

#### Education and general reference (packs: core, sciences)

| Resource | Source URL | Format | Size (MiB) | License | Seen in |
|---|---|---|---|---|---|
| Simple English Wikipedia (maxi / nopic / mini) | download.kiwix.org/zim/wikipedia/wikipedia_en-simple_all_maxi_{date}.zim (2026-09) | ZIM | 2,976 / 1,044 / 505 | CC BY-SA | IIAB menus; Kiwix |
| Wiktionary Simple English | …/wiktionary/wiktionary_en-simple_all_{date}.zim | ZIM | 29 | CC BY-SA | IIAB menus |
| English Wikivoyage (maxi / nopic; Europe subset 367/79) | …/wikivoyage/wikivoyage_en_all_maxi_{date}.zim (2026-09) | ZIM | 1,021 / 259 | CC BY-SA | IIAB menus |
| English Wikisource (maxi / nopic) | …/wikisource/wikisource_en_all_maxi_{date}.zim (2026-09) | ZIM | 8,216 / 3,724 | PD / CC BY-SA | IIAB menus |
| Wikiversity EN (nopic / maxi) | …/wikiversity/wikiversity_en_all_nopic_{date}.zim (2026-11) | ZIM | 1,540 / 2,012 | CC BY-SA | NOMAD education-standard |
| Wikispecies (maxi / nopic) | …/other/wikispecies_en_all_maxi_{date}.zim (2026-10) | ZIM | 3,382 / 1,592 | CC BY-SA | IIAB menus |
| Wikiquote EN | …/wikiquote/wikiquote_en_all_{date}.zim | ZIM | 952 / 382 | CC BY-SA | IIAB menus |
| Wikibooks EN with images (maxi) | …/wikibooks/wikibooks_en_all_maxi_{date}.zim | ZIM | 4,520 (NOMAD figure) | CC BY-SA | NOMAD (Svalbard has nopic only) |
| PhET simulations, English | download.kiwix.org/zim/phet/phet_en_all_{date}.zim (2026-11) | ZIM | 109 | CC BY | IIAB, Kolibri (Svalbard has phet_fi only) |
| TED-Ed | …/ted/ted_mul_ted-ed_{date}.zim (2026-09) | ZIM (video) | 6,085 | CC BY-NC-ND | NOMAD education-standard |
| CrashCourse | …/crashcourse_en_all_{date}.zim (2026-05) | ZIM (video) | 21,967 | CC BY-NC-SA (per Crash Course) | IIAB menus (`en-crashcourse_en_all`) |
| LibreTexts Humanities / Business | …/libretexts/libretexts.org_en_{human,biz}_{date}.zim | ZIM | 3,730 / 801 (NOMAD figures) | CC BY-NC-SA mostly | NOMAD education-comprehensive |
| OpenStax textbooks | openstax.org/subjects (122 PDFs, 11 GB in the blog archive) or Kolibri "Open Stax" channel (2.08 GB) | PDF | ≈11 GB PDFs | CC BY 4.0 | 170 GB archive; Kolibri; IIAB |
| CK-12 FlexBooks | Kolibri channel `1d8f6d84618153c18c695d85074952a7` | Kolibri channel | 6.66 GB | CC BY-NC | Kolibri; IIAB |
| One More Library (public-domain books) | …/zimit/onemorelibrary.com_en_all_{date}.zim (2026-11) | ZIM | 3,835 (mul: 6,578) | PD books | Kiwix |
| Milne Open Textbooks (SUNY) | …/zimit/milneopentextbooks.org_en_all_{date}.zim (2026-11) | ZIM | 1,554 | CC BY / BY-NC (per book) | Kiwix |
| Internet Encyclopedia of Philosophy | …/zimit/internet-encyclopedia-philosophy_en_all_{date}.zim | ZIM | 117 | site terms (verify) | Kiwix |
| Explorable explanations (Nicky Case) | …/zimit/ncase.me_en_all_{date}.zim | ZIM | 1,798 | CC0 / public domain (verify) | Kiwix |
| Vikidia EN (children's encyclopedia) | …/vikidia_en_all_{date}.zim | ZIM | 10 | CC BY-SA | Kiwix; NOMAD TODO "Children" |
| Book Dash picture books | …/booksdash_en_all_{date}.zim (2024-06) | ZIM | 11,162 | CC BY 4.0 | Kiwix; IIAB menus (`en-bookdash`) |
| Open Music Theory | …/zimit/openmusictheory.com_en_all_{date}.zim | ZIM | 73 | CC BY-SA | Kiwix; NOMAD TODO |

#### Food and agriculture (pack: homesteading)

| Resource | Source URL | Format | Size (MiB) | License | Seen in |
|---|---|---|---|---|---|
| FOSS Cooking | download.kiwix.org/zim/zimit/foss.cooking_en_all_{date}.zim (2026-11) | ZIM | 29 | open (verify) | NOMAD agriculture-essential |
| Public Domain Recipes | …/zimit/publicdomainrecipes.com_en_all_{date}.zim (2026-11) | ZIM | 16 | Unlicense / PD | Kiwix |
| Infonet-Biovision (farming, animal and human health for East Africa) | IIAB menu `en-infonet` (module source not resolved) | HTML module | not found | not verified | IIAB menus |

#### Computing (packs: computing, cybersec-reference)

| Resource | Source URL | Format | Size (MiB) | License | Seen in |
|---|---|---|---|---|---|
| DevDocs HTML, CSS, Node.js, React, Git, Docker, Bash | download.kiwix.org/zim/devdocs/devdocs_en_{html,css,node,react,git,docker,bash}_{date}.zim | ZIM | 1–5 each | per upstream docs (mostly MIT/CC) | NOMAD computing tiers |
| MDN Web Docs | …/zimit/developer.mozilla.org_en_all_{date}.zim (2026-11) | ZIM | 560 | CC BY-SA 2.5 | Kiwix |
| docs.python.org (full) | …/zimit/docs.python.org_en_all_{date}.zim (2026-11) | ZIM | 3,657 | PSF docs license | Kiwix |
| freeCodeCamp (all, plus Project Euler, Rosetta Code) | download.kiwix.org/zim/freecodecamp/freecodecamp_en_all_{date}.zim (2026-11) | ZIM | 11 | BSD-3 (curriculum) | NOMAD computing-essential |
| Go by Example | …/zimit/gobyexample.com_en_all_{date}.zim | ZIM | <1 | CC BY 3.0 | Kiwix |
| Lua.org (manual + PiL) | …/zimit/lua.org_en_all_{date}.zim | ZIM | 362 | site terms | Kiwix |
| Jeff Erickson, *Algorithms* | …/zimit/jeffe.cs.illinois.edu_en_all_{date}.zim | ZIM | 492 | CC BY 4.0 (book) | Kiwix |
| Open Data Structures | …/zimit/opendatastructures.org_en_all_{date}.zim | ZIM | 110 | CC BY 2.5 | Kiwix |
| Cloudflare Learning Center (networking/security primers) | …/zimit/cloudflare.com_en_learning-center_{date}.zim (2025-12) | ZIM | 181 | proprietary site (verify) | Kiwix |
| Cheatography | …/zimit/cheatography.com_en_all_{date}.zim (2025-07) | ZIM | 10,909 | per-sheet author licenses (verify) | Kiwix |
| Goalkicker programming books (from Stack Overflow Documentation) | goalkicker.com; Kolibri "Goalkicker Tech Books" (0.49 GB, 3,580 items) | PDF | ≈0.5 GB | CC BY-SA per SO Docs origin (not confirmed on site) | Kolibri |
| w3schools offline scrape | IIAB module `en-wl_w3schools` (≈1.5 GB) | HTML | ≈1.5 GB | proprietary; redistribution doubtful | IIAB module catalog |

#### Maps

| Resource | Source URL | Format | Size | License | Seen in |
|---|---|---|---|---|---|
| World OSM vector basemap, zoom 0–10 | rsync://medbox.iiab.me/modules/en-osm-omt-min | OpenMapTiles vector tiles | ≈1.9 GB | ODbL (OSM) | [IIAB module catalog](https://iiab-share.github.io/iiab-content/iiab-module-cat.json) |
| Per-US-state topographic PMTiles | github.com/Crosstalk-Solutions/project-nomad-maps/raw/refs/heads/master/pmtiles/<state>_2025-12.pmtiles | PMTiles | 28 MB (Hawaii) to 1.1 GB (California) | ODbL-derived (verify) | [NOMAD maps.json](https://github.com/Crosstalk-Solutions/project-nomad/blob/main/collections/maps.json) |

#### Problems found in existing recipes during the cross-check
- `content/usda-nutrition.yaml` describes `usda-2015_en` as the "USDA National Nutrient Database". The Kiwix catalog describes this ZIM as "Research-based recommendations for canning safer and better quality food at home" (18 MiB, 2025-04), which is the USDA home canning guide, not a nutrient database — [Kiwix catalog](https://library.kiwix.org/catalog/v2/entries?lang=eng)
- The newest Gutenberg LCC builds (lcc-r/s/t/u, dated `2026-11a`) are published under `download.kiwix.org/zim/books/`, while `/zim/gutenberg/` (which Svalbard's gutenberg recipes use) has only `gutenberg_en_lcc-s_2026-03.zim` as its latest. `gutenberg_en_all` is still at `/zim/gutenberg/` (2025-11, 211,001 MiB). Checked with directory listings and the Kiwix catalog hrefs on 2026-11-26.

### Inferences
- The cheapest wins are ZIMs that fit Svalbard's existing recipe shape with only a URL change: armypubs, gutenberg lcc-u, canadian-prepper ×3, beready UK, theworldfactbook, simple-en Wikipedia, wikivoyage-en, phet-en, the DevDocs set, MDN and freecodecamp. NOMAD ships most of the DevDocs/freeCodeCamp/Canadian Prepper set already, which is evidence that they are worth carrying.
- openFDA drug labels plus the NOMAD-style symptom/remedy JSON would add a public-domain medicine dataset to the survival-medical pack. It is ≈1.8 GB zipped, so it would likely need a builder step (e.g. into SQLite for Svalbard's search) rather than a raw download.
- The medical PDFs (Hesperian, MSF, WHO ship guide, ICRC, NIOSH) are the main distinctive content in personal doomsday builds, but several are NC or have unclear redistribution terms. Svalbard should treat them as user-fetched recipes (like the `.local.yaml` site archives) rather than as defaults.
- Kolibri-only content (CK-12, Sikana, MIT Blossoms) is best skipped or replaced with ZIM/PDF equivalents, given the server dependency.

### Gaps
- Licenses were not verified for: Skin of Color Society ZIM, Cloudflare Learning Center, Cheatography, Lua.org, Stanford EM videos, HealthPhone, MSF guidelines, Goalkicker (site text did not contain a license string), and Be ready UK (OGL assumed, not confirmed).
- The NIOSH Pocket Guide PDF, IFRC guidelines and solarspell.org returned HTTP 403 to automated requests, so their current URLs and sizes are unverified.
- The Hesperian 2025 PDFs are split per chapter with filenames that change between printings ([languages.hesperian.org warns of this](https://languages.hesperian.org/pages/uz/pdf.html)). A stable single-file URL was not found, and `hesperian.org/books-and-resources/...` pages returned 404.
- The Infonet-Biovision and Practical Action pack download sources were not resolved to a current URL.
- No complete RACHEL module list could be retrieved, because both catalog APIs and the module index page return 404. RACHEL-only modules beyond those in IIAB's catalog were therefore not enumerated.
