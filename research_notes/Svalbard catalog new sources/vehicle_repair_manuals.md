# Vehicle repair and maintenance manuals (cars, trucks, motorcycles, small engines, tractors, bicycles)

Scope: sources that the catalog does not already have. Already present, so excluded here: iFixit ZIM (`content/ifixit.yaml`), Motor Vehicle Maintenance & Repair SE (`content/stackexchange-mechanics.yaml`), Bicycles SE (`content/stackexchange-bicycles.yaml`), Restarters, FarOutRide, LibreTexts Workforce/Engineering, English Wikibooks (`content/wikibooks-en.yaml`), Gutenberg LCC-T Technology (`content/gutenberg-en-lcc-t.yaml`), Appropedia, Wikiciv, DIY SE, FAS Military Medicine (per the catalog index, `/private/tmp/claude-501/-Users-pkronstrom-Projects-own-svalbard/2fd22a2d-583e-439a-9d48-6da08ba2799a/scratchpad/catalog-index.txt`).

Status: checked 2026-11-26. Three classes are used below: **(A) redistributable** (public domain or open licence, fine to ship in a catalog recipe), **(B) free to download but not redistributable** (each user fetches it, via `svalbard import` or a locally built recipe), and **(C) legally uncertain** (flagged, not recommended).

## How "bring your own manual" fits the existing Svalbard seams

### Takeaway
A per-vehicle manual already fits without a new recipe: `svalbard import <file.pdf> --vault <path> --add` copies one file into `library/`, adds a `local:<name>` id to the desired items, and records it as realized with `source_strategy: local`. The `*.local.yaml` recipes are a different thing: recipes that the host builds by mirroring a website (wget to WARC, then warc2zim). They are the existing pattern for sites that are fetchable but not redistributable.

### Cited Findings
- `svalbard import <file> --vault <path> [--add] [--name <name>]` "Import a local file into the vault library"; `--add` "also appends the new `local:*` id to desired state". Example given: `svalbard import ../manual.pdf --vault /Volumes/MyDrive --add --name field-manual`. Remote web or media ingestion is not part of `import`. Source: repo `docs/usage.md` (lines ~122-136).
- The command takes exactly one input (`cobra.ExactArgs(1)`). On `--add` it records a realized entry whose `Type` is the file extension (for example `pdf`), with `RelativePath` `library/<file>` and `SourceStrategy: "local"`. Sources: repo `host-cli/internal/cli/root.go` (`newImportCommand`) and `host-cli/internal/commands/import.go`. A test confirms that `local:guide` maps to `library/guide.pdf` of type `pdf` (`host-cli/internal/commands/import_test.go`).
- `*.local.yaml` recipes (for example `recipes/content/faroutride.local.yaml` and `open-source-ecology.local.yaml`) use `strategy: build`. They run `wget --mirror --warc-file ...`, then `warc2zim`, then a `verify` step. FarOutRide's licence is recorded as `All-Rights-Reserved`. This is the catalog's precedent: a site that may be read but not redistributed is built on the user's own machine and not downloaded from a mirror.
- `svalbard zim <url>` also exists: it crawls a website into a ZIM on the vault with the zimit container (needs Docker). Source: repo `host-cli/internal/cli/root.go` (`newZimCommand`).

### Inferences
- Manufacturer owner's manuals (class B) fit `svalbard import` well: the user downloads the PDF for their VIN or model and imports it. Two gaps for this use: importing a folder of PDFs (one car plus a mower plus a chainsaw) takes one call per file, and nothing groups or tags imported manuals (for example a "Vehicles" shelf). These are UX improvements, not blockers.
- Class B websites (such as Sheldon Brown) would follow the `faroutride.local.yaml` pattern, as long as the site's terms do not forbid crawling. Park Tool's terms do forbid it (see below).

### Gaps
- I did not check whether `importer.ImportLocalFile` accepts a directory or archive. Only the single-argument CLI was confirmed.

## Public-domain US military technical manuals: which are generic and high-value, and where they are hosted

### Takeaway
The best single generic automotive text is the US Army's *Principles of Automotive Vehicles* (TM 9-8000, 1985, with CHG-1 in 1988; its predecessor is TM 9-2700 from 1947). Both are on archive.org as clean PDFs of 18 MB and 30 MB. These are US government works, so they are class A. Kiwix also publishes an 8.2 GB `armypubs_en_all` ZIM that the catalog lacks, but I could not confirm whether it contains maintenance TMs. Welding (TC 9-237) is widely mirrored, but only by third parties.

### Cited Findings
- TM 9-8000 *Principles of Automotive Vehicles*: EverySpec lists it with CHG-1 dated 14 Nov 1988 and marks it active. The text "explains the automotive vehicle in general" and avoids "reference to any specific vehicle or unit wherever possible", which makes it generic. Sources: [EverySpec TM 9-8000 CHG-1](https://everyspec.com/ARMY/TM-Tech-Manual/TM_9-8000_CHG-1_48512/), [tpub HTML copy](https://automotiveenginemechanics.tpub.com/TM-9-8000).
- archive.org item `tm-9-8000-principles-of-automotive-vehicles-1985`: the file `TM9-8000_Principles_of_automotive_vehicles_1985.pdf` is 18,049,630 bytes, in the collections `manuals_contributions` and `manuals`. It has no licenseurl set. Source: [archive.org metadata](https://archive.org/metadata/tm-9-8000-principles-of-automotive-vehicles-1985); [item](https://archive.org/details/tm-9-8000-principles-of-automotive-vehicles-1985).
- archive.org item `TM9-2700` *Principles of Automotive Vehicles*, dated 1947-11-18: the file `TM9-2700.pdf` is 30,006,758 bytes. Its licenseurl is the Public Domain Mark 1.0, in the collection `military-manuals`. Source: [archive.org metadata](https://archive.org/metadata/TM9-2700). The UNT catalogue also lists TM 9-2700 (Nov 1947, Changes 1 on 13 Sept 1950) as the earlier version. Source: [UNT MoCat](https://mocat.library.unt.edu/catalog/642-14965).
- TM 9-2800 (1943, *Standard Military Motor Vehicles*) is on archive.org as a PDF. Source: [archive.org TM9-2800.pdf](https://ia802508.us.archive.org/29/items/TM9-2800/TM9-2800.pdf).
- TC 9-237 (*Welding: theory and application*) covers basic welding processes plus brazing and soldering. Copies found are mostly on Scribd, Slideshare and pdfcoffee, not an official host. The archive.org `MManuals` pack includes `UsArmyWeldingOperationsIi.pdf` and `Tm5-805UsArmyWeldingDesignProceduresInspection1985.pdf` (TM 5-805-7). Sources: [archive.org welding PDF](https://archive.org/download/MManuals/UsArmyWeldingOperationsIi.pdf), [archive.org TM 5-805-7](https://archive.org/download/MManuals/Tm5-805UsArmyWeldingDesignProceduresInspection1985.pdf), [Slideshare TC 9-237](https://es.slideshare.net/stickweld/theory-of-welding-us-army-tc-9237-1498913-1498913).
- The archive.org `MManuals` ("Military Manuals", collection `manual_packs`) is a large mixed pack. Besides government documents it holds commercial firearms manuals (for example `ArmaliteAr10M15OperatorsManual1997.pdf`, `Beretta90Two.pdf`). Source: [archive.org metadata MManuals](https://archive.org/metadata/MManuals).
- Many vehicle-specific Army TMs (forklifts, 5-ton trucks, construction loaders, generator-driven welders) are on archive.org and tpub, for example `TM10-3930-643-20` (forklift organizational maintenance, Public Domain Mark) and TM 5-3431-221-15 (gasoline-driven arc welding generator). Sources: [archive.org TM10-3930-643-20](https://archive.org/details/TM10-3930-643-20), [tpub welding TMs](https://welding.tpub.com/TM-5-3431-229-13/TM-5-3431-229-130015.htm).
- Kiwix OPDS (queried 2026-11-26) lists `armypubs_en_all` ("Army Publishing Directorate - The United States Army's centralized publications and forms management resource"): 8,232,397,824 bytes, updated 2024-12-06, at `https://download.kiwix.org/zim/zimit/armypubs_en_all_2024-12.zim`. The catalog does not have it. Source: [Kiwix OPDS catalog](https://library.kiwix.org/catalog/v2/entries?count=2000&lang=eng).
- Kiwix also lists `gutenberg_en_lcc-u` (Military science), 906 MB, 2026-11-13. Source: same OPDS feed.

### Inferences
- **Recommended class A recipe:** a small "military automotive primer" PDF bundle. It would hold TM 9-8000 (1985) and TM 9-2700 (1947) from archive.org, plus a welding TM. Each file needs a sha256 pin, because the archive.org uploads are third-party scans. TM 9-8000 is the high-value one: it covers engines, fuel, cooling, lubrication, electrical, clutch, transmission, axles, brakes, steering and suspension. It is generic and offline-friendly, and about 50 MB covers the core.
- Vehicle-specific Army TMs (HMMWV, M35 and so on) are low value for civilians. Exceptions are generic equipment TMs: generators, forklifts and welders. Only cherry-pick from `MManuals`. Do not mirror the whole pack, because parts of it are not public domain.
- `armypubs_en_all` (8.2 GB, about 2 years old) is a candidate for an "everything military doctrine" tier, but on its own it is not a clear vehicle-repair source. APD mainly publishes doctrine (FM/ATP/TC) and admin documents, and many equipment TMs are distribution-restricted. This needs checking before it is recommended for this purpose.

### Gaps
- I could not open the `armypubs_en_all` ZIM's contents: the Kiwix browse endpoint returned an "Access confirmation" bot-check page. Whether it includes TM 9-8000, TC 9-237 or small-engine and generator TMs is unverified.
- I did not find an EverySpec PDF download URL or an official host for TC 9-237, and I did not verify TM 5-685 (believed to cover generator operation) at all.
- The archive.org TM 9-8000 1985 item has no licence field set. Its PD status rests on its being a US government work (17 U.S.C. §105), not on an archive.org label.

## Open and CC-licensed sources: Wikibooks, Kiwix ZIMs, LibreTexts, Sheldon Brown, Park Tool

### Takeaway
There is no dedicated vehicle or bicycle repair ZIM in the Kiwix English catalog (as of Nov 2026) beyond the SE and iFixit ZIMs already present. Wikibooks automotive content is thin and already covered by `wikibooks-en`. Sheldon Brown carries no open licence (class B, per-user crawl only). Park Tool's terms explicitly forbid mirroring and robots, so exclude it.

### Cited Findings
- A filtered scan of the Kiwix English OPDS catalog (2026-11-26) for bike/bicycle/car/automotive/motor/mechanic/vehicle/tractor/engine/diesel/weld/repair titles turned up only items already in the catalog (mechanics SE 342 MB, bicycles SE 499 MB, iFixit 3.57 GB dated 2025-12-21, restarters) or unrelated ones. It found nothing for Sheldon Brown, Park Tool or any car or motorcycle wiki. Adjacent ZIMs that are not in the catalog: `aviation.stackexchange.com_en_all` (510 MB, 2026-10-20) and `ted_mul_engineering`. Source: [Kiwix OPDS catalog](https://library.kiwix.org/catalog/v2/entries?count=2000&lang=eng).
- Wikibooks "Subject:Automotive engineering" lists *Electric Vehicle Conversion* (nearing completion), *How To Build A Teardrop Trailer* (half-finished) and *Automotive Systems* (partly developed). The subject page was last edited in 2017. Through the Wikibooks API, `Automotive_Systems` is 9,643 bytes and `Electric_Vehicle_Conversion` is 40,276 bytes. Source: [Wikibooks Subject:Automotive engineering](https://en.wikibooks.org/wiki/Subject:Automotive_engineering).
- Park Tool terms: viewing is licensed only for "your own personal, noncommercial use". Users may not "link to, mirror or frame any portion of the Site". Using "any robot, spider, other automatic device, or manual process to" copy pages needs prior, express, written permission. Source: [Park Tool Terms of Use](https://www.parktool.com/en-us/terms-of-use).
- The Sheldon Brown homepage states no licence or mirroring terms. Pages carry "Last Updated: by John Allen". Source: [sheldonbrown.com](https://www.sheldonbrown.com/).

### Inferences
- Wikibooks automotive books are already in `wikibooks-en` (nopic). Nothing to add.
- Sheldon Brown is the highest-value bicycle reference. With no licence it defaults to all rights reserved, so class B: a `sheldonbrown.local.yaml` build recipe (wget to warc2zim), following the FarOutRide precedent. The site is static HTML, so a crawl should be small, though I did not measure the size. Asking the maintainers for redistribution permission would be needed before any hosted mirror.
- Park Tool: exclude, including from local crawling, because the ToS prohibits robots.

### Gaps
- LibreTexts Workforce automotive coverage is unverified. The Workforce bookshelf page renders client-side, so a grep found nothing, and web search returned no LibreTexts automotive or diesel text. I found no evidence of an automotive or diesel shelf. Checking the ZIM's index directly would settle it.
- I did not find an OpenStax automotive title, nor confirm that none exists.
- Other candidates I did not check: open-licensed bicycle manuals (for example community "bike kitchen" guides) and motorcycle wikis.

## Gray area: Operation CHARM (charm.li) and its successor LEMON

### Takeaway
CHARM is a free, anonymously run site of factory-grade repair manuals for many makes, 1982 to 2013. It publishes its whole database and code as a torrent of more than 700 GB. It claims a right-to-repair rationale but states no licence or rights grant. The material is copyrighted OEM or third-party service information, so this is **legally uncertain or likely infringing (class C)**. Do not ship it as a catalog recipe. At most, document it as something the user can obtain themselves.

### Cited Findings
- charm.li about page: "The Collection of High-quality Auto Repair Manuals spans many makes and models from 1982 through 2013. Our data will be available free of charge, permanently. You are entitled the right to repair, understand, and upgrade what's yours without paying extra for a workshop manual." Source: [charm.li/about.html](https://charm.li/about.html).
- Distribution, from the same page: "We release the database and computer code that powers Operation CHARM publicly... The download is over 700GB and may be difficult to use if you are not a programmer." It is offered via a P2P torrent (`operation-charm.torrent`, with the note "Add charm.li:17471 as a peer"). The operators are "a group of anonymous programmers" and the contact is a tuta.io address. Source: [charm.li/about.html](https://charm.li/about.html).
- The home page lists about 50 makes (Acura through Yugo, including Freightliner, UD and Workhorse commercial trucks). It carries a banner: "Manuals through 2025 now available! Our trusted friends have launched a new website named LEMON, which has newer manuals. It also contains all the CHARM manuals." The domains given are lemon-manuals.la and lemon-manuals.org.ua, with connection issues noted. Source: [charm.li](https://charm.li/). A Lemmy post also describes LEMON as CHARM's successor. Source: [lemmy.uninsane.org](https://lemmy.uninsane.org/post/958493).
- A Korben article (about 2 years old) also gives the torrent size as more than 700 GB and the range as 1982 to 2013. Source: [korben.info](https://korben.info/en/operation-charm-free-auto-repair-manuals.html).

### Inferences
- Neither site states a licence, and its operators are anonymous. The content is the kind normally sold under licence (OEM or ALLDATA-type service data). Redistributing it from a Svalbard mirror, or listing it as a catalog recipe, would carry real copyright exposure. Right-to-repair laws (for example the US state laws or the EU vehicle repair-information rules) require manufacturers to *sell or provide* repair information. None of them grants third parties a right to republish it.
- 700 GB is also far beyond a typical USB vault. Even a self-hosted subset, such as one make, would need tooling to extract it from CHARM's database format.
- If it is mentioned at all: describe it in docs as user-obtained material that `svalbard import` can bring in, with an explicit legal-uncertainty warning. No URL template, and no recipe.

### Gaps
- I did not verify the commonly repeated claim that the CHARM data is derived from ALLDATA. charm.li itself does not name its source.
- I did not verify LEMON's size, coverage or availability. The charm.li banner itself reports connection problems.
- I found no takedown or legal action against CHARM, and no reliable reporting either way.

## Manufacturer owner's manual portals: bulk fetch or per vehicle?

### Takeaway
The major automakers (Ford, Toyota, Honda and others) offer free owner's manuals, but only per vehicle through VIN or year-and-model lookup. They grant no redistribution licence, and bulk scraping is neither intended nor licensed. This is class B: a per-user download brought in with `svalbard import --add`. Workshop or service manuals are generally paid (for example Toyota TIS).

### Cited Findings
- Ford: a free digital owner's manual is available "through the Ford website or your vehicle's SYNC 4A or SYNC 4 screen". Copies (including warranty guides) are on Ford Support by "entering either your VIN or your vehicle's year and model". Source: [Ford support](https://www.ford.com/support/how-tos/owner-resources/vehicle-documents/where-can-i-get-an-owners-manual/).
- Toyota: there is an owner's manual lookup on the owners site. For Canada, manuals are downloadable "for the majority of vehicles manufactured in 2008 or later". For most models produced before 1990, the user must subscribe to the Technical Information System (TIS). Sources: [Toyota owners manuals](https://www.toyota.com/owners/web/pages/resources/owners-manuals), [Toyota Canada manuals](https://www.toyota.ca/toyota/en/owners/manuals).
- Honda: owner's manuals go back to 1980, per third-party guides. Source: [Edmunds how-to](https://edmunds.com/how-to/how-to-find-your-car-owners-manual-online.html). VW UK has an owner's manual portal. Source: [VW UK manuals](https://www.volkswagen.co.uk/en/owners-and-services/my-car/manuals-landing.html).

### Inferences
- Owner's manuals cover operation, fluids, capacities, fuses, bulbs and maintenance schedules. They are high-value offline for a user's own vehicle but low-value as a generic bundle. A wizard hint such as "Add your vehicle's owner's manual (download from the maker's site, then `svalbard import`)" fits better than a recipe.

### Gaps
- I did not inspect any portal's terms of use for explicit bulk-download or redistribution clauses. The "not redistributable" status rests on default copyright, not on a quoted clause.

## Small engines, tractors, chainsaws, outboards

### Takeaway
Briggs & Stratton and Honda engines publish official **operator's/owner's** manuals as free PDFs per model number (class B, per user). Service or shop manuals are sold. Nothing here is redistributable as a bundle. The generic public-domain fallback is the Army generator, forklift and engine TMs in the military section above.

### Cited Findings
- Briggs & Stratton hosts operator's manual PDFs on its support site, keyed by model number. Examples: the manual for 083100 engines (2024), at form `80131412_A`, and the manual for models 400000/440000/490000, at form `80107982_A`. The manuals page asks the user to identify the engine model number format first. Sources: [B&S 80131412_A](https://www.briggsandstratton.com/na/en_us/support/results/_jcr_content/par/productmanuallisting.downloadmanual.80131412_A_LO.pdf.html), [B&S 80107982_A](https://www.briggsandstratton.com/na/en_us/support/results/_jcr_content/par/productmanuallisting.downloadmanual.80107982_A_LO.pdf.html), [B&S manuals page](https://m.briggsandstratton.com/as/en_ph/support/manuals.html).
- Honda Power Equipment offers free owner's manuals to download. Example engine PDFs are on its CDN (for example `cdn.powerequipment.honda.com/engines/pdf/manuals/00X31Z0A6021.pdf`, the GX610/GX620 owner's manual). Shop manuals are sold as "Honda Genuine service materials for purchase". Sources: [Honda PE owners manuals](https://direct.powerequipment.honda.com/support/owners-manuals), [Honda GX610/620 PDF](https://cdn.powerequipment.honda.com/engines/pdf/manuals/00X31Z0A6021.pdf), [Honda Engines shop manuals](https://engines.honda.com/support-and-service/shop-manuals).
- archive.org also holds third-party uploads of older Briggs & Stratton manuals (from the manualsonline collection). Source: [archive.org manualsonline B&S](https://archive.org/download/manualsonline-id-511b8d1c-98ac-5604-ed97-b85881ff67c5/511b8d1c-98ac-5604-ed97-b85881ff67c5.pdf).

### Inferences
- A tempting option is a small curated "common engines" bundle: B&S operator manuals for the most common single-cylinder families plus the Honda GX160/GX200/GX390 owner's manuals. It is small (each PDF is a few MB) and very useful off-grid for generators, pumps and tillers. However, nothing indicates it may be redistributed, so it would be a class B per-user recipe at best. Pinned direct manufacturer URLs that are fetched at apply time would mean the user downloads from the manufacturer and Svalbard does not redistribute. This is legally cleaner than mirroring, but whether the manufacturers object to scripted fetching is unverified.
- archive.org third-party uploads of manufacturer manuals (manualsonline and "Liberated Manuals" items) are not cleared for redistribution despite being on archive.org. Treat them as class C.

### Gaps
- I did not check Stihl, Husqvarna, Kohler, Kubota, John Deere (operator's manuals are partly free on its "Technical Information Bookstore"), Mercury, Yamaha or Evinrude outboards, or motorcycle makers. Their portals are presumably per model as well, but I did not verify this.
- I found no openly licensed small-engine repair textbook.
